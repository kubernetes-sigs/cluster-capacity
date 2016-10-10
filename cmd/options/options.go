package options

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/yaml"
	schedopt "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	"runtime"
	"path"
)

type ClusterCapacityConfig struct {
	Schedulers  []*schedopt.SchedulerServer
	Pod        *api.Pod
	KubeClient *unversioned.Client
	Options    *ClusterCapacityOptions
	DefaultScheduler *schedopt.SchedulerServer
}

type ClusterCapacityOptions struct {
	Master              string
	Kubeconfig          string
	SchedulerConfigFile []string
	MaxLimit            int64
	verbose             bool
	PodSpecFile         string
}

func NewClusterCapacityConfig(opt *ClusterCapacityOptions) *ClusterCapacityConfig {
	return &ClusterCapacityConfig{
		Schedulers: make([]*schedopt.SchedulerServer,0),
		Options: opt,
		DefaultScheduler: schedopt.NewSchedulerServer(),
	}
}

func NewClusterCapacityOptions() *ClusterCapacityOptions {
	return &ClusterCapacityOptions{
	}
}

func (s *ClusterCapacityOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Master, "master", s.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.StringVar(&s.PodSpecFile, "podspec", s.PodSpecFile, "Path to JSON or YAML file containing pod definition.")
	fs.Int64Var(&s.MaxLimit, "maxLimit", 0, "Number of pods to be scheduled.")
	fs.StringArrayVar(&s.SchedulerConfigFile, "config", s.SchedulerConfigFile, "Paths to files containing scheduler configuration in JSON or YAML format")
}

func parseSchedulerConfig(path string) (*schedopt.SchedulerServer,error) {
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
	for i := 0; i < len(s.Options.SchedulerConfigFile); i++ {
		newScheduler, err := parseSchedulerConfig(s.Options.SchedulerConfigFile[i])
		if err != nil {
			return err
		}
		//newScheduler.Master = s.Options.Master
		//newScheduler.Kubeconfig = s.Options.Kubeconfig
		s.Schedulers = append(s.Schedulers, newScheduler)
	}
	return nil
}

func (s *ClusterCapacityConfig) ParseAPISpec() error {
	filename, _ := filepath.Abs(s.Options.PodSpecFile)
	config, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Failed to open config file: %v", err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(config, 4096)
	decoder.Decode(&(s.Pod))
	if err != nil {
		return fmt.Errorf("Failed to decode config file: %v", err)
	}
	return nil
}


func (s *ClusterCapacityConfig) SetDefaultScheduler() error {
	_, filename, _, _ := runtime.Caller(1)
	filepath := path.Join(path.Dir(filename), "../config/default-scheduler.yaml")
	s.Options.SchedulerConfigFile = append(s.Options.SchedulerConfigFile, filepath)
	var err error
	s.DefaultScheduler, err = parseSchedulerConfig(filepath)
	if err != nil {
		return err
	}
	//s.DefaultScheduler.Master = s.Options.Master
	//s.DefaultScheduler.Kubeconfig = s.Options.Kubeconfig
	return nil
}
