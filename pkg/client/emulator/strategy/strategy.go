package strategy

import (
	"k8s.io/kubernetes/pkg/api"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	"fmt"
)

type Strategy interface {
	// Add new objects
	Add(objs []interface{}) error

	// Update objects
	Update(objs []interface{}) error

	// Delete objects
	Delete(objs []interface{}) error
}

type predictiveStrategy struct {
	resourceStore store.ResourceStore
}

func (*predictiveStrategy) addPod(pod *api.Pod) error {
	// 1. get node the pod is scheduled on

	// 2. update the node info to include new pod's resources

	// 3. reflect new pod and update node in the resource store
	return nil
}

// Simulate creation of new object (only pods currently supported)
func (*predictiveStrategy) Add(obj []interface{}) error {
	return nil
}

func (*predictiveStrategy) Update(objs []interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func (*predictiveStrategy) Delete(objs []interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func NewPredictiveStrategy(resourceStore store.ResourceStore) *predictiveStrategy {
	return &predictiveStrategy{
		resourceStore: resourceStore,
	}
}
