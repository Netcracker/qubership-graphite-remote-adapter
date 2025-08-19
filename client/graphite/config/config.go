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
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"text/template"
	"time"

	graphitetmpl "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/template"
	"github.com/Netcracker/qubership-graphite-remote-adapter/utils"
	utilstmpl "github.com/Netcracker/qubership-graphite-remote-adapter/utils/template"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"
)

const (
	LZ4fBlockSizeDefault    LZ4FBlockSize = "default"
	LZ4fBlockSizeMax64kb    LZ4FBlockSize = "max64KB"
	LZ4fBlockSizeMax256kb   LZ4FBlockSize = "max256KB"
	LZ4fBlockSizeMax1mb     LZ4FBlockSize = "max1MB"
	LZ4fBlockSizeMax4mb     LZ4FBlockSize = "max4MB"
	LZ4                     CompressType  = "lz4"
	Plain                   CompressType  = "plain"
	LZ4CompressLevelDefault               = 9
)

type CompressType string
type LZ4FBlockSize string

func (ct *CompressType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type compressionTypeDef CompressType
	ctDef := (*compressionTypeDef)(ct)
	if err := unmarshal(&ctDef); err != nil {
		return err
	}
	ctVal := CompressType(*ctDef)
	switch ctVal {
	case LZ4, Plain:
		*ct = ctVal
	default:
		*ct = Plain
	}
	return nil
}

func (bs *LZ4FBlockSize) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type compressionBlockSize LZ4FBlockSize
	ctDef := (*compressionBlockSize)(bs)
	if err := unmarshal(&ctDef); err != nil {
		return err
	}
	ctVal := LZ4FBlockSize(*ctDef)
	switch ctVal {
	case LZ4fBlockSizeDefault, LZ4fBlockSizeMax64kb, LZ4fBlockSizeMax256kb,
		LZ4fBlockSizeMax1mb, LZ4fBlockSizeMax4mb:
		*bs = ctVal
	default:
		*bs = LZ4fBlockSizeDefault
	}
	return nil
}

// DefaultConfig is the default graphite configuration.
var DefaultConfig = Config{
	DefaultPrefix:        "",
	EnableTags:           false,
	UseOpenMetricsFormat: false,
	Write: WriteConfig{
		CarbonAddress:           "",
		CarbonTransport:         "tcp",
		CarbonReconnectInterval: 1 * time.Hour,
		EnablePathsCache:        true,
		PathsCacheTTL:           7 * time.Minute,
		PathsCachePurgeInterval: 8 * time.Minute,
	},
	Read: ReadConfig{
		URL:           "",
		MaxPointDelta: time.Duration(0),
	},
}

// Config is the graphite configuration.
type Config struct {
	Write                WriteConfig `yaml:"write,omitempty" json:"write,omitempty"`
	Read                 ReadConfig  `yaml:"read,omitempty" json:"read,omitempty"`
	DefaultPrefix        string      `yaml:"default_prefix,omitempty" json:"default_prefix,omitempty"`
	EnableTags           bool        `yaml:"enable_tags,omitempty" json:"enable_tags,omitempty"`
	UseOpenMetricsFormat bool        `yaml:"openmetrics,omitempty" json:"openmetrics,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

func (c Config) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating config string: %s>", err)
	}
	return string(b)
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig
	type plain Config
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	return utils.CheckOverflow(c.XXX, "graphite config")
}

// StoragePrefixFromRequest returns the prefix from either the config or the request's Query
func (c *Config) StoragePrefixFromRequest(r *http.Request) string {
	p := r.URL.Query().Get("graphite.default-prefix")
	if p == "" {
		p = c.DefaultPrefix
	}
	return p
}

// ReadConfig is the read graphite configuration.
type ReadConfig struct {
	URL string `yaml:"url,omitempty" json:"url,omitempty"`
	// If set, MaxPointDelta is used to linearly interpolate intermediate points.
	// It helps support prom1.x reading metrics with larger retention than staleness delta.
	MaxPointDelta time.Duration `yaml:"max_point_delta,omitempty" json:"max_point_delta,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *ReadConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ReadConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return utils.CheckOverflow(c.XXX, "readConfig")
}

// WriteConfig is the write graphite configuration.
type WriteConfig struct {
	CarbonAddress           string                 `yaml:"carbon_address,omitempty" json:"carbon_address,omitempty"`
	CarbonTransport         string                 `yaml:"carbon_transport,omitempty" json:"carbon_transport,omitempty"`
	CompressType            CompressType           `yaml:"compress_type,omitempty" json:"compress_type,omitempty"`
	CompressLZ4Preferences  *LZ4Preferences        `yaml:"lz4_preferences,omitempty" json:"lz4_preferences,omitempty"`
	CarbonReconnectInterval time.Duration          `yaml:"carbon_reconnect_interval,omitempty" json:"carbon_reconnect_interval,omitempty"`
	EnablePathsCache        bool                   `yaml:"enable_paths_cache,omitempty" json:"enable_paths_cache,omitempty"`
	PathsCacheTTL           time.Duration          `yaml:"paths_cache_ttl,omitempty" json:"paths_cache_ttl,omitempty"`
	PathsCachePurgeInterval time.Duration          `yaml:"paths_cache_purge_interval,omitempty" json:"paths_cache_purge_interval,omitempty"`
	TemplateData            map[string]interface{} `yaml:"template_data,omitempty" json:"template_data,omitempty"`
	Rules                   []*Rule                `yaml:"rules,omitempty" json:"rules,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// LZ4FrameInfo makes it possible to set or read frame parameters.
type LZ4FrameInfo struct {
	// The larger the block size, the (slightly) better the compression ratio.
	// Larger blocks also increase memory usage on both compression and decompression sides.
	// Supported values: max64KB, max256KB, max1MB, max4MB. Default: max64KB.
	BlockSizeID LZ4FBlockSize `yaml:"block_size,omitempty" json:"block_size,omitempty"`
	// Linked blocks sharply reduce inefficiencies when using small blocks, they compress better.
	BlockMode bool `yaml:"block_mode,omitempty" json:"block_mode,omitempty"`
	// Add a 32-bit checksum of frame's decompressed data. Default - false, i.e. disabled.
	ContentChecksumFlag bool `yaml:"content_checksum,omitempty" json:"content_checksum,omitempty"`
	// Each block followed by a checksum of block's compressed data. Default - false, i.e. disabled.
	BlockChecksumFlag bool `yaml:"block_checksum,omitempty" json:"block_checksum,omitempty"`
}

// LZ4Preferences contains parameters for lz4 streaming compression.
type LZ4Preferences struct {
	FrameInfo *LZ4FrameInfo `yaml:"frame,omitempty" json:"frame,omitempty"`
	// min value 3, max 12, default 9
	CompressionLevel int `yaml:"compression_level,omitempty" json:"compression_level,omitempty"`
	// always flush; reduces usage of internal buffers. Default - false
	AutoFlush bool `yaml:"auto_flush,omitempty" json:"auto_flush,omitempty"`
	// parser favors decompression speed vs compression ratio. Works for high compression modes (compression_level >= 10) only.
	DecompressionSpeed bool `yaml:"decompression_speed,omitempty" json:"decompression_speed,omitempty"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *WriteConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain WriteConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	return utils.CheckOverflow(c.XXX, "writeConfig")
}

// LabelSet pairs a LabelName to a LabelValue.
type LabelSet map[model.LabelName]model.LabelValue

// LabelSetRE defines pairs like LabelSet but does regular expression
type LabelSetRE map[model.LabelName]Regexp

// Rule defines a templating rule that customize graphite path using the
// Tmpl if a metric matching the labels exists.
type Rule struct {
	Tmpl     Template   `yaml:"template,omitempty" json:"template,omitempty"`
	Match    LabelSet   `yaml:"match,omitempty" json:"match,omitempty"`
	MatchRE  LabelSetRE `yaml:"match_re,omitempty" json:"match_re,omitempty"`
	Continue bool       `yaml:"continue,omitempty" json:"continue,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (r *Rule) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Rule
	if err := unmarshal((*plain)(r)); err != nil {
		return err
	}

	return utils.CheckOverflow(r.XXX, "rule")
}

// Template is a parsable template.
type Template struct {
	*template.Template
	original string
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (tmpl *Template) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	templ, err := template.New("").Funcs(utilstmpl.TmplFuncMap).Funcs(graphitetmpl.TmplFuncMap).Parse(s)
	if err != nil {
		return err
	}
	tmpl.Template = templ
	tmpl.original = s
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (tmpl Template) MarshalYAML() (interface{}, error) {
	return tmpl.original, nil
}

// Regexp encapsulates a regexp.Regexp and makes it YAML marshalable.
type Regexp struct {
	*regexp.Regexp
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (re *Regexp) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	regex, err := regexp.Compile("^(?:" + s + ")$")
	if err != nil {
		return err
	}
	re.Regexp = regex
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface.
func (re Regexp) MarshalYAML() (interface{}, error) {
	return re.String(), nil
}

// MarshalJSON implements the json.Marshaler interface.
func (re Regexp) MarshalJSON() ([]byte, error) {
	if re.Regexp != nil {
		return json.Marshal(re.String())
	}
	return nil, nil
}
