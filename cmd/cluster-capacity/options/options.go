package options

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"path"

	"github.com/ingvagabund/cluster-capacity/pkg/apiserver"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/yaml"
	schedopt "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
)

type ClusterCapacityConfig struct {
	Schedulers       []*schedopt.SchedulerServer
	Pod              *api.Pod
	KubeClient       clientset.Interface
	Options          *ClusterCapacityOptions
	DefaultScheduler *schedopt.SchedulerServer
	Reports          *apiserver.Cache
}

type ClusterCapacityOptions struct {
	Master                     string
	Kubeconfig                 string
	SchedulerConfigFile        []string
	DefaultSchedulerConfigFile string
	MaxLimit                   int
	Verbose                    bool
	PodSpecFile                string
	Period                     int
	OutputFormat               string
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
	fs.StringVar(&s.Master, "master", s.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.StringVar(&s.PodSpecFile, "podspec", s.PodSpecFile, "Path to JSON or YAML file containing pod definition.")
	fs.IntVar(&s.MaxLimit, "maxLimit", 0, "Number of pods to be scheduled.")

	//TODO(jchaloup): uncomment this line once the multi-schedulers are fully implemented
	//fs.StringArrayVar(&s.SchedulerConfigFile, "config", s.SchedulerConfigFile, "Paths to files containing scheduler configuration in JSON or YAML format")

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalf("Unable to get current directory: %v", err)
	}

	filepath := path.Join(dir, "config/default-scheduler.yaml")

	fs.StringVar(&s.DefaultSchedulerConfigFile, "default-config", filepath, "Path to JSON or YAML file containing pod definition.")
	fs.BoolVar(&s.Verbose, "verbose", s.Verbose, "Verbose mode")
	fs.IntVar(&s.Period, "period", 0, "Number of seconds between cluster capacity checks, if period=0 cluster-capacity will be checked just once")
	fs.StringVarP(&s.OutputFormat, "output", "o", s.OutputFormat, "Output format. One of: json|yaml")
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
		newScheduler.Master = s.Options.Master
		newScheduler.Kubeconfig = s.Options.Kubeconfig
		s.Schedulers = append(s.Schedulers, newScheduler)
	}
	return nil
}

func (s *ClusterCapacityConfig) ParseAPISpec() error {
	filename, _ := filepath.Abs(s.Options.PodSpecFile)
	spec, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Failed to open config file: %v", err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(spec, 4096)
	decoder.Decode(&(s.Pod))
	if err != nil {
		return fmt.Errorf("Failed to decode config file: %v", err)
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

	s.DefaultScheduler.Master = s.Options.Master
	s.DefaultScheduler.Kubeconfig = s.Options.Kubeconfig
	return nil
}
