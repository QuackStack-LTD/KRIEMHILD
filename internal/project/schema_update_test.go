package project

import (
	"reflect"
	"testing"
)

func TestSchemaRenameMigratesEntitiesAtomicallyAndPreservesAges(t *testing.T) {
	s, st := fixture(t)
	schema := Record{ID: NewID(), Kind: "schema", Name: "Port", Fields: []Field{{Key: "population", Type: "number"}}}
	st = put(t, s, st, schema)
	entity := Record{ID: NewID(), Kind: "entity", Name: "Harbor", Type: "Port", Properties: map[string]any{"population": float64(42), "custom": "preserved"}}
	st = put(t, s, st, entity)
	source := st
	st = apply(t, s, st, Command{Action: "copy-age", Name: "Later Age"})
	schema.Name = "Seaport"
	if _, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "put", Record: &schema}); e == nil {
		t.Fatal("rename detached the old entity type")
	}
	before := st
	invalid := schema
	invalid.Fields = []Field{{Key: "population", Type: "boolean"}}
	if _, e := s.Apply(Command{Expected: st.Revision, Age: st.Age.ID, Action: "update-schema", Record: &invalid}); e == nil {
		t.Fatal("invalid migration accepted")
	}
	unchanged, _ := s.State(st.Age.ID)
	if unchanged.Revision != st.Revision {
		t.Fatal("failed migration changed head")
	}
	st = apply(t, s, st, Command{Action: "update-schema", Record: &schema})
	if st.Records[entity.ID].Type != "Seaport" || !reflect.DeepEqual(st.Records[entity.ID].Properties, entity.Properties) {
		t.Fatal("schema association or custom values lost")
	}
	old, e := s.State(source.Age.ID)
	if e != nil || !reflect.DeepEqual(old.Records, source.Records) {
		t.Fatal("source age changed", e)
	}
	st = apply(t, s, st, Command{Action: "undo"})
	if !reflect.DeepEqual(st.Records, before.Records) {
		t.Fatal("migration undo not atomic")
	}
}
