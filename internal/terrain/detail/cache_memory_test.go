package detail

import (
	"encoding/json"
	"kriemhild/internal/terrain/model"
	"runtime"
	"testing"
	"weak"
)

func transientSampler() weak.Pointer[model.Planet] {
	p := &model.Planet{Spec: model.DefaultSpec(), Surface: &model.Surface{}}
	New(p)
	return weak.Make(p)
}

func TestSamplerCacheDoesNotRetainClosedPlanet(t *testing.T) {
	p := transientSampler()
	for i := 0; i < 4; i++ {
		runtime.GC()
	}
	if p.Value() != nil {
		t.Fatal("derived sampler pinned a closed planet")
	}
}

func TestSamplerOwnedCachePreservesSerializationAndCopyIdentity(t *testing.T) {
	p := &model.Planet{Spec: model.DefaultSpec(), Surface: &model.Surface{}}
	before, _ := json.Marshal(p)
	first := New(p)
	if New(p) != first {
		t.Fatal("live terrain rebuilt its sampler")
	}
	copy := *p
	if New(&copy) == first {
		t.Fatal("copied planet reused another planet's sampler")
	}
	after, _ := json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("derived cache changed saved planet format")
	}
}
