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

package graphite

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/paths"
	"github.com/Netcracker/qubership-graphite-remote-adapter/utils"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/model"
	plabels "github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/prompb"
	"golang.org/x/net/context"
)

func (client *Client) QueryToTargets(ctx context.Context, query *prompb.Query, graphitePrefix string) ([]string, error) {
	// Parse metric name from query
	var name string

	for _, m := range query.Matchers {
		if m.Name == model.MetricNameLabel && m.Type == prompb.LabelMatcher_EQ {
			name = m.Value
		}
	}

	if name == "" {
		err := fmt.Errorf("invalid remote query: no %s label provided", model.MetricNameLabel)
		return nil, err
	}

	// Prepare the url to fetch
	queryStr := graphitePrefix + name + ".**"
	expandURL, err := PrepareURL(client.cfg.Read.URL, expandEndpoint, map[string]string{"format": "json", "leavesOnly": "1", "query": queryStr})
	if err != nil {
		_ = level.Warn(client.logger).Log(
			"graphite_web", client.cfg.Read.URL, "path", expandEndpoint,
			"err", err, "msg", "Error preparing URL")
		return nil, err
	}

	// Get the list of targets
	expandResponse := ExpandResponse{}
	body, err := FetchURL(ctx, client.logger, expandURL)
	if err != nil {
		_ = level.Warn(client.logger).Log(
			"url", expandURL, "body", utils.TruncateString(string(body), 140)+"...",
			"err", err, "msg", "Error fetching URL")
		return nil, err
	}

	err = json.Unmarshal(body, &expandResponse)
	if err != nil {
		_ = level.Warn(client.logger).Log(
			"url", expandURL, "body", utils.TruncateString(string(body), 140)+"...",
			"err", err, "msg", "Error parsing expand endpoint response body")
		return nil, err
	}

	targets, err := client.filterTargets(query, expandResponse.Results, graphitePrefix)
	return targets, err
}

func (client *Client) QueryToTargetsWithTags(ctx context.Context, query *prompb.Query, graphitePrefix string) ([]string, error) {
	var tagSet []string

	for _, m := range query.Matchers {
		var name string
		var value string
		if m.Name == model.MetricNameLabel {
			name = "name"
			value = graphitePrefix + m.Value
		} else {
			name = m.Name
			value = m.Value
		}

		switch m.Type {
		case prompb.LabelMatcher_EQ:
			tagSet = append(tagSet, "\""+name+"="+value+"\"")
		case prompb.LabelMatcher_NEQ:
			tagSet = append(tagSet, "\""+name+"!="+value+"\"")
		case prompb.LabelMatcher_RE:
			tagSet = append(tagSet, "\""+name+"=~^("+value+")$\"")
		case prompb.LabelMatcher_NRE:
			tagSet = append(tagSet, "\""+name+"!=~^("+value+")$\"")
		default:
			return nil, fmt.Errorf("unknown match type %v", m.Type)
		}
	}

	targets := []string{"seriesByTag(" + strings.Join(tagSet, ",") + ")"}
	return targets, nil
}

func (client *Client) filterTargets(query *prompb.Query, targets []string, graphitePrefix string) ([]string, error) {
	// Filter out targets that do not match the query's label matcher
	var results []string
	for _, target := range targets {
		// Put labels in a map.
		prompbLabels, err := paths.MetricLabelsFromPath(target, graphitePrefix)
		if err != nil {
			_ = level.Warn(client.logger).Log(
				"path", target, "prefix", graphitePrefix, "err", err)
			continue
		}
		labelMap := make(map[string]string, len(prompbLabels))

		for _, label := range prompbLabels {
			labelMap[label.Name] = label.Value
		}

		_ = level.Debug(client.logger).Log(
			"target", target, "prefix", graphitePrefix,
			"labels", labelMap, "msg", "Filtering target")

		// See if all matchers are satisfied.
		match := true
		for _, m := range query.Matchers {
			matcher, err := plabels.NewMatcher(
				plabels.MatchType(m.Type), m.Name, m.Value)
			if err != nil {
				return nil, err
			}

			if !matcher.Matches(labelMap[m.Name]) {
				match = false
				break
			}
		}

		// If everything is fine, keep this target.
		if match {
			results = append(results, target)
		}
	}
	return results, nil
}

func (client *Client) TargetToTimeseries(ctx context.Context, target string, from string, until string, graphitePrefix string) ([]*prompb.TimeSeries, error) {
	renderURL, err := PrepareURL(client.cfg.Read.URL, renderEndpoint, map[string]string{"format": "json", "from": from, "until": until, "target": target})
	if err != nil {
		_ = level.Warn(client.logger).Log(
			"graphite_web", client.cfg.Read.URL, "path", renderEndpoint,
			"err", err, "msg", "Error preparing URL")
		return nil, err
	}

	renderResponses := make([]RenderResponse, 0)
	body, err := FetchURL(ctx, client.logger, renderURL)
	if err != nil {
		_ = level.Warn(client.logger).Log(
			"url", renderURL, "body", utils.TruncateString(string(body), 140)+"...",
			"err", err, "ctx", ctx, "msg", "Error fetching URL")
		return nil, err
	}

	err = json.Unmarshal(body, &renderResponses)
	if err != nil {
		_ = level.Warn(client.logger).Log(
			"url", renderURL, "body", utils.TruncateString(string(body), 140)+"...",
			"err", err, "msg", "Error parsing render endpoint response body")
		return nil, err
	}

	ret := make([]*prompb.TimeSeries, len(renderResponses))
	for i, renderResponse := range renderResponses {
		ts := &prompb.TimeSeries{}

		if client.cfg.EnableTags {
			ts.Labels, err = paths.MetricLabelsFromTags(renderResponse.Tags, graphitePrefix)
		} else {
			ts.Labels, err = paths.MetricLabelsFromPath(renderResponse.Target, graphitePrefix)
		}

		if err != nil {
			_ = level.Warn(client.logger).Log(
				"path", renderResponse.Target, "prefix", graphitePrefix, "err", err)
			return nil, err
		}

		ts.Samples = samplesFromDatapoints(renderResponse.Datapoints, client.cfg.Read.MaxPointDelta)

		ret[i] = ts
	}
	return ret, nil
}

func samplesFromDatapoints(datapoints []*Datapoint, maxPointDelta time.Duration) []prompb.Sample {
	var samples []prompb.Sample
	for i, datapoint := range datapoints {
		timestampMs := datapoint.Timestamp * 1000
		if datapoint.Value == nil {
			continue
		}
		samples = append(samples, prompb.Sample{
			Value:     *datapoint.Value,
			Timestamp: timestampMs})

		// If not last point and interpolation is enabled,
		// then linearly interpolate intermediate samples.
		if (i+1) < len(datapoints) && maxPointDelta != time.Duration(0) {
			intervalSecond := int64(maxPointDelta.Seconds())
			nextDatapoint := datapoints[i+1]
			if nextDatapoint.Value == nil {
				continue
			}

			deltaSecond := nextDatapoint.Timestamp - datapoint.Timestamp
			variation := (*nextDatapoint.Value - *datapoint.Value) / float64(deltaSecond)

			for j := int64(1); j < deltaSecond/intervalSecond; j++ {
				timestamp := datapoint.Timestamp + j*intervalSecond
				value := *datapoint.Value + float64(timestamp-datapoint.Timestamp)*variation
				samples = append(samples, prompb.Sample{
					Value:     value,
					Timestamp: timestamp * 1000})
			}
		}
	}
	return samples
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (client *Client) handleReadQuery(ctx context.Context, query *prompb.Query, graphitePrefix string) (*prompb.QueryResult, error) {
	queryResult := &prompb.QueryResult{}

	now := int(time.Now().Unix())
	from := int(query.StartTimestampMs / 1000)
	until := int(query.EndTimestampMs / 1000)
	delta := int(client.readDelay.Seconds())
	until = min(now-delta, until)

	if until < from {
		_ = level.Debug(client.logger).Log("msg", "Skipping query with empty time range")
		return queryResult, nil
	}
	fromStr := strconv.Itoa(from)
	untilStr := strconv.Itoa(until)

	var targets []string
	var err error

	if client.cfg.EnableTags {
		targets, err = client.QueryToTargetsWithTags(ctx, query, graphitePrefix)
	} else {
		// If we don't have tags we try to emulate then with normal paths.
		targets, err = client.QueryToTargets(ctx, query, graphitePrefix)
	}
	if err != nil {
		return nil, err
	}

	_ = level.Debug(client.logger).Log(
		"targets", targets, "from", fromStr, "until", untilStr, "msg", "Fetching data")
	client.fetchData(ctx, queryResult, targets, fromStr, untilStr, graphitePrefix)
	return queryResult, nil

}

func (client *Client) fetchData(ctx context.Context, queryResult *prompb.QueryResult, targets []string, fromStr string, untilStr string, graphitePrefix string) {
	input := make(chan string, len(targets))
	output := make(chan *prompb.TimeSeries, len(targets)+1)

	wg := sync.WaitGroup{}

	// TODO: Send multiple targets per query, Graphite supports that.
	// Start only a few workers to avoid killing graphite.
	for i := 0; i < maxFetchWorkers; i++ {
		wg.Add(1)

		go func(fromStr string, untilStr string, ctx context.Context) {
			defer wg.Done()

			for target := range input {
				// We simply ignore errors here as it is better to return "some" data
				// than nothing.
				ts, err := client.TargetToTimeseries(ctx, target, fromStr, untilStr, graphitePrefix)
				if err != nil {
					_ = level.Warn(client.logger).Log("target", target, "err", err, "msg", "Error fetching and parsing target datapoints")
				} else {
					_ = level.Debug(client.logger).Log("reading responses")
					for _, t := range ts {
						output <- t
					}
				}
			}
		}(fromStr, untilStr, ctx)
	}

	// Feed the input.
	for _, target := range targets {
		input <- target
	}
	close(input)

	// Close the output as soon as all jobs are done.
	go func() {
		wg.Wait()
		output <- nil
		close(output)
	}()

	// Read output until channel is closed.
	for {
		done := false
		ts := <-output
		if ts != nil {
			queryResult.Timeseries = append(queryResult.Timeseries, ts)
		} else {
			// A nil result means that we are done.
			done = true
		}
		if done {
			break
		}
	}
}

// Read implements the client.Reader interface.
func (client *Client) Read(req *prompb.ReadRequest, r *http.Request) (*prompb.ReadResponse, error) {
	_ = level.Debug(client.logger).Log("req", req, "msg", "Remote read")

	if client.cfg.Read.URL == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), client.readTimeout)
	defer cancel()

	graphitePrefix := client.cfg.StoragePrefixFromRequest(r)

	resp := &prompb.ReadResponse{}
	for _, query := range req.Queries {
		queryResult, err := client.handleReadQuery(ctx, query, graphitePrefix)
		if err != nil {
			return nil, err
		}
		resp.Results = append(resp.Results, queryResult)
	}
	return resp, nil
}
