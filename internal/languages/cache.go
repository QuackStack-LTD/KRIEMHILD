package languages

import (
	"container/list"
	"sync"
)

// Cache retains immutable artifacts using bounded LRU reservations. Oversized
// artifacts execute in their bounded job but are not retained. Compilation never
// holds this mutex. Reservations are conservative accounting, not heap telemetry.
type Cache struct {
	mu           sync.Mutex
	budget, used int64
	order        *list.List
	entries      map[string]*list.Element
}
type cacheEntry struct {
	key      string
	artifact *Artifact
	bytes    int64
}

func NewCache(budget int64) *Cache {
	if budget < 0 {
		budget = 0
	}
	return &Cache{budget: budget, order: list.New(), entries: map[string]*list.Element{}}
}
func ArtifactKey(world, age string, d Definition) string {
	return Hash(struct{ World, Age, Revision, Engine, Adapter string }{world, age, Hash(d), EngineVersion, AdapterVersion})
}
func (a *Artifact) AccountedBytes() int64 {
	return int64(len(Canonical(a.definition)))*12 + int64(a.report.States)*1024 + int64(a.report.Forms)*4096 + int64(len(Canonical(a.report)))*4 + 65536
}
func (c *Cache) Get(key string) *Artifact {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil {
		return nil
	}
	c.order.MoveToFront(entry)
	return entry.Value.(cacheEntry).artifact
}
func (c *Cache) Put(key string, a *Artifact) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	size := a.AccountedBytes()
	if size > c.budget || a.report.Status == "cancelled" {
		return false
	}
	if old := c.entries[key]; old != nil {
		c.used -= old.Value.(cacheEntry).bytes
		c.order.Remove(old)
		delete(c.entries, key)
	}
	for c.used+size > c.budget {
		last := c.order.Back()
		entry := last.Value.(cacheEntry)
		c.used -= entry.bytes
		delete(c.entries, entry.key)
		c.order.Remove(last)
	}
	c.entries[key] = c.order.PushFront(cacheEntry{key, a, size})
	c.used += size
	return true
}
func (c *Cache) Stats() (entries int, used, budget int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries), c.used, c.budget
}
