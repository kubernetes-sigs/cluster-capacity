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
	"context"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	externalclientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	schedconfig "k8s.io/kubernetes/cmd/kube-scheduler/app/config"
	schedoptions "k8s.io/kubernetes/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/pkg/scheduler"
	framework "k8s.io/kubernetes/pkg/scheduler/framework"
	frameworkruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	"k8s.io/kubernetes/pkg/scheduler/profile"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/events"

	uuid "github.com/satori/go.uuid"
	"sigs.k8s.io/cluster-capacity/pkg/framework/plugins/clustercapacitybinder"
	"sigs.k8s.io/cluster-capacity/pkg/framework/strategy"
)

const (
	podProvisioner = "cc.kubernetes.io/provisioned-by"
)

type ClusterCapacity struct {
	// emulation strategy
	strategy strategy.Strategy

	externalkubeclient externalclientset.Interface
	informerFactory    informers.SharedInformerFactory
	dynInformerFactory dynamicinformer.DynamicSharedInformerFactory

	// schedulers
	schedulers           map[string]*scheduler.Scheduler
	defaultSchedulerName string
	defaultSchedulerConf *schedconfig.CompletedConfig

	// pod to schedule
	simulatedPod     *v1.Pod
	lastSimulatedPod *v1.Pod
	maxSimulated     int
	simulated        int
	status           Status
	report           *ClusterCapacityReview

	// analysis limitation
	informerStopCh chan struct{}
	// schedulers channel
	schedulerCh chan struct{}

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

// Create new cluster capacity analysis
// The analysis is completely independent of apiserver so no need
// for kubeconfig nor for apiserver url
func New(kubeSchedulerConfig *schedconfig.CompletedConfig, kubeConfig *restclient.Config, simulatedPod *v1.Pod, maxPods int) (*ClusterCapacity, error) {
	client := fakeclientset.NewSimpleClientset()
	sharedInformerFactory := informers.NewSharedInformerFactory(client, 0)

	// build a list of informers to wait for
	sharedInformerFactory.Core().V1().Namespaces().Informer()

	kubeSchedulerConfig.Client = client

	cc := &ClusterCapacity{
		strategy:           strategy.NewPredictiveStrategy(client),
		externalkubeclient: client,
		simulatedPod:       simulatedPod,
		simulated:          0,
		maxSimulated:       maxPods,
		stop:               make(chan struct{}),
		informerFactory:    sharedInformerFactory,
		informerStopCh:     make(chan struct{}),
		schedulerCh:        make(chan struct{}),
	}

	if kubeConfig != nil {
		dynClient := dynamic.NewForConfigOrDie(kubeConfig)
		cc.dynInformerFactory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, 0, v1.NamespaceAll, nil)
	}

	cc.schedulers = make(map[string]*scheduler.Scheduler)

	scheduler, err := cc.createScheduler(v1.DefaultSchedulerName, kubeSchedulerConfig)
	if err != nil {
		return nil, err
	}

	cc.schedulers[v1.DefaultSchedulerName] = scheduler
	cc.defaultSchedulerName = v1.DefaultSchedulerName

	cc.informerFactory.Start(cc.informerStopCh)
	cc.informerFactory.WaitForCacheSync(cc.informerStopCh)
	if cc.dynInformerFactory != nil {
		cc.dynInformerFactory.Start(cc.informerStopCh)
		cc.dynInformerFactory.WaitForCacheSync(cc.informerStopCh)
	}

	return cc, nil
}

func (c *ClusterCapacity) Report() *ClusterCapacityReview {
	if c.report == nil {
		// Preparation before pod sequence scheduling is done
		pods := make([]*v1.Pod, 0)
		pods = append(pods, c.simulatedPod)
		c.report = GetReport(pods, c.status)
		c.report.Spec.Replicas = int32(c.maxSimulated)
	}

	return c.report
}

func (c *ClusterCapacity) SyncWithClient(client externalclientset.Interface) error {
	nsItems, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list ns: %v", err)
	}

	for _, item := range nsItems.Items {
		if _, err := c.externalkubeclient.CoreV1().Namespaces().Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy ns: %v", err)
		}
	}

	podItems, err := client.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list pods: %v", err)
	}

	for _, item := range podItems.Items {
		if _, err := c.externalkubeclient.CoreV1().Pods(item.Namespace).Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy pod: %v", err)
		}
	}

	nodeItems, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list nodes: %v", err)
	}

	for _, item := range nodeItems.Items {
		if _, err := c.externalkubeclient.CoreV1().Nodes().Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy node: %v", err)
		}
	}

	serviceItems, err := client.CoreV1().Services(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list services: %v", err)
	}

	for _, item := range serviceItems.Items {
		if _, err := c.externalkubeclient.CoreV1().Services(item.Namespace).Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy service: %v", err)
		}
	}

	pvcItems, err := client.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list pvcs: %v", err)
	}

	for _, item := range pvcItems.Items {
		if _, err := c.externalkubeclient.CoreV1().PersistentVolumeClaims(item.Namespace).Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy pvc: %v", err)
		}
	}

	rcItems, err := client.CoreV1().ReplicationControllers(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list RCs: %v", err)
	}

	for _, item := range rcItems.Items {
		if _, err := c.externalkubeclient.CoreV1().ReplicationControllers(item.Namespace).Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy RC: %v", err)
		}
	}

	pdbItems, err := client.PolicyV1beta1().PodDisruptionBudgets(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list PDBs: %v", err)
	}

	for _, item := range pdbItems.Items {
		if _, err := c.externalkubeclient.PolicyV1beta1().PodDisruptionBudgets(item.Namespace).Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy PDB: %v", err)
		}
	}

	replicaSetItems, err := client.AppsV1().ReplicaSets(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list replicas sets: %v", err)
	}

	for _, item := range replicaSetItems.Items {
		if _, err := c.externalkubeclient.AppsV1().ReplicaSets(item.Namespace).Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy replica set: %v", err)
		}
	}

	statefulSetItems, err := client.AppsV1().StatefulSets(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list stateful sets: %v", err)
	}

	for _, item := range statefulSetItems.Items {
		if _, err := c.externalkubeclient.AppsV1().StatefulSets(item.Namespace).Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy stateful set: %v", err)
		}
	}

	storageClassesItems, err := client.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list storage classes: %v", err)
	}

	for _, item := range storageClassesItems.Items {
		if _, err := c.externalkubeclient.StorageV1().StorageClasses().Create(context.TODO(), &item, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to copy storage class: %v", err)
		}
	}

	return nil
}

func (c *ClusterCapacity) postBindHook(updatedPod *v1.Pod) error {
	c.status.Pods = append(c.status.Pods, updatedPod)

	if c.maxSimulated > 0 && c.simulated >= c.maxSimulated {
		c.status.StopReason = fmt.Sprintf("LimitReached: Maximum number of pods simulated: %v", c.maxSimulated)
		c.Close()
		c.stop <- struct{}{}
		return nil
	}

	// all good, create another pod
	if err := c.nextPod(); err != nil {
		return fmt.Errorf("Unable to create next pod for simulated scheduling: %v", err)
	}
	return nil
}

func (c *ClusterCapacity) Close() {
	c.closedMux.Lock()
	defer c.closedMux.Unlock()

	if c.closed {
		return
	}

	close(c.schedulerCh)
	close(c.informerStopCh)
	c.closed = true
}

func (c *ClusterCapacity) Update(pod *v1.Pod, podCondition *v1.PodCondition, schedulerName string) error {
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
	pod := v1.Pod{}
	pod = *c.simulatedPod.DeepCopy()
	// reset any node designation set
	pod.Spec.NodeName = ""
	// use simulated pod name with an index to construct the name
	pod.ObjectMeta.Name = fmt.Sprintf("%v-%v", c.simulatedPod.Name, c.simulated)
	pod.ObjectMeta.UID = types.UID(uuid.NewV4().String())
	pod.Spec.SchedulerName = c.defaultSchedulerName

	// Add pod provisioner annotation
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}

	// Stores the scheduler name
	pod.ObjectMeta.Annotations[podProvisioner] = c.defaultSchedulerName

	c.simulated++
	c.lastSimulatedPod = &pod

	_, err := c.externalkubeclient.CoreV1().Pods(pod.Namespace).Create(context.TODO(), &pod, metav1.CreateOptions{})
	return err
}

func (c *ClusterCapacity) Run() error {
	// Start all informers.
	// First sync the NS informer
	// Disable the pods informer until the namespaces are populated.
	// c.informerFactory.

	// c.informerFactory.Start(c.informerStopCh)
	// c.informerFactory.WaitForCacheSync(c.informerStopCh)
	// c.dynInformerFactory.Start(c.informerStopCh)
	// c.dynInformerFactory.WaitForCacheSync(c.informerStopCh)

	ctx, cancel := context.WithCancel(context.Background())

	// TODO(jchaloup): remove all pods that are not scheduled yet
	for _, scheduler := range c.schedulers {
		s := scheduler
		go func() {
			s.Run(ctx)
		}()
	}
	// wait some time before at least nodes are populated
	// TODO(jchaloup); find a better way how to do this or at least decrease it to <100ms
	time.Sleep(100 * time.Millisecond)
	// create the first simulated pod
	err := c.nextPod()
	if err != nil {
		cancel()
		c.Close()
		close(c.stop)
		return fmt.Errorf("Unable to create next pod to schedule: %v", err)
	}
	<-c.stop
	cancel()
	close(c.stop)
	return nil
}

func (c *ClusterCapacity) createScheduler(schedulerName string, cc *schedconfig.CompletedConfig) (*scheduler.Scheduler, error) {
	outOfTreeRegistry := frameworkruntime.Registry{
		"ClusterCapacityBinder": func(configuration runtime.Object, f framework.Handle) (framework.Plugin, error) {
			return clustercapacitybinder.New(c.externalkubeclient, configuration, f, c.postBindHook)
		},
	}

	c.informerFactory.Core().V1().Pods().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				if pod, ok := obj.(*v1.Pod); ok && pod.Spec.SchedulerName == schedulerName {
					return true
				}
				return false
			},
			Handler: cache.ResourceEventHandlerFuncs{
				UpdateFunc: func(oldObj, newObj interface{}) {
					if pod, ok := newObj.(*v1.Pod); ok {
						for _, podCondition := range pod.Status.Conditions {
							if podCondition.Type == v1.PodScheduled {
								c.Update(pod, &podCondition, schedulerName)
							}
						}
					}
				},
			},
		},
	)

	// Create the scheduler.
	return scheduler.New(
		c.externalkubeclient,
		c.informerFactory,
		c.dynInformerFactory,
		getRecorderFactory(cc),
		c.schedulerCh,
		scheduler.WithComponentConfigVersion(cc.ComponentConfig.TypeMeta.APIVersion),
		scheduler.WithKubeConfig(cc.KubeConfig),
		scheduler.WithProfiles(cc.ComponentConfig.Profiles...),
		scheduler.WithPercentageOfNodesToScore(cc.ComponentConfig.PercentageOfNodesToScore),
		scheduler.WithFrameworkOutOfTreeRegistry(outOfTreeRegistry),
		scheduler.WithPodMaxBackoffSeconds(cc.ComponentConfig.PodMaxBackoffSeconds),
		scheduler.WithPodInitialBackoffSeconds(cc.ComponentConfig.PodInitialBackoffSeconds),
		scheduler.WithExtenders(cc.ComponentConfig.Extenders...),
		scheduler.WithParallelism(cc.ComponentConfig.Parallelism),
	)
}

// TODO(avesh): enable when support for multiple schedulers is added.
/*func (c *ClusterCapacity) AddScheduler(s *sapps.SchedulerServer) error {
	scheduler, err := c.createScheduler(s)
	if err != nil {
		return err
	}

	c.schedulers[s.SchedulerName] = scheduler
	return nil
}*/

func getRecorderFactory(cc *schedconfig.CompletedConfig) profile.RecorderFactory {
	return func(name string) events.EventRecorder {
		return cc.EventBroadcaster.NewRecorder(name)
	}
}

func InitKubeSchedulerConfiguration(opts *schedoptions.Options) (*schedconfig.CompletedConfig, error) {
	c := &schedconfig.Config{}
	// clear out all unnecesary options so no port is bound
	// to allow running multiple instances in a row
	opts.Deprecated = nil
	opts.SecureServing = nil
	if err := opts.ApplyTo(c); err != nil {
		return nil, fmt.Errorf("unable to get scheduler config: %v", err)
	}

	// Get the completed config
	cc := c.Complete()

	// completely ignore the events
	cc.EventBroadcaster = events.NewEventBroadcasterAdapter(fakeclientset.NewSimpleClientset())

	return &cc, nil
}
