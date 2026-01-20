// Copyright 2018 Thibault Chataigner <thibault.chataigner@gmail.com>
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

package paths

import (
	"math"
	"testing"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var (
	metric = model.Metric{
		model.MetricNameLabel: "test:metric",
		"testlabel":           "test:value",
		"owner":               "team-X",
		"many_chars":          "abc!ABC:012-3!45ö67~89./(){},=.\"\\",
	}
	metricY = model.Metric{
		model.MetricNameLabel: "test:metric",
		"testlabel":           "test:value",
		"owner":               "team-Y",
		"many_chars":          "abc!ABC:012-3!45ö67~89./(){},=.\"\\",
	}

	// editorconfig-checker-disable used because next lines are part of the template
	testConfigStr = `
write:
  template_data:
    shared: data.foo
  rules:
  - match:
      owner: team-X
    match_re:
      testlabel: ^test:.*$
    template: 'tmpl_1.{{.shared | escape}}.{{.labels.owner}}'
    continue: true
  - match:
      owner: team-X
      testlabel2:   test:value2
    template: 'tmpl_2.{{.labels.owner}}.{{.shared}}'
    continue: false
  - match:
      owner: team-Y
    template: 'tmpl_3.{{.labels.owner}}.{{.shared}}'
    continue: false
  - match:
      owner: team-Z
    continue: false`

	// editorconfig-checker-enable
	testConfig = loadTestConfig(testConfigStr)
)

func loadTestConfig(s string) *config.Config {
	cfg := &config.Config{}
	if err := yaml.Unmarshal([]byte(s), cfg); err != nil {
		return nil
	}
	return cfg
}

func TestDefaultPathsFromMetric(t *testing.T) {
	expected := "prefix." +
		"test:metric" +
		".many_chars.abc!ABC:012-3!45%C3%B667~89%2E%2F\\(\\)\\{\\}\\,%3D%2E\\\"\\\\" +
		".owner.team-X" +
		".testlabel.test:value"
	actual, err := pathsFromMetric(metric, FormatCarbon, "prefix.", nil, nil)
	require.Equal(t, expected, string(actual[0]))
	require.Empty(t, err)

	// This expected result is different from other expect expressions in this test because for work with
	// Graphite + ClickHouse + Prometheus datasource was added new EscapeTagged into
	// ./client/graphite/template/escape.go. This new method change escape behavior for format "Carbon Tags"
	expected = "prefix." +
		"test:metric" +
		".many_chars.abc!ABC:012-3!45%C3%B667~89%2E%2F\\(\\)\\{\\}\\,%3D%2E\\\"\\\\" +
		".owner.team-X" +
		".testlabel.test:value"

	actual, err = pathsFromMetric(metric, FormatCarbonTags, "prefix.", nil, nil)
	require.Equal(t, expected, string(actual[0]))
	require.Empty(t, err)

	expected = "prefix." +
		"test:metric" +
		".many_chars.abc!ABC:012-3!45%C3%B667~89%2E%2F\\(\\)\\{\\}\\,%3D%2E\\\"\\\\" +
		".owner.team-X" +
		".testlabel.test:value"
	actual, err = pathsFromMetric(metric, FormatCarbonOpenMetrics, "prefix.", nil, nil)
	require.Equal(t, expected, string(actual[0]))
	require.Empty(t, err)
}

func TestUnmatchedMetricPathsFromMetric(t *testing.T) {
	unmatchedMetric := model.Metric{
		model.MetricNameLabel: "test:metric",
		"testlabel":           "test:value",
		"owner":               "team-K",
		"testlabel2":          "test:value2",
	}
	expected := make([][]byte, 0)
	expected = append(expected, []byte("prefix."+
		"test:metric"+
		".owner.team-K"+
		".testlabel.test:value"+
		".testlabel2.test:value2"))
	actual, err := pathsFromMetric(unmatchedMetric, FormatCarbon, "prefix.", testConfig.Write.Rules, testConfig.Write.TemplateData)
	require.Equal(t, expected, actual)
	require.Empty(t, err)
}

func TestTemplatedPathsFromMetric(t *testing.T) {
	expected := make([][]byte, 0)
	expected = append(expected, []byte("tmpl_3.team-Y.data.foo"))
	actual, err := pathsFromMetric(metricY, FormatCarbon, "", testConfig.Write.Rules, testConfig.Write.TemplateData)
	require.Equal(t, expected, actual)
	require.Empty(t, err)
}

func TestTemplatedPathsFromMetricWithDefault(t *testing.T) {
	expected := make([][]byte, 0)
	expected = append(expected, []byte("prefix."+
		"test:metric"+
		".many_chars.abc!ABC:012-3!45%C3%B667~89%2E%2F\\(\\)\\{\\}\\,%3D%2E\\\"\\\\"+
		".owner.team-X"+
		".testlabel.test:value"))
	actual, err := pathsFromMetric(metric, FormatCarbon, "prefix.", testConfig.Write.Rules, testConfig.Write.TemplateData)
	require.Equal(t, expected, actual)
	require.Empty(t, err)
}

func TestMultiTemplatedPathsFromMetric(t *testing.T) {
	multiMatchMetric := model.Metric{
		model.MetricNameLabel: "test:metric",
		"testlabel":           "test:value",
		"owner":               "team-X",
		"testlabel2":          "test:value2",
	}
	expected := make([][]byte, 0)
	expected = append(expected, []byte("tmpl_1.data%2Efoo.team-X"))
	expected = append(expected, []byte("tmpl_2.team-X.data.foo"))
	actual, err := pathsFromMetric(multiMatchMetric, FormatCarbon, "prefix.", testConfig.Write.Rules, testConfig.Write.TemplateData)
	require.Equal(t, expected, actual)
	require.Empty(t, err)
}

func TestSkipedTemplatedPathsFromMetric(t *testing.T) {
	skipedMetric := model.Metric{
		model.MetricNameLabel: "test:metric",
		"testlabel":           "test:value",
		"owner":               "team-Z",
		"testlabel2":          "test:value2",
	}
	t.Log(testConfig.Write.Rules[2])
	actual, err := pathsFromMetric(skipedMetric, FormatCarbon, "", testConfig.Write.Rules, testConfig.Write.TemplateData)
	require.Empty(t, actual)
	require.Empty(t, err)
}

func TestReplaceNilLabelTemplatedPathsFromMetric(t *testing.T) {
	// editorconfig-checker-disable used because next lines are part of the template
	testConfigNilLabelStr := `
write:
  rules:
  - match_re:
      testlabel: test:value
    template: 'test.{{ replace .labels.doesnotexist " " "_" }}'
    continue: false`
	// editorconfig-checker-enable
	testConfigNilLabel := loadTestConfig(testConfigNilLabelStr)

	t.Log(testConfigNilLabel.Write.Rules[0])
	actual, err := pathsFromMetric(metric, FormatCarbon, "", testConfigNilLabel.Write.Rules, testConfigNilLabel.Write.TemplateData)
	require.Len(t, actual, 1)
	require.Empty(t, err)
}

func TestToDatapoints(t *testing.T) {
	sample := &model.Sample{
		Metric:    metric,
		Value:     123.456,
		Timestamp: model.Time(1609459200 * 1000), // 2021-01-01 00:00:00 UTC
	}

	dataPoints, err := ToDatapoints(sample, FormatCarbon, "prefix.", nil, nil)
	require.NoError(t, err)
	require.Len(t, dataPoints, 1)
	expected := "prefix.test:metric.many_chars.abc!ABC:012-3!45%C3%B667~89%2E%2F\\(\\)\\{\\}\\,%3D%2E\\\"\\\\.owner.team-X.testlabel.test:value 123.456000 1609459200\n"
	require.Equal(t, expected, string(dataPoints[0]))
}

func TestToDatapointsInvalidValue(t *testing.T) {
	sample := &model.Sample{
		Metric:    metric,
		Value:     model.SampleValue(math.NaN()),
		Timestamp: model.Time(1609459200 * 1000),
	}

	_, err := ToDatapoints(sample, FormatCarbon, "prefix.", nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid sample value")
}
