package utils

import (
	"fmt"

	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
)

func PrintPod(pod *api.Pod, format string) error {
	var contentType string
	switch format {
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
	return nil
}

func GetMasterFromKubeConfig(filename string) (string, error) {
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil {
		return "", err
	}
	if val, ok := config.Clusters[config.CurrentContext]; ok {
		return val.Server, nil
	}
	return "", fmt.Errorf("Failed to get master address from kubeconfig")
}
