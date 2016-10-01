package emulator

import (
	"testing"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
	"reflect"
	"strings"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/runtime"
	"fmt"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/api/meta"
)


func testPodsData() *api.PodList {
	pods := &api.PodList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "15",
		},
	}
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("pod%v", i)
		item := api.Pod{
			ObjectMeta: api.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "10"},
			Spec:       apitesting.DeepEqualSafePodSpec(),
		}

		pods.Items = append(pods.Items, item)
	}

	return pods
}

func testServicesData() *api.ServiceList {
	svc := &api.ServiceList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "16",
		},
	}

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("service%v", i)
		item := api.Service{
			ObjectMeta: api.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "12"},
			Spec: api.ServiceSpec{
				SessionAffinity: "None",
				Type:            api.ServiceTypeClusterIP,
			},
		}

		svc.Items = append(svc.Items, item)
	}

	return svc
}

func testReplicationControllersData() *api.ReplicationControllerList {
	rc := &api.ReplicationControllerList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "17",
		},
	}

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("rc%v", i)
		item := api.ReplicationController{
			ObjectMeta: api.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "18"},
			Spec: api.ReplicationControllerSpec{
				Replicas: 1,
			},
		}
		rc.Items = append(rc.Items, item)
	}

	return rc
}

func testReplicaSetsData() *extensions.ReplicaSetList {
	rs := &extensions.ReplicaSetList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "10",
		},
	}

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("replicaset%v", i)
		item := extensions.ReplicaSet{
			ObjectMeta: api.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "125"},
			Spec: extensions.ReplicaSetSpec{
				Replicas: 3,
			},
		}
		rs.Items = append(rs.Items, item)
	}

	return rs
}

func newTestListRestClient() *RESTClient {

	resourceStore := &store.FakeResourceStore{
		PodsData: func() []api.Pod {
			return testPodsData().Items
		},
		ServicesData: func() []api.Service {
			return testServicesData().Items
		},
		ReplicationControllersData: func() []api.ReplicationController {
			return testReplicationControllersData().Items
		},
	}

	client := &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
		resourceStore: resourceStore,
	}

	return client
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

func getResourceList(client cache.Getter, resource string) runtime.Object {
	// client listerWatcher
	listerWatcher := cache.NewListWatchFromClient(client, resource, api.NamespaceAll, fields.ParseSelectorOrDie(""))
	options := api.ListOptions{ResourceVersion: "0"}
	l, _ := listerWatcher.List(options)
	return l
}

func TestSyncPods(t *testing.T) {

	fakeClient := newTestListRestClient()
	expected := fakeClient.Pods().Items

	list := getResourceList(fakeClient, "pods")
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}

	found := make([]api.Pod, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*api.Pod)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncServices(t *testing.T) {

	fakeClient := newTestListRestClient()
	expected := fakeClient.Services().Items

	list := getResourceList(fakeClient, "services")
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}

	found := make([]api.Service, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*api.Service)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncReplicationControllers(t *testing.T) {

	fakeClient := newTestListRestClient()
	expected := fakeClient.ReplicationControllers().Items

	list := getResourceList(fakeClient, "replicationControllers")
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}

	found := make([]api.ReplicationController, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*api.ReplicationController)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

//func testSyncReplicaSets(t *testing.T) {
//	fakeClient := newTestListRestClient()
//	expected := fakeClient.ReplicaSets().Items
//	emulator := NewClientEmulator()
//
//	err := emulator.sync(fakeClient)
//
//	if err != nil {
//		t.Fatalf("Unexpected error: %v", err)
//	}
//
//	storedItems := emulator.ReplicaSetCache.List()
//	actual := make([]extensions.ReplicaSet, 0, len(storedItems))
//	for _, value := range storedItems {
//		item, ok := value.(*extensions.ReplicaSet)
//		if !ok {
//			t.Errorf("Expected api.Service type, found different")
//		}
//		actual = append(actual, *item)
//	}
//
//	if !compareItems(expected, actual) {
//		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, actual)
//	}
//}
