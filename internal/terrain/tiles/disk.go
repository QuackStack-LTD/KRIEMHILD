package tiles

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Optional disposable presentation cache. Canonical terrain remains in the project.
// Hashed names, content checksums and a renderer revision prevent stale/corrupt hits.
func NewPersistentCache(memoryLimit int, root string, diskLimit int64) *Cache {
	c := NewCache(memoryLimit)
	if os.MkdirAll(root, 0700) == nil {
		c.diskRoot = root
		c.diskLimit = diskLimit
	}
	return c
}
func (c *Cache) diskPath(key string) string {
	return filepath.Join(c.diskRoot, fmt.Sprintf("%x.tile", sha256.Sum256([]byte("terrain-render-5/"+key))))
}
func (c *Cache) readDisk(key string) ([]byte, bool) {
	if c.diskRoot == "" {
		return nil, false
	}
	path := c.diskPath(key)
	info, err := os.Stat(path)
	if err != nil || info.Size() > int64(c.limit)+32 || info.Size() < 32 {
		return nil, false
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 32 {
		return nil, false
	}
	sum := sha256.Sum256(b[32:])
	if !bytes.Equal(sum[:], b[:32]) {
		_ = os.Remove(path)
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	return b[32:], true
}
func (c *Cache) writeDisk(key string, data []byte) {
	if c.diskRoot == "" || int64(len(data)+32) > c.diskLimit {
		return
	}
	path := c.diskPath(key)
	if _, err := os.Stat(path); err == nil {
		return
	}
	f, err := os.CreateTemp(c.diskRoot, "tile-*.tmp")
	if err != nil {
		return
	}
	name := f.Name()
	defer os.Remove(name)
	sum := sha256.Sum256(data)
	_, e1 := f.Write(sum[:])
	_, e2 := f.Write(data)
	e3 := f.Close()
	if e1 != nil || e2 != nil || e3 != nil {
		return
	}
	if os.Rename(name, path) != nil {
		return
	}
	entries, err := os.ReadDir(c.diskRoot)
	if err != nil {
		return
	}
	type file struct {
		name     string
		size     int64
		modified time.Time
	}
	files := []file{}
	total := int64(0)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".tile" {
			continue
		}
		i, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, file{e.Name(), i.Size(), i.ModTime()})
		total += i.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.Before(files[j].modified) })
	for _, f := range files {
		if total <= c.diskLimit {
			break
		}
		if os.Remove(filepath.Join(c.diskRoot, f.name)) == nil {
			total -= f.size
		}
	}
}
