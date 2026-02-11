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

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestLoadContext(t *testing.T) {
	templateData := map[string]interface{}{
		"key1": "value1",
	}
	m := model.Metric{
		"__name__": "test_metric",
		"label1":   "value1",
	}

	ctx := loadContext(templateData, m)
	assert.Equal(t, "value1", ctx["key1"])
	assert.Contains(t, ctx, "labels")
	labels := ctx["labels"].(map[string]string)
	assert.Equal(t, "test_metric", labels["__name__"])
	assert.Equal(t, "value1", labels["label1"])
}

func TestMatch(t *testing.T) {
	metric := model.Metric{
		"__name__": "test_metric",
		"label1":   "value1",
		"label2":   "value2",
	}

	matchLabels := config.LabelSet{
		"__name__": "test_metric",
		"label1":   "value1",
	}
	matchRE := config.LabelSetRE{}

	assert.True(t, match(metric, matchLabels, matchRE))

	// No match
	matchLabels["label1"] = "wrong"
	assert.False(t, match(metric, matchLabels, matchRE))
}
