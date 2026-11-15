package tiles

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
	"sync"
	"unsafe"
	"weak"
)

// Eight immutable halo tiles, independent of palette and authored edits. Holding
// weak planet identity prevents reuse without retaining closed worlds.
type physicalKey struct {
	Planet        weak.Pointer[model.Planet]
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
	key := physicalKey{weak.Make(p), z, x, y, size}
	physical.Lock()
	for k := range physical.items {
		if k.Planet.Value() == nil {
			delete(physical.items, k)
		}
	}
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
	trimPhysicalCache(64 << 20)
	physical.Unlock()
	return e.samples, e.err
}

// Caller holds physical.Mutex; in-flight work is never evicted.
func trimPhysicalCache(limit int) {
	for len(physical.items) > 8 || physicalCacheBytes() > limit {
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
}

// Caller holds physical.Mutex. Count sample storage rather than just tiles:
// a 512-pixel tile needs nearly four times the RAM of a 256-pixel tile.
func physicalCacheBytes() int {
	bytes := 0
	for _, e := range physical.items {
		bytes += cap(e.samples) * int(unsafe.Sizeof(model.Sample{}))
	}
	return bytes
}
func PhysicalKey(p *model.Planet, z, x, y, size int) string {
	return fmt.Sprintf("%s/%s/%d/%d/%d/%d", p.Spec.Version, p.Spec.Seed, z, x, y, size)
}
