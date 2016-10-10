package emulator

import (
	"fmt"
	"reflect"
	"time"

	ccapi "github.com/ingvagabund/cluster-capacity/pkg/api"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/restclient"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/strategy"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/watch"
	soptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"

	"io/ioutil"
	"os"

	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/runtime"
	// register algorithm providers
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
)

type ClusterCapacity struct {
	// caches modified by emulation strategy
	resourceStore store.ResourceStore

	// emulation strategy
	strategy strategy.Strategy

	// fake kube client
	kubeclient clientset.Interface

	// schedulers
	schedulers       map[string]*scheduler.Scheduler
	schedulerConfigs map[string]*scheduler.Config
	defaultScheduler string

	// pod to schedule
	simulatedPod *api.Pod
	maxSimulated int
	simulated    int
	status       Status

	// stop the analysis
	stop chan struct{}
}

// capture all scheduled pods with reason why the analysis could not continue
type Status struct {
	Pods       []*api.Pod
	StopReason string
}

func (c *ClusterCapacity) Status() Status {
	return c.status
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
		err := c.resourceStore.Replace(resource, resourceStore.List(resource), "0")
		if err != nil {
			fmt.Printf("Resource replace error: %v\n", err)
		}
	}
}

func (c *ClusterCapacity) Bind(binding *api.Binding, schedulerName string) error {
	// pod name: binding.Name
	// node name: binding.Target.Name
	// fmt.Printf("\nPod: %v, node: %v, scheduler: %v\n", binding.Name, binding.Target.Name, schedulerName)

	// run the pod through strategy
	key := &api.Pod{
		ObjectMeta: api.ObjectMeta{Name: binding.Name, Namespace: binding.Namespace},
	}
	pod, exists, err := c.resourceStore.Get(ccapi.Pods, runtime.Object(key))
	if err != nil {
		return fmt.Errorf("Unable to bind: %v", err)
	}
	if !exists {
		return fmt.Errorf("Unable to bind, pod %v not found", pod)
	}
	updatedPod := *pod.(*api.Pod)
	updatedPod.Spec.NodeName = binding.Target.Name
	// fmt.Printf("Pod binding: %v\n", updatedPod)

	// TODO(jchaloup): rename Add to Update as this actually updates the scheduled pod
	if err := c.strategy.Add(&updatedPod); err != nil {
		return fmt.Errorf("Unable to recompute new cluster state: %v", err)
	}

	c.status.Pods = append(c.status.Pods, &updatedPod)
	go func() {
		<-c.schedulerConfigs[schedulerName].Recorder.(*record.FakeRecorder).Events
		//fmt.Printf("Scheduling event: %v\n", event)
	}()
	// all good, create another pod
	if err := c.nextPod(); err != nil {
		return fmt.Errorf("Unable to create next pod to schedule: %v", err)
	}
	return nil
}

func (c *ClusterCapacity) Update(pod *api.Pod, podCondition *api.PodCondition, schedulerName string) error {
	// once the api.PodCondition
	podUnschedulableCond := &api.PodCondition{
		Type:   api.PodScheduled,
		Status: api.ConditionFalse,
		Reason: "Unschedulable",
	}

	stop := reflect.DeepEqual(podCondition, podUnschedulableCond)

	//fmt.Printf("pod condition: %v\n", podCondition)
	go func() {
		event := <-c.schedulerConfigs[schedulerName].Recorder.(*record.FakeRecorder).Events
		// end the simulation
		// TODO(jchaloup): this needs to be reworked in a case of multiple schedulers
		// The stop condition is different for a case of multi-pods
		if stop {
			c.status.StopReason = event
			for _, name := range c.schedulerConfigs {
				close(name.StopEverything)
			}
			c.stop <- struct{}{}
		}
	}()

	return nil
}

func (c *ClusterCapacity) nextPod() error {
	pod := *c.simulatedPod
	// use simulated pod name with an index to construct the name
	pod.ObjectMeta.Name = fmt.Sprintf("%v-%v", c.simulatedPod.Name, c.simulated)
	c.simulated++
	return c.resourceStore.Add(ccapi.Pods, runtime.Object(&pod))
}

func (c *ClusterCapacity) Run() error {
	// TODO(jchaloup): remove all pods that are not scheduled yet

	for _, scheduler := range c.schedulers {
		scheduler.Run()
	}
	// wait some time before at least nodes are populated
	// TODO(jchaloup); find a better way how to do this or at least increase it to <100ms
	time.Sleep(100 * time.Millisecond)
	// create the first simulated pod
	err := c.nextPod()
	if err != nil {
		return fmt.Errorf("Unable to create next pod to schedule: %v", err)
	}

	<-c.stop
	close(c.stop)

	return nil
}

type localBinderPodConditionUpdater struct {
	SchedulerName string
	C             *ClusterCapacity
}

func (b *localBinderPodConditionUpdater) Bind(binding *api.Binding) error {
	return b.C.Bind(binding, b.SchedulerName)
}

func (b *localBinderPodConditionUpdater) Update(pod *api.Pod, podCondition *api.PodCondition) error {
	return b.C.Update(pod, podCondition, b.SchedulerName)
}

func (c *ClusterCapacity) createSchedulerConfig(s *soptions.SchedulerServer) (*scheduler.Config, error) {
	configFactory := factory.NewConfigFactory(c.kubeclient, s.SchedulerName, s.HardPodAffinitySymmetricWeight, s.FailureDomains)
	config, err := createConfig(s, configFactory)

	if err != nil {
		return nil, fmt.Errorf("Failed to create scheduler configuration: %v", err)
	}

	// Collect scheduler succesfully/failed scheduled pod
	config.Recorder = record.NewFakeRecorder(10)
	// Replace the binder with simulator pod counter
	lbpcu := &localBinderPodConditionUpdater{
		SchedulerName: s.SchedulerName,
		C:             c,
	}
	config.Binder = lbpcu
	config.PodConditionUpdater = lbpcu
	return config, nil
}

func (c *ClusterCapacity) AddScheduler(s *soptions.SchedulerServer) error {
	config, err := c.createSchedulerConfig(s)
	if err != nil {
		return err
	}

	c.schedulers[s.SchedulerName] = scheduler.New(config)
	c.schedulerConfigs[s.SchedulerName] = config
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

// Create new cluster capacity analysis
// The analysis is completely independent of apiserver so no need
// for kubeconfig nor for apiserver url
func New(s *soptions.SchedulerServer, simulatedPod *api.Pod) (*ClusterCapacity, error) {
	resourceStore := store.NewResourceStore()
	restClient := restclient.NewRESTClient(resourceStore)

	cc := &ClusterCapacity{
		resourceStore: resourceStore,
		strategy:      strategy.NewPredictiveStrategy(resourceStore),
		kubeclient:    clientset.New(restClient),
		simulatedPod:  simulatedPod,
		simulated:     0,
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

	cc.schedulers = make(map[string]*scheduler.Scheduler)
	cc.schedulerConfigs = make(map[string]*scheduler.Config)

	// read the default scheduler name from configuration
	config, err := cc.createSchedulerConfig(s)
	if err != nil {
		return nil, fmt.Errorf("Unable to create cluster capacity analyzer: %v", err)
	}

	cc.schedulers[s.SchedulerName] = scheduler.New(config)
	cc.schedulerConfigs[s.SchedulerName] = config
	cc.defaultScheduler = s.SchedulerName

	cc.stop = make(chan struct{})

	// Create empty event recorder, broadcaster, metrics and everything up to binder.
	// Binder is redirected to cluster capacity's counter.

	return cc, nil
}
