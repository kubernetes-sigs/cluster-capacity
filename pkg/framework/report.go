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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/apis/core/v1/helper"
)

const (
	ResourceNvidiaGPU v1.ResourceName = "nvdia.com/gpu"
)

type ClusterCapacityReview struct {
	metav1.TypeMeta
	Spec   ClusterCapacityReviewSpec   `json:"spec"`
	Status ClusterCapacityReviewStatus `json:"status"`
}

type ClusterCapacityReviewSpec struct {
	// the pod desired for scheduling
	Templates []v1.Pod `json:"templates"`

	// desired number of replicas that should be scheduled
	// +optional
	Replicas int32 `json:"replicas"`

	PodRequirements []*Requirements `json:"podRequirements"`
}

type ClusterCapacityReviewStatus struct {
	CreationTimestamp time.Time `json:"creationTimestamp"`
	// actual number of replicas that could schedule
	Replicas int32 `json:"replicas"`

	FailReason *ClusterCapacityReviewScheduleFailReason `json:"failReason"`

	// per node information about the scheduling simulation
	Pods []*ClusterCapacityReviewResult `json:"pods"`
}

type ClusterCapacityReviewResult struct {
	PodName string `json:"podName"`
	// numbers of replicas on nodes
	ReplicasOnNodes []*ReplicasOnNode `json:"replicasOnNodes"`
	// reason why no more pods could schedule (if any on this node)
	FailSummary []FailReasonSummary `json:"failSummary"`
}

type ReplicasOnNode struct {
	NodeName string `json:"nodeName"`
	Replicas int    `json:"replicas"`
}

type FailReasonSummary struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type Resources struct {
	PrimaryResources v1.ResourceList           `json:"primaryResources"`
	ScalarResources  map[v1.ResourceName]int64 `json:"scalarResources"`
}

type Requirements struct {
	PodName       string            `json:"podName"`
	Resources     *Resources        `json:"resources"`
	NodeSelectors map[string]string `json:"nodeSelectors"`
}

type ClusterCapacityReviewScheduleFailReason struct {
	FailType    string `json:"failType"`
	FailMessage string `json:"failMessage"`
}

func getMainFailReason(message string) *ClusterCapacityReviewScheduleFailReason {
	slicedMessage := strings.Split(message, "\n")
	colon := strings.Index(slicedMessage[0], ":")

	fail := &ClusterCapacityReviewScheduleFailReason{
		FailType:    slicedMessage[0][:colon],
		FailMessage: strings.Trim(slicedMessage[0][colon+1:], " "),
	}
	return fail
}

func getResourceRequest(pod *v1.Pod) *Resources {
	result := Resources{
		PrimaryResources: v1.ResourceList{
			v1.ResourceName(v1.ResourceCPU):    *resource.NewMilliQuantity(0, resource.DecimalSI),
			v1.ResourceName(v1.ResourceMemory): *resource.NewQuantity(0, resource.BinarySI),
			v1.ResourceName(ResourceNvidiaGPU): *resource.NewMilliQuantity(0, resource.DecimalSI),
		},
	}

	for _, container := range pod.Spec.Containers {
		for rName, rQuantity := range container.Resources.Requests {
			switch rName {
			case v1.ResourceMemory:
				rQuantity.Add(*(result.PrimaryResources.Memory()))
				result.PrimaryResources[v1.ResourceMemory] = rQuantity
			case v1.ResourceCPU:
				rQuantity.Add(*(result.PrimaryResources.Cpu()))
				result.PrimaryResources[v1.ResourceCPU] = rQuantity
				//case v1.ResourceNvidiaGPU:
				//	rQuantity.Add(*(result.PrimaryResources.NvidiaGPU()))
				//	result.PrimaryResources[v1.ResourceNvidiaGPU] = rQuantity
			default:
				if helper.IsScalarResourceName(rName) {
					// Lazily allocate this map only if required.
					if result.ScalarResources == nil {
						result.ScalarResources = map[v1.ResourceName]int64{}
					}
					result.ScalarResources[rName] += rQuantity.Value()
				}
			}
		}
	}
	return &result
}

func parsePodsReview(templatePods []*v1.Pod, status Status) []*ClusterCapacityReviewResult {
	templatesCount := len(templatePods)
	result := make([]*ClusterCapacityReviewResult, 0)

	for i := 0; i < templatesCount; i++ {
		result = append(result, &ClusterCapacityReviewResult{
			ReplicasOnNodes: make([]*ReplicasOnNode, 0),
			PodName:         templatePods[i].Name,
		})
	}

	for i, pod := range status.Pods {
		nodeName := pod.Spec.NodeName
		first := true
		for _, sum := range result[i%templatesCount].ReplicasOnNodes {
			if sum.NodeName == nodeName {
				sum.Replicas++
				first = false
			}
		}
		if first {
			result[i%templatesCount].ReplicasOnNodes = append(result[i%templatesCount].ReplicasOnNodes, &ReplicasOnNode{
				NodeName: nodeName,
				Replicas: 1,
			})
		}
	}

	slicedMessage := strings.Split(status.StopReason, "\n")
	if len(slicedMessage) == 1 {
		return result
	}

	return result
}

func getPodsRequirements(pods []*v1.Pod) []*Requirements {
	result := make([]*Requirements, 0)
	for _, pod := range pods {
		podRequirements := &Requirements{
			PodName:       pod.Name,
			Resources:     getResourceRequest(pod),
			NodeSelectors: pod.Spec.NodeSelector,
		}
		result = append(result, podRequirements)
	}
	return result
}

func deepCopyPods(in []*v1.Pod, out []v1.Pod) {
	for i, pod := range in {
		out[i] = *pod.DeepCopy()
	}
}

func getReviewSpec(podTemplates []*v1.Pod) ClusterCapacityReviewSpec {

	podCopies := make([]v1.Pod, len(podTemplates))
	deepCopyPods(podTemplates, podCopies)
	return ClusterCapacityReviewSpec{
		Templates:       podCopies,
		PodRequirements: getPodsRequirements(podTemplates),
	}
}

func getReviewStatus(pods []*v1.Pod, status Status) ClusterCapacityReviewStatus {
	return ClusterCapacityReviewStatus{
		CreationTimestamp: time.Now(),
		Replicas:          int32(len(status.Pods)),
		FailReason:        getMainFailReason(status.StopReason),
		Pods:              parsePodsReview(pods, status),
	}
}

func GetReport(pods []*v1.Pod, status Status) *ClusterCapacityReview {
	return &ClusterCapacityReview{
		Spec:   getReviewSpec(pods),
		Status: getReviewStatus(pods, status),
	}
}

func instancesSum(replicasOnNodes []*ReplicasOnNode) int {
	result := 0
	for _, v := range replicasOnNodes {
		result += v.Replicas
	}
	return result
}

func clusterCapacityReviewPrettyPrint(r *ClusterCapacityReview, verbose bool) {
	if verbose {
		for _, req := range r.Spec.PodRequirements {
			fmt.Printf("%v pod requirements:\n", req.PodName)
			fmt.Printf("\t- CPU: %v\n", req.Resources.PrimaryResources.Cpu().String())
			fmt.Printf("\t- Memory: %v\n", req.Resources.PrimaryResources.Memory().String())
			//if !req.Resources.PrimaryResources.NvidiaGPU().IsZero() {
			//	fmt.Printf("\t- NvidiaGPU: %v\n", req.Resources.PrimaryResources.NvidiaGPU().String())
			//}
			if req.Resources.ScalarResources != nil {
				fmt.Printf("\t- ScalarResources: %v\n", req.Resources.ScalarResources)
			}

			if req.NodeSelectors != nil {
				fmt.Printf("\t- NodeSelector: %v\n", labels.SelectorFromSet(labels.Set(req.NodeSelectors)).String())
			}
			fmt.Printf("\n")
		}
	}

	for _, pod := range r.Status.Pods {
		if verbose {
			fmt.Printf("The cluster can schedule %v instance(s) of the pod %v.\n", instancesSum(pod.ReplicasOnNodes), pod.PodName)
		} else {
			fmt.Printf("%v\n", instancesSum(pod.ReplicasOnNodes))
		}
	}

	if verbose {
		fmt.Printf("\nTermination reason: %v: %v\n", r.Status.FailReason.FailType, r.Status.FailReason.FailMessage)
	}

	if verbose && r.Status.Replicas > 0 {
		for _, pod := range r.Status.Pods {
			if pod.FailSummary != nil {
				fmt.Printf("fit failure summary on nodes: ")
				for _, fs := range pod.FailSummary {
					fmt.Printf("%v (%v), ", fs.Reason, fs.Count)
				}
				fmt.Printf("\n")
			}
		}
		fmt.Printf("\nPod distribution among nodes:\n")
		for _, pod := range r.Status.Pods {
			fmt.Printf("%v\n", pod.PodName)
			for _, ron := range pod.ReplicasOnNodes {
				fmt.Printf("\t- %v: %v instance(s)\n", ron.NodeName, ron.Replicas)
			}
		}
	}
}

func clusterCapacityReviewPrintJson(r *ClusterCapacityReview) error {
	jsoned, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("Failed to create json: %v", err)
	}
	fmt.Println(string(jsoned))
	return nil
}

func clusterCapacityReviewPrintYaml(r *ClusterCapacityReview) error {
	yamled, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("Failed to create yaml: %v", err)
	}
	fmt.Print(string(yamled))
	return nil
}

func ClusterCapacityReviewPrint(r *ClusterCapacityReview, verbose bool, format string) error {
	switch format {
	case "json":
		return clusterCapacityReviewPrintJson(r)
	case "yaml":
		return clusterCapacityReviewPrintYaml(r)
	case "":
		clusterCapacityReviewPrettyPrint(r, verbose)
		return nil
	default:
		return fmt.Errorf("output format %q not recognized", format)
	}
}
