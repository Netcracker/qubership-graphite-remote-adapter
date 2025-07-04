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

package test

import (
	"net/http"
	"reflect"
	"testing"

	graphiteClient "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite"
	graphiteCfg "github.com/Netcracker/qubership-graphite-remote-adapter/client/graphite/config"
	"github.com/go-kit/log"
)

var (
	testClient = graphiteClient.NewClientGraphiteCfg(
		&graphiteCfg.Config{
			DefaultPrefix: "prometheus-prefix.",
			Write:         graphiteCfg.WriteConfig{},
			Read: graphiteCfg.ReadConfig{
				URL: "http://testHost:6666",
			},
		},
		log.NewNopLogger())
)

func TestGetGraphitePrefix(t *testing.T) {
	TestRequest, _ := http.NewRequest("POST", "http://testHost:6666", nil)
	expectedPrefix := testClient.Cfg().DefaultPrefix

	actualPrefix := testClient.Cfg().StoragePrefixFromRequest(TestRequest)
	if !reflect.DeepEqual(expectedPrefix, actualPrefix) {
		t.Errorf("Expected %s, got %s", expectedPrefix, actualPrefix)
	}
}

func TestGetCustomGraphitePrefix(t *testing.T) {
	TestRequest, _ := http.NewRequest("POST", "http://testHost:6666?graphite.default-prefix=foo.bar.custom.", nil)
	expectedPrefix := "foo.bar.custom."

	actualPrefix := testClient.Cfg().StoragePrefixFromRequest(TestRequest)
	if !reflect.DeepEqual(expectedPrefix, actualPrefix) {
		t.Errorf("Expected %s, got %s", expectedPrefix, actualPrefix)
	}
}
