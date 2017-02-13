package options

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"path"

	"github.com/kubernetes-incubator/cluster-capacity/pkg/apiserver/cache"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/yaml"
	schedopt "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/utils"
)

type ClusterCapacityConfig struct {
	Schedulers       []*schedopt.SchedulerServer
	Pod              *api.Pod
	KubeClient       clientset.Interface
	Options          *ClusterCapacityOptions
	DefaultScheduler *schedopt.SchedulerServer
	Reports          *cache.Cache
	ApiServerOptions *framework.ApiServerOptions
	ResourceStore    store.ResourceStore
}

type ClusterCapacityOptions struct {
	Kubeconfig                 string
	SchedulerConfigFile        []string
	DefaultSchedulerConfigFile string
	MaxLimit                   int
	Verbose                    bool
	PodSpecFile                string
	Period                     int
	OutputFormat               string
	ApiserverConfigFile        string
	ResourceSpaceMode          string
}

func NewClusterCapacityConfig(opt *ClusterCapacityOptions) *ClusterCapacityConfig {
	return &ClusterCapacityConfig{
		Schedulers:       make([]*schedopt.SchedulerServer, 0),
		Options:          opt,
		DefaultScheduler: schedopt.NewSchedulerServer(),
	}
}

func NewClusterCapacityOptions() *ClusterCapacityOptions {
	return &ClusterCapacityOptions{}
}

func (s *ClusterCapacityOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.StringVar(&s.PodSpecFile, "podspec", s.PodSpecFile, "Path to JSON or YAML file containing pod definition.")
	fs.IntVar(&s.MaxLimit, "max-limit", 0, "Number of instances of pod to be scheduled after which analysis stops. By default unlimited.")

	//TODO(jchaloup): uncomment this line once the multi-schedulers are fully implemented
	//fs.StringArrayVar(&s.SchedulerConfigFile, "config", s.SchedulerConfigFile, "Paths to files containing scheduler configuration in JSON or YAML format")

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalf("Unable to get current directory: %v", err)
	}

	filepath := path.Join(dir, "config/default-scheduler.yaml")

	fs.StringVar(&s.DefaultSchedulerConfigFile, "default-config", filepath, "Path to JSON or YAML file containing scheduler configuration.")
	fs.StringVar(&s.ApiserverConfigFile, "apiserver-config", s.ApiserverConfigFile, "Path to JSON or YAML file containing apiserver configuration.")
	fs.StringVar(&s.ResourceSpaceMode, "resource-space-mode", "ResourceSpaceFull", "Resource space limitation. Defaults to ResourceSpaceFull. If set to ResourceSpacePartial, ResourceQuota admission is applied.")

	fs.BoolVar(&s.Verbose, "verbose", s.Verbose, "Verbose mode")
	fs.IntVar(&s.Period, "period", 0, "Number of seconds between cluster capacity checks, if period=0 cluster-capacity will be checked just once")
	fs.StringVarP(&s.OutputFormat, "output", "o", s.OutputFormat, "Output format. One of: json|yaml")
}

func parseApiServerOptions(path string) (*framework.ApiServerOptions, error) {
	filename, _ := filepath.Abs(path)
	config, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open config file: %v", err)
	}

	apiServerRunOptions := &framework.ApiServerOptions{}
	decoder := yaml.NewYAMLOrJSONDecoder(config, 4096)
	err = decoder.Decode(apiServerRunOptions)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	return apiServerRunOptions, nil
}

func parseSchedulerConfig(path string) (*schedopt.SchedulerServer, error) {
	filename, _ := filepath.Abs(path)
	config, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to open config file: %v", err)
	}

	newScheduler := schedopt.NewSchedulerServer()
	decoder := yaml.NewYAMLOrJSONDecoder(config, 4096)
	decoder.Decode(&(newScheduler.KubeSchedulerConfiguration))
	return newScheduler, nil
}

func (s *ClusterCapacityConfig) ParseAdditionalSchedulerConfigs() error {
	for _, config := range s.Options.SchedulerConfigFile {
		if config == "default-scheduler.yaml" {
			continue
		}
		newScheduler, err := parseSchedulerConfig(config)
		if err != nil {
			return err
		}
		newScheduler.Master, err = utils.GetMasterFromKubeConfig(s.Options.Kubeconfig)
		if err != nil {
			return err
		}
		newScheduler.Kubeconfig = s.Options.Kubeconfig
		s.Schedulers = append(s.Schedulers, newScheduler)
	}
	return nil
}

func (s *ClusterCapacityConfig) ParseAPISpec() error {
	var spec io.Reader
	var err error
	if strings.HasPrefix(s.Options.PodSpecFile, "http://") || strings.HasPrefix(s.Options.PodSpecFile, "https://") {
		response, err := http.Get(s.Options.PodSpecFile)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("unable to read URL %q, server reported %v, status code=%v", s.Options.PodSpecFile, response.Status, response.StatusCode)
		}
		spec = response.Body
	} else {
		filename, _ := filepath.Abs(s.Options.PodSpecFile)
		spec, err = os.Open(filename)
		if err != nil {
			return fmt.Errorf("Failed to open config file: %v", err)
		}
	}

	decoder := yaml.NewYAMLOrJSONDecoder(spec, 4096)
	err = decoder.Decode(&(s.Pod))
	if err != nil {
		return fmt.Errorf("Failed to decode config file: %v", err)
	}

	if s.Pod.ObjectMeta.Namespace == "" {
		s.Pod.ObjectMeta.Namespace = "default"
	}

	// hardcoded from kube api defaults and validation
	// TODO: rewrite when object validation gets more available for non kubectl approaches in kube
	if s.Pod.Spec.DNSPolicy == "" {
		s.Pod.Spec.DNSPolicy = api.DNSClusterFirst
	}
	if s.Pod.Spec.RestartPolicy == "" {
		s.Pod.Spec.RestartPolicy = api.RestartPolicyAlways
	}

	if errs := validation.ValidatePod(s.Pod); len(errs) > 0 {
		var errStrs []string
		for _, err := range errs {
			errStrs = append(errStrs, fmt.Sprintf("%v: %v", err.Type, err.Field))
		}
		return fmt.Errorf("Invalid pod: %#v", strings.Join(errStrs, ", "))
	}
	return nil
}

func (s *ClusterCapacityConfig) SetDefaultScheduler() error {
	var err error
	s.DefaultScheduler, err = parseSchedulerConfig(s.Options.DefaultSchedulerConfigFile)
	if err != nil {
		return err
	}

	s.DefaultScheduler.Master, err = utils.GetMasterFromKubeConfig(s.Options.Kubeconfig)
	if err != nil {
		return err
	}
	s.DefaultScheduler.Kubeconfig = s.Options.Kubeconfig
	return nil
}

func (s *ClusterCapacityConfig) ParseApiServerConfig() error {
	if len(s.Options.ApiserverConfigFile) == 0 {
		s.ApiServerOptions = &framework.ApiServerOptions{}
		return nil
	}
	options, err := parseApiServerOptions(s.Options.ApiserverConfigFile)
	if err != nil {
		return err
	}
	s.ApiServerOptions = options
	return nil
}
