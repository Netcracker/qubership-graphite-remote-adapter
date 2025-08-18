// Copyright 2015 The Prometheus Authors
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

package template

import (
	"testing"
)

func TestEscape(t *testing.T) {
	// Can we correctly keep and escape valid chars.
	value := "abzABZ019(){},'\"\\"
	expected := "abzABZ019\\(\\)\\{\\}\\,\\'\\\"\\\\"
	actual := Escape(value)
	if expected != string(actual) {
		t.Errorf("Expected %s, got %s", expected, actual)
	}

	// Test percent-encoding.
	value = "é/|_;:%."
	expected = "%C3%A9%2F|_;:%25%2E"
	actual = Escape(value)
	if expected != string(actual) {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestEscapeTags(t *testing.T) {
	// Can we correctly keep and escape valid chars.
	value := "abzABZ019(){c=\"h/v0.0.0 (linux/amd64) k8s/$f\",c=\"200\",ct=\"app/vnd.k8s.p;s=w\",e=\"https\",i=\"10.0.0.0:8443\"},'\"\\"
	expected := "abzABZ019\\(\\)\\{c%3D\\\"h%2Fv0%2E0%2E0%20\\(linux%2Famd64\\)%20k8s%2F$f\\\"\\,c%3D\\\"200\\\"\\,ct%3D\\\"app%2Fvnd%2Ek8s%2Ep;s%3Dw\\\"\\,e%3D\\\"https\\\"\\,i%3D\\\"10%2E0%2E0%2E0:8443\\\"\\}\\,\\'\\\"\\\\"
	actual := Escape(value)
	if expected != string(actual) {
		t.Errorf("Expected %s, got %s", expected, actual)
	}

	// Test percent-encoding.
	value = "é/|_;:%."
	expected = "%C3%A9%2F|_;:%25%2E"
	actual = Escape(value)
	if expected != string(actual) {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestEscapeTagsSpecialFunction(t *testing.T) {
	// Can we correctly keep and escape valid chars.
	value := "abzABZ019(){c=\"h/v0.0.0 (linux/amd64) k8s/$f\",c=\"200\",ct=\"app/vnd.k8s.p;s=w\",e=\"https\",i=\"10.0.0.0:8443\"},'\"\\"
	expected := "abzABZ019(){c_\"h/v0.0.0_(linux/amd64)_k8s/$f\",c_\"200\",ct_\"app/vnd.k8s.p_s_w\",e_\"https\",i_\"10.0.0.0:8443\"},'\"\\"
	actual := EscapeTagged(value)
	if expected != string(actual) {
		t.Errorf("Expected %s, got %s", expected, actual)
	}

	// Test percent-encoding.
	value = "é/|_;:%."
	expected = "%C3%A9/|__:%."
	actual = EscapeTagged(value)
	if expected != string(actual) {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestUnescape(t *testing.T) {
	// Can we correctly keep and unescape valid chars.
	value := "abzABZ019\\(\\)\\{\\}\\,\\'\\\"\\\\"
	expected := "abzABZ019(){},'\"\\"
	actual := Unescape(value)
	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}

	// Test percent-decoding.
	value = "%C3%A9%2F|_;:%25%2E"
	expected = "é/|_;:%."
	actual = Unescape(value)
	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}
