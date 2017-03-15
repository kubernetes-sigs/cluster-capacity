/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/renstrom/dedent"
	"github.com/spf13/cobra"

	"k8s.io/client-go/tools/clientcmd"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"github.com/kubernetes-incubator/cluster-capacity/cmd/cluster-capacity/app/options"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/apiserver"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/apiserver/cache"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/utils"
)

var (
	clusterCapacityLong = dedent.Dedent(`
		Cluster-capacity simulates an API server with initial state copied from the Kubernetes enviroment
		with its configuration specified in KUBECONFIG. The simulated API server tries to schedule the number of
		pods specified by --max-limits flag. If the --max-limits flag is not specified, pods are scheduled until
		the simulated API server runs out of resources.
	`)

	MAXREPORTS = 100
)

func NewClusterCapacityCommand() *cobra.Command {
	opt := options.NewClusterCapacityOptions()
	cmd := &cobra.Command{
		Use:   "cluster-capacity --kubeconfig KUBECONFIG --podspec PODSPEC",
		Short: "Cluster-capacity is used for simulating scheduling of one or multiple pods",
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

	if len(opt.AdmissionControl) > 0 {
		admissionsNames := strings.Split(opt.AdmissionControl, ",")
		admissionNamesSets := sets.NewString(admissionsNames...)
		if !options.SupportedAdmissionControllers.IsSuperset(admissionNamesSets) {
			return fmt.Errorf("Requested not supported admission control plugin. Supported admission control plugins are: %v",
				strings.Join(options.SupportedAdmissionControllers.List(), ", "))
		}
	}

	if len(opt.PodSpecFile) == 0 {
		return fmt.Errorf("Pod spec file is missing")
	}

	if len(opt.Kubeconfig) == 0 {
		return fmt.Errorf("kubeconfig is missing")
	}

	if opt.ResourceSpaceMode != "" && opt.ResourceSpaceMode != "ResourceSpaceFull" && opt.ResourceSpaceMode != "ResourceSpacePartial" {
		return fmt.Errorf("Resource space mode not recognized. Valid values are: ResourceSpaceFull, ResourceSpacePartial")
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

	master, err := utils.GetMasterFromKubeConfig(conf.Options.Kubeconfig)
	if err != nil {
		return fmt.Errorf("Failed to parse kubeconfig file: %v ", err)
	}
	conf.KubeClient, err = getKubeClient(master, conf.Options.Kubeconfig)

	if err != nil {
		return err
	}

	if opt.Period == 0 {
		report, err := runSimulator(conf, true)
		if err != nil {
			return err
		}
		if err := framework.ClusterCapacityReviewPrint(report, conf.Options.Verbose, conf.Options.OutputFormat); err != nil {
			return fmt.Errorf("Error while printing: %v", err)
		}
		return nil
	}

	conf.Reports = cache.NewCache(MAXREPORTS)

	r := apiserver.NewResource(conf)

	go func() {
		log.Fatal(apiserver.ListenAndServe(r))
	}()

	// sync and watch apiserver
	conf.ResourceStore = store.NewResourceReflectors(conf.KubeClient, wait.NeverStop)

	runSimulation := func(syncWithClient bool) {
		report, err := runSimulator(conf, syncWithClient)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		conf.Reports.Add(report)
		r.PutStatus(report)

		if conf.Options.Verbose {
			framework.ClusterCapacityReviewPrint(report, conf.Options.Verbose, conf.Options.OutputFormat)
		}
	}

	runSimulation(true)
	time.Sleep(time.Duration(opt.Period) * time.Second)

	for {
		runSimulation(false)
		time.Sleep(time.Duration(opt.Period) * time.Second)
	}
}

func getKubeClient(master string, config string) (clientset.Interface, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(master, config)
	if err != nil {
		return nil, fmt.Errorf("Unable to build config: %v", err)
	}
	kubeClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Invalid API configuration: %v", err)
	}

	if _, err = kubeClient.Discovery().ServerVersion(); err != nil {
		return nil, fmt.Errorf("Unable to get server version: %v\n", err)
	}

	return kubeClient, nil
}

func runSimulator(s *options.ClusterCapacityConfig, syncWithClient bool) (*framework.ClusterCapacityReview, error) {
	mode, err := framework.StringToResourceSpaceMode(s.Options.ResourceSpaceMode)
	if err != nil {
		return nil, err
	}

	cc, err := framework.New(s.DefaultScheduler, s.Pod, s.Options.MaxLimit, mode, s.Options.AdmissionControl)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(s.Schedulers); i++ {
		if err = cc.AddScheduler(s.Schedulers[i]); err != nil {
			return nil, err
		}
	}

	if syncWithClient {
		err = cc.SyncWithClient(s.KubeClient)
	} else {
		err = cc.SyncWithStore(s.ResourceStore)
	}
	if err != nil {
		return nil, err
	}

	err = cc.Run()
	if err != nil {
		return nil, err
	}

	report := cc.Report()
	return report, nil
}
