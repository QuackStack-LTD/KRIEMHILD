package project

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalDrawingIsolationHierarchyAndPublication(t *testing.T) {
	s, st := fixture(t)
	known := Record{ID: NewID(), Kind: "entity", Type: "Unit", Name: "Guard"}
	secret := Record{ID: NewID(), Kind: "entity", Type: "Unit", Name: "SECRET_GARRISON"}
	st = put(t, s, st, known)
	st = put(t, s, st, secret)
	parent := Record{ID: NewID(), Kind: "map", Name: "Province", Width: 1000, Height: 700}
	st = put(t, s, st, parent)
	d := LocalDrawing{Parent: parent.ID, Anchor: Point{.5, .5}, Span: 100, Unit: "metres", Grid: "hex", Columns: 20, Shapes: []LocalShape{{NewID(), "Hall", "room", 0, []Point{{.1, .1}, {.5, .1}, {.5, .5}}}}, Tokens: []LocalToken{{NewID(), known.ID, .2, .2, 0, 90}, {NewID(), secret.ID, .3, .3, 0, 180}}}
	child := Record{ID: NewID(), Kind: "map", Name: "Fort", Width: 1000, Height: 700, Pins: []Pin{{known.ID, .1, .1}, {secret.ID, .5, .5}}, Features: []Feature{{ID: NewID(), Name: "SECRET_APPROACH", Kind: "route", Entity: secret.ID, Points: []Point{{.1, .1}, {.2, .2}}}}, Properties: map[string]any{"_localMap": d}}
	st = put(t, s, st, child)
	source := st
	b := apply(t, s, st, Command{Action: "copy-age", Name: "Siege"})
	changed := cloneProperties(child)
	d.Tokens[0].X = .8
	changed.Properties["_localMap"] = d
	b = put(t, s, b, changed)
	original, _ := s.State(source.Age.ID)
	od, e := localDrawing(original.Records[child.ID])
	if e != nil || od.Tokens[0].X != .2 {
		t.Fatal("copied map altered original", e)
	}
	cycle := cloneProperties(parent)
	d.Parent = child.ID
	cycle.Properties["_localMap"] = d
	if _, e = s.Apply(Command{Age: b.Age.ID, Expected: b.Revision, Action: "put", Record: &cycle}); e == nil {
		t.Fatal("parent cycle accepted")
	}
	d.Parent = parent.ID
	d.Tokens[0].X = 2
	changed.Properties["_localMap"] = d
	if _, e = s.Apply(Command{Age: b.Age.ID, Expected: b.Revision, Action: "put", Record: &changed}); e == nil {
		t.Fatal("out of bounds token accepted")
	}
	edition, e := s.Export(ExportRequest{Snapshot: source.Age.Snapshot, IDs: []string{child.ID, known.ID}, Format: "publication"})
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(edition.Data, []byte("SECRET")) || bytes.Contains(edition.Data, []byte(secret.ID)) {
		t.Fatal("public map leaked unselected entity")
	}
	if !bytes.Contains(edition.Data, []byte("edition-search")) || !bytes.Contains(edition.Data, []byte("<svg")) || !bytes.Contains(edition.Data, []byte("sha256-")) {
		t.Fatal("interactive edition missing maps/search/CSP")
	}
}

func TestVerifiedObjectCacheDoesNotShareMutableStateOrHideChangedFiles(t *testing.T) {
	s, st := fixture(t)
	r := Record{ID: NewID(), Kind: "entity", Type: "Person", Name: "Original", Properties: map[string]any{"notes": "unchanged"}}
	st = put(t, s, st, r)
	first, e := s.State(st.Age.ID)
	if e != nil {
		t.Fatal(e)
	}
	value := first.Records[r.ID]
	value.Properties["notes"] = "mutated caller"
	first.Records[r.ID] = value
	second, e := s.State(st.Age.ID)
	if e != nil || second.Records[r.ID].Properties["notes"] != "unchanged" {
		t.Fatal("cache exposed mutable values", e)
	}
	snap, e := s.snapshot(st.Age.Snapshot)
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(s.Dir, "objects", snap.Records[r.ID])
	raw, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	raw = bytes.Replace(raw, []byte("Original"), []byte("Modified"), 1)
	if e = os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	future := time.Now().Add(time.Second)
	os.Chtimes(p, future, future)
	if _, e = s.State(st.Age.ID); e == nil {
		t.Fatal("cache hid externally modified object")
	}
}

func TestValidationMemoDoesNotSurviveIntoLaterHistoryChecks(t *testing.T) {
	s, st := fixture(t)
	record := Record{ID: NewID(), Kind: "entity", Type: "Place", Name: "Old place"}
	st = put(t, s, st, record)
	original, e := s.snapshot(st.Age.Snapshot)
	if e != nil {
		t.Fatal(e)
	}
	record.Name = "Current place"
	st = put(t, s, st, record)
	if s.validationObjects != nil || s.validationRecords != nil || s.validationAssets != nil {
		t.Fatal("mutable validation cache escaped the pass")
	}
	path := filepath.Join(s.Dir, "objects", original.Records[record.ID])
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	raw = bytes.Replace(raw, []byte("Old place"), []byte("Bad place"), 1)
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	future := time.Now().Add(time.Second)
	if e = os.Chtimes(path, future, future); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "rename-age", Name: "Should fail"}); e == nil {
		t.Fatal("validation memo hid corruption in an older snapshot")
	}
	unchanged, _ := s.State(st.Age.ID)
	if unchanged.Revision != st.Revision {
		t.Fatal("failed historical validation changed the head")
	}
}

func TestContextualConlangAndInflectionPreviews(t *testing.T) {
	s, st := fixture(t)
	lex := Record{ID: NewID(), Kind: "entity", Name: "Patapa", Type: "Dictionary entry", Properties: map[string]any{"_domain": "lexeme", "phonemes": "p a t a p a", "form": "patapa"}}
	st = put(t, s, st, lex)
	request := ExperimentRequest{Expected: st.Revision, Age: st.Age.ID, IDs: []string{lex.ID}, Operation: "sound-change", Values: map[string]string{"rules": "p > f / # _ V\nt > d / V _ V", "classes": "V = a e i o u"}}
	result, e := s.Experiment(request)
	if e != nil || result.Proposals[0].Properties["phonemes"] != "f a d a p a" {
		t.Fatal("context-sensitive change failed", e, result)
	}
	request.Operation = "phonotactics"
	request.Values = map[string]string{"classes": "C = p t k\nV = a e i o u", "patterns": "C V C V C V"}
	result, e = s.Experiment(request)
	if e != nil || result.Rows[0]["matching pattern"] != "C V C V C V" {
		t.Fatal("phonotactic check", e)
	}
	request.Operation = "inflection"
	request.Values = map[string]string{"affixes": "singular | |\nplural | | i"}
	result, e = s.Experiment(request)
	if e != nil || len(result.Proposals) != 1 || result.Rows[1]["result"] != "patapai" {
		t.Fatal("inflection preview", e)
	}
	unchanged, _ := s.State(st.Age.ID)
	if unchanged.Revision != st.Revision {
		t.Fatal("language preview changed canonical world")
	}
	st = apply(t, s, st, Command{Action: "put-many", Records: result.Proposals})
	raw, _ := json.Marshal(st.Records[lex.ID].Properties["inflections"])
	if !strings.Contains(string(raw), "patapai") {
		t.Fatal("accepted paradigm lost")
	}
}

func TestDiscardImportCannotRemoveOutsideStaging(t *testing.T) {
	library := t.TempDir()
	protected := filepath.Join(library, "keep.txt")
	os.WriteFile(protected, []byte("keep"), 0600)
	if e := DiscardImport(library, ImportPreview{Token: ".."}); e == nil {
		t.Fatal("unsafe token accepted")
	}
	if _, e := os.Stat(protected); e != nil {
		t.Fatal("discard touched library")
	}
	s, _ := fixture(t)
	archive, e := s.Archive()
	if e != nil {
		t.Fatal(e)
	}
	p, e := PrepareImport(library, archive)
	if e != nil {
		t.Fatal(e)
	}
	if e = DiscardImport(library, p); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(p.Directory); !os.IsNotExist(e) {
		t.Fatal("discard left imported staging data")
	}
}

func TestTextImportStagesIndependentAgeAndRejectsLossyJSON(t *testing.T) {
	s, st := fixture(t)
	original := st
	req := ContentImportRequest{Expected: st.Revision, Age: st.Age.ID, Format: "csv", Name: "cities.csv", Text: "name,type,notes,id\nRivergate,Settlement,An authored city,old-1\n"}
	preview, e := s.PreviewContent(req)
	if e != nil || len(preview.Records) != 1 {
		t.Fatal("CSV preview", e)
	}
	unchanged, _ := s.State(st.Age.ID)
	if unchanged.Revision != st.Revision {
		t.Fatal("preview changed world")
	}
	imported := apply(t, s, st, Command{Action: "import-records", Name: "Review cities", Records: preview.Records})
	if imported.Age.ID == original.Age.ID || len(imported.Root.Ages) != 2 {
		t.Fatal("import did not create independent Age")
	}
	originalNow, _ := s.State(original.Age.ID)
	if len(originalNow.Records) != len(original.Records) {
		t.Fatal("text import modified source Age")
	}
	req.Expected = imported.Revision
	req.Age = imported.Age.ID
	req.Format = "markdown"
	req.Text = "# Uninterpreted\n<script>alert('no')</script>"
	preview, e = s.PreviewContent(req)
	if e != nil || !strings.Contains(textDocument(preview.Records[0].Document), "<script>") {
		t.Fatal("Markdown text not preserved", e)
	}
	req.Format = "json"
	req.Text = `[{"name":"A","unrecognized":"would be lost"}]`
	if _, e = s.PreviewContent(req); e == nil {
		t.Fatal("silently discarded unrecognized JSON fields")
	}
	req.Format = "csv"
	req.Text = "name,type,_collaborationState\nA,Person,evil"
	if _, e = s.PreviewContent(req); e == nil {
		t.Fatal("reserved state accepted from CSV")
	}
}

func TestFamilyTraversalPreservesMultipleParentRolesAndCycles(t *testing.T) {
	s, st := fixture(t)
	a := Record{ID: NewID(), Kind: "entity", Type: "Person", Name: "Ancestor"}
	b := Record{ID: NewID(), Kind: "entity", Type: "Person", Name: "Adoptive parent"}
	c := Record{ID: NewID(), Kind: "entity", Type: "Person", Name: "Child"}
	relations := []Record{{ID: NewID(), Kind: "relation", Name: "biological parent of", From: a.ID, To: c.ID}, {ID: NewID(), Kind: "relation", Name: "adoptive parent of", From: b.ID, To: c.ID}, {ID: NewID(), Kind: "relation", Name: "divine parent of", From: c.ID, To: a.ID}}
	st = apply(t, s, st, Command{Action: "put-many", Records: append([]Record{a, b, c}, relations...)})
	graph, e := s.RelationshipGraph(GraphRequest{Snapshot: st.Age.Snapshot, Start: c.ID, Direction: "ancestors", Depth: 8})
	if e != nil || len(graph.Nodes) != 3 || len(graph.Edges) != 3 || graph.Truncated {
		t.Fatal("cyclic genealogy traversal", e, graph)
	}
	filtered, e := s.RelationshipGraph(GraphRequest{Snapshot: st.Age.Snapshot, Start: c.ID, Direction: "ancestors", Depth: 8, Roles: []string{"adoptive parent of"}})
	if e != nil || len(filtered.Nodes) != 2 || len(filtered.Edges) != 1 {
		t.Fatal("role-filtered genealogy", e)
	}
	for _, bad := range []string{"0xa", "1/2", "1e3", "NaN", "."} {
		if _, e := decimal(bad); e == nil {
			t.Fatal("nondecimal quantity accepted", bad)
		}
	}
}
