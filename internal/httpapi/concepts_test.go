package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
)

func TestConceptRestartAgeAndZIP(t *testing.T) {
	ctx, dir := context.Background(), t.TempDir()
	db, err := storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	d := history.New("Organizations")
	a := d.World.CurrentAge
	entities := []history.Entity{
		{ID: "org", Name: "Nomad league", Kind: "organization", Properties: json.RawMessage(`{"governance":"decentralized","status":"active"}`)},
		{ID: "person", Name: "Ada", Kind: "character", Properties: json.RawMessage(`{"graphPosition":{"x":240,"y":180}}`)},
		{ID: "unit", Name: "Northern cell", Kind: "organization-unit", Properties: json.RawMessage(`{"organizationId":"org","order":3}`)},
		{ID: "office", Name: "Speaker", Kind: "organization-position", Properties: json.RawMessage(`{"organizationId":"org","unitId":"unit","shared":true}`)},
		{ID: "city", Name: "Rivergate", Kind: "city", Properties: json.RawMessage(`{}`)},
		{ID: "faith", Name: "River tradition", Kind: "religion", Properties: json.RawMessage(`{"startYear":42,"endYear":87}`), Content: &history.ConceptContent{Version: 1, Template: "religion", Tags: []string{"river"}, Components: []history.ConceptComponent{{ID: "doctrine", Type: "belief", Title: "Memory of water", Claim: &history.Claim{Statement: "Water remembers", WorldTruth: "unknown"}, Fields: []history.TypedField{{Definition: history.FieldDefinition{ID: "importance", Name: "Importance", Type: "number"}, Value: json.RawMessage(`3`)}}, Contexts: []history.ConceptVariation{{ID: "local", ContextID: "city", Stance: "metaphorical"}}}}}},
	}
	for _, e := range entities {
		if err = d.Apply(history.Command{Kind: "entity", AgeID: a, Entity: &e}); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []history.Relation{
		{ID: "membership", Source: "person", Destination: "org", Kind: "member-of", UnitID: "unit", Status: "active", Role: "Scout", Rank: "First", ValidFrom: "42", Notes: "Voluntary", AppointmentSource: "Elected"},
		{ID: "appointment", Source: "person", Destination: "office", Kind: "appointed-to", Status: "temporary"},
		{ID: "acceptance", Source: "person", Destination: "faith", Kind: "interprets", Stance: "metaphorical", Authority: "personal", Prevalence: "minority", Provenance: "authored", ValidFrom: "42"},
		{ID: "presence", Source: "unit", Destination: "city", Kind: "operates-in", Description: "Seasonal market"},
	} {
		if err = d.Apply(history.Command{Kind: "relation", AgeID: a, Relation: &r}); err != nil {
			t.Fatal(err)
		}
	}
	if err = d.Apply(history.Command{Kind: "age", AgeID: a, Mode: "branch", Name: "Successor"}); err != nil {
		t.Fatal(err)
	}
	b := d.World.CurrentAge
	concept := *d.Entity(b, "faith")
	concept.Content.Claim = &history.Claim{Statement: "A new interpretation", WorldTruth: "undecided"}
	if err = d.Apply(history.Command{Kind: "entity", AgeID: b, Entity: &concept}); err != nil {
		t.Fatal(err)
	}
	unit := *d.Entity(b, "unit")
	unit.Name = "Southern cell"
	if err = d.Apply(history.Command{Kind: "entity", AgeID: b, Entity: &unit}); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateHistory(ctx, d); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = storage.Open(ctx, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewWithOptions(t.TempDir(), Options{Projects: db, CacheDir: t.TempDir()})
	out := httptest.NewRecorder()
	s.ServeHTTP(out, httptest.NewRequest("GET", "/api/worlds/"+d.World.ID+"/export", nil))
	if out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	target, err := storage.Open(ctx, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	fresh := NewWithOptions(t.TempDir(), Options{Projects: target, CacheDir: t.TempDir()})
	result := httptest.NewRecorder()
	fresh.ServeHTTP(result, httptest.NewRequest("POST", "/api/worlds/import", bytes.NewReader(out.Body.Bytes())))
	if result.Code != 200 && result.Code != 201 {
		t.Fatal(result.Code, result.Body.String())
	}
	restored, err := target.History(ctx, d.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(d.Entities, func(i, j int) bool {
		return d.Entities[i].AgeID+d.Entities[i].ID < d.Entities[j].AgeID+d.Entities[j].ID
	})
	sort.Slice(restored.Entities, func(i, j int) bool {
		return restored.Entities[i].AgeID+restored.Entities[i].ID < restored.Entities[j].AgeID+restored.Entities[j].ID
	})
	sort.Slice(d.Relations, func(i, j int) bool {
		return d.Relations[i].AgeID+d.Relations[i].ID < d.Relations[j].AgeID+d.Relations[j].ID
	})
	sort.Slice(restored.Relations, func(i, j int) bool {
		return restored.Relations[i].AgeID+restored.Relations[i].ID < restored.Relations[j].AgeID+restored.Relations[j].ID
	})
	if !reflect.DeepEqual(restored.Entities, d.Entities) || !reflect.DeepEqual(restored.Relations, d.Relations) {
		t.Fatal("organization state or graph layout changed during restart/ZIP restoration")
	}
	if !reflect.DeepEqual(restored.Years, d.Years) || restored.Entity(a, "faith").Content.Claim != nil || restored.Entity(b, "faith").Content.Claim.WorldTruth != "undecided" {
		t.Fatal("concept history or dates changed")
	}
	if restored.Entity(a, "unit").Name != "Northern cell" || restored.Entity(b, "unit").Name != "Southern cell" {
		t.Fatal("Age isolation lost")
	}
}
