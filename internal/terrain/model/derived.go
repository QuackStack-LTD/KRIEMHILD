package model

import "sync"

type derivedCache struct {
	owner  *Planet
	mu     sync.Mutex
	values map[string]any
}

var derivedInit sync.Mutex

// Derived owns disposable indexes for this immutable planet. Keeping them on
// their owner lets GC collect both when a terrain session closes, including
// indexes that refer back to the planet. Nothing here is serialized. Builders
// must not recursively call Derived; returned values must be treated as immutable.
func (p *Planet) Derived(key string, build func() any) any {
	derivedInit.Lock()
	if p.derived == nil || p.derived.owner != p {
		p.derived = &derivedCache{owner: p, values: map[string]any{}}
	}
	c := p.derived
	derivedInit.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if value, ok := c.values[key]; ok {
		return value
	}
	value := build()
	c.values[key] = value
	return value
}
