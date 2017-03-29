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

package test

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
)

func NodeExample(name string) api.Node {
	return api.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "123"},
		Spec: api.NodeSpec{
			ExternalID: "ext",
		},
	}
}

func PodExample(name string) api.Pod {
	pod := api.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "10"},
		Spec:       apitesting.DeepEqualSafePodSpec(),
	}
	pod.Spec.Containers = []api.Container{}
	return pod
}

func ServiceExample(name string) api.Service {
	return api.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "12"},
		Spec: api.ServiceSpec{
			SessionAffinity: "None",
			Type:            api.ServiceTypeClusterIP,
		},
	}
}

func ReplicationControllerExample(name string) api.ReplicationController {
	return api.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "18"},
		Spec: api.ReplicationControllerSpec{
			Replicas: 1,
		},
	}
}
func PersistentVolumeExample(name string) api.PersistentVolume {
	return api.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: name, ResourceVersion: "123"},
		Spec: api.PersistentVolumeSpec{
			Capacity: api.ResourceList{
				api.ResourceName(api.ResourceStorage): resource.MustParse("10G"),
			},
			PersistentVolumeSource: api.PersistentVolumeSource{
				HostPath: &api.HostPathVolumeSource{Path: "/foo"},
			},
			PersistentVolumeReclaimPolicy: "Retain",
		},
		Status: api.PersistentVolumeStatus{
			Phase: api.PersistentVolumePhase("Pending"),
		},
	}
}

func PersistentVolumeClaimExample(name string) api.PersistentVolumeClaim {
	return api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "123"},
		Spec: api.PersistentVolumeClaimSpec{
			VolumeName: "volume",
		},
		Status: api.PersistentVolumeClaimStatus{
			Phase: api.PersistentVolumeClaimPhase("Pending"),
		},
	}
}
