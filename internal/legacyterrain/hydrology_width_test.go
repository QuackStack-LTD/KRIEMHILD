package terrain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestGeologicalChannelWidthsAreSeededAndPhysical(t *testing.T) {
	e := branchTestWorld(t)
	for i := range e.Mask {
		e.set("slope", i, 0)
		e.set("mountainCore", i, 0)
	}
	// Change bank rock while holding location, flow, gradient and seed constant.
	for i := range e.Geology.Formations {
		e.Geology.Formations[i].Rock = "granite"
	}
	e.prepareChannelGeology()
	narrow, _ := e.ChannelDimensions(80, 81, 300, 1, .5)
	for i := range e.Geology.Formations {
		e.Geology.Formations[i].Rock = "alluvium"
	}
	e.prepareChannelGeology()
	broad, _ := e.ChannelDimensions(80, 81, 300, 1, .5)
	if broad < narrow*2 {
		t.Fatal("bank geology did not change physical width", narrow, broad)
	}
	before := append([]float64(nil), e.Fields["channelWidthFactor"]...)
	e.prepareChannelGeology()
	if !reflect.DeepEqual(before, e.Fields["channelWidthFactor"]) {
		t.Fatal("widths rerolled for same seed")
	}
	e.Options.Seed += "-different"
	e.prepareChannelGeology()
	if reflect.DeepEqual(before, e.Fields["channelWidthFactor"]) {
		t.Fatal("seed does not affect geological fabric")
	}
	thin := DetailFeature{Path: [][3]float64{{4, 5, 100}, {6, 5, 95}}, Width: .01, ValleyWidth: .3, Discharge: 300}
	wide := thin
	wide.Width = .15
	narrowBed, _ := ChannelBed(SampleChannel(thin, 5, 5.05))
	wideBed, _ := ChannelBed(SampleChannel(wide, 5, 5.05))
	if wideBed >= narrowBed-10 {
		t.Fatal("width changed only the drawing, not channel terrain", narrowBed, wideBed)
	}
}

func TestGeneratedWidthProfilesAndGeologySurviveRestoration(t *testing.T) {
	// A large catchment is necessary to exercise confluence widths. Several
	// separated continents at this tiny resolution only contain short streams.
	e := BuildEnvironment(realismOptions(t, `{"columns":48,"rows":32,"majorContinents":1,"seed":"river-thickness","rainfall":1.8}`))
	if len(e.Hydrology.Reaches) < 10 {
		t.Fatal("insufficient fixture rivers")
	}
	factors := map[string]float64{}
	minimum, maximum := math.Inf(1), 0.
	for _, r := range e.Hydrology.Reaches {
		if r.BankWidthFactor <= 0 || r.WidthMetres <= 0 {
			t.Fatal("missing persisted physical width", r.ID)
		}
		m, c := e.channelDimensions(r.From, r.To)
		if math.Abs(m-r.WidthMetres) > 1e-8 || math.Abs(c-r.Width) > 1e-10 {
			t.Fatal("stored reach disagrees with physical model")
		}
		factors[r.RiverID] = r.BankWidthFactor
		minimum = math.Min(minimum, r.WidthMetres)
		maximum = math.Max(maximum, r.WidthMetres)
	}
	if len(factors) < 3 || maximum/minimum < 6 {
		t.Fatal("width distribution collapsed", len(factors), minimum, maximum)
	}
	m := NewDetailModel(e)
	features := map[string]DetailFeature{}
	for _, feature := range m.Features {
		features[feature.ID] = feature
	}
	reaches := map[string]HydroReach{}
	for _, reach := range e.Hydrology.Reaches {
		reaches[reach.ID] = reach
	}
	heads, joins := 0, 0
	for _, reach := range e.Hydrology.Reaches {
		if len(reach.Upstream) == 0 {
			heads++
			feature := features[reach.ID]
			if reach.SourceWidthMetres <= 0 || reach.SourceWidthMetres > 8 || reach.WidthMetres > 41.600001 || reach.SourceWidthMetres >= reach.WidthMetres*.1 || len(feature.Widths) < 2 {
				t.Fatal("headwater starts as a broad river", reach.ID)
			}
			metres := feature.Widths[0] / feature.Width * feature.PhysicalWidthMetres
			share := 1.
			for _, branch := range reach.Branches {
				if len(branch.Path) == 0 {
					share -= branch.Fraction
				}
			}
			if math.Abs(metres-reach.SourceWidthMetres*math.Pow(share, .56)) > 1e-7 || feature.Widths[0] >= feature.Widths[len(feature.Widths)-1] {
				t.Fatal("source taper lost in physical/rendered channel", reach.ID, metres)
			}
		}
		if len(reach.Upstream) > 1 {
			joins++
			for _, id := range reach.Upstream {
				if reach.WidthMetres+1e-7 < reaches[id].WidthMetres {
					t.Fatal("confluence narrows below an incoming stream", reach.ID, id)
				}
			}
		}
	}
	if heads < 3 || joins == 0 {
		t.Fatal("fixture lacks sources or confluences", heads, joins)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var saved Environment
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreDetailModel(&saved, m.ProjectState())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.Hydrology.Reaches, saved.Hydrology.Reaches) || !reflect.DeepEqual(m.Features, restored.Features) || !reflect.DeepEqual(e.Fields["channelWidthFactor"], saved.Fields["channelWidthFactor"]) {
		t.Fatal("saved width geometry changed")
	}
	// Missing detail uses persisted reach widths, not the current width formula.
	for i := range saved.Fields["channelWidthFactor"] {
		saved.Fields["channelWidthFactor"][i] *= 10
	}
	rebuilt := NewDetailModel(&saved)
	byID := map[string]DetailFeature{}
	for _, f := range rebuilt.Features {
		byID[f.ID] = f
	}
	for _, f := range m.Features {
		if f.Class != "" && f.PhysicalWidthMetres > 0 {
			if byID[f.ID].Width != f.Width || !reflect.DeepEqual(byID[f.ID].Widths, f.Widths) {
				t.Fatal("stored river width regenerated", f.ID)
			}
		}
	}
}
