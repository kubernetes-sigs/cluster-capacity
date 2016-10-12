package cmd

import (
	"fmt"

	"github.com/ingvagabund/cluster-capacity/cmd/options"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

const CLR_0 = "\x1b[30;1m"
const CLR_R = "\x1b[31;1m"
const CLR_G = "\x1b[32;1m"
const CLR_Y = "\x1b[33;1m"
const CLR_B = "\x1b[34;1m"
const CLR_M = "\x1b[35;1m"
const CLR_C = "\x1b[36;1m"
const CLR_W = "\x1b[37;1m"
const CLR_N = "\x1b[0m"

func NewClusterCapacityCommand() *cobra.Command {
	opt := options.NewClusterCapacityOptions()
	cmd := &cobra.Command{
		Use:   "cluster-capacity --master MASTER --kubeconfig KUBECONFIG --podspec PODSPEC",
		Short: "Cluster-capacity is used for emulating scheduling of one or multiple pods",
		Long: `Cluster-capacity simulates API server with initial state copied from kubernetes enviroment running
		on address MASTER with its configuration specified in KUBECONFIG. Simulated API server tries to schedule number of
		pods specified by --maxLimits flag. If the --maxLimits flag is not specified, pods are scheduled till
		the simulated API server runs out of resources.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := Validate(opt)
			if err != nil {
				fmt.Println(err)
				cmd.Help()
				return
			}
			err = Run(opt)
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	opt.AddFlags(cmd.Flags())
	return cmd
}

func Validate(opt *options.ClusterCapacityOptions) error {
	if len(opt.Kubeconfig) == 0 {
		return fmt.Errorf("Path to Kubernetes config file missing")
	}
	if len(opt.Master) == 0 {
		return fmt.Errorf("Adress of Kubernetes API server missing")
	}
	if len(opt.PodSpecFile) == 0 {
		return fmt.Errorf("Pod spec file is missing")
	}
	return nil
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

	conf.KubeClient, err = getKubeClient(conf.Options.Master, conf.Options.Kubeconfig)

	if err != nil {
		return err
	}
	return runSimulator(conf)
}

func getKubeClient(master string, config string) (*unversioned.Client, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags(master, config)
	if err != nil {
		return nil, fmt.Errorf("unable to build config from flags: %v", err)
	}

	kubeClient, err := unversioned.New(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Invalid API configuration: %v", err)
	}

	if _, err = kubeClient.Discovery().ServerVersion(); err != nil {
		return nil, fmt.Errorf("Unable to get server version: %v\n", err)
	}
	return kubeClient, nil
}

func runSimulator(s *options.ClusterCapacityConfig) error {
	cc, err := emulator.New(s.DefaultScheduler, s.Pod, s.Options.MaxLimit)
	if err != nil {
		return err
	}
	for i := 0; i < len(s.Schedulers); i++ {
		if err = cc.AddScheduler(s.Schedulers[i]); err != nil {
			return err
		}
	}
	err = cc.SyncWithClient(s.KubeClient)
	if err != nil {
		return err
	}
	err = cc.Run()
	if err != nil {
		return err
	}

	// print the analysis status
	status := cc.Status()
	if s.Options.Verbose {
		fmt.Printf("%vPod requirements:%v\n", CLR_W, CLR_N)
		info := schedulercache.NewNodeInfo(s.Pod)
		fmt.Printf("\t- cpu: %v\n", float64(info.RequestedResource().MilliCPU)*0.001)
		fmt.Printf("\t- memory: %v\n", info.RequestedResource().Memory)
		fmt.Printf("\n")
	}

	fmt.Printf("The cluster can schedule %v%v%v instance(s) of the pod.\n", CLR_W, len(status.Pods), CLR_N)
	fmt.Printf("%vTermination reason%v: %v\n", CLR_G, CLR_N, status.StopReason)

	if s.Options.Verbose && len(status.Pods) > 0 {
		nodes := make(map[string]int)
		for _, pod := range status.Pods {
			_, ok := nodes[pod.Spec.NodeName]
			if !ok {
				nodes[pod.Spec.NodeName] = 1
			} else {
				nodes[pod.Spec.NodeName]++
			}
		}

		fmt.Printf("\nPod distribution among nodes:\n")
		for node, amount := range nodes {
			fmt.Printf("\t- %v: %v instance(s)\n", node, amount)
		}
	}

	//fmt.Println(cc.Status())
	return nil
}
