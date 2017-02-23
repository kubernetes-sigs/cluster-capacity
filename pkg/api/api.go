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

package api

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/apimachinery/pkg/runtime"
)

type ResourceType string

const (
	Pods                   ResourceType = "pods"
	PersistentVolumes      ResourceType = "persistentvolumes"
	ReplicationControllers ResourceType = "replicationcontrollers"
	Nodes                  ResourceType = "nodes"
	Services               ResourceType = "services"
	PersistentVolumeClaims ResourceType = "persistentvolumeclaims"
	ReplicaSets            ResourceType = "replicasets"
	ResourceQuota          ResourceType = "resourcequotas"
	Secrets                ResourceType = "secrets"
	ServiceAccounts        ResourceType = "serviceaccounts"
	LimitRanges            ResourceType = "limitranges"
	Namespaces             ResourceType = "namespaces"
)

func (r ResourceType) String() string {
	return string(r)
}

func (r ResourceType) ObjectType() runtime.Object {
	switch r {
	case "pods":
		return &api.Pod{}
	case "persistentvolumes":
		return &api.PersistentVolume{}
	case "replicationcontrollers":
		return &api.ReplicationController{}
	case "nodes":
		return &api.Node{}
	case "services":
		return &api.Service{}
	case "persistentvolumeclaims":
		return &api.PersistentVolumeClaim{}
	case "replicasets":
		return &extensions.ReplicaSet{}
	case "resourcequotas":
		return &api.ResourceQuota{}
	case "secrets":
		return &api.Secret{}
	case "serviceaccounts":
		return &api.ServiceAccount{}
	case "limitranges":
		return &api.LimitRange{}
	case "namespaces":
		return &api.Namespace{}
	}
	return nil
}

func StringToResourceType(resource string) (ResourceType, error) {
	switch resource {
	case "pods":
		return Pods, nil
	case "persistentvolumes":
		return PersistentVolumes, nil
	case "replicationcontrollers":
		return ReplicationControllers, nil
	case "nodes":
		return Nodes, nil
	case "services":
		return Services, nil
	case "persistentvolumeclaims":
		return PersistentVolumeClaims, nil
	case "replicasets":
		return ReplicaSets, nil
	case "resourcequotas":
		return ResourceQuota, nil
	case "secrets":
		return Secrets, nil
	case "serviceaccounts":
		return ServiceAccounts, nil
	case "limitranges":
		return LimitRanges, nil
	case "namespaces":
		return Namespaces, nil
	default:
		return "", fmt.Errorf("Resource type %v not recognized", resource)
	}
}
