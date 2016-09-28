package emulator

import (
	"testing"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
	"reflect"
	"strings"
)


func testPodsData() *api.PodList {
	pods := &api.PodList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "15",
		},
		Items: []api.Pod{
			{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "test", ResourceVersion: "10"},
				Spec:       apitesting.DeepEqualSafePodSpec(),
			},
			{
				ObjectMeta: api.ObjectMeta{Name: "bar", Namespace: "test", ResourceVersion: "11"},
				Spec:       apitesting.DeepEqualSafePodSpec(),
			},
		},
	}

	return pods
}

func testServicesData() *api.ServiceList {
	svc := &api.ServiceList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "16",
		},
		Items: []api.Service{
			{
				ObjectMeta: api.ObjectMeta{Name: "baz", Namespace: "test", ResourceVersion: "12"},
				Spec: api.ServiceSpec{
					SessionAffinity: "None",
					Type:            api.ServiceTypeClusterIP,
				},
			},
		},
	}
	return svc
}

func testReplicationControllersData() *api.ReplicationControllerList {
	rc := &api.ReplicationControllerList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "17",
		},
		Items: []api.ReplicationController{
			{
				ObjectMeta: api.ObjectMeta{Name: "rc1", Namespace: "test", ResourceVersion: "18"},
				Spec: api.ReplicationControllerSpec{
					Replicas: 1,
				},
			},
		},
	}
	return rc
}

func NewTestRestClient() *RESTClient {

	client := &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
		podsDataSource: testPodsData,
		servicesDataSource: testServicesData,
		replicationControllersDataSource: testReplicationControllersData,
	}

	return client
}

// splitPath returns the segments for a URL path.
func splitPath(path string) []string {
        path = strings.Trim(path, "/")
        if path == "" {
                return []string{}
        }
        return strings.Split(path, "/")
}

func compareItems(expected, actual interface{}) bool {
	if reflect.TypeOf(expected).Kind() != reflect.Slice {
		return false
	}

	if reflect.TypeOf(actual).Kind() != reflect.Slice {
		return false
	}

	expectedSlice := reflect.ValueOf(expected)
	expectedMap := make(map[string]interface{})
	for i := 0; i < expectedSlice.Len(); i++ {
		meta := expectedSlice.Index(i).FieldByName("ObjectMeta").Interface().(api.ObjectMeta)
		key := strings.Join([]string{meta.Namespace, meta.Name, meta.ResourceVersion}, "/")
		expectedMap[key] = expectedSlice.Index(i).Interface()
	}

	actualMap := make(map[string]interface{})
	actualSlice := reflect.ValueOf(actual)
	for i := 0; i < actualSlice.Len(); i++ {
		meta := actualSlice.Index(i).FieldByName("ObjectMeta").Interface().(api.ObjectMeta)
		key := strings.Join([]string{meta.Namespace, meta.Name, meta.ResourceVersion}, "/")
		actualMap[key] = actualSlice.Index(i).Interface()
	}

	return reflect.DeepEqual(expectedMap, actualMap)
}

func TestSyncPods(t *testing.T) {

	fakeClient := NewTestRestClient()
	expected := fakeClient.Pods().Items
	emulator := NewClientEmulator()

	err := emulator.sync(fakeClient)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	storedItems := emulator.PodCache.List()
	actual := make([]api.Pod, 0, len(storedItems))
	for _, value := range storedItems {
		item, ok := value.(*api.Pod)
		if !ok {
			t.Errorf("Expected api.Pod type, found different")
		}
		actual = append(actual, *item)
	}
	if !compareItems(expected, actual) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, actual)
	}

}

func TestSyncServices(t *testing.T) {

	fakeClient := NewTestRestClient()
	expected := fakeClient.Services().Items
	emulator := NewClientEmulator()

	err := emulator.sync(fakeClient)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	storedItems := emulator.ServiceCache.List()
	actual := make([]api.Service, 0, len(storedItems))
	for _, value := range storedItems {
		item, ok := value.(*api.Service)
		if !ok {
			t.Errorf("Expected api.Service type, found different")
		}
		actual = append(actual, *item)
	}

	if !compareItems(expected, actual) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, actual)
	}
}

func TestSyncReplicationControllers(t *testing.T) {

	fakeClient := NewTestRestClient()
	expected := fakeClient.ReplicationControllers().Items
	emulator := NewClientEmulator()

	err := emulator.sync(fakeClient)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	storedItems := emulator.ReplicationControllerCache.List()
	actual := make([]api.ReplicationController, 0, len(storedItems))
	for _, value := range storedItems {
		item, ok := value.(*api.ReplicationController)
		if !ok {
			t.Errorf("Expected api.Service type, found different")
		}
		actual = append(actual, *item)
	}

	if !compareItems(expected, actual) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, actual)
	}
}
