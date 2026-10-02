package project

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

var ErrConflict = errors.New("the project changed; reload before applying this edit")

type Store struct {
	validationAssets      map[string]bool
	validationObjects     map[string]Record
	validationObjectBytes int
	searchIndexes         map[string]*searchIndex
	searchOrder           []string
	validationRecords     map[string]map[string]Record
	validationBytes       int
	cache                 map[string]*list.Element
	cacheOrder            *list.List
	cacheBytes            int
	liveMu                sync.Mutex
	rooms                 map[string]*liveRoom
	mu                    sync.Mutex
	Dir                   string
	lock                  *flock.Flock
	recovered             bool
	fail                  func(string) error
}

const objectCacheLimit = 32 << 20

type cachedObject struct {
	key  string
	data []byte
	info os.FileInfo
}

// Cache only verified serialized bytes. Every read still checks the file's
// identity, size and modification time, and decoding gives callers independent
// mutable values. The cache is derived, bounded, and empty after restart.
func (s *Store) objectBytes(dir, id string) ([]byte, error) {
	key := dir + "/" + id
	path := filepath.Join(s.Dir, dir, id)
	info, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("object must be a regular file")
	}
	if item := s.cache[key]; item != nil {
		cached := item.Value.(cachedObject)
		if cached.info.Size() == info.Size() && cached.info.ModTime().Equal(info.ModTime()) && os.SameFile(cached.info, info) {
			s.cacheOrder.MoveToFront(item)
			return cached.data, nil
		}
		s.cacheBytes -= len(cached.data)
		s.cacheOrder.Remove(item)
		delete(s.cache, key)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	hash := sha256.Sum256(b)
	if hex.EncodeToString(hash[:]) != id {
		return nil, fmt.Errorf("integrity check failed for %s", id)
	}
	if len(b) <= objectCacheLimit/4 {
		if s.cache == nil {
			s.cache = map[string]*list.Element{}
			s.cacheOrder = list.New()
		}
		for s.cacheBytes+len(b) > objectCacheLimit || len(s.cache) >= 65536 {
			last := s.cacheOrder.Back()
			v := last.Value.(cachedObject)
			s.cacheBytes -= len(v.data)
			delete(s.cache, v.key)
			s.cacheOrder.Remove(last)
		}
		s.cache[key] = s.cacheOrder.PushFront(cachedObject{key, b, info})
		s.cacheBytes += len(b)
	}
	return b, nil
}

func syncWrite(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func atomicWrite(path string, data []byte) error {
	tmp := path + "." + NewID() + ".tmp"
	defer os.Remove(tmp)
	if err := syncWrite(tmp, data); err != nil {
		return err
	}
	return replaceFile(tmp, path)
}
func WriteAtomic(path string, data []byte) error { return atomicWrite(path, data) }
func (s *Store) put(dir string, v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	return s.putBytes(dir, b)
}
func (s *Store) putBytes(dir string, b []byte) (string, error) {
	h := sha256.Sum256(b)
	id := hex.EncodeToString(h[:])
	p := filepath.Join(s.Dir, dir, id)
	existing, e := os.ReadFile(p)
	if e == nil {
		if !bytes.Equal(existing, b) {
			return "", fmt.Errorf("corrupt existing object %s", id)
		}
		return id, nil
	}
	if !os.IsNotExist(e) {
		return "", e
	}
	return id, atomicWrite(p, b)
}
func (s *Store) read(dir, id string, v any) error {
	if !validHash(id) {
		return fmt.Errorf("invalid object hash")
	}
	b, e := s.objectBytes(dir, id)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func (s *Store) Root() (string, Root, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.root() }
func (s *Store) root() (string, Root, error) {
	var head Head
	var root Root
	b, e := os.ReadFile(filepath.Join(s.Dir, "project.head.json"))
	if e != nil {
		return "", root, e
	}
	if e = json.Unmarshal(b, &head); e != nil {
		return "", root, e
	}
	e = s.read("revisions", head.Revision, &root)
	if e == nil && root.Version != Format {
		e = fmt.Errorf("unsupported project format %d; files have not been changed", root.Version)
	}
	return head.Revision, root, e
}
func (s *Store) snapshot(id string) (Snapshot, error) {
	var v Snapshot
	e := s.read("snapshots", id, &v)
	if e == nil && (v.Version != Format || v.Records == nil) {
		e = fmt.Errorf("unsupported snapshot")
	}
	return v, e
}
func (s *Store) records(id string) (map[string]Record, error) {
	if cached := s.validationRecords[id]; cached != nil {
		return cached, nil
	}
	snap, e := s.snapshot(id)
	if e != nil {
		return nil, e
	}
	out := map[string]Record{}
	for id, hash := range snap.Records {
		r, err := s.validationRecord(hash)
		if err != nil {
			return nil, err
		}
		out[id] = r
	}
	// Validation never mutates records. Reuse a bounded working set while
	// checking shared scene pins and repeated Age snapshots in this one call.
	if s.validationRecords != nil && len(out) <= 20000 {
		if data, err := json.Marshal(out); err == nil && s.validationBytes+len(data) <= 16<<20 {
			s.validationRecords[id] = out
			s.validationBytes += len(data)
		}
	}
	return out, nil
}

// Validators do not mutate records. Reuse decoded immutable objects only within
// one validation pass, including objects shared by different historical snapshots.
// Nothing mutable survives into a subsequent command or State response.
func (s *Store) validationRecord(hash string) (Record, error) {
	if cached, ok := s.validationObjects[hash]; ok {
		return cached, nil
	}
	var r Record
	if e := s.read("objects", hash, &r); e != nil {
		return r, e
	}
	if s.validationObjects != nil {
		if raw, e := json.Marshal(r); e == nil && s.validationObjectBytes+len(raw)+512 <= 16<<20 {
			s.validationObjects[hash] = r
			s.validationObjectBytes += len(raw) + 512
		}
	}
	return r, nil
}
func (s *Store) validateSnapshot(id string, seen map[string]bool) error {
	if seen[id] {
		return nil
	}
	seen[id] = true
	records, e := s.records(id)
	if e != nil {
		return e
	}
	if e = validateRecords(records); e != nil {
		return e
	}
	if e = validateIntervals(records); e != nil {
		return e
	}
	for _, r := range records {
		if r.Chronology != nil {
			if e := s.validateSnapshot(r.Chronology.Baseline, seen); e != nil {
				return e
			}
		}
		if r.Event != nil {
			for _, ch := range r.Event.Changes {
				for _, hash := range []string{ch.Before, ch.After} {
					if hash == "" {
						continue
					}
					value, e := s.validationRecord(hash)
					if e != nil {
						return e
					}
					if value.ID != ch.Target || !safeNarrative(value) {
						return fmt.Errorf("invalid historical change target")
					}
				}
			}
		}
		if r.Asset != "" {
			if e := s.validateAsset(r.Asset); e != nil {
				return e
			}
		}
		if r.Kind == "scene" {
			if e := s.validateSnapshot(r.SettingSnapshot, seen); e != nil {
				return e
			}
			setting, e := s.records(r.SettingSnapshot)
			if e != nil {
				return fmt.Errorf("missing scene setting: %w", e)
			}
			if r.SettingDate != "" {
				historical, err := s.historical(r.SettingSnapshot, r.SettingDate)
				if err != nil {
					return err
				}
				setting = historical.Records
			}
			for _, id := range r.References {
				ref, ok := setting[id]
				if !ok || ref.Kind != "entity" {
					return fmt.Errorf("scene reference is absent from its pinned setting")
				}
			}
			for _, key := range []string{"povID", "locationID"} {
				if id, ok := r.Properties[key].(string); ok && id != "" && setting[id].Kind != "entity" {
					return fmt.Errorf("scene %s must exist in its pinned historical setting", key)
				}
			}
		}
	}
	return nil
}
func (s *Store) validateRoot(root Root) error {
	s.validationAssets = map[string]bool{}
	s.validationRecords = map[string]map[string]Record{}
	s.validationBytes = 0
	s.validationObjects = map[string]Record{}
	s.validationObjectBytes = 0
	defer func() {
		s.validationAssets = nil
		s.validationRecords = nil
		s.validationBytes = 0
		s.validationObjects = nil
		s.validationObjectBytes = 0
	}()
	if root.Version != Format {
		return fmt.Errorf("unsupported project version")
	}
	if !validID(root.World.ID) || !nameOK(root.World.Name) || root.World.Version != Format {
		return fmt.Errorf("invalid world metadata")
	}
	if len(root.MergeParents) > 8 {
		return fmt.Errorf("too many revision merge parents")
	}
	parents := map[string]bool{}
	for _, id := range append([]string{root.Parent}, root.MergeParents...) {
		if id == "" {
			continue
		}
		if !validHash(id) || parents[id] {
			return fmt.Errorf("invalid or duplicate revision parent")
		}
		parents[id] = true
		var parent Root
		if e := s.read("revisions", id, &parent); e != nil {
			return e
		}
		if parent.World.ID != root.World.ID {
			return fmt.Errorf("revision parent belongs to another world")
		}
	}
	if len(root.Ages) == 0 || len(root.Ages) > 10000 {
		return fmt.Errorf("world must contain 1–10000 Ages")
	}
	seen := map[string]bool{}
	for id, a := range root.Ages {
		if a.ID != id || !validID(id) || !nameOK(a.Name) {
			return fmt.Errorf("invalid Age metadata")
		}
		if a.SourceAge != "" {
			if _, ok := root.Ages[a.SourceAge]; !ok {
				return fmt.Errorf("source Age not found")
			}
			lineage := map[string]bool{id: true}
			for parent := a.SourceAge; parent != ""; parent = root.Ages[parent].SourceAge {
				if lineage[parent] {
					return fmt.Errorf("Age source lineage is cyclic")
				}
				lineage[parent] = true
			}
		}
		if e := s.validateSnapshot(a.Snapshot, seen); e != nil {
			return e
		}
		records, err := s.records(a.Snapshot)
		if err != nil {
			return err
		}
		for _, r := range records {
			if r.Chronology != nil && r.Chronology.Age != a.ID {
				return fmt.Errorf("chronology belongs to a different Age")
			}
			if r.Kind == "scene" {
				if _, ok := root.Ages[r.SettingAge]; !ok {
					return fmt.Errorf("scene setting Age not found")
				}
			}
		}
		for _, id := range append(append([]string{}, a.Undo...), a.Redo...) {
			if e := s.validateSnapshot(id, seen); e != nil {
				return e
			}
		}
		if a.SourceSnapshot != "" {
			if e := s.validateSnapshot(a.SourceSnapshot, seen); e != nil {
				return e
			}
		}
	}
	return nil
}

func (s *Store) validateAsset(id string) error {
	if s.validationAssets[id] {
		return nil
	}
	if !validHash(id) {
		return fmt.Errorf("invalid asset reference")
	}
	path := filepath.Join(s.Dir, "assets", id)
	info, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("asset must be a regular file")
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	hash := sha256.New()
	if _, e = io.Copy(hash, f); e != nil {
		return e
	}
	if hex.EncodeToString(hash.Sum(nil)) != id {
		return fmt.Errorf("corrupt asset")
	}
	if s.validationAssets != nil && len(s.validationAssets) < 100000 {
		s.validationAssets[id] = true
	}
	return nil
}

func Open(dir string) (*Store, error) {
	if e := os.MkdirAll(filepath.Join(dir, ".kriemhild"), 0700); e != nil {
		return nil, e
	}
	s := &Store{Dir: dir, lock: flock.New(filepath.Join(dir, ".kriemhild", "writer.lock"))}
	ok, e := s.lock.TryLock()
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, fmt.Errorf("project is already open in another KRIEMHILD process")
	}
	_, root, e := s.root()
	if root.Version != 0 && root.Version != Format {
		s.Close()
		return nil, fmt.Errorf("unsupported project format %d; files have not been changed", root.Version)
	}
	if e == nil {
		e = s.validateRoot(root)
	}
	if e != nil {
		// Only an interrupted commit with a valid previous root permits automatic recovery.
		var tx struct {
			Old string `json:"old"`
			New string `json:"new"`
		}
		b, je := os.ReadFile(filepath.Join(dir, ".kriemhild", "transaction.json"))
		if je == nil {
			je = json.Unmarshal(b, &tx)
		}
		var old Root
		if je == nil {
			je = s.read("revisions", tx.Old, &old)
		}
		if je == nil {
			je = s.validateRoot(old)
		}
		if je != nil {
			s.Close()
			return nil, fmt.Errorf("cannot safely open project: %w", e)
		}
		h, _ := json.Marshal(Head{tx.Old})
		if e = atomicWrite(filepath.Join(dir, "project.head.json"), h); e != nil {
			s.Close()
			return nil, e
		}
		s.recovered = true
	}
	os.Remove(filepath.Join(dir, ".kriemhild", "transaction.json"))
	return s, nil
}
func Create(dir, name, ageName string) (*Store, string, error) {
	if !nameOK(name) || !nameOK(ageName) {
		return nil, "", fmt.Errorf("world and Age names are required (maximum 300 characters)")
	}
	if e := os.Mkdir(dir, 0700); e != nil {
		return nil, "", e
	}
	for _, d := range []string{"objects", "snapshots", "revisions", "assets", ".kriemhild"} {
		if e := os.Mkdir(filepath.Join(dir, d), 0700); e != nil {
			return nil, "", e
		}
	}
	s := &Store{Dir: dir, lock: flock.New(filepath.Join(dir, ".kriemhild", "writer.lock"))}
	ok, e := s.lock.TryLock()
	if e != nil || !ok {
		return nil, "", fmt.Errorf("cannot lock new project")
	}
	marker := Marker{Format, filepath.Base(dir), name}
	b, _ := json.MarshalIndent(marker, "", "  ")
	if e = atomicWrite(filepath.Join(dir, "kriemhild.json"), b); e != nil {
		s.Close()
		return nil, "", e
	}
	snap, e := s.put("snapshots", Snapshot{Format, map[string]string{}})
	if e != nil {
		s.Close()
		return nil, "", e
	}
	age := Age{ID: NewID(), Name: ageName, Snapshot: snap, Undo: []string{}, Redo: []string{}}
	root := Root{Version: Format, World: marker, Ages: map[string]Age{age.ID: age}, Message: "Created world"}
	_, e = s.commit("", root)
	if e != nil {
		s.Close()
		return nil, "", e
	}
	return s, age.ID, nil
}
func (s *Store) Close() error { return s.lock.Unlock() }
func (s *Store) checkpoint(stage string) error {
	if s.fail != nil {
		return s.fail(stage)
	}
	return nil
}
func (s *Store) commit(previous string, root Root, merged ...string) (string, error) {
	root.Parent = previous
	root.MergeParents = append([]string(nil), merged...)
	if e := s.validateRoot(root); e != nil {
		return "", e
	}
	root.SavedAt = time.Now().UTC().Format(time.RFC3339Nano)
	id, e := s.put("revisions", root)
	if e != nil {
		return "", e
	}
	if e = s.checkpoint("objects"); e != nil {
		return "", e
	}
	tx, _ := json.Marshal(map[string]string{"old": previous, "new": id})
	if e = atomicWrite(filepath.Join(s.Dir, ".kriemhild", "transaction.json"), tx); e != nil {
		return "", e
	}
	if e = s.checkpoint("journal"); e != nil {
		return "", e
	}
	// Check again after staging: never overwrite a root changed by an external tool.
	if previous != "" {
		actual, _, err := s.root()
		if err != nil {
			return "", err
		}
		if actual != previous {
			return "", ErrConflict
		}
	}
	h, _ := json.Marshal(Head{id})
	if e = atomicWrite(filepath.Join(s.Dir, "project.head.json"), h); e != nil {
		return "", e
	}
	if e = s.checkpoint("head"); e != nil {
		return "", e
	}
	os.Remove(filepath.Join(s.Dir, ".kriemhild", "transaction.json"))
	return id, nil
}
func (s *Store) State(ageID string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state(ageID)
}
func (s *Store) state(ageID string) (State, error) {
	id, root, e := s.root()
	if e != nil {
		return State{}, e
	}
	if ageID == "" {
		ids := make([]string, 0, len(root.Ages))
		for id := range root.Ages {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		if len(ids) > 0 {
			ageID = ids[0]
		}
	}
	a, ok := root.Ages[ageID]
	if !ok {
		return State{}, fmt.Errorf("Age not found")
	}
	records, e := s.records(a.Snapshot)
	return State{Revision: id, Root: root, Age: a, Records: records, Recovered: s.recovered}, e
}
func (s *Store) Apply(c Command) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rev, root, e := s.root()
	if e != nil {
		return State{}, e
	}
	if c.Expected != rev {
		return State{}, ErrConflict
	}
	age, ok := root.Ages[c.Age]
	if !ok {
		return State{}, fmt.Errorf("Age not found")
	}
	old := age.Snapshot
	switch c.Action {
	case "copy-age", "import-records":
		if !nameOK(c.Name) {
			return State{}, fmt.Errorf("Age name is required")
		}
		age = Age{ID: NewID(), Name: c.Name, Snapshot: old, SourceAge: age.ID, SourceSnapshot: old, SourceRevision: rev, Undo: []string{}, Redo: []string{}}
		snap, err := s.snapshot(old)
		if err != nil {
			return State{}, err
		}
		records, err := s.records(old)
		if err != nil {
			return State{}, err
		}
		for id, r := range records {
			if r.Chronology != nil {
				r.Chronology = &Chronology{Age: age.ID, Baseline: old, Branch: "alternate", Calendar: r.Chronology.Calendar}
				hash, err := s.put("objects", r)
				if err != nil {
					return State{}, err
				}
				snap.Records[id] = hash
			}
		}
		if c.Action == "import-records" {
			if len(c.Records) == 0 || len(c.Records) > 500 {
				return State{}, fmt.Errorf("import must contain 1–500 records")
			}
			for _, r := range c.Records {
				if snap.Records[r.ID] != "" || (r.Kind != "entity" && r.Kind != "note" && r.Kind != "scene") {
					return State{}, fmt.Errorf("import requires new entity, note or scene identities")
				}
				hash, err := s.put("objects", r)
				if err != nil {
					return State{}, err
				}
				snap.Records[r.ID] = hash
			}
		}
		age.Snapshot, err = s.put("snapshots", snap)
		if err != nil {
			return State{}, err
		}
	case "update-schema":
		if c.Record == nil || c.Record.Kind != "schema" || !validID(c.Record.ID) || !nameOK(c.Record.Name) {
			return State{}, fmt.Errorf("a valid entity type definition is required")
		}
		snap, err := s.snapshot(old)
		if err != nil {
			return State{}, err
		}
		records, err := s.records(old)
		if err != nil {
			return State{}, err
		}
		before, exists := records[c.Record.ID]
		if exists && before.Kind != "schema" {
			return State{}, fmt.Errorf("record kind cannot change")
		}
		records[c.Record.ID] = *c.Record
		changed := []Record{*c.Record}
		if exists && before.Name != c.Record.Name {
			for id, r := range records {
				if r.Kind == "entity" && r.Type == before.Name {
					r.Type = c.Record.Name
					records[id] = r
					changed = append(changed, r)
				}
			}
		}
		if err = validateRecords(records); err != nil {
			return State{}, err
		}
		for _, r := range changed {
			hash, err := s.put("objects", r)
			if err != nil {
				return State{}, err
			}
			snap.Records[r.ID] = hash
		}
		age.Snapshot, err = s.put("snapshots", snap)
		if err != nil {
			return State{}, err
		}
		if age.Snapshot == old {
			return s.state(age.ID)
		}
		age.Undo = append(age.Undo, old)
		age.Redo = []string{}
	case "configure-time":
		if c.Chronology == nil {
			return State{}, fmt.Errorf("chronology is required")
		}
		snap, err := s.snapshot(old)
		if err != nil {
			return State{}, err
		}
		records, err := s.records(old)
		if err != nil {
			return State{}, err
		}
		configuration := *c.Chronology
		configuration.Age = age.ID
		configuration.Baseline = old
		r := Record{ID: NewID(), Kind: "chronology", Name: "Age chronology"}
		for _, existing := range records {
			if existing.Chronology != nil {
				r = existing
				configuration.Baseline = existing.Chronology.Baseline
			}
		}
		r.Chronology = &configuration
		hash, err := s.put("objects", r)
		if err != nil {
			return State{}, err
		}
		snap.Records[r.ID] = hash
		age.Snapshot, err = s.put("snapshots", snap)
		if err != nil {
			return State{}, err
		}
		age.Undo = append(age.Undo, old)
		age.Redo = []string{}
	case "record-event":
		next, err := s.recordEvent(age, old, c)
		if err != nil {
			return State{}, err
		}
		age.Snapshot = next
		age.Undo = append(age.Undo, old)
		age.Redo = []string{}
	case "rename-age":
		if !nameOK(c.Name) {
			return State{}, fmt.Errorf("Age name is required")
		}
		age.Name = c.Name
	case "undo":
		if len(age.Undo) == 0 {
			return State{}, fmt.Errorf("nothing to undo")
		}
		age.Snapshot = age.Undo[len(age.Undo)-1]
		age.Undo = age.Undo[:len(age.Undo)-1]
		age.Redo = append(age.Redo, old)
	case "redo":
		if len(age.Redo) == 0 {
			return State{}, fmt.Errorf("nothing to redo")
		}
		age.Snapshot = age.Redo[len(age.Redo)-1]
		age.Redo = age.Redo[:len(age.Redo)-1]
		age.Undo = append(age.Undo, old)
	case "put", "put-many", "delete", "apply-corrections":
		snap, err := s.snapshot(old)
		if err != nil {
			return State{}, err
		}
		switch c.Action {
		case "put-many":
			if len(c.Records) == 0 || len(c.Records) > 500 {
				return State{}, fmt.Errorf("batch must contain 1–500 records")
			}
			seen := map[string]bool{}
			for _, r := range c.Records {
				if seen[r.ID] || r.Kind == "chronology" || r.Kind == "event" {
					return State{}, fmt.Errorf("duplicate or reserved batch record")
				}
				seen[r.ID] = true
				if hash := snap.Records[r.ID]; hash != "" {
					var before Record
					if err := s.read("objects", hash, &before); err != nil {
						return State{}, err
					}
					if before.Kind != r.Kind {
						return State{}, fmt.Errorf("record kind cannot change")
					}
					if before.Kind == "schema" && before.Name != r.Name {
						return State{}, fmt.Errorf("use update-schema to rename a type and its entities together")
					}
					r = clearStaleCollaboration(r, before)
				}
				hash, err := s.put("objects", r)
				if err != nil {
					return State{}, err
				}
				snap.Records[r.ID] = hash
			}
		case "put":
			if c.Record == nil {
				return State{}, fmt.Errorf("record is required")
			}
			if c.Record.Kind == "chronology" {
				return State{}, fmt.Errorf("use configure-time to change chronology")
			}
			if c.Record.Event != nil && len(c.Record.Event.Changes) > 0 {
				var before Record
				hash := snap.Records[c.Record.ID]
				if hash == "" {
					return State{}, fmt.Errorf("use record-event to record changes")
				}
				if err := s.read("objects", hash, &before); err != nil {
					return State{}, err
				}
				a, _ := json.Marshal(before.Event.Changes)
				b, _ := json.Marshal(c.Record.Event.Changes)
				if !bytes.Equal(a, b) || before.Event.Age != c.Record.Event.Age {
					return State{}, fmt.Errorf("historical change values cannot be rewritten")
				}
			}
			if existing, ok := snap.Records[c.Record.ID]; ok {
				var before Record
				if e = s.read("objects", existing, &before); e != nil {
					return State{}, e
				}
				if before.Kind != c.Record.Kind {
					return State{}, fmt.Errorf("record kind cannot change")
				}
				if before.Kind == "schema" && before.Name != c.Record.Name {
					return State{}, fmt.Errorf("use update-schema to rename a type and its entities together")
				}
				record := clearStaleCollaboration(*c.Record, before)
				c.Record = &record
			}
			hash, err := s.put("objects", c.Record)
			if err != nil {
				return State{}, err
			}
			snap.Records[c.Record.ID] = hash
		case "delete":
			if _, ok := snap.Records[c.ID]; !ok {
				return State{}, fmt.Errorf("record not found")
			}
			delete(snap.Records, c.ID)
		case "apply-corrections":
			source, ok := root.Ages[age.SourceAge]
			if !ok || source.Snapshot != c.Incoming {
				return State{}, ErrConflict
			}
			base, err := s.snapshot(age.SourceSnapshot)
			if err != nil {
				return State{}, err
			}
			incoming, err := s.snapshot(c.Incoming)
			if err != nil {
				return State{}, err
			}
			for _, id := range c.IDs {
				if hash := incoming.Records[id]; hash != "" {
					var record Record
					if err := s.read("objects", hash, &record); err != nil {
						return State{}, err
					}
					if record.Kind == "chronology" {
						return State{}, fmt.Errorf("chronology cannot be imported as a source correction")
					}
				}
				if base.Records[id] == incoming.Records[id] {
					return State{}, fmt.Errorf("record is not a source correction")
				}
				if hash, ok := incoming.Records[id]; ok {
					snap.Records[id] = hash
				} else {
					delete(snap.Records, id)
				}
			}
		}
		next, err := s.put("snapshots", snap)
		if err != nil {
			return State{}, err
		}
		if next == old {
			return s.state(age.ID)
		}
		age.Snapshot = next
		age.Undo = append(age.Undo, old)
		age.Redo = []string{}
	default:
		return State{}, fmt.Errorf("unknown command")
	}
	root.Ages[age.ID] = age
	root.Message = commandSummary(c) + " · " + age.Name
	if _, e = s.commit(rev, root); e != nil {
		return State{}, e
	}
	return s.state(age.ID)
}

func commandSummary(c Command) string {
	switch c.Action {
	case "put":
		return "Saved " + c.Record.Name
	case "update-schema":
		return "Updated entity type " + c.Record.Name
	case "put-many":
		return fmt.Sprintf("Saved %d entries", len(c.Records))
	case "delete":
		return "Removed an entry"
	case "copy-age":
		return "Began a new Age"
	case "import-records":
		return fmt.Sprintf("Imported %d entries into a new Age", len(c.Records))
	case "configure-time":
		return "Updated Age chronology"
	case "record-event":
		return "Recorded " + c.Event.Name
	case "rename-age":
		return "Renamed Age"
	case "undo":
		return "Undid the last Age change"
	case "redo":
		return "Redid an Age change"
	case "apply-corrections":
		return "Applied selected source corrections"
	default:
		return "Saved world changes"
	}
}
func (s *Store) SnapshotRecords(id string) (map[string]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.records(id)
}
func (s *Store) Differences(from, to string) ([]Difference, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.differences(from, to, "")
}
func (s *Store) Corrections(ageID string) ([]Difference, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, root, e := s.root()
	if e != nil {
		return nil, "", e
	}
	a, ok := root.Ages[ageID]
	if !ok || a.SourceAge == "" {
		return nil, "", fmt.Errorf("this Age has no source")
	}
	source := root.Ages[a.SourceAge]
	d, e := s.differences(a.SourceSnapshot, source.Snapshot, a.Snapshot)
	return d, source.Snapshot, e
}
func (s *Store) differences(from, to, current string) ([]Difference, error) {
	a, e := s.snapshot(from)
	if e != nil {
		return nil, e
	}
	b, e := s.snapshot(to)
	if e != nil {
		return nil, e
	}
	cur := Snapshot{}
	if current != "" {
		cur, e = s.snapshot(current)
		if e != nil {
			return nil, e
		}
	}
	ids := map[string]bool{}
	for id := range a.Records {
		ids[id] = true
	}
	for id := range b.Records {
		ids[id] = true
	}
	out := []Difference{}
	read := func(hash string) *Record {
		if hash == "" {
			return nil
		}
		var r Record
		if err := s.read("objects", hash, &r); err != nil {
			e = err
			return nil
		}
		return &r
	}
	for id := range ids {
		if a.Records[id] == b.Records[id] {
			continue
		}
		d := Difference{ID: id, Change: "changed", Before: read(a.Records[id]), After: read(b.Records[id])}
		if d.Before == nil {
			d.Change = "added"
		}
		if d.After == nil {
			d.Change = "removed"
		}
		if current != "" {
			d.Current = read(cur.Records[id])
			d.Conflict = cur.Records[id] != a.Records[id] && cur.Records[id] != b.Records[id]
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, e
}
func (s *Store) AddAsset(b []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putBytes("assets", b)
}
func (s *Store) Asset(id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.asset(id)
}
func (s *Store) asset(id string) ([]byte, error) {
	if !validHash(id) {
		return nil, fmt.Errorf("invalid asset")
	}
	b, e := os.ReadFile(filepath.Join(s.Dir, "assets", id))
	if e != nil {
		return nil, e
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != id {
		return nil, fmt.Errorf("corrupt asset")
	}
	return b, nil
}
func (s *Store) Search(ageID, q string) ([]Record, error) {
	state, e := s.State(ageID)
	if e != nil {
		return nil, e
	}
	q = strings.ToLower(strings.TrimSpace(q))
	out := []Record{}
	for _, r := range state.Records {
		b, _ := json.Marshal(r)
		if strings.Contains(strings.ToLower(string(b)), q) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
