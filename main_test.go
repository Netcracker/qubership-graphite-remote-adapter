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

package main

import (
	"testing"
	"time"

	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReload(t *testing.T) {
	logger := log.NewNopLogger()

	// Test with no config file
	cliCfg := &config.Config{}
	cfg, err := reload(cliCfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Test with default config merging
	cliCfg.Read.Delay = 5 * time.Second
	cfg, err = reload(cliCfg, logger)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, cfg.Read.Delay)
}
