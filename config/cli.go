// Copyright 2017 Thibault Chataigner <thibault.chataigner@gmail.com>
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
	"fmt"
	"os"
	"path/filepath"

	graphite "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/alecthomas/kingpin/v2"
	"github.com/pkg/errors"
	promslogflag "github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
)

// ParseCommandLine parse flags and args from cli.
func ParseCommandLine() *Config {
	cfg := &Config{}

	a := kingpin.New(filepath.Base(os.Args[0]), "The Graphite remote adapter")

	a.Version(version.Print("graphite-remote-adapter"))

	a.HelpFlag.Short('h')

	a.Flag("config.file", "Graphite-remote-adapter configuration file path.").
		StringVar(&cfg.ConfigFile)

	a.Flag("web.listen-address", "Address to listen on for UI and telemtry.").
		StringVar(&cfg.Web.ListenAddress)

	a.Flag("web.telemetry-path", "Path to listen for telemtry.").
		StringVar(&cfg.Web.TelemetryPath)

	a.Flag("write.timeout",
		"Maximum duration before timing out remote write requests. Default is 5m").
		Default(DefaultConfig.Write.Timeout.String()).
		DurationVar(&cfg.Write.Timeout)

	a.Flag("read.timeout",
		"Maximum duration before timing out remote read requests. Default is 5m").
		Default(DefaultConfig.Read.Timeout.String()).
		DurationVar(&cfg.Read.Timeout)

	a.Flag("read.delay",
		"Duration ignoring recent samples from all remote read requests. Default is 1h").
		Default(DefaultConfig.Read.Delay.String()).
		DurationVar(&cfg.Read.Delay)

	a.Flag("read.ignore-error",
		"Avoid returning error to promtheus returning empty result instead.").
		BoolVar(&cfg.Read.IgnoreError)

	// Add logLevel flag
	a.Flag(promslogflag.LevelFlagName, promslogflag.LevelFlagHelp).
		Default("info").SetValue(&cfg.LogLevel)

	// Add graphite flag
	graphite.AddCommandLine(a, &cfg.Graphite)

	_, err := a.Parse(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error parsing commandline arguments"))
		a.Usage(os.Args[1:])
		os.Exit(2)
	}
	return cfg
}
