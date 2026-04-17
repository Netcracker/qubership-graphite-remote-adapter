// Copyright 2026 NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package graphite

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"log/slog"

	graphiteCfg "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/paths"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeSample(metricName string, timestamp int64, value float64) *model.Sample {
	return &model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel: model.LabelValue(metricName),
			"owner":               "team-X",
		},
		Value:     model.SampleValue(value),
		Timestamp: model.Time(timestamp),
	}
}

func TestPrepareWriteSplitsUDPBuffers(t *testing.T) {
	client := &Client{
		cfg: &graphiteCfg.Config{
			Write: graphiteCfg.WriteConfig{
				CarbonTransport: "udp",
			},
		},
		logger: slog.New(slog.DiscardHandler),
		format: paths.FormatCarbon,
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com?graphite.default-prefix=prefix.", nil)

	samples := make(model.Samples, 0, 200)
	for i := 0; i < 200; i++ {
		samples = append(samples, makeSample("test", int64(1+i), float64(i)))
	}

	buffers, err := client.prepareWrite(samples, 256, req)
	require.NoError(t, err)
	require.Greater(t, len(buffers), 1)
	for _, buf := range buffers {
		assert.NotZero(t, buf.Len())
	}
	assert.Contains(t, buffers[0].String(), "prefix.test.owner.team-X")
}

func TestWriteDryRunReturnsPayload(t *testing.T) {
	client := &Client{
		cfg: &graphiteCfg.Config{
			Write: graphiteCfg.WriteConfig{
				CarbonAddress:   ":2003",
				CarbonTransport: "tcp",
			},
		},
		logger: slog.New(slog.DiscardHandler),
		format: paths.FormatCarbon,
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	samples := model.Samples{makeSample("test", int64(time.Now().Unix()), 1.23)}

	payload, err := client.Write(samples, 1024, req, true)
	require.NoError(t, err)
	assert.Contains(t, string(payload), "test")
	assert.Contains(t, string(payload), "\n")
}

func TestWriteReturnsContextCancelled(t *testing.T) {
	client := &Client{
		cfg: &graphiteCfg.Config{
			Write: graphiteCfg.WriteConfig{
				CarbonAddress:   ":2003",
				CarbonTransport: "tcp",
			},
		},
		logger: slog.New(slog.DiscardHandler),
		format: paths.FormatCarbon,
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	result, err := client.Write(model.Samples{makeSample("test", int64(time.Now().Unix()), 1.23)}, 1024, req, false)
	require.Error(t, err)
	assert.Equal(t, "context cancelled.", string(result))
	assert.Contains(t, err.Error(), "request context cancelled")
}

func TestConnectToCarbonReusesConnection(t *testing.T) {
	client := &Client{
		cfg: &graphiteCfg.Config{
			Write: graphiteCfg.WriteConfig{
				CarbonTransport:         "tcp",
				CarbonAddress:           "127.0.0.1:0",
				CarbonReconnectInterval: time.Minute,
			},
		},
		logger: slog.New(slog.DiscardHandler),
	}

	c1, c2 := net.Pipe()
	defer func() {
		_ = c2.Close()
	}()
	client.carbonCon = c1
	client.carbonLastReconnectTime = time.Now()

	conn, err := client.connectToCarbon()
	require.NoError(t, err)
	assert.Equal(t, c1, conn)
}

func TestCompressLZ4WritesCompressedData(t *testing.T) {
	client := &Client{
		cfg:    &graphiteCfg.Config{},
		logger: slog.New(slog.DiscardHandler),
	}

	pipeReader, pipeWriter := io.Pipe()
	var compressed bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&compressed, pipeReader)
		close(readDone)
	}()

	buf := bytes.NewBufferString("hello world\n")
	written, err := client.compressLZ4(pipeWriter, buf)
	require.NoError(t, err)
	require.NoError(t, pipeWriter.Close())
	assert.Greater(t, written, int64(0))
	<-readDone
	assert.Greater(t, compressed.Len(), 0)
}

func TestReadReturnsTimeseriesFromGraphite(t *testing.T) {
	oldFetchURL := FetchURL
	defer func() { FetchURL = oldFetchURL }()

	FetchURL = func(ctx context.Context, logger *slog.Logger, u *url.URL) ([]byte, error) {
		switch u.Path {
		case expandEndpoint:
			return []byte(`{"results":["prefix.test.owner.team-X"]}`), nil
		case renderEndpoint:
			return []byte(`[{"target":"prefix.test.owner.team-X","datapoints":[[1.0,123],[2.0,124]]}]`), nil
		default:
			return nil, fmt.Errorf("unexpected path %s", u.Path)
		}
	}

	client := &Client{
		cfg: &graphiteCfg.Config{
			Read:          graphiteCfg.ReadConfig{URL: "http://localhost"},
			DefaultPrefix: "prefix.",
		},
		logger:      slog.New(slog.DiscardHandler),
		format:      paths.FormatCarbon,
		readDelay:   0,
		readTimeout: 5 * time.Second,
	}

	req := httptest.NewRequest(http.MethodPost, "http://example.com?graphite.default-prefix=prefix.", nil)
	now := time.Now().Unix()
	query := &prompb.Query{
		StartTimestampMs: (now - 10) * 1000,
		EndTimestampMs:   now * 1000,
		Matchers: []*prompb.LabelMatcher{
			{Name: model.MetricNameLabel, Type: prompb.LabelMatcher_EQ, Value: "test"},
		},
	}

	resp, err := client.Read(&prompb.ReadRequest{Queries: []*prompb.Query{query}}, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results[0].Timeseries, 1)
	series := resp.Results[0].Timeseries[0]
	assert.Equal(t, 2, len(series.Samples))

	labelMap := make(map[string]string, len(series.Labels))
	for _, label := range series.Labels {
		labelMap[label.Name] = label.Value
	}
	assert.Equal(t, "test", labelMap[model.MetricNameLabel])
	assert.Equal(t, "team-X", labelMap["owner"])
}

func TestReadSkipsFutureQueryRange(t *testing.T) {
	client := &Client{
		cfg: &graphiteCfg.Config{
			Read: graphiteCfg.ReadConfig{URL: "http://localhost"},
		},
		logger:      slog.New(slog.DiscardHandler),
		format:      paths.FormatCarbon,
		readDelay:   0,
		readTimeout: 5 * time.Second,
	}

	now := time.Now().Unix()
	query := &prompb.Query{
		StartTimestampMs: (now + 10) * 1000,
		EndTimestampMs:   (now + 20) * 1000,
		Matchers: []*prompb.LabelMatcher{
			{Name: model.MetricNameLabel, Type: prompb.LabelMatcher_EQ, Value: "test"},
		},
	}

	resp, err := client.Read(&prompb.ReadRequest{Queries: []*prompb.Query{query}}, httptest.NewRequest(http.MethodPost, "http://example.com", nil))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Empty(t, resp.Results[0].Timeseries)
}

func TestQueryToTargetsWithTagsUnknownMatchType(t *testing.T) {
	client := &Client{logger: slog.New(slog.DiscardHandler)}
	query := &prompb.Query{
		Matchers: []*prompb.LabelMatcher{{Name: "label", Type: prompb.LabelMatcher_Type(999), Value: "value"}},
	}

	_, err := client.QueryToTargetsWithTags(context.Background(), query, "prefix.")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown match type")
}
