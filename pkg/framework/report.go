package framework

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
)

const CLR_0 = "\x1b[30;1m"
const CLR_R = "\x1b[31;1m"
const CLR_G = "\x1b[32;1m"
const CLR_Y = "\x1b[33;1m"
const CLR_B = "\x1b[34;1m"
const CLR_M = "\x1b[35;1m"
const CLR_C = "\x1b[36;1m"
const CLR_W = "\x1b[37;1m"
const CLR_N = "\x1b[0m"

type PodResources struct {
	CPU                *resource.Quantity
	Memory             *resource.Quantity
	NvidiaGPU          *resource.Quantity
	OpaqueIntResources map[api.ResourceName]int64
}

type FailReason struct {
	FailType     string
	FailMessage  string
	NodeFailures map[string]string
}

type Report struct {
	Timestamp         time.Time
	PodRequirements   *PodResources
	TotalInstances    int
	NodesNumInstances map[string]int
	FailReasons       FailReason
}

func CreateFullReport(pod *api.Pod, status Status) *Report {
	report := createReport(pod, status)
	if len(status.Pods) == 0 {
		return report
	}
	report.NodesNumInstances = make(map[string]int)
	for _, pod := range status.Pods {
		_, ok := report.NodesNumInstances[pod.Spec.NodeName]
		if !ok {
			report.NodesNumInstances[pod.Spec.NodeName] = 1
		} else {
			report.NodesNumInstances[pod.Spec.NodeName]++
		}
	}

	return report
}

func getReason(message string) FailReason {
	slicedMessage := strings.Split(message, "\n")
	colon := strings.Index(slicedMessage[0], ":")

	fail := FailReason{
		FailType:    slicedMessage[0][:colon],
		FailMessage: strings.Trim(slicedMessage[0][colon+1:], " "),
	}

	if len(slicedMessage) == 1 {
		return fail
	}

	fail.NodeFailures = make(map[string]string)
	for _, nodeReason := range slicedMessage[1:] {
		if len(nodeReason) < 4 {
			continue
		}
		nameStart := strings.Index(nodeReason, "(")
		nameEnd := strings.Index(nodeReason, ")")
		name := nodeReason[nameStart+1 : nameEnd]

		parts := strings.Split(nodeReason, ":")
		if len(parts) != 2 {
			fail.NodeFailures[name] = nodeReason[nameEnd+3:]
		} else {
			fail.NodeFailures[name] = strings.Trim(parts[1], " ")
		}
	}
	return fail
}

func GetResourceRequest(pod *api.Pod) *PodResources {
	result := PodResources{
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

func createReport(pod *api.Pod, status Status) *Report {
	return &Report{
		//TODO: set this sooner(right after the check is done)
		Timestamp:       time.Now(),
		PodRequirements: GetResourceRequest(pod),
		TotalInstances:  len(status.Pods),
		FailReasons:     getReason(status.StopReason),
	}
}

func (r *Report) prettyPrint(verbose bool) {
	if verbose {
		fmt.Printf("%vPod requirements:%v\n", CLR_W, CLR_N)
		fmt.Printf("\t- CPU: %v\n", r.PodRequirements.CPU.String())
		fmt.Printf("\t- Memory: %v\n", r.PodRequirements.Memory.String())
		if !r.PodRequirements.NvidiaGPU.IsZero() {
			fmt.Printf("\t- NvidiaGPU: %v\n", r.PodRequirements.NvidiaGPU.String())
		}
		if r.PodRequirements.OpaqueIntResources != nil {
			fmt.Printf("\t- OpaqueIntResources: %v\n", r.PodRequirements.OpaqueIntResources)
		}
		fmt.Printf("\n")
	}

	fmt.Printf("The cluster can schedule %v%v%v instance(s) of the pod.\n", CLR_W, r.TotalInstances, CLR_N)
	fmt.Printf("%vTermination reason%v: %v: %v\n", CLR_G, CLR_N, r.FailReasons.FailType, r.FailReasons.FailMessage)

	if verbose && r.TotalInstances > 0 {
		for node, fail := range r.FailReasons.NodeFailures {
			fmt.Printf("fit failure on node (%v): %v\n", node, fail)
		}
		fmt.Printf("\nPod distribution among nodes:\n")
		for node, instances := range r.NodesNumInstances {
			fmt.Printf("\t- %v: %v instance(s)\n", node, instances)
		}
	}
}

func (r *Report) printJson() error {
	jsoned, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("Failed to create json: %v", err)
	}
	fmt.Println(string(jsoned))
	return nil
}

func (r *Report) printYaml() error {
	yamled, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("Failed to create yaml: %v", err)
	}
	fmt.Print(string(yamled))
	return nil
}

func (r *Report) Print(verbose bool, format string) error {
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
