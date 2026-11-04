package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	terrain "kriemhild/internal/legacyterrain"
	"kriemhild/internal/terrain/generate"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/resources"
	"kriemhild/internal/world"
	"math"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestResourceRenewalMultiSeed(t *testing.T) {
	for _, seed := range []string{"resource-causal-0", "resource-causal-1", "resource-causal-2"} {
		t.Run(seed, func(t *testing.T) {
			spec := model.DefaultSpec()
			spec.Seed = seed
			spec.SurfaceWidth = 1024
			p, err := generate.Generate(context.Background(), spec, nil)
			if err != nil {
				t.Fatal(err)
			}
			before := resources.Source(p)
			solver, err := legacyProjection(p)
			if err != nil {
				t.Fatal(err)
			}
			e := solver.Environment
			area := 0.
			for i := range e.Mask {
				area += e.GeographicCellAreaKM2(i)
			}
			if math.Abs(area-4*math.Pi*spec.RadiusKM*spec.RadiusKM) > area*1e-10 {
				t.Fatal("native seam/polar footprints double-count planet area")
			}
			boundaryCells := e.NaturalCellsWithinKM(80.5, 44, .05)
			if !reflect.DeepEqual(boundaryCells, []int{44*192 + 80, 44*192 + 81}) {
				t.Fatalf("native vertex CRS shifted circle %v", boundaryCells)
			}
			if resources.Source(p) != before {
				t.Fatal("resource integration mutated physical terrain")
			}
			if err = e.ValidateNaturalData(); err != nil {
				t.Fatal(err)
			}
			if diagnostics := e.ValidateHydrology(); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			counts := map[string]int{}
			common, rare, shared := 0, 0, 0
			quantities := map[float64]bool{}
			for _, o := range e.Resources.Occurrences {
				if (o.Type == "solar-energy" || o.Type == "timber" || o.Type == "freshwater") && strings.Contains(o.QuantityUnit, "tonnes") {
					t.Fatal("renewable or ecological potential misrepresented as mineral tonnage")
				}
				counts[o.Type]++
				quantities[o.Quantity] = true
				if o.Type == "industrial-clay" || o.Type == "sand-gravel" || o.Type == "limestone" || o.Type == "construction-stone" {
					common += len(o.Geometry.Cells)
				}
				if o.Type == "diamonds" || o.Type == "rare-earth-elements" || o.Type == "platinum-group-metals" {
					rare += len(o.Geometry.Cells)
				}
				if o.SystemID == "" || o.DistrictID == "" || o.QuantityUnit == "" || o.GradeUnit == "" || o.ExtentKM2 <= 0 {
					t.Fatalf("missing hierarchy/endowment: %s", o.ID)
				}
			}
			for _, s := range e.Resources.Systems {
				if len(s.Commodities) > 1 {
					shared++
				}
			}
			if common <= rare || len(quantities) < 10 || shared == 0 {
				t.Fatalf("implausibly uniform inventory: common=%d rare=%d sizes=%d associations=%d", common, rare, len(quantities), shared)
			}
			t.Logf("source=%s occurrences=%d systems=%d districts=%d shared systems=%d coverage common/rare=%d/%d counts=%v", before, len(e.Resources.Occurrences), len(e.Resources.Systems), len(e.Resources.Districts), shared, common, rare, counts)
			// A second independent execution must preserve full hierarchy, provenance,
			// endowment and geometry, not just resource counts or a rendered image.
			golden, _ := json.Marshal(struct {
				G *terrain.GeologicalState
				R *terrain.ResourceState
				H *terrain.HydrologyState
			}{e.Geology, e.Resources, e.Hydrology})
			if err = resources.Populate(context.Background(), p, e); err != nil {
				t.Fatal(err)
			}
			replay, _ := json.Marshal(struct {
				G *terrain.GeologicalState
				R *terrain.ResourceState
				H *terrain.HydrologyState
			}{e.Geology, e.Resources, e.Hydrology})
			if !bytes.Equal(golden, replay) {
				t.Fatal("resource replay was not byte deterministic")
			}
			if resources.Source(p) != before {
				t.Fatal("replay mutated physical terrain")
			}
			// Render settings/timings do not invalidate geology; physical edits do.
			p.Diagnostics.Timings["test"] = 123
			p.Spec.TileSize = 512
			if resources.Source(p) != before {
				t.Fatal("render/timing affected resource dependency")
			}
			p.Grid.Fields["temperature"][0]++
			if resources.Source(p) == before {
				t.Fatal("physical dependency failed to invalidate resources")
			}
		})
	}
}

func TestSettlementResourceCircleFitsIrregularAndSeamShapes(t *testing.T) {
	for name, points := range map[string][][2]float64{
		"irregular":     {{80, 44}, {80.03, 44}, {80.03, 44.005}, {80.005, 44.005}, {80.005, 44.04}, {80, 44.04}},
		"seam":          {{190.99, 43.99}, {.01, 43.99}, {.01, 44.01}, {190.99, 44.01}},
		"point":         {{80, 44}},
		"high-latitude": {{80, 1}, {80.2, 1}, {80.2, 1.02}, {80, 1.02}},
	} {
		t.Run(name, func(t *testing.T) {
			center, r, err := settlementCircle(points, 192, 96, 6371)
			if err != nil {
				t.Fatal(err)
			}
			if r < .05 || r > 50 {
				t.Fatalf("bad fit radius %g", r)
			}
			for _, v := range points {
				pt := model.Point{v[0]/191*model.Tau - math.Pi, math.Pi/2 - v[1]/95*math.Pi}
				if geoDistance(center, pt, 6371) >= r {
					t.Fatal("outline clipped")
				}
			}
			c2, r2, _ := settlementCircle(points, 192, 96, 6371)
			if c2 != center || r != r2 {
				t.Fatal("unstable fitting")
			}
		})
	}
	center := model.Point{math.Pi - .001, .6}
	b := geoDestination(center, 5, math.Pi/2, 6371)
	if math.Abs(geoDistance(center, b, 6371)-5) > 1e-8 {
		t.Fatal("spherical distance")
	}
	// Crossing river segment must intersect even if both endpoints are outside.
	a, b := geoDestination(center, 20, math.Pi/2, 6371), geoDestination(center, 20, -math.Pi/2, 6371)
	if geoSegmentDistance(center, a, b, 6371) > 1e-6 {
		t.Fatal("segment crossing circle missed")
	}
	lo, hi := geoSegmentInterval(center, a, b, 1, 6371)
	if math.Abs(lo-.475) > 1e-7 || math.Abs(hi-.525) > 1e-7 {
		t.Fatalf("outside endpoint flow leaks into the circle: %g %g", lo, hi)
	}
}

func TestResourceRoutesReadOnlyAuthorizationAndSavedPreview(t *testing.T) {
	s, db, owner, stranger, doc := secureFixture(t)
	ctx := context.Background()
	spec := model.DefaultSpec()
	spec.SurfaceWidth = 1024
	p, err := generate.Generate(ctx, spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	solver, err := legacyProjection(p)
	if err != nil {
		t.Fatal(err)
	}
	v := &session{creatorID: owner.u.ID, planet: p, solver: solver, worldID: token(), last: time.Now(), undo: map[string]terrain.Snapshot{}, cacheRoot: t.TempDir()}
	s.sessions["resources"] = v
	// Existing save/accept path establishes actual immutable project snapshots.
	accept := secured(s, owner, "POST", "/api/worlds/"+doc.World.ID+"/accept", `{"session":"resources","ageId":"`+doc.World.CurrentAge+`","name":"Planet"}`)
	if accept.Code != 201 {
		t.Fatalf("accept %d %s", accept.Code, accept.Body)
	}
	ownerCtx := ctx
	doc, err = db.History(ownerCtx, doc.World.ID)
	if err != nil {
		t.Fatal(err)
	}
	age := doc.World.CurrentAge
	m := doc.Maps[0]
	doc, err = db.UpdateHistory(ownerCtx, doc.World.ID, doc.World.Revision, func(d *history.Document) error {
		d.Entities = append(d.Entities, history.Entity{ID: "town", Name: "Resource town", Kind: "town", AgeID: age, TerrainID: m.TerrainID, HomeMap: m.ID, Properties: json.RawMessage(`{}`)}, history.Entity{ID: "empire", Name: "Empire", Kind: "empire", AgeID: age, TerrainID: m.TerrainID, Properties: json.RawMessage(`{}`)})
		d.Representations = append(d.Representations, history.Representation{ID: "town-shape", AgeID: age, MapID: m.ID, EntityID: "town", Shape: world.Entity{ID: "town-shape", LogicalID: "town", Kind: "town", Name: "Resource town", Layer: "settlements", Geometry: "polygon", Color: "#c8a976", Points: [][2]float64{{80, 44}, {80.02, 44}, {80.02, 44.02}, {80, 44.02}}}})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	revision := doc.World.Revision
	path := "/api/sessions/resources/terrain/resources?longitude=0&latitude=0&radiusKm=10"
	profile := fmt.Sprintf("/api/worlds/%s/settlements/town/resources?age=%s", doc.World.ID, age)
	for _, path := range []string{path, profile, strings.Replace(profile, "/resources?", "/resources/preview?", 1)} {
		if w := secured(s, accountFixture{}, "GET", path, ""); w.Code != 401 {
			t.Fatalf("anonymous access %d", w.Code)
		}
		if w := secured(s, stranger, "GET", path, ""); w.Code != 404 && w.Code != 403 {
			t.Fatalf("private access %d %s", w.Code, w.Body)
		}
	}
	first := secured(s, owner, "GET", path, "")
	second := secured(s, owner, "GET", path, "")
	if first.Code != 200 || !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatalf("unstable/failed report %d %s", first.Code, first.Body)
	}
	for _, query := range []string{"radiusKm=NaN", "radiusKm=-1", "latitude=91", "longitude=181", "radiusKm=5001"} {
		path := "/api/sessions/resources/terrain/resources?" + query
		if w := secured(s, owner, "GET", path, ""); w.Code != 400 {
			t.Errorf("accepted %s: %d", query, w.Code)
		}
	}
	w := secured(s, owner, "GET", profile, "")
	if w.Code != 200 {
		t.Fatalf("profile %d %s", w.Code, w.Body)
	}
	var report terrainResourceReport
	if err = json.Unmarshal(w.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.MinimumRadiusKM <= 0 || report.MaximumRadiusKM != 50 {
		t.Fatal("missing radius constraints")
	}
	if _, ok := report.Fields["native.pet"]; !ok || math.Abs(report.Fields["native.pet"].Mean-report.Fields["pet"].Mean) > 1e-8 {
		t.Fatal("native generated climate fields missing or PET replaced by placeholder")
	}
	if _, ok := report.Fields["glacierMelt"]; ok {
		t.Fatal("unimplemented legacy placeholder reported as a generated measurement")
	}
	biomeShare := 0.
	for _, share := range report.Biomes {
		biomeShare += share
	}
	if math.Abs(biomeShare-1) > 1e-10 {
		t.Fatal("surface biome sampling incomplete")
	}
	for _, radius := range []float64{report.MinimumRadiusKM, 50} {
		url := strings.Replace(profile, "/resources?", "/resources/preview?", 1) + fmt.Sprintf("&radiusKm=%.17g", radius)
		w = secured(s, owner, "GET", url, "")
		if w.Code != 200 {
			t.Fatalf("preview: %d %s", w.Code, w.Body)
		}
		img, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
		if err != nil || img.Bounds().Dx() != 384 {
			t.Fatalf("bad image %v", err)
		}
		_, _, _, alpha := img.At(0, 0).RGBA()
		if alpha != 0 {
			t.Fatal("preview isn't circular")
		}
	}
	if w = secured(s, owner, "GET", profile+"&radiusKm=0.001", ""); w.Code != 400 {
		t.Fatal("clipped settlement accepted")
	}
	if w = secured(s, owner, "GET", profile+"&radiusKm=50.01", ""); w.Code != 400 {
		t.Fatal("over 50 km accepted")
	}
	if w = secured(s, owner, "GET", strings.Replace(profile, "/town/", "/empire/", 1), ""); w.Code != 404 {
		t.Fatal("political territory got settlement resources")
	}
	after, _ := db.History(ctx, doc.World.ID)
	if after.World.Revision != revision || v.unsaved || v.dirty {
		t.Fatal("resource inspection changed saved author data")
	}
	share(t, db, owner, stranger, doc.World.ID, "viewer")
	if w = secured(s, stranger, "GET", profile, ""); w.Code != 200 {
		t.Fatalf("viewer cannot inspect %d %s", w.Code, w.Body)
	}
}

func TestResourceBiosphereIncludesAllPresentGroupsByRange(t *testing.T) {
	d := history.New("Biosphere ranges")
	age := d.World.CurrentAge
	state := biosphere.State{AgeID: age, TerrainID: "terrain", Habitats: []biosphere.Habitat{{ID: "habitat", Name: "Local range", Cells: []int{1, 2}}}}
	for i, group := range []string{"plant", "animal", "fungus", "bacterium", "archaea", "protist", "virus", "algae"} {
		id := fmt.Sprint(i)
		d.Entities = append(d.Entities, history.Entity{ID: id, AgeID: age, Name: group, Biology: &biosphere.Lifeform{Group: group}})
		state.Occurrences = append(state.Occurrences, biosphere.Occurrence{SpeciesID: id, HabitatID: "habitat", Abundance: .5, Status: "present"})
	}
	state.Occurrences = append(state.Occurrences, biosphere.Occurrence{SpeciesID: "absent", HabitatID: "habitat", Abundance: 0, Status: "absent"})
	d.Biospheres = []biosphere.State{state}
	spots := biosphereSpots(d, age, "terrain")
	if len(spots[1].Lifeforms) != 8 || !reflect.DeepEqual(spots[1].Lifeforms, spots[2].Lifeforms) || len(spots[3].Lifeforms) != 0 {
		t.Fatal("Biosphere ranges lost/misplaced lifeforms")
	}
}

func TestResourceCircleBadQuery(t *testing.T) {
	for _, query := range []string{"radiusKm=Inf", "longitude=NaN", "latitude=-91"} {
		r := httptest.NewRequest("GET", "/?"+query, nil)
		if _, _, err := readResourceCircle(r, model.Point{}, .05, 50); err == nil {
			t.Fatal("invalid query accepted")
		}
	}
}

func TestSettlementResourceFitIncludesEveryMappedPart(t *testing.T) {
	d := history.New("Detached settlement")
	age := d.World.CurrentAge
	d.Entities = []history.Entity{{ID: "town", AgeID: age, Kind: "town"}}
	d.Maps = []history.Map{{ID: "root", AgeID: age, TerrainID: "terrain"}, {ID: "detail", AgeID: age, TerrainID: "terrain", EntityID: "town"}}
	d.Representations = []history.Representation{{AgeID: age, EntityID: "town", MapID: "root", Shape: world.Entity{Points: [][2]float64{{80, 44}, {80.01, 44}, {80, 44.01}}}}, {AgeID: age, EntityID: "town", MapID: "detail", Shape: world.Entity{Points: [][2]float64{{80.1, 44}, {80.11, 44}, {80.1, 44.01}}}}}
	m, points, err := settlementResourceMap(d, age, "town")
	if err != nil || m == nil || len(points) != 6 {
		t.Fatalf("mapped part disappeared: %v %v", points, err)
	}
	center, r, err := settlementCircle(points, 192, 96, 6371)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range points {
		if geoDistance(center, model.Point{p[0]/191*model.Tau - math.Pi, math.Pi/2 - p[1]/95*math.Pi}, 6371) >= r {
			t.Fatal("detached settlement part clipped")
		}
	}
}
