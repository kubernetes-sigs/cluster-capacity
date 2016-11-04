package api

import "fmt"

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
)

func (r ResourceType) String() string {
	return string(r)
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
	default:
		return "", fmt.Errorf("Resource type %v not recognized", resource)
	}
}
