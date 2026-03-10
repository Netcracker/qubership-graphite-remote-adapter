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

package graphite

import (
	"math/rand"
	"net/http/httptest"
	"testing"

	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
)

var (
	namespaces = model.LabelValues{"kafka", "consul", "arangodb", "monitoring", "logging", "cassandra", "zeebe", "postgres", "opensearch",
		"zookeeper", "grafana", "etcd", "jenkins", "pg-patroni-node", "prometheus", "kube-apiserver", "streaming-platform", "logging"}
	lName = model.LabelValues{"container_memory_usage_bytes", "container_memory_working_set_bytes", "container_network_receive_bytes_total", "container_network_receive_errors_total",
		"container_network_receive_packets_dropped_total", "container_network_receive_packets_total", "container_network_transmit_bytes_total", "container_network_transmit_errors_total",
		"container_network_transmit_packets_dropped_total", "container_network_transmit_packets_total", "container_processes", "container_scrape_error", "container_scrape_error",
		"container_sockets", "container_spec_cpu_period", "container_spec_cpu_quota", "container_spec_cpu_shares", "container_spec_memory_limit_bytes", "container_memory_working_set_bytes",
		"container_network_receive_bytes_total"}
)

func BenchmarkTestProcessPrepareWrite1(b *testing.B) {
	benchmarkTestProcessPrepareWrite(b, 1)
}

func BenchmarkTestProcessPrepareWrite10(b *testing.B) {
	benchmarkTestProcessPrepareWrite(b, 10)
}

func BenchmarkTestProcessPrepareWrite50(b *testing.B) {
	benchmarkTestProcessPrepareWrite(b, 50)
}

func BenchmarkTestProcessPrepareWrite100(b *testing.B) {
	benchmarkTestProcessPrepareWrite(b, 100)
}

func BenchmarkTestProcessPrepareWrite250(b *testing.B) {
	benchmarkTestProcessPrepareWrite(b, 250)
}

func BenchmarkTestProcessPrepareWrite500(b *testing.B) {
	benchmarkTestProcessPrepareWrite(b, 500)
}

func BenchmarkTestProcessPrepareWrite1000(b *testing.B) {
	benchmarkTestProcessPrepareWrite(b, 1000)
}

func benchmarkTestProcessPrepareWrite(b *testing.B, n int) {
	var err error
	var response []byte

	b.ReportAllocs()

	samples, bufSize := prepareSamples(n)

	lvl := promslog.NewLevel()
	err = lvl.Set("error")
	assert.Empty(b, err)
	logger := promslog.New(&promslog.Config{Level: lvl, Format: promslog.NewFormat()})
	cfg := &config.DefaultConfig
	cfg.Graphite.Write.CarbonAddress = "127.0.0.1"
	client := NewClient(cfg, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response, err = client.Write(samples, bufSize, httptest.NewRequest("POST", "http://example.com/write", nil), true)
		assert.NotEmpty(b, response)
		assert.Nil(b, err)
	}
}

func prepareSamples(n int) (samples model.Samples, bufSize int) {
	for i := 0; i < n; i++ {
		metric := model.Metric{model.MetricNameLabel: lName[rand.Intn(20)], "cluster": "cluster-kubernetes", "endpoint": "https-metrics",
			"id":    "/systemd/system.slice/kubepods-burstable-podb270ecd5_78cc_4232_9187_1dd035045cb1.slice:cri-containerd:169763650b87dfa9ebb90ee4dc704c57eb38abdd308a930cda1f45aab3bb7e6c",
			"image": "registry:17001/k8s.gcr.io/pause:3.2", "instance": "169763650b87dfa9ebb90ee4dc704c57eb38abdd308a930cda1f45aab3bb7e6c", "namespace": namespaces[rand.Intn(20)],
			"pod": namespaces[rand.Intn(20)], "prometheus": "monitoring/k8s", "prometheus_replica": "prometheus-k8s-0", "service": "kubelet", "team": "test_team"}
		for key, value := range metric {
			bufSize += len(key)
			bufSize += len(value)
		}
		value := model.SampleValue(rand.Float64())
		//math.MaxInt64 + '.' + 6 precision symbols
		bufSize += 26
		tt := model.Now()
		//math.MaxFloat64 + 0 precision symbols
		bufSize += 309
		sample := &model.Sample{
			Metric:    metric,
			Value:     value,
			Timestamp: tt}
		samples = append(samples, sample)
	}
	return
}
