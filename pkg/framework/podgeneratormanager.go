package framework

import (
	v1 "k8s.io/api/core/v1"
)

type podGeneratorManager struct {
	podGenerators []PodGenerator
	currentIndex  int
	total         int
}

func NewPodGeneratorManager(podGenerators []PodGenerator) PodGenerator {
	return &podGeneratorManager{
		currentIndex:  0,
		podGenerators: podGenerators,
		total:         0,
	}
}

func (g *podGeneratorManager) Generate() (*v1.Pod, int, bool) {
	if g.podGenerators == nil || g.currentIndex >= len(g.podGenerators) {
		return nil, g.total, true
	}

	genPod, genTotal, genFinish := g.podGenerators[g.currentIndex].Generate()
	if genFinish {
		g.currentIndex = g.currentIndex + 1
		g.total = g.total + genTotal
		return g.Generate()
	} else {
		return genPod, g.total, false
	}
}
