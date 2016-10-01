package store

import (
	"k8s.io/kubernetes/pkg/client/cache"
	"fmt"
)

type ResourceStore interface {
	Add(resource string, obj interface{}) error
	Update(resource string, obj interface{}) error
	Delete(resource string, obj interface{}) error
	List(resource string) []interface{}
	Get(resource string, obj interface{}) (item interface{}, exists bool, err error)
	GetByKey(key string) (item interface{}, exists bool, err error)
	RegisterEventHandler(resource string, handler cache.ResourceEventHandler) error
	// Replace will delete the contents of the store, using instead the
	// given list. Store takes ownership of the list, you should not reference
	// it after calling this function.
	Replace(resource string, items []interface{}, resourceVersion string) error

	Resources() []string
}

// TODO(jchaloup,hodovska): currently, only scheduler caches are considered.
// Later, include cache for each object.
type resourceStore struct {
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

	resourceToCache map[string]cache.Store
	eventHandler map[string]cache.ResourceEventHandler
}

func (s *resourceStore) Add(resource string, obj interface{}) error {
	return nil
}

func (s *resourceStore) Update(resource string, obj interface{}) error {
	return nil
}

func (s *resourceStore) Delete(resource string, obj interface{}) error {
	return nil
}

func (s *resourceStore) List(resource string) []interface{} {
	if cache, exists := s.resourceToCache[resource]; exists {
		return cache.List()
	}
	return nil
}

func (s *resourceStore) Get(resource string, obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (s *resourceStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (s *resourceStore) RegisterEventHandler(resource string, handler cache.ResourceEventHandler) error {
	s.eventHandler[resource] = handler
	return nil
}

// Replace will delete the contents of the store, using instead the
// given list. Store takes ownership of the list, you should not reference
// it after calling this function.
func (s *resourceStore) Replace(resource string, items []interface{}, resourceVersion string) error {
	if cache, exists := s.resourceToCache[resource]; exists {
		return cache.Replace(items, resourceVersion)
	}
	return fmt.Errorf("Resource %s not recognized", resource)
}

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

func (s *resourceStore) Resources() []string {
	keys := make([]string, len(s.resourceToCache))
	for key := range s.resourceToCache {
		keys = append(keys, key)
	}
	return keys
}

func NewResourceStore() *resourceStore {

	resourceStore := &resourceStore{
		PodCache:                   cache.NewStore(cache.MetaNamespaceKeyFunc),
		NodeCache:                  cache.NewStore(cache.MetaNamespaceKeyFunc),
		PVCache:                    cache.NewStore(cache.MetaNamespaceKeyFunc),
		PVCCache:                   cache.NewStore(cache.MetaNamespaceKeyFunc),
		ServiceCache:               cache.NewStore(cache.MetaNamespaceKeyFunc),
		ReplicaSetCache:            cache.NewStore(cache.MetaNamespaceKeyFunc),
		ReplicationControllerCache: cache.NewStore(cache.MetaNamespaceKeyFunc),
		eventHandler: make(map[string]cache.ResourceEventHandler),
	}

	resourceToCache := map[string]cache.Store{
		"pods":                   resourceStore.PodCache,
		//"node":                   resourceStore.NodeCache,
		//"persistentVolumes":      resourceStore.PVCache,
		//"persistentVolumeClaims": resourceStore.PVCCache,
		"services":               resourceStore.ServiceCache,
		//"replicasets":            resourceStore.ReplicaSetCache,
		"replicationControllers": resourceStore.ReplicationControllerCache,
	}

	resourceStore.resourceToCache = resourceToCache

	return resourceStore
}
