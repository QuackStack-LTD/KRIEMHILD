package history

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func archiveFixture(t *testing.T) (*Document, GatherRequest) {
	t.Helper()
	d := New("Knowledge boundaries")
	age := d.World.CurrentAge
	for _, x := range []struct{ id, kind, props string }{{"owner", "country", `{"startYear":0,"population":500}`}, {"town", "city", `{"startYear":12,"population":40,"biography":"private affairs"}`}, {"distant", "city", `{"startYear":14}`}, {"subject", "character", `{"startYear":5,"endYear":18}`}, {"visitor", "character", `{"startYear":9}`}, {"future", "city", `{"startYear":90}`}} {
		e := Entity{ID: x.id, Name: x.id, Kind: x.kind, Properties: json.RawMessage(x.props)}
		if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	e := Entity{ID: "archive", Name: "Royal archive", Kind: "archive", Archive: &ArchiveConfig{Version: 1, OwnerID: "owner", Purpose: "general", Intensity: "exhaustive", Categories: append([]string(nil), ArchiveCategories[:7]...), KnowledgeScope: "transmitted", Retention: "preserve"}}
	if err := d.Apply(Command{Kind: "entity", AgeID: age, Entity: &e}); err != nil {
		t.Fatal(err)
	}
	d.Relations = []Relation{{ID: "official", AgeID: age, Source: "owner", Destination: "town", Kind: "governs", ValidFrom: "10"}, {ID: "second-hop", AgeID: age, Source: "town", Destination: "distant", Kind: "controls", ValidFrom: "10"}, {ID: "late", AgeID: age, Source: "subject", Destination: "owner", Kind: "subordinate-to", ValidFrom: "10"}, {ID: "visit", AgeID: age, Source: "visitor", Destination: "owner", Kind: "resides-in", ValidFrom: "0"}, {ID: "future-control", AgeID: age, Source: "owner", Destination: "future", Kind: "governs", ValidFrom: "0"}}
	return d, GatherRequest{AgeID: age, ArchiveID: "archive", Scope: "age", SourceAge: age, RecordingYear: 20}
}
func TestArchiveKnowledgeIsSelectiveDatedAndDeterministic(t *testing.T) {
	d, r := archiveFixture(t)
	out, result, err := d.GatherDocuments(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("empty eligible archive")
	}
	for _, g := range out {
		if strings.Contains(g.Record.SourceIDs[0], "distant") || strings.Contains(g.Record.SourceIDs[0], "visitor") || strings.Contains(g.Record.SourceIDs[0], "future") || strings.Contains(g.Record.SourceIDs[0], "subject/start") || strings.Contains(g.Record.SourceIDs[0], "town/biography") {
			t.Fatalf("knowledge leak: %+v", g.Record)
		}
	}
	foundTown, foundOwner := false, false
	for _, g := range out {
		foundTown = foundTown || g.Record.SourceIDs[0] == "event/town/start"
		foundOwner = foundOwner || g.Record.SourceIDs[0] == "event/owner/start"
	}
	if !foundTown || !foundOwner {
		t.Fatal("direct and institutional events missing")
	}
	again, result2, err := d.GatherDocuments(r)
	if err != nil || !reflect.DeepEqual(out, again) || !reflect.DeepEqual(result, result2) {
		t.Fatal("nondeterministic gathering")
	}
	d.Entity(r.AgeID, "archive").Archive.Intensity = "none"
	out, _, err = d.GatherDocuments(r)
	if err != nil || len(out) != 0 {
		t.Fatal("None recorded documents")
	}
}
func TestArchiveDelayedReportAndNoIncidentalKnowledge(t *testing.T) {
	d, r := archiveFixture(t)
	c := d.Entity(r.AgeID, "archive").Archive
	c.Sources = []KnowledgeSource{{ID: "letter", EntityID: "distant", SourceAge: r.AgeID, ArrivalAge: r.AgeID, Channel: "messenger", Evidence: "Envoy's sealed account", Strength: "established", FromYear: 14, ToYear: 14, ArrivalYear: 30, Categories: []string{"chronicles"}}}
	for _, n := range []int64{29, 30} {
		r.RecordingYear = n
		out, _, err := d.GatherDocuments(r)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, g := range out {
			if g.Record.SourceIDs[0] == "event/distant/start" {
				found = true
				if g.Record.FromYear != 14 || g.Record.RecordingYear != 30 || g.Record.Artifact != "report" {
					t.Fatal("report backdated or missing context")
				}
			}
		}
		if found != (n == 30) {
			t.Fatal("incorrect arrival time")
		}
	}
	c.Sources[0].Strength = "incidental"
	out, _, _ := d.GatherDocuments(r)
	for _, g := range out {
		if g.Record.SourceIDs[0] == "event/distant/start" {
			t.Fatal("incidental edge granted knowledge")
		}
	}
	c.Sources[0].ArrivalYear = 12
	if _, _, err := d.GatherDocuments(r); err == nil {
		t.Fatal("accepted message before event")
	}
}
func TestArchiveInheritedSourcesStayIndependentAndRespectYearBranch(t *testing.T) {
	d, r := archiveFixture(t)
	c := d.Entity(r.AgeID, "archive").Archive
	c.Sources = []KnowledgeSource{{ID: "letter", EntityID: "distant", SourceAge: r.AgeID, ArrivalAge: r.AgeID, Channel: "messenger", Evidence: "Received report", Strength: "established", FromYear: 14, ToYear: 14, ArrivalYear: 30, Categories: []string{"chronicles"}}}
	if err := d.Apply(Command{Kind: "age", AgeID: r.AgeID, Mode: "branch", Name: "Successor"}); err != nil {
		t.Fatal(err)
	}
	next := d.World.CurrentAge
	copy := d.Entity(next, "archive").Archive
	if copy == c || &copy.Sources[0] == &c.Sources[0] {
		t.Fatal("shared mutable archive configuration")
	}
	if !d.sourceHasArrived(copy.Sources[0], next, 1) {
		t.Fatal("inherited report lost because Age clocks differ")
	}
	d.Age(next).StartedFromYear = &YearOrigin{AgeID: r.AgeID, Number: 20}
	if d.sourceHasArrived(copy.Sources[0], next, 100) {
		t.Fatal("future message inherited by earlier branch")
	}
	copy.Sources[0].Evidence = "Changed"
	if c.Sources[0].Evidence == "Changed" {
		t.Fatal("source Age changed")
	}
}
func TestArchiveFactsFormatsCoverageAndCSVInjection(t *testing.T) {
	d, r := archiveFixture(t)
	r.RecordingYear = 100
	c := d.Entity(r.AgeID, "archive").Archive
	out, _, err := d.GatherDocuments(r)
	if err != nil {
		t.Fatal(err)
	}
	csvFound := false
	for _, g := range out {
		if g.MIME == "text/csv" {
			csvFound = true
		}
		c.Coverage = append(c.Coverage, ArchiveCoverage{Key: g.Record.CoverageKey, DocumentID: g.Entity.ID, SourceAge: r.AgeID})
	}
	if !csvFound {
		t.Fatal("structured observations weren't registers")
	}
	r.RecordingYear = 101
	out, _, err = d.GatherDocuments(r)
	if err != nil || len(out) != 0 {
		t.Fatal("unchanged snapshots duplicated in a later year", err)
	}
	data, _, _ := renderArchiveDocument(archiveFact{artifact: "register", rows: [][]string{{"=HYPERLINK(\"bad\")"}}}, ArchiveRecord{})
	if !strings.Contains(string(data), "'=HYPERLINK") {
		t.Fatal("spreadsheet formula injection")
	}
	svg := surveySVG(`<script>alert(1)</script>`, "polygon", [][2]float64{{0, 0}, {5, 0}, {2, 5}})
	if strings.Contains(svg, "<script>") || !strings.Contains(svg, "polygon") {
		t.Fatal("unsafe or missing geographic diagram")
	}
}
func TestArchiveYearBranchDoesNotInheritFutureReports(t *testing.T) {
	d, r := archiveFixture(t)
	rec := &ArchiveRecord{Version: 1, Category: "chronicles", SourceIDs: []string{"event/distant/start"}, AuthorID: "owner", SourceAge: r.AgeID, RecordingAge: r.AgeID, RecordingYear: 30, CreationYear: 30, CoverageKey: archiveKey("future"), FromYear: 14, ToYear: 14}
	e := Entity{ID: "report", Name: "Later report", Kind: "document"}
	if err := d.Apply(Command{Kind: "entity", AgeID: r.AgeID, Entity: &e}); err != nil {
		t.Fatal(err)
	}
	d.LibraryFiles = []LibraryFile{{ID: "report", AgeID: r.AgeID, ArchiveID: "archive", Record: rec, Versions: []LibraryVersion{{Hash: archiveKey("bytes"), Name: "report.md", Size: 5, Provenance: rec}}}}
	d.Entity(r.AgeID, "archive").Archive.Coverage = []ArchiveCoverage{{Key: rec.CoverageKey, DocumentID: "report", SourceAge: r.AgeID}}
	var yearID string
	for _, y := range d.Years {
		if y.Number == 18 {
			yearID = y.ID
		}
	}
	if err := d.Apply(Command{Kind: "age", Mode: "branch", AgeID: r.AgeID, Name: "Earlier branch", SourceYearID: yearID}); err != nil {
		t.Fatal(err)
	}
	next := d.World.CurrentAge
	if d.LibraryFile(next, "report") != nil || len(d.Entity(next, "archive").Archive.Coverage) > 0 {
		t.Fatal("future report or coverage leaked into earlier branch")
	}
	if d.LibraryFile(r.AgeID, "report") == nil || len(d.Entity(r.AgeID, "archive").Archive.Coverage) != 1 {
		t.Fatal("pruning changed source Age")
	}
}
