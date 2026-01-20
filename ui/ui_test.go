package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAsset(t *testing.T) {
	data, err := Asset("static/css/bootstrap-lumen.min.css")
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestAssetInfo(t *testing.T) {
	info, err := AssetInfo("static/css/bootstrap-lumen.min.css")
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Greater(t, info.Size(), int64(0))
}

func TestAssetNames(t *testing.T) {
	names := AssetNames()
	assert.NotEmpty(t, names)
	assert.Contains(t, names, "static/css/bootstrap-lumen.min.css")
}
