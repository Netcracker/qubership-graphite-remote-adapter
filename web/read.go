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
	"fmt"
	"io"
	"net/http"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/prometheus/prompb"
)

var (
	readSamples = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "read_samples_total",
			Help:      "Total number of samples read from remote storage.",
		},
		[]string{"prefix", "remote"},
	)
	failedReads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "failed_reads_total",
			Help:      "Total number of reads which failed on the remote storage.",
		},
		[]string{"prefix", "remote"},
	)
)

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	h.logger.Debug("Handling /read request", "request", r)
	compressed, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Warn("Error reading request body", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reqBuf, err := snappy.Decode(nil, compressed)
	if err != nil {
		h.logger.Warn("Error decoding request body", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req prompb.ReadRequest
	if err = proto.Unmarshal(reqBuf, &req); err != nil {
		h.logger.Warn("Error unmarshalling protobuf", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: Support reading from more than one reader and merging the results.
	if len(h.readers) != 1 {
		http.Error(w, fmt.Sprintf("expected exactly one reader, found %d readers", len(h.readers)), http.StatusInternalServerError)
		return
	}
	reader := h.readers[0]
	prefix := h.cfg.Graphite.StoragePrefixFromRequest(r)

	var resp *prompb.ReadResponse
	resp, err = reader.Read(&req, r)
	if err != nil {
		h.logger.Warn("Error executing query", "query", req, "storage", reader.Name(), "err", err)
		failedReads.WithLabelValues(prefix, reader.Target()).Inc()
		if !h.cfg.Read.IgnoreError {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if resp == nil {
		resp = &prompb.ReadResponse{
			Results: []*prompb.QueryResult{
				{Timeseries: make([]*prompb.TimeSeries, 0)},
			},
		}
	} else {
		readSamples.WithLabelValues(prefix, reader.Target()).Add(float64(resp.Size()))
	}

	data, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Content-Encoding", "snappy")

	compressed = snappy.Encode(nil, data)
	if _, err := w.Write(compressed); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
