package cmd

import (
	"fmt"

	"github.com/ingvagabund/cluster-capacity/cmd/options"
	"github.com/ingvagabund/cluster-capacity/pkg/apiserver"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator"
	"github.com/renstrom/dedent"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
	"log"
	"time"
)

var (
	clusterCapacityLong = dedent.Dedent(`
		Cluster-capacity simulates API server with initial state copied from kubernetes enviroment running
		on address MASTER with its configuration specified in KUBECONFIG. Simulated API server tries to schedule number of
		pods specified by --maxLimits flag. If the --maxLimits flag is not specified, pods are scheduled till
		the simulated API server runs out of resources.
	`)

	MAXREPORTS = 5
)

func NewClusterCapacityCommand() *cobra.Command {
	opt := options.NewClusterCapacityOptions()
	cmd := &cobra.Command{
		Use:   "cluster-capacity --master MASTER --kubeconfig KUBECONFIG --podspec PODSPEC",
		Short: "Cluster-capacity is used for emulating scheduling of one or multiple pods",
		Long:  clusterCapacityLong,
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

	if opt.Period == 0 {
		_, err = runSimulator(conf)
		return err
	}

	conf.Reports = apiserver.NewCache(MAXREPORTS)

	watch := make(chan *apiserver.Report)
	go func() {
		r := apiserver.NewResource(conf.Reports, watch)
		log.Fatal(apiserver.ListenAndServe(r))
	}()

	for {
		report, err := runSimulator(conf)
		if err != nil {
			return err
		}
		watch <- report
		time.Sleep(time.Duration(opt.Period) * time.Second)
	}
	return nil
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

func runSimulator(s *options.ClusterCapacityConfig) (*apiserver.Report, error) {
	cc, err := emulator.New(s.DefaultScheduler, s.Pod, s.Options.MaxLimit)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(s.Schedulers); i++ {
		if err = cc.AddScheduler(s.Schedulers[i]); err != nil {
			return nil, err
		}
	}
	err = cc.SyncWithClient(s.KubeClient)
	if err != nil {
		return nil, err
	}
	err = cc.Run()
	if err != nil {
		return nil, err
	}

	report := createFullReport(s, cc.Status())

	if s.Options.Period == 0 {
		//TODO: print output in yaml
		report.Print(s.Options.Verbose)
	} else {
		s.Reports.Add(report)
	}
	return report, nil
}

func createFullReport(s *options.ClusterCapacityConfig, status emulator.Status) *apiserver.Report {
	report := createReport(s, status)
	if len(status.Pods) == 0 {
		return report
	}
	nodes := make(map[string]int)
	for _, pod := range status.Pods {
		_, ok := nodes[pod.Spec.NodeName]
		if !ok {
			nodes[pod.Spec.NodeName] = 1
		} else {
			nodes[pod.Spec.NodeName]++
		}
	}

	report.Nodes = make([]apiserver.ReportNode, 0)
	for name, count := range nodes {
		nodereport := apiserver.ReportNode{
			NodeName:  name,
			Instances: count,
		}
		report.Nodes = append(report.Nodes, nodereport)
	}
	return report
}

func createReport(s *options.ClusterCapacityConfig, status emulator.Status) *apiserver.Report {
	info := schedulercache.NewNodeInfo(s.Pod)
	return &apiserver.Report{
		//TODO: set this sooner(right after the check is done)
		Timestamp: time.Now(),
		PodRequirements: apiserver.PodResources{
			Cpu:    float64(info.RequestedResource().MilliCPU) * 0.001,
			Memory: info.RequestedResource().Memory,
		},
		Total: apiserver.ReportTotal{
			Instances: len(status.Pods),
			Reason:    status.StopReason,
		},
	}
}
