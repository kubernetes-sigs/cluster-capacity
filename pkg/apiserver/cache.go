package apiserver

import (
	"sync"
	"time"
)

type Cache struct {
	position int
	reports  []*Report
	size     int
	mux      sync.Mutex
}

func NewCache(size int) *Cache {
	return &Cache{
		position: 0,
		reports:  make([]*Report, 0),
		size:     size,
	}
}

func (c *Cache) Add(r *Report) {
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

func (c *Cache) GetLast(num int) []*Report {
	if len(c.reports) == 0 {
		return nil
	}
	sorted := c.All()

	if num >= len(sorted) {
		num = len(sorted)
	}

	return sorted[len(sorted)-num:]
}

func (c *Cache) All() []*Report {
	c.mux.Lock()
	defer c.mux.Unlock()
	sorted := make([]*Report, 0)

	for i := c.position; i < len(c.reports); i++ {
		sorted = append(sorted, c.reports[i])
	}
	for i := 0; i < c.position; i++ {
		sorted = append(sorted, c.reports[i])
	}
	return sorted
}
func (c *Cache) List(since time.Time, to time.Time) []*Report {
	all := c.All()
	if len(all) == 0 {
		return nil
	}

	list := make([]*Report, 0)
	for i := 0; i < len(all); i++ {
		if all[i].Timestamp.After(since) && all[i].Timestamp.Before(to) {
			list = append(list, all[i])
		}
	}
	return list

}
