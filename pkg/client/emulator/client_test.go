package emulator

import (
	"testing"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/testapi"
	"net/http"
	"k8s.io/kubernetes/pkg/runtime"
	"bytes"
	"io/ioutil"
	"reflect"
	"k8s.io/kubernetes/pkg/client/restclient"
	"fmt"
	"net/url"
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


// RESTClient provides a fake RESTClient interface.
type RESTClient struct {
	Client               *http.Client
	NegotiatedSerializer runtime.NegotiatedSerializer

	Req  *http.Request
	Resp *http.Response
	Err  error
}

func (c *RESTClient) Get() *restclient.Request {
	return c.request("GET")
}

func (c *RESTClient) Put() *restclient.Request {
	return c.request("PUT")
}

func (c *RESTClient) Patch(_ api.PatchType) *restclient.Request {
	return c.request("PATCH")
}

func (c *RESTClient) Post() *restclient.Request {
	return c.request("POST")
}

func (c *RESTClient) Delete() *restclient.Request {
	return c.request("DELETE")
}

func (c *RESTClient) request(verb string) *restclient.Request {
	config := restclient.ContentConfig{
		ContentType:          runtime.ContentTypeJSON,
		GroupVersion:         testapi.Default.GroupVersion(),
		NegotiatedSerializer: c.NegotiatedSerializer,
	}
	ns := c.NegotiatedSerializer
	serializer, _ := ns.SerializerForMediaType(runtime.ContentTypeJSON, nil)
	streamingSerializer, _ := ns.StreamingSerializerForMediaType(runtime.ContentTypeJSON, nil)
	internalVersion := unversioned.GroupVersion{
		Group:   testapi.Default.GroupVersion().Group,
		Version: runtime.APIVersionInternal,
	}
	internalVersion.Version = runtime.APIVersionInternal
	serializers := restclient.Serializers{
		Encoder:             ns.EncoderForVersion(serializer, *testapi.Default.GroupVersion()),
		Decoder:             ns.DecoderToVersion(serializer, internalVersion),
		StreamingSerializer: streamingSerializer,
		Framer:              streamingSerializer.Framer,
	}
	return restclient.NewRequest(c, verb, &url.URL{Host: "localhost"}, "", config, serializers, nil, nil)
}

func (c *RESTClient) Do(req *http.Request) (*http.Response, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	c.Req = req

	// //localhost/pods?resourceVersion=0
	parts := splitPath(req.URL.Path)
	if len(parts) < 1 {
		return nil, fmt.Errorf("Missing resource in REST client request url")
	}

	var obj runtime.Object

	switch parts[0] {
		case "pods":
			obj = testPodsData()
		case "services":
			obj = testServicesData()
		default:
			return nil, fmt.Errorf("Resource %s not recognized", parts[0])
	}

	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	body := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
	c.Resp = &http.Response{StatusCode: 200, Header: header, Body: body}

	return c.Resp, nil
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
	fakeClient := &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
	}

	data := testPodsData()
	expected := data.Items

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
	fakeClient := &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
	}

	data := testServicesData()
	expected := data.Items

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
