/*
Copyright 2025 The Kubernetes Authors.

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

package version

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    Info
	}{
		{
			name:    "parses automated container release tag",
			version: "v20240519-v0.30.0",
			want: Info{
				Major:      "0",
				Minor:      "30.0",
				GitVersion: "v20240519-v0.30.0",
			},
		},
		{
			name:    "parses automated container build",
			version: "v20240520-v0.30.0-5-g79990946",
			want: Info{
				Major:      "0",
				Minor:      "30.0",
				GitVersion: "v20240520-v0.30.0-5-g79990946",
			},
		},
		{
			name:    "parses helm release tag",
			version: "v20240606-descheduler-helm-chart-0.30.0-18-g8714397b",
			want: Info{
				Major:      "0",
				Minor:      "30.0",
				GitVersion: "v20240606-descheduler-helm-chart-0.30.0-18-g8714397b",
			},
		},
	}
	ignoreRuntimeFields := cmpopts.IgnoreFields(Info{}, "GoVersion", "Compiler", "Platform")
	for _, tt := range tests {
		version = tt.version
		t.Run(tt.name, func(t *testing.T) {
			got := Get()
			if diff := cmp.Diff(got, tt.want, ignoreRuntimeFields); diff != "" {
				t.Errorf("Get (-want, +got):\n%s", diff)
			}
		})
	}
}
