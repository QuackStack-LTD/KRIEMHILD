package httpapi

import (
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/languages"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLanguagesScriptComputeRechecksRevocationOutsideGate(t *testing.T) {
	s, db, owner, _, doc := secureFixture(t)
	raw, err := os.ReadFile("../languages/testdata/vector-script.json")
	if err != nil {
		t.Fatal(err)
	}
	var definition languages.ScriptDefinition
	if err = json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	body := string(languages.Canonical(map[string]any{"ageId": doc.World.CurrentAge, "definition": definition, "svg": `<svg><path d="M0 0L1 1"/></svg>`, "glyphId": "authored", "glyphName": "Private authored input"}))
	started, release := make(chan struct{}), make(chan struct{})
	reader := &languageBlockingReader{started: started, release: release, data: strings.NewReader(body)}
	r := httptest.NewRequest("POST", "/api/worlds/"+doc.World.ID+"/languages/"+definition.ScriptID+"/import-svg", reader)
	r.AddCookie(&http.Cookie{Name: "__Host-kriemhild", Value: owner.secret})
	r.Header.Set("X-CSRF-Token", owner.csrf)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { w := httptest.NewRecorder(); s.ServeHTTP(w, r); done <- w }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("script compute did not enter isolated work")
	}
	gate := make(chan struct{})
	go func() { s.accessMu.Lock(); s.accessMu.Unlock(); close(gate) }()
	select {
	case <-gate:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("script compute held global access gate")
	}
	if err = db.RevokeSession(context.Background(), owner.secret); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case w := <-done:
		if w.Code != 401 || strings.Contains(w.Body.String(), `"import"`) {
			t.Fatal("revoked session received script output", w.Code, w.Body)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("script compute did not finish")
	}
}

func TestLanguagesVectorScriptAPIAndSafeImport(t *testing.T) {
	s, db, owner, viewer, doc := secureFixture(t)
	share(t, db, owner, viewer, doc.World.ID, "viewer")
	base := "/api/worlds/" + doc.World.ID + "/languages"
	age := doc.World.CurrentAge
	raw, err := os.ReadFile("../languages/testdata/vector-script.json")
	if err != nil {
		t.Fatal(err)
	}
	script, err := languages.DecodeScript(raw)
	if err != nil {
		t.Fatal(err)
	}
	body := func(v any) string { return string(languages.Canonical(v)) }
	response := secured(s, owner, "POST", base, body(map[string]any{"ageId": age, "revision": doc.World.Revision, "kind": "script", "definition": script, "metadata": languages.State{}}))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	if err = json.Unmarshal(response.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	path := base + "/" + script.ScriptID
	req := map[string]any{"ageId": age, "revision": doc.World.Revision, "definitionRevision": languages.Hash(script)}
	if response = secured(s, viewer, "POST", path+"/compile", body(req)); response.Code != 403 {
		t.Fatal("viewer persisted compile report", response.Code)
	}
	response = secured(s, owner, "POST", path+"/compile", body(req))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body)
	}
	var compiled struct {
		Document *history.Document `json:"document"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &compiled); err != nil {
		t.Fatal(err)
	}
	doc = compiled.Document
	q := languages.ScriptRenderRequest{Revision: languages.Hash(script), Engine: languages.VectorEngine, Adapter: languages.VectorAdapter, Compiler: languages.VectorCompiler, SequenceID: "mixed-phrase"}
	req["scriptRequest"] = q
	response = secured(s, viewer, "POST", path+"/run", body(req))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"status":"success"`) || !strings.Contains(response.Body.String(), `"classifier"`) {
		t.Fatal("script did not execute through shared API", response.Code, response.Body)
	}
	req["definition"] = script
	req["svg"] = `<svg viewBox="0 0 100 100"><path d="M10 10L90 90" stroke="#000000" fill="none"/></svg>`
	req["glyphId"] = "new-stable-glyph"
	req["glyphName"] = "Safe imported artwork"
	response = secured(s, viewer, "POST", path+"/import-svg", body(req))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"op":"L"`) {
		t.Fatal("normalized import missing", response.Code, response.Body)
	}
	req["svg"] = `<svg><image href="https://example.com/unsafe"/></svg>`
	response = secured(s, viewer, "POST", path+"/import-svg", body(req))
	if response.Code != 422 || !strings.Contains(response.Body.String(), "svg_import_rejected") {
		t.Fatal("unsafe import was not rejected", response.Code, response.Body)
	}
	q.Revision = ""
	req["scriptRequest"] = q
	response = secured(s, viewer, "POST", path+"/render-script", body(req))
	if response.Code != 200 || !strings.Contains(response.Body.String(), languages.Hash(script)) {
		t.Fatal("draft preview did not capture a versioned input", response.Code, response.Body)
	}
	if response = secured(s, accountFixture{}, "POST", path+"/render-script", body(req)); response.Code != 401 {
		t.Fatal("script output bypassed session gate")
	}
}
