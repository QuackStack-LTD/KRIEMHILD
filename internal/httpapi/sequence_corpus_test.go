package httpapi

import (
	"context"
	"encoding/json"
	"kriemhild/internal/languages"
	"kriemhild/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLanguagesSequenceCorpusAPIExactPinsReadOnlyAndPermissions(t *testing.T) {
	s, db, owner, viewer, doc := secureFixture(t)
	share(t, db, owner, viewer, doc.World.ID, "viewer")
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile("../languages/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	base := "/api/worlds/" + doc.World.ID + "/languages"
	for _, v := range [][2]string{{"script", "sequence-script.json"}, {"language", "sequence.json"}, {"language", "sequence-normalized.json"}} {
		w := secured(s, owner, "POST", base, string(languages.Canonical(map[string]any{"kind": v[0], "ageId": doc.World.CurrentAge, "revision": doc.World.Revision, "definition": json.RawMessage(read(v[1])), "metadata": languages.State{}})))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
			t.Fatal(err)
		}
	}
	c, err := languages.DecodeSequenceCorpus(read("sequence.corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := base + "/" + c.Cases[0].Pin.LanguageID + "/corpus"
	revision := doc.World.Revision
	for _, mode := range []string{"validate", "execute"} {
		w := secured(s, viewer, "POST", path, string(languages.Canonical(map[string]any{"ageId": doc.World.CurrentAge, "mode": mode, "corpus": c})))
		if w.Code != 200 {
			t.Fatal("viewer could not verify read-only corpus", w.Code, w.Body)
		}
		var response struct {
			Corpus languages.CorpusReport `json:"corpus"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Corpus.Status != "success" || mode == "validate" && response.Corpus.Calls != 0 || mode == "execute" && response.Corpus.Calls != 18 {
			t.Fatal(response.Corpus.Status, response.Corpus.Calls)
		}
	}
	w := secured(s, owner, "GET", "/api/worlds/"+doc.World.ID, "")
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.World.Revision != revision {
		t.Fatal("corpus mutated immutable storage")
	}
	for _, e := range doc.Entities {
		if e.Language != nil && (len(e.Language.Compiled) > 0 || e.Language.Active != "") {
			t.Fatal("verification persisted compile/activation")
		}
	}
	bad := languages.Clone(c)
	bad.Cases[3].Pin.Script.Revision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	w = secured(s, viewer, "POST", path, string(languages.Canonical(map[string]any{"ageId": doc.World.CurrentAge, "mode": "execute", "corpus": bad})))
	if w.Code != 400 && w.Code != 422 {
		t.Fatal("mismatched dependency accepted", w.Code)
	}
	started, release := make(chan struct{}), make(chan struct{})
	body := string(languages.Canonical(map[string]any{"ageId": doc.World.CurrentAge, "mode": "execute", "corpus": c}))
	req := httptest.NewRequest("POST", path, &languageBlockingReader{started: started, release: release, data: strings.NewReader(body)})
	req.AddCookie(&http.Cookie{Name: "__Host-kriemhild", Value: viewer.secret})
	req.Header.Set("X-CSRF-Token", viewer.csrf)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { w := httptest.NewRecorder(); s.ServeHTTP(w, req); done <- w }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("corpus did not release capture gate")
	}
	gate := make(chan struct{})
	go func() { s.accessMu.Lock(); s.accessMu.Unlock(); close(gate) }()
	select {
	case <-gate:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("corpus held global access gate")
	}
	if err := db.SetMember(storage.WithActor(context.Background(), owner.u.ID), doc.World.ID, viewer.u.ID, ""); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case w := <-done:
		if w.Code != 404 || strings.Contains(w.Body.String(), "preimages") {
			t.Fatal("revoked member received computed corpus", w.Code, w.Body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revoked corpus did not finish")
	}
	w = secured(s, viewer, "POST", path, string(languages.Canonical(map[string]any{"ageId": doc.World.CurrentAge, "mode": "execute", "corpus": c})))
	if w.Code != 404 {
		t.Fatal("revoked member accessed corpus", w.Code)
	}
}
