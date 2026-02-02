// Copyright 2018 Thibault Chataigner <thibault.chataigner@gmail.com>
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

package paths

import (
	"bytes"
	"errors"
	"math"
	"sort"
	"strconv"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	graphitetmpl "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/template"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/common/model"
)

// ToDatapoints builds points from samples.
func ToDatapoints(s *model.Sample, format Format, prefix string, rules []*config.Rule, templateData map[string]interface{}) ([][]byte, error) {
	t := float64(s.Timestamp.UnixNano()) / 1e9
	v := float64(s.Value)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, errors.New("invalid sample value")
	}

	paths, err := pathsFromMetric(s.Metric, format, prefix, rules, templateData)
	if err != nil {
		return nil, err
	}

	dataPoints := make([][]byte, 0, len(paths))
	//math.MaxInt64 + '.' + 6 precision symbols
	valBuf := make([]byte, 0, 26)
	val := strconv.AppendFloat(valBuf, v, 'f', 6, 64)
	//math.MaxFloat64 + 0 precision symbols
	tmBuf := make([]byte, 0, 309)
	tm := strconv.AppendFloat(tmBuf, t, 'f', 0, 64)
	var length int
	for i := range paths {
		pathLength := len(paths[i])
		if pathLength > length {
			length = pathLength
		}
	}
	buf := bytes.NewBuffer(make([]byte, 0, length+len(val)+len(tm)+3))

	for _, path := range paths {
		buf.Write(path)
		buf.WriteByte(' ')
		buf.Write(val)
		buf.WriteByte(' ')
		buf.Write(tm)
		buf.WriteByte('\n')
		dataPoints = append(dataPoints, buf.Bytes())
		buf.Reset()
	}
	return dataPoints, nil
}

func pathsFromMetric(m model.Metric, format Format, prefix string, rules []*config.Rule, templateData map[string]interface{}) ([][]byte, error) {
	var fingerPrint string
	if pathsCacheEnabled {
		ffp := m.FastFingerprint()
		//math.MaxUint64
		buf := make([]byte, 0, 16)
		fingerPrintBYtes := strconv.AppendUint(buf, uint64(ffp), 16)
		fingerPrint = string(fingerPrintBYtes)
		cachedPaths, cached := pathsCache.Get(fingerPrint)
		if cached {
			return cachedPaths.([][]byte), nil
		}
	}
	paths, stop, err := templatedPaths(m, rules, templateData)
	// if it doesn't match any rule, use default path
	if !stop {
		paths = append(paths, defaultPath(m, format, prefix))
	}
	if pathsCacheEnabled {
		pathsCache.Set(fingerPrint, paths, cache.DefaultExpiration)
	}
	return paths, err
}

func templatedPaths(m model.Metric, rules []*config.Rule, templateData map[string]interface{}) ([][]byte, bool, error) {
	var paths [][]byte
	var stop = false
	var err error
	for _, rule := range rules {
		ruleMatch := match(m, rule.Match, rule.MatchRE)
		if !ruleMatch {
			continue
		}
		// We have a rule to silence this metric
		if !rule.Continue && (rule.Tmpl == config.Template{}) {
			return nil, true, nil
		}

		context := loadContext(templateData, m)
		stop = !rule.Continue
		var path bytes.Buffer
		err = rule.Tmpl.Execute(&path, context)
		if err != nil {
			// We had an error processing the template so we break the loop
			break
		}
		paths = append(paths, path.Bytes())

		if !rule.Continue {
			break
		}
	}
	return paths, stop, err
}

func defaultPath(m model.Metric, format Format, prefix string) []byte {
	var lBufferSize int
	// We want to sort the labels.
	labels := make(model.LabelNames, 0, len(m))
	for l := range m {
		labels = append(labels, l)
		lBufferSize += len(l)
		lBufferSize += len(m[l])
	}
	sort.Sort(labels)

	buf := make([]byte, 0, lBufferSize*2)
	lbuffer := bytes.NewBuffer(buf)

	first := true
	for _, l := range labels {
		if l == model.MetricNameLabel || len(l) == 0 {
			continue
		}

		k := []byte(l)
		v := string(m[l])
		switch format {
		case FormatCarbonOpenMetrics:
			// See https://github.com/RichiH/OpenMetrics/blob/master/metric_exposition_format.md
			if !first {
				lbuffer.WriteByte(',')
			}
			lbuffer.Write(k)
			lbuffer.WriteByte('=')
			lbuffer.WriteByte('"')
			val := graphitetmpl.Escape(v)
			lbuffer.Write(val)
			lbuffer.WriteByte('"')
		case FormatCarbonTags:
			// See http://graphite.readthedocs.io/en/latest/tags.html
			lbuffer.WriteByte(';')
			lbuffer.Write(k)
			lbuffer.WriteByte('=')
			val := graphitetmpl.EscapeTagged(v)
			lbuffer.Write(val)
		default:
			// For each label, in order, add ".<label>.<value>".
			// Since we use '.' instead of '=' to separate label and values
			// it means that we can't have an '.' in the metric name. Fortunately
			// this is prohibited in prometheus metrics.
			lbuffer.WriteByte('.')
			lbuffer.Write(k)
			lbuffer.WriteByte('.')
			val := graphitetmpl.Escape(v)
			lbuffer.Write(val)
		}
		first = false
	}

	metricNameLabel := graphitetmpl.Escape(string(m[model.MetricNameLabel]))
	buf = make([]byte, 0, len(prefix)+len(metricNameLabel)+len(lbuffer.Bytes())+2)
	buffer := bytes.NewBuffer(buf)

	buffer.WriteString(prefix)
	buffer.Write(metricNameLabel)

	if lbuffer.Len() > 0 {
		if format == FormatCarbonOpenMetrics {
			buffer.WriteByte('{')
			buffer.Write(lbuffer.Bytes())
			buffer.WriteByte('}')
		} else {
			buffer.Write(lbuffer.Bytes())
		}
	}
	return buffer.Bytes()
}
