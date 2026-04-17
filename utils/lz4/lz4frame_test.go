// Copyright 2020 Charles-Antoine Mathieu authored and melchiormoulin committed
// Copyright NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package lz4

import (
	"bytes"
	"io"
	"testing"

	"log/slog"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLZ4CompressionDecompression(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	input := []byte("Hello, World! This is a test string for LZ4 compression.")

	// Create a buffer to write compressed data
	var compressed bytes.Buffer

	// Create LZ4 writer
	writer, err := NewWriter(&compressed, logger, nil)
	require.NoError(t, err)

	// Write data
	n, err := writer.Write(input)
	require.NoError(t, err)
	assert.Equal(t, len(input), n)

	// Close writer
	err = writer.Close()
	require.NoError(t, err)

	// Create LZ4 reader
	reader, err := NewReader(&compressed, logger, 1024)
	require.NoError(t, err)

	// Read decompressed data
	var decompressed bytes.Buffer
	_, err = io.Copy(&decompressed, reader)
	require.NoError(t, err)

	// Close reader
	err = reader.Close()
	require.NoError(t, err)

	// Check if decompressed matches original
	assert.Equal(t, input, decompressed.Bytes())
}

func TestLZ4WriterWithPreferences(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	input := []byte("Test data for LZ4 with preferences.")

	cfg := &config.LZ4Preferences{
		CompressionLevel: 5,
		AutoFlush:        true,
		FrameInfo: &config.LZ4FrameInfo{
			BlockSizeID:         config.LZ4fBlockSizeMax1mb,
			BlockMode:           true,
			ContentChecksumFlag: true,
			BlockChecksumFlag:   true,
		},
	}

	var compressed bytes.Buffer
	writer, err := NewWriter(&compressed, logger, cfg)
	require.NoError(t, err)

	n, err := writer.Write(input)
	require.NoError(t, err)
	assert.Equal(t, len(input), n)

	err = writer.Close()
	require.NoError(t, err)

	// Verify compressed data is not empty
	assert.Greater(t, compressed.Len(), 0)
}
