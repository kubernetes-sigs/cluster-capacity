package emulator

import (
	"net/http"
	"k8s.io/kubernetes/pkg/runtime"
	"bytes"
	"io/ioutil"
	"k8s.io/kubernetes/pkg/client/restclient"
	"fmt"
	"net/url"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/testapi"
)

// RESTClient provides a fake RESTClient interface.
type RESTClient struct {
	NegotiatedSerializer runtime.NegotiatedSerializer

	Req  *http.Request
	Resp *http.Response
	Err  error

	podsDataSource func() *api.PodList
	servicesDataSource func() *api.ServiceList
	replicationControllersDataSource func() *api.ReplicationControllerList
}

func (c *RESTClient) Pods() *api.PodList {
	return c.podsDataSource()
}

func (c *RESTClient) Services() *api.ServiceList {
	return c.servicesDataSource()
}

func (c *RESTClient) ReplicationControllers() *api.ReplicationControllerList {
	return c.replicationControllersDataSource()
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
			obj = c.Pods()
		case "services":
			obj = c.Services()
		case "replicationcontrollers":
			obj = c.ReplicationControllers()
		default:
			return nil, fmt.Errorf("Resource %s not recognized", parts[0])
	}

	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	body := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
	c.Resp = &http.Response{StatusCode: 200, Header: header, Body: body}

	return c.Resp, nil
}
