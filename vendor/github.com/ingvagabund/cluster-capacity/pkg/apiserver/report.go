package apiserver

import (
	"fmt"
	"time"
	"encoding/json"
	"github.com/ghodss/yaml"
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

type FailReason struct {
	FailType string
	FailMessage string
	NodeFailures map[string]string

}

type Report struct {
	Timestamp         time.Time
	PodRequirements   PodResources
	TotalInstances    int
	NodesNumInstances map[string]int
	FailReasons    	  FailReason
}

func (r *Report) prettyPrint(verbose bool) {
	if verbose {
		fmt.Printf("%vPod requirements:%v\n", CLR_W, CLR_N)
		fmt.Printf("\t- cpu: %v\n", r.PodRequirements.Cpu)
		fmt.Printf("\t- memory: %v\n", r.PodRequirements.Memory)
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
	jsoned, err:= json.Marshal(r)
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
