// Copyright 2015 The Prometheus Authors
// Copyright 2017 Thibault Chataigner <thibault.chataigner@gmail.com>
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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"time"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	gpaths "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/paths"
	"github.com/Netcracker/qubership-graphite-remote-adapter/utils/lz4"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/model"
)

const udpMaxBytes = 1024

func (client *Client) connectToCarbon() (net.Conn, error) {
	if client.carbonCon != nil {
		if time.Since(client.carbonLastReconnectTime) < client.cfg.Write.CarbonReconnectInterval {
			// Last reconnect is not too long ago, re-use the connection.
			return client.carbonCon, nil
		}
		_ = level.Debug(client.logger).Log(
			"last", client.carbonLastReconnectTime,
			"msg", "Reinitializing the connection to carbon")
		client.disconnectFromCarbon()
	}

	_ = level.Debug(client.logger).Log(
		"transport", client.cfg.Write.CarbonTransport,
		"address", client.cfg.Write.CarbonAddress,
		"timeout", client.writeTimeout,
		"msg", "Connecting to carbon")
	conn, err := net.DialTimeout(client.cfg.Write.CarbonTransport, client.cfg.Write.CarbonAddress, client.writeTimeout)
	if err != nil {
		client.carbonCon = nil
	} else {
		client.carbonLastReconnectTime = time.Now()
		client.carbonCon = conn
	}

	return client.carbonCon, err
}

func (client *Client) disconnectFromCarbon() {
	if client.carbonCon != nil {
		_ = client.carbonCon.Close()
	}
	client.carbonCon = nil
}

func (client *Client) prepareWrite(samples model.Samples, reqBufLen int, r *http.Request) ([]*bytes.Buffer, error) {
	_ = level.Debug(client.logger).Log("num_samples", len(samples), "storage", client.Name(), "msg", "Remote write")

	graphitePrefix := client.cfg.StoragePrefixFromRequest(r)

	var currentBuf *bytes.Buffer
	if client.cfg.Write.CarbonTransport == "udp" {
		buf := make([]byte, 0, udpMaxBytes)
		currentBuf = bytes.NewBuffer(buf)
	} else {
		buf := make([]byte, 0, reqBufLen)
		currentBuf = bytes.NewBuffer(buf)
	}

	bytesBuffers := []*bytes.Buffer{currentBuf}
	for _, s := range samples {
		datapoints, err := gpaths.ToDatapoints(s, client.format, graphitePrefix, client.cfg.Write.Rules, client.cfg.Write.TemplateData)
		//_ = level.Debug(c.logger).Log("sample", s.String())
		if err != nil {
			_ = level.Debug(client.logger).Log("sample", s, "err", err)
			client.ignoredSamples.Inc()
			continue
		}
		for _, str := range datapoints {
			if client.cfg.Write.CarbonTransport == "udp" && (currentBuf.Len()+len(str)) > udpMaxBytes {
				currentBuf = bytes.NewBuffer(make([]byte, 0, udpMaxBytes))
				bytesBuffers = append(bytesBuffers, currentBuf)
			}
			currentBuf.Write(str)
			//level.Debug(c.logger).Log("line", str, "msg", "Sending")
		}
	}
	return bytesBuffers, nil
}

// Write implements the client.Writer interface.
func (client *Client) Write(samples model.Samples, reqBufLen int, r *http.Request, dryRun bool) ([]byte, error) {
	if client.cfg.Write.CarbonAddress == "" {
		return []byte("Skipped: Not set carbon address."), nil
	}

	bytesBuffers, err := client.prepareWrite(samples, reqBufLen, r)
	if err != nil {
		return nil, err
	}

	if dryRun {
		dryRunResponse := make([]byte, 0)
		for _, buf := range bytesBuffers {
			dryRunResponse = append(dryRunResponse, buf.Bytes()...)
		}
		return dryRunResponse, nil

	}
	// We are going to use the socket, lock it.
	client.carbonConLock.Lock()
	defer client.carbonConLock.Unlock()

	select {
	case <-r.Context().Done():
		return []byte("context cancelled."), fmt.Errorf("request context cancelled")
	default:
	}

	for _, buf := range bytesBuffers {
		var conn net.Conn
		conn, err = client.connectToCarbon()
		if err != nil {
			return nil, err
		}
		pipeReader, pipeWriter := io.Pipe()

		switch client.cfg.Write.CompressType {
		case config.LZ4:
			go func() {
				defer client.closePipeWrite(pipeWriter)

				_, err = client.compressLZ4(pipeWriter, buf)
			}()
		case config.Plain:
			fallthrough
		default:
			go func() {
				defer client.closePipeWrite(pipeWriter)

				_, err = buf.WriteTo(pipeWriter)
			}()
		}

		var written int64
		written, err = io.Copy(conn, pipeReader)
		if err != nil {
			if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
				_ = level.Error(client.logger).Log("msg", "Pipe is broken. Connection closed")
			}
			err = pipeReader.Close()
			if err != nil {
				_ = level.Error(client.logger).Log("err", err.Error(), "msg", "failed to close pipe reader")
			}
			client.disconnectFromCarbon()
			return nil, err
		}

		_ = level.Debug(client.logger).Log("msg", conn.LocalAddr().String()+"->"+conn.RemoteAddr().String(), "sent", strconv.FormatInt(written, 10))

		err = pipeReader.Close()
		if err != nil {
			_ = level.Error(client.logger).Log("err", err.Error(), "msg", "failed to close pipe reader")
		}
	}

	return []byte("Done."), err
}

func (client *Client) compressLZ4(pipeWriter *io.PipeWriter, buf *bytes.Buffer) (written int64, err error) {
	var lz4Writer *lz4.Writer
	lz4Writer, err = lz4.NewWriter(pipeWriter, client.logger, client.cfg.Write.CompressLZ4Preferences)
	if err != nil {
		_ = level.Error(client.logger).Log("err", err)
		return
	}
	defer func(lz4Writer *lz4.Writer) {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic occurred: %v", r)
			_ = level.Error(client.logger).Log("err", err.Error())
		}
		errClose := lz4Writer.Close()
		if errClose != nil {
			_ = level.Error(client.logger).Log("err", errClose.Error(), "msg", "failed to close pipe writer")
			if err == nil {
				err = errClose
			}
		}
	}(lz4Writer) // Make sure the writer is closed

	// Compress the input.
	written, err = io.Copy(lz4Writer, bytes.NewReader(buf.Bytes()))
	if err != nil {
		if !errors.Is(err, io.ErrShortWrite) {
			_ = level.Error(client.logger).Log("err", err)
		}
	}
	return
}

func (client *Client) closePipeWrite(pipeWriter *io.PipeWriter) {
	err := pipeWriter.Close()
	if err != nil {
		_ = level.Error(client.logger).Log("err", err.Error(), "msg", "failed to close pipe writer")
	}
	return
}
