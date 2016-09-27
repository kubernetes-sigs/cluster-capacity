package emulator

import (
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/watch"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/api"
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

func NewClientEmulator() {
	caches := &caches{
		PodCache:                   cache.NewStore(cache.MetaNamespaceKeyFunc),
		NodeCache:                  cache.NewStore(cache.MetaNamespaceKeyFunc),
		PVCache:                    cache.NewStore(cache.MetaNamespaceKeyFunc),
		PVCCache:                   cache.NewStore(cache.MetaNamespaceKeyFunc),
		ServiceCache:               cache.NewStore(cache.MetaNamespaceKeyFunc),
		ReplicaSetCache:            cache.NewStore(cache.MetaNamespaceKeyFunc),
		ReplicationControllerCache: cache.NewStore(cache.MetaNamespaceKeyFunc),
	}
	return ClientEmulator{
		caches: caches,
	}
}
func (*predictiveStrategy) Add(obj []interface{}) error {
}

func (*predictiveStrategy) Update(objs []interface{}) error {
}

func (*predictiveStrategy) Delete(objs []interface{}) error {
}

type ClientEmulator struct {
	// caches modified by emulation strategy
	*caches

	// watch events emulator
	watcher *watch.FakeWatcher

	// emulation strategy
	strategy *Strategy
}

func (c *ClientEmulator) sync (client cache.Getter) {
	podLW := cache.NewListWatchFromClient(client, "pods", api.NamespaceAll, fields.ParseSelectorOrDie(""))
	nodeLW := cache.NewListWatchFromClient(client, "nodes", api.NamespaceAll, fields.ParseSelectorOrDie(""))
	pvLW := cache.NewListWatchFromClient(client, "persistentVolumes", api.NamespaceAll, fields.ParseSelectorOrDie(""))
	pvcLW := cache.NewListWatchFromClient(client, "persistentVolumeClaims", api.NamespaceAll, fields.ParseSelectorOrDie(""))
	serviceLW := cache.NewListWatchFromClient(client, "services", api.NamespaceAll, fields.ParseSelectorOrDie(""))
	replicasetLW := cache.NewListWatchFromClient(client, "replicasets", api.NamespaceAll, fields.ParseSelectorOrDie(""))
	replicationcontrollerLW := cache.NewListWatchFromClient(client, "replicationControllers", api.NamespaceAll, fields.ParseSelectorOrDie(""))

}