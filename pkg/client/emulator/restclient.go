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
	"strings"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"io"
	"k8s.io/kubernetes/pkg/watch"
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
	replicaSetsDataSource func() *extensions.ReplicaSetList

	podsWatcherReadGetter *WatchBuffer
	servicesWatcherReadGetter *WatchBuffer
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

func (c *RESTClient) ReplicaSets() *extensions.ReplicaSetList {
	return c.replicaSetsDataSource()
}


func (c *RESTClient) EmitPodWatchEvent(eType watch.EventType, object *api.Pod) {
	if c.podsWatcherReadGetter != nil {
		//event := watch.Event{
		//	Type: eType,
		//	Object: object,
		//}
		var buffer bytes.Buffer
		buffer.WriteString("{\"type\":\"")
		buffer.WriteString(string(eType))
		buffer.WriteString("\",\"object\":")

		payload := []byte(buffer.String())
		payload = append(payload, ([]byte)(runtime.EncodeOrDie(testapi.Default.Codec(), object))...)
		payload = append(payload, []byte("}")...)

		c.podsWatcherReadGetter.Write(payload)
	}
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

// splitPath returns the segments for a URL path.
func splitPath(path string) []string {
        path = strings.Trim(path, "/")
        if path == "" {
                return []string{}
        }
        return strings.Split(path, "/")
}

func (c *RESTClient) createListReadCloser(resource string) (rc *io.ReadCloser, err error) {
	var obj runtime.Object
	switch resource {
		case "pods":
			obj = c.Pods()
		case "services":
			obj = c.Services()
		case "replicationcontrollers":
			obj = c.ReplicationControllers()
		default:
			return nil, fmt.Errorf("Resource %s not recognized", resource)
	}

	nopCloser := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
	return &nopCloser, nil

}

func (c *RESTClient) createWatchReadCloser(resource string) (rc *WatchBuffer, err error) {
	rc = NewWatchBuffer()
	switch resource {
		case "pods":
			if c.podsWatcherReadGetter != nil {
				c.podsWatcherReadGetter.Close()
			}
			c.podsWatcherReadGetter = rc
		case "services":
			if c.servicesWatcherReadGetter != nil {
				c.servicesWatcherReadGetter.Close()
			}
			c.servicesWatcherReadGetter = rc
		default:
			return nil, fmt.Errorf("Resource %s not recognized", resource)
	}
	return rc, nil
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

	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)

	if parts[0] == "watch" {
		if len(parts) < 2 {
			return nil, fmt.Errorf("Missing resource in REST client request url")
		}
		body, err := c.createWatchReadCloser(parts[1])
		if err != nil {
			return nil, fmt.Errorf("Unable to create watcher for %s\n", parts[1])
		}
		//var t io.ReadCloser = body
		c.Resp = &http.Response{StatusCode: 200, Header: header, Body: (io.ReadCloser)(body)}

	} else {
		body, err := c.createListReadCloser(parts[0])
		if err != nil {
			return nil, fmt.Errorf("Unable to create lister for %s\n", parts[1])
		}
		c.Resp = &http.Response{StatusCode: 200, Header: header, Body: *body}
	}

	return c.Resp, nil
}
