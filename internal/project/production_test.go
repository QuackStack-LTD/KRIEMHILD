package project

import (
	"encoding/json"
	"reflect"
	"testing"
)

func inventory(name, value, unit string) Record {
	return Record{ID: NewID(), Kind: "entity", Name: name, Type: "Resource or good", Properties: map[string]any{"_domain": "resource", "quantity": Quantity{Mode: "exact", Value: value, Unit: unit}}}
}
func inventoryValue(r Record) string {
	raw, _ := json.Marshal(r.Properties["quantity"])
	var q Quantity
	json.Unmarshal(raw, &q)
	return q.Value
}
func TestProductionNetworkPreviewAcceptanceAndHistory(t *testing.T) {
	s, st := fixture(t)
	grain, flour, bread := inventory("Grain", "10", "kg"), inventory("Flour", "0", "kg"), inventory("Bread", "0", "loaves")
	mill := Record{ID: NewID(), Kind: "entity", Type: "Production recipe", Name: "Mill", Properties: map[string]any{"_domain": "recipe", "_production": ProductionRule{Inputs: []ProductionFlow{{grain.ID, "2", "kg"}}, Outputs: []ProductionFlow{{flour.ID, "3", "kg"}}, Batches: 4, Priority: 1}}}
	bakery := Record{ID: NewID(), Kind: "entity", Type: "Production recipe", Name: "Bakery", Properties: map[string]any{"_domain": "recipe", "_production": ProductionRule{Inputs: []ProductionFlow{{flour.ID, "2", "kg"}}, Outputs: []ProductionFlow{{bread.ID, "5", "loaves"}}, Batches: 5, Priority: 2}}}
	st = apply(t, s, st, Command{Action: "put-many", Records: []Record{grain, flour, bread, mill, bakery}})
	source := st
	st = apply(t, s, st, Command{Action: "copy-age", Name: "Industrial age"})
	req := ExperimentRequest{Expected: st.Revision, Age: st.Age.ID, Operation: "production-network", IDs: []string{bakery.ID, mill.ID}, Values: map[string]string{"periods": "2"}}
	result, err := s.Experiment(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AllOrNothing || len(result.Proposals) != 3 {
		t.Fatalf("linked proposals missing: %+v", result)
	}
	expected := map[string]string{grain.ID: "0", flour.ID: "1", bread.ID: "35"}
	for _, r := range result.Proposals {
		if inventoryValue(r) != expected[r.ID] {
			t.Fatalf("%s inventory %s", r.Name, inventoryValue(r))
		}
	}
	if result.Rows[0]["completed batches"] != "5" || result.Rows[1]["completed batches"] != "7" {
		t.Fatal(result.Rows)
	}
	again, err := s.Experiment(req)
	if err != nil || !reflect.DeepEqual(result, again) {
		t.Fatal("nondeterministic", err)
	}
	saved, _ := s.State(st.Age.ID)
	if !reflect.DeepEqual(saved.Records, st.Records) || saved.Revision != st.Revision {
		t.Fatal("preview mutated canon")
	}
	st = apply(t, s, st, Command{Action: "put-many", Records: result.Proposals})
	if _, err = s.Experiment(req); err != ErrConflict {
		t.Fatal("stale preview allowed", err)
	}
	old, err := s.State(source.Age.ID)
	if err != nil || !reflect.DeepEqual(old.Records, source.Records) {
		t.Fatal("source age changed", err)
	}
	if _, err = s.Apply(Command{Action: "delete", ID: grain.ID, Age: st.Age.ID, Expected: st.Revision}); err == nil {
		t.Fatal("dangling recipe allowed")
	}
}

func TestProductionExactFractionsCyclesAndInvalidInputs(t *testing.T) {
	s, st := fixture(t)
	a, b := inventory("A", "0.3", "kg"), inventory("B", "0", "kg")
	first := Record{ID: NewID(), Kind: "entity", Type: "Production recipe", Name: "First", Properties: map[string]any{"_domain": "recipe", "_production": ProductionRule{Inputs: []ProductionFlow{{a.ID, "0.1", "kg"}}, Outputs: []ProductionFlow{{b.ID, "0.2", "kg"}}, Batches: 3, Priority: 1}}}
	second := Record{ID: NewID(), Kind: "entity", Type: "Production recipe", Name: "Return", Properties: map[string]any{"_domain": "recipe", "_production": ProductionRule{Inputs: []ProductionFlow{{b.ID, "0.2", "kg"}}, Outputs: []ProductionFlow{{a.ID, "0.1", "kg"}}, Batches: 1, Priority: 2}}}
	st = apply(t, s, st, Command{Action: "put-many", Records: []Record{a, b, first, second}})
	req := ExperimentRequest{Expected: st.Revision, Age: st.Age.ID, Operation: "production-network", IDs: []string{first.ID, second.ID}, Values: map[string]string{"periods": "2"}}
	out, err := s.Experiment(req)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{a.ID: "0.1", b.ID: "0.4"}
	for _, r := range out.Proposals {
		if inventoryValue(r) != expected[r.ID] {
			t.Fatal("fraction or cycle error", r)
		}
	}
	for _, periods := range []string{"0", "101", "1.5"} {
		req.Values["periods"] = periods
		if _, err = s.Experiment(req); err == nil {
			t.Fatal("bad periods allowed")
		}
	}
	req.Values["periods"] = "1"
	for _, q := range []Quantity{{Mode: "unknown"}, {Mode: "exact", Value: "3", Unit: "tons"}, {Mode: "exact", Value: "-1", Unit: "kg"}} {
		changed := cloneProperties(st.Records[a.ID])
		changed.Properties["quantity"] = q
		st = put(t, s, st, changed)
		req.Expected = st.Revision
		if _, err = s.Experiment(req); err == nil {
			t.Fatal("invalid inventory inferred", q)
		}
	}
	bad := cloneProperties(first)
	bad.Properties["_production"] = ProductionRule{Inputs: []ProductionFlow{{a.ID, "1", "kg"}, {a.ID, "2", "kg"}}, Outputs: []ProductionFlow{{b.ID, "1", "kg"}}, Batches: 1}
	if _, err = s.Apply(Command{Action: "put", Record: &bad, Age: st.Age.ID, Expected: st.Revision}); err == nil {
		t.Fatal("duplicate input accepted")
	}
}
