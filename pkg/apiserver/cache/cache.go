package cache

import (
	"sync"
	"time"

	"github.com/ingvagabund/cluster-capacity/pkg/framework"
)

type Cache struct {
	position int
	reports  []*framework.Report
	size     int
	mux      sync.Mutex
}

func NewCache(size int) *Cache {
	return &Cache{
		position: 0,
		reports:  make([]*framework.Report, 0),
		size:     size,
	}
}

func (c *Cache) GetSize() int {
	return c.size
}

func (c *Cache) Add(r *framework.Report) {
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

func (c *Cache) GetLast(num int) []*framework.Report {
	if len(c.reports) == 0 {
		return nil
	}
	sorted := c.All()

	if num >= len(sorted) {
		num = len(sorted)
	}

	return sorted[len(sorted)-num:]
}

func (c *Cache) All() []*framework.Report {
	c.mux.Lock()
	defer c.mux.Unlock()
	sorted := make([]*framework.Report, 0)

	for i := c.position; i < len(c.reports); i++ {
		sorted = append(sorted, c.reports[i])
	}
	for i := 0; i < c.position; i++ {
		sorted = append(sorted, c.reports[i])
	}
	return sorted
}
func (c *Cache) List(since time.Time, to time.Time, num int) []*framework.Report {
	all := c.All()
	if len(all) == 0 {
		return nil
	}

	list := make([]*framework.Report, 0)
	for i := 0; i < len(all); i++ {
		if all[i].Timestamp.After(since) && all[i].Timestamp.Before(to) {
			list = append(list, all[i])
		}
	}

	if num >= len(list) || num == -1 {
		num = len(list)
	}

	return list[len(list)-num:]

}
