package apiserver

import (
	"fmt"
	"time"
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
	Cpu    float64
	Memory int64
}

type ReportTotal struct {
	Instances int
	Reason    string
}

type ReportNode struct {
	NodeName  string
	Instances int
	Reason    string
}

type Report struct {
	Timestamp       time.Time
	PodRequirements PodResources
	Total           ReportTotal
	Nodes           []ReportNode
}

func (r *Report) Print(verbose bool) {
	if verbose {
		fmt.Printf("%vPod requirements:%v\n", CLR_W, CLR_N)
		fmt.Printf("\t- cpu: %v\n", r.PodRequirements.Cpu)
		fmt.Printf("\t- memory: %v\n", r.PodRequirements.Memory)
		fmt.Printf("\n")
	}

	fmt.Printf("The cluster can schedule %v%v%v instance(s) of the pod.\n", CLR_W, r.Total.Instances, CLR_N)
	fmt.Printf("%vTermination reason%v: %v\n", CLR_G, CLR_N, r.Total.Reason)

	if verbose && r.Total.Instances > 0 {
		fmt.Printf("\nPod distribution among nodes:\n")
		for _, node := range r.Nodes {
			fmt.Printf("\t- %v: %v instance(s)\n", node.NodeName, node.Instances)
		}
	}
}
