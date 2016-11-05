package emulator

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"time"

	ccapi "github.com/ingvagabund/cluster-capacity/pkg/api"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/record"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/restclient"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/strategy"
	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	clientsetextensions "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/extensions/internalversion"
	"k8s.io/kubernetes/pkg/controller/informers"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/genericapiserver/authorizer"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
	soptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"

	"io/ioutil"
	"os"

	"k8s.io/kubernetes/pkg/runtime"
	// register algorithm providers
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	// Admission policies
	_ "k8s.io/kubernetes/plugin/pkg/admission/admit"
	_ "k8s.io/kubernetes/plugin/pkg/admission/alwayspullimages"
	_ "k8s.io/kubernetes/plugin/pkg/admission/antiaffinity"
	_ "k8s.io/kubernetes/plugin/pkg/admission/deny"
	_ "k8s.io/kubernetes/plugin/pkg/admission/exec"
	_ "k8s.io/kubernetes/plugin/pkg/admission/gc"
	_ "k8s.io/kubernetes/plugin/pkg/admission/imagepolicy"
	_ "k8s.io/kubernetes/plugin/pkg/admission/initialresources"
	_ "k8s.io/kubernetes/plugin/pkg/admission/limitranger"
	_ "k8s.io/kubernetes/plugin/pkg/admission/namespace/autoprovision"
	_ "k8s.io/kubernetes/plugin/pkg/admission/namespace/exists"
	_ "k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle"
	_ "k8s.io/kubernetes/plugin/pkg/admission/persistentvolume/label"
	_ "k8s.io/kubernetes/plugin/pkg/admission/podnodeselector"
	_ "k8s.io/kubernetes/plugin/pkg/admission/resourcequota"
	_ "k8s.io/kubernetes/plugin/pkg/admission/security/podsecuritypolicy"
	_ "k8s.io/kubernetes/plugin/pkg/admission/securitycontext/scdeny"
	_ "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	_ "k8s.io/kubernetes/plugin/pkg/admission/storageclass/default"
)

// Main goal: given a pod with non-zero requested resources how many times the pod can be scheduled in the cluster
// Constraints to consider:
// - resource quota for memory/cpu: limit the cluster space to explore
// - namespace node selector: limit the cluster space to explore
//
// Due to the constraints the CC framework operates in two modes:
// - full resource space
// - partial resource space
//
// The full resource space mode explores the entire cluster resource space
// with emphasis to schedule as much instances of pods as possible.
// The partial resource space is limited artificialy and the analysis
// is bounded.

type ResourceSpaceMode string

const (
	ResourceSpaceFull    ResourceSpaceMode = "ResourceSpaceFull"
	ResourceSpacePartial ResourceSpaceMode = "ResourceSpacePartial"
)

func StringToResourceSpaceMode(mode string) (ResourceSpaceMode, error) {
	switch mode {
	case "ResourceSpaceFull":
		return ResourceSpaceFull, nil
	case "ResourceSpacePartial":
		return ResourceSpacePartial, nil
	default:
		return "", fmt.Errorf("Resource space mode not recognized")
	}
}

type ApiServerOptions struct {
	AdmissionControl           string
	AdmissionControlConfigFile string

	// Authorization mode and associated flags.
	AuthorizationMode                        string
	AuthorizationPolicyFile                  string
	AuthorizationWebhookConfigFile           string
	AuthorizationWebhookCacheAuthorizedTTL   unversioned.Duration
	AuthorizationWebhookCacheUnauthorizedTTL unversioned.Duration
	AuthorizationRBACSuperUser               string
}

type ClusterCapacity struct {
	// caches modified by emulation strategy
	resourceStore store.ResourceStore

	// emulation strategy
	strategy strategy.Strategy

	// fake kube client
	kubeclient *clientset.Clientset

	// fake rest clients
	coreRestClient       *restclient.RESTClient
	extensionsRestClient *restclient.RESTClient

	// schedulers
	schedulers       map[string]*scheduler.Scheduler
	schedulerConfigs map[string]*scheduler.Config
	defaultScheduler string

	// pod to schedule
	simulatedPod *api.Pod
	maxSimulated int
	simulated    int
	status       Status
	report       *Report

	// analysis limitation
	resourceSpaceMode   ResourceSpaceMode
	admissionController admission.Interface

	// stop the analysis
	stop      chan struct{}
	stopMux   sync.RWMutex
	stopped   bool
	closedMux sync.RWMutex
	closed    bool
}

// capture all scheduled pods with reason why the analysis could not continue
type Status struct {
	Pods       []*api.Pod
	StopReason string
}

func (c *ClusterCapacity) Report() *Report {
	if c.report == nil {
		c.report = CreateFullReport(c.simulatedPod, c.status)
	}
	return c.report
}

func (c *ClusterCapacity) SyncWithClient(client clientset.Interface) error {
	for _, resource := range c.resourceStore.Resources() {
		var listWatcher *cache.ListWatch
		if resource == ccapi.ReplicaSets {
			listWatcher = cache.NewListWatchFromClient(client.Extensions().RESTClient(), resource.String(), api.NamespaceAll, fields.ParseSelectorOrDie(""))
		} else {
			listWatcher = cache.NewListWatchFromClient(client.Core().RESTClient(), resource.String(), api.NamespaceAll, fields.ParseSelectorOrDie(""))
		}

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
		<-c.schedulerConfigs[schedulerName].Recorder.(*record.Recorder).Events
		//fmt.Printf("Scheduling event: %v\n", event)
	}()

	if c.maxSimulated > 0 && c.simulated >= c.maxSimulated {
		c.status.StopReason = fmt.Sprintf("LimitReached: Maximal number %v of pods simulated", c.maxSimulated)
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

	for _, name := range c.schedulerConfigs {
		close(name.StopEverything)
	}

	c.coreRestClient.Close()
	c.extensionsRestClient.Close()

	c.closed = true
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
		event := <-c.schedulerConfigs[schedulerName].Recorder.(*record.Recorder).Events
		// end the simulation
		// TODO(jchaloup): this needs to be reworked in a case of multiple schedulers
		// The stop condition is different for a case of multi-pods
		if stop {
			c.status.StopReason = fmt.Sprintf("%v: %v", event.Reason, event.Message)
			c.Close()

			// The Update function can be run more than once before any corresponding
			// scheduler is closed. The behaviour is implementation specific
			c.stopMux.Lock()
			defer c.stopMux.Unlock()
			if c.stopped {
				return
			}
			c.stopped = true
			c.stop <- struct{}{}
		}
	}()

	return nil
}

func (c *ClusterCapacity) nextPod() error {
	pod := *c.simulatedPod
	// use simulated pod name with an index to construct the name
	pod.ObjectMeta.Name = fmt.Sprintf("%v-%v", c.simulatedPod.Name, c.simulated)

	if c.admissionController != nil {
		gv := unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}.GroupVersion()
		userInfo, _ := api.UserFrom(api.WithUserAgent(api.NewContext(), "Cluster-Capacity-Agent"))

		attr := admission.NewAttributesRecord(runtime.Object(&pod), nil, unversioned.FromAPIVersionAndKind("v1", "pods"), pod.Namespace, pod.Name, gv.WithResource("pods"), "", admission.Create, userInfo)

		err := c.admissionController.Admit(attr)
		if err != nil {
			return fmt.Errorf("Admission controller error: %v", err)
		}
	}
	c.simulated++
	return c.resourceStore.Add(ccapi.Pods, runtime.Object(&pod))
}

func (c *ClusterCapacity) Run() error {
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
	config.Recorder = record.NewRecorder(10)
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
func New(s *soptions.SchedulerServer, simulatedPod *api.Pod, maxPods int, resourceSpaceMode ResourceSpaceMode, apiserverConfig *ApiServerOptions) (*ClusterCapacity, error) {
	resourceStore := store.NewResourceStore()
	restClient := restclient.NewRESTClient(resourceStore, "core")
	extensionsRestClient := restclient.NewRESTClient(resourceStore, "extensions")

	cc := &ClusterCapacity{
		resourceStore:        resourceStore,
		strategy:             strategy.NewPredictiveStrategy(resourceStore),
		kubeclient:           clientset.New(restClient),
		simulatedPod:         simulatedPod,
		simulated:            0,
		maxSimulated:         maxPods,
		coreRestClient:       restClient,
		extensionsRestClient: extensionsRestClient,
		resourceSpaceMode:    resourceSpaceMode,
	}

	cc.kubeclient.ExtensionsClient = clientsetextensions.New(extensionsRestClient)

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

	// initialize admission controllers if specified
	if len(apiserverConfig.AdmissionControl) > 0 {
		admissionsNames := strings.Split(apiserverConfig.AdmissionControl, ",")

		// filter out limitations that forbid the analysis to expand to entire resource space
		var admissionControlPluginNames []string
		if cc.resourceSpaceMode == ResourceSpaceFull {
			// filter out ResourceQuota admission
			for _, item := range admissionsNames {
				if item == "ResourceQuota" {
					continue
				}
				admissionControlPluginNames = append(admissionControlPluginNames, item)
			}
		} else {
			admissionControlPluginNames = admissionsNames
		}

		sharedInformers := informers.NewSharedInformerFactory(cc.kubeclient, 10*time.Minute)
		authorizationConfig := authorizer.AuthorizationConfig{
			PolicyFile:                  apiserverConfig.AuthorizationPolicyFile,
			WebhookConfigFile:           apiserverConfig.AuthorizationWebhookConfigFile,
			WebhookCacheAuthorizedTTL:   apiserverConfig.AuthorizationWebhookCacheAuthorizedTTL.Duration,
			WebhookCacheUnauthorizedTTL: apiserverConfig.AuthorizationWebhookCacheUnauthorizedTTL.Duration,
			RBACSuperUser:               apiserverConfig.AuthorizationRBACSuperUser,
			InformerFactory:             sharedInformers,
		}

		authorizationModeNames := strings.Split(apiserverConfig.AuthorizationMode, ",")
		apiAuthorizer, err := authorizer.NewAuthorizerFromAuthorizationConfig(authorizationModeNames, authorizationConfig)
		if err != nil {
			log.Fatalf("Invalid Authorization Config: %v", err)
		}

		pluginInitializer := admission.NewPluginInitializer(sharedInformers, apiAuthorizer)
		admissionController, err := admission.NewFromPlugins(cc.kubeclient, admissionControlPluginNames, "", pluginInitializer)
		if err != nil {
			log.Fatalf("Failed to initialize plugins: %v", err)
		}

		cc.admissionController = admissionController

		sharedInformers.Start(wait.NeverStop)
	}

	return cc, nil
}
