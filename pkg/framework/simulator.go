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
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/informers"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	policyinformers "k8s.io/client-go/informers/policy/v1beta1"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	externalclientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	schedconfig "k8s.io/kubernetes/cmd/kube-scheduler/app/config"
	schedoptions "k8s.io/kubernetes/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/scheduler"
	"k8s.io/kubernetes/pkg/scheduler/algorithmprovider"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/pkg/scheduler/api/latest"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/core"
	"k8s.io/kubernetes/pkg/scheduler/factory"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"

	kuberecord "k8s.io/client-go/tools/record"

	uuid "github.com/satori/go.uuid"
	"sigs.k8s.io/cluster-capacity/pkg/framework/record"
	"sigs.k8s.io/cluster-capacity/pkg/framework/strategy"
)

const (
	podProvisioner = "cc.kubernetes.io/provisioned-by"
)

type ClusterCapacity struct {
	// emulation strategy
	strategy strategy.Strategy

	externalkubeclient            externalclientset.Interface
	nodeInformer                  corev1informers.NodeInformer
	podInformer                   corev1informers.PodInformer
	pvInformer                    corev1informers.PersistentVolumeInformer
	pvcInformer                   corev1informers.PersistentVolumeClaimInformer
	replicationControllerInformer corev1informers.ReplicationControllerInformer
	replicaSetInformer            appsinformers.ReplicaSetInformer
	statefulSetInformer           appsinformers.StatefulSetInformer
	serviceInformer               corev1informers.ServiceInformer
	pdbInformer                   policyinformers.PodDisruptionBudgetInformer
	storageClassInformer          storageinformers.StorageClassInformer

	informerFactory informers.SharedInformerFactory

	// schedulers
	schedulers           map[string]*scheduler.Scheduler
	schedulerConfigs     map[string]*factory.Config
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
	podItems, err := client.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list pods: %v", err)
	}

	for _, item := range podItems.Items {
		if _, err := c.externalkubeclient.CoreV1().Pods(item.Namespace).Create(&item); err != nil {
			return fmt.Errorf("unable to copy pod: %v", err)
		}
	}

	nodeItems, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list nodes: %v", err)
	}

	for _, item := range nodeItems.Items {
		if _, err := c.externalkubeclient.CoreV1().Nodes().Create(&item); err != nil {
			return fmt.Errorf("unable to copy node: %v", err)
		}
	}

	serviceItems, err := client.CoreV1().Services(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list services: %v", err)
	}

	for _, item := range serviceItems.Items {
		if _, err := c.externalkubeclient.CoreV1().Services(item.Namespace).Create(&item); err != nil {
			return fmt.Errorf("unable to copy service: %v", err)
		}
	}

	pvcItems, err := client.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list pvcs: %v", err)
	}

	for _, item := range pvcItems.Items {
		if _, err := c.externalkubeclient.CoreV1().PersistentVolumeClaims(item.Namespace).Create(&item); err != nil {
			return fmt.Errorf("unable to copy pvc: %v", err)
		}
	}

	rcItems, err := client.CoreV1().ReplicationControllers(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list RCs: %v", err)
	}

	for _, item := range rcItems.Items {
		if _, err := c.externalkubeclient.CoreV1().ReplicationControllers(item.Namespace).Create(&item); err != nil {
			return fmt.Errorf("unable to copy RC: %v", err)
		}
	}

	pdbItems, err := client.PolicyV1beta1().PodDisruptionBudgets(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list PDBs: %v", err)
	}

	for _, item := range pdbItems.Items {
		if _, err := c.externalkubeclient.PolicyV1beta1().PodDisruptionBudgets(item.Namespace).Create(&item); err != nil {
			return fmt.Errorf("unable to copy PDB: %v", err)
		}
	}

	replicaSetItems, err := client.AppsV1().ReplicaSets(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list replicas sets: %v", err)
	}

	for _, item := range replicaSetItems.Items {
		if _, err := c.externalkubeclient.AppsV1().ReplicaSets(item.Namespace).Create(&item); err != nil {
			return fmt.Errorf("unable to copy replica set: %v", err)
		}
	}

	statefulSetItems, err := client.AppsV1().StatefulSets(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list stateful sets: %v", err)
	}

	for _, item := range statefulSetItems.Items {
		if _, err := c.externalkubeclient.AppsV1().StatefulSets(item.Namespace).Create(&item); err != nil {
			return fmt.Errorf("unable to copy stateful set: %v", err)
		}
	}

	storageClassesItems, err := client.StorageV1().StorageClasses().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list storage classes: %v", err)
	}

	for _, item := range storageClassesItems.Items {
		if _, err := c.externalkubeclient.StorageV1().StorageClasses().Create(&item); err != nil {
			return fmt.Errorf("unable to copy storage class: %v", err)
		}
	}

	return nil
}

func (c *ClusterCapacity) Bind(binding *v1.Binding, schedulerName string) error {
	// run the pod through strategy
	pod, err := c.externalkubeclient.CoreV1().Pods(binding.Namespace).Get(binding.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Unable to bind: %v", err)
	}
	updatedPod := pod.DeepCopy()
	updatedPod.Spec.NodeName = binding.Target.Name
	updatedPod.Status.Phase = v1.PodRunning

	// TODO(jchaloup): rename Add to Update as this actually updates the scheduled pod
	if err := c.strategy.Add(updatedPod); err != nil {
		return fmt.Errorf("Unable to recompute new cluster state: %v", err)
	}

	c.status.Pods = append(c.status.Pods, updatedPod)
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

	_, err := c.externalkubeclient.CoreV1().Pods(pod.Namespace).Create(&pod)
	return err
}

func (c *ClusterCapacity) Run() error {
	// Start all informers.
	c.informerFactory.Start(c.informerStopCh)
	c.informerFactory.WaitForCacheSync(c.informerStopCh)

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

func (c *ClusterCapacity) createScheduler(cc *schedconfig.CompletedConfig) (*scheduler.Scheduler, error) {
	c.informerFactory = cc.InformerFactory

	schedulerConfig, err := c.SchedulerConfigLocal(cc)
	if err != nil {
		return nil, err
	}

	// Replace the binder with simulator pod counter
	lbpcu := &localBinderPodConditionUpdater{
		SchedulerName: cc.ComponentConfig.SchedulerName,
		C:             c,
	}
	schedulerConfig.GetBinder = func(pod *v1.Pod) factory.Binder {
		return lbpcu
	}
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
	// Create the scheduler.
	sched := scheduler.NewFromConfig(schedulerConfig)

	scheduler.AddAllEventHandlers(
		sched,
		cc.ComponentConfig.SchedulerName,
		c.nodeInformer,
		c.podInformer,
		c.pvInformer,
		c.pvcInformer,
		c.serviceInformer,
		c.storageClassInformer,
	)

	return sched, nil
}

// TODO(avesh): enable when support for multiple schedulers is added.
/*func (c *ClusterCapacity) AddScheduler(s *sapps.SchedulerServer) error {
	scheduler, err := c.createScheduler(s)
	if err != nil {
		return err
	}

	c.schedulers[s.SchedulerName] = scheduler
	c.schedulerConfigs[s.SchedulerName] = scheduler.Config()
	return nil
}*/

// Create new cluster capacity analysis
// The analysis is completely independent of apiserver so no need
// for kubeconfig nor for apiserver url
func New(kubeSchedulerConfig *schedconfig.CompletedConfig, simulatedPod *v1.Pod, maxPods int) (*ClusterCapacity, error) {
	client := fakeclientset.NewSimpleClientset()
	sharedInformerFactory := informers.NewSharedInformerFactory(client, 0)

	cc := &ClusterCapacity{
		strategy:                      strategy.NewPredictiveStrategy(client),
		externalkubeclient:            client,
		nodeInformer:                  sharedInformerFactory.Core().V1().Nodes(),
		podInformer:                   sharedInformerFactory.Core().V1().Pods(),
		pvInformer:                    sharedInformerFactory.Core().V1().PersistentVolumes(),
		pvcInformer:                   sharedInformerFactory.Core().V1().PersistentVolumeClaims(),
		replicationControllerInformer: sharedInformerFactory.Core().V1().ReplicationControllers(),
		replicaSetInformer:            sharedInformerFactory.Apps().V1().ReplicaSets(),
		statefulSetInformer:           sharedInformerFactory.Apps().V1().StatefulSets(),
		serviceInformer:               sharedInformerFactory.Core().V1().Services(),
		pdbInformer:                   sharedInformerFactory.Policy().V1beta1().PodDisruptionBudgets(),
		simulatedPod:                  simulatedPod,
		simulated:                     0,
		maxSimulated:                  maxPods,
		stop:                          make(chan struct{}),
		informerStopCh:                make(chan struct{}),
		schedulerCh:                   make(chan struct{}),
	}

	// Apply algorithms based on feature gates.
	// TODO: make configurable?
	algorithmprovider.ApplyFeatureGates()

	if utilfeature.DefaultFeatureGate.Enabled(features.VolumeScheduling) {
		cc.storageClassInformer = sharedInformerFactory.Storage().V1().StorageClasses()
	}

	kubeSchedulerConfig.Client = client
	kubeSchedulerConfig.InformerFactory = sharedInformerFactory
	kubeSchedulerConfig.PodInformer = cc.podInformer
	kubeSchedulerConfig.EventClient = client.CoreV1()
	kubeSchedulerConfig.Recorder = record.NewRecorder(10)

	// Prepare event clients.
	eventBroadcaster := kuberecord.NewBroadcaster()
	kubeSchedulerConfig.Broadcaster = eventBroadcaster

	cc.schedulers = make(map[string]*scheduler.Scheduler)
	cc.schedulerConfigs = make(map[string]*factory.Config)

	scheduler, err := cc.createScheduler(kubeSchedulerConfig)
	if err != nil {
		return nil, err
	}

	cc.schedulers[kubeSchedulerConfig.ComponentConfig.SchedulerName] = scheduler
	cc.schedulerConfigs[kubeSchedulerConfig.ComponentConfig.SchedulerName] = scheduler.Config()
	cc.defaultSchedulerName = kubeSchedulerConfig.ComponentConfig.SchedulerName
	return cc, nil
}

// SchedulerConfig creates the scheduler configuration.
func (c *ClusterCapacity) SchedulerConfigLocal(cc *schedconfig.CompletedConfig) (*factory.Config, error) {
	var bindTimeoutSeconds int64 = 1
	if cc.ComponentConfig.BindTimeoutSeconds != nil {
		bindTimeoutSeconds = *cc.ComponentConfig.BindTimeoutSeconds
	}

	// Set up the configurator which can create schedulers from configs.
	configurator := factory.NewConfigFactory(&factory.ConfigFactoryArgs{
		SchedulerName:                  cc.ComponentConfig.SchedulerName,
		Client:                         cc.Client,
		NodeInformer:                   c.nodeInformer,
		PodInformer:                    c.podInformer,
		PvInformer:                     c.pvInformer,
		PvcInformer:                    c.pvcInformer,
		ReplicationControllerInformer:  c.replicationControllerInformer,
		ReplicaSetInformer:             c.replicaSetInformer,
		StatefulSetInformer:            c.statefulSetInformer,
		ServiceInformer:                c.serviceInformer,
		PdbInformer:                    c.pdbInformer,
		StorageClassInformer:           c.storageClassInformer,
		HardPodAffinitySymmetricWeight: cc.ComponentConfig.HardPodAffinitySymmetricWeight,
		DisablePreemption:              cc.ComponentConfig.DisablePreemption,
		PercentageOfNodesToScore:       cc.ComponentConfig.PercentageOfNodesToScore,
		BindTimeoutSeconds:             bindTimeoutSeconds,
		Registry:                       framework.NewRegistry(),
		Plugins:                        cc.ComponentConfig.Plugins,
		PluginConfig:                   cc.ComponentConfig.PluginConfig,
	})

	var config *factory.Config
	source := cc.ComponentConfig.AlgorithmSource
	switch {
	case source.Provider != nil:
		// Create the config from a named algorithm provider.
		sc, err := configurator.CreateFromProvider(*source.Provider)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler using provider %q: %v", *source.Provider, err)
		}
		config = sc
	case source.Policy != nil:
		// Create the config from a user specified policy source.
		policy := &schedulerapi.Policy{}
		switch {
		case source.Policy.File != nil:
			if err := initPolicyFromFile(source.Policy.File.Path, policy); err != nil {
				return nil, err
			}
		case source.Policy.ConfigMap != nil:
			if err := initPolicyFromConfigMap(cc.Client, source.Policy.ConfigMap, policy); err != nil {
				return nil, err
			}
		}
		sc, err := configurator.CreateFromConfig(*policy)
		if err != nil {
			return nil, fmt.Errorf("couldn't create scheduler from policy: %v", err)
		}
		config = sc
	default:
		return nil, fmt.Errorf("unsupported algorithm source: %v", source)
	}
	// Additional tweaks to the config produced by the configurator.
	config.Recorder = cc.Recorder
	config.DisablePreemption = cc.ComponentConfig.DisablePreemption
	config.StopEverything = c.schedulerCh

	return config, nil
}

// initPolicyFromFile initialize policy from file
func initPolicyFromFile(policyFile string, policy *schedulerapi.Policy) error {
	// Use a policy serialized in a file.
	_, err := os.Stat(policyFile)
	if err != nil {
		return fmt.Errorf("missing policy config file %s", policyFile)
	}
	data, err := ioutil.ReadFile(policyFile)
	if err != nil {
		return fmt.Errorf("couldn't read policy config: %v", err)
	}
	err = runtime.DecodeInto(latestschedulerapi.Codec, []byte(data), policy)
	if err != nil {
		return fmt.Errorf("invalid policy: %v", err)
	}
	return nil
}

// initPolicyFromConfigMap initialize policy from configMap
func initPolicyFromConfigMap(client externalclientset.Interface, policyRef *kubeschedulerconfig.SchedulerPolicyConfigMapSource, policy *schedulerapi.Policy) error {
	// Use a policy serialized in a config map value.
	policyConfigMap, err := client.CoreV1().ConfigMaps(policyRef.Namespace).Get(policyRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get policy config map %s/%s: %v", policyRef.Namespace, policyRef.Name, err)
	}
	data, found := policyConfigMap.Data[kubeschedulerconfig.SchedulerPolicyConfigMapKey]
	if !found {
		return fmt.Errorf("missing policy config map value at key %q", kubeschedulerconfig.SchedulerPolicyConfigMapKey)
	}
	err = runtime.DecodeInto(latestschedulerapi.Codec, []byte(data), policy)
	if err != nil {
		return fmt.Errorf("invalid policy: %v", err)
	}
	return nil
}

func InitKubeSchedulerConfiguration(opts *schedoptions.Options) (*schedconfig.CompletedConfig, error) {
	c := &schedconfig.Config{}
	if err := opts.ApplyTo(c); err != nil {
		return nil, fmt.Errorf("unable to get scheduler config: %v", err)
	}

	// Get the completed config
	cc := c.Complete()

	return &cc, nil
}
