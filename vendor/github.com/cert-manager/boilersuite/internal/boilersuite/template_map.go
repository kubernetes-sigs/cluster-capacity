/*
Copyright 2023 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package boilersuite

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"
)

type TemplateMap map[string]BoilerplateTemplate

// LoadTemplates attempts to read all of the templates under the given embedded filesystem
// and return a TemplateMap which can be used for fetching templates later.
func LoadTemplates(templateDir embed.FS, expectedAuthor string) (TemplateMap, error) {
	allEntries, err := templateDir.ReadDir("boilerplate-templates")
	if err != nil {
		return nil, fmt.Errorf("failed to read templates: %s", err.Error())
	}

	if len(allEntries) == 0 {
		return nil, fmt.Errorf("found no templates in template dir")
	}

	out := make(TemplateMap)

	for _, entry := range allEntries {
		name := entry.Name()
		path := filepath.Join("boilerplate-templates", name)

		trimmedName := strings.TrimSuffix(name, ".boilertmpl")

		target := strings.TrimPrefix(filepath.Ext(trimmedName), ".")

		contents, err := templateDir.ReadFile(path)
		if err != nil {
			// if files were embedded properly, shouldn't fail to read
			return nil, fmt.Errorf("failed to read %q: %s", path, err.Error())
		}

		var normalizationFunc func(string) string

		if target == "go" {
			normalizationFunc = normalizeGoFile
		} else if target == "sh" || target == "bash" || target == "py" {
			normalizationFunc = normalizeShebang
		}

		out[target], err = NewBoilerplateTemplate(string(contents), BoilerplateTemplateConfiguration{
			ExpectedAuthor:    expectedAuthor,
			NormalizationFunc: normalizationFunc,
		})
		if err != nil {
			// all templates should be valid before embedding
			return nil, fmt.Errorf("invalid template %q: %s", path, err.Error())
		}
	}

	return out, nil
}

// TemplateMap returns a template which matches the given name, if one exists in the map.
func (tm TemplateMap) TemplateFor(path string) (BoilerplateTemplate, bool) {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")

	tmpl, ok := tm[ext]

	if ok {
		return tmpl, true
	}

	// might be a prefix type (e.g. Dockerfile.abc should match the "Dockerfile" template)

	base := filepath.Base(path)
	name := strings.SplitN(base, ".", 2)[0]

	tmpl, ok = tm[name]

	if ok {
		return tmpl, true
	}

	return BoilerplateTemplate{}, false
}
