package framework

import (
	"fmt"

	uuid "github.com/satori/go.uuid"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type PodGenerator interface {
	Generate() (*v1.Pod, int, bool)
}

type singlePodGenerator struct {
	counter     int
	podTemplate *v1.Pod
	maxPods     int
}

func NewSinglePodGenerator(podTemplate *v1.Pod, maxPods int) PodGenerator {
	return &singlePodGenerator{
		counter:     0,
		podTemplate: podTemplate,
		maxPods:     maxPods,
	}
}

func (g *singlePodGenerator) Generate() (*v1.Pod, int, bool) {
	if g.maxPods != 0 && g.counter > g.maxPods {
		return nil, g.counter, true
	}
	pod := g.podTemplate.DeepCopy()

	// reset any node designation set
	pod.Spec.NodeName = ""

	// use simulated pod name with an index to construct the name
	pod.ObjectMeta.Name = fmt.Sprintf("%v-%v", g.podTemplate.Name, g.counter)
	pod.ObjectMeta.UID = types.UID(uuid.NewV4().String())

	// Add pod provisioner annotation
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}

	// Ensures uniqueness
	g.counter++

	return pod, g.counter, false
}
