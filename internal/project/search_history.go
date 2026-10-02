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

type AgeSearchHit struct {
	Age       string `json:"age"`
	AgeName   string `json:"ageName"`
	SourceAge string `json:"sourceAge,omitempty"`
	Snapshot  string `json:"snapshot"`
	Record    Record `json:"record"`
}
type AgeSearchPage struct {
	Hits        []AgeSearchHit `json:"hits"`
	Next        string         `json:"next"`
	Revision    string         `json:"revision"`
	AgesScanned int            `json:"agesScanned"`
}
type ageSearchCursor struct {
	Revision string `json:"revision"`
	Query    string `json:"query"`
	Age      int    `json:"age"`
	Offset   int    `json:"offset"`
}

// SearchAcrossAges returns bounded pages with explicit Age/snapshot context.
// It builds at most three Age indexes per request, even for an absent query.
func (s *Store) SearchAcrossAges(query, cursor string, limit int) (AgeSearchPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := AgeSearchPage{Hits: []AgeSearchHit{}}
	query = strings.ToLower(strings.TrimSpace(query))
	if len(query) > 500 || limit < 1 || limit > 200 {
		return out, fmt.Errorf("invalid search query or page size")
	}
	revision, root, e := s.root()
	if e != nil {
		return out, e
	}
	out.Revision = revision
	ages := make([]Age, 0, len(root.Ages))
	for _, a := range root.Ages {
		ages = append(ages, a)
	}
	sort.Slice(ages, func(i, j int) bool {
		if ages[i].Name != ages[j].Name {
			return ages[i].Name < ages[j].Name
		}
		return ages[i].ID < ages[j].ID
	})
	digest := sha256.Sum256([]byte(query))
	key := hex.EncodeToString(digest[:])
	position := ageSearchCursor{Revision: revision, Query: key}
	if cursor != "" {
		if len(cursor) > 2048 {
			return out, fmt.Errorf("invalid search cursor")
		}
		raw, e := base64.RawURLEncoding.DecodeString(cursor)
		if e != nil || json.Unmarshal(raw, &position) != nil {
			return out, fmt.Errorf("invalid search cursor")
		}
		if position.Revision != revision || position.Query != key {
			return out, ErrConflict
		}
	}
	if position.Age < 0 || position.Age >= len(ages) || position.Offset < 0 {
		return out, fmt.Errorf("invalid search position")
	}
	next := func(age, offset int) {
		raw, _ := json.Marshal(ageSearchCursor{revision, key, age, offset})
		out.Next = base64.RawURLEncoding.EncodeToString(raw)
	}
	for a := position.Age; a < len(ages); a++ {
		if out.AgesScanned == 3 {
			next(a, 0)
			break
		}
		age := ages[a]
		index, e := s.searchIndex(age.Snapshot)
		if e != nil {
			return out, e
		}
		out.AgesScanned++
		offset := 0
		if a == position.Age {
			offset = position.Offset
		}
		if offset > len(index.entries) {
			return out, fmt.Errorf("invalid search offset")
		}
		for i := offset; i < len(index.entries); i++ {
			entry := index.entries[i]
			if !strings.Contains(entry.Text, query) {
				continue
			}
			if len(out.Hits) == limit {
				next(a, i)
				return out, nil
			}
			var r Record
			if e = s.read("objects", entry.Hash, &r); e != nil {
				return out, e
			}
			out.Hits = append(out.Hits, AgeSearchHit{age.ID, age.Name, age.SourceAge, age.Snapshot, r})
		}
	}
	return out, nil
}
