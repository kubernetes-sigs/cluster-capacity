// /*
// Copyright 2017 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
package framework

import (
	goruntime "runtime"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/component-base/version"

	"sigs.k8s.io/cluster-capacity/pkg/utils"
)

// func init() {
// 	flag.Set("alsologtostderr", fmt.Sprintf("%t", true))
// 	var logLevel string
// 	flag.StringVar(&logLevel, "logLevel", "4", "test")
// 	flag.Lookup("v").Value.Set(logLevel)
// }

func getGeneralNode(nodeName string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Spec:       v1.NodeSpec{},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeMemoryPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasSufficientMemory",
					Message:            "kubelet has sufficient memory available",
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               v1.NodeDiskPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasNoDiskPressure",
					Message:            "kubelet has no disk pressure",
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionTrue,
					Reason:             "KubeletReady",
					Message:            "kubelet is posting ready status",
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
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(4e9, resource.BinarySI),
				v1.ResourcePods:   *resource.NewQuantity(10, resource.DecimalSI),
				//v1.ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
			},
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(0, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(0, resource.BinarySI),
				v1.ResourcePods:   *resource.NewQuantity(0, resource.DecimalSI),
				//v1.ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
			},
			Addresses: []v1.NodeAddress{
				{Type: v1.NodeExternalIP, Address: "127.0.0.1"},
				{Type: v1.NodeInternalIP, Address: "127.0.0.1"},
			},
			Images: []v1.ContainerImage{},
		},
	}
}

func setupNodes() []*v1.Node {
	// - create three nodes, each node with different resources (cpu, memory)

	// create first node with 2 cpus and 4GB, with some resources already consumed
	node1 := getGeneralNode("test-node-1")
	node1.Status.Capacity = v1.ResourceList{
		v1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(4e9, resource.BinarySI),
		v1.ResourcePods:   *resource.NewQuantity(10, resource.DecimalSI),
		//ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
	}
	node1.Status.Allocatable = v1.ResourceList{
		v1.ResourceCPU:    *resource.NewMilliQuantity(300, resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(1e9, resource.BinarySI),
		v1.ResourcePods:   *resource.NewQuantity(3, resource.DecimalSI),
		//ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
	}

	// create second node with 2 cpus and 1GB, with some resources already consumed
	node2 := getGeneralNode("test-node-2")
	node2.Status.Capacity = v1.ResourceList{
		v1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(4e9, resource.BinarySI),
		v1.ResourcePods:   *resource.NewQuantity(10, resource.DecimalSI),
		//v1.ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
	}
	node2.Status.Allocatable = v1.ResourceList{
		v1.ResourceCPU:    *resource.NewMilliQuantity(400, resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(2e9, resource.BinarySI),
		v1.ResourcePods:   *resource.NewQuantity(3, resource.DecimalSI),
		//v1.ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
	}

	// create third node with 2 cpus and 4GB, with some resources already consumed
	node3 := getGeneralNode("test-node-3")
	node3.Status.Capacity = v1.ResourceList{
		v1.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(4e9, resource.BinarySI),
		v1.ResourcePods:   *resource.NewQuantity(10, resource.DecimalSI),
		//ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
	}
	node3.Status.Allocatable = v1.ResourceList{
		v1.ResourceCPU:    *resource.NewMilliQuantity(1200, resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(1e9, resource.BinarySI),
		v1.ResourcePods:   *resource.NewQuantity(3, resource.DecimalSI),
		//ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
	}

	return []*v1.Node{node1, node2, node3}
}

func TestPrediction(t *testing.T) {

	tests := []struct {
		name     string
		failType string
		limit    int
		nodes    []*v1.Node
	}{
		{
			name:     "Limit reached fail type",
			failType: "LimitReached",
			limit:    6,
			nodes:    setupNodes(),
		},
		{
			name:     "insufficient resources fail type",
			failType: "Unschedulable",
			limit:    0,
			nodes:    setupNodes(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 1. create fake storage with initial data
			grace := int64(30)
			simulatedPod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "simulated-pod", Namespace: "test-node-3", ResourceVersion: "10"},
				Spec: v1.PodSpec{
					RestartPolicy:                 v1.RestartPolicyAlways,
					DNSPolicy:                     v1.DNSClusterFirst,
					TerminationGracePeriodSeconds: &grace,
					SecurityContext:               &v1.PodSecurityContext{},
				},
			}

			limitResourceList := make(map[v1.ResourceName]resource.Quantity)
			requestsResourceList := make(map[v1.ResourceName]resource.Quantity)

			limitResourceList[v1.ResourceCPU] = *resource.NewMilliQuantity(100, resource.DecimalSI)
			limitResourceList[v1.ResourceMemory] = *resource.NewQuantity(5e6, resource.BinarySI)
			limitResourceList[ResourceNvidiaGPU] = *resource.NewQuantity(0, resource.DecimalSI)
			requestsResourceList[v1.ResourceCPU] = *resource.NewMilliQuantity(100, resource.DecimalSI)
			requestsResourceList[v1.ResourceMemory] = *resource.NewQuantity(5e6, resource.BinarySI)
			requestsResourceList[ResourceNvidiaGPU] = *resource.NewQuantity(0, resource.DecimalSI)

			var objs []runtime.Object
			for _, node := range test.nodes {
				objs = append(objs, node)
			}
			client := fakeclientset.NewSimpleClientset(objs...)

			// set pod's resource consumption
			simulatedPod.Spec.Containers = []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Limits:   limitResourceList,
						Requests: requestsResourceList,
					},
				},
			}

			// 2. create predictor
			// - create simple configuration file for scheduler (use the default values or from systemd env file if reasonable)
			kubeSchedulerConfig, err := utils.BuildKubeSchedulerCompletedConfig(nil)
			if err != nil {
				t.Fatal(err)
			}

			cc, err := New(kubeSchedulerConfig,
				nil,
				simulatedPod,
				test.limit,
				nil,
			)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// 3. run predictor
			if err := cc.SyncWithClient(client); err != nil {
				t.Errorf("Unable to sync resources: %v", err)
			}
			if err := cc.Run(); err != nil {
				t.Errorf("Unable to run analysis: %v", err)
			}

			// TODO: modify when sequence is implemented
			for reason, replicas := range cc.Report().Status.Pods[0].ReplicasOnNodes {
				t.Logf("Reason: %v, instances: %v\n", reason, replicas)
			}

			t.Logf("Stop reason: %v\n", cc.Report().Status.FailReason)
			// time.Sleep(5 * time.Second)
			//4. check expected number of pods is scheduled and reflected in the resource storage
			if cc.Report().Status.FailReason.FailType != test.failType {
				t.Errorf("Unexpected stop reason occured: %v, expecting: %v", cc.Report().Status.FailReason.FailType, test.failType)
			}

			cc.Close()
		})
		// Wait until the scheduler is torn down
		// time.Sleep(5 * time.Second)
	}
}
