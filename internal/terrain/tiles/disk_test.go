package tiles

import (
	"bytes"
	"os"
	"testing"
)

func TestPersistentCacheRestartCorruptionAndBudget(t *testing.T) {
	root := t.TempDir()
	c := NewPersistentCache(1024, root, 300)
	want := bytes.Repeat([]byte{7}, 100)
	c.Put("planet/tile", want)
	reopened := NewPersistentCache(1024, root, 300)
	got, ok := reopened.Get("planet/tile")
	if !ok || !bytes.Equal(got, want) {
		t.Fatal("restart lost cached tile")
	}
	if err := os.WriteFile(c.diskPath("planet/tile"), bytes.Repeat([]byte{2}, 132), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok = reopened.Get("planet/tile"); ok {
		t.Fatal("corrupt tile accepted")
	}
	for _, key := range []string{"a", "b", "c", "d"} {
		c.Put(key, want)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	total := int64(0)
	for _, e := range entries {
		i, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		total += i.Size()
	}
	if total > 300 {
		t.Fatalf("disk budget exceeded: %d", total)
	}
}
