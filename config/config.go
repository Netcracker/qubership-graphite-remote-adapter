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
	"strings"
	"time"

	graphite "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/utils"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/promlog"
	"gopkg.in/yaml.v3"
)

// Load parses the YAML input s into a Config.
func Load(s string) (*Config, error) {
	cfg := &Config{}
	*cfg = DefaultConfig

	err := yaml.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}

	cfg.original = s
	return cfg, nil
}

// LoadFile parses the given YAML file into a Config.
func LoadFile(logger log.Logger, filename string) (*Config, error) {
	_ = level.Info(logger).Log("file", filename, "msg", "Loading configuration file")
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	cfg, err := Load(string(content))
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// DefaultConfig is the default top-level configuration.
var DefaultConfig = Config{
	Web: webOptions{
		ListenAddress: "0.0.0.0:9201",
		TelemetryPath: "/metrics",
	},
	Read: readOptions{
		Timeout:     5 * time.Minute,
		Delay:       1 * time.Hour,
		IgnoreError: true,
	},
	Write: writeOptions{
		Timeout: 5 * time.Minute,
	},
	Graphite: graphite.DefaultConfig,
}

// Config is the top-level configuration.
type Config struct {
	ConfigFile string
	LogLevel   promlog.AllowedLevel
	Web        webOptions      `yaml:"web,omitempty" json:"web,omitempty"`
	Read       readOptions     `yaml:"read,omitempty" json:"read,omitempty"`
	Write      writeOptions    `yaml:"write,omitempty" json:"write,omitempty"`
	Graphite   graphite.Config `yaml:"graphite,omitempty" json:"graphite,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`

	// original is the input from which the Config was parsed.
	original string
}

func (c Config) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	str := strings.ReplaceAll(string(b), "loglevel: {}", "loglevel: "+c.LogLevel.String())
	return str
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Config

	*c = DefaultConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	return utils.CheckOverflow(c.XXX, "config")
}

type webOptions struct {
	ListenAddress string `yaml:"listen_address,omitempty" json:"listen_address,omitempty"`
	TelemetryPath string `yaml:"telemetry_path,omitempty" json:"telemetry_path,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (opts *webOptions) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain webOptions

	*opts = DefaultConfig.Web
	if err := unmarshal((*plain)(opts)); err != nil {
		return err
	}

	return utils.CheckOverflow(opts.XXX, "webOptions")
}

type readOptions struct {
	Timeout     time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Delay       time.Duration `yaml:"delay,omitempty" json:"delay,omitempty"`
	IgnoreError bool          `yaml:"ignore_error,omitempty" json:"ignore_error,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (opts *readOptions) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain readOptions

	*opts = DefaultConfig.Read
	if err := unmarshal((*plain)(opts)); err != nil {
		return err
	}

	return utils.CheckOverflow(opts.XXX, "readOptions")
}

type writeOptions struct {
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (opts *writeOptions) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain writeOptions

	*opts = DefaultConfig.Write
	if err := unmarshal((*plain)(opts)); err != nil {
		return err
	}

	return utils.CheckOverflow(opts.XXX, "writeOptions")
}
