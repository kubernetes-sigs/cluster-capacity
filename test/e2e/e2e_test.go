/*
Copyright 2020 The Kubernetes Authors.

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

package e2e

import (
	"fmt"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubescheduleroptions "k8s.io/kubernetes/cmd/kube-scheduler/app/options"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-capacity/pkg/framework"
)

const (
	failType = "LimitReached"
	limit    = 5
)

func CreateClient(kubeconfig string) (clientset.Interface, error) {
	var cfg *rest.Config
	if len(kubeconfig) != 0 {
		master, err := GetMasterFromKubeconfig(kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse kubeconfig file: %v ", err)
		}

		cfg, err = clientcmd.BuildConfigFromFlags(master, kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("Unable to build config: %v", err)
		}

	} else {
		var err error
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("Unable to build in cluster config: %v", err)
		}
	}

	return clientset.NewForConfig(cfg)
}

func GetMasterFromKubeconfig(filename string) (string, error) {
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil {
		return "", err
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

func initializeClient(t *testing.T) (clientset.Interface, chan struct{}) {
	clientSet, err := CreateClient(os.Getenv("KUBECONFIG"))
	if err != nil {
		t.Fatalf("Error during client creation with %v", err)
	}

	stopChannel := make(chan struct{}, 0)

	sharedInformerFactory := informers.NewSharedInformerFactory(clientSet, 0)
	sharedInformerFactory.Start(stopChannel)
	sharedInformerFactory.WaitForCacheSync(stopChannel)

	return clientSet, stopChannel
}

func buildSimulatedPod() *v1.Pod {
	simulatedPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "simulated-pod", Namespace: "test-node-3", ResourceVersion: "10"},
		Spec:       apitesting.V1DeepEqualSafePodSpec(),
	}

	limitResourceList := make(map[v1.ResourceName]resource.Quantity)
	requestsResourceList := make(map[v1.ResourceName]resource.Quantity)

	limitResourceList[v1.ResourceCPU] = *resource.NewMilliQuantity(100, resource.DecimalSI)
	limitResourceList[v1.ResourceMemory] = *resource.NewQuantity(5e6, resource.BinarySI)
	limitResourceList[framework.ResourceNvidiaGPU] = *resource.NewQuantity(0, resource.DecimalSI)
	requestsResourceList[v1.ResourceCPU] = *resource.NewMilliQuantity(100, resource.DecimalSI)
	requestsResourceList[v1.ResourceMemory] = *resource.NewQuantity(5e6, resource.BinarySI)
	requestsResourceList[framework.ResourceNvidiaGPU] = *resource.NewQuantity(0, resource.DecimalSI)

	// set pod's resource consumption
	simulatedPod.Spec.Containers = []v1.Container{
		{
			Resources: v1.ResourceRequirements{
				Limits:   limitResourceList,
				Requests: requestsResourceList,
			},
		},
	}

	return simulatedPod
}

func TestLimitReached(t *testing.T) {
	clientSet, stopCh := initializeClient(t)
	defer close(stopCh)

	opts, err := kubescheduleroptions.NewOptions()
	if err != nil {
		t.Fatalf("unable to create scheduler options: %v", err)
	}

	opts.ComponentConfig = kubeschedulerconfig.KubeSchedulerConfiguration{
		AlgorithmSource: kubeschedulerconfig.SchedulerAlgorithmSource{
			Provider: pointer.StringPtr("DefaultProvider"),
		},
		Profiles: []kubeschedulerconfig.KubeSchedulerProfile{
			{
				SchedulerName: v1.DefaultSchedulerName,
				Plugins: &kubeschedulerconfig.Plugins{
					Bind: &kubeschedulerconfig.PluginSet{
						Enabled: []kubeschedulerconfig.Plugin{
							{
								Name: "ClusterCapacityBinder",
							},
						},
						Disabled: []kubeschedulerconfig.Plugin{
							{
								Name: "DefaultBinder",
							},
						},
					},
				},
			},
		},
	}

	kubeSchedulerConfig, err := framework.InitKubeSchedulerConfiguration(opts)
	if err != nil {
		t.Fatal(err)
	}

	cc, err := framework.New(kubeSchedulerConfig,
		buildSimulatedPod(),
		limit,
	)
	defer cc.Close()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := cc.SyncWithClient(clientSet); err != nil {
		t.Fatalf("Unable to sync resources: %v", err)
	}
	if err := cc.Run(); err != nil {
		t.Fatalf("Unable to run analysis: %v", err)
	}

	for reason, replicas := range cc.Report().Status.Pods[0].ReplicasOnNodes {
		t.Logf("Reason: %v, instances: %v\n", reason, replicas)
	}

	t.Logf("Stop reason: %v\n", cc.Report().Status.FailReason)

	if cc.Report().Status.FailReason.FailType != failType {
		t.Fatalf("Unexpected stop reason occured: %v, expecting: %v", cc.Report().Status.FailReason.FailType, failType)
	}
}
