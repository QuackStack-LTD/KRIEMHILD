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

func TestLanguagesIndexedAPIUsesPinnedAdapterAndReadGate(t *testing.T) {
	s, db, owner, viewer, doc := secureFixture(t)
	share(t, db, owner, viewer, doc.World.ID, "viewer")
	raw, err := os.ReadFile("../languages/testdata/indexed.json")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := languages.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	age := doc.World.CurrentAge
	base := "/api/worlds/" + doc.World.ID + "/languages"
	body := func(v any) string { return string(languages.Canonical(v)) }
	response := secured(s, owner, "POST", base, body(map[string]any{"ageId": age, "revision": doc.World.Revision, "kind": "language", "definition": definition, "metadata": languages.State{}}))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	if err = json.Unmarshal(response.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	path := base + "/" + definition.LanguageID
	envelope := map[string]any{"ageId": age, "revision": doc.World.Revision, "definitionRevision": languages.Hash(definition)}
	response = secured(s, owner, "POST", path+"/compile", body(envelope))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	q := languages.Request{Request: spike.Request{Revision: languages.Hash(definition), Engine: languages.IndexedEngine, Operation: "analyze", Segments: []string{"p", "a", "d", "a"}, Policy: "all", Limit: 64}, Adapter: languages.IndexedAdapter, Compiler: languages.IndexedCompiler}
	envelope["request"] = q
	response = secured(s, viewer, "POST", path+"/run", body(envelope))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"survivor-trie/1"`) || !strings.Contains(response.Body.String(), `"knownValid":2`) {
		t.Fatal(response.Code, response.Body)
	}
	if response = secured(s, accountFixture{}, "POST", path+"/run", body(envelope)); response.Code != 401 {
		t.Fatal("private indexed result bypassed session gate", response.Code)
	}
	q.Adapter = languages.ProductiveAdapter
	envelope["request"] = q
	response = secured(s, viewer, "POST", path+"/run", body(envelope))
	if !strings.Contains(response.Body.String(), `"incompatible_engine"`) {
		t.Fatal("fallback adapter used", response.Body)
	}
}

func TestLanguagesProductiveAPIAndCapabilities(t *testing.T) {
	s, db, owner, viewer, d := secureFixture(t)
	share(t, db, owner, viewer, d.World.ID, "viewer")
	basePath := "/api/worlds/" + d.World.ID + "/languages"
	age := d.World.CurrentAge
	capabilities := secured(s, viewer, "GET", basePath+"/capabilities", "")
	if capabilities.Code != 200 || !strings.Contains(capabilities.Body.String(), `"finite-compose/1"`) || !strings.Contains(capabilities.Body.String(), `"unsupported"`) {
		t.Fatal("capabilities not sourced from adapter", capabilities.Code, capabilities.Body)
	}
	raw, err := os.ReadFile("../languages/testdata/productive.json")
	if err != nil {
		t.Fatal(err)
	}
	definition, err := languages.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	body := func(v any) string { return string(languages.Canonical(v)) }
	response := secured(s, owner, "POST", basePath, body(map[string]any{"ageId": age, "revision": d.World.Revision, "kind": "language", "definition": definition, "metadata": languages.State{}}))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	if err = json.Unmarshal(response.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	head := d.Entity(age, definition.LanguageID).Language.Draft
	envelope := map[string]any{"ageId": age, "revision": d.World.Revision, "definitionRevision": head}
	path := basePath + "/" + definition.LanguageID
	if response = secured(s, viewer, "POST", path+"/compile", body(envelope)); response.Code != 403 {
		t.Fatal("viewer compiled productive source")
	}
	response = secured(s, owner, "POST", path+"/compile", body(envelope))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	var compiled struct {
		Document *history.Document `json:"document"`
		Report   languages.Report  `json:"report"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &compiled); err != nil {
		t.Fatal(err)
	}
	if compiled.Report.Compiler != languages.ProductiveCompiler || !compiled.Report.Ready["inflect"] || compiled.Report.Forms != 0 {
		t.Fatal("API did not compile productive plans", compiled.Report)
	}
	d = compiled.Document
	q := languages.Request{Request: spike.Request{Revision: head, Engine: languages.ProductiveEngine, Operation: "analyze", Segments: []string{"p", "a", "d", "a"}, Policy: "all", Limit: 64}, Adapter: languages.ProductiveAdapter, Compiler: languages.ProductiveCompiler}
	envelope["request"] = q
	response = secured(s, viewer, "POST", path+"/run", body(envelope))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	var result struct {
		Result  languages.Result  `json:"result"`
		Context map[string]string `json:"context"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Result.Status != "ambiguous" || result.Result.KnownValid != 2 || len(result.Result.Derivations) != 2 || result.Context["worldId"] != d.World.ID || result.Context["ageId"] != age {
		t.Fatal("API lost analyses/provenance", response.Body)
	}
	before, err := db.History(context.Background(), d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.World.Revision != d.World.Revision {
		t.Fatal("run mutated a World")
	}
	q.Engine = languages.EngineVersion
	envelope["request"] = q
	response = secured(s, viewer, "POST", path+"/run", body(envelope))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"incompatible_engine"`) {
		t.Fatal("API used fallback adapter", response.Body)
	}
	if response = secured(s, accountFixture{}, "GET", basePath+"/capabilities", ""); response.Code != 401 {
		t.Fatal("capability route bypassed authentication")
	}
}

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
