/*
Copyright 2026 The Kubernetes Authors.

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

package options

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestParseAPISpecImagePullPolicy(t *testing.T) {
	const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name      string
		image     string
		policy    v1.PullPolicy
		want      v1.PullPolicy
		wantError bool
	}{
		{name: "untagged", image: "busybox", want: v1.PullAlways},
		{name: "latest", image: "busybox:latest", want: v1.PullAlways},
		{name: "versioned", image: "busybox:1.36", want: v1.PullIfNotPresent},
		{name: "digest", image: "busybox@" + digest, want: v1.PullIfNotPresent},
		{name: "latest with digest", image: "busybox:latest@" + digest, want: v1.PullAlways},
		{name: "explicit never", image: "busybox:latest", policy: v1.PullNever, want: v1.PullNever},
		{name: "explicit always", image: "busybox:1.36", policy: v1.PullAlways, want: v1.PullAlways},
		{name: "explicit if not present", image: "busybox:latest", policy: v1.PullIfNotPresent, want: v1.PullIfNotPresent},
		{name: "invalid policy", image: "busybox", policy: "invalid", wantError: true},
	}
	for _, format := range []string{"json", "yaml"} {
		for _, tt := range tests {
			t.Run(format+"/"+tt.name, func(t *testing.T) {
				pod := &v1.Pod{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
					Spec: v1.PodSpec{
						InitContainers: []v1.Container{{Name: "init", Image: tt.image, ImagePullPolicy: tt.policy}},
						Containers:     []v1.Container{{Name: "main", Image: tt.image, ImagePullPolicy: tt.policy}},
					},
				}
				data, err := json.Marshal(pod)
				if err != nil {
					t.Fatal(err)
				}
				if format == "yaml" {
					data, err = yaml.JSONToYAML(data)
					if err != nil {
						t.Fatal(err)
					}
				}
				filename := filepath.Join(t.TempDir(), "pod."+format)
				if err := os.WriteFile(filename, data, 0600); err != nil {
					t.Fatal(err)
				}
				config := NewClusterCapacityConfig(&ClusterCapacityOptions{PodSpecFile: filename})
				err = config.ParseAPISpec("test-scheduler")
				if tt.wantError {
					if err == nil || !strings.Contains(err.Error(), "imagePullPolicy") {
						t.Fatalf("expected imagePullPolicy validation error, got %v", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("ParseAPISpec failed: %v", err)
				}
				for _, container := range []v1.Container{config.Pod.Spec.InitContainers[0], config.Pod.Spec.Containers[0]} {
					if container.TerminationMessagePolicy != v1.TerminationMessageReadFile {
						t.Errorf("%s terminationMessagePolicy = %q, want File", container.Name, container.TerminationMessagePolicy)
					}
					if container.ImagePullPolicy != tt.want {
						t.Errorf("%s imagePullPolicy = %q, want %q", container.Name, container.ImagePullPolicy, tt.want)
					}
				}
				if config.Pod.Spec.TerminationGracePeriodSeconds == nil || *config.Pod.Spec.TerminationGracePeriodSeconds != 30 {
					t.Errorf("terminationGracePeriodSeconds = %v, want 30", config.Pod.Spec.TerminationGracePeriodSeconds)
				}
				if config.Pod.Namespace != "default" || config.Pod.Spec.SchedulerName != "test-scheduler" {
					t.Errorf("unexpected namespace/scheduler: %q/%q", config.Pod.Namespace, config.Pod.Spec.SchedulerName)
				}
			})
		}
	}
}

func TestParseAPISpecPreservesExplicitValues(t *testing.T) {
	const spec = `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: custom-namespace
spec:
  schedulerName: custom-scheduler
  restartPolicy: Never
  dnsPolicy: Default
  terminationGracePeriodSeconds: 0
  containers:
  - name: main
    image: busybox:latest
    imagePullPolicy: Never
    terminationMessagePolicy: FallbackToLogsOnError
`
	filename := filepath.Join(t.TempDir(), "pod.yaml")
	if err := os.WriteFile(filename, []byte(spec), 0600); err != nil {
		t.Fatal(err)
	}
	config := NewClusterCapacityConfig(&ClusterCapacityOptions{PodSpecFile: filename})
	if err := config.ParseAPISpec("test-scheduler"); err != nil {
		t.Fatal(err)
	}
	pod := config.Pod
	if pod.Namespace != "custom-namespace" || pod.Spec.SchedulerName != "custom-scheduler" {
		t.Errorf("namespace/scheduler changed: %q/%q", pod.Namespace, pod.Spec.SchedulerName)
	}
	if pod.Spec.RestartPolicy != v1.RestartPolicyNever || pod.Spec.DNSPolicy != v1.DNSDefault {
		t.Errorf("restart/DNS policy changed: %q/%q", pod.Spec.RestartPolicy, pod.Spec.DNSPolicy)
	}
	if pod.Spec.TerminationGracePeriodSeconds == nil || *pod.Spec.TerminationGracePeriodSeconds != 0 {
		t.Errorf("terminationGracePeriodSeconds changed: %v", pod.Spec.TerminationGracePeriodSeconds)
	}
	container := pod.Spec.Containers[0]
	if container.ImagePullPolicy != v1.PullNever || container.TerminationMessagePolicy != v1.TerminationMessageFallbackToLogsOnError {
		t.Errorf("pull/termination message policy changed: %q/%q", container.ImagePullPolicy, container.TerminationMessagePolicy)
	}
}
