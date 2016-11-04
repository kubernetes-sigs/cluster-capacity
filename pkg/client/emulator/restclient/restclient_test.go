package restclient

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	ccapi "github.com/ingvagabund/cluster-capacity/pkg/api"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	"github.com/ingvagabund/cluster-capacity/pkg/test"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
)

func testPodsData() []*api.Pod {
	pods := make([]*api.Pod, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("pod%v", i)
		item := test.PodExample(name)
		pods = append(pods, &item)
	}
	return pods
}

func testServicesData() []*api.Service {
	svcs := make([]*api.Service, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("service%v", i)
		item := test.ServiceExample(name)
		svcs = append(svcs, &item)
	}
	return svcs
}

func testReplicationControllersData() []*api.ReplicationController {
	rcs := make([]*api.ReplicationController, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("rc%v", i)
		item := test.ReplicationControllerExample(name)
		rcs = append(rcs, &item)
	}
	return rcs
}

func testPersistentVolumesData() []*api.PersistentVolume {
	pvs := make([]*api.PersistentVolume, 0, 10)
	for i := 0; i < 1; i++ {
		name := fmt.Sprintf("pv%v", i)
		item := test.PersistentVolumeExample(name)
		pvs = append(pvs, &item)
	}
	return pvs
}

func testPersistentVolumeClaimsData() []*api.PersistentVolumeClaim {
	pvcs := make([]*api.PersistentVolumeClaim, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("pvc%v", i)
		item := test.PersistentVolumeClaimExample(name)
		pvcs = append(pvcs, &item)
	}
	return pvcs
}

func testNodesData() []*api.Node {
	nodes := make([]*api.Node, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("node%v", i)
		item := test.NodeExample(name)
		nodes = append(nodes, &item)
	}
	return nodes
}

func testReplicaSetsData() []*extensions.ReplicaSet {
	rss := make([]*extensions.ReplicaSet, 0, 10)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("replicaset%v", i)
		item := extensions.ReplicaSet{
			ObjectMeta: api.ObjectMeta{Name: name, Namespace: "test", ResourceVersion: "125"},
			Spec: extensions.ReplicaSetSpec{
				Replicas: 3,
			},
		}
		rss = append(rss, &item)
	}
	return rss
}

func newTestListRestClient() *RESTClient {

	resourceStore := &store.FakeResourceStore{
		PodsData: func() []*api.Pod {
			return testPodsData()
		},
		ServicesData: func() []*api.Service {
			return testServicesData()
		},
		ReplicationControllersData: func() []*api.ReplicationController {
			return testReplicationControllersData()
		},
		PersistentVolumesData: func() []*api.PersistentVolume {
			return testPersistentVolumesData()
		},
		PersistentVolumeClaimsData: func() []*api.PersistentVolumeClaim {
			return testPersistentVolumeClaimsData()
		},
		NodesData: func() []*api.Node {
			return testNodesData()
		},
	}

	client := &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
		resourceStore:        resourceStore,
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

func getResourceList(client cache.Getter, resource ccapi.ResourceType) runtime.Object {
	// client listerWatcher
	listerWatcher := cache.NewListWatchFromClient(client, resource.String(), api.NamespaceAll, fields.ParseSelectorOrDie(""))
	options := api.ListOptions{ResourceVersion: "0"}
	l, _ := listerWatcher.List(options)
	return l
}

func TestSyncPods(t *testing.T) {

	fakeClient := newTestListRestClient()
	expected := fakeClient.Pods(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.Pods)
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
	expected := fakeClient.Services(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.Services)
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
	expected := fakeClient.ReplicationControllers(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.ReplicationControllers)
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

func TestSyncPersistentVolumes(t *testing.T) {
	fakeClient := newTestListRestClient()
	expected := fakeClient.PersistentVolumes(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.PersistentVolumes)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}
	found := make([]api.PersistentVolume, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*api.PersistentVolume)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncPersistentVolumeClaims(t *testing.T) {
	fakeClient := newTestListRestClient()
	expected := fakeClient.PersistentVolumeClaims(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.PersistentVolumeClaims)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}
	found := make([]api.PersistentVolumeClaim, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*api.PersistentVolumeClaim)))
	}

	if !compareItems(expected, found) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, found)
	}
}

func TestSyncNodes(t *testing.T) {
	fakeClient := newTestListRestClient()
	expected := fakeClient.Nodes(fields.Everything()).Items

	list := getResourceList(fakeClient, ccapi.Nodes)
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Errorf("Unable to understand list result %#v (%v)", list, err)
	}
	found := make([]api.Node, 0, len(items))
	for _, item := range items {
		found = append(found, *((interface{})(item).(*api.Node)))
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
