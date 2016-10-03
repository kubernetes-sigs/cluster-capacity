package emulator

import (
	"fmt"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/watch"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/strategy"
	ccapi "github.com/ingvagabund/cluster-capacity/pkg/api"
)

type ClientEmulator struct {
	// caches modified by emulation strategy
	resourceStore store.ResourceStore

	// emulation strategy
	strategy *strategy.Strategy

	// fake rest client
	restClient *RESTClient
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

	resourceStore.RegisterEventHandler(ccapi.Pods, cache.ResourceEventHandlerFuncs{
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

	resourceStore.RegisterEventHandler(ccapi.Services, cache.ResourceEventHandlerFuncs{
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

	resourceStore.RegisterEventHandler(ccapi.ReplicationControllers, cache.ResourceEventHandlerFuncs{
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

	resourceStore.RegisterEventHandler(ccapi.PersistentVolumes, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			restClient.EmitPersistentVolumeWatchEvent(watch.Added, obj.(*api.PersistentVolume))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			restClient.EmitPersistentVolumeWatchEvent(watch.Modified, newObj.(*api.PersistentVolume))
		},
		DeleteFunc: func(obj interface{}) {
			restClient.EmitPersistentVolumeWatchEvent(watch.Deleted, obj.(*api.PersistentVolume))
		},
	})

	resourceStore.RegisterEventHandler(ccapi.Nodes, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			restClient.EmitNodeWatchEvent(watch.Added, obj.(*api.Node))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			restClient.EmitNodeWatchEvent(watch.Modified, newObj.(*api.Node))
		},
		DeleteFunc: func(obj interface{}) {
			restClient.EmitNodeWatchEvent(watch.Deleted, obj.(*api.Node))
		},
	})

	resourceStore.RegisterEventHandler(ccapi.PersistentVolumeClaims, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			restClient.EmitPersistentVolumeClaimWatchEvent(watch.Added, obj.(*api.PersistentVolumeClaim))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			restClient.EmitPersistentVolumeClaimWatchEvent(watch.Modified, newObj.(*api.PersistentVolumeClaim))
		},
		DeleteFunc: func(obj interface{}) {
			restClient.EmitPersistentVolumeClaimWatchEvent(watch.Deleted, obj.(*api.PersistentVolumeClaim))
		},
	})

	return client
}
