// Copyright 2018 Thibault Chataigner <thibault.chataigner@gmail.com>
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
	"bytes"
	"testing"
	"text/template"
)

var hostWithPort = "hostbase-otherpart1234:4567"
var hostWithNumbers = "long-hostname-machine01234"

func Test_aTemplateCanSplitStringArray(t *testing.T) {

	tmpl, err := template.New("test").Funcs(TmplFuncMap).Parse(`{{ index ( split . ":" ) 0}}`)
	if err != nil {
		t.Errorf("error parsing template: %v", err)
	}

	buf := bytes.NewBufferString("")
	if err = tmpl.Execute(buf, hostWithPort); err != nil {
		t.Errorf("error executing template: %v", err)
	}

	if buf.String() != "hostbase-otherpart1234" {
		t.Errorf("split function not properly implemented or template misconfigured")
	}
}

func Test_aTemplateCanReplaceRegex(t *testing.T) {
	tmpl, err := template.New("test").Funcs(TmplFuncMap).Parse("{{ replaceRegex . `^([a-z_\\-]*)[0-9]*$` `$1` }}")
	if err != nil {
		t.Errorf("error parsing template: %v", err)
	}

	buf := bytes.NewBufferString("")
	if err = tmpl.Execute(buf, hostWithNumbers); err != nil {
		t.Errorf("error executing template: %v", err)
	}
	actual := buf.String()
	if actual != "long-hostname-machine" {
		t.Errorf("replaceRegex function not properly implemented or template misconfigured: result %s", actual)
	}
}
