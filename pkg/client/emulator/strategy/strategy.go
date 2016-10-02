package strategy

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	"fmt"
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
}

func (s *predictiveStrategy) addPod(pod *api.Pod) error {
	// 1. get node the pod is scheduled on
	node := &api.Node{
		ObjectMeta: api.ObjectMeta{Name: pod.Spec.NodeName},
	}
	scheduledNode, exists, err := s.resourceStore.Get("nodes", meta.Object(node))
	if err != nil {
		return fmt.Errorf("Unable to get node: %v", err)
	}
	if !exists {
		return fmt.Errorf("Unable to find scheduled pod's node")
	}

	// 2. update the node info to include new pod's resources

	// 3. reflect new pod and update node in the resource store
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
