package benchmark

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	kubeschedulerconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"

	"sigs.k8s.io/cluster-capacity/pkg/framework"
	"sigs.k8s.io/cluster-capacity/pkg/utils"
)

func TestPodAffinityHardConstraintSingleNode(t *testing.T) {
	nodes := []*v1.Node{
		BuildTestNode("node1", 1000, 1000, 30, setHostname()),
		BuildTestNode("node2", 1000, 1000, 30, setHostname()),
		BuildTestNode("node3", 1000, 1000, 30, setHostname()),
	}

	pod := BuildTestPod("pod-affinity", 10, 10, "", nil)
	pod.Labels["key"] = "value"
	pod.Spec.Affinity = &v1.Affinity{
		PodAffinity: &v1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					TopologyKey: "kubernetes.io/hostname",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": "value",
						},
					},
				},
			},
		},
	}

	ns := BuildNamespace("default")

	kubeSchedulerConfig, err := utils.BuildKubeSchedulerCompletedConfig(nil)
	if err != nil {
		t.Fatal(err)
	}

	kubeSchedulerConfig.Config.ComponentConfig.Profiles[0].Plugins.Filter.Enabled = []kubeschedulerconfig.Plugin{
		{
			Name: "InterPodAffinity",
		},
	}

	cc, err := framework.NewSinglePod(kubeSchedulerConfig,
		nil,
		pod,
		100,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	objs := []runtime.Object{ns}
	for _, node := range nodes {
		objs = append(objs, node)
	}
	client := fakeclientset.NewSimpleClientset(objs...)

	if err := cc.SyncWithClient(client); err != nil {
		t.Errorf("Unable to sync resources: %v", err)
	}
	if err := cc.Run(); err != nil {
		t.Errorf("Unable to run analysis: %v", err)
	}

	t.Logf("Simulation stop reason: %v: %v", cc.Report().Status.FailReason.FailType, cc.Report().Status.FailReason.FailMessage)
	nodeDistribution := map[string]int{}
	for _, pod := range cc.ScheduledPods() {
		nodeDistribution[pod.Spec.NodeName]++
	}

	if len(nodeDistribution) > 1 {
		t.Fatalf("Expected all pods with pod affinity to be colocated on a single node, got %v nodes instead", len(nodeDistribution))
	}

	if len(nodeDistribution) < 1 {
		t.Fatalf("No pod scheduled")
	}

	t.Logf("All pods with affinity located on the same node")
}

func TestPodAffinityHardConstraintManyNodes(t *testing.T) {
	topologyDomainKey := "topology.domain/zone"

	setTopologyDomain := func(value string) func(*v1.Node) {
		return func(node *v1.Node) {
			node.Labels[topologyDomainKey] = value
		}
	}

	nodes := map[string]*v1.Node{
		"node1-1": BuildTestNode("node1-1", 1000, 1000, 30, setTopologyDomain("zone1")),
		"node1-2": BuildTestNode("node1-2", 1000, 1000, 30, setTopologyDomain("zone1")),
		"node1-3": BuildTestNode("node1-3", 1000, 1000, 30, setTopologyDomain("zone1")),

		"node2-1": BuildTestNode("node2-1", 1000, 1000, 30, setTopologyDomain("zone2")),
		"node2-2": BuildTestNode("node2-2", 1000, 1000, 30, setTopologyDomain("zone2")),
		"node2-3": BuildTestNode("node2-3", 1000, 1000, 30, setTopologyDomain("zone2")),

		"node3-1": BuildTestNode("node3-1", 1000, 1000, 30, setTopologyDomain("zone3")),
		"node3-2": BuildTestNode("node3-2", 1000, 1000, 30, setTopologyDomain("zone3")),
		"node3-3": BuildTestNode("node3-3", 1000, 1000, 30, setTopologyDomain("zone3")),
	}

	pod := BuildTestPod("pod-affinity", 10, 10, "", nil)
	pod.Labels["key"] = "value"
	pod.Spec.Affinity = &v1.Affinity{
		PodAffinity: &v1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					TopologyKey: topologyDomainKey,
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": "value",
						},
					},
				},
			},
		},
	}

	ns := BuildNamespace("default")

	kubeSchedulerConfig, err := utils.BuildKubeSchedulerCompletedConfig(nil)
	if err != nil {
		t.Fatal(err)
	}

	kubeSchedulerConfig.Config.ComponentConfig.Profiles[0].Plugins.Filter.Enabled = []kubeschedulerconfig.Plugin{
		{
			Name: "InterPodAffinity",
		},
		// To limit the number of pods per node
		{
			Name: "NodeResourcesFit",
		},
	}

	cc, err := framework.NewSinglePod(kubeSchedulerConfig,
		nil,
		pod,
		100,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	objs := []runtime.Object{ns}
	for _, node := range nodes {
		objs = append(objs, node)
	}
	client := fakeclientset.NewSimpleClientset(objs...)

	if err := cc.SyncWithClient(client); err != nil {
		t.Errorf("Unable to sync resources: %v", err)
	}
	if err := cc.Run(); err != nil {
		t.Errorf("Unable to run analysis: %v", err)
	}

	t.Logf("Simulation stop reason: %v: %v", cc.Report().Status.FailReason.FailType, cc.Report().Status.FailReason.FailMessage)
	nodeZoneDistribution := map[string]int{}
	for _, pod := range cc.ScheduledPods() {
		nodeZoneDistribution[nodes[pod.Spec.NodeName].Labels[topologyDomainKey]]++
	}

	if len(nodeZoneDistribution) > 1 {
		t.Fatalf("Expected all pods with pod affinity to be colocated in a single zone, got %v zones instead", len(nodeZoneDistribution))
	}

	if len(nodeZoneDistribution) < 1 {
		t.Fatalf("No pod scheduled")
	}

	t.Logf("All pods with affinity located in the same zone")
}

// BuildTestNode creates a node with specified capacity.
func BuildTestNode(name string, millicpu int64, mem int64, pods int64, apply func(*v1.Node)) *v1.Node {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:     name,
			SelfLink: fmt.Sprintf("/api/v1/nodes/%s", name),
			Labels:   map[string]string{},
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourcePods:   *resource.NewQuantity(pods, resource.DecimalSI),
				v1.ResourceCPU:    *resource.NewMilliQuantity(millicpu, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(mem, resource.DecimalSI),
			},
			Allocatable: v1.ResourceList{
				v1.ResourcePods:   *resource.NewQuantity(pods, resource.DecimalSI),
				v1.ResourceCPU:    *resource.NewMilliQuantity(millicpu, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(mem, resource.DecimalSI),
			},
			Phase: v1.NodeRunning,
			Conditions: []v1.NodeCondition{
				{Type: v1.NodeReady, Status: v1.ConditionTrue},
			},
		},
	}
	if apply != nil {
		apply(node)
	}
	return node
}

func setHostname() func(*v1.Node) {
	return func(node *v1.Node) {
		node.Labels["kubernetes.io/hostname"] = node.Name
	}
}

// BuildTestPod creates a test pod with given parameters.
func BuildTestPod(name string, cpu int64, memory int64, nodeName string, apply func(*v1.Pod)) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			SelfLink:  fmt.Sprintf("/api/v1/namespaces/default/pods/%s", name),
			Labels:    map[string]string{},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{},
						Limits:   v1.ResourceList{},
					},
				},
			},
			NodeName: nodeName,
		},
	}
	if cpu >= 0 {
		pod.Spec.Containers[0].Resources.Requests[v1.ResourceCPU] = *resource.NewMilliQuantity(cpu, resource.DecimalSI)
	}
	if memory >= 0 {
		pod.Spec.Containers[0].Resources.Requests[v1.ResourceMemory] = *resource.NewQuantity(memory, resource.DecimalSI)
	}
	if apply != nil {
		apply(pod)
	}
	return pod
}

func BuildNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:     name,
			SelfLink: fmt.Sprintf("/api/v1/namespaces/%s", name),
		},
	}
}
