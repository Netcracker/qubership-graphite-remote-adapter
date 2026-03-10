// Copyright 2017 The Prometheus Authors
// Copyright 2024-2026 NetCracker Technology Corporation
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

// The main package for the Prometheus server executable.
package main

import (
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"dario.cat/mergo"
	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/web"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/version"
	"go.uber.org/automaxprocs/maxprocs"
)

func reload(cliCfg *config.Config, logger *slog.Logger) (*config.Config, error) {
	cfg := &config.DefaultConfig
	// Parse config file if needed
	if cliCfg.ConfigFile != "" {
		fileCfg, err := config.LoadFile(logger, cliCfg.ConfigFile)
		if err != nil {
			logger.Error("Error loading config file", "err", err)
			return nil, err
		}
		cfg = fileCfg
	}
	// Merge overwriting cliCfg into cfg
	if err := mergo.Merge(cfg, cliCfg, mergo.WithOverride); err != nil {
		logger.Error("Error merging config file with flags", "err", err)
		return nil, err
	}

	if cliCfg.Read.Delay == 0 {
		cfg.Read.Delay = cliCfg.Read.Delay
	}

	if cliCfg.Read.Timeout == 0 {
		cfg.Read.Timeout = cliCfg.Read.Timeout
	}

	if cliCfg.Write.Timeout == 0 {
		cfg.Write.Timeout = cliCfg.Write.Timeout
	}

	return cfg, nil
}

func main() {
	cliCfg := config.ParseCommandLine()

	logger := promslog.New(&promslog.Config{Level: &cliCfg.LogLevel, Format: promslog.NewFormat()})

	logger.Info("Starting graphite-remote-adapter", "version", version.Info(), "build_context", version.BuildContext())

	undo, err := maxprocs.Set()
	defer undo()
	if err != nil {
		logger.Error("failed to set GOMAXPROCS", "err", err)
		return
	}

	// Load the config once.
	cfg, err := reload(cliCfg, logger)
	if err != nil {
		logger.Error("Error first loading config", "err", err)
		return
	}

	webHandler := web.New(logger.With("component", "web"), cfg)
	if err = webHandler.ApplyConfig(cfg); err != nil {
		logger.Error("Error applying webHandler config", "err", err)
		return
	}

	// Tooling to dynamically reload the config for each clients.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-hup:
				cfg, err = reload(cliCfg, logger)
				if err != nil {
					logger.Error("Error reloading config", "err", err)
					continue
				}
				if err = webHandler.ApplyConfig(cfg); err != nil {
					logger.Error("Error applying webHandler config", "err", err)
					continue
				}
				logger.Info("Reloaded config file")
			case rc := <-webHandler.Reload():
				cfg, err = reload(cliCfg, logger)
				if err != nil {
					logger.Error("Error reloading config", "err", err)
					rc <- err
				} else if err = webHandler.ApplyConfig(cfg); err != nil {
					logger.Error("Error applying webHandler config", "err", err)
					rc <- err
				} else {
					logger.Info("Reloaded config file")
					rc <- nil
				}
			}
		}
	}()

	err = webHandler.Run()
	if err != nil {
		logger.Warn("Run error", "err", err)
	}
	logger.Info("See you next time!")
}
