package options

import (
	"github.com/spf13/pflag"
)

type GenPodOptions struct {
	Master     string
	Kubeconfig string
	Verbose    bool
	Namespace  string
	Format     string
}

func NewGenPodOptions() *GenPodOptions {
	return &GenPodOptions{}
}

func (s *GenPodOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig, "Path to kubeconfig file with authorization and master location information.")
	fs.BoolVar(&s.Verbose, "verbose", s.Verbose, "Verbose mode")
	fs.StringVar(&s.Namespace, "namespace", s.Namespace, "Cluster namespace")
	fs.StringVar(&s.Format, "output", s.Format, "Output format")
}
