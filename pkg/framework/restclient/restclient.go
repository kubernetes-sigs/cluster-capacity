/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	"sync"

	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	"k8s.io/kubernetes/pkg/watch"

	ccapi "github.com/kubernetes-incubator/cluster-capacity/pkg/api"
	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
	ewatch "github.com/kubernetes-incubator/cluster-capacity/pkg/framework/watch"
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
	field = strings.Replace(field, "type", "Type", -1)
	fieldPath := fmt.Sprintf("{{.%v}}", field)
	t := template.Must(template.New("fieldPath").Parse(fieldPath))
	err := t.Execute(o, o.obj)
	if err != nil {
		fmt.Printf("Error when accessing object field %v: %v\n", fieldPath, err)
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

	// resource:selector
	watcherReadGetters    map[ccapi.ResourceType]map[string][]*ewatch.WatchBuffer
	watcherReadGettersMux sync.RWMutex
	// name the rest client
	name string
}

func (c *RESTClient) Pods(fieldsSelector fields.Selector) *api.PodList {
	items := c.resourceStore.List(ccapi.Pods)
	typedItems := make([](api.Pod), 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.Pod))
	}

	return &api.PodList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) Services(fieldsSelector fields.Selector) *api.ServiceList {
	items := c.resourceStore.List(ccapi.Services)
	typedItems := make([]api.Service, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.Service))
	}

	return &api.ServiceList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) ReplicationControllers(fieldsSelector fields.Selector) *api.ReplicationControllerList {
	items := c.resourceStore.List(ccapi.ReplicationControllers)
	typedItems := make([]api.ReplicationController, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.ReplicationController))
	}

	return &api.ReplicationControllerList{
		ListMeta: unversioned.ListMeta{
			// choose arbitrary value as the cache does not store the ResourceVersion
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) PersistentVolumes(fieldsSelector fields.Selector) *api.PersistentVolumeList {
	items := c.resourceStore.List(ccapi.PersistentVolumes)
	typedItems := make([]api.PersistentVolume, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.PersistentVolume))
	}

	return &api.PersistentVolumeList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) PersistentVolumeClaims(fieldsSelector fields.Selector) *api.PersistentVolumeClaimList {
	items := c.resourceStore.List(ccapi.PersistentVolumeClaims)
	typedItems := make([]api.PersistentVolumeClaim, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.PersistentVolumeClaim))
	}

	return &api.PersistentVolumeClaimList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) Nodes(fieldsSelector fields.Selector) *api.NodeList {
	items := c.resourceStore.List(ccapi.Nodes)
	typedItems := make([]api.Node, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.Node))
	}

	return &api.NodeList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) ReplicaSets(fieldsSelector fields.Selector) *extensions.ReplicaSetList {
	items := c.resourceStore.List(ccapi.ReplicaSets)
	typedItems := make([]extensions.ReplicaSet, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*extensions.ReplicaSet))
	}

	return &extensions.ReplicaSetList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) ResourceQuota(fieldsSelector fields.Selector) *api.ResourceQuotaList {
	items := c.resourceStore.List(ccapi.ResourceQuota)
	typedItems := make([]api.ResourceQuota, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.ResourceQuota))
	}

	return &api.ResourceQuotaList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) Secrets(fieldsSelector fields.Selector) *api.SecretList {
	items := c.resourceStore.List(ccapi.Secrets)
	typedItems := make([]api.Secret, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.Secret))
	}

	return &api.SecretList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) ServiceAccounts(fieldsSelector fields.Selector) *api.ServiceAccountList {
	items := c.resourceStore.List(ccapi.ServiceAccounts)
	typedItems := make([]api.ServiceAccount, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.ServiceAccount))
	}

	return &api.ServiceAccountList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) LimitRanges(fieldsSelector fields.Selector) *api.LimitRangeList {
	items := c.resourceStore.List(ccapi.LimitRanges)
	typedItems := make([]api.LimitRange, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.LimitRange))
	}

	return &api.LimitRangeList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) Namespaces(fieldsSelector fields.Selector) *api.NamespaceList {
	items := c.resourceStore.List(ccapi.Namespaces)
	typedItems := make([]api.Namespace, 0, len(items))
	for _, item := range items {
		if !fieldsSelector.Matches(NewObjectFieldsAccessor(item)) {
			continue
		}
		typedItems = append(typedItems, *item.(*api.Namespace))
	}

	return &api.NamespaceList{
		ListMeta: unversioned.ListMeta{
			ResourceVersion: "0",
		},
		Items: typedItems,
	}
}

func (c *RESTClient) List(resource ccapi.ResourceType, fieldsSelector fields.Selector) (runtime.Object, error) {
	switch resource {
	case ccapi.Pods:
		return c.Pods(fieldsSelector), nil
	case ccapi.Services:
		return c.Services(fieldsSelector), nil
	case ccapi.ReplicationControllers:
		return c.ReplicationControllers(fieldsSelector), nil
	case ccapi.PersistentVolumes:
		return c.PersistentVolumes(fieldsSelector), nil
	case ccapi.PersistentVolumeClaims:
		return c.PersistentVolumeClaims(fieldsSelector), nil
	case ccapi.Nodes:
		return c.Nodes(fieldsSelector), nil
	case ccapi.ReplicaSets:
		return c.ReplicaSets(fieldsSelector), nil
	case ccapi.ResourceQuota:
		return c.ResourceQuota(fieldsSelector), nil
	case ccapi.Secrets:
		return c.Secrets(fieldsSelector), nil
	case ccapi.ServiceAccounts:
		return c.ServiceAccounts(fieldsSelector), nil
	case ccapi.LimitRanges:
		return c.LimitRanges(fieldsSelector), nil
	case ccapi.Namespaces:
		return c.Namespaces(fieldsSelector), nil
	default:
		return nil, fmt.Errorf("Resource %s not recognized", resource)
	}
}

func (c *RESTClient) EmitObjectWatchEvent(resource ccapi.ResourceType, eType watch.EventType, object runtime.Object) error {
	rg, exists := c.watcherReadGetters[resource]
	if !exists {
		return fmt.Errorf("Watch buffer for pods not initialized")
	}

	for fieldsSelector, watchers := range rg {
		if !fields.ParseSelectorOrDie(fieldsSelector).Matches(NewObjectFieldsAccessor(object)) {
			continue
		}
		for _, w := range watchers {
			err := w.EmitWatchEvent(eType, object)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *RESTClient) Close() {
	for _, rg := range c.watcherReadGetters {
		for _, watchers := range rg {
			for _, w := range watchers {
				w.Close()
			}
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
	info, _ := runtime.SerializerInfoForMediaType(ns.SupportedMediaTypes(), runtime.ContentTypeJSON)

	var targetVersion unversioned.GroupVersion
	if c.name == "extensions" {
		gvr := unversioned.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "replicasets"}
		targetVersion = gvr.GroupVersion()
	} else {
		targetVersion = unversioned.GroupVersion{
			Group:   testapi.Default.GroupVersion().Group,
			Version: runtime.APIVersionInternal,
		}
	}

	serializers := restclient.Serializers{
		Encoder: ns.EncoderForVersion(info.Serializer, *testapi.Default.GroupVersion()),
		Decoder: ns.DecoderToVersion(info.Serializer, targetVersion),
	}

	if info.StreamSerializer != nil {
		serializers.StreamingSerializer = info.StreamSerializer.Serializer
		serializers.Framer = info.StreamSerializer.Framer
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

func (c *RESTClient) createListReadCloser(resource ccapi.ResourceType, fieldsSelector fields.Selector) (rc *io.ReadCloser, err error) {
	obj, err := c.List(resource, fieldsSelector)
	if err != nil {
		return nil, err
	}

	if resource == ccapi.ReplicaSets {
		gvr := unversioned.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "replicasets"}
		info, ok := runtime.SerializerInfoForMediaType(c.NegotiatedSerializer.SupportedMediaTypes(), runtime.ContentTypeJSON)
		if !ok {
			return nil, fmt.Errorf("serializer for %s not registered", runtime.ContentTypeJSON)
		}

		encoder := api.Codecs.EncoderForVersion(info.Serializer, gvr.GroupVersion())
		nopCloser := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(encoder, obj.(*extensions.ReplicaSetList)))))
		return &nopCloser, nil
	} else {
		nopCloser := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
		return &nopCloser, nil
	}
}

func (c *RESTClient) createGetReadCloser(resource ccapi.ResourceType, resourceName string, namespace string) (rc *io.ReadCloser, err error) {
	key := &api.ObjectMeta{Name: resourceName, Namespace: namespace}
	item, exists, err := c.resourceStore.Get(resource, key)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve requested %v resource %v: %v", resource, resourceName, err)
	}
	if !exists {
		return nil, fmt.Errorf("Requested %v resource %v not found", resource, resourceName)
	}

	var obj runtime.Object
	var ns string

	switch resource {
	case ccapi.Pods:
		obj = runtime.Object(item.(*api.Pod))
		ns = item.(*api.Pod).Namespace
	case ccapi.Services:
		obj = runtime.Object(item.(*api.Service))
		ns = item.(*api.Service).Namespace
	case ccapi.ReplicationControllers:
		obj = runtime.Object(item.(*api.ReplicationController))
		ns = item.(*api.ReplicationController).Namespace
	case ccapi.PersistentVolumes:
		obj = runtime.Object(item.(*api.PersistentVolume))
		ns = item.(*api.PersistentVolume).Namespace
	case ccapi.PersistentVolumeClaims:
		obj = runtime.Object(item.(*api.PersistentVolumeClaim))
		ns = item.(*api.PersistentVolumeClaim).Namespace
	case ccapi.Nodes:
		obj = runtime.Object(item.(*api.Node))
	case ccapi.ReplicaSets:
		obj = runtime.Object(item.(*extensions.ReplicaSet))
		ns = item.(*extensions.ReplicaSet).Namespace
	case ccapi.ResourceQuota:
		obj = runtime.Object(item.(*api.ResourceQuota))
		ns = item.(*api.ResourceQuota).Namespace
	case ccapi.Secrets:
		obj = runtime.Object(item.(*api.Secret))
		ns = item.(*api.Secret).Namespace
	case ccapi.ServiceAccounts:
		obj = runtime.Object(item.(*api.ServiceAccount))
		ns = item.(*api.ServiceAccount).Namespace
	case ccapi.LimitRanges:
		obj = runtime.Object(item.(*api.LimitRange))
		ns = item.(*api.LimitRange).Namespace
	case ccapi.Namespaces:
		obj = runtime.Object(item.(*api.Namespace))
	default:
		return nil, fmt.Errorf("Resource %v not recognized", resource)
	}

	if namespace != "" {
		if ns != namespace {
			return nil, fmt.Errorf("Requested %v resource %v not found. Namespace does not match", resource, resourceName)
		}
	}

	nopCloser := ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(testapi.Default.Codec(), obj))))
	return &nopCloser, nil
}

func (c *RESTClient) createWatchReadCloser(resource ccapi.ResourceType, fieldsSelector fields.Selector) (rc *ewatch.WatchBuffer, err error) {
	c.watcherReadGettersMux.Lock()
	defer c.watcherReadGettersMux.Unlock()

	resourceWatcherReadGetter, ok := c.watcherReadGetters[resource]
	if !ok {
		return nil, fmt.Errorf("Resource %s not recognized", resource)
	}

	// multi-schedulers environment may require multiple instances of a watcher
	// for the same resource and fields selector.
	watchers, exists := resourceWatcherReadGetter[fieldsSelector.String()]
	if !exists {
		watchers = make([]*ewatch.WatchBuffer, 0)
	}

	rg := ewatch.NewWatchBuffer(resource)
	c.watcherReadGetters[resource][fieldsSelector.String()] = append(watchers, rg)

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
	case ccapi.ReplicaSets:
		for _, item := range c.ReplicaSets(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.ResourceQuota:
		for _, item := range c.ResourceQuota(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.Secrets:
		for _, item := range c.Secrets(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.ServiceAccounts:
		for _, item := range c.ServiceAccounts(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.LimitRanges:
		for _, item := range c.LimitRanges(fieldsSelector).Items {
			rg.EmitWatchEvent(watch.Added, runtime.Object(&item))
		}
	case ccapi.Namespaces:
		for _, item := range c.Namespaces(fieldsSelector).Items {
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
	//fmt.Printf("URL request path: %v, rawQuery: %v, fields selector: %v\n", req.URL.Path, queryParams, fieldsSelector)
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
		resource, err := ccapi.StringToResourceType(parts[1])
		if err != nil {
			return nil, fmt.Errorf("Unable to process request: %v", err)
		}
		body, err := c.createWatchReadCloser(resource, fieldsSelector)
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
		switch len(parts) {
		case 1:
			resource, err := ccapi.StringToResourceType(parts[0])
			if err != nil {
				return nil, fmt.Errorf("Unable to process request: %v", err)
			}
			body, err = c.createListReadCloser(resource, fieldsSelector)
			if err != nil {
				return nil, fmt.Errorf("Unable to create lister for %s\n", parts[0])
			}
		case 2:
			resource, err := ccapi.StringToResourceType(parts[0])
			if err != nil {
				return nil, fmt.Errorf("Unable to process request: %v", err)
			}
			body, err = c.createGetReadCloser(resource, parts[1], "")
			if err != nil {
				return nil, fmt.Errorf("Unable to create getter for %s: %v\n", parts[0], err)
			}
		case 3:
			if parts[0] != "namespaces" {
				return nil, fmt.Errorf("Unable to decode query url: %v. Expected namespaces, got %v", req.URL.Path, parts[0])
			}
			resource, err := ccapi.StringToResourceType(parts[2])
			if err != nil {
				return nil, fmt.Errorf("Unable to process request: %v", err)
			}
			body, err = c.createListReadCloser(resource, fields.ParseSelectorOrDie(fmt.Sprintf("Namespace=%v", parts[1])))
			if err != nil {
				return nil, fmt.Errorf("Unable to create lister for %s\n", parts[0])
			}
		case 4, 5:
			if len(parts) == 5 {
				if !strings.EqualFold(parts[4], "status") {
					return nil, fmt.Errorf("Cluster capacity RESTClient not implemented: query url does not end with status: %v", req.URL.Path)
				}

				if parts[2] != "resourcequotas" {
					return nil, fmt.Errorf("Cluster capacity RESTClient not implemented: status can be queried only for resourcequotas: %v", req.URL.Path)
				}

				// decode and update resource quota in the local cache
				var buffer bytes.Buffer
				buff := make([]byte, 100, 100)
				for {
					n, err := req.Body.Read(buff)
					buffer.WriteString(string(buff[:n]))
					if err != nil {
						break
					}
				}

				obj := &api.ResourceQuota{}
				runtime.DecodeInto(testapi.Default.Codec(), buffer.Bytes(), runtime.Object(obj))
				c.resourceStore.Add(ccapi.ResourceQuota, obj)
			}

			if parts[0] != "namespaces" {
				return nil, fmt.Errorf("Unable to decode query url: %v. Expected namespaces, got %v", req.URL.Path, parts[0])
			}
			resource, err := ccapi.StringToResourceType(parts[2])
			if err != nil {
				return nil, fmt.Errorf("Unable to process request: %v", err)
			}
			body, err = c.createGetReadCloser(resource, parts[3], parts[1])
			if err != nil {
				return nil, fmt.Errorf("Unable to create getter for %s: %v\n", parts[0], err)
			}
		default:
			return nil, fmt.Errorf("Cluster capacity RESTClient not implemented: unable to decode query url: %v", req.URL.Path)
		}
		c.Resp = &http.Response{StatusCode: 200, Header: header, Body: *body}
	}

	return c.Resp, nil
}

func NewRESTClient(resourceStore store.ResourceStore, name string) *RESTClient {
	client := &RESTClient{
		NegotiatedSerializer: testapi.Default.NegotiatedSerializer(),
		resourceStore:        resourceStore,
		watcherReadGetters:   make(map[ccapi.ResourceType]map[string][]*ewatch.WatchBuffer),
		name:                 name,
	}

	for _, resource := range resourceStore.Resources() {
		client.watcherReadGetters[resource] = make(map[string][]*ewatch.WatchBuffer)
	}

	return client
}
