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

package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Netcracker/qubership-graphite-remote-adapter/config"
	"github.com/go-kit/log"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
)

func TestHandler_read(t *testing.T) {
	cfg := &config.Config{}
	logger := log.NewNopLogger()
	h := New(logger, cfg)

	// Create a mock read request
	req := &prompb.ReadRequest{
		Queries: []*prompb.Query{
			{
				StartTimestampMs: 1000,
				EndTimestampMs:   2000,
			},
		},
	}

	data, err := proto.Marshal(req)
	assert.NoError(t, err)

	compressed := snappy.Encode(nil, data)

	httpReq := httptest.NewRequest("POST", "/read", bytes.NewReader(compressed))
	w := httptest.NewRecorder()

	h.read(w, httpReq)

	// Since no readers, it should error
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
