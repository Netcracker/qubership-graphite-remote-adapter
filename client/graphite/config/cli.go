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

package config

import (
	kgp "github.com/alecthomas/kingpin/v2"
)

// AddCommandLine setup Graphite specific cli args and flags.
func AddCommandLine(app *kgp.Application, cfg *Config) {
	app.Flag("graphite.default-prefix",
		"The prefix to prepend to all metrics exported to Graphite.").
		StringVar(&cfg.DefaultPrefix)

	app.Flag("graphite.read.url",
		"The URL of the remote Graphite Web server to send samples to.").
		StringVar(&cfg.Read.URL)

	app.Flag("graphite.read.max-point-delta",
		"If set, interval used to linearly interpolate intermediate points.").
		DurationVar(&cfg.Read.MaxPointDelta)

	app.Flag("graphite.write.carbon-address",
		"The host:port of the Graphite server to send samples to.").
		StringVar(&cfg.Write.CarbonAddress)

	app.Flag("graphite.write.carbon-transport",
		"Transport protocol to use to communicate with Graphite.").
		StringVar(&cfg.Write.CarbonTransport)

	app.Flag("graphite.write.enable-paths-cache",
		"Enables a cache to graphite paths lists for written metrics.").
		BoolVar(&cfg.Write.EnablePathsCache)

	app.Flag("graphite.write.paths-cache-ttl",
		"Duration TTL of items within the paths cache.").
		DurationVar(&cfg.Write.PathsCacheTTL)

	app.Flag("graphite.write.paths-cache-purge-interval",
		"Duration between purges for expired items in the paths cache.").
		DurationVar(&cfg.Write.PathsCachePurgeInterval)

	app.Flag("graphite.enable-tags",
		"Use Graphite tags.").
		BoolVar(&cfg.EnableTags)
}
