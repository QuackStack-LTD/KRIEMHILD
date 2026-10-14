package tiles

import (
	"container/list"
	"sync"
)

// Cache bounds encoded texture memory. Eviction has no effect on terrain data.
type Cache struct {
	mu          sync.Mutex
	limit, used int
	items       map[string]*list.Element
	lru         *list.List
}
type entry struct {
	key  string
	data []byte
}

func NewCache(limit int) *Cache {
	return &Cache{limit: limit, items: map[string]*list.Element{}, lru: list.New()}
}
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.items[key]
	if e == nil {
		return nil, false
	}
	c.lru.MoveToFront(e)
	return e.Value.(entry).data, true
}
func (c *Cache) Put(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(data) > c.limit {
		return
	}
	if e := c.items[key]; e != nil {
		c.used -= len(e.Value.(entry).data)
		c.lru.Remove(e)
		delete(c.items, key)
	}
	for c.used+len(data) > c.limit {
		e := c.lru.Back()
		v := e.Value.(entry)
		delete(c.items, v.key)
		c.used -= len(v.data)
		c.lru.Remove(e)
	}
	c.items[key] = c.lru.PushFront(entry{key, data})
	c.used += len(data)
}
