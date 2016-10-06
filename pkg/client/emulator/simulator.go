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
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/restclient"
	ccapi "github.com/ingvagabund/cluster-capacity/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	soptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"os"
	"io/ioutil"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/client/record"
)

type ClusterCapacity struct {
	// caches modified by emulation strategy
	resourceStore store.ResourceStore

	// emulation strategy
	strategy *strategy.Strategy

	// fake kube client
	kubeclient clientset.Interface

	// schedulers
	schedulers map[string]*scheduler.Scheduler
	defaultScheduler string
}

func (c *ClusterCapacity) SyncWithClient(client cache.Getter) error {

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

func (c *ClusterCapacity) SyncWithStore(resourceStore store.ResourceStore) {
	for _, resource := range resourceStore.Resources() {
		fmt.Println(resource)
	}
}

func (c *ClusterCapacity) Run() {
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

func (c *ClusterCapacity) AddScheduler(s *soptions.SchedulerServer) error {
	configFactory := factory.NewConfigFactory(c.kubeclient, s.SchedulerName, s.HardPodAffinitySymmetricWeight, s.FailureDomains)
	config, err := createConfig(s, configFactory)

	if err != nil {
		return fmt.Errorf("Failed to create scheduler configuration: %v", err)
	}

	// Does it make sense to collect events here?
	config.Recorder = &record.FakeRecorder{}
	c.schedulers[s.SchedulerName] = scheduler.New(config)
	return nil
}

func createConfig(s *soptions.SchedulerServer, configFactory *factory.ConfigFactory) (*scheduler.Config, error) {
	if _, err := os.Stat(s.PolicyConfigFile); err == nil {
		var (
			policy     schedulerapi.Policy
			configData []byte
		)
		configData, err := ioutil.ReadFile(s.PolicyConfigFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read policy config: %v", err)
		}
		if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, &policy); err != nil {
			return nil, fmt.Errorf("invalid configuration: %v", err)
		}
		return configFactory.CreateFromConfig(policy)
	}

	// if the config file isn't provided, use the specified (or default) provider
	return configFactory.CreateFromProvider(s.AlgorithmProvider)
}
func New() *ClusterCapacity {
	resourceStore := store.NewResourceStore()
	restClient := restclient.NewRESTClient(resourceStore)

	cc := &ClusterCapacity{
		resourceStore: resourceStore,
		kubeclient: clientset.New(restClient),
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

	// read the default scheduler name from configuration
	cc.defaultScheduler = "default"

	// Create empty event recorder, broadcaster, metrics and everything up to binder.
	// Binder is redirected to cluster capacity's counter.

	return cc
}
