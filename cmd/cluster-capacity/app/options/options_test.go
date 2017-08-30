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
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
)

func encodeOrDie(obj runtime.Object) []byte {
	data, err := runtime.Encode(internal.Codecs.LegacyCodec(v1.SchemeGroupVersion, extensions.SchemeGroupVersion), obj)
	if err != nil {
		panic(err.Error())
	}
	return data
}

func newPodSpec() *v1.PodSpec {
	return &v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "container1",
				Image: "fake",
			},
		},
	}
}

func getV1Pod(podName, resourceVersion string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: "test", ResourceVersion: resourceVersion},
		Spec:       *newPodSpec(),
	}
}

func newReplicationController(replicas int) *v1.ReplicationController {
	rc := &v1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "foobar",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "18",
		},
		Spec: v1.ReplicationControllerSpec{
			Replicas: func() *int32 { i := int32(replicas); return &i }(),
			Selector: map[string]string{"foo": "bar"},
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "foo",
						"type": "production",
					},
				},
				Spec: *newPodSpec(),
			},
		},
	}
	return rc
}

func newReplicaSet(replicas int) *extensions.ReplicaSet {
	return &extensions.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "rs-1",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "18",
		},
		Spec: extensions.ReplicaSetSpec{
			Replicas: func() *int32 { i := int32(replicas); return &i }(),
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "foo",
						"type": "production",
					},
				},
				Spec: *newPodSpec(),
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
		name           string
		spec           runtime.Object
		expPodReplicas []int
		expError       bool
	}{
		{
			name: "PodList",
			spec: &v1.PodList{
				Items: []v1.Pod{
					*getV1Pod("pod1", "1"),
					*getV1Pod("pod2", "2"),
				},
			},
			expPodReplicas: []int{1, 1},
		},
		{
			name:           "SinglePod",
			spec:           getV1Pod("pod1", "1"),
			expPodReplicas: []int{1},
		},
		{
			name: "ReplicationController",
			spec: &v1.List{
				Items: []runtime.RawExtension{
					{Raw: encodeOrDie(getV1Pod("pod", "1"))},
					{Raw: encodeOrDie(newReplicationController(3))},
				},
			},
			expError:       false,
			expPodReplicas: []int{1, 3},
		},
		{
			name: "ReplicaSet",
			spec: &v1.List{
				Items: []runtime.RawExtension{
					{Raw: encodeOrDie(getV1Pod("pod", "1"))},
					{Raw: encodeOrDie(newReplicaSet(4))},
				},
			},
			expError:       false,
			expPodReplicas: []int{1, 4},
		},
		{
			name: "ZeroReplicas",
			spec: &v1.List{
				Items: []runtime.RawExtension{
					{Raw: encodeOrDie(newReplicationController(0))},
					{Raw: encodeOrDie(newReplicaSet(0))},
				},
			},
			expError:       false,
			expPodReplicas: []int{0, 0},
		},
		{
			name: "NotSupportedType",
			spec: &v1.List{
				Items: []runtime.RawExtension{
					{Raw: encodeOrDie(&v1.Service{})},
				},
			},
			expError: true,
		},
	}
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := filepath.Join(tmpDir, fmt.Sprintf("pods-spec-%d.yaml", i))
			if err = ioutil.WriteFile(tmpFile, []byte(runtime.EncodeOrDie(testapi.Default.Codec(), tc.spec)), 0644); err != nil {
				t.Fatalf("error creating test file %#v", err)
			}
			c := &ClusterCapacityConfig{Options: &ClusterCapacityOptions{PodSpecFile: tmpFile}}
			err = c.ParseAPISpec()
			if err != nil && !tc.expError {
				t.Error(err)
			}
			if a, e := len(c.Pods), len(tc.expPodReplicas); a != e {
				t.Errorf("Expected %d pod specs but got %d", e, a)
			}
			for i, r := range tc.expPodReplicas {
				if len(c.Pods) > i {
					if a, e := int(c.Pods[i].Replicas), r; a != e {
						t.Errorf("Expected %d replicas for pod spec %d but got %d", e, i, a)
					}
				}
			}
		})
	}
}
