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
	"regexp"
)

var (
	// YearMarkerRegex matches the marker which should appear in boilerplate sample files but not in actual files
	YearMarkerRegex = regexp.MustCompile(`<<YEAR>>`)

	// AuthorMarkerRegex matches the marker which should appear in boilerplate sample files but not in actual files
	AuthorMarkerRegex = regexp.MustCompile(`<<AUTHOR>>`)

	// DateRegex matches the actual date found inside a file
	DateRegex = regexp.MustCompile(`Copyright 20\d\d`)

	// BuildConstraintsRegex matches golang build constraints
	BuildConstraintsRegex = regexp.MustCompile(`(?m)^(\/\/(go:build| \+build).*\n)+$`)

	// ShebangRegex matches shebangs in scripts; most shebangs should be on the first line
	// but we use a multiline here to be safe
	ShebangRegex = regexp.MustCompile(`(?m)^#!.*\n`)

	// SkipFileRegex matches files which should not be validated
	SkipFileRegex = regexp.MustCompile(`(?m)^(\/\/|#) \+skip_license_check$`)

	// GeneratedRegex matches comments added by k8s code generators
	GeneratedRegex = regexp.MustCompile(`(?m)^[\/*#]+.*DO NOT EDIT\.$`)
)
