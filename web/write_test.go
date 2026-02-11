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

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/go-kit/log"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
)

func TestProtoToSamples(t *testing.T) {
	req := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "test_metric"},
					{Name: "label1", Value: "value1"},
				},
				Samples: []prompb.Sample{
					{Value: 1.5, Timestamp: 1234567890},
				},
			},
		},
	}

	samples, size := protoToSamples(req)
	assert.Len(t, samples, 1)
	assert.Equal(t, model.SampleValue(1.5), samples[0].Value)
	assert.Equal(t, model.LabelValue("test_metric"), samples[0].Metric["__name__"])
	assert.Greater(t, size, 0)
}

func TestHandler_parseTestWriteRequest(t *testing.T) {
	cfg := &config.Config{}
	logger := log.NewNopLogger()
	h := New(logger, cfg)

	samples := []*model.Sample{
		{
			Metric: model.Metric{"__name__": "test"},
			Value:  1.0,
		},
	}

	data, err := json.Marshal(samples)
	assert.NoError(t, err)

	req := httptest.NewRequest("POST", "/write", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	parsedSamples, err := h.parseTestWriteRequest(w, req)
	assert.NoError(t, err)
	assert.Len(t, parsedSamples, 1)
	assert.Equal(t, model.SampleValue(1.0), parsedSamples[0].Value)
}

func TestHandler_parseWriteRequest(t *testing.T) {
	cfg := &config.Config{}
	logger := log.NewNopLogger()
	h := New(logger, cfg)

	writeReq := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "test_metric"},
					{Name: "label", Value: "value"},
				},
				Samples: []prompb.Sample{
					{Value: 2.5, Timestamp: 1609459200000},
				},
			},
		},
	}

	data, err := proto.Marshal(writeReq)
	assert.NoError(t, err)
	compressed := snappy.Encode(nil, data)

	req := httptest.NewRequest("POST", "/write", bytes.NewReader(compressed))
	w := httptest.NewRecorder()

	samples, reqBufLen, err := h.parseWriteRequest(w, req)
	assert.NoError(t, err)
	assert.Len(t, samples, 1)
	assert.Equal(t, model.SampleValue(2.5), samples[0].Value)
	assert.Greater(t, reqBufLen, 0)
}

type mockWriter struct {
	name   string
	target string
}

func (m *mockWriter) Write(samples model.Samples, reqBufLen int, r *http.Request, dryRun bool) ([]byte, error) {
	return []byte("success"), nil
}

func (m *mockWriter) Name() string {
	return m.name
}

func (m *mockWriter) Target() string {
	return m.target
}

func (m *mockWriter) String() string {
	return m.name + ":" + m.target
}

func (m *mockWriter) Shutdown() {}

func TestHandler_instrumentedWriteSamples(t *testing.T) {
	cfg := &config.Config{}
	logger := log.NewNopLogger()
	h := New(logger, cfg)

	writer := &mockWriter{name: "test", target: "localhost:8080"}
	samples := model.Samples{
		&model.Sample{
			Metric: model.Metric{"__name__": "test"},
			Value:  1.0,
		},
	}

	req := httptest.NewRequest("POST", "/write", nil)
	reqBufLen := 100

	msgBytes, err := h.instrumentedWriteSamples(writer, samples, reqBufLen, req, false)
	assert.NoError(t, err)
	assert.Equal(t, []byte("success"), msgBytes)
}
