# Developer guide

* [Developer guide](#developer-guide)
  * [Requirements](#requirements)
  * [Run carbon-c-relay](#run-carbon-c-relay)
    * [Prerequisites](#prerequisites)
    * [Running](#running)
  * [Run graphite-remote-adapter](#run-graphite-remote-adapter)
    * [Run graphite-remote-adapter on local machine](#run-graphite-remote-adapter-on-local-machine)
    * [Run test](#run-test)
    * [Generate snappy compressed metric payload](#generate-snappy-compressed-metric-payload)
  * [Run linter before commit](#run-linter-before-commit)

## Requirements

* [Go version](https://go.dev/dl/) version `1.21+`
* [Windows Subsystem for Linux 2 (WSL2)](https://docs.microsoft.com/en-us/windows/wsl/install)
* [Ubuntu for WSL2](https://canonical-ubuntu-wsl.readthedocs-hosted.com/en/latest/guides/install-ubuntu-wsl2/)
* Docker on wsl.
* IDE for Go, we recommended to use:
  * either [Visual Studio Code](https://code.visualstudio.com/), free
  * or [IntelliJ GoLand](https://www.jetbrains.com/go/) or IntelliJ Idea with Go plugin, but both require a license

## Run carbon-c-relay

### Prerequisites

1. Prepare config file. Configuration syntax - [https://manpages.debian.org/testing/carbon-c-relay/carbon-c-relay.1.en.html](https://manpages.debian.org/testing/carbon-c-relay/carbon-c-relay.1.en.html)

   Example for writing metrics in local file:

   ```txt

   # example configuration scenarios.
   # Refer to /usr/share/doc/carbon-c-relay/README.md.gz for additional examples!

   ## mirror all incoming metrics towards two destinations
   #cluster send-through
   #  forward
   #    host1:2003
   #    host2:2003
   #  ;
   #
   #match * send to send-through;

   ## send all incoming metrics to the first responsive host
   #cluster send-to-any-one
   #  any_of host1:2003 host2:2003;
   #match * send to send-to-any-one;

   ## send all incoming metrics to the file
   cluster send-to-file
     file /tmp/metrics.txt;
   match * send to send-to-file;

   #listen
   #    type linemode transport gzip
   #        2004 proto tcp
   #    ;
   #listen
   #    type linemode transport snappy
   #        2005 proto tcp
   #    ;
   #listen
   #    type linemode transport plain
   #       2006 proto tcp
   #    ;

   listen
       type linemode transport lz4
           2003 proto tcp

   ```

2. Create a txt file for writing metrics, e.g. /tmp/metrics.txt

### Running

Example:

1. Run container:

    ```bash
    docker run --rm -p 2003:2003 --name relay -v /etc/carbon-c-relay.conf:/etc/carbon-c-relay.conf:ro -v /tmp/metrics.txt:/tmp/metrics.txt %link to carbon relay artifact% -f /etc/carbon-c-relay.conf
    ```

2. Check list of running containers:

    ```bash
    $ docker ps

    CONTAINER ID   IMAGE                                                                                                                 COMMAND                  CREATED         STATUS         PORTS                                       NAMES
    5f80b9f5d4b3  carbon-c-relay:3.7.4-74193738_20240221-064349   "carbon-c-relay -f …"   7 seconds ago   Up 6 seconds   0.0.0.0:2003->2003/tcp, :::2003->2003/tcp   relay
    ```

## Run graphite-remote-adapter

### Run graphite-remote-adapter on local machine

There is an ability to run or debug graphite-remote-adapter on your local machine.

Configure you environment to run graphite-remote-adapter:

1. Install all required tools from [Requirements](#requirements) section
2. Configure and run [carbon-c-relay](#run-carbon-c-relay)
3. Modify [Default config](../../../client/graphite/config/config.go#L66) like:

   ```go
    var DefaultConfig = Config{
        DefaultPrefix: "",
        EnableTags: false,
            UseOpenMetricsFormat: false,
            Write: WriteConfig{
                CarbonAddress: ":2003",
                CompressType:  LZ4,
            },
        CarbonTransport: "tcp",
        ...
   }
   ```

4. Run main() on wsl.

### Run test

Use [snappy archive](../../../client/graphite/test/testdata/req.sz) for sending to graphite-remote-adapter.
Send snappy compressed metrics like:

```bash
curl --data-binary @./req.sz http://0.0.0.0:9201/write
```

Look through /tmp/metrics.txt

```bash
cat /tmp/metrics.txt
```

It is to contain all metrics from [testdata/sample.txt](../../../client/graphite/test/testdata/sample.txt)

### Generate snappy compressed metric payload

To add more TCs with diff snappy compressed metric payloads use this code for generating:

```go
func GenerateCompression(t *testing.T) {
    var timeseries []prompb.TimeSeries
    for i := 1; i < 2; i++ {
        timeseries = append(timeseries, prompb.TimeSeries{
            Labels: []prompb.Label{
                {Name: "__name__", Value: "test_metric" + strconv.Itoa(i)},
                {Name: "b", Value: "c" + strconv.Itoa(i)},
                {Name: "baz", Value: "qux" + strconv.Itoa(i)},
                {Name: "d", Value: "e" + strconv.Itoa(i)},
                {Name: "foo", Value: "bar" + strconv.Itoa(i)},
            },
            Samples: []prompb.Sample{{Value: float64(i), Timestamp: 0}},
        })
    }

    req := &prompb.WriteRequest{
        Timeseries: timeseries,
    }

    data, err := proto.Marshal(req)
    assert.NoError(t, err)
    compressed := snappy.Encode(nil, data)
    assert.NotNil(t, compressed)
    file, err := os.OpenFile("./test/testdata/short_req.sz", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    _, err = io.Copy(file, bytes.NewReader(compressed))
    assert.NoError(t, err)
    file.Close()
}
```
## Run linter before commit
Before push your commits and create PR run linters and test.
* SuperLinter
```shell
docker run \
  -e RUN_LOCAL=true \
  -e DEFAULT_BRANCH=$(git rev-parse --abbrev-ref HEAD) \
  --env-file .github/super-linter.env \
  -v ${PWD}:/tmp/lint \
  --rm \
  ghcr.io/super-linter/super-linter:slim-$(sed -nE 's#.*uses:\s+super-linter/super-linter/slim@([^\s]+).*#\1#p' .github/workflows/super-linter.yaml)
```
