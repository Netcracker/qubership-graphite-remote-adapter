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
package utils

import "testing"

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		length int
		want   string
	}{
		{"negative length", "test", -1, ""},
		{"zero length", "test", 0, ""},
		{"shorter than length", "test", 10, "test"},
		{"exact length", "test", 4, "test"},
		{"truncate ascii", "0123456789abcd...", 10, "0123456789"},
		{"truncate unicode", "测验", 1, "测"},
		{"truncate emoji", "😊😊😊", 2, "😊😊"},
		{"truncate mixed ascii unicode", "a😊b", 2, "a😊"},
		{"empty string", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.length)
			if result != tt.want {
				t.Fatalf("TruncateString(%q, %d) = %q; want %q", tt.input, tt.length, result, tt.want)
			}
		})
	}
}
