package project

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type searchEntry struct{ ID, Hash, Name, Text string }
type searchIndex struct {
	entries []searchEntry
	byHash  map[string]searchEntry
	bytes   int
}
type SearchPage struct {
	Records  []Record `json:"records"`
	Next     string   `json:"next"`
	Snapshot string   `json:"snapshot"`
}
type searchCursor struct {
	Snapshot string `json:"snapshot"`
	Query    string `json:"query"`
	Offset   int    `json:"offset"`
}

func searchable(r Record) string {
	properties := map[string]any{}
	for key, value := range r.Properties {
		if !strings.HasPrefix(key, "_") {
			properties[key] = value
		}
	}
	metadata, _ := json.Marshal(properties)
	return strings.ToLower(strings.Join([]string{r.ID, r.Kind, r.Name, r.Type, r.Status, r.Notes, r.Story, r.From, r.To, textDocument(r.Document), string(metadata)}, "\n"))
}
func (s *Store) searchIndex(snapshot string) (*searchIndex, error) {
	if index := s.searchIndexes[snapshot]; index != nil {
		return index, nil
	}
	snap, e := s.snapshot(snapshot)
	if e != nil {
		return nil, e
	}
	index := &searchIndex{byHash: map[string]searchEntry{}}
	for id, hash := range snap.Records {
		entry := searchEntry{}
		for _, prior := range s.searchIndexes {
			if cached, ok := prior.byHash[hash]; ok {
				entry = cached
				break
			}
		}
		if entry.ID == "" {
			var r Record
			if e = s.read("objects", hash, &r); e != nil {
				return nil, e
			}
			entry = searchEntry{id, hash, r.Name, searchable(r)}
		}
		index.entries = append(index.entries, entry)
		index.byHash[hash] = entry
		index.bytes += len(entry.ID) + len(entry.Hash) + len(entry.Name) + len(entry.Text) + 128
		if index.bytes > 96<<20 {
			return nil, fmt.Errorf("search index exceeds the current 96 MiB budget; narrow the project workload")
		}
	}
	sort.Slice(index.entries, func(i, j int) bool {
		a, b := index.entries[i], index.entries[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
	// Keep two snapshots at most. Unchanged object text is reused while building
	// a changed Age, so edits do not reread every unchanged manuscript.
	if s.searchIndexes == nil {
		s.searchIndexes = map[string]*searchIndex{}
	}
	for len(s.searchIndexes) >= 2 {
		delete(s.searchIndexes, s.searchOrder[0])
		s.searchOrder = s.searchOrder[1:]
	}
	for s.searchMemory()+index.bytes > 128<<20 && len(s.searchOrder) > 0 {
		delete(s.searchIndexes, s.searchOrder[0])
		s.searchOrder = s.searchOrder[1:]
	}
	s.searchIndexes[snapshot] = index
	s.searchOrder = append(s.searchOrder, snapshot)
	return index, nil
}
func (s *Store) searchMemory() int {
	total := 0
	for _, index := range s.searchIndexes {
		total += index.bytes
	}
	return total
}

func (s *Store) SearchPage(age, q, cursor string, limit int) (SearchPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := SearchPage{Records: []Record{}}
	if limit < 1 || limit > 200 {
		return out, fmt.Errorf("search page size must be 1–200")
	}
	q = strings.ToLower(strings.TrimSpace(q))
	if len(q) > 500 {
		return out, fmt.Errorf("search query is too long")
	}
	_, root, e := s.root()
	if e != nil {
		return out, e
	}
	a, ok := root.Ages[age]
	if !ok {
		return out, fmt.Errorf("Age not found")
	}
	out.Snapshot = a.Snapshot
	digest := sha256.Sum256([]byte(q))
	key := hex.EncodeToString(digest[:])
	offset := 0
	if cursor != "" {
		raw, e := base64.RawURLEncoding.DecodeString(cursor)
		var c searchCursor
		if e != nil || len(raw) > 1024 || json.Unmarshal(raw, &c) != nil {
			return out, fmt.Errorf("invalid search cursor")
		}
		if c.Snapshot != a.Snapshot || c.Query != key {
			return out, ErrConflict
		}
		offset = c.Offset
	}
	index, e := s.searchIndex(a.Snapshot)
	if e != nil {
		return out, e
	}
	if offset < 0 || offset > len(index.entries) {
		return out, fmt.Errorf("invalid search offset")
	}
	for i := offset; i < len(index.entries); i++ {
		entry := index.entries[i]
		if !strings.Contains(entry.Text, q) {
			continue
		}
		if len(out.Records) == limit {
			raw, _ := json.Marshal(searchCursor{a.Snapshot, key, i})
			out.Next = base64.RawURLEncoding.EncodeToString(raw)
			break
		}
		var r Record
		if e = s.read("objects", entry.Hash, &r); e != nil {
			return out, e
		}
		out.Records = append(out.Records, r)
	}
	return out, nil
}
