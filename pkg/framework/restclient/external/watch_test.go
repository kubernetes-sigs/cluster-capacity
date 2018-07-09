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

package external

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	ccapi "github.com/kubernetes-incubator/cluster-capacity/pkg/api"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/test"
)

func newTestWatchRestClient() *RESTClient {
	return NewRESTClient(&store.FakeResourceStore{}, "")
}

func getResourceWatcher(client cache.Getter, resource ccapi.ResourceType) watch.Interface {
	// client listerWatcher
	listerWatcher := cache.NewListWatchFromClient(client, resource.String(), metav1.NamespaceAll, fields.ParseSelectorOrDie(""))
	// ask for watcher data
	timemoutseconds := int64(10)

	options := metav1.ListOptions{
		ResourceVersion: "0",
		// We want to avoid situations of hanging watchers. Stop any wachers that do not
		// receive any events within the timeout window.
		TimeoutSeconds: &timemoutseconds,
	}

	w, _ := listerWatcher.Watch(options)
	return w
}

func emitEvent(client *RESTClient, resource ccapi.ResourceType, test eventTest) {
	switch resource {
	case ccapi.Pods:
		client.EmitObjectWatchEvent(resource, test.event, test.item.(*v1.Pod))
	case ccapi.Services:
		client.EmitObjectWatchEvent(resource, test.event, test.item.(*v1.Service))
	case ccapi.PersistentVolumes:
		client.EmitObjectWatchEvent(resource, test.event, test.item.(*v1.PersistentVolume))
	case ccapi.Nodes:
		client.EmitObjectWatchEvent(resource, test.event, test.item.(*v1.Node))
	case ccapi.PersistentVolumeClaims:
		client.EmitObjectWatchEvent(resource, test.event, test.item.(*v1.PersistentVolumeClaim))
	default:
		fmt.Printf("Unsupported resource %s", resource)
		// TODO(jchaloup): log the error
	}
}

type eventTest struct {
	event watch.EventType
	item  interface{}
}

func testWatch(tests []eventTest, resource ccapi.ResourceType, t *testing.T) {

	client := newTestWatchRestClient()
	w := getResourceWatcher(client, resource)

	t.Logf("Emitting first two events")
	emitEvent(client, resource, tests[0])
	emitEvent(client, resource, tests[1])
	// wait for a while so both events are in one byte stream
	time.Sleep(10 * time.Millisecond)
	sync := make(chan struct{})

	// retrieve all events one by one in the same order
	go func() {
		for _, test := range tests {
			t.Logf("Waiting for event")
			event, ok := <-w.ResultChan()
			if !ok {
				t.Errorf("Unexpected watch close")
			}
			t.Logf("Event received")
			if event.Type != test.event {
				t.Errorf("Expected event type %q, got %q", test.event, event.Type)
			}
			if !reflect.DeepEqual(test.item, event.Object) {
				t.Errorf("unexpected object:\n\n expected: %#v\n actual  : %#v", test.item, event.Object)
			}
		}
		sync <- struct{}{}
	}()

	// send remaining events
	t.Logf("Emitting remaining events")
	for _, test := range tests[2:] {
		time.Sleep(10 * time.Millisecond)
		emitEvent(client, resource, test)
		t.Logf("Event emitted")
	}

	// wait for all events
	<-sync
	close(sync)
	client.Close()
}

func TestWatchPods(t *testing.T) {

	pod1 := test.PodExample("pod1")
	pod2 := test.PodExample("pod2")
	pod3 := test.PodExample("pod3")
	pod4 := test.PodExample("pod4")

	tests := []eventTest{
		{
			event: watch.Modified,
			item:  &pod1,
		},
		{
			event: watch.Added,
			item:  &pod2,
		},
		{
			event: watch.Modified,
			item:  &pod3,
		},
		{
			event: watch.Deleted,
			item:  &pod4,
		},
	}

	testWatch(tests, ccapi.Pods, t)
}

func TestWatchServices(t *testing.T) {

	service1 := test.ServiceExample("service1")
	service2 := test.ServiceExample("service2")
	service3 := test.ServiceExample("service3")
	service4 := test.ServiceExample("service4")

	tests := []eventTest{
		{
			event: watch.Modified,
			item:  &service1,
		},
		{
			event: watch.Added,
			item:  &service2,
		},
		{
			event: watch.Modified,
			item:  &service3,
		},
		{
			event: watch.Deleted,
			item:  &service4,
		},
	}

	testWatch(tests, ccapi.Services, t)
}

func TestWatchPersistentVolumes(t *testing.T) {
	pv1 := test.PersistentVolumeExample("persistentvolume1")
	pv2 := test.PersistentVolumeExample("persistentvolume2")
	pv3 := test.PersistentVolumeExample("persistentvolume3")
	pv4 := test.PersistentVolumeExample("persistentvolume4")

	tests := []eventTest{
		{
			event: watch.Modified,
			item:  &pv1,
		},
		{
			event: watch.Added,
			item:  &pv2,
		},
		{
			event: watch.Modified,
			item:  &pv3,
		},
		{
			event: watch.Deleted,
			item:  &pv4,
		},
	}

	testWatch(tests, ccapi.PersistentVolumes, t)
}

func TestWatchPersistentVolumeClaims(t *testing.T) {
	pvc1 := test.PersistentVolumeClaimExample("persistentVolumeClaim1")
	pvc2 := test.PersistentVolumeClaimExample("persistentVolumeClaim2")
	pvc3 := test.PersistentVolumeClaimExample("persistentVolumeClaim3")
	pvc4 := test.PersistentVolumeClaimExample("persistentVolumeClaim4")

	tests := []eventTest{
		{
			event: watch.Modified,
			item:  &pvc1,
		},
		{
			event: watch.Added,
			item:  &pvc2,
		},
		{
			event: watch.Modified,
			item:  &pvc3,
		},
		{
			event: watch.Deleted,
			item:  &pvc4,
		},
	}

	testWatch(tests, ccapi.PersistentVolumeClaims, t)
}

func TestWatchNodes(t *testing.T) {
	node1 := test.NodeExample("node1")
	node2 := test.NodeExample("node2")
	node3 := test.NodeExample("node3")
	node4 := test.NodeExample("node4")

	tests := []eventTest{
		{
			event: watch.Modified,
			item:  &node1,
		},
		{
			event: watch.Added,
			item:  &node2,
		},
		{
			event: watch.Modified,
			item:  &node3,
		},
		{
			event: watch.Deleted,
			item:  &node4,
		},
	}

	testWatch(tests, ccapi.Nodes, t)
}
