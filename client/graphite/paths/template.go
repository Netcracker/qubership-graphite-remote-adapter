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
	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/prometheus/common/model"
)

func loadContext(templateData map[string]interface{}, m model.Metric) map[string]interface{} {
	ctx := make(map[string]interface{})
	for k, v := range templateData {
		ctx[k] = v
	}
	labels := make(map[string]string)
	for ln, lv := range m {
		labels[string(ln)] = string(lv)
	}
	ctx["labels"] = labels
	return ctx
}

func match(m model.Metric, match config.LabelSet, matchRE config.LabelSetRE) bool {
	for ln, lv := range match {
		if m[ln] != lv {
			return false
		}
	}
	for ln, r := range matchRE {
		if !r.MatchString(string(m[ln])) {
			return false
		}
	}
	return true
}
