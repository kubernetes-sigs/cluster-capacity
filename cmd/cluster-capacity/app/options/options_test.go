/*
Copyright 2017 The Kubernetes Authors.

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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	internal "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/v1"
)

func encodeOrDie(obj runtime.Object) []byte {
	data, err := runtime.Encode(internal.Codecs.LegacyCodec(v1.SchemeGroupVersion), obj)
	if err != nil {
		panic(err.Error())
	}
	return data
}

func getV1Pod(podName, resourceVersion string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: "test", ResourceVersion: resourceVersion},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "container1",
					Image: "fake",
				},
			},
		},
	}
}

func TestParseAPISpec(t *testing.T) {
	var tmpDir string
	var err error
	if tmpDir, err = ioutil.TempDir(os.TempDir(), "pods-spec"); err != nil {
		t.Fatalf("error creating test dir: %#v", err)
	}
	defer os.RemoveAll(tmpDir)

	testCases := []struct {
		name       string
		spec       runtime.Object
		expNumPods int
		expError   bool
	}{
		{
			name: "PodList",
			spec: &v1.PodList{
				Items: []v1.Pod{
					*getV1Pod("pod1", "1"),
					*getV1Pod("pod2", "2"),
				},
			},
			expNumPods: 2,
		},
		{
			name:       "SinglePod",
			spec:       getV1Pod("pod1", "1"),
			expNumPods: 1,
		},
		{
			name: "NotOnlyPods",
			spec: &v1.List{
				Items: []runtime.RawExtension{
					{Raw: encodeOrDie(getV1Pod("pod", "1"))},
					{Raw: encodeOrDie(
						&v1.ReplicationController{
							ObjectMeta: metav1.ObjectMeta{Name: "rc", ResourceVersion: "7"},
						},
					)},
				},
			},
			expError: true,
		},
	}
	for i, tc := range testCases {
		t.Logf("Test case: %v\n", tc.name)

		tmpFile := filepath.Join(tmpDir, fmt.Sprintf("pods-spec-%d.yaml", i))
		if err = ioutil.WriteFile(tmpFile, []byte(runtime.EncodeOrDie(testapi.Default.Codec(), tc.spec)), 0644); err != nil {
			t.Fatalf("error creating test file %#v", err)
		}
		c := &ClusterCapacityConfig{Options: &ClusterCapacityOptions{PodSpecFile: tmpFile}}
		err = c.ParseAPISpec()
		if err != nil && !tc.expError {
			t.Error(err)
		}
		if a, e := len(c.Pods), tc.expNumPods; a != e {
			t.Errorf("Expected %d pods but got %d", e, a)
		}
	}
}
