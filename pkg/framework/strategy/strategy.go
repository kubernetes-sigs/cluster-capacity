package strategy

import (
	"fmt"

	"github.com/ingvagabund/cluster-capacity/pkg/framework/store"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

type Strategy interface {
	// Add new objects
	Add(obj interface{}) error

	// Update objects
	Update(obj interface{}) error

	// Delete objects
	Delete(obj interface{}) error
}

type predictiveStrategy struct {
	resourceStore store.ResourceStore

	// for each node keep its NodeInfo
	nodeInfo map[string]*schedulercache.NodeInfo
}

func (s *predictiveStrategy) addPod(pod *api.Pod) error {
	// No need to update any node.
	// The scheduler keep resources consumed by all pods in its scheduler cache
	// which is than confronted with pod's node Allocatable field.

	// mark the pod as running rather than keeping the phase empty
	pod.Status.Phase = api.PodRunning

	// here asuming the pod is already in the resource storage
	// so the update is needed to emit update event in case a handler is registered
	err := s.resourceStore.Update("pods", meta.Object(pod))
	if err != nil {
		return fmt.Errorf("Unable to add new node: %v", err)
	}

	return nil
}

// Simulate creation of new object (only pods currently supported)
// The method returns error on the first occurence of processing error.
// If so, all succesfully processed objects up to the first failed are reflected in the resource store.
func (s *predictiveStrategy) Add(obj interface{}) error {
	switch item := obj.(type) {
	case *api.Pod:
		return s.addPod(item)
	default:
		return fmt.Errorf("resource kind not recognized")
	}
}

func (s *predictiveStrategy) Update(obj interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func (s *predictiveStrategy) Delete(obj interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func NewPredictiveStrategy(resourceStore store.ResourceStore) *predictiveStrategy {
	return &predictiveStrategy{
		resourceStore: resourceStore,
	}
}
