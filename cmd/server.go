package cmd

import (
	"fmt"

	"github.com/ingvagabund/cluster-capacity/cmd/options"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
)

func NewClusterCapacityCommand() *cobra.Command {
	s := options.NewClusterCapacityOptions()
	cmd := &cobra.Command{
		Use:  "cluster-capacity",
		Long: `Cluster-capacity is used for emulating scheduling of one or multiple pods`,
		Run: func(cmd *cobra.Command, args []string) {
			err := Run(s)
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	s.AddFlags(cmd.Flags())
	return cmd
}

func Run(opt *options.ClusterCapacityOptions) error {
	conf := options.NewClusterCapacityConfig(opt)
	err := conf.ParseAPISpec()
	if err != nil {
		return fmt.Errorf("Failed to parse pod spec file: %v ", err)
	}

	err = conf.SetDefaultScheduler()
	if err != nil {
		return fmt.Errorf("Failed to set default scheduler config: %v ", err)
	}
	err = conf.ParseAdditionalSchedulerConfigs()
	if err != nil {
		return fmt.Errorf("Failed to parse config file: %v ", err)
	}
	kubeconfig, err := clientcmd.BuildConfigFromFlags(conf.Options.Master, conf.Options.Kubeconfig)
	if err != nil {
		return fmt.Errorf("unable to build config from flags: %v", err)
	}

	conf.KubeClient, err = unversioned.New(kubeconfig)
	if err != nil {
		return fmt.Errorf("Invalid API configuration: %v", err)
	}
	return runSimulator(conf)
}


func runSimulator(s *options.ClusterCapacityConfig) error {
	cc, err := emulator.New(s.DefaultScheduler, s.Pod)
	if err != nil {
		return err
	}
	for i := 0; i < len(s.Schedulers); i++ {
		cc.AddScheduler(s.Schedulers[0])
	}
	fmt.Printf("max limit = %v\n", s.Options.MaxLimit)
	cc.SyncWithClient(s.KubeClient)
	return cc.Run()

}