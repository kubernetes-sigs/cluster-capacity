package emulator

import (
	"testing"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/client/unversioned/fake"
	"net/http"
	"k8s.io/kubernetes/pkg/runtime"
	"bytes"
	"io/ioutil"
	"reflect"
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

//func testServiceData() *api.Service {
//	svc := &api.ServiceList{
//		ListMeta: unversioned.ListMeta{
//			ResourceVersion: "16",
//		},
//		Items: []api.Service{
//			{
//				ObjectMeta: api.ObjectMeta{Name: "baz", Namespace: "test", ResourceVersion: "12"},
//				Spec: api.ServiceSpec{
//					SessionAffinity: "None",
//					Type:            api.ServiceTypeClusterIP,
//				},
//			},
//		},
//	}
//	return svc
//}
//
//func testReplicationControllerData() *api.ReplicationController {
//	rc := &api.ReplicationControllerList{
//		ListMeta: unversioned.ListMeta{
//			ResourceVersion: "17",
//		},
//		Items: []api.ReplicationController{
//			{
//				ObjectMeta: api.ObjectMeta{Name: "rc1", Namespace: "test", ResourceVersion: "18"},
//				Spec: api.ReplicationControllerSpec{
//					Replicas: 1,
//				},
//			},
//		},
//	}
//	return rc
//}

func TestSyncPods(t *testing.T) {
	ns := testapi.Default.NegotiatedSerializer()
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)

	body := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), testPodsData()))))

	client := &fake.RESTClient{
		NegotiatedSerializer: ns,
		Resp:                 &http.Response{StatusCode: 200, Header: header, Body: body},
	}

	emulator := NewClientEmulator()
	err := emulator.sync(client)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}


	expected := testPodsData()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actual := emulator.PodCache.List()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("unexpected object: expected: %#v\n actual: %#v", expected, actual)
	}
}