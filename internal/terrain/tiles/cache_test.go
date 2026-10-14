package tiles

import "testing"

func TestCacheBoundAndLRU(t *testing.T) {
	c := NewCache(6)
	c.Put("a", []byte{1, 2, 3})
	c.Put("b", []byte{4, 5, 6})
	if _, ok := c.Get("a"); !ok {
		t.Fatal("missing entry")
	}
	c.Put("c", []byte{7, 8})
	if _, ok := c.Get("b"); ok {
		t.Fatal("least recently used entry not evicted")
	}
	c.Put("a", []byte{9})
	c.Put("oversize", make([]byte, 7))
	if c.used != 3 || len(c.items) != 2 {
		t.Fatal("replacement or size accounting failed")
	}
}
