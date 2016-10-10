package restclient

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	ccapi "github.com/ingvagabund/cluster-capacity/pkg/api"
	"github.com/ingvagabund/cluster-capacity/pkg/client/emulator/store"
	ewatch "github.com/ingvagabund/cluster-capacity/pkg/client/emulator/watch"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	"k8s.io/kubernetes/pkg/watch"
)

type ObjectFieldsAccessor struct {
	obj interface{}
	buf string
}

func NewObjectFieldsAccessor(obj interface{}) *ObjectFieldsAccessor {
	return &ObjectFieldsAccessor{
		obj: obj,
	}
}

func (o *ObjectFieldsAccessor) Has(field string) (exists bool) {
	fieldPath := fmt.Sprintf("{{.%v}}", field)
	t := template.Must(template.New("field").Parse(fieldPath))
	err := t.Execute(o, o.obj)
	return err == nil
}

// Get returns the value for the provided field.
func (o *ObjectFieldsAccessor) Get(field string) (value string) {
	// transform fields .spec.nodeName, .status.phase
	// TODO(jchaloup): very hacky, find a way to actually access fields by its json alias equivalent
	field = strings.Replace(field, "spec", "Spec", -1)
	field = strings.Replace(field, "status", "Status", -1)
	field = strings.Replace(field, "nodeName", "NodeName", -1)
	field = strings.Replace(field, "phase", "Phase", -1)
	fieldPath := fmt.Sprintf("{{.%v}}", field)
	t := template.Must(template.New("fieldPath").Parse(fieldPath))
	err := t.Execute(o, o.obj)
	if err != nil {
		fmt.Printf("Error during accessing object fields: %v\n", err)
	}
	return string(o.buf)
}

func (o *ObjectFieldsAccessor) Write(p []byte) (n int, err error) {
	o.buf = string(p)
	return len(p), nil
}

var _ fields.Fields = &ObjectFieldsAccessor{}
var _ io.Writer = &ObjectFieldsAccessor{}

// RESTClient provides a fake RESTClient interface.
type RESTClient struct {
	NegotiatedSerializer runtime.NegotiatedSerializer

	Req  *http.Request
	Resp *http.Response
	Err  error

	resourceStore store.ResourceStore

	watcherReadGetters map[string]map[string]*ewatch.WatchBuffer
}

func (c *RESTClient) Pods(fieldsSelector fields.Selector) *api.PodList {
	items := c.resourceStore.List(ccapi.Pods)
	podItems := make([](api.Pod), 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item.(*api.Pod))) {
			continue
		}
		podItems = append(podItems, *item.(*api.Pod))
	}

	return &api.PodList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: podItems,
	}
}

func (c *RESTClient) Services(fieldsSelector fields.Selector) *api.ServiceList {
	items := c.resourceStore.List(ccapi.Services)
	serviceItems := make([]api.Service, 0, len(items))
	for _, item := range items {
		serviceItems = append(serviceItems, *item.(*api.Service))
	}

	return &api.ServiceList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: serviceItems,
	}
}

func (c *RESTClient) ReplicationControllers(fieldsSelector fields.Selector) *api.ReplicationControllerList {
	items := c.resourceStore.List(ccapi.ReplicationControllers)
	rcItems := make([]api.ReplicationController, 0, len(items))
	for _, item := range items {
		rcItems = append(rcItems, *item.(*api.ReplicationController))
	}

	return &api.ReplicationControllerList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: rcItems,
	}
}

func (c *RESTClient) PersistentVolumes(fieldsSelector fields.Selector) *api.PersistentVolumeList {
	items := c.resourceStore.List(ccapi.PersistentVolumes)
	pvItems := make([]api.PersistentVolume, 0, len(items))
	for _, item := range items {
		pvItems = append(pvItems, *item.(*api.PersistentVolume))
	}

	return &api.PersistentVolumeList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: pvItems,
	}
}

func (c *RESTClient) PersistentVolumeClaims(fieldsSelector fields.Selector) *api.PersistentVolumeClaimList {
	items := c.resourceStore.List(ccapi.PersistentVolumeClaims)
	pvcItems := make([]api.PersistentVolumeClaim, 0, len(items))
	for _, item := range items {
		pvcItems = append(pvcItems, *item.(*api.PersistentVolumeClaim))
	}

	return &api.PersistentVolumeClaimList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: pvcItems,
	}
}

func (c *RESTClient) Nodes(fieldsSelector fields.Selector) *api.NodeList {
	items := c.resourceStore.List(ccapi.Nodes)
	nodeItems := make([]api.Node, 0, len(items))
	for _, item := range items {
		nodeItems = append(nodeItems, *item.(*api.Node))
	}

	return &api.NodeList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: nodeItems,
	}
}

func (c *RESTClient) ReplicaSets(fieldsSelector fields.Selector) *extensions.ReplicaSetList {
	return nil
}

func (c *RESTClient) EmitObjectWatchEvent(resource string, eType watch.EventType, object runtime.Object) error {
	rg, exists := c.watcherReadGetters[resource]
	if !exists {
		return fmt.Errorf("Watch buffer for pods not initialized")
	}

	for fieldsSelector, w := range rg {
		if !fields.ParseSelectorOrDie(fieldsSelector).Matches(NewObjectFieldsAccessor(object)) {
			continue
		}
		err := w.EmitWatchEvent(eType, object)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *RESTClient) EmitPodWatchEvent(eType watch.EventType, object *api.Pod) error {
	return c.EmitObjectWatchEvent(ccapi.Pods, eType, object)
}

func (c *RESTClient) EmitServiceWatchEvent(eType watch.EventType, object *api.Service) error {
	return c.EmitObjectWatchEvent(ccapi.Services, eType, object)
}

func (c *RESTClient) EmitReplicationControllerWatchEvent(eType watch.EventType, object *api.ReplicationController) error {
	return c.EmitObjectWatchEvent(ccapi.ReplicationControllers, eType, object)
}

func (c *RESTClient) EmitPersistentVolumeWatchEvent(eType watch.EventType, object *api.PersistentVolume) error {
	return c.EmitObjectWatchEvent(ccapi.PersistentVolumes, eType, object)
}

func (c *RESTClient) EmitPersistentVolumeClaimWatchEvent(eType watch.EventType, object *api.PersistentVolumeClaim) error {
	return c.EmitObjectWatchEvent(ccapi.PersistentVolumeClaims, eType, object)
}

func (c *RESTClient) EmitNodeWatchEvent(eType watch.EventType, object *api.Node) error {
	return c.EmitObjectWatchEvent(ccapi.Nodes, eType, object)
}

func (c *RESTClient) Close() {
	for _, rg := range c.watcherReadGetters {
		for _, w := range rg {
			w.Close()
		}
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

func (c *RESTClient) createListReadCloser(resource string, fieldsSelector fields.Selector) (rc *io.ReadCloser, err error) {
	var obj runtime.Object
	switch resource {
	case ccapi.Pods:
		obj = c.Pods(fieldsSelector)
	case ccapi.Services:
		obj = c.Services(fieldsSelector)
	case ccapi.ReplicationControllers:
		obj = c.ReplicationControllers(fieldsSelector)
	case ccapi.PersistentVolumes:
		obj = c.PersistentVolumes(fieldsSelector)
	case ccapi.PersistentVolumeClaims:
		obj = c.PersistentVolumeClaims(fieldsSelector)
	case ccapi.Nodes:
		obj = c.Nodes(fieldsSelector)
	default:
		return nil, fmt.Errorf("Resource %s not recognized", resource)
	}

	nopCloser := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
	return &nopCloser, nil
}

func (c *RESTClient) createGetReadCloser(resource string, resourceName string, namespace string) (rc *io.ReadCloser, err error) {
	key := &api.ObjectMeta{Name: resourceName, Namespace: namespace}
	item, exists, err := c.resourceStore.Get(resource, key)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve requested %v resource %v: %v", resource, resourceName, err)
	}
	if !exists {
		return nil, fmt.Errorf("Requested %v resource %v not found", resource, resourceName)
	}

	var obj runtime.Object
	switch resource {
	case ccapi.Pods:
		if namespace != "" {
			if item.(*api.Pod).Namespace != namespace {
				return nil, fmt.Errorf("Requested %v resource %v not found. Namespace does not match", resource, resourceName)
			}
		}
		obj = runtime.Object(item.(*api.Pod))
	case ccapi.Nodes:
		if namespace != "" {
			if item.(*api.Pod).Namespace != namespace {
				return nil, fmt.Errorf("Requested %v resource %v not found. Namespace does not match", resource, resourceName)
			}
		}
		obj = runtime.Object(item.(*api.Node))
	default:
		return nil, fmt.Errorf("Resource %v not recognized", resource)
	}

	nopCloser := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
	return &nopCloser, nil
}

func (c *RESTClient) createWatchReadCloser(resource string, fieldsSelector fields.Selector) (rc *ewatch.WatchBuffer, err error) {
	resourceWatcherReadGetter, ok := c.watcherReadGetters[resource]
	if !ok {
		return nil, fmt.Errorf("Resource %s not recognized", resource)
	}

	rg, exists := resourceWatcherReadGetter[fieldsSelector.String()]
	if !exists {
		rg = ewatch.NewWatchBuffer()
		c.watcherReadGetters[resource][fieldsSelector.String()] = rg
	}

	// list all objects of the given resource to the wormhole
	switch resource {
	case ccapi.Pods:
		for _, item := range c.Pods(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.Services:
		for _, item := range c.Services(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.ReplicationControllers:
		for _, item := range c.ReplicationControllers(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.PersistentVolumes:
		for _, item := range c.PersistentVolumes(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.PersistentVolumeClaims:
		for _, item := range c.PersistentVolumeClaims(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.Nodes:
		for _, item := range c.Nodes(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	default:
		return nil, fmt.Errorf("Resource %s not recognized", resource)
	}

	return rg, nil
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

	fieldsSelector := fields.Everything()
	queryParams := req.URL.Query()

	// check all fields
	fmt.Printf("URL request path: %v, rawQuery: %v, fields selector: %v\n", req.URL.Path, queryParams, fieldsSelector)
	// is field selector on?
	value, ok := queryParams[unversioned.FieldSelectorQueryParam(testapi.Default.GroupVersion().String())]
	if ok {
		fieldsSelector = fields.ParseSelectorOrDie(value[0])
	}

	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)

	// /watch/pods
	// /services
	// /namespaces/test-node-3/pods/pod-stub,

	if parts[0] == "watch" {
		if len(parts) < 2 {
			return nil, fmt.Errorf("Missing resource in REST client request url")
		}
		body, err := c.createWatchReadCloser(parts[1], fieldsSelector)
		if err != nil {
			return nil, fmt.Errorf("Unable to create watcher for %s\n", parts[1])
		}
		//var t io.ReadCloser = body
		c.Resp = &http.Response{StatusCode: 200, Header: header, Body: (io.ReadCloser)(body)}

	} else {
		// l = len(parts)
		// if l == 1 => list objects of a given resource
		// if l == 2 => list one objects of a given resource
		// if l == 3 => list objects of a given resource from a given namespace
		// if l == 4 => list one object of a given resource from a given namespace
		var body *io.ReadCloser
		var err error
		switch len(parts) {
		case 1:
			body, err = c.createListReadCloser(parts[0], fieldsSelector)
			if err != nil {
				return nil, fmt.Errorf("Unable to create lister for %s\n", parts[0])
			}
		case 2:
			body, err = c.createGetReadCloser(parts[0], parts[1], "")
			if err != nil {
				return nil, fmt.Errorf("Unable to create getter for %s: %v\n", parts[0], err)
			}
		case 3:
			if parts[0] != "namespaces" {
				return nil, fmt.Errorf("Unable to decode query url: %v. Expected namespaces, got %v", req.URL.Path, parts[0])
			}
			body, err = c.createListReadCloser(parts[2], fields.ParseSelectorOrDie(fmt.Sprintf("Namespace=%v", parts[1])))
			if err != nil {
				return nil, fmt.Errorf("Unable to create lister for %s\n", parts[0])
			}
		case 4:
			if parts[0] != "namespaces" {
				return nil, fmt.Errorf("Unable to decode query url: %v. Expected namespaces, got %v", req.URL.Path, parts[0])
			}
			body, err = c.createGetReadCloser(parts[2], parts[3], parts[1])
			if err != nil {
				return nil, fmt.Errorf("Unable to create getter for %s: %v\n", parts[0], err)
			}
		default:
			return nil, fmt.Errorf("Unable to decode query url: %v", req.URL.Path)
		}
		c.Resp = &http.Response{StatusCode: 200, Header: header, Body: *body}
	}

	return c.Resp, nil
}

func NewRESTClient(resourceStore store.ResourceStore) *RESTClient {
	client := &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
		resourceStore:        resourceStore,
		watcherReadGetters:   make(map[string]map[string]*ewatch.WatchBuffer),
	}

	client.watcherReadGetters[ccapi.Pods] = make(map[string]*ewatch.WatchBuffer)
	client.watcherReadGetters[ccapi.Nodes] = make(map[string]*ewatch.WatchBuffer)
	client.watcherReadGetters[ccapi.PersistentVolumes] = make(map[string]*ewatch.WatchBuffer)
	client.watcherReadGetters[ccapi.PersistentVolumeClaims] = make(map[string]*ewatch.WatchBuffer)
	client.watcherReadGetters[ccapi.Services] = make(map[string]*ewatch.WatchBuffer)
	client.watcherReadGetters[ccapi.ReplicationControllers] = make(map[string]*ewatch.WatchBuffer)

	return client
}
