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

package web

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
)

var (
	receivedSamples = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "received_samples_total",
			Help:      "Total number of received samples.",
		},
		[]string{"prefix"},
	)
	sentSamples = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sent_samples_total",
			Help:      "Total number of processed samples sent to remote storage.",
		},
		[]string{"prefix", "remote"},
	)
	failedSamples = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "failed_samples_total",
			Help:      "Total number of processed samples which failed on send to remote storage.",
		},
		[]string{"prefix", "remote"},
	)
	sentBatchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "sent_batch_duration_seconds",
			Help:      "Duration of sample batch send calls to the remote storage.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"remote"},
	)
)

func (h *Handler) write(w http.ResponseWriter, r *http.Request) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	h.logger.Debug("Handling /write request", "remote", r.RemoteAddr, "method", r.Method, "url", r.URL)

	// As default, we expected snappy encoded protobuf.
	// But for simulation purpose we also accept json.
	dryRun := false
	if ct := r.Header.Get("Content-Type"); ct == "application/json" {
		dryRun = true
	}

	// Parse samples from request.
	var samples model.Samples
	var err error
	var reqBufLen int
	if dryRun {
		samples, err = h.parseTestWriteRequest(w, r)
	} else {
		samples, reqBufLen, err = h.parseWriteRequest(w, r)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	prefix := h.cfg.Graphite.StoragePrefixFromRequest(r)

	receivedSamples.WithLabelValues(prefix).Add(float64(len(samples)))

	// Execute write on each writer clients.
	var wg sync.WaitGroup
	writeResponse := make(map[string]string)
	var msgBytes []byte
	for _, writer := range h.writers {
		wg.Add(1)
		go func(client client.Writer) {
			msgBytes, err = h.instrumentedWriteSamples(client, samples, reqBufLen, r, dryRun)
			if err != nil {
				failedSamples.WithLabelValues(prefix, client.Target()).Add(float64(len(samples)))
				writeResponse[client.Name()] = err.Error()
			} else {
				sentSamples.WithLabelValues(prefix, client.Target()).Add(float64(len(samples)))
				writeResponse[client.Name()] = string(msgBytes)
			}
			wg.Done()
		}(writer)
	}
	wg.Wait()

	// Write response body.
	data, err := json.Marshal(writeResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(data)
}

func (h *Handler) parseTestWriteRequest(w http.ResponseWriter, r *http.Request) (model.Samples, error) {
	decoder := json.NewDecoder(r.Body)
	var samples []*model.Sample
	err := decoder.Decode(&samples)
	if err != nil {
		return nil, err
	}
	return samples, nil
}

func (h *Handler) parseWriteRequest(w http.ResponseWriter, r *http.Request) (model.Samples, int, error) {
	req, err := remote.DecodeWriteRequest(r.Body)
	if err != nil {
		h.logger.Error("Error decoding remote write request", "err", err.Error())
		return nil, 0, err
	}

	samples, sSize := protoToSamples(req)

	return samples, sSize, nil
}

func protoToSamples(req *prompb.WriteRequest) (samples model.Samples, sSize int) {
	for _, ts := range req.Timeseries {
		metric := make(model.Metric, len(ts.Labels))
		for _, l := range ts.Labels {
			metric[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		}

		for _, s := range ts.Samples {
			samples = append(samples, &model.Sample{
				Metric:    metric,
				Value:     model.SampleValue(s.Value),
				Timestamp: model.Time(s.Timestamp),
			})
		}
		sSize += ts.Size()
	}
	return
}

func (h *Handler) instrumentedWriteSamples(
	w client.Writer, samples model.Samples, reqBufLen int, r *http.Request, dryRun bool) ([]byte, error) {

	begin := time.Now()
	msgBytes, err := w.Write(samples, reqBufLen, r, dryRun)
	duration := time.Since(begin).Seconds()
	if err != nil {
		h.logger.Warn("Error sending samples to remote storage", "num_samples", len(samples), "storage", w.Name(), "err", err)
		return nil, err
	}
	sentBatchDuration.WithLabelValues(w.Target()).Observe(duration)
	return msgBytes, nil
}
