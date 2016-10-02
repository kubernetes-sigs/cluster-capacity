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
//	resourceStore store.ResourceStore
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
//func NewPredictiveStrategy(resourceStore store.ResourceStore) Strategy {
//	return &predictiveStrategy{
//		resourceStore: resourceStore,
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
	if err := c.resourceStore.Add("pods", pod); err != nil {
		return fmt.Errorf("Unable to store pod: %v", err)
	}

	// emit watch event
	return c.restClient.EmitPodWatchEvent(watch.Added, pod)
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

func (c *ClientEmulator) Run() {
	// 1. sync store with cluster
	// 2. init rest client (lister part)
	// 3. register rest client to resource store (watch part)
	// 4. init strategy (pod -> (As,Us,Ds)
	// 5. bake scheduler with the rest client
	// 6. init predictive loop
	// 6.1. schedule pod or get error
	// 6.2. run strategy, get items to add/update/delete (e.g. new pod, new volume, updated node info) or error (e.g max. number of PD exceeded)
	// 6.3. update resource store
}

func NewClientEmulator() *ClientEmulator {
	resourceStore := store.NewResourceStore()
	restClient := NewRESTClient(resourceStore)

	client := &ClientEmulator{
		resourceStore: resourceStore,
		restClient: restClient,
	}

	resourceStore.RegisterEventHandler("pods", cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			restClient.EmitPodWatchEvent(watch.Added, obj.(*api.Pod))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			restClient.EmitPodWatchEvent(watch.Modified, newObj.(*api.Pod))
		},
		DeleteFunc: func(obj interface{}) {
			restClient.EmitPodWatchEvent(watch.Deleted, obj.(*api.Pod))
		},
	})

	resourceStore.RegisterEventHandler("services", cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			restClient.EmitServiceWatchEvent(watch.Added, obj.(*api.Service))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			restClient.EmitServiceWatchEvent(watch.Modified, newObj.(*api.Service))
		},
		DeleteFunc: func(obj interface{}) {
			restClient.EmitServiceWatchEvent(watch.Deleted, obj.(*api.Service))
		},
	})

	resourceStore.RegisterEventHandler("replicationControllers", cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			restClient.EmitReplicationControllerWatchEvent(watch.Added, obj.(*api.ReplicationController))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			restClient.EmitReplicationControllerWatchEvent(watch.Modified, newObj.(*api.ReplicationController))
		},
		DeleteFunc: func(obj interface{}) {
			restClient.EmitReplicationControllerWatchEvent(watch.Deleted, obj.(*api.ReplicationController))
		},
	})

	return client
}
