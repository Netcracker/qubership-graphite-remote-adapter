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
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/Netcracker/qubership-graphite-remote-adapter/ui"
)

func getTemplate(name string) (string, error) {
	baseTmpl, err := ui.Asset("templates/_base.html")
	if err != nil {
		return "", fmt.Errorf("error reading base template: %s", err)
	}
	pageTmpl, err := ui.Asset(filepath.Join("templates", name))
	if err != nil {
		return "", fmt.Errorf("error reading page template %s: %s", name, err)
	}
	return string(baseTmpl) + string(pageTmpl), nil
}

// ExecuteTemplate renders template for given name with provided data.
func ExecuteTemplate(name string, data interface{}) ([]byte, error) {
	text, err := getTemplate(name)
	if err != nil {
		return nil, err
	}

	tmpl := template.New(name).Funcs(TmplFuncMap)
	tmpl, err = tmpl.Parse(text)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, data)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
