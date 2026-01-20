//go:build cgo

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

package test

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Netcracker/qubership-graphite-remote-adapter/utils/lz4"
	"github.com/Netcracker/qubership-graphite-remote-adapter/web"

	graphiteconfig "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/promlog"
	"github.com/stretchr/testify/assert"
)

type Server interface {
	Run(wg *sync.WaitGroup) error
	Close() error
}

// NewServer creates a new Server using given protocol, addr and Reader
func NewServer(protocol, addr string, compressType graphiteconfig.CompressType, logger log.Logger) (Server, error) {
	pipeReader, pipeWriter := io.Pipe()
	switch strings.ToLower(protocol) {
	case "tcp":
		return &TCPServer{
			addr:         addr,
			logger:       logger,
			reader:       pipeReader,
			writer:       pipeWriter,
			compressType: compressType,
		}, nil
	case "udp":
	}
	return nil, errors.New("invalid protocol given")
}

type TCPServer struct {
	addr         string
	server       net.Listener
	logger       log.Logger
	reader       *io.PipeReader
	writer       *io.PipeWriter
	compressType graphiteconfig.CompressType
}

// Run starts the TCP Server.
func (t *TCPServer) Run(wg *sync.WaitGroup) (err error) {
	t.server, err = net.Listen("tcp", t.addr)
	if err != nil {
		return
	} else {
		wg.Done()
	}
	for {
		conn, srvErr := t.server.Accept()
		if srvErr != nil {
			if !errors.Is(srvErr, net.ErrClosed) {
				err = errors.New("could not accept connection")
				_ = level.Error(t.logger).Log("err", srvErr.Error(), "msg", "failed to accept connection")
				break
			}
		}
		if conn == nil {
			err = errors.New("could not create connection")
			break
		}
		// Handle the connection in a new goroutine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		switch t.compressType {
		case graphiteconfig.LZ4:
			go func(c net.Conn) {
				var lz4reader *lz4.Reader
				lz4reader, err = lz4.NewReader(c, t.logger, 1<<18)
				defer func(lz4reader *lz4.Reader) {
					errClose := lz4reader.Close()
					if errClose != nil {
						_ = level.Error(t.logger).Log("err", errClose.Error(), "msg", "failed to close pipe reader")
						err = errClose
					}
				}(lz4reader)
				_, err = io.CopyBuffer(t.writer, lz4reader, make([]byte, 1<<18))
				if err != nil {
					_ = level.Error(t.logger).Log("err", err)
				}
				// Shut down the connection.
				err = conn.Close()
				if err != nil {
					_ = level.Error(t.logger).Log("err", err.Error(), "msg", "failed to close connection")
				}
			}(conn)
		case graphiteconfig.Plain:
			fallthrough
		default:
			go func(c net.Conn) {
				_, err = io.CopyBuffer(t.writer, c, make([]byte, 1<<18))
				if err != nil {
					_ = level.Error(t.logger).Log("err", err)
				}
				// Shut down the connection.
				err = c.Close()
				if err != nil {
					_ = level.Error(t.logger).Log("err", err.Error(), "msg", "failed to close connection")
				}
			}(conn)
		}
	}
	return
}

// Close shuts down the TCP Server
func (t *TCPServer) Close() (err error) {
	t.writer.Close()
	t.reader.Close()
	return t.server.Close()
}

func TestCompression(t *testing.T) {
	debugLevel := &promlog.AllowedLevel{}
	err := debugLevel.Set("debug")
	assert.NoError(t, err)
	logger := promlog.New(&promlog.Config{Level: debugLevel, Format: &promlog.AllowedFormat{}})

	cfg := config.DefaultConfig
	cfg.Web.ListenAddress = "127.0.0.1:9201"
	cfg.Graphite.Write.CarbonAddress = ":2003"
	cfg.Graphite.Write.CompressType = graphiteconfig.LZ4

	webHandler := web.New(log.With(logger, "component", "web"), &cfg)
	assert.NoError(t, err)

	go func() {
		err = webHandler.Run()
		if err != nil {
			_ = level.Error(logger).Log("err", err)
		}
	}()
	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	var srv Server
	srv, err = NewServer("tcp", cfg.Graphite.Write.CarbonAddress, cfg.Graphite.Write.CompressType, logger)
	assert.NoError(t, err, "error starting TCP server")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = srv.Run(&wg)
		assert.NoError(t, err, "error running TCP server")
		err = srv.Close()
		assert.NoError(t, err, "error closing TCP server")
	}()
	wg.Wait()

	file, err := os.Open("./testdata/req.sz")
	assert.NoError(t, err)

	defer file.Close()
	stats, statsErr := file.Stat()
	assert.NoError(t, statsErr)
	var size = stats.Size()
	metrics := make([]byte, size)
	buffer := bufio.NewReader(file)
	_, err = buffer.Read(metrics)
	assert.NoError(t, err)

	var inputBuffer []byte
	inputBuffer, err = os.ReadFile("./testdata/sample.txt")
	assert.NoError(t, err)

	posturl := "http://" + cfg.Web.ListenAddress + "/write"
	r, err := http.NewRequest("POST", posturl, bytes.NewBuffer(metrics))
	assert.NoError(t, err)

	wg.Add(1)
	go func() {
		defer wg.Done()
		client := &http.Client{}
		var res *http.Response
		// Retry up to 10 times with 100ms delay
		for i := 0; i < 10; i++ {
			res, err = client.Do(r)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		assert.NoError(t, err)
		defer func(Body io.ReadCloser) {
			respErr := Body.Close()
			if respErr != nil {
				_ = level.Error(logger).Log("err", respErr, "msg", "failed to close response body")
			}
		}(res.Body)
		assert.NotEmpty(t, res)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	}()
	wg.Wait()

	b := make([]byte, len(inputBuffer))
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := srv.(*TCPServer).reader
		_, err = io.ReadFull(reader, b)
		assert.NoError(t, err)
	}()
	wg.Wait()

	assert.NotEmpty(t, b)
	assert.True(t, len(inputBuffer) == len(b))
	assert.True(t, bytes.Compare(inputBuffer, b) == 0)
}

func TestShortSizeCompression(t *testing.T) {
	debugLevel := &promlog.AllowedLevel{}
	err := debugLevel.Set("debug")
	assert.NoError(t, err)
	logger := promlog.New(&promlog.Config{Level: debugLevel, Format: &promlog.AllowedFormat{}})

	cfg := config.DefaultConfig
	cfg.Web.ListenAddress = "127.0.0.1:9202"
	cfg.Graphite.Write.CarbonAddress = ":2004"
	cfg.Graphite.Write.CompressType = graphiteconfig.LZ4

	webHandler := web.New(log.With(logger, "component", "web"), &cfg)
	assert.NoError(t, err)

	go func() {
		err = webHandler.Run()
		if err != nil {
			_ = level.Error(logger).Log("err", err)
		}
	}()

	var srv Server
	srv, err = NewServer("tcp", cfg.Graphite.Write.CarbonAddress, cfg.Graphite.Write.CompressType, logger)
	assert.NoError(t, err, "error starting TCP server")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = srv.Run(&wg)
		assert.NoError(t, err, "error running TCP server")
		err = srv.Close()
		assert.NoError(t, err, "error closing TCP server")
	}()
	wg.Wait()

	file, err := os.Open("./testdata/short_req.sz")
	assert.NoError(t, err)

	defer file.Close()
	stats, statsErr := file.Stat()
	assert.NoError(t, statsErr)
	var size = stats.Size()
	metrics := make([]byte, size)
	buffer := bufio.NewReader(file)
	_, err = buffer.Read(metrics)
	assert.NoError(t, err)

	var inputBuffer []byte
	inputBuffer, err = os.ReadFile("./testdata/short_sample.txt")
	assert.NoError(t, err)

	posturl := "http://" + cfg.Web.ListenAddress + "/write"
	r, err := http.NewRequest("POST", posturl, bytes.NewBuffer(metrics))
	assert.NoError(t, err)

	wg.Add(1)
	go func() {
		defer wg.Done()
		client := &http.Client{}
		var res *http.Response
		// Retry up to 10 times with 100ms delay
		for i := 0; i < 10; i++ {
			res, err = client.Do(r)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		assert.NoError(t, err)
		defer func(Body io.ReadCloser) {
			respErr := Body.Close()
			if respErr != nil {
				_ = level.Error(logger).Log("err", respErr, "msg", "failed to close response body")
			}
		}(res.Body)
		assert.NotEmpty(t, res)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	}()
	wg.Wait()

	b := make([]byte, len(inputBuffer))
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := srv.(*TCPServer).reader
		_, err = io.ReadFull(reader, b)
		assert.NoError(t, err)
	}()
	wg.Wait()

	assert.NotEmpty(t, b)
	assert.True(t, len(inputBuffer) == len(b))
	assert.True(t, bytes.Compare(inputBuffer, b) == 0)
}

func TestWithoutCompression(t *testing.T) {
	debugLevel := &promlog.AllowedLevel{}
	err := debugLevel.Set("debug")
	assert.NoError(t, err)
	logger := promlog.New(&promlog.Config{Level: debugLevel, Format: &promlog.AllowedFormat{}})

	cfg := config.DefaultConfig
	cfg.Web.ListenAddress = "127.0.0.1:9203"
	cfg.Graphite.Write.CarbonAddress = ":2005"
	cfg.Graphite.Write.CompressType = graphiteconfig.Plain

	webHandler := web.New(log.With(logger, "component", "web"), &cfg)
	err = webHandler.ApplyConfig(&cfg)
	assert.NoError(t, err)

	go func() {
		err = webHandler.Run()
		if err != nil {
			_ = level.Error(logger).Log("err", err)
		}
	}()

	var srv Server
	srv, err = NewServer("tcp", cfg.Graphite.Write.CarbonAddress, cfg.Graphite.Write.CompressType, logger)
	assert.NoError(t, err, "error starting TCP server")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = srv.Run(&wg)
		assert.NoError(t, err, "error running TCP server")
		err = srv.Close()
		assert.NoError(t, err, "error closing TCP server")
	}()
	wg.Wait()

	file, err := os.Open("./testdata/req.sz")
	assert.NoError(t, err)

	defer file.Close()
	stats, statsErr := file.Stat()
	assert.NoError(t, statsErr)
	var size = stats.Size()
	metrics := make([]byte, size)
	buffer := bufio.NewReader(file)
	_, err = buffer.Read(metrics)
	assert.NoError(t, err)

	var inputBuffer []byte
	inputBuffer, err = os.ReadFile("./testdata/sample.txt")
	assert.NoError(t, err)

	posturl := "http://" + cfg.Web.ListenAddress + "/write"
	r, err := http.NewRequest("POST", posturl, bytes.NewBuffer(metrics))
	assert.NoError(t, err)

	wg.Add(1)
	go func() {
		defer wg.Done()
		client := &http.Client{}
		var res *http.Response
		// Retry up to 10 times with 100ms delay
		for i := 0; i < 10; i++ {
			res, err = client.Do(r)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		assert.NoError(t, err)
		defer func(Body io.ReadCloser) {
			respErr := Body.Close()
			if respErr != nil {
				_ = level.Error(logger).Log("err", respErr, "msg", "failed to close response body")
			}
		}(res.Body)
		assert.NotEmpty(t, res)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	}()
	wg.Wait()

	b := make([]byte, len(inputBuffer))
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := srv.(*TCPServer).reader
		_, err = io.ReadFull(reader, b)
		assert.NoError(t, err)
	}()
	wg.Wait()

	assert.NotEmpty(t, b)
	assert.True(t, len(inputBuffer) == len(b))
	assert.True(t, bytes.Compare(inputBuffer, b) == 0)
}
