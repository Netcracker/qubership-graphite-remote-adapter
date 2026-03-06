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
