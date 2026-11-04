package storage

import (
	"context"
	"kriemhild/internal/history"
	"kriemhild/internal/languages"
	"kriemhild/internal/languages/spike"
	"os"
	"testing"
)

func TestProductiveMixedRevisionsPinnedScriptRestartAndArchive(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	doc := history.New("Disposable productive portability")
	if err = db.CreateHistory(ctx, doc); err != nil {
		t.Fatal(err)
	}
	world, age := doc.World.ID, doc.World.CurrentAge
	raw, err := os.ReadFile("../languages/testdata/productive.json")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := languages.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	old := languages.Clone(definition)
	old.Schema = 1
	old.Productive = nil
	old.Lexicon = nil
	doc, err = db.SaveLinguisticDefinition(ctx, world, age, "language", doc.World.Revision, languages.Canonical(old), languages.State{}, "Retain schema 1")
	if err != nil {
		t.Fatal(err)
	}
	oldHash := languages.Hash(old)
	raw, err = os.ReadFile("../languages/testdata/script.json")
	if err != nil {
		t.Fatal(err)
	}
	script, err := languages.DecodeScript(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc, err = db.SaveLinguisticDefinition(ctx, world, age, "script", doc.World.Revision, languages.Canonical(script), languages.State{}, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile("../languages/testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	reference, err := languages.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	definition.Orthography = reference.Orthography
	doc, err = db.SaveLinguisticDefinition(ctx, world, age, "language", doc.World.Revision, languages.Canonical(definition), languages.State{}, "Explicit schema-2 revision")
	if err != nil {
		t.Fatal(err)
	}
	head := languages.Hash(definition)
	state := doc.Entity(age, definition.LanguageID).Language
	if len(state.Revisions) != 2 || state.Revisions[0].Schema != 1 || state.Revisions[1].Schema != 2 || state.Active != "" {
		t.Fatal("schema transition migrated/activated old source", state)
	}
	deps, err := db.LanguageDependencies(ctx, definition)
	if err != nil {
		t.Fatal(err)
	}
	artifact := languages.Compile(ctx, definition, deps)
	doc, err = db.RecordLanguageCompile(ctx, world, age, definition.LanguageID, doc.World.Revision, artifact.Report())
	if err != nil {
		t.Fatal(err)
	}
	doc, err = db.ActivateLanguage(ctx, world, age, definition.LanguageID, head, doc.World.Revision)
	if err != nil {
		t.Fatal(err)
	}
	script.Name = "Changed current script draft"
	doc, err = db.SaveLinguisticDefinition(ctx, world, age, "script", doc.World.Revision, languages.Canonical(script), languages.State{}, "")
	if err != nil {
		t.Fatal(err)
	}
	doc, err = db.UpdateHistory(ctx, world, doc.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", Mode: "branch", AgeID: age, Name: "Productive successor"})
	})
	if err != nil {
		t.Fatal(err)
	}
	nextAge := doc.World.CurrentAge
	if doc.Entity(nextAge, definition.LanguageID).Language.Draft != head || doc.Entity(nextAge, definition.LanguageID).Language.Compiled[0].Compiler != languages.ProductiveCompiler {
		t.Fatal("Age inheritance lost schema-2 compiler/state")
	}
	nextDefinition := languages.Clone(definition)
	nextDefinition.Notes = "Only the successor Age draft changes"
	doc, err = db.SaveLinguisticDefinition(ctx, world, nextAge, "language", doc.World.Revision, languages.Canonical(nextDefinition), languages.State{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Entity(age, definition.LanguageID).Language.Draft != head || len(doc.Entity(age, definition.LanguageID).Language.Revisions) != 2 {
		t.Fatal("saving successor changed source Age")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	q := languages.Request{Request: spike.Request{Revision: head, Engine: languages.ProductiveEngine, Operation: "inflect", Lexeme: "stone", Features: languages.Features{"number": "many", "shape": "framed"}, Policy: "all", Limit: 64}, Adapter: languages.ProductiveAdapter, Compiler: languages.ProductiveCompiler}
	expected := string(languages.Canonical(artifact.Execute(ctx, q)))
	blob, err := db.LinguisticDefinition(ctx, world, age, definition.LanguageID, head)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := languages.Decode(blob)
	if err != nil {
		t.Fatal(err)
	}
	deps, err = db.LanguageDependencies(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	if string(languages.Canonical(languages.Compile(ctx, restored, deps).Execute(ctx, q))) != expected {
		t.Fatal("restart or script draft changed productive replay")
	}
	bundle, err := db.HistoryBundle(ctx, world)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Schema != 3 || bundle.Validate() != nil {
		t.Fatal("schema-3 portable bundle invalid", bundle.Schema)
	}
	for _, schema := range []int{1, 2} {
		bad := languages.Clone(bundle)
		bad.Schema = schema
		if bad.Validate() == nil {
			t.Fatal("old archive schema accepted productive content")
		}
	}
	bad := languages.Clone(bundle)
	for i := range bad.Document.Entities {
		e := &bad.Document.Entities[i]
		if e.Language != nil {
			e.Language.Revisions[1].Schema = 1
		}
	}
	if bad.Validate() == nil {
		t.Fatal("reference/body schema mismatch accepted")
	}
	copyDB, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	if err = copyDB.ImportHistory(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	blob, err = copyDB.LinguisticDefinition(ctx, world, age, definition.LanguageID, oldHash)
	if err != nil || string(blob) != string(languages.Canonical(old)) {
		t.Fatal("import changed schema-1 revision", err)
	}
	blob, err = copyDB.LinguisticDefinition(ctx, world, age, definition.LanguageID, head)
	if err != nil {
		t.Fatal(err)
	}
	restored, err = languages.Decode(blob)
	if err != nil {
		t.Fatal(err)
	}
	deps, err = copyDB.LanguageDependencies(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	if string(languages.Canonical(languages.Compile(ctx, restored, deps).Execute(ctx, q))) != expected {
		t.Fatal("import changed productive replay")
	}
	delete(bundle.Blobs, definition.Orthography.Script.Revision)
	if bundle.Validate() == nil {
		t.Fatal("missing pinned script accepted")
	}
}

func TestProductivePinnedBodyRequiresArchiveThreeWithoutCurrentAgeRecord(t *testing.T) {
	raw, err := os.ReadFile("../languages/testdata/productive.json")
	if err != nil {
		t.Fatal(err)
	}
	source, err := languages.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc := history.New("Synthetic retained dependency")
	target := languages.Definition{Schema: 1, LanguageID: history.ID(), Name: "Retained provenance", Lexical: &languages.LexicalModel{Sources: []languages.LexicalSource{{ID: "authored-source", Kind: "cognate", LanguageID: source.LanguageID, Revision: languages.Hash(source), AgeID: history.ID()}}}}
	ref := languages.RevisionRef{Hash: languages.Hash(target), Schema: 1, Canonicalizer: languages.CanonicalVersion, Size: len(languages.Canonical(target))}
	doc.Entities = append(doc.Entities, history.Entity{ID: target.LanguageID, AgeID: doc.World.CurrentAge, Kind: "language", Name: target.Name, Language: &languages.State{Draft: ref.Hash, Revisions: []languages.RevisionRef{ref}}})
	blobs := map[string][]byte{ref.Hash: languages.Canonical(target), languages.Hash(source): languages.Canonical(source)}
	if hasProductiveRevisions(doc) {
		t.Fatal("fixture unexpectedly retained a visible schema-2 reference")
	}
	if validateLinguisticBundle(doc, blobs, 2) == nil {
		t.Fatal("old archive schema accepted a pinned productive body")
	}
	if err = validateLinguisticBundle(doc, blobs, 3); err != nil {
		t.Fatal("schema 3 lost retained source after Age removal", err)
	}
}

func TestLanguageRevisionsAgesRestartAndPortability(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	d := history.New("Linguistic world")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	world, first, id := d.World.ID, d.World.CurrentAge, history.ID()
	definition := languages.Definition{Schema: 1, LanguageID: id, Name: "Authored language", Notes: "undecided modules"}
	d, err = db.SaveLinguisticDefinition(ctx, world, first, "language", d.World.Revision, languages.Canonical(definition), languages.State{Status: "living"}, "First draft")
	if err != nil {
		t.Fatal(err)
	}
	initial := d.Entity(first, id).Language.Draft
	if d.Entity(first, id).Language.Active != "" || len(d.Entity(first, id).Language.Compiled) != 0 {
		t.Fatal("save compiled or activated a draft")
	}
	definition.Notes = "A revised draft"
	d, err = db.SaveLinguisticDefinition(ctx, world, first, "language", d.World.Revision, languages.Canonical(definition), languages.State{Status: "living"}, "Revised")
	if err != nil {
		t.Fatal(err)
	}
	head := d.Entity(first, id).Language.Draft
	if head == initial || len(d.Entity(first, id).Language.Revisions) != 2 || d.Entity(first, id).Language.Revisions[1].Parent != initial {
		t.Fatal("revision was not appended")
	}
	if _, err = db.SaveLinguisticDefinition(ctx, world, first, "language", d.World.Revision-1, languages.Canonical(definition), languages.State{}, ""); err == nil {
		t.Fatal("stale write accepted")
	}
	d, err = db.UpdateHistory(ctx, world, d.World.Revision, func(d *history.Document) error {
		return d.Apply(history.Command{Kind: "age", Mode: "branch", AgeID: first, Name: "Next Age"})
	})
	if err != nil {
		t.Fatal(err)
	}
	next := d.World.CurrentAge
	if d.Entity(next, id).ContinuityID != id || d.Entity(next, id).Language.Draft != head {
		t.Fatal("lost stable identity or head across Ages")
	}
	d.Entity(next, id).Language.Status = "extinct"
	if d.Entity(first, id).Language.Status != "living" {
		t.Fatal("Age state aliases predecessor")
	}
	malicious := *d.Entity(next, id)
	malicious.Language = languages.Clone(malicious.Language)
	malicious.Language.Draft = initial
	if err = d.Apply(history.Command{Kind: "entity", AgeID: next, Entity: &malicious}); err == nil {
		t.Fatal("generic entity command rewrote definition references")
	}
	d, err = db.History(ctx, world)
	if err != nil {
		t.Fatal(err)
	}
	report := languages.Compile(ctx, definition, nil).Report()
	d, err = db.RecordLanguageCompile(ctx, world, first, id, d.World.Revision, report)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ActivateLanguage(ctx, world, first, id, head, d.World.Revision); err == nil {
		t.Fatal("activated an empty language without ready operations")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := db.LinguisticDefinition(ctx, world, next, id, initial)
	if err != nil {
		t.Fatal(err)
	}
	old, err := languages.Decode(raw)
	if err != nil || old.Notes != "undecided modules" {
		t.Fatal("restart lost immutable source")
	}
	other := history.New("Unrelated")
	if err = db.CreateHistory(ctx, other); err != nil {
		t.Fatal(err)
	}
	if _, err = db.LinguisticDefinition(ctx, other.World.ID, other.World.CurrentAge, id, head); err == nil {
		t.Fatal("hash ownership escaped World")
	}
	b, err := db.HistoryBundle(ctx, world)
	if err != nil {
		t.Fatal(err)
	}
	if b.Schema != 2 {
		t.Fatal("language archive did not protect itself from older lossy readers")
	}
	legacy := b
	legacy.Schema = 1
	if legacy.Validate() == nil {
		t.Fatal("schema 1 accepted language state it cannot preserve")
	}
	if len(b.Blobs) != 2 {
		t.Fatal("bundle omitted retained definitions")
	}
	copyDB, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	if err = copyDB.ImportHistory(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err = copyDB.LinguisticDefinition(ctx, world, next, id, initial); err != nil {
		t.Fatal("portable import lost old revision", err)
	}
	b.Blobs[initial] = []byte(`{}`)
	if err = b.Validate(); err == nil {
		t.Fatal("corrupt language archive accepted")
	}
}

func TestIndependentScriptsAndPinnedDependencies(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	d := history.New("Scripts")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	age := d.World.CurrentAge
	script := languages.ScriptDefinition{Schema: 1, ScriptID: history.ID(), Name: "Shared", Direction: "rtl", Graphemes: []languages.Grapheme{{ID: "combined", Text: "ā"}}}
	d, err = db.SaveLinguisticDefinition(ctx, d.World.ID, age, "script", d.World.Revision, languages.Canonical(script), languages.State{}, "")
	if err != nil {
		t.Fatal(err)
	}
	ref := languages.ScriptRef{ID: script.ScriptID, Revision: languages.Hash(script)}
	for i := 0; i < 2; i++ {
		definition := languages.Definition{Schema: 1, LanguageID: history.ID(), Name: "A language", Orthography: &languages.Orthography{Script: ref, Normalization: "NFC"}}
		d, err = db.SaveLinguisticDefinition(ctx, d.World.ID, age, "language", d.World.Revision, languages.Canonical(definition), languages.State{}, "")
		if err != nil {
			t.Fatal(err)
		}
	}
	b, err := db.HistoryBundle(ctx, d.World.ID)
	if err != nil || len(b.Blobs) != 3 || b.Validate() != nil {
		t.Fatal("shared script bundle failed", err)
	}
	delete(b.Blobs, ref.Revision)
	if b.Validate() == nil {
		t.Fatal("missing script dependency accepted")
	}
	definition := languages.Definition{Schema: 1, LanguageID: history.ID(), Name: "Stolen", Orthography: &languages.Orthography{Script: languages.ScriptRef{ID: history.ID(), Revision: ref.Revision}}}
	if _, err = db.SaveLinguisticDefinition(ctx, d.World.ID, age, "language", d.World.Revision, languages.Canonical(definition), languages.State{}, ""); err == nil {
		t.Fatal("script hash accepted under another identity")
	}
}
func TestAgeNamesAreSeparateFromImmutableDefinitionLabels(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	d := history.New("Names")
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	age := d.World.CurrentAge
	definition := languages.Definition{Schema: 1, LanguageID: history.ID(), Name: "Definition label"}
	alias := "Name in this Age"
	d, err = db.SaveLinguisticDefinition(ctx, d.World.ID, age, "language", d.World.Revision, languages.Canonical(definition), languages.State{}, "", LinguisticSaveOptions{DisplayName: &alias})
	if err != nil {
		t.Fatal(err)
	}
	initial := d.Entity(age, definition.LanguageID).Language.Draft
	definition.Notes = "A changed rule draft"
	d, err = db.SaveLinguisticDefinition(ctx, d.World.ID, age, "language", d.World.Revision, languages.Canonical(definition), languages.State{}, "")
	if err != nil {
		t.Fatal(err)
	}
	head := d.Entity(age, definition.LanguageID).Language.Draft
	if d.Entity(age, definition.LanguageID).Name != alias || len(d.Entity(age, definition.LanguageID).Language.Revisions) != 2 {
		t.Fatal("definition save overwrote the Age name")
	}
	alias = "Renamed in this Age"
	d, err = db.SaveLinguisticDefinition(ctx, d.World.ID, age, "language", d.World.Revision, languages.Canonical(definition), languages.State{}, "", LinguisticSaveOptions{DisplayName: &alias})
	if err != nil {
		t.Fatal(err)
	}
	if d.Entity(age, definition.LanguageID).Language.Draft != head || len(d.Entity(age, definition.LanguageID).Language.Revisions) != 2 {
		t.Fatal("identity rename changed linguistic source revision")
	}
	d, err = db.RestoreLinguisticDraft(ctx, d.World.ID, age, definition.LanguageID, initial, head, d.World.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if d.Entity(age, definition.LanguageID).Name != alias {
		t.Fatal("restoration changed the Age identity name")
	}
}
