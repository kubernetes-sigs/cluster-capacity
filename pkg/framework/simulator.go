/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api/v1"
	externalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	einformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	sapps "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	soptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	"k8s.io/kubernetes/plugin/pkg/scheduler/core"

	// register algorithm providers
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	ccapi "github.com/kubernetes-incubator/cluster-capacity/pkg/api"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/record"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/restclient/external"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/strategy"
)

const (
	podProvisioner         = "cc.kubernetes.io/provisioned-by"
	podSpecIndexAnnotation = "cluster-capacity/pod-spec-index"
)

// SimulatedPod represents a pod spec to be scheduled with the associated
// replica count.
type SimulatedPod struct {
	*v1.Pod
	Replicas int32
}

func newPodsIterator(podSpecs []SimulatedPod, wrapAround bool) *podsIterator {
	return &podsIterator{podSpecs: podSpecs, idx: -1, replicaCounter: 0, wrapAround: wrapAround}
}

// podsIterator iterates the pod specs. If wrapAround is set to 'true'
// iteration never stops and after last pod spec the iteration restarts from
// the first one. Incase it is set to false the iteration stops when the end of
// the podSpecs slice is reached.
type podsIterator struct {
	podSpecs []SimulatedPod
	idx      int
	// counts the visited replicas for the current pod spec
	replicaCounter int32
	// counts the visiter replicas for all pod specs
	totalCounter int32
	// flag saying wether we should or not restart from the first when all pod
	// specs has been visited.
	wrapAround bool
}

// Next passes to the next pod spec and return true if the end of the
// iterations is reached.
func (p *podsIterator) Next() bool {
	if p.idx >= len(p.podSpecs) {
		if p.wrapAround && p.totalCounter > 0 {
			// restart from the first, totalCounter is used to stop the
			// recursion in case there is no pod with at least one replica.
			p.idx = 0
		} else {
			// no more pod specs
			return false
		}
	}
	// if the exp number of replicas is reached pass to next pod spec,
	// otherwise increment the counters
	if p.idx < 0 || p.replicaCounter >= p.podSpecs[p.idx].Replicas {
		p.idx++
		p.replicaCounter = 0
		return p.Next()
	}
	p.replicaCounter++
	p.totalCounter++
	return true
}

// Value returns current pod spec and the corresponding index in the podSpecs
// list.
func (p *podsIterator) Value() (int, *v1.Pod) {
	if p.idx >= 0 && p.idx < len(p.podSpecs) {
		return p.idx, p.podSpecs[p.idx].Pod
	}
	return p.idx, nil
}

// GetAll returns all the pod specs.
func (p *podsIterator) GetAll() []*v1.Pod {
	var podSpecs []*v1.Pod
	for _, p := range p.podSpecs {
		podSpecs = append(podSpecs, p.Pod)
	}
	return podSpecs
}

type ClusterCapacity struct {
	// caches modified by emulation strategy
	resourceStore store.ResourceStore

	// emulation strategy
	strategy strategy.Strategy

	// fake kube client
	externalkubeclient *externalclientset.Clientset

	informerFactory einformers.SharedInformerFactory

	// fake rest clients
	coreRestClient *external.RESTClient

	// schedulers
	schedulers       map[string]*scheduler.Scheduler
	schedulerConfigs map[string]*scheduler.Config
	defaultScheduler string

	// pod to schedule
	simulatedPodsIterator *podsIterator
	lastSimulatedPod      *v1.Pod
	maxSimulated          int
	simulated             int
	status                Status
	report                *ClusterCapacityReview

	// analysis limitation
	informerStopCh chan struct{}

	// stop the analysis
	stop      chan struct{}
	stopMux   sync.RWMutex
	stopped   bool
	closedMux sync.RWMutex
	closed    bool
}

// capture all scheduled pods with reason why the analysis could not continue
type Status struct {
	Pods       []*v1.Pod
	StopReason string
}

func (c *ClusterCapacity) Report() *ClusterCapacityReview {
	if c.report == nil {
		// Preparation before pod sequence scheduling is done
		c.report = GetReport(c.simulatedPodsIterator.GetAll(), c.status)
		c.report.Spec.Replicas = int32(c.maxSimulated)
	}

	return c.report
}

func (c *ClusterCapacity) SyncWithClient(client externalclientset.Interface) error {
	for _, resource := range c.resourceStore.Resources() {
		listWatcher := cache.NewListWatchFromClient(client.Core().RESTClient(), resource.String(), metav1.NamespaceAll, fields.ParseSelectorOrDie(""))

		options := metav1.ListOptions{ResourceVersion: "0"}
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

func (c *ClusterCapacity) SyncWithStore(resourceStore store.ResourceStore) error {
	for _, resource := range resourceStore.Resources() {
		err := c.resourceStore.Replace(resource, resourceStore.List(resource), "0")
		if err != nil {
			return fmt.Errorf("Resource replace error: %v\n", err)
		}
	}
	return nil
}

func (c *ClusterCapacity) Bind(binding *v1.Binding, schedulerName string) error {
	// run the pod through strategy
	key := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: binding.Name, Namespace: binding.Namespace},
	}
	pod, exists, err := c.resourceStore.Get(ccapi.Pods, runtime.Object(key))
	if err != nil {
		return fmt.Errorf("Unable to bind: %v", err)
	}
	if !exists {
		return fmt.Errorf("Unable to bind, pod %v not found", pod)
	}
	updatedPod := *pod.(*v1.Pod)
	updatedPod.Spec.NodeName = binding.Target.Name
	updatedPod.Status.Phase = v1.PodRunning

	// TODO(jchaloup): rename Add to Update as this actually updates the scheduled pod
	if err := c.strategy.Add(&updatedPod); err != nil {
		return fmt.Errorf("Unable to recompute new cluster state: %v", err)
	}

	c.status.Pods = append(c.status.Pods, &updatedPod)
	go func() {
		<-c.schedulerConfigs[schedulerName].Recorder.(*record.Recorder).Events
	}()

	if c.maxSimulated > 0 && c.simulated >= c.maxSimulated {
		c.status.StopReason = fmt.Sprintf("LimitReached: Maximum number of pods simulated: %v", c.maxSimulated)
		c.Close()
		c.stop <- struct{}{}
		return nil
	}

	// all good, create another pod
	if err := c.nextPod(); err != nil {
		if strings.HasPrefix(c.status.StopReason, "PodSpecsExhausted") {
			c.Close()
			c.stop <- struct{}{}
			return nil
		}
		return fmt.Errorf("Unable to create next pod to schedule: %v", err)
	}
	return nil
}

func (c *ClusterCapacity) Close() {
	c.closedMux.Lock()
	defer c.closedMux.Unlock()

	if c.closed {
		return
	}

	for _, name := range c.schedulerConfigs {
		close(name.StopEverything)
	}

	c.coreRestClient.Close()
	close(c.informerStopCh)
	c.closed = true
}

func (c *ClusterCapacity) Update(pod *v1.Pod, podCondition *v1.PodCondition, schedulerName string) error {
	// TODO: For the moment it stops when the first unschedulable pod is met, it
	// would be interesting to allow other strategies ( e.g. stop the
	// simulation when there are no more schedulable pods ).
	stop := podCondition.Type == v1.PodScheduled && podCondition.Status == v1.ConditionFalse && podCondition.Reason == "Unschedulable"

	// Only for pending pods provisioned by cluster-capacity
	if stop && metav1.HasAnnotation(pod.ObjectMeta, podProvisioner) {
		c.status.StopReason = fmt.Sprintf("%v: %v", podCondition.Reason, podCondition.Message)
		c.Close()
		// The Update function can be run more than once before any corresponding
		// scheduler is closed. The behaviour is implementation specific
		c.stopMux.Lock()
		defer c.stopMux.Unlock()
		c.stopped = true
		c.stop <- struct{}{}
	}
	return nil
}

func (c *ClusterCapacity) nextPod() error {
	if !c.simulatedPodsIterator.Next() {
		c.status.StopReason = "PodSpecsExhausted: No more pod specs to schedule"
		return errors.New(c.status.StopReason)
	}
	cloner := conversion.NewCloner()
	pod := v1.Pod{}
	idx, currPod := c.simulatedPodsIterator.Value()
	if err := v1.DeepCopy_v1_Pod(currPod, &pod, cloner); err != nil {
		return err
	}
	// Adds annotations map if not present
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}
	// Stores the index pointing to the pod spec used to generate this pod
	pod.ObjectMeta.Annotations[podSpecIndexAnnotation] = strconv.Itoa(idx)

	// reset any node designation set
	pod.Spec.NodeName = ""
	// use simulated pod name with an index to construct the name
	pod.ObjectMeta.Name = fmt.Sprintf("%v-%v", pod.ObjectMeta.Name, c.simulated)

	// Add pod provisioner annotation
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}

	// Stores the scheduler name
	pod.ObjectMeta.Annotations[podProvisioner] = c.defaultScheduler

	c.simulated++
	c.lastSimulatedPod = &pod

	return c.resourceStore.Add(ccapi.Pods, runtime.Object(&pod))
}

func (c *ClusterCapacity) Run() error {
	c.informerFactory.Start(c.informerStopCh)
	// TODO(jchaloup): remove all pods that are not scheduled yet
	for _, scheduler := range c.schedulers {
		scheduler.Run()
	}
	// wait some time before at least nodes are populated
	// TODO(jchaloup); find a better way how to do this or at least decrease it to <100ms
	time.Sleep(100 * time.Millisecond)
	// create the first simulated pod
	err := c.nextPod()
	if err != nil {
		c.Close()
		close(c.stop)
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

func (b *localBinderPodConditionUpdater) Bind(binding *v1.Binding) error {
	return b.C.Bind(binding, b.SchedulerName)
}

func (b *localBinderPodConditionUpdater) Update(pod *v1.Pod, podCondition *v1.PodCondition) error {
	return b.C.Update(pod, podCondition, b.SchedulerName)
}

func (c *ClusterCapacity) createScheduler(s *soptions.SchedulerServer) (*scheduler.Scheduler, error) {
	// TODO improve this
	if c.informerFactory == nil {
		c.informerFactory = einformers.NewSharedInformerFactory(c.externalkubeclient, 0)
	}

	fakeClient := fake.NewSimpleClientset()
	fakeInformerFactory := einformers.NewSharedInformerFactory(fakeClient, 0)

	scheduler, err := sapps.CreateScheduler(s,
		c.externalkubeclient,
		c.informerFactory.Core().V1().Nodes(),
		c.informerFactory.Core().V1().Pods(),
		c.informerFactory.Core().V1().PersistentVolumes(),
		c.informerFactory.Core().V1().PersistentVolumeClaims(),
		fakeInformerFactory.Core().V1().ReplicationControllers(),
		fakeInformerFactory.Extensions().V1beta1().ReplicaSets(),
		fakeInformerFactory.Apps().V1beta1().StatefulSets(),
		c.informerFactory.Core().V1().Services(),
		record.NewRecorder(10))

	if err != nil {
		return nil, fmt.Errorf("error creating scheduler: %v", err)
	}
	schedulerConfig := scheduler.Config()
	// Replace the binder with simulator pod counter
	lbpcu := &localBinderPodConditionUpdater{
		SchedulerName: s.SchedulerName,
		C:             c,
	}
	schedulerConfig.Binder = lbpcu
	schedulerConfig.PodConditionUpdater = lbpcu
	// pending merge of https://github.com/kubernetes/kubernetes/pull/44115
	// we wrap how error handling is done to avoid extraneous logging
	errorFn := schedulerConfig.Error
	wrappedErrorFn := func(pod *v1.Pod, err error) {
		if _, ok := err.(*core.FitError); !ok {
			errorFn(pod, err)
		}
	}
	schedulerConfig.Error = wrappedErrorFn
	return scheduler, nil
}

func (c *ClusterCapacity) AddScheduler(s *soptions.SchedulerServer) error {
	scheduler, err := c.createScheduler(s)
	if err != nil {
		return err
	}

	c.schedulers[s.SchedulerName] = scheduler
	c.schedulerConfigs[s.SchedulerName] = scheduler.Config()
	return nil
}

// Create new cluster capacity analysis
// The analysis is completely independent of apiserver so no need
// for kubeconfig nor for apiserver url
func New(s *soptions.SchedulerServer, simulatedPods []SimulatedPod, maxPods int, oneShot bool) (*ClusterCapacity, error) {
	resourceStore := store.NewResourceStore()
	restClient := external.NewRESTClient(resourceStore, "core")

	cc := &ClusterCapacity{
		resourceStore:         resourceStore,
		strategy:              strategy.NewPredictiveStrategy(resourceStore),
		externalkubeclient:    externalclientset.New(restClient),
		simulatedPodsIterator: newPodsIterator(simulatedPods, !oneShot),
		simulated:             0,
		maxSimulated:          maxPods,
		coreRestClient:        restClient,
	}

	for _, resource := range resourceStore.Resources() {
		// The resource variable would be shared among all [Add|Update|Delete]Func functions
		// and resource would be set to the last item in resources list.
		// Thus, it needs to be stored to a local variable in each iteration.
		rt := resource
		resourceStore.RegisterEventHandler(rt, cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				restClient.EmitObjectWatchEvent(rt, watch.Added, obj.(runtime.Object))
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				restClient.EmitObjectWatchEvent(rt, watch.Modified, newObj.(runtime.Object))
			},
			DeleteFunc: func(obj interface{}) {
				restClient.EmitObjectWatchEvent(rt, watch.Deleted, obj.(runtime.Object))
			},
		})
	}

	cc.schedulers = make(map[string]*scheduler.Scheduler)
	cc.schedulerConfigs = make(map[string]*scheduler.Config)

	scheduler, err := cc.createScheduler(s)
	if err != nil {
		return nil, err
	}

	cc.schedulers[s.SchedulerName] = scheduler
	cc.schedulerConfigs[s.SchedulerName] = scheduler.Config()
	cc.defaultScheduler = s.SchedulerName
	cc.stop = make(chan struct{})
	cc.informerStopCh = make(chan struct{})
	return cc, nil
}
