package restclient

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
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	ewatch "github.com/ingvagabund/cluster-capacity/pkg/client/emulator/watch"
	ccapi "github.com/ingvagabund/cluster-capacity/pkg/api"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
)

// RESTClient provides a fake RESTClient interface.
type RESTClient struct {
	NegotiatedSerializer runtime.NegotiatedSerializer

	Req  *http.Request
	Resp *http.Response
	Err  error

	resourceStore store.ResourceStore

	podsWatcherReadGetter *ewatch.WatchBuffer
	servicesWatcherReadGetter *ewatch.WatchBuffer
	rcsWatcherReadGetter *ewatch.WatchBuffer
	pvWatcherReadGetter *ewatch.WatchBuffer
	pvcWatcherReadGetter *ewatch.WatchBuffer
	nodesWatcherReadGetter *ewatch.WatchBuffer
}

func (c *RESTClient) Pods() *api.PodList {
	items := c.resourceStore.List(ccapi.Pods)
	podItems := make([]api.Pod, 0, len(items))
	for _, item := range items {
		podItems = append(podItems, item.(api.Pod))
	}

	return &api.PodList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: podItems,
	}
}

func (c *RESTClient) Services() *api.ServiceList {
	items := c.resourceStore.List(ccapi.Services)
	serviceItems := make([]api.Service, 0, len(items))
	for _, item := range items {
		serviceItems = append(serviceItems, item.(api.Service))
	}

	return &api.ServiceList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: serviceItems,
	}
}

func (c *RESTClient) ReplicationControllers() *api.ReplicationControllerList {
	items := c.resourceStore.List(ccapi.ReplicationControllers)
	rcItems := make([]api.ReplicationController, 0, len(items))
	for _, item := range items {
		rcItems = append(rcItems, item.(api.ReplicationController))
	}

	return &api.ReplicationControllerList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: rcItems,
	}
}

func (c *RESTClient) PersistentVolumes() *api.PersistentVolumeList {
	items := c.resourceStore.List(ccapi.PersistentVolumes)
	pvItems := make([]api.PersistentVolume, 0, len(items))
	for _, item := range items {
		pvItems = append(pvItems, item.(api.PersistentVolume))
	}

	return &api.PersistentVolumeList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: pvItems,
	}
}

func (c *RESTClient) PersistentVolumeClaims() *api.PersistentVolumeClaimList {
	items := c.resourceStore.List(ccapi.PersistentVolumeClaims)
	pvcItems := make([]api.PersistentVolumeClaim, 0, len(items))
	for _, item := range items {
		pvcItems = append(pvcItems, item.(api.PersistentVolumeClaim))
	}

	return &api.PersistentVolumeClaimList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: pvcItems,
	}
}

func (c *RESTClient) Nodes() *api.NodeList {
	items := c.resourceStore.List(ccapi.Nodes)
	nodeItems := make([]api.Node, 0, len(items))
	for _, item := range items {
		nodeItems = append(nodeItems, item.(api.Node))
	}

	return &api.NodeList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion:  "0",
		},
		Items: nodeItems,
	}
}

func (c *RESTClient) ReplicaSets() *extensions.ReplicaSetList {
	return nil
}


func (c *RESTClient) EmitPodWatchEvent(eType watch.EventType, object *api.Pod) error {
	if c.podsWatcherReadGetter != nil {
		return c.podsWatcherReadGetter.EmitWatchEvent(eType, object)
	}
	return fmt.Errorf("Watch buffer for pods not initialized")
}

func (c *RESTClient) EmitServiceWatchEvent(eType watch.EventType, object *api.Service) error {
	if c.servicesWatcherReadGetter != nil {
		return c.servicesWatcherReadGetter.EmitWatchEvent(eType, object)
	}
	return fmt.Errorf("Watch buffer for services not initialized")
}

func (c *RESTClient) EmitReplicationControllerWatchEvent(eType watch.EventType, object *api.ReplicationController) error {
	if c.rcsWatcherReadGetter != nil {
		return c.rcsWatcherReadGetter.EmitWatchEvent(eType, object)
	}
	return fmt.Errorf("Watch buffer for replication controllers not initialized")
}

func (c *RESTClient) EmitPersistentVolumeWatchEvent(eType watch.EventType, object *api.PersistentVolume) error {
	if c.pvWatcherReadGetter != nil {
		return c.pvWatcherReadGetter.EmitWatchEvent(eType, object)
	}
	return fmt.Errorf("Watch buffer for persistent volumes not initialized")
}

func (c *RESTClient) EmitPersistentVolumeClaimWatchEvent(eType watch.EventType, object *api.PersistentVolumeClaim) error {
	if c.pvcWatcherReadGetter != nil {
		return c.pvcWatcherReadGetter.EmitWatchEvent(eType, object)
	}
	return fmt.Errorf("Watch buffer for persistent volume claims not initialized")
}

func (c *RESTClient) EmitNodeWatchEvent(eType watch.EventType, object *api.Node) error {
	if c.nodesWatcherReadGetter != nil {
		return c.nodesWatcherReadGetter.EmitWatchEvent(eType, object)
	}
	return fmt.Errorf("Watch buffer for nodes not initialized")
}

func (c *RESTClient) Close() {
	if c.podsWatcherReadGetter != nil {
		c.podsWatcherReadGetter.Close()
	}
	if c.servicesWatcherReadGetter != nil {
		c.servicesWatcherReadGetter.Close()
	}
	if c.rcsWatcherReadGetter != nil {
		c.rcsWatcherReadGetter.Close()
	}
	if c.pvWatcherReadGetter != nil {
		c.pvWatcherReadGetter.Close()
	}
	if c.nodesWatcherReadGetter != nil {
		c.nodesWatcherReadGetter.Close()
	}
	if c.pvcWatcherReadGetter != nil {
		c.pvcWatcherReadGetter.Close()
	}
}

func (c *RESTClient) GetRateLimiter() flowcontrol.RateLimiter {
	return nil
}

func (c *RESTClient) Verb(verb string) *restclient.Request {
	return c.request(verb)
}

func (c *RESTClient) APIVersion() unversioned.GroupVersion {
	return *(testapi.Default.GroupVersion())
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
		case ccapi.Pods:
			obj = c.Pods()
		case ccapi.Services:
			obj = c.Services()
		case ccapi.ReplicationControllers:
			obj = c.ReplicationControllers()
		case ccapi.PersistentVolumes:
			obj = c.PersistentVolumes()
		case ccapi.PersistentVolumeClaims:
			obj = c.PersistentVolumeClaims()
		case ccapi.Nodes:
			obj = c.Nodes()
		default:
			return nil, fmt.Errorf("Resource %s not recognized", resource)
	}

	nopCloser := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
	return &nopCloser, nil

}

func (c *RESTClient) createWatchReadCloser(resource string) (rc *ewatch.WatchBuffer, err error) {
	rc = ewatch.NewWatchBuffer()
	switch resource {
		case ccapi.Pods:
			if c.podsWatcherReadGetter != nil {
				c.podsWatcherReadGetter.Close()
			}
			c.podsWatcherReadGetter = rc
		case ccapi.Services:
			if c.servicesWatcherReadGetter != nil {
				c.servicesWatcherReadGetter.Close()
			}
			c.servicesWatcherReadGetter = rc
		case ccapi.ReplicationControllers:
			if c.rcsWatcherReadGetter != nil {
				c.rcsWatcherReadGetter.Close()
			}
			c.rcsWatcherReadGetter = rc
		case ccapi.PersistentVolumes:
			if c.pvWatcherReadGetter != nil {
				c.pvWatcherReadGetter.Close()
			}
			c.pvWatcherReadGetter = rc
		case ccapi.PersistentVolumeClaims:
			if c.pvcWatcherReadGetter != nil {
				c.pvcWatcherReadGetter.Close()
			}
			c.pvcWatcherReadGetter = rc
		case ccapi.Nodes:
			if c.nodesWatcherReadGetter != nil {
				c.nodesWatcherReadGetter.Close()
			}
			c.nodesWatcherReadGetter = rc
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
			return nil, fmt.Errorf("Unable to create lister for %s\n", parts[0])
		}
		c.Resp = &http.Response{StatusCode: 200, Header: header, Body: *body}
	}

	return c.Resp, nil
}

func NewRESTClient(resourceStore store.ResourceStore) *RESTClient {
	return &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
		resourceStore: resourceStore,
	}
}
