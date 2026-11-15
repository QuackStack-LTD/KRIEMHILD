package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	terrainmodel "kriemhild/internal/terrain/model"
	"kriemhild/internal/world"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestAuthenticatedContinuousPreviewDoesNotImplicitlySave(t *testing.T) {
	s, db, owner, _, _ := secureFixture(t)
	v := projectFixture(t)
	v.creatorID, v.actorID = owner.u.ID, owner.u.ID
	// Only the presence of the continuous source is needed by these policy
	// checks: none of these requests should encode or persist the terrain.
	v.planet = &terrainmodel.Planet{}
	s.sessions["preview"] = v
	request := func(path, payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/sessions/preview"+path, strings.NewReader(payload))
		r.AddCookie(&http.Cookie{Name: "__Host-kriemhild", Value: owner.secret})
		r.Header.Set("X-CSRF-Token", owner.csrf)
		r.Header.Set("If-Match", strconv.Itoa(v.collabRevision))
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	r := request("/view", `{"ui":{"continuousView":{"mode":"generate"}},"merge":true}`)
	if r.Code != 200 || !v.unsaved || v.persisted {
		t.Fatal("view staging persisted terrain", r.Code, r.Body)
	}
	for _, path := range []string{"/save", "/world"} {
		payload := `{}`
		if path == "/world" {
			payload = `{"kind":"save","revision":0}`
		}
		r = request(path, payload)
		if r.Code != 409 || !bytes.Contains(r.Body.Bytes(), []byte("Terrain Generator")) {
			t.Fatal("unlinked continuous save was accepted", r.Code, r.Body)
		}
	}
	projects, err := db.List(context.Background())
	if err != nil || len(projects) != 0 {
		t.Fatal("preview created a standalone project", projects, err)
	}
}

func TestSavedIllustrationRecoveryRespectsMapAgeAndTerrainRevision(t *testing.T) {
	art := world.Illustration{Version: 1, ID: "medieval-world", Title: "Library design", Scope: "world", Style: "medieval", Bounds: [4]float64{0, 0, 2, 1}, Layers: []world.IllustrationLayer{{ID: "decoration", Name: "Decoration", Visible: true, Opacity: 1}}, Objects: map[string]world.IllustrationObject{}}
	m := history.Map{ID: "map-a", AgeID: "age-a"}
	for _, tc := range []struct {
		name, mapID, ageID string
		revision           int
		existing, deleted  bool
		want               string
	}{
		{"missing design", "map-a", "age-a", 5, false, false, art.Title},
		{"newer Library design", "map-a", "age-a", 12, true, false, art.Title},
		{"newer terrain save", "map-a", "age-a", 5, true, false, "Terrain design"},
		{"other map", "map-b", "age-a", 12, true, false, "Terrain design"},
		{"other Age", "map-a", "age-b", 12, true, false, "Terrain design"},
		{"deleted design", "map-a", "age-a", 12, true, true, "Terrain design"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := world.New(48, 32)
			model.Header.Revision = 10
			if tc.existing {
				old := art
				old.Title = "Terrain design"
				model.Header.Illustrations = map[string]world.Illustration{old.ID: old}
			}
			properties, _ := json.Marshal(map[string]any{"illustration": map[string]any{"sourceMapId": tc.mapID, "sourceRevision": tc.revision, "design": art}})
			d := &history.Document{Entities: []history.Entity{{ID: "image", Kind: "illustrated-map", AgeID: tc.ageID, Deleted: tc.deleted, Properties: properties}}}
			restoreSavedIllustrationDesigns(model, d, m)
			if model.Header.Illustrations[art.ID].Title != tc.want {
				t.Fatal(model.Header.Illustrations)
			}
			if len(model.Operations) != 0 || len(model.Header.History) != 0 {
				t.Fatal("illustration recovery restored pending terrain edits")
			}
		})
	}
}
