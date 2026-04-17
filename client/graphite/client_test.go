// Copyright 2020 Charles-Antoine Mathieu authored and melchiormoulin committed
// Copyright NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package graphite

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"log/slog"

	graphiteCfg "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/paths"
	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		Graphite: graphiteCfg.Config{
			Write: graphiteCfg.WriteConfig{
				CarbonAddress:           ":2003",
				EnablePathsCache:        true,
				PathsCacheTTL:           7 * time.Minute,
				PathsCachePurgeInterval: 0,
			},
		},
	}
	cfg.Write.Timeout = 5 * time.Minute
	cfg.Read.Timeout = 5 * time.Minute
	cfg.Read.Delay = time.Hour
	logger := slog.New(slog.DiscardHandler)

	client := NewClient(cfg, logger)
	defer client.Shutdown()

	assert.NotNil(t, client)
	assert.Equal(t, logger, client.logger)
	assert.Equal(t, &cfg.Graphite, client.cfg)
	assert.Equal(t, 5*time.Minute, client.writeTimeout)
	assert.Equal(t, 5*time.Minute, client.readTimeout)
	assert.Equal(t, time.Hour, client.readDelay)
}

func TestNewClientGraphiteCfg(t *testing.T) {
	gcfg := &graphiteCfg.Config{}
	logger := slog.New(slog.DiscardHandler)

	client := NewClientGraphiteCfg(gcfg, logger)

	assert.NotNil(t, client)
	assert.Equal(t, logger, client.logger)
	assert.Equal(t, gcfg, client.cfg)
}

func TestClient_Name(t *testing.T) {
	client := &Client{}
	assert.Equal(t, "graphite", client.Name())
}

func TestClient_Target(t *testing.T) {
	client := &Client{
		cfg: &graphiteCfg.Config{
			Write: graphiteCfg.WriteConfig{
				CarbonAddress: ":2003",
			},
			Read: graphiteCfg.ReadConfig{
				URL: "http://localhost:8080",
			},
		},
	}
	// Since no connection, returns "unknown"
	assert.Equal(t, "unknown", client.Target())
}

func TestClient_String(t *testing.T) {
	client := &Client{
		cfg: &graphiteCfg.Config{
			Write: graphiteCfg.WriteConfig{
				CarbonAddress: ":2003",
			},
		},
	}
	// Returns the config string
	str := client.String()
	assert.Contains(t, str, ":2003")
}

func TestClient_Cfg(t *testing.T) {
	cfg := &graphiteCfg.Config{}
	client := &Client{
		cfg: cfg,
	}
	assert.Equal(t, cfg, client.Cfg())
}

func TestNewClient_ReturnsNilWhenConfigEmpty(t *testing.T) {
	cfg := &config.Config{}
	logger := slog.New(slog.DiscardHandler)
	client := NewClient(cfg, logger)
	assert.Nil(t, client)
}

func TestNewClient_UsesOpenMetricsFormatWhenEnabled(t *testing.T) {
	cfg := &config.Config{
		Graphite: graphiteCfg.Config{
			Write: graphiteCfg.WriteConfig{
				CarbonAddress:    ":2003",
				EnablePathsCache: false,
			},
			EnableTags:           true,
			UseOpenMetricsFormat: true,
		},
	}
	cfg.Write.Timeout = 5 * time.Minute
	cfg.Read.Timeout = 5 * time.Minute
	cfg.Read.Delay = time.Hour
	logger := slog.New(slog.DiscardHandler)

	client := NewClient(cfg, logger)
	require.NotNil(t, client)
	assert.Equal(t, paths.FormatCarbonOpenMetrics, client.format)
}

func TestClient_ShutdownClosesConnection(t *testing.T) {
	client := &Client{}
	c1, c2 := net.Pipe()
	client.carbonCon = c1

	client.Shutdown()

	_, err := c2.Write([]byte("x"))
	assert.Error(t, err)
}

func TestClient_TargetWithConnection(t *testing.T) {
	client := &Client{}
	c1, c2 := net.Pipe()
	client.carbonCon = c1
	defer func() {
		if err := c2.Close(); err != nil {
			t.Logf("Error closing pipe: %v", err)
		}
	}()

	target := client.Target()
	assert.NotEqual(t, "unknown", target)
}

func TestQueryToTargetsWithTags(t *testing.T) {
	client := &Client{}
	query := &prompb.Query{
		Matchers: []*prompb.LabelMatcher{
			{Name: model.MetricNameLabel, Type: prompb.LabelMatcher_EQ, Value: "test"},
			{Name: "owner", Type: prompb.LabelMatcher_NEQ, Value: "team-X"},
			{Name: "region", Type: prompb.LabelMatcher_RE, Value: "us-.*"},
			{Name: "env", Type: prompb.LabelMatcher_NRE, Value: "prod"},
		},
	}

	targets, err := client.QueryToTargetsWithTags(context.Background(), query, "prefix.")
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Contains(t, targets[0], "seriesByTag(")
	assert.Contains(t, targets[0], "\"name=prefix.test\"")
	assert.Contains(t, targets[0], "\"owner!=team-X\"")
	assert.Contains(t, targets[0], "\"region=~^(us-.*)$\"")
	assert.Contains(t, targets[0], "\"env!=~^(prod)$\"")
}

func TestFilterTargets(t *testing.T) {
	client := &Client{
		logger: slog.New(slog.DiscardHandler),
	}
	query := &prompb.Query{
		Matchers: []*prompb.LabelMatcher{
			{Name: model.MetricNameLabel, Type: prompb.LabelMatcher_EQ, Value: "test"},
			{Name: "owner", Type: prompb.LabelMatcher_EQ, Value: "team-X"},
		},
	}
	graphitePrefix := "prefix."
	targets := []string{
		"prefix.test.owner.team-X",
		"prefix.test.owner.team-Y",
	}

	filtered, err := client.filterTargets(query, targets, graphitePrefix)
	require.NoError(t, err)
	require.Equal(t, []string{"prefix.test.owner.team-X"}, filtered)
}

func TestSamplesFromDatapointsInterpolation(t *testing.T) {
	timestamp1 := int64(1)
	timestamp2 := int64(4)
	val1 := 1.0
	val2 := 4.0
	datapoints := []*Datapoint{
		{Value: &val1, Timestamp: timestamp1},
		{Value: &val2, Timestamp: timestamp2},
	}

	samples := samplesFromDatapoints(datapoints, 1*time.Second)
	require.Len(t, samples, 4)
	assert.Equal(t, prompb.Sample{Value: 1.0, Timestamp: timestamp1 * 1000}, samples[0])
	assert.Equal(t, prompb.Sample{Value: 2.0, Timestamp: 2 * 1000}, samples[1])
	assert.Equal(t, prompb.Sample{Value: 3.0, Timestamp: 3 * 1000}, samples[2])
	assert.Equal(t, prompb.Sample{Value: 4.0, Timestamp: timestamp2 * 1000}, samples[3])
}

func TestSamplesFromDatapointsSkipsNilValue(t *testing.T) {
	val := 1.0
	datapoints := []*Datapoint{
		{Value: &val, Timestamp: 1},
		{Value: nil, Timestamp: 2},
	}

	samples := samplesFromDatapoints(datapoints, 0)
	require.Len(t, samples, 1)
}

func TestMin(t *testing.T) {
	assert.Equal(t, 1, min(1, 2))
	assert.Equal(t, 1, min(2, 1))
}

func TestDatapointUnmarshalJSON(t *testing.T) {
	var dp Datapoint
	data := []byte("[1.5, 12345]")
	require.NoError(t, json.Unmarshal(data, &dp))
	require.NotNil(t, dp.Value)
	assert.Equal(t, 1.5, *dp.Value)
	assert.Equal(t, int64(12345), dp.Timestamp)
}
