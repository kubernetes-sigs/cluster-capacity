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
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/lithammer/dedent"
	"github.com/spf13/cobra"

	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	aflag "k8s.io/component-base/cli/flag"
	schedconfig "k8s.io/kubernetes/cmd/kube-scheduler/app/config"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	kubeschedulerscheme "k8s.io/kubernetes/pkg/scheduler/apis/config/scheme"
	"k8s.io/kubernetes/pkg/scheduler/apis/config/validation"

	"sigs.k8s.io/cluster-capacity/cmd/cluster-capacity/app/options"
	"sigs.k8s.io/cluster-capacity/pkg/framework"
	"sigs.k8s.io/cluster-capacity/pkg/utils"
)

var (
	clusterCapacityLong = dedent.Dedent(`
		Cluster-capacity simulates an API server with initial state copied from the Kubernetes enviroment
		with its configuration specified in KUBECONFIG. The simulated API server tries to schedule the number of
		pods specified by --max-limits flag. If the --max-limits flag is not specified, pods are scheduled until
		the simulated API server runs out of resources.
	`)
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
				if err := cmd.Help(); err != nil {
					fmt.Println(err)
				}
				return
			}
			err = Run(opt)
			if err != nil {
				fmt.Println(err)
			}
		},
	}

	flags := cmd.Flags()
	flags.SetNormalizeFunc(aflag.WordSepNormalizeFunc)
	flags.AddGoFlagSet(flag.CommandLine)
	opt.AddFlags(flags)

	return cmd
}

func Validate(opt *options.ClusterCapacityOptions) error {
	if len(opt.PodSpecFile) == 0 {
		return fmt.Errorf("Pod spec file is missing")
	}

	_, present := os.LookupEnv("CC_INCLUSTER")
	if !present {
		if len(opt.Kubeconfig) == 0 {
			return fmt.Errorf("kubeconfig is missing")
		}
	}
	return nil
}

func Run(opt *options.ClusterCapacityOptions) error {
	conf := options.NewClusterCapacityConfig(opt)

	var kcfg *kubeschedulerconfig.KubeSchedulerConfiguration
	if len(conf.Options.DefaultSchedulerConfigFile) > 0 {
		cfg, err := loadConfigFromFile(conf.Options.DefaultSchedulerConfigFile)
		if err != nil {
			return err
		}
		if err := validation.ValidateKubeSchedulerConfiguration(cfg); err != nil {
			return err
		}
		kcfg = cfg
	} else {
		kcfg = nil
	}

	cc, err := utils.BuildKubeSchedulerCompletedConfig(kcfg)
	if err != nil {
		return fmt.Errorf("failed to init kube scheduler configuration: %v ", err)
	}

	err = conf.ParseAPISpec(v1.DefaultSchedulerName)
	if err != nil {
		return fmt.Errorf("Failed to parse pod spec file: %v ", err)
	}

	var cfg *restclient.Config
	if len(conf.Options.Kubeconfig) != 0 {
		master, err := utils.GetMasterFromKubeConfig(conf.Options.Kubeconfig)
		if err != nil {
			return fmt.Errorf("Failed to parse kubeconfig file: %v ", err)
		}

		cfg, err = clientcmd.BuildConfigFromFlags(master, conf.Options.Kubeconfig)
		if err != nil {
			return fmt.Errorf("Unable to build config: %v", err)
		}

	} else {
		cfg, err = restclient.InClusterConfig()
		if err != nil {
			return fmt.Errorf("Unable to build in cluster config: %v", err)
		}
	}

	conf.KubeClient, err = clientset.NewForConfig(cfg)
	conf.RestConfig = cfg

	if err != nil {
		return err
	}

	report, err := runSimulator(conf, cc)
	if err != nil {
		return err
	}
	if err := framework.ClusterCapacityReviewPrint(report, conf.Options.Verbose, conf.Options.OutputFormat); err != nil {
		return fmt.Errorf("Error while printing: %v", err)
	}
	return nil
}

func runSimulator(s *options.ClusterCapacityConfig, kubeSchedulerConfig *schedconfig.CompletedConfig) (*framework.ClusterCapacityReview, error) {
	cc, err := framework.New(kubeSchedulerConfig, s.RestConfig, s.Pod, s.Options.MaxLimit, s.Options.ExcludeNodes)
	if err != nil {
		return nil, err
	}

	err = cc.SyncWithClient(s.KubeClient)
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

func loadConfigFromFile(file string) (*kubeschedulerconfig.KubeSchedulerConfiguration, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return loadConfig(data)
}

func loadConfig(data []byte) (*kubeschedulerconfig.KubeSchedulerConfiguration, error) {
	// The UniversalDecoder runs defaulting and returns the internal type by default.
	obj, gvk, err := kubeschedulerscheme.Codecs.UniversalDecoder().Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}
	if cfgObj, ok := obj.(*kubeschedulerconfig.KubeSchedulerConfiguration); ok {
		// We don't set this field in pkg/scheduler/apis/config/{version}/conversion.go
		// because the field will be cleared later by API machinery during
		// conversion. See KubeSchedulerConfiguration internal type definition for
		// more details.
		cfgObj.TypeMeta.APIVersion = gvk.GroupVersion().String()
		return cfgObj, nil
	}
	return nil, fmt.Errorf("couldn't decode as KubeSchedulerConfiguration, got %s: ", gvk)
}
