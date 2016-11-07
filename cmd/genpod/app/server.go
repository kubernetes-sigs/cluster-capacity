package app

import (
	"fmt"
	"log"

	"github.com/ingvagabund/cluster-capacity/cmd/genpod/app/options"
	nspod "github.com/ingvagabund/cluster-capacity/pkg/client"
	"github.com/spf13/cobra"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/client/restclient"
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
		var contentType string
		switch opt.Format {
		case "json":
			contentType = runtime.ContentTypeJSON
		case "yaml":
			contentType = "application/yaml"
		default:
			contentType = "application/yaml"
		}

		info, ok := runtime.SerializerInfoForMediaType(testapi.Default.NegotiatedSerializer().SupportedMediaTypes(), contentType)
		if !ok {
			return fmt.Errorf("serializer for %s not registered", contentType)
		}
		gvr := unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
		encoder := api.Codecs.EncoderForVersion(info.Serializer, gvr.GroupVersion())
		stream, err := runtime.Encode(encoder, pod)

		if err != nil {
			return fmt.Errorf("Failed to create pod: %v", err)
		}
		fmt.Print(string(stream))
	}

	return nil
}

func getKubeClient(master string, config string) (clientset.Interface, error) {
	var cfg *restclient.Config
	var err error
	if master != "" && config != "" {
		cfg, err = clientcmd.BuildConfigFromFlags(master, config)
		if err != nil {
			return nil, fmt.Errorf("unable to build config from flags: %v", err)
		}
	} else {
		cfg, err = restclient.InClusterConfig()
		if err != nil {
			return nil, err
		}
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
