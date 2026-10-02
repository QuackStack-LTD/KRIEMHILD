package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEventWithoutBaselineAndInterruptedAtomicSave(t *testing.T) {
	for _, stage := range []string{"objects", "journal", "head"} {
		t.Run(stage, func(t *testing.T) {
			s, st := fixture(t)
			city := Record{ID: NewID(), Kind: "entity", Name: "Port", Type: "Settlement", Status: "active"}
			st = put(t, s, st, city)
			changed := city
			changed.Status = "destroyed"
			event := Record{ID: NewID(), Kind: "event", Name: "Storm", Event: &Event{Date: FictionalDate{Precision: "exact", Tick: "10"}}}
			s.fail = func(at string) error {
				if at == stage {
					return errors.New("simulated interruption")
				}
				return nil
			}
			if _, err := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "record-event", Event: &event, Records: []Record{changed}}); err == nil {
				t.Fatal("interruption not injected")
			}
			dir := s.Dir
			s.Close()
			reopened, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			actual, err := reopened.State(st.Age.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stage == "head" {
				if actual.Records[city.ID].Status != "destroyed" || actual.Records[event.ID].ID == "" {
					t.Fatal("partial compound save")
				}
				before, err := reopened.Historical(actual.Age.Snapshot, "9")
				if err != nil || len(before.Records) != 0 {
					t.Fatal("unknown opening state was invented")
				}
				after, err := reopened.Historical(actual.Age.Snapshot, "10")
				if err != nil || after.Records[city.ID].Status != "destroyed" {
					t.Fatal("explicit assertion needs no baseline date", err)
				}
			} else if actual.Revision != st.Revision {
				t.Fatal("staged event changed the committed head")
			}
		})
	}
}

func TestTerrainDeterminismPreviewAndImpacts(t *testing.T) {
	a, b := generated("coast", true), generated("coast", true)
	if !reflect.DeepEqual(a, b) || reflect.DeepEqual(a.Heights, generated("other", true).Heights) {
		t.Fatal("seed determinism")
	}
	if e := validateTerrain(a); e != nil {
		t.Fatal(e)
	}
	for i, next := range a.Flow {
		if next >= 0 && a.Heights[next] >= a.Heights[i] {
			t.Fatal("non-descending flow")
		}
	}
	for y := 0; y < a.Rows; y++ {
		if a.Heights[y*a.Columns] != a.Heights[y*a.Columns+a.Columns-1] {
			t.Fatal("wrapping seam")
		}
	}
	s, st := fixture(t)
	city := Record{ID: NewID(), Kind: "entity", Name: "Port", Type: "Settlement"}
	st = put(t, s, st, city)
	terrain := generated("dry", false)
	for i := range terrain.Heights {
		terrain.Heights[i] = 200
	}
	derive(terrain)
	m := Record{ID: NewID(), Kind: "map", Name: "Coast", Width: 1200, Height: 800, Terrain: terrain, Pins: []Pin{{city.ID, .5, .5}}}
	st = put(t, s, st, m)
	request := TerrainRequest{Expected: st.Revision, Age: st.Age.ID, Map: m.ID, Operation: "flood", X: .5, Y: .5, Radius: .3, Strength: 300}
	preview, e := s.PreviewTerrain(request)
	if e != nil {
		t.Fatal(e)
	}
	if preview.Changed == 0 || len(preview.Impacts) != 1 || preview.Impacts[0].Change != "newly submerged" {
		t.Fatalf("missing flood impact: %+v", preview.Impacts)
	}
	unchanged, _ := s.State(st.Age.ID)
	if unchanged.Revision != st.Revision || !reflect.DeepEqual(unchanged.Records[m.ID].Terrain, terrain) {
		t.Fatal("preview changed saved data")
	}
	st = put(t, s, st, preview.Record)
	if st.Records[city.ID].Status != "" {
		t.Fatal("invented settlement lifecycle")
	}
	if _, e = s.PreviewTerrain(request); e != ErrConflict {
		t.Fatal("stale preview accepted", e)
	}
	bad := *preview.Record.Terrain
	bad.Water = append([]int{}, bad.Water...)
	bad.Water[0] = 99
	if validateTerrain(&bad) == nil {
		t.Fatal("stale derived water accepted")
	}
}

func TestP2FloodHistoryBranchesScenesAndUndo(t *testing.T) {
	s, a := fixture(t)
	city := Record{ID: NewID(), Kind: "entity", Name: "Port", Type: "Settlement", Status: "active"}
	a = put(t, s, a, city)
	m := Record{ID: NewID(), Kind: "map", Name: "Coast", Width: 100, Height: 100, Terrain: generated("coast", false), Pins: []Pin{{city.ID, .5, .5}}}
	a = put(t, s, a, m)
	a = apply(t, s, a, Command{Action: "configure-time", Chronology: &Chronology{Start: "0", End: "100", Branch: "canon"}})
	source := a.Age.Snapshot
	b := apply(t, s, a, Command{Action: "copy-age", Name: "Age of Floods"})
	if chronology(b.Records).Age != b.Age.ID || chronology(b.Records).Baseline != source || chronology(b.Records).Start != "" {
		t.Fatal("new Age chronology did not reset")
	}
	b = apply(t, s, b, Command{Action: "configure-time", Chronology: &Chronology{Start: "100", End: "200", Branch: "alternate"}})
	before := b.Age.Snapshot
	proposal, e := s.PreviewTerrain(TerrainRequest{Expected: b.Revision, Age: b.Age.ID, Map: m.ID, Operation: "sea", Sea: 1500})
	if e != nil {
		t.Fatal(e)
	}
	event := Record{ID: NewID(), Kind: "event", Name: "The Flood", Notes: "The sea broke through.", Event: &Event{Date: FictionalDate{Precision: "exact", Tick: "120"}, Track: "Geography"}}
	b = apply(t, s, b, Command{Action: "record-event", Event: &event, Records: []Record{proposal.Record}})
	after := b.Age.Snapshot
	for _, check := range []struct {
		tick string
		sea  int
	}{{"119", 0}, {"120", 1500}, {"199", 1500}} {
		h, e := s.Historical(after, check.tick)
		if e != nil {
			t.Fatal(e)
		}
		if h.Records[m.ID].Terrain.Sea != check.sea {
			t.Fatalf("history at %s", check.tick)
		}
	}
	outside, e := s.Historical(after, "200")
	if e != nil || len(outside.Records) != 0 || len(outside.Unresolved) == 0 {
		t.Fatal("end is not exclusive")
	}
	original, e := s.State(a.Age.ID)
	if e != nil || original.Age.Snapshot != source {
		t.Fatal("source Age changed")
	}
	b = apply(t, s, b, Command{Action: "undo"})
	if b.Age.Snapshot != before || b.Records[event.ID].ID != "" {
		t.Fatal("event and terrain were not one undo")
	}
	b = apply(t, s, b, Command{Action: "redo"})
	if b.Age.Snapshot != after {
		t.Fatal("redo lost compound change")
	}
	scene := simpleScene(b)
	scene.SettingDate = "119"
	scene.References = []string{city.ID}
	b = put(t, s, b, scene)
	// An undated edit changes the overview, never the dated map or pinned scene.
	changed := b.Records[m.ID]
	changed.Name = "Overview correction"
	b = put(t, s, b, changed)
	pinned, e := s.Historical(scene.SettingSnapshot, scene.SettingDate)
	if e != nil || pinned.Records[m.ID].Name != "Coast" || pinned.Records[m.ID].Terrain.Sea != 0 {
		t.Fatal("scene pin drifted")
	}
	dir := s.Dir
	s.Close()
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	historical, e := reopened.Historical(after, "120")
	if e != nil || historical.Records[m.ID].Terrain.Sea != 1500 {
		t.Fatal("dated state lost after restart")
	}
}

func TestTemporalValidationAndLargeTicks(t *testing.T) {
	s, st := fixture(t)
	city := Record{ID: NewID(), Kind: "entity", Name: "Old city", Type: "Settlement"}
	st = put(t, s, st, city)
	st = apply(t, s, st, Command{Action: "configure-time", Chronology: &Chronology{Start: "-999999999999999999999999999999", Branch: "canon"}})
	updated := city
	updated.Name = "New city"
	ev := Record{ID: NewID(), Kind: "event", Name: "Renaming", Event: &Event{Date: FictionalDate{Precision: "exact", Tick: "9007199254740993123456"}}}
	st = apply(t, s, st, Command{Action: "record-event", Records: []Record{updated}, Event: &ev})
	earlier, _ := s.Historical(st.Age.Snapshot, "9007199254740993123455")
	later, _ := s.Historical(st.Age.Snapshot, "9007199254740993123456")
	if earlier.Records[city.ID].Name != "Old city" || later.Records[city.ID].Name != "New city" {
		t.Fatal("large date precision lost")
	}
	for _, bad := range []string{"01", "1.5", "+2", "-0", strings.Repeat("1", 101)} {
		if _, e := tick(bad); e == nil {
			t.Fatalf("bad tick accepted: %s", bad)
		}
	}
	for _, until := range []string{"wrong", "0"} {
		bad := ev
		bad.ID = NewID()
		copy := *ev.Event
		bad.Event = &copy
		bad.Event.Until = until
		_, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "record-event", Event: &bad, Records: []Record{city}})
		if e == nil {
			t.Fatal("bad interval accepted")
		}
	}
	uncertain := Record{ID: NewID(), Kind: "event", Name: "Uncertain", Event: &Event{Date: FictionalDate{Precision: "approximate", Tick: "10"}}}
	if _, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "record-event", Event: &uncertain, Records: []Record{city}}); e == nil {
		t.Fatal("uncertainty invented state")
	}
	st = apply(t, s, st, Command{Action: "record-event", Event: &uncertain})
	h, _ := s.Historical(st.Age.Snapshot, "10")
	if len(h.Unresolved) == 0 {
		t.Fatal("uncertainty not disclosed")
	}
	cyc := st.Records[uncertain.ID]
	cyc.Event.Causes = []string{cyc.ID}
	if _, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "put", Record: &cyc}); e == nil {
		t.Fatal("causal cycle allowed")
	}
	newer := Record{ID: NewID(), Kind: "event", Name: "Second renaming", Event: &Event{Date: FictionalDate{Precision: "exact", Tick: "9007199254740993123457"}}}
	st = apply(t, s, st, Command{Action: "record-event", Event: &newer, Records: []Record{city}})
	if st.Records[ev.ID].Event.Until != "9007199254740993123457" {
		t.Fatal("earlier open assertion not closed")
	}
	h, _ = s.Historical(st.Age.Snapshot, "9007199254740993123456")
	if h.Records[city.ID].Name != "New city" {
		t.Fatal("successor changed earlier validity")
	}
}

func TestCalendarNegativeDatesLeapAndSnapshotPin(t *testing.T) {
	c := Calendar{Epoch: "0", Era: "Reckoning", Months: []Month{{"Sun", 2}, {"Moon", 2}}, Week: 3, LeapEvery: 2, LeapDays: 1}
	if e := validateCalendar(&c); e != nil {
		t.Fatal(e)
	}
	for at, want := range map[string]string{"0": "1 Sun, day 1", "3": "1 Moon, day 2", "4": "2 Sun, day 1", "8": "2 Moon, day 3", "9": "3 Sun, day 1", "-1": "-1 Moon, day 3", "-9": "-2 Sun, day 1"} {
		if got := calendarLabel(&c, at); !strings.HasPrefix(got, want) {
			t.Errorf("%s: got %s want %s", at, got, want)
		}
	}
	s, st := fixture(t)
	cal := Record{ID: NewID(), Kind: "calendar", Name: "Reckoning", Calendar: &c}
	st = put(t, s, st, cal)
	st = apply(t, s, st, Command{Action: "configure-time", Chronology: &Chronology{Start: "-100", Branch: "canon", Calendar: cal.ID}})
	pinned := st.Age.Snapshot
	c.Era = "Changed"
	st = put(t, s, st, cal)
	old, e := s.Historical(pinned, "8")
	if e != nil || !strings.Contains(old.Label, "Reckoning") {
		t.Fatal("calendar pin changed")
	}
	current, _ := s.Historical(st.Age.Snapshot, "8")
	if !strings.Contains(current.Label, "Changed") {
		t.Fatal("calendar edit missing")
	}
}

func TestFormatOneIsRejectedWithoutChangingHead(t *testing.T) {
	s, st := fixture(t)
	root := st.Root
	root.Version = 1
	hash, e := s.put("revisions", root)
	if e != nil {
		t.Fatal(e)
	}
	head, _ := json.Marshal(Head{hash})
	if e = atomicWrite(filepath.Join(s.Dir, "project.head.json"), head); e != nil {
		t.Fatal(e)
	}
	s.Close()
	if opened, e := Open(s.Dir); e == nil {
		opened.Close()
		t.Fatal("P1 was silently upgraded")
	}
	actual, _ := os.ReadFile(filepath.Join(s.Dir, "project.head.json"))
	if string(actual) != string(head) {
		t.Fatal("P1 head changed")
	}
}
