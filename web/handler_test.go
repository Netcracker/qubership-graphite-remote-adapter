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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client"
	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWriter struct {
	name        string
	target      string
	writeFn     func(samples model.Samples, reqBufLen int, r *http.Request, dryRun bool) ([]byte, error)
	shutdowns   int
	lastSamples model.Samples
	lastReqLen  int
	lastDryRun  bool
}

func (w *fakeWriter) Name() string   { return w.name }
func (w *fakeWriter) Target() string { return w.target }
func (w *fakeWriter) String() string { return w.name }
func (w *fakeWriter) Shutdown()      { w.shutdowns++ }

func (w *fakeWriter) Write(samples model.Samples, reqBufLen int, r *http.Request, dryRun bool) ([]byte, error) {
	w.lastSamples = samples
	w.lastReqLen = reqBufLen
	w.lastDryRun = dryRun
	if w.writeFn != nil {
		return w.writeFn(samples, reqBufLen, r, dryRun)
	}
	return []byte("ok"), nil
}

type fakeReader struct {
	name      string
	target    string
	readFn    func(req *prompb.ReadRequest, r *http.Request) (*prompb.ReadResponse, error)
	shutdowns int
	lastReq   *prompb.ReadRequest
}

func (r *fakeReader) Name() string   { return r.name }
func (r *fakeReader) Target() string { return r.target }
func (r *fakeReader) String() string { return r.name }
func (r *fakeReader) Shutdown()      { r.shutdowns++ }

func (r *fakeReader) Read(req *prompb.ReadRequest, httpReq *http.Request) (*prompb.ReadResponse, error) {
	r.lastReq = req
	if r.readFn != nil {
		return r.readFn(req, httpReq)
	}
	return &prompb.ReadResponse{}, nil
}

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Web.ListenAddress = "127.0.0.1:9201"
	cfg.Web.TelemetryPath = "/metrics"
	cfg.Graphite.Write.CarbonAddress = ""
	cfg.Graphite.Read.URL = ""
	return cfg
}

func testHandler() *Handler {
	logger := slog.New(slog.DiscardHandler)
	return New(logger, testConfig())
}

func encodeReadRequest(t *testing.T, req *prompb.ReadRequest) []byte {
	t.Helper()
	data, err := proto.Marshal(req)
	require.NoError(t, err)
	return snappy.Encode(nil, data)
}

func encodeWriteRequest(t *testing.T, req *prompb.WriteRequest) []byte {
	t.Helper()
	data, err := proto.Marshal(req)
	require.NoError(t, err)
	return snappy.Encode(nil, data)
}

func decodeReadResponse(t *testing.T, payload []byte) *prompb.ReadResponse {
	t.Helper()
	decoded, err := snappy.Decode(nil, payload)
	require.NoError(t, err)

	var resp prompb.ReadResponse
	require.NoError(t, proto.Unmarshal(decoded, &resp))
	return &resp
}

func TestHandler_Healthy(t *testing.T) {
	handler := testHandler()

	req := httptest.NewRequest(http.MethodGet, "/-/healthy", nil)
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OK")
	assert.Equal(t, "max-age=31536000; includeSubDomains; preload", w.Header().Get("Strict-Transport-Security"))
}

func TestHandler_Home(t *testing.T) {
	handler := testHandler()
	handler.readers = []client.Reader{&fakeReader{name: "reader-a", target: "graphite://reader"}}
	handler.writers = []client.Writer{&fakeWriter{name: "writer-a", target: "graphite://writer"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Graphite Remote Adapter")
	assert.Contains(t, w.Body.String(), "reader-a")
	assert.Contains(t, w.Body.String(), "writer-a")
}

func TestHandler_Simulation(t *testing.T) {
	handler := testHandler()

	req := httptest.NewRequest(http.MethodGet, "/simulation", nil)
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String())
}

func TestHandler_ReloadSuccess(t *testing.T) {
	handler := testHandler()
	go func() {
		ch := <-handler.Reload()
		ch <- nil
	}()

	req := httptest.NewRequest(http.MethodPost, "/-/reload", nil)
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Config successfully reloaded.")
}

func TestHandler_ReloadFailure(t *testing.T) {
	handler := testHandler()
	go func() {
		ch := <-handler.Reload()
		ch <- errors.New("boom")
	}()

	req := httptest.NewRequest(http.MethodPost, "/-/reload", nil)
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to reload config: boom")
}

func TestHandler_ApplyConfigShutsDownOldClients(t *testing.T) {
	handler := testHandler()
	oldWriter := &fakeWriter{name: "old-writer", target: "old"}
	oldReader := &fakeReader{name: "old-reader", target: "old"}
	handler.writers = []client.Writer{oldWriter}
	handler.readers = []client.Reader{oldReader}

	cfg := testConfig()
	cfg.Graphite.Write.CarbonAddress = "127.0.0.1:2003"

	err := handler.ApplyConfig(cfg)
	require.NoError(t, err)
	assert.Equal(t, 1, oldWriter.shutdowns)
	assert.Equal(t, 1, oldReader.shutdowns)
	assert.Len(t, handler.writers, 1)
	assert.Len(t, handler.readers, 1)
}

func TestHandlerParseTestWriteRequest(t *testing.T) {
	handler := testHandler()
	payload, err := json.Marshal([]*model.Sample{
		{
			Metric: model.Metric{
				model.MetricNameLabel: "cpu_usage",
				"env":                 "prod",
			},
			Value:     model.SampleValue(12.5),
			Timestamp: model.Time(1234),
		},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/write", bytes.NewReader(payload))

	samples, err := handler.parseTestWriteRequest(httptest.NewRecorder(), req)
	require.NoError(t, err)
	require.Len(t, samples, 1)
	assert.Equal(t, model.LabelValue("cpu_usage"), samples[0].Metric[model.MetricNameLabel])
	assert.Equal(t, model.SampleValue(12.5), samples[0].Value)
	assert.Equal(t, model.Time(1234), samples[0].Timestamp)
}

func TestHandlerParseTestWriteRequestReturnsError(t *testing.T) {
	handler := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/write", strings.NewReader(`{`))

	_, err := handler.parseTestWriteRequest(httptest.NewRecorder(), req)

	require.Error(t, err)
}

func TestProtoToSamples(t *testing.T) {
	req := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: string(model.MetricNameLabel), Value: "requests_total"},
					{Name: "job", Value: "adapter"},
				},
				Samples: []prompb.Sample{
					{Value: 1.5, Timestamp: 111},
					{Value: 2.5, Timestamp: 222},
				},
			},
		},
	}

	samples, size := protoToSamples(req)

	require.Len(t, samples, 2)
	assert.Greater(t, size, 0)
	assert.Equal(t, model.LabelValue("requests_total"), samples[0].Metric[model.MetricNameLabel])
	assert.Equal(t, model.LabelValue("adapter"), samples[0].Metric["job"])
	assert.Equal(t, model.SampleValue(2.5), samples[1].Value)
	assert.Equal(t, model.Time(222), samples[1].Timestamp)
}

func TestHandlerWriteDryRunJSON(t *testing.T) {
	handler := testHandler()
	writer := &fakeWriter{
		name:   "writer-a",
		target: "graphite://writer",
		writeFn: func(samples model.Samples, reqBufLen int, r *http.Request, dryRun bool) ([]byte, error) {
			return []byte("dry-run"), nil
		},
	}
	handler.writers = []client.Writer{writer}

	payload, err := json.Marshal([]*model.Sample{
		{
			Metric: model.Metric{
				model.MetricNameLabel: "cpu_usage",
			},
			Value:     model.SampleValue(12.5),
			Timestamp: model.Time(1234),
		},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/write", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, writer.lastDryRun)
	assert.Len(t, writer.lastSamples, 1)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "dry-run", resp["writer-a"])
}

func TestHandlerWriteReturnsBadRequestForInvalidJSON(t *testing.T) {
	handler := testHandler()
	handler.writers = []client.Writer{&fakeWriter{name: "writer-a", target: "graphite://writer"}}

	req := httptest.NewRequest(http.MethodPost, "/write", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerWriteRemoteWriteRequest(t *testing.T) {
	handler := testHandler()
	writer := &fakeWriter{name: "writer-a", target: "graphite://writer"}
	handler.writers = []client.Writer{writer}

	reqPayload := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: string(model.MetricNameLabel), Value: "cpu_usage"},
				},
				Samples: []prompb.Sample{
					{Value: 12.5, Timestamp: 1234},
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/write", bytes.NewReader(encodeWriteRequest(t, reqPayload)))
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, writer.lastDryRun)
	assert.Len(t, writer.lastSamples, 1)
	assert.Greater(t, writer.lastReqLen, 0)
}

func TestInstrumentedWriteSamplesReturnsError(t *testing.T) {
	handler := testHandler()
	writer := &fakeWriter{
		name:   "writer-a",
		target: "graphite://writer",
		writeFn: func(samples model.Samples, reqBufLen int, r *http.Request, dryRun bool) ([]byte, error) {
			return nil, errors.New("write failed")
		},
	}

	msg, err := handler.instrumentedWriteSamples(writer, model.Samples{}, 0, httptest.NewRequest(http.MethodPost, "/write", nil), false)

	require.Error(t, err)
	assert.Nil(t, msg)
}

func TestHandlerReadReturnsBadRequestForInvalidSnappy(t *testing.T) {
	handler := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/read", strings.NewReader("not-snappy"))
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerReadReturnsBadRequestForInvalidProtobuf(t *testing.T) {
	handler := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/read", bytes.NewReader(snappy.Encode(nil, []byte("bad-proto"))))
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerReadReturnsErrorWhenReaderCountIsInvalid(t *testing.T) {
	handler := testHandler()
	handler.readers = nil
	reqPayload := &prompb.ReadRequest{
		Queries: []*prompb.Query{{StartTimestampMs: 1, EndTimestampMs: 2}},
	}

	req := httptest.NewRequest(http.MethodPost, "/read", bytes.NewReader(encodeReadRequest(t, reqPayload)))
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "expected exactly one reader, found 0 readers")
}

func TestHandlerReadReturnsErrorWhenIgnoreDisabled(t *testing.T) {
	handler := testHandler()
	handler.cfg.Read.IgnoreError = false
	reader := &fakeReader{
		name:   "reader-a",
		target: "graphite://reader",
		readFn: func(req *prompb.ReadRequest, r *http.Request) (*prompb.ReadResponse, error) {
			return nil, errors.New("query failed")
		},
	}
	handler.readers = []client.Reader{reader}

	reqPayload := &prompb.ReadRequest{
		Queries: []*prompb.Query{{StartTimestampMs: 1, EndTimestampMs: 2}},
	}
	req := httptest.NewRequest(http.MethodPost, "/read", bytes.NewReader(encodeReadRequest(t, reqPayload)))
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "query failed")
}

func TestHandlerReadReturnsEmptyResponseWhenIgnoreEnabled(t *testing.T) {
	handler := testHandler()
	handler.cfg.Read.IgnoreError = true
	reader := &fakeReader{
		name:   "reader-a",
		target: "graphite://reader",
		readFn: func(req *prompb.ReadRequest, r *http.Request) (*prompb.ReadResponse, error) {
			return nil, errors.New("query failed")
		},
	}
	handler.readers = []client.Reader{reader}

	reqPayload := &prompb.ReadRequest{
		Queries: []*prompb.Query{{StartTimestampMs: 1, EndTimestampMs: 2}},
	}
	req := httptest.NewRequest(http.MethodPost, "/read", bytes.NewReader(encodeReadRequest(t, reqPayload)))
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-protobuf", w.Header().Get("Content-Type"))
	assert.Equal(t, "snappy", w.Header().Get("Content-Encoding"))
	resp := decodeReadResponse(t, w.Body.Bytes())
	require.Len(t, resp.Results, 1)
	assert.Empty(t, resp.Results[0].Timeseries)
}

func TestHandlerReadSuccess(t *testing.T) {
	handler := testHandler()
	reader := &fakeReader{
		name:   "reader-a",
		target: "graphite://reader",
		readFn: func(req *prompb.ReadRequest, r *http.Request) (*prompb.ReadResponse, error) {
			return &prompb.ReadResponse{
				Results: []*prompb.QueryResult{
					{
						Timeseries: []*prompb.TimeSeries{
							{
								Labels: []prompb.Label{
									{Name: string(model.MetricNameLabel), Value: "cpu_usage"},
								},
								Samples: []prompb.Sample{
									{Value: 12.5, Timestamp: 1234},
								},
							},
						},
					},
				},
			}, nil
		},
	}
	handler.readers = []client.Reader{reader}

	reqPayload := &prompb.ReadRequest{
		Queries: []*prompb.Query{{StartTimestampMs: 1, EndTimestampMs: 2}},
	}
	req := httptest.NewRequest(http.MethodPost, "/read", bytes.NewReader(encodeReadRequest(t, reqPayload)))
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/x-protobuf", w.Header().Get("Content-Type"))
	assert.Equal(t, "snappy", w.Header().Get("Content-Encoding"))
	require.NotNil(t, reader.lastReq)
	resp := decodeReadResponse(t, w.Body.Bytes())
	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results[0].Timeseries, 1)
	assert.Equal(t, "cpu_usage", resp.Results[0].Timeseries[0].Labels[0].Value)
	assert.Equal(t, 12.5, resp.Results[0].Timeseries[0].Samples[0].Value)
}

func TestHandlerReadReturnsServerErrorWhenBodyReadFails(t *testing.T) {
	handler := testHandler()
	req := httptest.NewRequest(http.MethodPost, "/read", io.NopCloser(errReader{}))
	w := httptest.NewRecorder()

	handler.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read failed")
}
