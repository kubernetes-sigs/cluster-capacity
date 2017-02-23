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
	"strconv"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"k8s.io/kubernetes/pkg/api"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterCapacityReview struct {
	metav1.TypeMeta
	Spec   ClusterCapacityReviewSpec
	Status ClusterCapacityReviewStatus
}

type ClusterCapacityReviewSpec struct {
	// the pod desired for scheduling
	Templates []api.Pod

	// desired number of replicas that should be scheduled
	// +optional
	Replicas int32

	PodRequirements []*Requirements
}

type ClusterCapacityReviewStatus struct {
	CreationTimestamp time.Time
	// actual number of replicas that could schedule
	Replicas int32

	FailReason *ClusterCapacityReviewScheduleFailReason

	// per node information about the scheduling simulation
	Pods []*ClusterCapacityReviewResult
}

type ClusterCapacityReviewResult struct {
	PodName string
	// numbers of replicas on nodes
	ReplicasOnNodes map[string]int
	// reason why no more pods could schedule (if any on this node)
	// [reason]num of nodes with that reason
	FailSummary map[string]int
}

type Resources struct {
	CPU                *resource.Quantity
	Memory             *resource.Quantity
	NvidiaGPU          *resource.Quantity
	OpaqueIntResources map[api.ResourceName]int64
}

type Requirements struct {
	PodName       string
	Resources     *Resources
	NodeSelectors map[string]string
}

type ClusterCapacityReviewScheduleFailReason struct {
	FailType    string
	FailMessage string
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

func getResourceRequest(pod *api.Pod) *Resources {
	result := Resources{
		CPU:       resource.NewMilliQuantity(0, resource.DecimalSI),
		Memory:    resource.NewQuantity(0, resource.BinarySI),
		NvidiaGPU: resource.NewMilliQuantity(0, resource.DecimalSI),
	}
	for _, container := range pod.Spec.Containers {
		for rName, rQuantity := range container.Resources.Requests {
			switch rName {
			case api.ResourceMemory:
				result.Memory.Add(rQuantity)
			case api.ResourceCPU:
				result.CPU.Add(rQuantity)
			case api.ResourceNvidiaGPU:
				result.NvidiaGPU.Add(rQuantity)
			default:
				if api.IsOpaqueIntResourceName(rName) {
					// Lazily allocate this map only if required.
					if result.OpaqueIntResources == nil {
						result.OpaqueIntResources = map[api.ResourceName]int64{}
					}
					result.OpaqueIntResources[rName] += rQuantity.Value()
				}
			}
		}
	}
	return &result
}

func parsePodsReview(templatePods []*api.Pod, status Status) []*ClusterCapacityReviewResult {
	templatesCount := len(templatePods)
	result := make([]*ClusterCapacityReviewResult, 0)

	for i := 0; i < templatesCount; i++ {
		result = append(result, &ClusterCapacityReviewResult{
			ReplicasOnNodes: make(map[string]int),
			PodName:         templatePods[i].Name,
		})
	}

	for i, pod := range status.Pods {
		nodeName := pod.Spec.NodeName
		result[i%templatesCount].ReplicasOnNodes[nodeName]++
	}

	slicedMessage := strings.Split(status.StopReason, "\n")
	if len(slicedMessage) == 1 {
		return result
	}

	slicedMessage = strings.Split(slicedMessage[1][31:], `, `)
	allReasons := make(map[string]int)
	for _, nodeReason := range slicedMessage {
		leftParenthesis := strings.LastIndex(nodeReason, `(`)

		reason := nodeReason[:leftParenthesis-1]
		replicas, _ := strconv.Atoi(nodeReason[leftParenthesis+1 : len(nodeReason)-1])
		allReasons[reason] = replicas
	}

	result[(len(status.Pods)-1)%templatesCount].FailSummary = allReasons
	return result
}

func getPodsRequirements(pods []*api.Pod) []*Requirements {
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

func deepCopyPods(in []*api.Pod, out []api.Pod) {
	cloner := conversion.NewCloner()
	for i, pod := range in {
		api.DeepCopy_api_Pod(pod, &out[i], cloner)
	}
}

func getReviewSpec(podTemplates []*api.Pod) ClusterCapacityReviewSpec {

	podCopies := make([]api.Pod, len(podTemplates))
	deepCopyPods(podTemplates, podCopies)
	return ClusterCapacityReviewSpec{
		Templates:       podCopies,
		PodRequirements: getPodsRequirements(podTemplates),
	}
}

func getReviewStatus(pods []*api.Pod, status Status) ClusterCapacityReviewStatus {
	return ClusterCapacityReviewStatus{
		CreationTimestamp: time.Now(),
		Replicas:          int32(len(status.Pods)),
		FailReason:        getMainFailReason(status.StopReason),
		Pods:              parsePodsReview(pods, status),
	}
}

func GetReport(pods []*api.Pod, status Status) *ClusterCapacityReview {
	return &ClusterCapacityReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterCapacityReview",
			APIVersion: "",
		},
		Spec:   getReviewSpec(pods),
		Status: getReviewStatus(pods, status),
	}
}

func instancesSum(replicasOnNodes map[string]int) int {
	result := 0
	for _, v := range replicasOnNodes {
		result += v
	}
	return result
}

func (r *ClusterCapacityReview) prettyPrint(verbose bool) {
	if verbose {
		for _, req := range r.Spec.PodRequirements {
			fmt.Printf("%v pod requirements:\n", req.PodName)
			fmt.Printf("\t- CPU: %v\n", req.Resources.CPU.String())
			fmt.Printf("\t- Memory: %v\n", req.Resources.Memory.String())
			if !req.Resources.NvidiaGPU.IsZero() {
				fmt.Printf("\t- NvidiaGPU: %v\n", req.Resources.NvidiaGPU.String())
			}
			if req.Resources.OpaqueIntResources != nil {
				fmt.Printf("\t- OpaqueIntResources: %v\n", req.Resources.OpaqueIntResources)
			}

			if req.NodeSelectors != nil {
				fmt.Printf("\t- NodeSelector: %v\n", labels.SelectorFromSet(labels.Set(req.NodeSelectors)).String())
			}
			fmt.Printf("\n")
		}
	}

	for _, pod := range r.Status.Pods {
		fmt.Printf("The cluster can schedule %v instance(s) of the pod %v.\n", instancesSum(pod.ReplicasOnNodes), pod.PodName)
	}
	fmt.Printf("\nTermination reason: %v: %v\n", r.Status.FailReason.FailType, r.Status.FailReason.FailMessage)

	if verbose && r.Status.Replicas > 0 {
		for _, pod := range r.Status.Pods {
			if pod.FailSummary != nil {
				fmt.Printf("fit failure summary on nodes: ")
				for reason, occurence := range pod.FailSummary {
					fmt.Printf("%v (%v), ", reason, occurence)
				}
				fmt.Printf("\n")
			}
		}
		fmt.Printf("\nPod distribution among nodes:\n")
		for _, pod := range r.Status.Pods {
			fmt.Printf("%v\n", pod.PodName)
			for node, replicas := range pod.ReplicasOnNodes {
				fmt.Printf("\t- %v: %v instance(s)\n", node, replicas)
			}
		}
	}
}

func (r *ClusterCapacityReview) printJson() error {
	jsoned, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("Failed to create json: %v", err)
	}
	fmt.Println(string(jsoned))
	return nil
}

func (r *ClusterCapacityReview) printYaml() error {
	yamled, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("Failed to create yaml: %v", err)
	}
	fmt.Print(string(yamled))
	return nil
}

func (r *ClusterCapacityReview) Print(verbose bool, format string) error {
	switch format {
	case "json":
		return r.printJson()
	case "yaml":
		return r.printYaml()
	case "":
		r.prettyPrint(verbose)
		return nil
	default:
		return fmt.Errorf("output format %q not recognized", format)
	}
}
