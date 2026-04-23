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

	"log/slog"

	graphiteconfig "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
)

type Server interface {
	Run(wg *sync.WaitGroup) error
	Close() error
}

// NewServer creates a new Server using given protocol, addr and Reader
func NewServer(protocol, addr string, compressType graphiteconfig.CompressType, logger *slog.Logger) (Server, error) {
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
	logger       *slog.Logger
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
				t.logger.Error("failed to accept connection", "err", srvErr.Error())
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
						t.logger.Error("failed to close pipe reader", "err", errClose.Error())
						err = errClose
					}
				}(lz4reader)
				_, err = io.CopyBuffer(t.writer, lz4reader, make([]byte, 1<<18))
				if err != nil {
					t.logger.Error("error copying from lz4 reader", "err", err)
				}
				// Shut down the connection.
				err = conn.Close()
				if err != nil {
					t.logger.Error("failed to close connection", "err", err.Error())
				}
			}(conn)
		case graphiteconfig.Plain:
			fallthrough
		default:
			go func(c net.Conn) {
				_, err = io.CopyBuffer(t.writer, c, make([]byte, 1<<18))
				if err != nil {
					t.logger.Error("error copying from conn", "err", err)
				}
				// Shut down the connection.
				err = c.Close()
				if err != nil {
					t.logger.Error("failed to close connection", "err", err.Error())
				}
			}(conn)
		}
	}
	return
}

// Close shuts down the TCP Server
func (t *TCPServer) Close() (err error) {
	err = t.writer.Close()
	if err != nil {
		t.logger.Error("failed to close pipe writer", "err", err.Error())
		return err
	}
	err = t.reader.Close()
	if err != nil {
		t.logger.Error("failed to close pipe reader", "err", err.Error())
		return err
	}
	if t.server == nil {
		return nil
	}
	return t.server.Close()
}

func waitForHTTPServer(t *testing.T, addr string) {
	t.Helper()

	assert.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return false
		}
		if closeErr := conn.Close(); closeErr != nil {
			t.Logf("failed to close readiness probe connection: %v", closeErr)
		}
		return true
	}, 5*time.Second, 50*time.Millisecond, "web server did not become ready on %s", addr)
}

func TestCompression(t *testing.T) {
	debugLevel := promslog.NewLevel()
	err := debugLevel.Set("debug")
	assert.NoError(t, err)
	logger := promslog.New(&promslog.Config{Level: debugLevel, Format: promslog.NewFormat()})

	cfg := config.DefaultConfig
	cfg.Web.ListenAddress = "127.0.0.1:9201"
	cfg.Graphite.Write.CarbonAddress = ":2003"
	cfg.Graphite.Write.CompressType = graphiteconfig.LZ4

	webHandler := web.New(logger.With("component", "web"), &cfg)
	assert.NoError(t, err)

	go func() {
		runErr := webHandler.Run()
		if runErr != nil {
			logger.Error("web handler run error", "err", runErr)
		}
	}()

	var srv Server
	srv, err = NewServer("tcp", cfg.Graphite.Write.CarbonAddress, cfg.Graphite.Write.CompressType, logger)
	assert.NoError(t, err, "error starting TCP server")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		runErr := srv.Run(&wg)
		assert.NoError(t, runErr, "error running TCP server")
		if runErr == nil {
			closeErr := srv.Close()
			assert.NoError(t, closeErr, "error closing TCP server")
		}
	}()
	wg.Wait()
	waitForHTTPServer(t, cfg.Web.ListenAddress)

	file, err := os.Open("./testdata/req.sz")
	assert.NoError(t, err)

	defer func() {
		errClose := file.Close()
		if errClose != nil {
			t.Logf("failed to close file: %v", errClose)
		}
	}()
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

	client := &http.Client{}
	res, reqErr := client.Do(r)
	assert.NoError(t, reqErr)
	if reqErr == nil {
		assert.NotNil(t, res)
		if res != nil {
			defer func(body io.ReadCloser) {
				respErr := body.Close()
				if respErr != nil {
					logger.Error("failed to close response body", "err", respErr)
				}
			}(res.Body)
			assert.Equal(t, http.StatusOK, res.StatusCode)
		}
	}

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
	assert.True(t, bytes.Equal(inputBuffer, b))
}

func TestShortSizeCompression(t *testing.T) {
	debugLevel := promslog.NewLevel()
	err := debugLevel.Set("debug")
	assert.NoError(t, err)
	logger := promslog.New(&promslog.Config{Level: debugLevel, Format: promslog.NewFormat()})

	cfg := config.DefaultConfig
	cfg.Web.ListenAddress = "127.0.0.1:9202"
	cfg.Graphite.Write.CarbonAddress = ":2004"
	cfg.Graphite.Write.CompressType = graphiteconfig.LZ4

	webHandler := web.New(logger.With("component", "web"), &cfg)
	assert.NoError(t, err)

	go func() {
		runErr := webHandler.Run()
		if runErr != nil {
			logger.Error("web handler run error", "err", runErr)
		}
	}()

	var srv Server
	srv, err = NewServer("tcp", cfg.Graphite.Write.CarbonAddress, cfg.Graphite.Write.CompressType, logger)
	assert.NoError(t, err, "error starting TCP server")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		runErr := srv.Run(&wg)
		assert.NoError(t, runErr, "error running TCP server")
		if runErr == nil {
			closeErr := srv.Close()
			assert.NoError(t, closeErr, "error closing TCP server")
		}
	}()
	wg.Wait()
	waitForHTTPServer(t, cfg.Web.ListenAddress)

	file, err := os.Open("./testdata/short_req.sz")
	assert.NoError(t, err)

	defer func() {
		errClose := file.Close()
		if errClose != nil {
			logger.Error("failed to close file", "err", errClose)
		}
	}()
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

	client := &http.Client{}
	res, reqErr := client.Do(r)
	assert.NoError(t, reqErr)
	if reqErr == nil {
		assert.NotNil(t, res)
		if res != nil {
			defer func(body io.ReadCloser) {
				respErr := body.Close()
				if respErr != nil {
					logger.Error("failed to close response body", "err", respErr)
				}
			}(res.Body)
			assert.Equal(t, http.StatusOK, res.StatusCode)
		}
	}

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
	assert.True(t, bytes.Equal(inputBuffer, b))
}

func TestWithoutCompression(t *testing.T) {
	debugLevel := promslog.NewLevel()
	err := debugLevel.Set("debug")
	assert.NoError(t, err)
	logger := promslog.New(&promslog.Config{Level: debugLevel, Format: promslog.NewFormat()})

	cfg := config.DefaultConfig
	cfg.Web.ListenAddress = "127.0.0.1:9203"
	cfg.Graphite.Write.CarbonAddress = ":2005"
	cfg.Graphite.Write.CompressType = graphiteconfig.Plain

	webHandler := web.New(logger.With("component", "web"), &cfg)
	err = webHandler.ApplyConfig(&cfg)
	assert.NoError(t, err)

	go func() {
		runErr := webHandler.Run()
		if runErr != nil {
			logger.Error("web handler run error", "err", runErr)
		}
	}()

	var srv Server
	srv, err = NewServer("tcp", cfg.Graphite.Write.CarbonAddress, cfg.Graphite.Write.CompressType, logger)
	assert.NoError(t, err, "error starting TCP server")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		runErr := srv.Run(&wg)
		assert.NoError(t, runErr, "error running TCP server")
		if runErr == nil {
			closeErr := srv.Close()
			assert.NoError(t, closeErr, "error closing TCP server")
		}
	}()
	wg.Wait()
	waitForHTTPServer(t, cfg.Web.ListenAddress)

	file, err := os.Open("./testdata/req.sz")
	assert.NoError(t, err)

	defer func() {
		err := file.Close()
		if err != nil {
			logger.Error("failed to close file", "err", err)
		}
	}()
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

	client := &http.Client{}
	res, reqErr := client.Do(r)
	assert.NoError(t, reqErr)
	if reqErr == nil {
		assert.NotNil(t, res)
		if res != nil {
			defer func(body io.ReadCloser) {
				respErr := body.Close()
				if respErr != nil {
					logger.Error("failed to close response body", "err", respErr)
				}
			}(res.Body)
			assert.Equal(t, http.StatusOK, res.StatusCode)
		}
	}

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
	assert.True(t, bytes.Equal(inputBuffer, b))
}
