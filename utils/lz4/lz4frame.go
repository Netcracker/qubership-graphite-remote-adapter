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

//go:build cgo

package lz4

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unsafe"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

/*
#cgo CFLAGS: -I/usr/include
#cgo linux LDFLAGS: -l:liblz4.so
#include <lz4frame.h>
#include <stdlib.h>
*/
import "C"

// Writer is a wrapper around an io.Writer that compresses data using lz4frame c library before writing it.
type Writer struct {
	logger           log.Logger
	writer           io.Writer
	ctx              *C.LZ4F_cctx
	preferences      *C.LZ4F_preferences_t
	outputBufferSize int
	inputBuffer      []byte
	outputBuffer     []byte
}

// NewWriter creates a new Writer with the given underlying io.Writer and compression preferences.
func NewWriter(writer io.Writer, logger log.Logger, cfg *config.LZ4Preferences) (*Writer, error) {
	var ctx *C.LZ4F_cctx
	// Create a LZ4F compression context
	errCode := C.LZ4F_createCompressionContext(&ctx, C.LZ4F_getVersion())
	if C.LZ4F_isError(errCode) != 0 {
		err := errors.New(C.GoString(C.LZ4F_getErrorName(errCode)))
		_ = level.Error(logger).Log("err", err, "msg", "error creating compression context")
		return nil, err
	}

	blockSizeId := C.LZ4F_max64KB
	blockMode := C.LZ4F_blockLinked
	contentChecksumFlag := C.LZ4F_noContentChecksum
	blockChecksumFlag := C.LZ4F_noBlockChecksum
	compressionLevel := C.int(config.LZ4CompressLevelDefault)
	autoFlush := C.uint(0)
	decompressionSpeed := C.uint(0)
	if cfg != nil {
		compressionLevel = C.int(cfg.CompressionLevel)
		if cfg.AutoFlush {
			autoFlush = C.uint(1)
		}
		if cfg.DecompressionSpeed {
			decompressionSpeed = C.uint(1)
		}
		if cfg.FrameInfo != nil {
			switch cfg.FrameInfo.BlockSizeID {
			case config.LZ4fBlockSizeMax256kb:
				blockSizeId = C.LZ4F_max256KB
			case config.LZ4fBlockSizeMax1mb:
				blockSizeId = C.LZ4F_max1MB
			case config.LZ4fBlockSizeMax4mb:
				blockSizeId = C.LZ4F_max4MB
			}
			if cfg.FrameInfo.BlockMode {
				blockMode = C.LZ4F_blockIndependent
			}
			if cfg.FrameInfo.ContentChecksumFlag {
				contentChecksumFlag = C.LZ4F_contentChecksumEnabled
			}
			if cfg.FrameInfo.BlockChecksumFlag {
				blockChecksumFlag = C.LZ4F_blockChecksumEnabled
			}
		}
	}

	// Create a LZ4F preferences structure
	preferences := C.LZ4F_preferences_t{
		frameInfo: C.LZ4F_frameInfo_t{
			blockSizeID:         C.LZ4F_blockSizeID_t(blockSizeId),
			blockMode:           C.LZ4F_blockMode_t(blockMode),
			contentChecksumFlag: C.LZ4F_contentChecksum_t(contentChecksumFlag),
			blockChecksumFlag:   C.LZ4F_blockChecksum_t(blockChecksumFlag),
		},
		compressionLevel: compressionLevel,
		autoFlush:        autoFlush,
		favorDecSpeed:    decompressionSpeed,
	}

	outputBufferSize := int(C.LZ4F_compressBound(C.size_t(C.LZ4F_max256KB), &preferences)) // Use LZ4F_compressBound to get the maximum output size

	return &Writer{
		logger:           logger,
		writer:           writer,
		ctx:              ctx,
		preferences:      &preferences,
		inputBuffer:      make([]byte, outputBufferSize),
		outputBuffer:     make([]byte, outputBufferSize),
		outputBufferSize: outputBufferSize,
	}, nil
}

// Write compresses p using lz4frame and writes it to the underlying io.Writer.
// It returns the number of bytes written and any error encountered.
func (writer *Writer) Write(inputData []byte) (int, error) {
	// Start the frame
	ensureBuffer(writer, len(inputData))
	outputPtr := unsafe.Pointer(&writer.outputBuffer[0]) // Create a C pointer to the output buffer
	headerSize := C.LZ4F_compressBegin(writer.ctx, outputPtr, C.size_t(writer.outputBufferSize), writer.preferences)
	if C.LZ4F_isError(headerSize) != 0 {
		err := errors.New(C.GoString(C.LZ4F_getErrorName(headerSize)))
		_ = level.Error(writer.logger).Log("err", err, "msg", "error creating frame")
		return 0, err
	}

	// Write the frame header to the destination file
	sent, err := writer.writer.Write(writer.outputBuffer[:uint64(headerSize)])
	if err != nil {
		_ = level.Error(writer.logger).Log("err", err, "msg", "error writing frame header")
		return 0, err
	}

	if uint64(sent) != uint64(headerSize) {
		_ = level.Error(writer.logger).Log("err", io.ErrShortWrite, "msg", "error writing frame header")
		return 0, io.ErrShortWrite
	}

	_ = level.Debug(writer.logger).Log("msg", "input data", "size", strconv.Itoa(len(inputData)))
	var sentOut int
	_ = level.Debug(writer.logger).Log("msg", "frame header", "size", strconv.FormatUint(uint64(headerSize), 10))
	_ = level.Debug(writer.logger).Log("msg", "frame sent", "size", strconv.Itoa(sent))
	sentOut += int(headerSize)

	// convert byte slice to io.Reader
	reader := bytes.NewReader(inputData)
	// Loop until the end of the source file
	var m int
	for {
		// Read a chunk of data from the source
		m, err = reader.Read(writer.inputBuffer)
		if err != nil && err != io.EOF {
			_ = level.Error(writer.logger).Log("err", err, "msg", "error reading source")
			return 0, err
		}
		if m == 0 {
			break // End of buffer
		}

		// Compress the chunk of data
		compressedSize := C.LZ4F_compressUpdate(writer.ctx, outputPtr, C.size_t(writer.outputBufferSize), unsafe.Pointer(&writer.inputBuffer[0]), C.size_t(m), nil)
		if C.LZ4F_isError(compressedSize) != 0 {
			err = errors.New(C.GoString(C.LZ4F_getErrorName(compressedSize)))
			_ = level.Error(writer.logger).Log("err", err, "msg", "error compressing data")
			return 0, err
		}

		if compressedSize == 0 {
			// zero meaning input data was just buffered and is not written into outputBuffer
			continue
		}

		// Write the compressed data to the destination file
		sent, err = writer.writer.Write(writer.outputBuffer[:uint64(compressedSize)])
		if err != nil {
			_ = level.Error(writer.logger).Log("err", err, "msg", "error writing compressed data")
			return 0, err
		}

		_ = level.Debug(writer.logger).Log("msg", "frame compressed", "size", strconv.FormatUint(uint64(compressedSize), 10))
		_ = level.Debug(writer.logger).Log("msg", "frame sent", "size", strconv.Itoa(sent))

		if uint64(sent) != uint64(compressedSize) {
			_ = level.Error(writer.logger).Log("err", io.ErrShortWrite, "msg", "error writing frame header")
			return 0, io.ErrShortWrite
		}
		sentOut += int(compressedSize)
	}

	// End the frame
	tailSize := C.LZ4F_compressEnd(writer.ctx, outputPtr, C.size_t(writer.outputBufferSize), nil)
	if C.LZ4F_isError(tailSize) != 0 {
		err = errors.New(C.GoString(C.LZ4F_getErrorName(tailSize)))
		_ = level.Error(writer.logger).Log("err", err, "msg", "error ending frame")
		return 0, err
	}

	// Write the frame footer to the destination file
	sent, err = writer.writer.Write(writer.outputBuffer[:uint64(tailSize)])
	if err != nil {
		_ = level.Error(writer.logger).Log("err", err, "msg", "error writing frame footer")
		return 0, err
	}
	_ = level.Debug(writer.logger).Log("msg", "frame tail", "size", strconv.FormatUint(uint64(tailSize), 10))
	_ = level.Debug(writer.logger).Log("msg", "frame sent", "size", strconv.Itoa(sent))

	if uint64(sent) != uint64(tailSize) {
		_ = level.Error(writer.logger).Log("err", io.ErrShortWrite, "msg", "error writing frame footer")
		return 0, io.ErrShortWrite
	}
	sentOut += int(tailSize)

	_ = level.Debug(writer.logger).Log("msg", "compression done", "size", strconv.Itoa(sentOut))

	return len(inputData), err
}

// Close flushes any remaining data and frees the compression context
func (writer *Writer) Close() error {
	// free the compression context
	errCode := C.LZ4F_freeCompressionContext(writer.ctx)
	if C.LZ4F_isError(errCode) != 0 {
		return errors.New(C.GoString(C.LZ4F_getErrorName(errCode)))
	}
	return nil
}

// Reader is a reader that decompresses lz4 streams using lz4frame c library
type Reader struct {
	logger log.Logger
	reader io.Reader    // the underlying reader
	ctx    *C.LZ4F_dctx // the decompression context
	buffer []byte       // the buffer to store decompressed data
	offset int
	size   int
}

// NewReader creates a new Reader with the given underlying io.Reader and decompression preferences.
func NewReader(reader io.Reader, logger log.Logger, bufferSize int) (*Reader, error) {
	// Create a decompression context
	var ctx *C.LZ4F_dctx
	errCode := C.LZ4F_createDecompressionContext(&ctx, C.LZ4F_getVersion())
	if C.LZ4F_isError(errCode) != 0 {
		return nil, fmt.Errorf("failed to create decompression context: %s", C.GoString(C.LZ4F_getErrorName(errCode)))
	}

	return &Reader{
		logger: logger,
		reader: reader,
		ctx:    ctx,
		buffer: make([]byte, bufferSize),
		offset: 0,
		size:   0,
	}, nil
}

// Read implements the io.Reader interface
func (reader *Reader) Read(p []byte) (int, error) {
	n := 0 // the number of bytes read
	var srcSize, dstSize C.size_t
	var srcPtr, dstPtr *C.char
	for n < len(p) {
		if reader.offset == reader.size {
			// the buffer is empty, read more data from the underlying reader
			read, err := reader.reader.Read(reader.buffer)
			if err != nil {
				return n, err
			}
			reader.offset = 0
			reader.size = read
		}
		// decompress the data from the buffer
		srcSize = C.size_t(reader.size - reader.offset)
		dstSize = C.size_t(len(p) - n)
		srcPtr = (*C.char)(unsafe.Pointer(&reader.buffer[reader.offset]))
		dstPtr = (*C.char)(unsafe.Pointer(&p[n]))
		consumed := C.LZ4F_decompress(reader.ctx, unsafe.Pointer(dstPtr), &dstSize, unsafe.Pointer(srcPtr), &srcSize, nil)
		if C.LZ4F_isError(consumed) != 0 {
			return n, errors.New(C.GoString(C.LZ4F_getErrorName(consumed)))
		}
		reader.offset += int(srcSize)
		n += int(dstSize)
		if consumed == 0 {
			// the end of the lz4 stream
			break
		}
	}

	_ = level.Info(reader.logger).Log("msg", "decompression done", "received", strconv.Itoa(n))
	return n, nil
}

// Close frees the decompression context
func (reader *Reader) Close() error {
	// free the decompression context
	errCode := C.LZ4F_freeDecompressionContext(reader.ctx)
	if C.LZ4F_isError(errCode) != 0 {
		return errors.New(C.GoString(C.LZ4F_getErrorName(errCode)))
	}
	return nil
}

func ensureBuffer(writer *Writer, inputSize int) {
	newOutputBufferSize := int(C.LZ4F_compressBound(C.size_t(inputSize), writer.preferences))
	if writer.outputBufferSize < newOutputBufferSize {
		writer.inputBuffer = make([]byte, newOutputBufferSize)
		writer.outputBuffer = make([]byte, newOutputBufferSize)
	} else {
		writer.inputBuffer = writer.inputBuffer[:newOutputBufferSize]
		writer.outputBuffer = writer.outputBuffer[:newOutputBufferSize]
	}
	writer.outputBufferSize = newOutputBufferSize
}
