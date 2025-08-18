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
	"os"
	"regexp"
	"testing"
	"text/template"
	"time"

	graphitetmpl "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/template"
	utilstmpl "github.com/Netcracker/qubership-graphite-remote-adapter/utils/template"
	"gopkg.in/yaml.v3"
)

var (
	expectedConf = &Config{
		DefaultPrefix:        "test.prefix.",
		EnableTags:           true,
		UseOpenMetricsFormat: true,
		Read: ReadConfig{
			URL:           "greatGraphiteWebURL",
			MaxPointDelta: 5 * time.Minute,
		},
		Write: WriteConfig{
			CarbonAddress:           "greatCarbonAddress",
			CarbonTransport:         "tcp",
			EnablePathsCache:        true,
			CarbonReconnectInterval: 2 * time.Minute,
			PathsCacheTTL:           18 * time.Minute,
			PathsCachePurgeInterval: 42 * time.Minute,
			TemplateData: map[string]interface{}{
				"site_mapping": map[string]string{"eu-par": "fr_eqx"},
			},
			Rules: []*Rule{
				{
					Match: LabelSet{
						"owner": "team-X",
					},
					MatchRE: LabelSetRE{
						"service": prepareExpectedRegexp("^(foo1|foo2|baz)$"),
					},
					Continue: true,
					Tmpl:     prepareExpectedTemplate("great.graphite.path.host.{{.labels.owner}}.{{.labels.service}}{{if ne .labels.env \"prod\"}}.{{.labels.env}}{{end}}"),
				},
				{
					Match: LabelSet{
						"owner": "team-X",
						"env":   "prod",
					},
					Continue: true,
					Tmpl:     prepareExpectedTemplate("bla.bla.{{.labels.owner | escape}}.great.path"),
				},
				{
					Match: LabelSet{
						"owner": "team-Z",
					},
					Continue: false,
				},
			},
		},
	}

	expectedPlainConf = &Config{
		DefaultPrefix:        "test.prefix.",
		EnableTags:           true,
		UseOpenMetricsFormat: true,
		Read: ReadConfig{
			URL:           "greatGraphiteWebURL",
			MaxPointDelta: 5 * time.Minute,
		},
		Write: WriteConfig{
			CarbonAddress:           "greatCarbonAddress",
			CarbonTransport:         "tcp",
			CompressType:            Plain,
			EnablePathsCache:        true,
			CarbonReconnectInterval: 2 * time.Minute,
			PathsCacheTTL:           18 * time.Minute,
			PathsCachePurgeInterval: 42 * time.Minute,
			TemplateData: map[string]interface{}{
				"site_mapping": map[string]string{"eu-par": "fr_eqx"},
			},
			Rules: []*Rule{
				{
					Match: LabelSet{
						"owner": "team-X",
					},
					MatchRE: LabelSetRE{
						"service": prepareExpectedRegexp("^(foo1|foo2|baz)$"),
					},
					Continue: true,
					Tmpl:     prepareExpectedTemplate("great.graphite.path.host.{{.labels.owner}}.{{.labels.service}}{{if ne .labels.env \"prod\"}}.{{.labels.env}}{{end}}"),
				},
				{
					Match: LabelSet{
						"owner": "team-X",
						"env":   "prod",
					},
					Continue: true,
					Tmpl:     prepareExpectedTemplate("bla.bla.{{.labels.owner | escape}}.great.path"),
				},
				{
					Match: LabelSet{
						"owner": "team-Z",
					},
					Continue: false,
				},
			},
		},
	}

	expectedLZ4Conf = &Config{
		DefaultPrefix:        "test.prefix.",
		EnableTags:           true,
		UseOpenMetricsFormat: true,
		Read: ReadConfig{
			URL:           "greatGraphiteWebURL",
			MaxPointDelta: 5 * time.Minute,
		},
		Write: WriteConfig{
			CarbonAddress:   "greatCarbonAddress",
			CarbonTransport: "tcp",
			CompressType:    LZ4,
			CompressLZ4Preferences: &LZ4Preferences{
				FrameInfo: &LZ4FrameInfo{
					BlockSizeID:         LZ4fBlockSizeMax256kb,
					BlockMode:           true,
					ContentChecksumFlag: true,
					BlockChecksumFlag:   true,
				},
				CompressionLevel:   12,
				AutoFlush:          true,
				DecompressionSpeed: true,
			},
			EnablePathsCache:        true,
			CarbonReconnectInterval: 2 * time.Minute,
			PathsCacheTTL:           18 * time.Minute,
			PathsCachePurgeInterval: 42 * time.Minute,
			TemplateData: map[string]interface{}{
				"site_mapping": map[string]string{"eu-par": "fr_eqx"},
			},
			Rules: []*Rule{
				{
					Match: LabelSet{
						"owner": "team-X",
					},
					MatchRE: LabelSetRE{
						"service": prepareExpectedRegexp("^(foo1|foo2|baz)$"),
					},
					Continue: true,
					Tmpl:     prepareExpectedTemplate("great.graphite.path.host.{{.labels.owner}}.{{.labels.service}}{{if ne .labels.env \"prod\"}}.{{.labels.env}}{{end}}"),
				},
				{
					Match: LabelSet{
						"owner": "team-X",
						"env":   "prod",
					},
					Continue: true,
					Tmpl:     prepareExpectedTemplate("bla.bla.{{.labels.owner | escape}}.great.path"),
				},
				{
					Match: LabelSet{
						"owner": "team-Z",
					},
					Continue: false,
				},
			},
		},
	}
	testConfigFile      = "testdata/graphite.good.yml"
	testConfigPlainFile = "testdata/graphite.good.plain.yml"
	testLZ4ConfigFile   = "testdata/graphite.good.lz4.yml"
)

func prepareExpectedRegexp(s string) Regexp {
	r, _ := regexp.Compile("^(?:" + s + ")$")
	return Regexp{r}
}

func prepareExpectedTemplate(s string) Template {
	t, _ := template.New("").Funcs(utilstmpl.TmplFuncMap).Funcs(graphitetmpl.TmplFuncMap).Parse(s)
	return Template{t, s}
}

func TestUnmarshalConfig(t *testing.T) {
	cfg := &Config{}
	content, _ := os.ReadFile(testConfigFile)
	err := yaml.Unmarshal(content, cfg)
	if err != nil {
		t.Fatalf("Error parsing %s: %s", "testdata/graphite.good.yml", err)
	}

	if cfg.String() != expectedConf.String() {
		t.Fatalf("%s: unexpected config result: \n%s\nExpecting:\n%s",
			"testdata/graphite.good.yml", cfg.String(), expectedConf.String())
	}
}

func TestUnmarshalPlainConfig(t *testing.T) {
	cfg := &Config{}
	content, _ := os.ReadFile(testConfigPlainFile)
	err := yaml.Unmarshal(content, cfg)
	if err != nil {
		t.Fatalf("Error parsing %s: %s", "testdata/graphite.good.plain.yml", err)
	}

	if cfg.String() != expectedPlainConf.String() {
		t.Fatalf("%s: unexpected config result: \n%s\nExpecting:\n%s",
			"testdata/graphite.good.plain.yml", cfg.String(), expectedConf.String())
	}
}

func TestUnmarshalLZ4Config(t *testing.T) {
	cfg := &Config{}
	content, _ := os.ReadFile(testLZ4ConfigFile)
	err := yaml.Unmarshal(content, cfg)
	if err != nil {
		t.Fatalf("Error parsing %s: %s", "testdata/graphite.good.lz4.yml", err)
	}

	if cfg.String() != expectedLZ4Conf.String() {
		t.Fatalf("%s: unexpected config result: \n%s\nExpecting:\n%s",
			"testdata/graphite.good.lz4.yml", cfg.String(), expectedConf.String())
	}
}
