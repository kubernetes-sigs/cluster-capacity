package options

import (
	"github.com/spf13/pflag"
	schedopt "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
)

type ClusterCapacityServer struct {
	clusterCapacityConfiguration
	Scheduler *schedopt.SchedulerServer
}

func NewClusterCapacityServer() *ClusterCapacityServer {
	return &ClusterCapacityServer{
		Scheduler: schedopt.NewSchedulerServer(),
	}
}
func (s *ClusterCapacityServer) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Master, "master", s.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
}

type clusterCapacityConfiguration struct {
	Master     string
	Kubeconfig string
}
