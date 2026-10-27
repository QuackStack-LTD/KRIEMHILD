package storage

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/languages"
	"os"
	"testing"
)

func TestLanguageSequenceImmutableRestartArchiveAndOwnership(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	read := func(name string) []byte {
		t.Helper()
		raw, e := os.ReadFile("../languages/testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	d, err := languages.Decode(read("sequence.json"))
	if err != nil {
		t.Fatal(err)
	}
	script, err := languages.DecodeScript(read("sequence-script.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc := history.New("Disposable sequence portability")
	if err = db.CreateHistory(ctx, doc); err != nil {
		t.Fatal(err)
	}
	world, age := doc.World.ID, doc.World.CurrentAge
	save := func(kind string, body any) {
		t.Helper()
		var e error
		doc, e = db.SaveLinguisticDefinition(ctx, world, age, kind, doc.World.Revision, languages.Canonical(body), languages.State{}, "Authored spelling source")
		if e != nil {
			t.Fatal(e)
		}
	}
	save("script", script)
	save("language", d)
	compile := func(source languages.Definition) {
		t.Helper()
		a := languages.Compile(ctx, source, map[languages.ScriptRef]languages.ScriptDefinition{{script.ScriptID, languages.Hash(script)}: script})
		if !a.Report().Ready["spell"] {
			t.Fatal(a.Report())
		}
		doc, err = db.RecordLanguageCompile(ctx, world, age, d.LanguageID, doc.World.Revision, a.Report())
		if err != nil {
			t.Fatal(err)
		}
		doc, err = db.ActivateLanguage(ctx, world, age, d.LanguageID, languages.Hash(source), doc.World.Revision)
		if err != nil {
			t.Fatal(err)
		}
	}
	compile(d)
	var q languages.Request
	if err = json.Unmarshal(read("sequence-forward.request.json"), &q); err != nil {
		t.Fatal(err)
	}
	var expected languages.Result
	if err = json.Unmarshal(read("sequence-forward.golden.json"), &expected); err != nil {
		t.Fatal(err)
	}
	oldHash := languages.Hash(d)
	script.Writing.Glyphs[0].Paths[0].Commands[1].Values[1] = 250
	script.Writing.Signs[4].Readings[0].Text = "new explicit reading"
	save("script", script)
	next := languages.Clone(d)
	next.SequenceWriting.Script.Revision = languages.Hash(script)
	next.SequenceWriting.Stages[0].Rules[0].Priority = 42
	save("language", next)
	compile(next)
	if doc.Entity(age, d.LanguageID).Language.Active == oldHash {
		t.Fatal("new revision was not explicitly activated")
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
		raw, e := store.LinguisticDefinition(ctx, world, age, d.LanguageID, oldHash)
		if e != nil {
			t.Fatal(e)
		}
		old, e := languages.Decode(raw)
		if e != nil {
			t.Fatal(e)
		}
		deps, e := store.LanguageDependencies(ctx, old)
		if e != nil {
			t.Fatal(e)
		}
		actual := languages.Compile(ctx, old, deps).Execute(ctx, q)
		if string(languages.Canonical(actual)) != string(languages.Canonical(expected)) {
			t.Fatal("pinned old spelling changed after newer activation/restart/import")
		}
	}
	verify(db)
	other := history.New("Separate World")
	if err = db.CreateHistory(ctx, other); err != nil {
		t.Fatal(err)
	}
	if _, err = db.SaveLinguisticDefinition(ctx, other.World.ID, other.World.CurrentAge, "language", other.World.Revision, languages.Canonical(d), languages.State{}, "Forbidden cross-World reference"); err == nil {
		t.Fatal("a globally present Script blob bypassed World/Age ownership")
	}
	bundle, err := db.HistoryBundle(ctx, world)
	if err != nil || bundle.Schema != 5 || bundle.Validate() != nil {
		t.Fatal("archive schema/pins invalid", err, bundle.Schema)
	}
	bad := languages.Clone(bundle)
	bad.Schema = 4
	if bad.Validate() == nil {
		t.Fatal("old reader barrier missing")
	}
	bad = languages.Clone(bundle)
	delete(bad.Blobs, d.SequenceWriting.Script.Revision)
	if bad.Validate() == nil {
		t.Fatal("missing retained Script dependency accepted")
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
