// Copyright 2015 The Prometheus Authors
// Copyright 2017 Corentin Chary <corentin.chary@gmail.com>
// Copyright 2024-2025 NetCracker Technology Corporation
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
//

package graphite

import (
	"net"
	"sync"
	"time"

	graphiteCfg "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/paths"
	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"log/slog"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	expandEndpoint  = "/metrics/expand"
	renderEndpoint  = "/render/"
	maxFetchWorkers = 10
)

// Client allows sending batches of Prometheus samples to Graphite.
type Client struct {
	//lock           sync.RWMutex
	cfg            *graphiteCfg.Config
	writeTimeout   time.Duration
	readTimeout    time.Duration
	readDelay      time.Duration
	ignoredSamples prometheus.Counter
	format         paths.Format

	carbonCon               net.Conn
	carbonLastReconnectTime time.Time
	carbonConLock           sync.Mutex

	logger *slog.Logger
}

// NewClient returns a new Client.
func NewClient(cfg *config.Config, logger *slog.Logger) *Client {
	if cfg.Graphite.Write.CarbonAddress == "" && cfg.Graphite.Read.URL == "" {
		return nil
	}
	if cfg.Graphite.Write.EnablePathsCache {
		paths.InitPathsCache(cfg.Graphite.Write.PathsCacheTTL,
			cfg.Graphite.Write.PathsCachePurgeInterval)
		logger.Debug("Paths cache initialized",
			"PathsCacheTTL", cfg.Graphite.Write.PathsCacheTTL,
			"PathsCachePurgeInterval", cfg.Graphite.Write.PathsCachePurgeInterval)
	}

	// Which format are we using to write points?
	format := paths.FormatCarbon
	if cfg.Graphite.EnableTags {
		if cfg.Graphite.UseOpenMetricsFormat {
			format = paths.FormatCarbonOpenMetrics
		} else {
			format = paths.FormatCarbonTags
		}
	}

	return &Client{
		logger:       logger,
		cfg:          &cfg.Graphite,
		writeTimeout: cfg.Write.Timeout,
		format:       format,
		readTimeout:  cfg.Read.Timeout,
		readDelay:    cfg.Read.Delay,
		ignoredSamples: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "remote_adapter_graphite",
				Name:      "ignored_samples_total",
				Help:      "The total number of samples not sent to Graphite due to unsupported float values (Inf, -Inf, NaN).",
			},
		),
		carbonCon:               nil,
		carbonLastReconnectTime: time.Time{},
		carbonConLock:           sync.Mutex{},
	}
}

// NewClient returns a new Client.
func NewClientGraphiteCfg(cfg *graphiteCfg.Config, logger *slog.Logger) *Client {
	return &Client{
		logger: logger,
		cfg:    cfg,
	}
}

// Shutdown the client.
func (client *Client) Shutdown() {
	client.carbonConLock.Lock()
	defer client.carbonConLock.Unlock()
	client.disconnectFromCarbon()
}

// Name implements the client.Client interface.
func (client *Client) Name() string {
	return "graphite"
}

// Target respond with a more low level representation of the client's remote
func (client *Client) Target() string {
	if client.carbonCon == nil {
		return "unknown"
	}
	return client.carbonCon.RemoteAddr().String()
}

// String implements the client.Client interface.
func (client *Client) String() string {
	// TODO: add more stuff here.
	return client.cfg.String()
}

// String implements the client.Client interface.
func (client *Client) Cfg() *graphiteCfg.Config {
	// TODO: add more stuff here.
	return client.cfg
}
