package world

import (
	"encoding/json"
	"kriemhild/internal/terrain"
	"math"
	"reflect"
	"testing"
)

func riverWorld() *terrain.Environment {
	e := &terrain.Environment{Options: terrain.EnvironmentOptions{Columns: 65, Rows: 65}, Fields: map[string][]float64{}}
	for _, name := range []string{"elevation", "waterBody", "waterLevel", "waterDepth"} {
		e.Fields[name] = make([]float64, 65*65)
	}
	for i := range e.Fields["elevation"] {
		e.Fields["elevation"][i] = 500 - float64(i%65)*3
	}
	return e
}
func TestAuthoredRiverSharedProfileAndPersistence(t *testing.T) {
	s := New(65, 65)
	env := riverWorld()
	op := Operation{ID: "river-a", Kind: "river", Layer: "water", Center: [2]float64{20, 10}, Radius: 2, Amount: 100, Path: [][2]float64{{20, 10}, {30, 12}, {40, 10}}}
	s.Prepare(&op, env)
	if _, err := s.AddOperation(op); err != nil {
		t.Fatal(err)
	}
	f := op.River
	if f == nil || len(f.Path) <= len(op.Path) || len(f.Widths) != len(f.Path) || f.Widths[0] >= f.Widths[len(f.Widths)-1] {
		t.Fatal("missing refined, widening channel")
	}
	for i := 1; i < len(f.Path); i++ {
		if f.Path[i][2] > f.Path[i-1][2] {
			t.Fatal("water runs uphill")
		}
	}
	at := f.Path[16]
	r := terrain.SampleChannel(*f, at[0], at[1])
	target, _ := terrain.ChannelBed(r)
	p := terrain.DetailPoint{Elevation: 500 - at[0]*3, Vegetation: .6}
	got := Apply(p, at[0], at[1], []Operation{op})
	if math.Abs(got.Elevation-target) > 1e-8 || got.RiverDepth <= 0 || got.RiverLevel != r.Level || got.Floodplain < .99 {
		t.Fatal("authored river diverged from shared channel profile", got)
	}
	bank := Apply(p, at[0], at[1]+r.Width*2, []Operation{op})
	if bank.RiverDepth != 0 || bank.Floodplain <= 0 {
		t.Fatal("valley painted entirely as water")
	}
	distant := Apply(p, at[0], at[1]+10, []Operation{op})
	if distant != p {
		t.Fatal("river changed unrelated terrain")
	}
	for _, tx := range []int{0, 1} {
		tile := terrain.DetailTile{Level: 0, X: tx, Y: 0, Size: 33, Step: 1, Points: make([]terrain.DetailPoint, 33*33)}
		for i := range tile.Points {
			tile.Points[i].Elevation = 500 - float64(tx*32+i%33)*3
		}
		s.ApplyTile(&tile, nil)
		if len(tile.Features) != 1 || !reflect.DeepEqual(tile.Features[0], *f) {
			t.Fatal("neighbor tiles disagree on saved river identity/geometry")
		}
	}
	data, _ := json.Marshal(s)
	restored := New(65, 65)
	if err := json.Unmarshal(data, restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(65, 65); err != nil {
		t.Fatal(err)
	}
	// A different environmental elevation must never overwrite the saved profile.
	for i := range env.Fields["elevation"] {
		env.Fields["elevation"][i] += 1000
	}
	restored.PrepareLegacyRivers(env)
	if !reflect.DeepEqual(s.Operations, restored.Operations) {
		t.Fatal("stored channel regenerated on reload")
	}
	if replay := Apply(p, at[0], at[1], restored.Ops(Bounds{at[0], at[1], 0, 0})); replay != got {
		t.Fatal("saved surface changed after reload")
	}
	broken := op
	copyFeature := *f
	broken.River = &copyFeature
	broken.River.Widths = append([]float64(nil), f.Widths...)
	broken.River.Widths[0] = -1
	if s.ValidateOperation(broken) == nil {
		t.Fatal("invalid saved channel accepted")
	}
}
func TestLegacyRiverUpgradesOnceAndPersists(t *testing.T) {
	s := New(65, 65)
	op := Operation{ID: "old", Kind: "river", Layer: "water", Center: [2]float64{10, 10}, Radius: 2, Amount: 50, Path: [][2]float64{{10, 10}, {20, 12}}}
	s.AddOperation(op)
	s.Saved()
	s.PrepareLegacyRivers(riverWorld())
	if s.Operations["old"].River == nil || s.Parts(false)["builder/operation/old"] == nil || len(s.Header.History) != 0 {
		t.Fatal("legacy upgrade not persisted independently of object history")
	}
	s.Saved()
	s.PrepareLegacyRivers(riverWorld())
	if len(s.Parts(false)) != 1 {
		t.Fatal("repeated river migration")
	}
}
