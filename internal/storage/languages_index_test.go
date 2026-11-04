package storage

import (
	"context"
	"kriemhild/internal/history"
	"kriemhild/internal/languages"
	"kriemhild/internal/languages/spike"
	"os"
	"testing"
)

func TestLanguageIndexedRestartZIPAndImmutableVersions(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	doc := history.New("Disposable indexed inverse")
	if err = db.CreateHistory(ctx, doc); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../languages/testdata/productive.json")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := languages.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	world, age := doc.World.ID, doc.World.CurrentAge
	doc, err = db.SaveLinguisticDefinition(ctx, world, age, "language", doc.World.Revision, languages.Canonical(legacy), languages.State{}, "Retained A1")
	if err != nil {
		t.Fatal(err)
	}
	indexed := languages.Clone(legacy)
	indexed.Schema = 3
	doc, err = db.SaveLinguisticDefinition(ctx, world, age, "language", doc.World.Revision, languages.Canonical(indexed), languages.State{}, "Explicit A2 opt in")
	if err != nil {
		t.Fatal(err)
	}
	a := languages.Compile(ctx, indexed, nil)
	doc, err = db.RecordLanguageCompile(ctx, world, age, indexed.LanguageID, doc.World.Revision, a.Report())
	if err != nil {
		t.Fatal(err)
	}
	doc, err = db.ActivateLanguage(ctx, world, age, indexed.LanguageID, languages.Hash(indexed), doc.World.Revision)
	if err != nil {
		t.Fatal(err)
	}
	q := languages.Request{Request: spike.Request{Revision: languages.Hash(indexed), Engine: languages.IndexedEngine, Operation: "analyze", Segments: []string{"p", "a", "d", "a"}, Policy: "all", Limit: 64}, Adapter: languages.IndexedAdapter, Compiler: languages.IndexedCompiler}
	expected := string(languages.Canonical(a.Execute(ctx, q)))
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err = db.LinguisticDefinition(ctx, world, age, indexed.LanguageID, languages.Hash(indexed))
	if err != nil {
		t.Fatal(err)
	}
	restored, err := languages.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(languages.Canonical(languages.Compile(ctx, restored, nil).Execute(ctx, q))) != expected {
		t.Fatal("restart did not rebuild identical index")
	}
	bundle, err := db.HistoryBundle(ctx, world)
	if err != nil || bundle.Schema != 4 || bundle.Validate() != nil {
		t.Fatal("new archive barrier missing", bundle.Schema, err)
	}
	bad := languages.Clone(bundle)
	bad.Schema = 3
	if bad.Validate() == nil {
		t.Fatal("older archive silently accepted new indexed source")
	}
	imported, err := Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer imported.Close()
	if err = imported.ImportHistory(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	raw, err = imported.LinguisticDefinition(ctx, world, age, indexed.LanguageID, languages.Hash(indexed))
	if err != nil {
		t.Fatal(err)
	}
	restored, err = languages.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(languages.Canonical(languages.Compile(ctx, restored, nil).Execute(ctx, q))) != expected {
		t.Fatal("import changed result/provenance")
	}
	raw, err = imported.LinguisticDefinition(ctx, world, age, legacy.LanguageID, languages.Hash(legacy))
	if err != nil || string(raw) != string(languages.Canonical(legacy)) {
		t.Fatal("A1 body changed", err)
	}
}
