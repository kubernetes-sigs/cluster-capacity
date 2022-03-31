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

package utils

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/events"
	configv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/component-base/logs"
	kubeschedulerconfigv1beta2 "k8s.io/kube-scheduler/config/v1beta2"
	schedconfig "k8s.io/kubernetes/cmd/kube-scheduler/app/config"
	kubescheduleroptions "k8s.io/kubernetes/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	kubeschedulerscheme "k8s.io/kubernetes/pkg/scheduler/apis/config/scheme"
)

func init() {
	if err := v1.AddToScheme(legacyscheme.Scheme); err != nil {
		fmt.Printf("err: %v\n", err)
	}
}

func PrintPod(pod *v1.Pod, format string) error {
	var contentType string
	switch format {
	case "json":
		contentType = runtime.ContentTypeJSON
	case "yaml":
		contentType = "application/yaml"
	default:
		contentType = "application/yaml"
	}

	info, ok := runtime.SerializerInfoForMediaType(legacyscheme.Codecs.SupportedMediaTypes(), contentType)
	if !ok {
		return fmt.Errorf("serializer for %s not registered", contentType)
	}
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	encoder := legacyscheme.Codecs.EncoderForVersion(info.Serializer, gvr.GroupVersion())
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
		return "", fmt.Errorf("can not load kubeconfig file: %v", err)
	}

	context, ok := config.Contexts[config.CurrentContext]
	if !ok {
		return "", fmt.Errorf("Failed to get master address from kubeconfig")
	}

	if val, ok := config.Clusters[context.Cluster]; ok {
		return val.Server, nil
	}
	return "", fmt.Errorf("Failed to get master address from kubeconfig")
}

func BuildKubeSchedulerCompletedConfig(kcfg *kubeschedulerconfig.KubeSchedulerConfiguration) (*schedconfig.CompletedConfig, error) {
	if kcfg == nil {
		kcfg = &kubeschedulerconfig.KubeSchedulerConfiguration{}
		versionedCfg := kubeschedulerconfigv1beta2.KubeSchedulerConfiguration{}
		versionedCfg.DebuggingConfiguration = *configv1alpha1.NewRecommendedDebuggingConfiguration()

		kubeschedulerscheme.Scheme.Default(&versionedCfg)
		if err := kubeschedulerscheme.Scheme.Convert(&versionedCfg, kcfg, nil); err != nil {
			return nil, err
		}
	}
	// inject scheduler config config
	if len(kcfg.Profiles) == 0 {
		kcfg.Profiles = []kubeschedulerconfig.KubeSchedulerProfile{
			{},
		}
	}

	kcfg.Profiles[0].SchedulerName = v1.DefaultSchedulerName
	if kcfg.Profiles[0].Plugins == nil {
		kcfg.Profiles[0].Plugins = &kubeschedulerconfig.Plugins{}
	}

	kcfg.Profiles[0].Plugins.Bind = kubeschedulerconfig.PluginSet{
		Enabled:  []kubeschedulerconfig.Plugin{{Name: "ClusterCapacityBinder"}},
		Disabled: []kubeschedulerconfig.Plugin{{Name: "DefaultBinder"}},
	}

	opts := &kubescheduleroptions.Options{
		ComponentConfig: kcfg,
		Logs:            logs.NewOptions(),
	}

	c := &schedconfig.Config{}
	// clear out all unnecesary options so no port is bound
	// to allow running multiple instances in a row
	opts.Deprecated = nil
	opts.SecureServing = nil
	if err := opts.ApplyTo(c); err != nil {
		return nil, fmt.Errorf("unable to get scheduler config: %v", err)
	}

	// Get the completed config
	cc := c.Complete()

	// completely ignore the events
	cc.EventBroadcaster = events.NewEventBroadcasterAdapter(fakeclientset.NewSimpleClientset())

	return &cc, nil
}
