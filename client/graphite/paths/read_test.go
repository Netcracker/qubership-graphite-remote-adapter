// Copyright 2018 Thibault Chataigner <thibault.chataigner@gmail.com>
// Copyright NetCracker Technology Corporation
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
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
)

func TestMetricLabelsFromPath(t *testing.T) {
	path := "prometheus-prefix.test.owner.team-X"
	prefix := "prometheus-prefix"
	expectedLabels := []prompb.Label{
		{Name: model.MetricNameLabel, Value: "test"},
		{Name: "owner", Value: "team-X"},
	}
	actualLabels, _ := MetricLabelsFromPath(path, prefix)
	require.Equal(t, expectedLabels, actualLabels)
}
func TestMetricLabelsFromSpecialPath(t *testing.T) {
	path := "prometheus-prefix.test.owner.team-Y.interface.Hu0%2F0%2F1%2F3%2E99"
	prefix := "prometheus-prefix"
	expectedLabels := []prompb.Label{
		{Name: model.MetricNameLabel, Value: "test"},
		{Name: "owner", Value: "team-Y"},
		{Name: "interface", Value: "Hu0/0/1/3.99"},
	}
	actualLabels, _ := MetricLabelsFromPath(path, prefix)
	require.Equal(t, expectedLabels, actualLabels)
}

func TestMetricLabelsFromTags(t *testing.T) {
	tags := map[string]string{
		"name":   "prefix.metric",
		"owner":  "team-X",
		"label1": "value1",
	}
	prefix := "prefix."
	expectedLabels := []prompb.Label{
		{Name: "label1", Value: "value1"},
		{Name: model.MetricNameLabel, Value: "metric"},
		{Name: "owner", Value: "team-X"},
	}
	actualLabels, err := MetricLabelsFromTags(tags, prefix)
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}
