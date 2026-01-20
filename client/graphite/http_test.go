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

package graphite

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatapointUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected Datapoint
		hasError bool
	}{
		{
			name: "valid datapoint",
			json: `[1.5, 1234567890]`,
			expected: Datapoint{
				Value:     &[]float64{1.5}[0],
				Timestamp: 1234567890,
			},
			hasError: false,
		},
		{
			name:     "null value",
			json:     `[null, 1234567890]`,
			expected: Datapoint{Timestamp: 1234567890},
			hasError: false,
		},
		{
			name:     "invalid json",
			json:     `invalid`,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dp Datapoint
			err := json.Unmarshal([]byte(tt.json), &dp)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.expected.Value != nil {
					assert.Equal(t, *tt.expected.Value, *dp.Value)
				} else {
					assert.Nil(t, dp.Value)
				}
				assert.Equal(t, tt.expected.Timestamp, dp.Timestamp)
			}
		})
	}
}
