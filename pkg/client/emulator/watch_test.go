package emulator

import (
	"testing"
	"k8s.io/kubernetes/pkg/client/cache"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/watch"
	"time"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/api"
	"reflect"
)

func newTestRestClient() *RESTClient {
	return &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
	}
}

func getResourceWatcher(client cache.Getter, resource string) watch.Interface {
	// client listerWatcher
	listerWatcher := cache.NewListWatchFromClient(client, resource, api.NamespaceAll, fields.ParseSelectorOrDie(""))
	// ask for watcher data
	timemoutseconds := int64(10)

	options := api.ListOptions{
		ResourceVersion: "0",
		// We want to avoid situations of hanging watchers. Stop any wachers that do not
		// receive any events within the timeout window.
		TimeoutSeconds: &timemoutseconds,
	}

	w, _ := listerWatcher.Watch(options)
	return w
}

func emitEvent(client *RESTClient, resource string, test eventTest) {
	switch resource {
		case "pods":
			client.EmitPodWatchEvent(test.event, test.item.(*api.Pod))
	}
}

type eventTest struct{
	event watch.EventType
	item interface{}
}

func testWatch(tests []eventTest, resource string, t *testing.T) {

	client := newTestRestClient()
	w := getResourceWatcher(client, resource)

	t.Logf("Emitting first two events")
	emitEvent(client, resource, tests[0])
	emitEvent(client, resource, tests[1])
	// wait for a while so both events are in one byte stream
	time.Sleep(time.Second)
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
				t.Errorf("unexpected object: expected: %#v\n actual: %#v", test.item, event.Object)
			}

		}
		sync<- struct{}{}
	}()

	// send remaining events
	t.Logf("Emitting remaining events")
	for _, test := range tests[2:] {
		time.Sleep(time.Second)
		emitEvent(client, resource, test)
		t.Logf("Event emitted")
	}

	// wait for all events
	<-sync
}

func TestWatchPods(t *testing.T) {

	pod := &api.Pod{
		ObjectMeta: api.ObjectMeta{Name: "pod1", Namespace: "test", ResourceVersion: "10"},
		Spec:       apitesting.DeepEqualSafePodSpec(),
	}

	tests := []eventTest{
		{
			event: watch.Modified,
			item: pod,
		},
		{
			event: watch.Added,
			item: pod,
		},
		{
			event: watch.Modified,
			item: pod,
		},
		{
			event: watch.Deleted,
			item: pod,
		},
	}

	testWatch(tests, "pods", t)
}
