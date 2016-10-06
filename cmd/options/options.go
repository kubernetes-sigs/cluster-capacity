package options

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/util/yaml"
	schedopt "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
)

type ClusterCapacityOptions struct {
	schedulerConfigFile string
	Scheduler           *schedopt.SchedulerServer
}

func NewClusterCapacityOptions() *ClusterCapacityOptions {
	return &ClusterCapacityOptions{
		Scheduler: schedopt.NewSchedulerServer(),
	}
}

func (s *ClusterCapacityOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.schedulerConfigFile, "config", s.schedulerConfigFile, "Path to file containing scheduler configuration in JSON or YAML format")
}

func (s *ClusterCapacityOptions) validateOptions() error {
	if len(s.Scheduler.Master) == 0 {
		return fmt.Errorf("master needs to be specified")
	}

	if len(s.Scheduler.Kubeconfig) == 0 {
		return fmt.Errorf("kubeconfig needs to be specified")
	}
	return nil
}

func (s *ClusterCapacityOptions) ParseSchedulerConfig() error {
	if len(s.schedulerConfigFile) == 0 {
		return fmt.Errorf("missing --config flag argument")
	}
	filename, _ := filepath.Abs(s.schedulerConfigFile)
	config, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Failed to open config file: %v", err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(config, 4096)
	decoder.Decode(&(s.Scheduler))
	if err != nil {
		return fmt.Errorf("Failed to decode config file: %v", err)
	}

	return s.validateOptions()
}
