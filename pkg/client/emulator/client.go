package emulator

import (
	"fmt"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/watch"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// TODO(jchaloup,hodovska): currently, only scheduler caches are considered.
// Later, include cache for each object.
type caches struct {
	// Pod cache modifed by emulation strategy
	PodCache cache.Store

	// Node cache modifed by emulation strategy
	NodeCache cache.Store

	// PVC cache modifed by emulation strategy
	PVCCache cache.Store

	// PV cache modifed by emulation strategy
	PVCache cache.Store

	// Service cache modifed by emulation strategy
	ServiceCache cache.Store

	// RC cache modifed by emulation strategy
	ReplicationControllerCache cache.Store

	// RS cache modifed by emulation strategy
	ReplicaSetCache cache.Store
}

type Strategy interface {
	// Add new objects
	Add(objs []interface{}) error

	// Update objects
	Update(objs []interface{}) error

	// Delete objects
	Delete(objs []interface{}) error
}

type predictiveStrategy struct {
	caches *caches
}

func NewPredictiveStrategy(c *caches) Strategy {
	return &predictiveStrategy{
		caches: c,
	}
}

func NewClientEmulator() ClientEmulator {
	caches := &caches{
		PodCache:                   cache.NewStore(cache.MetaNamespaceKeyFunc),
		NodeCache:                  cache.NewStore(cache.MetaNamespaceKeyFunc),
		PVCache:                    cache.NewStore(cache.MetaNamespaceKeyFunc),
		PVCCache:                   cache.NewStore(cache.MetaNamespaceKeyFunc),
		ServiceCache:               cache.NewStore(cache.MetaNamespaceKeyFunc),
		ReplicaSetCache:            cache.NewStore(cache.MetaNamespaceKeyFunc),
		ReplicationControllerCache: cache.NewStore(cache.MetaNamespaceKeyFunc),
	}

	resourceToCache := map[string]cache.Store{
		"pods":                   caches.PodCache,
		//"node":                   caches.NodeCache,
		//"persistentVolumes":      caches.PVCache,
		//"persistentVolumeClaims": caches.PVCCache,
		"services":               caches.ServiceCache,
		//"replicasets":            caches.ServiceCache,
		"replicationControllers": caches.ReplicationControllerCache,
	}
	return ClientEmulator{
		caches:          caches,
		resourceToCache: resourceToCache,
	}
}
func (*predictiveStrategy) Add(obj []interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func (*predictiveStrategy) Update(objs []interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

func (*predictiveStrategy) Delete(objs []interface{}) error {
	return fmt.Errorf("Not implemented yet")
}

type ClientEmulator struct {
	// caches modified by emulation strategy
	*caches

	// watch events emulator
	watcher *watch.FakeWatcher

	// emulation strategy
	strategy *Strategy

	resourceToCache map[string]cache.Store
}

func storeItems(lw *cache.ListWatch, store cache.Store) error {
	options := api.ListOptions{ResourceVersion: "0"}
	list, err := lw.List(options)
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
	err = store.Replace(found, resourceVersion)
	if err != nil {
		return fmt.Errorf("Unable to store list result: %v", err)
	}
	return nil
}

func (c *ClientEmulator) sync(client cache.Getter) error {
	for resource, objectCache := range c.resourceToCache {
		listWatcher := cache.NewListWatchFromClient(client, resource, api.NamespaceAll, fields.ParseSelectorOrDie(""))
		if err := storeItems(listWatcher, objectCache); err != nil {
			return fmt.Errorf("Unable to sync %s: %v", resource, err)
		}
	}
	return nil
}

func (c *ClientEmulator) RESTClient() *RESTClient {
	return &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
		// define methods for ListerWatcher's List()
		podsDataSource: func() *api.PodList {
			items := c.caches.PodCache.List()
			podItems := make([]api.Pod, 0, len(items))
			for _, item := range items {
				podItems = append(podItems, item.(api.Pod))
			}

			return &api.PodList{
				ListMeta: unversioned.ListMeta{
					// choose arbitrary value as the cache does not store the ResourceVersion
					ResourceVersion: "0",
				},
				Items: podItems,
			}
		},
		servicesDataSource: func() *api.ServiceList {
			items := c.caches.ServiceCache.List()
			serviceItems := make([]api.Service, 0, len(items))
			for _, item := range items {
				serviceItems = append(serviceItems, item.(api.Service))
			}

			return &api.ServiceList{
				ListMeta: unversioned.ListMeta{
					// choose arbitrary value as the cache does not store the ResourceVersion
					ResourceVersion: "0",
				},
				Items: serviceItems,
			}

		},
		replicationControllersDataSource: func() *api.ReplicationControllerList {
			items := c.caches.ReplicationControllerCache.List()
			rcItems := make([]api.ReplicationController, 0, len(items))
			for _, item := range items {
				rcItems = append(rcItems, item.(api.ReplicationController))
			}

			return &api.ReplicationControllerList{
				ListMeta: unversioned.ListMeta{
					// choose arbitrary value as the cache does not store the ResourceVersion
					ResourceVersion: "0",
				},
				Items: rcItems,
			}
		},
		// TODO(jchaloup): add methods for missing resources
	}
}
