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

package cache

import (
	"sync"
	"time"

	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework"
)

type Cache struct {
	position int
	reports  []*framework.ClusterCapacityReview
	size     int
	mux      sync.Mutex
}

func NewCache(size int) *Cache {
	return &Cache{
		position: 0,
		reports:  make([]*framework.ClusterCapacityReview, 0),
		size:     size,
	}
}

func (c *Cache) GetSize() int {
	return c.size
}

func (c *Cache) Add(r *framework.ClusterCapacityReview) {
	c.mux.Lock()
	defer c.mux.Unlock()
	if len(c.reports) < c.size {
		c.reports = append(c.reports, r)
	} else {
		c.reports[c.position] = r
	}

	c.position++
	if c.position == c.size {
		c.position = 0
	}
}

func (c *Cache) GetLast(num int) []*framework.ClusterCapacityReview {
	if len(c.reports) == 0 {
		return nil
	}
	sorted := c.All()

	if num >= len(sorted) {
		num = len(sorted)
	}

	return sorted[len(sorted)-num:]
}

func (c *Cache) All() []*framework.ClusterCapacityReview {
	c.mux.Lock()
	defer c.mux.Unlock()
	sorted := make([]*framework.ClusterCapacityReview, 0)

	for i := c.position; i < len(c.reports); i++ {
		sorted = append(sorted, c.reports[i])
	}
	for i := 0; i < c.position; i++ {
		sorted = append(sorted, c.reports[i])
	}
	return sorted
}
func (c *Cache) List(since time.Time, to time.Time, num int) []*framework.ClusterCapacityReview {
	all := c.All()
	if len(all) == 0 {
		return nil
	}

	list := make([]*framework.ClusterCapacityReview, 0)
	for i := 0; i < len(all); i++ {
		if all[i].Status.CreationTimestamp.After(since) && all[i].Status.CreationTimestamp.Before(to) {
			list = append(list, all[i])
		}
	}

	if num >= len(list) || num == -1 {
		num = len(list)
	}

	return list[len(list)-num:]

}
