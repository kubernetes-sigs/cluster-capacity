package store

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"reflect"
)

type FakeResourceStore struct {
	PodsData func() []api.Pod
	ServicesData func() []api.Service
	ReplicationControllersData func() []api.ReplicationController
	// TODO(jchaloup): fill missing resource functions
}

func (s *FakeResourceStore) Add(resource string, obj interface{}) error {
	return nil
}

func (s *FakeResourceStore) Update(resource string, obj interface{}) error {
	return nil
}

func (s *FakeResourceStore) Delete(resource string, obj interface{}) error {
	return nil
}

func resourcesToItems(objs interface{}) []interface{} {
	objsSlice := reflect.ValueOf(objs)
	items := make([]interface{}, 0, objsSlice.Len())
	for i := 0; i < objsSlice.Len(); i++ {
		items = append(items, objsSlice.Index(i).Interface())
	}
	return items
}

func (s *FakeResourceStore) List(resource string) []interface{} {
	switch resource {
		case "pods":
			return resourcesToItems(s.PodsData())
		case "services":
			return resourcesToItems(s.ServicesData())
		case "replicationControllers":
			return resourcesToItems(s.ReplicationControllersData())
		//case "replicasets":
		//	return testReplicaSetsData().Items
	}
	return nil
}

func (s *FakeResourceStore) Get(resource string, obj interface{}) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (s *FakeResourceStore) GetByKey(key string) (item interface{}, exists bool, err error) {
	return nil, false, nil
}

func (s *FakeResourceStore) RegisterEventHandler(resource string, handler cache.ResourceEventHandler) error {
	return nil
}

func (s *FakeResourceStore) Replace(resource string, items []interface{}, resourceVersion string) error {
	return nil
}

func (s *FakeResourceStore) Resources() []string {
	return []string{"pods", "services", "replicationControllers"}
}

