package httpapi

import (
	"context"
	"encoding/json"
	"kriemhild/internal/auth"
	"kriemhild/internal/history"
	"kriemhild/internal/languages"
	"kriemhild/internal/languages/spike"
	"kriemhild/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLanguagesPersistCompileRunAndRoles(t *testing.T) {
	s, db, owner, viewer, d := secureFixture(t)
	share(t, db, owner, viewer, d.World.ID, "viewer")
	age := d.World.CurrentAge
	id := history.ID()
	path := "/api/worlds/" + d.World.ID + "/languages"
	raw, err := os.ReadFile("../languages/spike/testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := spike.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	definition := languages.Definition{Schema: 1, LanguageID: id, Name: "Authored", Phonology: v.Phonology, Lexicon: v.Lexicon, Morphology: v.Morphology}
	body := func(v any) string { return string(languages.Canonical(v)) }
	save := map[string]any{"ageId": age, "revision": d.World.Revision, "kind": "language", "definition": definition, "metadata": languages.State{}, "note": "Initial"}
	if w := secured(s, viewer, "POST", path, body(save)); w.Code != 403 {
		t.Fatal("viewer saved", w.Code, w.Body)
	}
	w := secured(s, owner, "POST", path, body(save))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if err = json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	hash := d.Entity(age, id).Language.Draft
	base := map[string]any{"ageId": age, "revision": d.World.Revision, "definitionRevision": hash}
	if w = secured(s, viewer, "POST", path+"/"+id+"/validate", body(base)); w.Code != 200 {
		t.Fatal("viewer could not validate", w.Code, w.Body)
	}
	if w = secured(s, viewer, "POST", path+"/"+id+"/compile", body(base)); w.Code != 403 {
		t.Fatal("viewer compiled persisted metadata", w.Code)
	}
	w = secured(s, owner, "POST", path+"/"+id+"/compile", body(base))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var compiled struct {
		Document *history.Document `json:"document"`
		Report   languages.Report  `json:"report"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &compiled); err != nil {
		t.Fatal(err)
	}
	d = compiled.Document
	q := languages.Request{Request: spike.Request{Revision: hash, Engine: languages.EngineVersion, Operation: "inflect", Lexeme: "stone", Features: languages.Features{"number": "many"}, Policy: "all", Limit: 64}, Adapter: languages.AdapterVersion}
	base["request"] = q
	w = secured(s, viewer, "POST", path+"/"+id+"/run", body(base))
	if w.Code != 200 {
		t.Fatal("viewer run denied", w.Code, w.Body)
	}
	var result struct {
		Result languages.Result `json:"result"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Result.Status != "success" || len(result.Result.Candidates) != 1 {
		t.Fatal(w.Body)
	}
	before, _ := db.History(context.Background(), d.World.ID)
	if before.World.Revision != d.World.Revision || len(before.Entity(age, id).Language.Revisions) != 1 {
		t.Fatal("execution mutated author data")
	}
	stranger := accountFixture{}
	if w = secured(s, stranger, "GET", path+"/"+id+"?age="+age+"&revision="+hash, ""); w.Code != 401 {
		t.Fatal("anonymous definition read")
	}
	if w = secured(s, owner, "POST", path+"/"+id+"/compile", body(base)); w.Code != 409 {
		t.Fatal("stale compile write accepted", w.Code)
	}
	delete(base, "request")
	base["revision"] = d.World.Revision
	activate := map[string]any{"ageId": age, "revision": d.World.Revision, "definitionRevision": hash}
	w = secured(s, owner, "POST", path+"/"+id+"/activate", body(activate))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if !validHistoryRoute("/worlds/" + d.World.ID + "/languages") {
		t.Fatal("deep link unavailable")
	}
}

func TestLanguagesComputeReleasesGateAndRechecksSession(t *testing.T) {
	s, db, owner, _, d := secureFixture(t)
	// A blocked body read represents a request doing work after initial access
	// capture. Another request must still acquire the gate and revoke access.
	started, release := make(chan struct{}), make(chan struct{})
	reader := &languageBlockingReader{started: started, release: release, data: strings.NewReader(`{}`)}
	r := httptest.NewRequest("POST", "/api/worlds/"+d.World.ID+"/languages/"+history.ID()+"/validate", reader)
	r.AddCookie(&http.Cookie{Name: "__Host-kriemhild", Value: owner.secret})
	r.Header.Set("X-CSRF-Token", owner.csrf)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { w := httptest.NewRecorder(); s.ServeHTTP(w, r); done <- w }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("compute did not start")
	}
	gate := make(chan struct{})
	go func() { s.accessMu.Lock(); s.accessMu.Unlock(); close(gate) }()
	select {
	case <-gate:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("linguistic computation held global access gate")
	}
	if err := db.RevokeSession(context.Background(), owner.secret); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("compute did not finish")
	}
	// Directly test the final boundary, including mutation callback suppression.
	r = r.WithContext(auth.Context(r.Context(), owner.u))
	published := false
	w := httptest.NewRecorder()
	s.publishLanguage(w, r, func() { published = true })
	if published || w.Code != 401 {
		t.Fatal("revoked session published result or mutation")
	}
}

type languageBlockingReader struct {
	started, release chan struct{}
	data             *strings.Reader
	once             bool
}

func (r *languageBlockingReader) Read(p []byte) (int, error) {
	if !r.once {
		r.once = true
		close(r.started)
		<-r.release
	}
	return r.data.Read(p)
}

func TestLanguagePublicationRejectsRevokedMembership(t *testing.T) {
	s, db, owner, editor, d := secureFixture(t)
	share(t, db, owner, editor, d.World.ID, "editor")
	r := httptest.NewRequest("POST", "/api/worlds/"+d.World.ID+"/languages/"+history.ID()+"/compile", nil)
	r.AddCookie(&http.Cookie{Name: "__Host-kriemhild", Value: editor.secret})
	r = r.WithContext(auth.Context(r.Context(), editor.u))
	if err := db.SetMember(storage.WithActor(context.Background(), owner.u.ID), d.World.ID, editor.u.ID, ""); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	published := false
	s.publishLanguage(w, r, func() { published = true })
	if published || w.Code != 404 {
		t.Fatal("revoked membership published private result")
	}
}
