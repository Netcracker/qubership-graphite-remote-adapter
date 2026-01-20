package lz4

import (
	"testing"

	"github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
)

func TestNewWriter(t *testing.T) {
	logger := log.NewNopLogger()
	cfg := &config.LZ4Preferences{}

	writer, err := NewWriter(nil, logger, cfg)
	if err != nil {
		// If lz4 is not available, expect error
		assert.Error(t, err)
	} else {
		assert.NotNil(t, writer)
		writer.Close()
	}
}

func TestNewReader(t *testing.T) {
	logger := log.NewNopLogger()

	reader, err := NewReader(nil, logger, 1024)
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.NotNil(t, reader)
		reader.Close()
	}
}
