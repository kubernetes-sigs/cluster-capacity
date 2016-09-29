package emulator

import (
	"testing"
	"k8s.io/kubernetes/pkg/client/cache"
	"fmt"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/watch"
	"time"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/api"

)

func newTestRestClient() *RESTClient {

	client := &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
	}

	return client
}

// TODO:
// - add one event, collect one event, test for equality, repeat
// - add two events to buffer and then test both events are decoded one by one
// - add one event, collect one event, close the watcher and check correct error is returned

func TestWatchPods(t *testing.T) {
	client := newTestRestClient()
	// client listerWatcher
	listerWatcher := cache.NewListWatchFromClient(client, "pods", api.NamespaceAll, fields.ParseSelectorOrDie(""))
	// ask for watcher data
	timemoutseconds := int64(1)

	options := api.ListOptions{
		ResourceVersion: "0",
		// We want to avoid situations of hanging watchers. Stop any wachers that do not
		// receive any events within the timeout window.
		TimeoutSeconds: &timemoutseconds,
	}

	w, _ := listerWatcher.Watch(options)

	item := &api.Pod{
		ObjectMeta: api.ObjectMeta{Name: "pod1", Namespace: "test", ResourceVersion: "10"},
		Spec:       apitesting.DeepEqualSafePodSpec(),
	}

	fmt.Println("Emitting two events")
	client.EmitPodWatchEvent(watch.Modified, item)
	client.EmitPodWatchEvent(watch.Added, item)
	time.Sleep(time.Second)

	go func() {
		//select {
		//	case event, ok := <-w.ResultChan():
		//		fmt.Printf("event type: %s", event.Type)
		//}
		for i:=0; i < 6; i++ {
			fmt.Println("Waiting for event")
			event := <-w.ResultChan()
			fmt.Printf("event type: %s\n", event.Type)
		}
		fmt.Println("Watch ended")
	}()

	// send watch data
	time.Sleep(2*time.Second)
	client.EmitPodWatchEvent(watch.Modified, item)
	time.Sleep(time.Second)
	client.EmitPodWatchEvent(watch.Added, item)
	time.Sleep(time.Second)
}
