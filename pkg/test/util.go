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
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
)

func NodeExample(name string) v1.Node {
	return v1.Node{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "123"},
		Spec: v1.NodeSpec{
			ExternalID: "ext",
		},
	}
}

func PodExample(name string) v1.Pod {
	return v1.Pod{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "10"},
		Spec:       apitesting.DeepEqualSafePodSpec(),
	}
}

func ServiceExample(name string) v1.Service {
	return v1.Service{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "12"},
		Spec: v1.ServiceSpec{
			SessionAffinity: "None",
			Type:            v1.ServiceTypeClusterIP,
		},
	}
}

func ReplicationControllerExample(name string) v1.ReplicationController {
	return v1.ReplicationController{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "18"},
		Spec: v1.ReplicationControllerSpec{
			Replicas: 1,
		},
	}
}
func PersistentVolumeExample(name string) v1.PersistentVolume {
	return v1.PersistentVolume{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: name, ResourceVersion: "123"},
		Spec: v1.PersistentVolumeSpec{
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): resource.MustParse("10G"),
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{Path: "/foo"},
			},
			PersistentVolumeReclaimPolicy: "Retain",
		},
		Status: v1.PersistentVolumeStatus{
			Phase: v1.PersistentVolumePhase("Pending"),
		},
	}
}

func PersistentVolumeClaimExample(name string) v1.PersistentVolumeClaim {
	return v1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "123"},
		Spec: v1.PersistentVolumeClaimSpec{
			VolumeName: "volume",
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: v1.PersistentVolumeClaimPhase("Pending"),
		},
	}
}
