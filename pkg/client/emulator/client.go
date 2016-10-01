package emulator

import (
	"fmt"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/watch"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
)

//type Strategy interface {
//	// Add new objects
//	Add(objs []interface{}) error
//
//	// Update objects
//	Update(objs []interface{}) error
//
//	// Delete objects
//	Delete(objs []interface{}) error
//}
//
//type predictiveStrategy struct {
//	caches *caches
//}
//
//func (*predictiveStrategy) Add(obj []interface{}) error {
//	return fmt.Errorf("Not implemented yet")
//}
//
//func (*predictiveStrategy) Update(objs []interface{}) error {
//	return fmt.Errorf("Not implemented yet")
//}
//
//func (*predictiveStrategy) Delete(objs []interface{}) error {
//	return fmt.Errorf("Not implemented yet")
//}
//
//func NewPredictiveStrategy(c *caches) Strategy {
//	return &predictiveStrategy{
//		caches: c,
//	}
//}

type ClientEmulator struct {
	// caches modified by emulation strategy
	resourceStore store.ResourceStore

	// emulation strategy
	//strategy *Strategy

	// fake rest client
	restClient *RESTClient
}

// Add pod resource object to a cache
func (c *ClientEmulator) AddPod(pod *api.Pod) error {
	// add pod to cache

	// emit watch event
	c.restClient.EmitPodWatchEvent(watch.Added, pod)
	return nil
}

func (c *ClientEmulator) sync(client cache.Getter) error {

	for _, resource := range c.resourceStore.Resources() {
		listWatcher := cache.NewListWatchFromClient(client, resource, api.NamespaceAll, fields.ParseSelectorOrDie(""))

		options := api.ListOptions{ResourceVersion: "0"}
		list, err := listWatcher.List(options)
		if err != nil {
			return fmt.Errorf("Failed to list objects: %v", err)
		}

		listMetaInterface, err := meta.ListAccessor(list)
		if err != nil {
			return fmt.Errorf("Unable to understand list result %#v: %v", list, err)
		}
		resourceVersion := listMetaInterface.GetResourceVersion()

		items, err := meta.ExtractList(list)
		if err != nil {
			return fmt.Errorf("Unable to understand list result %#v (%v)", list, err)
		}
		found := make([]interface{}, 0, len(items))
		for _, item := range items {
			found = append(found, item)
		}
		err = c.resourceStore.Replace(resource, found, resourceVersion)
		if err != nil {
			return fmt.Errorf("Unable to store %s list result: %v", resource, err)
		}
	}
	return nil
}

func NewClientEmulator() *ClientEmulator {
	resourceStore := store.NewResourceStore()

	client := &ClientEmulator{
		resourceStore: resourceStore,
		restClient: NewRESTClient(resourceStore),
	}

	return client
}
