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

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
)

func TestHandler_healthy(t *testing.T) {
	cfg := &config.Config{}
	logger := log.NewNopLogger()
	h := New(logger, cfg)

	req := httptest.NewRequest("GET", "/-/healthy", nil)
	w := httptest.NewRecorder()

	h.healthy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OK")
}

func TestHandler_home(t *testing.T) {
	cfg := &config.Config{}
	logger := log.NewNopLogger()
	h := New(logger, cfg)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.home(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String())
}

func TestHandler_simulation(t *testing.T) {
	cfg := &config.Config{}
	logger := log.NewNopLogger()
	h := New(logger, cfg)

	req := httptest.NewRequest("GET", "/simulation", nil)
	w := httptest.NewRecorder()

	h.simulation(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String())
}
