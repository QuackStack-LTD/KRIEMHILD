package tiles

import (
	"kriemhild/internal/terrain/model"
	"runtime"
	"testing"
	"weak"
)

func cacheTransientPlanet() weak.Pointer[model.Planet] {
	p := &model.Planet{Spec: model.DefaultSpec()}
	key := physicalKey{Planet: weak.Make(p)}
	ready := make(chan struct{})
	close(ready)
	physical.items[key] = &physicalEntry{ready: ready, samples: make([]model.Sample, 1)}
	return weak.Make(p)
}

func TestPhysicalCacheDoesNotPinClosedWorldsAndUsesByteBudget(t *testing.T) {
	physical.Lock()
	defer physical.Unlock()
	previous := physical.items
	physical.items = map[physicalKey]*physicalEntry{}
	defer func() { physical.items = previous }()
	dead := cacheTransientPlanet()
	for i := 0; i < 4; i++ {
		runtime.GC()
	}
	if dead.Value() != nil {
		t.Fatal("presentation cache retained a closed world")
	}
	physical.items = map[physicalKey]*physicalEntry{}
	p := &model.Planet{Spec: model.DefaultSpec()}
	for i := 0; i < 3; i++ {
		ready := make(chan struct{})
		close(ready)
		physical.items[physicalKey{Planet: weak.Make(p), X: i}] = &physicalEntry{ready: ready, samples: make([]model.Sample, 16), used: uint64(i)}
	}
	trimPhysicalCache(4000)
	if physicalCacheBytes() > 4000 || len(physical.items) != 2 {
		t.Fatal("sample cache bounded tile count instead of actual storage")
	}
	if _, ok := physical.items[physicalKey{Planet: weak.Make(p), X: 0}]; ok {
		t.Fatal("sample byte eviction ignored LRU")
	}
	runtime.KeepAlive(p)
}
