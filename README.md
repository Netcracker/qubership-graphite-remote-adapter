# Qubership Graphite Remote Storage Adapter

[![Super-Linter](https://github.com/Netcracker/qubership-graphite-remote-adapter/actions/workflows/super-linter.yaml/badge.svg)](https://github.com/marketplace/actions/super-linter)

This is a read/write adapter that receives samples via Prometheus's remote write
protocol and stores them in remote storage like Graphite.

Initially it was a fork of [graphite_remote_adapter](https://github.com/criteo/graphite-remote-adapter).

But after that, we made a lot of changes to its source code.

## Differences from initial graphite-remote-adapter

* Added LZ4 compression for Graphite protocol
* Added performance fixes to improve performance
* Added escaping symbols `;`, `~`, ' ' (space), `=` of label values to correct work with Graphite + ClickHouse stack
* Added Helm chart to deploy this adapter in Kubernetes
* Added Grafana dashboard to visualize self-metrics

## Throughput

To reach 35k rps it is necessary at least:

* 2 core
* 2.5 GB
* Handle write request duration on graphite carbon side less than 1 sec

## Metric routing

To route metrics to graphite-remote-adapter change platform monitoring CRD(Custom Resource Definition) fields:

* externalLabels
* remoteWrite

The externalLabels to add to any time series or alerts when
communicating with external systems (federation, remote
storage, Alertmanager).

Example:

```yaml
externalLabels:
  cluster: test-cluster
  environment: test-environment
  project: development
  team: test_team`
```

RemoteWriteSpec defines the remote_write configuration
for prometheus. The `remote_write` allows transparently
send samples to a long term storage.

Example:

```yaml
remoteWrite:
  - tlsConfig:
      insecureSkipVerify: true
    url: >-
      https://url_for_remote_write/write
```

### LZ4 compression

LZ4 streaming compression can be turned on in configuration.

Example:

```yaml
additionalGraphiteConfig:
  graphite:
    write:
      compress_type: lz4
      lz4_preferences:
        frame:
          block_size: max256KB
          block_mode: false
          content_checksum: false
          block_checksum: false
        compression_level: 12
        auto_flush: false
        decompression_speed: false
```

Parameters:

* `compress_type` field support `plain`, `lz4` and empty (means `plain`) values.
* `lz4_preferences` contains parameters for lz4 streaming compression.
* `frame` - lz4 frame info.
* `frame.block_size` - the larger the block size, the (slightly) better the compression ratio.
  Larger blocks also increase memory usage on both compression and decompression sides.
  Supported values: max64KB, max256KB, max1MB, max4MB. Default: max64KB.
* `frame.block_mode` - linked blocks sharply reduce inefficiencies when using small blocks, they compress better.
  However, some LZ4 decoders are only compatible with independent blocks. Default - false, i.e. blocks are linked.
* `frame.content_checksum` - add a 32-bit checksum of frame's decompressed data. Default - false, i.e. disabled.
* `frame.block_checksum` -  each block followed by a checksum of block's compressed data. Default - false, i.e. disabled.
* `compression_level` - min value 3, max 12, default 9
* `auto_flush` - always flush; reduces usage of internal buffers. Default - `false`
* `decompression_speed` - parser favors decompression speed vs compression ratio.
  Works for high compression modes (compression_level >= 10) only.

## Metrics list

```prometheus
# HELP go_gc_duration_seconds A summary of the pause duration of garbage collection cycles.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 1.5378e-05
go_gc_duration_seconds{quantile="0.25"} 2.4186e-05
go_gc_duration_seconds{quantile="0.5"} 3.5375e-05
go_gc_duration_seconds{quantile="0.75"} 5.7691e-05
go_gc_duration_seconds{quantile="1"} 0.004353922
go_gc_duration_seconds_sum 0.68606493
go_gc_duration_seconds_count 474
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 11
# HELP go_info Information about the Go environment.
# TYPE go_info gauge
go_info{version="go1.13.8"} 1
# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.
# TYPE go_memstats_alloc_bytes gauge
go_memstats_alloc_bytes 4.3156784e+07
# HELP go_memstats_alloc_bytes_total Total number of bytes allocated, even if freed.
# TYPE go_memstats_alloc_bytes_total counter
go_memstats_alloc_bytes_total 3.6797211216e+10
# HELP go_memstats_buck_hash_sys_bytes Number of bytes used by the profiling bucket hash table.
# TYPE go_memstats_buck_hash_sys_bytes gauge
go_memstats_buck_hash_sys_bytes 1.559402e+06
# HELP go_memstats_frees_total Total number of frees.
# TYPE go_memstats_frees_total counter
go_memstats_frees_total 4.63408148e+08
# HELP go_memstats_gc_cpu_fraction The fraction of this program's available CPU time used by the GC since the program started.
# TYPE go_memstats_gc_cpu_fraction gauge
go_memstats_gc_cpu_fraction 0.025775199892331825
# HELP go_memstats_gc_sys_bytes Number of bytes used for garbage collection system metadata.
# TYPE go_memstats_gc_sys_bytes gauge
go_memstats_gc_sys_bytes 2.413568e+07
# HELP go_memstats_heap_alloc_bytes Number of heap bytes allocated and still in use.
# TYPE go_memstats_heap_alloc_bytes gauge
go_memstats_heap_alloc_bytes 4.3156784e+07
# HELP go_memstats_heap_idle_bytes Number of heap bytes waiting to be used.
# TYPE go_memstats_heap_idle_bytes gauge
go_memstats_heap_idle_bytes 4.8390144e+08
# HELP go_memstats_heap_inuse_bytes Number of heap bytes that are in use.
# TYPE go_memstats_heap_inuse_bytes gauge
go_memstats_heap_inuse_bytes 1.18898688e+08
# HELP go_memstats_heap_objects Number of allocated objects.
# TYPE go_memstats_heap_objects gauge
go_memstats_heap_objects 312946
# HELP go_memstats_heap_released_bytes Number of heap bytes released to OS.
# TYPE go_memstats_heap_released_bytes gauge
go_memstats_heap_released_bytes 4.49609728e+08
# HELP go_memstats_heap_sys_bytes Number of heap bytes obtained from system.
# TYPE go_memstats_heap_sys_bytes gauge
go_memstats_heap_sys_bytes 6.02800128e+08
# HELP go_memstats_last_gc_time_seconds Number of seconds since 1970 of last garbage collection.
# TYPE go_memstats_last_gc_time_seconds gauge
go_memstats_last_gc_time_seconds 1.5917296489909294e+09
# HELP go_memstats_lookups_total Total number of pointer lookups.
# TYPE go_memstats_lookups_total counter
go_memstats_lookups_total 0
# HELP go_memstats_mallocs_total Total number of mallocs.
# TYPE go_memstats_mallocs_total counter
go_memstats_mallocs_total 4.63721094e+08
# HELP go_memstats_mcache_inuse_bytes Number of bytes in use by mcache structures.
# TYPE go_memstats_mcache_inuse_bytes gauge
go_memstats_mcache_inuse_bytes 6944
# HELP go_memstats_mcache_sys_bytes Number of bytes used for mcache structures obtained from system.
# TYPE go_memstats_mcache_sys_bytes gauge
go_memstats_mcache_sys_bytes 16384
# HELP go_memstats_mspan_inuse_bytes Number of bytes in use by mspan structures.
# TYPE go_memstats_mspan_inuse_bytes gauge
go_memstats_mspan_inuse_bytes 2.959088e+06
# HELP go_memstats_mspan_sys_bytes Number of bytes used for mspan structures obtained from system.
# TYPE go_memstats_mspan_sys_bytes gauge
go_memstats_mspan_sys_bytes 7.389184e+06
# HELP go_memstats_next_gc_bytes Number of heap bytes when next garbage collection will take place.
# TYPE go_memstats_next_gc_bytes gauge
go_memstats_next_gc_bytes 7.8598512e+07
# HELP go_memstats_other_sys_bytes Number of bytes used for other system allocations.
# TYPE go_memstats_other_sys_bytes gauge
go_memstats_other_sys_bytes 2.08219e+06
# HELP go_memstats_stack_inuse_bytes Number of bytes in use by the stack allocator.
# TYPE go_memstats_stack_inuse_bytes gauge
go_memstats_stack_inuse_bytes 1.179648e+06
# HELP go_memstats_stack_sys_bytes Number of bytes obtained from system for stack allocator.
# TYPE go_memstats_stack_sys_bytes gauge
go_memstats_stack_sys_bytes 1.179648e+06
# HELP go_memstats_sys_bytes Number of bytes obtained from system.
# TYPE go_memstats_sys_bytes gauge
go_memstats_sys_bytes 6.39162616e+08
# HELP go_threads Number of OS threads created.
# TYPE go_threads gauge
go_threads 14
# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds.
# TYPE process_cpu_seconds_total counter
process_cpu_seconds_total 340.33
# HELP process_max_fds Maximum number of open file descriptors.
# TYPE process_max_fds gauge
process_max_fds 1.048576e+06
# HELP process_open_fds Number of open file descriptors.
# TYPE process_open_fds gauge
process_open_fds 9
# HELP process_resident_memory_bytes Resident memory size in bytes.
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes 1.92991232e+08
# HELP process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE process_start_time_seconds gauge
process_start_time_seconds 1.59172872096e+09
# HELP process_virtual_memory_bytes Virtual memory size in bytes.
# TYPE process_virtual_memory_bytes gauge
process_virtual_memory_bytes 6.90434048e+08
# HELP process_virtual_memory_max_bytes Maximum amount of virtual memory available in bytes.
# TYPE process_virtual_memory_max_bytes gauge
process_virtual_memory_max_bytes -1
# HELP promhttp_metric_handler_requests_in_flight Current number of scrapes being served.
# TYPE promhttp_metric_handler_requests_in_flight gauge
promhttp_metric_handler_requests_in_flight 1
# HELP promhttp_metric_handler_requests_total Total number of scrapes by HTTP status code.
# TYPE promhttp_metric_handler_requests_total counter
promhttp_metric_handler_requests_total{code="200"} 1
promhttp_metric_handler_requests_total{code="500"} 0
promhttp_metric_handler_requests_total{code="503"} 0
# HELP remote_adapter_api_request_duration_seconds A histogram of latencies for requests.
# TYPE remote_adapter_api_request_duration_seconds histogram
remote_adapter_api_request_duration_seconds_bucket{handler="write",method="post",le="0.25"} 27489
remote_adapter_api_request_duration_seconds_bucket{handler="write",method="post",le="0.5"} 28057
remote_adapter_api_request_duration_seconds_bucket{handler="write",method="post",le="1"} 29442
remote_adapter_api_request_duration_seconds_bucket{handler="write",method="post",le="2.5"} 34115
remote_adapter_api_request_duration_seconds_bucket{handler="write",method="post",le="5"} 45541
remote_adapter_api_request_duration_seconds_bucket{handler="write",method="post",le="10"} 63879
remote_adapter_api_request_duration_seconds_bucket{handler="write",method="post",le="+Inf"} 63879
remote_adapter_api_request_duration_seconds_sum{handler="write",method="post"} 164068.22068123778
remote_adapter_api_request_duration_seconds_count{handler="write",method="post"} 63879
# HELP remote_adapter_api_requests_total A counter for requests to the wrapped handler.
# TYPE remote_adapter_api_requests_total counter
remote_adapter_api_requests_total{code="200",handler="write",method="post"} 63879
# HELP remote_adapter_api_response_size_bytes A histogram of response sizes for requests.
# TYPE remote_adapter_api_response_size_bytes histogram
remote_adapter_api_response_size_bytes_bucket{handler="write",le="200"} 63879
remote_adapter_api_response_size_bytes_bucket{handler="write",le="500"} 63879
remote_adapter_api_response_size_bytes_bucket{handler="write",le="900"} 63879
remote_adapter_api_response_size_bytes_bucket{handler="write",le="1500"} 63879
remote_adapter_api_response_size_bytes_bucket{handler="write",le="+Inf"} 63879
remote_adapter_api_response_size_bytes_sum{handler="write"} 1.27758e+06
remote_adapter_api_response_size_bytes_count{handler="write"} 63879
# HELP remote_adapter_received_samples_total Total number of received samples.
# TYPE remote_adapter_received_samples_total counter
remote_adapter_received_samples_total{prefix="test.miniha_kubernetes."} 6.268375e+06
# HELP remote_adapter_sent_batch_duration_seconds Duration of sample batch send calls to the remote storage.
# TYPE remote_adapter_sent_batch_duration_seconds histogram
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="0.005"} 24169
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="0.01"} 26018
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="0.025"} 26530
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="0.05"} 26788
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="0.1"} 27207
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="0.25"} 27708
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="0.5"} 28295
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="1"} 29634
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="2.5"} 34454
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="5"} 47266
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="10"} 63879
remote_adapter_sent_batch_duration_seconds_bucket{remote="10.0.0.0:2003",le="+Inf"} 63879
remote_adapter_sent_batch_duration_seconds_sum{remote="10.0.0.0:2003"} 157999.59580358808
remote_adapter_sent_batch_duration_seconds_count{remote="10.0.0.0:2003"} 63879
# HELP remote_adapter_sent_samples_total Total number of processed samples sent to remote storage.
# TYPE remote_adapter_sent_samples_total counter
remote_adapter_sent_samples_total{prefix="test.miniha_kubernetes.",remote="10.0.0.0:2003"} 6.268375e+06
```
