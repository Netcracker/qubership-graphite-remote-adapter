//go:build !cgo

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

package lz4

import (
	"errors"
	"io"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/go-kit/log"
)

// Writer is a stub for non-CGO builds
type Writer struct {
}

// NewWriter returns an error for non-CGO builds
func NewWriter(writer io.Writer, logger log.Logger, cfg *config.LZ4Preferences) (*Writer, error) {
	return nil, errors.New("LZ4 compression not available: CGO disabled")
}

// Write is a stub
func (writer *Writer) Write(inputData []byte) (int, error) {
	return 0, errors.New("LZ4 compression not available: CGO disabled")
}

// Close is a stub
func (writer *Writer) Close() error {
	return nil
}

// Reader is a stub for non-CGO builds
type Reader struct {
}

// NewReader returns an error for non-CGO builds
func NewReader(reader io.Reader, logger log.Logger, bufferSize int) (*Reader, error) {
	return nil, errors.New("LZ4 decompression not available: CGO disabled")
}

// Read is a stub
func (reader *Reader) Read(p []byte) (int, error) {
	return 0, errors.New("LZ4 decompression not available: CGO disabled")
}

// Close is a stub
func (reader *Reader) Close() error {
	return nil
}
