package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCrashProcess(t *testing.T) {
	if dir := os.Getenv("KRIEMHILD_CRASH_PROJECT"); dir != "" {
		s, e := Open(dir)
		if e != nil {
			os.Exit(91)
		}
		st, e := s.State("")
		if e != nil {
			os.Exit(92)
		}
		s.fail = func(stage string) error {
			if stage == os.Getenv("KRIEMHILD_CRASH_STAGE") {
				os.Exit(90)
			}
			return nil
		}
		r := Record{ID: NewID(), Kind: "note", Name: "Crash test record"}
		s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "put", Record: &r})
		os.Exit(93)
	}
	for _, stage := range []string{"objects", "journal", "head"} {
		t.Run(stage, func(t *testing.T) {
			s, st := fixture(t)
			s.Close()
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashProcess$")
			cmd.Env = append(os.Environ(), "KRIEMHILD_CRASH_PROJECT="+s.Dir, "KRIEMHILD_CRASH_STAGE="+stage)
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 90 {
				t.Fatalf("crash helper: %v", err)
			}
			reopened, e := Open(s.Dir)
			if e != nil {
				t.Fatal(e)
			}
			defer reopened.Close()
			actual, e := reopened.State(st.Age.ID)
			if e != nil {
				t.Fatal(e)
			}
			want := 0
			if stage == "head" {
				want = 1
			}
			if len(actual.Records) != want {
				t.Fatalf("crash at %s left %d records", stage, len(actual.Records))
			}
		})
	}
}

func fixture(t *testing.T) (*Store, State) {
	t.Helper()
	s, age, e := Create(filepath.Join(t.TempDir(), NewID()), "Test world", "Age of Rivers")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	st, e := s.State(age)
	if e != nil {
		t.Fatal(e)
	}
	return s, st
}
func apply(t *testing.T, s *Store, st State, c Command) State {
	t.Helper()
	c.Age = st.Age.ID
	c.Expected = st.Revision
	next, e := s.Apply(c)
	if e != nil {
		t.Fatal(e)
	}
	return next
}
func put(t *testing.T, s *Store, st State, r Record) State {
	t.Helper()
	return apply(t, s, st, Command{Action: "put", Record: &r})
}
func simpleScene(st State) Record {
	return Record{ID: NewID(), Kind: "scene", Name: "Arrival", Story: "River stories", Document: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"The city waited."}]}]}`), SettingAge: st.Age.ID, SettingSnapshot: st.Age.Snapshot}
}

func TestFullAgeIsolationAndRestart(t *testing.T) {
	s, a := fixture(t)
	schema := Record{ID: NewID(), Kind: "schema", Name: "Settlement", Fields: []Field{{"population", "number"}}}
	a = put(t, s, a, schema)
	city := Record{ID: NewID(), Kind: "entity", Name: "Valer", Type: "Settlement", Properties: map[string]any{"population": float64(1200)}}
	a = put(t, s, a, city)
	person := Record{ID: NewID(), Kind: "entity", Name: "Mira", Type: "Person"}
	a = put(t, s, a, person)
	relation := Record{ID: NewID(), Kind: "relation", Name: "lives in", From: person.ID, To: city.ID}
	a = put(t, s, a, relation)
	asset, e := s.AddAsset([]byte("test image bytes"))
	if e != nil {
		t.Fatal(e)
	}
	m := Record{ID: NewID(), Kind: "map", Name: "Coast", Asset: asset, Width: 100, Height: 100, Pins: []Pin{{city.ID, .3, .4}}}
	a = put(t, s, a, m)
	note := Record{ID: NewID(), Kind: "note", Name: "Research", Notes: "A source"}
	a = put(t, s, a, note)
	scene := simpleScene(a)
	scene.References = []string{city.ID}
	a = put(t, s, a, scene)
	original := a
	b := apply(t, s, a, Command{Action: "copy-age", Name: "Age of Ash"})
	if b.Age.Snapshot != a.Age.Snapshot || !reflect.DeepEqual(a.Records, b.Records) {
		t.Fatal("copy is not complete")
	}
	if b.Records[scene.ID].SettingAge != a.Age.ID {
		t.Fatal("copied scene was silently retargeted")
	}
	city.Name = "Velar"
	city.Status = "destroyed"
	city.Properties["population"] = float64(0)
	b = put(t, s, b, city)
	person.Notes = "Departed"
	b = put(t, s, b, person)
	relation.Name = "once lived in"
	b = put(t, s, b, relation)
	m.Pins[0].X = .8
	b = put(t, s, b, m)
	note.Notes = "Changed source"
	b = put(t, s, b, note)
	schema.Fields = append(schema.Fields, Field{"fate", "text"})
	b = put(t, s, b, schema)
	scene.Name = "Ruins"
	scene.Document = json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Nothing remained."}]}]}`)
	b = put(t, s, b, scene)
	unchanged, e := s.State(a.Age.ID)
	if e != nil {
		t.Fatal(e)
	}
	if unchanged.Age.Snapshot != original.Age.Snapshot || !reflect.DeepEqual(unchanged.Records, original.Records) {
		t.Fatal("editing B changed A")
	}
	differences, e := s.Differences(a.Age.Snapshot, b.Age.Snapshot)
	if e != nil || len(differences) != 7 {
		t.Fatalf("diff = %d, %v", len(differences), e)
	}
	// Editing the source later cannot alter the descendant.
	corrected := unchanged.Records[city.ID]
	corrected.Notes = "Spelling note"
	unchanged = put(t, s, unchanged, corrected)
	stillB, e := s.State(b.Age.ID)
	if e != nil || stillB.Age.Snapshot != b.Age.Snapshot {
		t.Fatal("source edit leaked into B")
	}
	dir := s.Dir
	s.Close()
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	readA, e := reopened.State(a.Age.ID)
	if e != nil || readA.Age.Snapshot != unchanged.Age.Snapshot {
		t.Fatal("A did not survive restart")
	}
	readB, e := reopened.State(b.Age.ID)
	if e != nil || !reflect.DeepEqual(readB.Records, b.Records) {
		t.Fatal("B did not survive restart")
	}
	image, e := reopened.Asset(asset)
	if e != nil || !bytes.Equal(image, []byte("test image bytes")) {
		t.Fatal("asset did not survive")
	}
}

func TestUndoRedoConflictAndReferences(t *testing.T) {
	s, st := fixture(t)
	city := Record{ID: NewID(), Kind: "entity", Name: "Port", Type: "Settlement"}
	st = put(t, s, st, city)
	before := st
	city.Name = "Ruins"
	st = put(t, s, st, city)
	if _, e := s.Apply(Command{Expected: before.Revision, Age: st.Age.ID, Action: "delete", ID: city.ID}); !errors.Is(e, ErrConflict) {
		t.Fatal("stale writer accepted")
	}
	st = apply(t, s, st, Command{Action: "undo"})
	if st.Records[city.ID].Name != "Port" {
		t.Fatal("undo failed")
	}
	st = apply(t, s, st, Command{Action: "redo"})
	if st.Records[city.ID].Name != "Ruins" {
		t.Fatal("redo failed")
	}
	m := Record{ID: NewID(), Kind: "map", Name: "Map", Width: 100, Height: 100, Pins: []Pin{{city.ID, .5, .5}}}
	st = put(t, s, st, m)
	if _, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "delete", ID: city.ID}); e == nil {
		t.Fatal("dangling map reference allowed")
	}
	after, _ := s.State(st.Age.ID)
	if after.Revision != st.Revision {
		t.Fatal("invalid write modified root")
	}
	invalid := city
	invalid.Kind = "note"
	if _, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "put", Record: &invalid}); e == nil {
		t.Fatal("kind change allowed")
	}
}

func TestSingleWriterAndCorruption(t *testing.T) {
	s, st := fixture(t)
	if another, e := Open(s.Dir); e == nil {
		another.Close()
		t.Fatal("second writer accepted")
	}
	city := Record{ID: NewID(), Kind: "entity", Name: "Port", Type: "Settlement"}
	st = put(t, s, st, city)
	snap, _ := s.snapshot(st.Age.Snapshot)
	s.Close()
	os.WriteFile(filepath.Join(s.Dir, "objects", snap.Records[city.ID]), []byte("corrupt"), 0600)
	if reopened, e := Open(s.Dir); e == nil {
		reopened.Close()
		t.Fatal("corrupt object accepted")
	}
}

func TestInterruptedCommitRecovery(t *testing.T) {
	for _, stage := range []string{"objects", "journal", "head"} {
		t.Run(stage, func(t *testing.T) {
			s, st := fixture(t)
			s.fail = func(at string) error {
				if at == stage {
					return errors.New("simulated power loss")
				}
				return nil
			}
			r := Record{ID: NewID(), Kind: "entity", Name: "Survivor", Type: "Person"}
			_, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "put", Record: &r})
			if e == nil {
				t.Fatal("injection failed")
			}
			s.Close()
			reopened, e := Open(s.Dir)
			if e != nil {
				t.Fatal(e)
			}
			defer reopened.Close()
			actual, e := reopened.State(st.Age.ID)
			if e != nil {
				t.Fatal(e)
			}
			_, exists := actual.Records[r.ID]
			if exists != (stage == "head") {
				t.Fatalf("unexpected recovered state at %s", stage)
			}
		})
	}
}
func TestDamagedHeadDuringCommitRecoversPrevious(t *testing.T) {
	s, st := fixture(t)
	s.fail = func(at string) error {
		if at == "journal" {
			return errors.New("interrupted")
		}
		return nil
	}
	r := Record{ID: NewID(), Kind: "note", Name: "Draft"}
	s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "put", Record: &r})
	os.WriteFile(filepath.Join(s.Dir, "project.head.json"), []byte("partial"), 0600)
	s.Close()
	reopened, e := Open(s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	actual, e := reopened.State(st.Age.ID)
	if e != nil || !actual.Recovered || actual.Revision != st.Revision {
		t.Fatal("previous complete root was not recovered")
	}
}

func TestCorrectionReviewAndExplicitApply(t *testing.T) {
	s, a := fixture(t)
	r := Record{ID: NewID(), Kind: "entity", Name: "City", Type: "Settlement"}
	a = put(t, s, a, r)
	b := apply(t, s, a, Command{Action: "copy-age", Name: "Later"})
	r.Name = "Later name"
	b = put(t, s, b, r)
	a, _ = s.State(a.Age.ID)
	r.Name = "Source correction"
	a = put(t, s, a, r)
	changes, incoming, e := s.Corrections(b.Age.ID)
	if e != nil || len(changes) != 1 || !changes[0].Conflict {
		t.Fatal("three-way conflict was not identified")
	}
	b, _ = s.State(b.Age.ID)
	b = apply(t, s, b, Command{Action: "apply-corrections", IDs: []string{r.ID}, Incoming: incoming})
	if b.Records[r.ID].Name != "Source correction" {
		t.Fatal("correction failed")
	}
	b = apply(t, s, b, Command{Action: "undo"})
	if b.Records[r.ID].Name != "Later name" {
		t.Fatal("correction was not undoable")
	}
}

func TestSchemaValidationSearchAndPinnedScene(t *testing.T) {
	s, st := fixture(t)
	schema := Record{ID: NewID(), Kind: "schema", Name: "City", Fields: []Field{{"population", "number"}}}
	st = put(t, s, st, schema)
	r := Record{ID: NewID(), Kind: "entity", Name: "Port", Type: "City", Properties: map[string]any{"population": "many"}}
	if _, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "put", Record: &r}); e == nil {
		t.Fatal("wrong property type accepted")
	}
	r.Properties["population"] = float64(0)
	st = put(t, s, st, r)
	scene := simpleScene(st)
	scene.References = []string{r.ID}
	st = put(t, s, st, scene)
	st = apply(t, s, st, Command{Action: "delete", ID: r.ID})
	if st.Records[scene.ID].References[0] != r.ID {
		t.Fatal("pinned reference removed")
	}
	results, e := s.Search(st.Age.ID, "waited")
	if e != nil || len(results) != 1 {
		t.Fatal("manuscript search failed")
	}
}

func TestRedoSurvivesRestart(t *testing.T) {
	s, st := fixture(t)
	r := Record{ID: NewID(), Kind: "note", Name: "Keep me"}
	st = put(t, s, st, r)
	st = apply(t, s, st, Command{Action: "undo"})
	s.Close()
	s2, e := Open(s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s2.Close()
	st, e = s2.State(st.Age.ID)
	if e != nil {
		t.Fatal(e)
	}
	st = apply(t, s2, st, Command{Action: "redo"})
	if st.Records[r.ID].Name != "Keep me" {
		t.Fatal("redo lost on restart")
	}
}
