package httpapi

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/languages"
	"kriemhild/internal/languages/spike"
	"kriemhild/internal/storage"
	"os"
	"testing"
)

func TestLanguageHistoricalApprovalRevisionAndPortability(t *testing.T) {
	s, db, owner, viewer, d := secureFixture(t)
	share(t, db, owner, viewer, d.World.ID, "viewer")
	ctx := storage.WithActor(context.Background(), owner.u.ID)
	age := d.World.CurrentAge
	raw, err := os.ReadFile("../languages/spike/testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := spike.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	source := languages.Definition{Schema: 1, LanguageID: history.ID(), Name: "Ancestor", Phonology: fixture.Phonology, Lexicon: fixture.Lexicon, Morphology: fixture.Morphology}
	d, err = db.SaveLinguisticDefinition(ctx, d.World.ID, age, "language", d.World.Revision, languages.Canonical(source), languages.State{}, "")
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := languages.Hash(source)
	target := languages.Clone(source)
	target.LanguageID = history.ID()
	target.Name = "Descendant"
	target.Lexicon = nil
	target.History = &languages.Historical{SourceLanguage: source.LanguageID, SourceRevision: sourceHash, SourceAge: age, Changes: []spike.SoundRule{{ID: "historical-voicing", Domain: "word", Input: []spike.Match{{Segment: "t"}}, Output: []string{"d"}}}}
	d, err = db.SaveLinguisticDefinition(ctx, d.World.ID, age, "language", d.World.Revision, languages.Canonical(target), languages.State{}, "")
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/worlds/" + d.World.ID + "/languages/" + target.LanguageID
	body := map[string]any{"ageId": age, "revision": d.World.Revision, "definitionRevision": languages.Hash(target)}
	w := secured(s, viewer, "POST", path+"/history-preview", string(languages.Canonical(body)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var response struct {
		Preview  languages.Evolution `json:"preview"`
		Document *history.Document   `json:"document"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Preview.Status != "success" || len(response.Preview.Changes) != 4 {
		t.Fatal(w.Body)
	}
	body["previewHash"] = response.Preview.PreviewHash
	if w = secured(s, owner, "POST", path+"/history-apply", string(languages.Canonical(body))); w.Code != 409 {
		t.Fatal("applied without explicit author approval")
	}
	body["approved"] = true
	if w = secured(s, viewer, "POST", path+"/history-apply", string(languages.Canonical(body))); w.Code != 403 {
		t.Fatal("viewer applied historical changes", w.Code)
	}
	body["revision"] = d.World.Revision - 1
	if w = secured(s, owner, "POST", path+"/history-apply", string(languages.Canonical(body))); w.Code != 409 {
		t.Fatal("stale historical apply accepted")
	}
	body["revision"] = d.World.Revision
	w = secured(s, owner, "POST", path+"/history-apply", string(languages.Canonical(body)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	d = response.Document
	if d.Entity(age, source.LanguageID).Language.Draft != sourceHash || len(d.Entity(age, target.LanguageID).Language.Revisions) != 2 {
		t.Fatal("historical apply changed source or replaced target revision")
	}
	loaded, err := db.LinguisticDefinition(ctx, d.World.ID, age, source.LanguageID, sourceHash)
	if err != nil || string(loaded) != string(languages.Canonical(source)) {
		t.Fatal("source body changed")
	}
	if w = secured(s, owner, "POST", path+"/history-apply", string(languages.Canonical(body))); w.Code != 409 {
		t.Fatal("historical approval replay accepted")
	}
	restore := map[string]any{"ageId": age, "revision": d.World.Revision, "definitionRevision": languages.Hash(target), "expectedDraft": d.Entity(age, target.LanguageID).Language.Draft}
	if w = secured(s, viewer, "POST", path+"/restore", string(languages.Canonical(restore))); w.Code != 403 {
		t.Fatal("viewer restored a draft")
	}
	w = secured(s, owner, "POST", path+"/restore", string(languages.Canonical(restore)))
	if w.Code != 200 {
		t.Fatal("draft restore failed", w.Code, w.Body)
	}
	if err = json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Entity(age, target.LanguageID).Language.Draft != languages.Hash(target) || len(d.Entity(age, target.LanguageID).Language.Revisions) != 2 || d.Entity(age, source.LanguageID).Language.Draft != sourceHash {
		t.Fatal("restoration rewrote a body or its source")
	}
	bundle, err := db.HistoryBundle(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Historical provenance remains portable even if the source's standalone
	// Age record is absent; the exact pinned body is still a reachable dependency.
	bundle.Document.Entities = []history.Entity{*d.Entity(age, target.LanguageID)}
	if err = bundle.Validate(); err != nil {
		t.Fatal("pinned historical source lost portability", err)
	}
	delete(bundle.Blobs, sourceHash)
	if err = bundle.Validate(); err == nil {
		t.Fatal("missing historical source accepted")
	}
}
