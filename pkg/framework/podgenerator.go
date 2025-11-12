/*
Copyright 2022 The Kubernetes Authors.

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
