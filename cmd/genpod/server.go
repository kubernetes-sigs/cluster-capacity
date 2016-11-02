package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/ghodss/yaml"

	"github.com/ingvagabund/cluster-capacity/cmd/genpod/options"
	nspod "github.com/ingvagabund/cluster-capacity/pkg/client"
	"github.com/spf13/cobra"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
)

func NewGenPodCommand() *cobra.Command {
	opt := options.NewGenPodOptions()
	cmd := &cobra.Command{
		Use:   "genpod --master MASTER --kubeconfig KUBECONFIG --namespace NAMESPACE",
		Short: "Generate pod based on namespace resource limits and node selector annotations",
		Long:  "Generate pod based on namespace resource limits and node selector annotations",
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

func Validate(opt *options.GenPodOptions) error {
	if len(opt.Kubeconfig) == 0 {
		return fmt.Errorf("Path to Kubernetes config file missing")
	}

	if len(opt.Master) == 0 {
		return fmt.Errorf("Adress of Kubernetes API server missing")
	}

	if len(opt.Namespace) == 0 {
		return fmt.Errorf("Cluster namespace missing")
	}

	if len(opt.Format) > 0 && opt.Format != "json" && opt.Format != "yaml" {
		return fmt.Errorf("Output format %v not recognized: only json and yaml are allowed", opt.Format)
	}

	return nil
}

func Run(opt *options.GenPodOptions) error {
	client, err := getKubeClient(opt.Master, opt.Kubeconfig)
	if err != nil {
		return err
	}

	pod, err := nspod.RetrieveNamespacePod(client, opt.Namespace)
	if err != nil {
		log.Fatalf("Error: %v\n", err)
	} else {
		//fmt.Printf("Pod: %v\n", *pod)
		var stream []byte
		var err error
		switch opt.Format {
		case "json":
			stream, err = json.Marshal(*pod)
		case "yaml":
		default:
			stream, err = yaml.Marshal(*pod)
		}

		if err != nil {
			return fmt.Errorf("Failed to create pod: %v", err)
		}
		fmt.Print(string(stream))
	}

	return nil
}

func getKubeClient(master string, config string) (clientset.Interface, error) {
	kubeconfig, err := clientcmd.BuildConfigFromFlags(master, config)
	if err != nil {
		return nil, fmt.Errorf("unable to build config from flags: %v", err)
	}

	kubeClient, err := clientset.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Invalid API configuration: %v", err)
	}

	if _, err = kubeClient.Discovery().ServerVersion(); err != nil {
		return nil, fmt.Errorf("Unable to get server version: %v\n", err)
	}
	return kubeClient, nil
}
