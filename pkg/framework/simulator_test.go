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

package framework

import (
	"fmt"
	goruntime "runtime"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
	"k8s.io/kubernetes/pkg/version"
	soptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/plugin/pkg/scheduler/factory"

	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
)

func getGeneralNode(nodeName string, capacity, allocatable v1.ResourceList) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Spec:       v1.NodeSpec{},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeOutOfDisk,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasSufficientDisk",
					Message:            fmt.Sprintf("kubelet has sufficient disk space available"),
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               v1.NodeMemoryPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasSufficientMemory",
					Message:            fmt.Sprintf("kubelet has sufficient memory available"),
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               v1.NodeDiskPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasNoDiskPressure",
					Message:            fmt.Sprintf("kubelet has no disk pressure"),
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionTrue,
					Reason:             "KubeletReady",
					Message:            fmt.Sprintf("kubelet is posting ready status"),
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
			},
			NodeInfo: v1.NodeSystemInfo{
				MachineID:               "123",
				SystemUUID:              "abc",
				BootID:                  "1b3",
				KernelVersion:           "3.16.0-0.bpo.4-amd64",
				OSImage:                 "Debian GNU/Linux 7 (wheezy)",
				OperatingSystem:         goruntime.GOOS,
				Architecture:            goruntime.GOARCH,
				ContainerRuntimeVersion: "test://1.5.0",
				KubeletVersion:          version.Get().String(),
				KubeProxyVersion:        version.Get().String(),
			},
			Capacity:    capacity,
			Allocatable: allocatable,
			Addresses: []v1.NodeAddress{
				{Type: v1.NodeExternalIP, Address: "127.0.0.1"},
				{Type: v1.NodeInternalIP, Address: "127.0.0.1"},
			},
			Images: []v1.ContainerImage{},
		},
	}
}

func getResourceList(cpu, memory, pods, nvidiaGPU int64) *v1.ResourceList {
	return &v1.ResourceList{
		v1.ResourceCPU:       *resource.NewMilliQuantity(cpu, resource.DecimalSI),
		v1.ResourceMemory:    *resource.NewQuantity(memory, resource.BinarySI),
		v1.ResourcePods:      *resource.NewQuantity(pods, resource.DecimalSI),
		v1.ResourceNvidiaGPU: *resource.NewQuantity(nvidiaGPU, resource.DecimalSI),
	}
}

func getPod(name, namespace string, cpuRequest, memoryRequest, nvidiaGPURequest int64) *v1.Pod {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, ResourceVersion: "10"},
		Spec:       apitesting.V1DeepEqualSafePodSpec(),
	}
	limitResourceList := make(map[v1.ResourceName]resource.Quantity)
	requestsResourceList := make(map[v1.ResourceName]resource.Quantity)
	limitResourceList[v1.ResourceCPU] = *resource.NewMilliQuantity(cpuRequest, resource.DecimalSI)
	limitResourceList[v1.ResourceMemory] = *resource.NewQuantity(memoryRequest, resource.BinarySI)
	limitResourceList[v1.ResourceNvidiaGPU] = *resource.NewQuantity(nvidiaGPURequest, resource.DecimalSI)
	requestsResourceList[v1.ResourceCPU] = *resource.NewMilliQuantity(cpuRequest, resource.DecimalSI)
	requestsResourceList[v1.ResourceMemory] = *resource.NewQuantity(memoryRequest, resource.BinarySI)
	requestsResourceList[v1.ResourceNvidiaGPU] = *resource.NewQuantity(nvidiaGPURequest, resource.DecimalSI)
	// set pod's resource consumption
	pod.Spec.Containers = []v1.Container{
		{
			Resources: v1.ResourceRequirements{
				Limits:   limitResourceList,
				Requests: requestsResourceList,
			},
		},
	}
	return &pod
}

func TestPrediction(t *testing.T) {
	testCases := []struct {
		name            string
		podSpecs        []*v1.Pod
		nodes           []*v1.Node
		maxNumberOfPods int
		expNumPods      []int
		expFailType     string
	}{
		{
			name: "Limit reached",
			podSpecs: []*v1.Pod{
				getPod("simulated-pod-1", "test-namespace-1", 100, 5E6, 0),
			},
			nodes: []*v1.Node{
				// create first node with 2 cpus and 4GB, with some resources already consumed
				getGeneralNode("test-node-1",
					*getResourceList(2000, 4E9, 10, 0), //capacity
					*getResourceList(300, 1E9, 3, 0),   //allocatable
				),
				// create second node with 1 cpus and 4GB, with some resources already consumed
				getGeneralNode("test-node-2",
					*getResourceList(1000, 4E9, 10, 0), //capacity
					*getResourceList(400, 2E9, 3, 0),   //allocatable
				),
				// create second node with 2 cpus and 4GB, with some resources already consumed
				getGeneralNode("test-node-3",
					*getResourceList(2000, 4E9, 10, 0), //capacity
					*getResourceList(1200, 1E9, 3, 0),  //allocatable
				),
			},
			maxNumberOfPods: 6,
			expNumPods:      []int{6},
			expFailType:     "LimitReached",
		},
		{
			name: "Unschedulable due to insufficient memory",
			podSpecs: []*v1.Pod{
				getPod("simulated-pod-1", "test-namespace-1", 100, 100E6, 0),
				getPod("simulated-pod-2", "test-namespace-1", 100, 50E6, 0),
			},
			nodes: []*v1.Node{
				// create first node with 2 cpus and 1GB
				getGeneralNode("test-node-1",
					*getResourceList(2000, 1E9, 20, 0), //capacity
					*getResourceList(2000, 1E9, 20, 0), //allocatable
				),
			},
			maxNumberOfPods: 100,
			expNumPods:      []int{7, 6},
			expFailType:     "Unschedulable",
		},
	}

	for _, tc := range testCases {
		t.Logf("Test case: %s\n", tc.name)
		// 1. create fake storage with initial data
		// - create three nodes, each node with different resources (cpu, memory)
		resourceStore := store.NewResourceStore()

		// Add nodes
		for _, node := range tc.nodes {
			if err := resourceStore.Add("nodes", metav1.Object(node)); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		}

		// 2. create predictor
		// - create simple configuration file for scheduler (use the default values or from systemd env file if reasonable)
		cc, err := New(&soptions.SchedulerServer{
			KubeSchedulerConfiguration: componentconfig.KubeSchedulerConfiguration{
				SchedulerName:                  v1.DefaultSchedulerName,
				HardPodAffinitySymmetricWeight: v1.DefaultHardPodAffinitySymmetricWeight,
				FailureDomains:                 kubeletapis.DefaultFailureDomains,
				AlgorithmProvider:              factory.DefaultProvider,
			},
			Master:     "http://localhost:8080",
			Kubeconfig: "/etc/kubernetes/kubeconfig",
		},
			tc.podSpecs,
			tc.maxNumberOfPods,
		)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// 3. run predictor
		if err := cc.SyncWithStore(resourceStore); err != nil {
			t.Errorf("Unable to sync resources: %v", err)
		}
		if err := cc.Run(); err != nil {
			t.Errorf("Unable to run analysis: %v", err)
		}

		//4. check expected number of pods is scheduled and reflected in the resource storage
		for i, pod := range cc.Report().Status.Pods {
			t.Logf("Report for pod: %s", pod.PodName)
			total := 0
			for _, replicas := range pod.ReplicasOnNodes {
				t.Logf("Node: %s, instances: %d\n", replicas.NodeName, replicas.Replicas)
				total += replicas.Replicas
			}
			if e, a := tc.expNumPods[i], total; e != a {
				t.Errorf("Expected number of replicas for pod %d was %d, but we got %d", i, e, a)
			}
		}

		t.Logf("Stop reason: %v\n", cc.Report().Status.FailReason)

		if cc.Report().Status.FailReason.FailType != tc.expFailType {
			t.Errorf("Unexpected stop reason occured: %v, expecting: %v", cc.Report().Status.FailReason.FailType, tc.expFailType)
		}
	}
}
