// Copyright 2020 Charles-Antoine Mathieu authored and melchiormoulin committed
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

package utils

import "testing"

func TestTruncateString(t *testing.T) {
	var result string

	result = TruncateString("test", -1)
	if "" != result {
		t.Errorf("Expected %s, got %s", "", result)
	}

	result = TruncateString("test", 10)
	if "test" != result {
		t.Errorf("Expected %s, got %s", "test", result)
	}

	result = TruncateString("0123456789abcd...", 10)
	if "0123456789" != result {
		t.Errorf("Expected %s, got %s", "0123456789", result)
	}

	result = TruncateString("测验", 1)
	if "测" != result {
		t.Errorf("Expected %s, got %s", "测", result)
	}

}
