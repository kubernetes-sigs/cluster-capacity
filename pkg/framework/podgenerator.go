package framework

import (
	"fmt"

	uuid "github.com/satori/go.uuid"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type PodGenerator interface {
	Generate() *v1.Pod
}

type singlePodGenerator struct {
	counter     uint
	podTemplate *v1.Pod
}

func NewSinglePodGenerator(podTemplate *v1.Pod) PodGenerator {
	return &singlePodGenerator{
		counter:     0,
		podTemplate: podTemplate,
	}
}

func (g *singlePodGenerator) Generate() *v1.Pod {
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

	return pod
}
