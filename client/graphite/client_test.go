package graphite

import (
	"testing"

	graphiteCfg "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	logger := log.NewNopLogger()
	cfg := config.DefaultConfig
	cfg.Graphite.Write.CarbonAddress = "localhost:2003"

	client := NewClient(&cfg, logger)
	assert.NotNil(t, client)
	assert.Equal(t, "graphite", client.Name())
	assert.NotNil(t, client.cfg)
}

func TestNewClientGraphiteCfg(t *testing.T) {
	logger := log.NewNopLogger()
	cfg := &graphiteCfg.Config{}

	client := NewClientGraphiteCfg(cfg, logger)
	assert.NotNil(t, client)
	assert.Equal(t, cfg, client.cfg)
}

func TestClient_Name(t *testing.T) {
	client := &Client{}
	assert.Equal(t, "graphite", client.Name())
}

func TestClient_Target(t *testing.T) {
	client := &Client{}
	// Without connection, should return "unknown"
	assert.Equal(t, "unknown", client.Target())
}

func TestClient_String(t *testing.T) {
	cfg := &graphiteCfg.Config{}
	client := &Client{cfg: cfg}
	// This will depend on cfg.String(), but we can test it doesn't panic
	str := client.String()
	assert.NotEmpty(t, str)
}

func TestClient_Cfg(t *testing.T) {
	cfg := &graphiteCfg.Config{}
	client := &Client{cfg: cfg}
	assert.Equal(t, cfg, client.Cfg())
}

func TestClient_Shutdown(t *testing.T) {
	client := &Client{}
	// Shutdown should not panic
	client.Shutdown()
}
