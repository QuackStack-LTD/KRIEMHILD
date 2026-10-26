package storage

import (
	"context"
	"kriemhild/internal/history"
	"kriemhild/internal/languages"
	"os"
	"testing"
)

func TestLanguageVectorScriptImmutableRestartAndZIP(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	doc := history.New("Disposable vector-script portability")
	if err = db.CreateHistory(ctx, doc); err != nil {
		t.Fatal(err)
	}
	world, age := doc.World.ID, doc.World.CurrentAge
	raw, err := os.ReadFile("../languages/testdata/vector-script.json")
	if err != nil {
		t.Fatal(err)
	}
	script, err := languages.DecodeScript(raw)
	if err != nil {
		t.Fatal(err)
	}
	old := languages.ScriptDefinition{Schema: 1, ScriptID: script.ScriptID, Name: "Retained Unicode-only script", Direction: "undecided", Graphemes: []languages.Grapheme{}}
	for _, d := range []languages.ScriptDefinition{old, script} {
		doc, err = db.SaveLinguisticDefinition(ctx, world, age, "script", doc.World.Revision, languages.Canonical(d), languages.State{}, "Explicit Script revision")
		if err != nil {
			t.Fatal(err)
		}
	}
	report := languages.CompileScript(ctx, script)
	doc, err = db.RecordLanguageCompile(ctx, world, age, script.ScriptID, doc.World.Revision, report)
	if err != nil {
		t.Fatal(err)
	}
	doc, err = db.ActivateLanguage(ctx, world, age, script.ScriptID, report.Revision, doc.World.Revision)
	if err != nil {
		t.Fatal(err)
	}
	q := languages.ScriptRenderRequest{Revision: languages.Hash(script), Engine: languages.VectorEngine, Adapter: languages.VectorAdapter, Compiler: languages.VectorCompiler, SequenceID: "mixed-phrase"}
	expected := string(languages.Canonical(languages.RenderScript(ctx, script, q)))
	next := languages.Clone(script)
	next.Writing.Glyphs[0].Name = "New name, stable ID"
	next.Writing.Glyphs[0].Paths[0].Commands[1].Values[1] = 250
	next.Writing.Signs[0].Grapheme = "new authored association"
	next.Writing.Signs[0].Readings[0].Text = "new authored reading"
	next.Direction = "rtl"
	doc, err = db.SaveLinguisticDefinition(ctx, world, age, "script", doc.World.Revision, languages.Canonical(next), languages.State{}, "New geometry/reading/layout")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Entity(age, script.ScriptID).Script.Active != languages.Hash(script) {
		t.Fatal("draft change silently activated next revision")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	verify := func(store *Store) {
		t.Helper()
		raw, err := store.LinguisticDefinition(ctx, world, age, script.ScriptID, languages.Hash(script))
		if err != nil {
			t.Fatal(err)
		}
		d, err := languages.DecodeScript(raw)
		if err != nil {
			t.Fatal(err)
		}
		if string(languages.Canonical(languages.RenderScript(ctx, d, q))) != expected {
			t.Fatal("old revision identity/output changed")
		}
		raw, err = store.LinguisticDefinition(ctx, world, age, old.ScriptID, languages.Hash(old))
		if err != nil || string(raw) != string(languages.Canonical(old)) {
			t.Fatal("old Script schema 1 body changed", err)
		}
	}
	verify(db)
	bundle, err := db.HistoryBundle(ctx, world)
	if err != nil || bundle.Schema != 4 || bundle.Validate() != nil {
		t.Fatal("vector archive invalid", bundle.Schema, err)
	}
	bad := languages.Clone(bundle)
	bad.Schema = 3
	if bad.Validate() == nil {
		t.Fatal("older schema accepted vector source")
	}
	bad = languages.Clone(bundle)
	delete(bad.Blobs, languages.Hash(script))
	if bad.Validate() == nil {
		t.Fatal("missing glyph body accepted")
	}
	imported, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer imported.Close()
	if err = imported.ImportHistory(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	verify(imported)
}
