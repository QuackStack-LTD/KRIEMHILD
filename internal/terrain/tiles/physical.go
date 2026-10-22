package tiles

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
	"sync"
)

// Eight immutable halo tiles, independent of palette and authored edits. Holding
// a planet pointer prevents identity reuse. Per-entry readiness deduplicates work.
type physicalKey struct {
	Planet        *model.Planet
	Z, X, Y, Size int
}
type physicalEntry struct {
	ready   chan struct{}
	samples []model.Sample
	err     error
	used    uint64
}

var physical = struct {
	sync.Mutex
	clock uint64
	items map[physicalKey]*physicalEntry
}{items: map[physicalKey]*physicalEntry{}}

func physicalSamples(ctx context.Context, p *model.Planet, z, x, y, size int) ([]model.Sample, error) {
	key := physicalKey{p, z, x, y, size}
	physical.Lock()
	physical.clock++
	if e := physical.items[key]; e != nil {
		e.used = physical.clock
		physical.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-e.ready:
			if e.err != nil && ctx.Err() == nil {
				return physicalSamples(ctx, p, z, x, y, size)
			}
			return e.samples, e.err
		}
	}
	e := &physicalEntry{ready: make(chan struct{}), used: physical.clock}
	physical.items[key] = e
	if len(physical.items) > 8 {
		var oldest physicalKey
		age := ^uint64(0)
		for k, v := range physical.items {
			select {
			case <-v.ready:
				if v.used < age {
					oldest, age = k, v.used
				}
			default:
			}
		}
		if age != ^uint64(0) {
			delete(physical.items, oldest)
		}
	}
	physical.Unlock()
	stride := size + 2
	samples := make([]model.Sample, stride*stride)
	sampler := detail.New(p)
	for j := -1; j <= size; j++ {
		if err := ctx.Err(); err != nil {
			e.err = err
			break
		}
		for i := -1; i <= size; i++ {
			lon, lat := sampleCoordinate(z, x, y, size, i, j)
			samples[(j+1)*stride+i+1] = sampler.Sample(lon, lat)
		}
	}
	physical.Lock()
	if e.err == nil {
		e.samples = samples
	} else {
		delete(physical.items, key)
	}
	close(e.ready)
	for len(physical.items) > 8 {
		var oldest physicalKey
		age := ^uint64(0)
		for k, v := range physical.items {
			select {
			case <-v.ready:
				if v.used < age {
					oldest, age = k, v.used
				}
			default:
			}
		}
		if age == ^uint64(0) {
			break
		}
		delete(physical.items, oldest)
	}
	physical.Unlock()
	return e.samples, e.err
}
func PhysicalKey(p *model.Planet, z, x, y, size int) string {
	return fmt.Sprintf("%s/%s/%d/%d/%d/%d", p.Spec.Version, p.Spec.Seed, z, x, y, size)
}
