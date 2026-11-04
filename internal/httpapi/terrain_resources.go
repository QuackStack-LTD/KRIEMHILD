package httpapi

import (
	"context"
	"fmt"
	"kriemhild/internal/history"
	terrain "kriemhild/internal/legacyterrain"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/resources"
	"kriemhild/internal/terrain/tiles"
	"kriemhild/internal/world"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type resourceSnapshot struct {
	id        string
	p         *model.Planet
	e         *terrain.Environment
	transform tiles.Transform
	derived   bool
	stale     bool
}
type resourceStatistic struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
}
type resourceWater struct {
	Rivers               []resourceRiver       `json:"rivers"`
	Lakes                []model.Lake          `json:"lakes"`
	Springs              []terrain.HydroSpring `json:"springs"`
	RechargeMMYear       float64               `json:"rechargeMmYear"`
	RechargeM3Year       float64               `json:"rechargeM3Year"`
	FreshSurfaceFraction float64               `json:"freshSurfaceFraction"`
	Explanation          string                `json:"explanation"`
}
type resourceRiver struct {
	ID                string  `json:"id"`
	DistanceKM        float64 `json:"distanceKm"`
	DischargeM3s      float64 `json:"dischargeM3s"`
	DrySeasonProxyM3s float64 `json:"drySeasonProxyM3s"`
	Destination       string  `json:"destination"`
}
type terrainResourceReport struct {
	Longitude       float64                       `json:"longitude"`
	Latitude        float64                       `json:"latitude"`
	RadiusKM        float64                       `json:"radiusKm"`
	AreaKM2         float64                       `json:"areaKm2"`
	MinimumRadiusKM float64                       `json:"minimumRadiusKm,omitempty"`
	MaximumRadiusKM float64                       `json:"maximumRadiusKm,omitempty"`
	Source          string                        `json:"source"`
	Generator       string                        `json:"generator"`
	Derived         bool                          `json:"derived"`
	Stale           bool                          `json:"stale"`
	RegionalCellKM  float64                       `json:"regionalCellKm"`
	Fields          map[string]resourceStatistic  `json:"fields"`
	Biomes          map[string]float64            `json:"biomes"`
	Geology         []terrain.GeologicalProvince  `json:"geology"`
	Formations      []terrain.GeologicalFormation `json:"formations"`
	Structures      []terrain.GeologicalStructure `json:"structures"`
	Resources       []terrain.ResourceOccurrence  `json:"resources"`
	Systems         []terrain.ResourceSystem      `json:"systems"`
	Districts       []terrain.ResourceDistrict    `json:"districts"`
	TotalResources  int                           `json:"totalResources"`
	Truncated       bool                          `json:"truncated"`
	FreshWater      resourceWater                 `json:"freshWater"`
	Biosphere       spotBiosphere                 `json:"biosphere"`
	Notes           []string                      `json:"notes"`
}

func queryNumber(r *http.Request, key string, def float64) (float64, error) {
	if !r.URL.Query().Has(key) {
		return def, nil
	}
	n, err := strconv.ParseFloat(r.URL.Query().Get(key), 64)
	if err != nil || !model.Finite(n) {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}
func readResourceCircle(r *http.Request, center model.Point, minimum, maximum float64) (model.Point, float64, error) {
	lon, err := queryNumber(r, "longitude", center[0]*180/math.Pi)
	if err != nil {
		return center, 0, err
	}
	lat, err := queryNumber(r, "latitude", center[1]*180/math.Pi)
	if err != nil {
		return center, 0, err
	}
	radius, err := queryNumber(r, "radiusKm", minimum)
	if err != nil || lon < -180 || lon > 180 || lat < -90 || lat > 90 || radius < minimum || radius > maximum {
		return center, 0, fmt.Errorf("circle coordinates or radius outside supported range")
	}
	return model.Point{lon * math.Pi / 180, lat * math.Pi / 180}, radius, nil
}

func (s *Server) sessionResources(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	maximum := 5000.
	if v.planet != nil {
		maximum = math.Min(maximum, math.Pi*v.planet.Spec.RadiusKM)
	}
	center, radius, err := readResourceCircle(r, model.Point{}, .05, maximum)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	env := v.solver.Environment
	derived := false
	if v.planet != nil && (env == nil || env.NaturalSource == "") {
		if v.resourcePlanet != v.planet || v.resourceEnvironment == nil {
			projected, e := legacyProjectionContext(r.Context(), v.planet)
			if e != nil {
				fail(w, 422, e.Error())
				return
			}
			v.resourcePlanet = v.planet
			v.resourceEnvironment = projected.Environment
		}
		env = v.resourceEnvironment
		derived = true
	}
	if env == nil || env.Geography == nil || env.Geology == nil || env.Resources == nil {
		fail(w, 409, "This terrain has no generated geological inventory or physical distance scale")
		return
	}
	var spots map[int]spotBiosphere
	if v.historical != nil && s.projects != nil {
		if d, e := s.projects.History(r.Context(), v.historyWorld); e == nil {
			spots = s.resourceBiosphere(r, d, v.historical.AgeID, v.historical.TerrainID)
		}
	}
	var transform tiles.Transform
	if v.world != nil && v.planet != nil {
		transform = continuousTransform(v.world.Ops(world.Bounds{Width: float64(v.solver.W - 1), Height: float64(v.solver.H - 1)}), v.solver.W, v.solver.H, v.planet)
	}
	out := buildResourceReport(r.Context(), v.planet, env, center, radius, spots, transform)
	out.Derived = derived
	if v.world != nil {
		for _, op := range v.world.Operations {
			if !op.Deleted {
				out.Stale = true
				break
			}
		}
	}
	respond(w, 200, out)
}

// One immutable saved snapshot is cached. Reading a profile never opens a
// builder session, commits a revision, activates a project or saves derived data.
func (s *Server) savedResourceSnapshot(ctx context.Context, id string) (*resourceSnapshot, error) {
	s.resourceMu.Lock()
	defer s.resourceMu.Unlock()
	if s.resourceSnapshot != nil && s.resourceSnapshot.id == id {
		return s.resourceSnapshot, nil
	}
	raw, err := s.projects.SnapshotPart(ctx, id, "environment")
	if err != nil {
		return nil, err
	}
	var e *terrain.Environment
	if err = expandJSON(raw, &e); err != nil || e == nil {
		return nil, fmt.Errorf("saved terrain environment unavailable")
	}
	var p *model.Planet
	meta, err := s.projects.SnapshotPart(ctx, id, "continuous/metadata")
	if err == nil {
		fields, e2 := s.projects.SnapshotPart(ctx, id, "continuous/fields")
		if e2 != nil {
			return nil, e2
		}
		p, err = model.Decode(meta, fields)
		if err != nil {
			return nil, err
		}
	}
	derived := p != nil && e.NaturalSource == ""
	if derived {
		projection, err := legacyProjectionContext(ctx, p)
		if err != nil {
			return nil, err
		}
		e = projection.Environment
	}
	if p != nil && e.NaturalSource != "" && e.NaturalSource != resources.Source(p) {
		return nil, fmt.Errorf("saved resource source does not match continuous terrain")
	}
	if e.Geography == nil || e.Geology == nil || e.Resources == nil {
		return nil, fmt.Errorf("saved terrain has no geological inventory or distance scale")
	}
	var transform tiles.Transform
	stale := false
	if p != nil {
		parts, err := s.projects.SnapshotPartsMatching(ctx, id, []string{"builder/header", "builder/operation/", "builder/entity/"})
		if err != nil {
			return nil, err
		}
		authored := world.New(e.Options.Columns, e.Options.Rows)
		if raw := parts["builder/header"]; raw != nil {
			if err := expandJSON(raw, &authored.Header); err != nil {
				return nil, err
			}
		}
		for key, raw := range parts {
			switch {
			case strings.HasPrefix(key, "builder/operation/"):
				var op world.Operation
				if err := expandJSON(raw, &op); err != nil {
					return nil, err
				}
				authored.Operations[op.ID] = op
			case strings.HasPrefix(key, "builder/entity/"):
				var entity world.Entity
				if err := expandJSON(raw, &entity); err != nil {
					return nil, err
				}
				authored.Entities[entity.ID] = entity
			}
		}
		authored.Reindex(e.Options.Columns, e.Options.Rows)
		ops := authored.Ops(world.Bounds{Width: float64(e.Options.Columns - 1), Height: float64(e.Options.Rows - 1)})
		stale = len(ops) > 0
		transform = continuousTransform(ops, e.Options.Columns, e.Options.Rows, p)
	}
	snapshot := &resourceSnapshot{id: id, p: p, e: e, transform: transform, derived: derived, stale: stale}
	s.resourceSnapshot = snapshot
	return snapshot, nil
}

func settlementResourceMap(d *history.Document, age, id string) (*history.Map, [][2]float64, error) {
	e := d.Entity(age, id)
	if e == nil || e.Deleted || history.Registry.Types[e.Kind].Category != "settlements" {
		return nil, nil, fmt.Errorf("choose a settlement, rather than a political territory")
	}
	// Prefer the most detailed saved footprint, falling back to its parent map.
	var selected *history.Map
	var points [][2]float64
	for _, rep := range d.Representations {
		if rep.AgeID != age || rep.EntityID != id || rep.Shape.Deleted || len(rep.Shape.Points) == 0 {
			continue
		}
		m := d.Map(age, rep.MapID)
		if m == nil || e.TerrainID != "" && m.TerrainID != e.TerrainID {
			continue
		}
		if selected == nil || m.EntityID == id || len(rep.Shape.Points) > len(points) {
			selected = m
			points = rep.Shape.Points
		}
	}
	if selected == nil {
		return nil, nil, fmt.Errorf("place this settlement on a saved terrain map to inspect its surroundings")
	}
	// A logical settlement can have outlines on several maps. Fit their union
	// on this terrain, rather than clipping a less detailed or detached part.
	points = nil
	for _, ring := range settlementResourceOutlines(d, age, id, selected.TerrainID) {
		if len(points)+len(ring) > 16384 {
			return nil, nil, fmt.Errorf("settlement footprint exceeds the supported point limit")
		}
		points = append(points, ring...)
	}
	return selected, points, nil
}

func settlementResourceOutlines(d *history.Document, age, id, terrainID string) [][][2]float64 {
	out := [][][2]float64{}
	for _, rep := range d.Representations {
		if rep.AgeID != age || rep.EntityID != id || rep.Shape.Deleted || len(rep.Shape.Points) == 0 {
			continue
		}
		m := d.Map(age, rep.MapID)
		if m != nil && m.TerrainID == terrainID {
			out = append(out, rep.Shape.Points)
		}
	}
	return out
}
func (s *Server) settlementResources(w http.ResponseWriter, r *http.Request) {
	if image := r.PathValue("image"); image != "" && image != "preview" {
		fail(w, 404, "Resource preview not found")
		return
	}
	if !s.historyStore(w) {
		return
	}
	d, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	m, points, err := settlementResourceMap(d, r.URL.Query().Get("age"), r.PathValue("entity"))
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	snapshot, err := s.savedResourceSnapshot(r.Context(), m.Snapshot)
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	center, minimum, err := settlementCircle(points, snapshot.e.Options.Columns, snapshot.e.Options.Rows, snapshot.e.Geography.Radius)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	if minimum > 50 {
		fail(w, 422, "This settlement's complete footprint needs a radius greater than the supported 50 km; it cannot be shown without clipping")
		return
	}
	radius, err := queryNumber(r, "radiusKm", minimum)
	if err != nil || radius < minimum-1e-7 || radius > 50 {
		fail(w, 400, "Observed radius must contain the complete settlement and cannot exceed 50 km")
		return
	}
	if r.PathValue("image") == "preview" {
		renderResourceCircle(w, r, snapshot, center, radius, settlementResourceOutlines(d, m.AgeID, r.PathValue("entity"), m.TerrainID))
		return
	}
	out := buildResourceReport(r.Context(), snapshot.p, snapshot.e, center, radius, s.resourceBiosphere(r, d, m.AgeID, m.TerrainID), snapshot.transform)
	out.Derived = snapshot.derived
	out.Stale = snapshot.stale
	out.MinimumRadiusKM = minimum
	out.MaximumRadiusKM = 50
	// Snapshot operations invalidate analyses; no authored height edit is used to
	// invent a new fossil history. Native base resources remain explicitly stale.
	if r.Context().Err() != nil {
		return
	}
	respond(w, 200, out)
}

func (s *Server) resourceBiosphere(r *http.Request, d *history.Document, age, terrainID string) map[int]spotBiosphere {
	spots := biosphereSpots(d, age, terrainID)
	if saved := d.Biosphere(age, terrainID); saved != nil {
		fresh, err := s.biosphereTerrain(r, d, age, terrainID)
		if err != nil || saved.Source != fresh.Source {
			for cell, b := range spots {
				b.Stale = true
				spots[cell] = b
			}
		}
	}
	return spots
}

func buildResourceReport(ctx context.Context, p *model.Planet, e *terrain.Environment, center model.Point, radius float64, spots map[int]spotBiosphere, transform tiles.Transform) terrainResourceReport {
	w, h := e.Options.Columns, e.Options.Rows
	planetRadius := e.Geography.Radius
	x, y := (center[0]+math.Pi)/model.Tau*float64(w-1), (.5-center[1]/math.Pi)*float64(h-1)
	cells := e.NaturalCellsWithinKM(x, y, radius)
	included := make(map[int]bool, len(cells))
	for _, i := range cells {
		included[i] = true
	}
	out := terrainResourceReport{Longitude: center[0] * 180 / math.Pi, Latitude: center[1] * 180 / math.Pi, RadiusKM: radius, AreaKM2: 2 * math.Pi * planetRadius * planetRadius * (1 - math.Cos(radius/planetRadius)), Source: e.NaturalSource, Generator: e.Resources.Generator, RegionalCellKM: math.Pi * planetRadius / float64(h-1), Fields: map[string]resourceStatistic{}, Geology: []terrain.GeologicalProvince{}, Formations: []terrain.GeologicalFormation{}, Structures: []terrain.GeologicalStructure{}, Resources: []terrain.ResourceOccurrence{}, Systems: []terrain.ResourceSystem{}, Districts: []terrain.ResourceDistrict{}, Biosphere: spotBiosphere{Habitats: []spotHabitat{}, Lifeforms: []spotLifeform{}}, Notes: []string{"Procedural, inferred regional geology and endowment, not measured deposits or recoverable reserves.", "Geological and habitat footprints use the saved parent grid; small circles can intersect the same regional range. Terrain imagery and river paths use the continuous snapshot.", "Whole deposit quantities describe the deposit, not the fraction inside the circle. All scalar field summaries use 256 equal-area samples; categorical field means have no physical interpretation."}}
	out.FreshWater = resourceWater{Rivers: []resourceRiver{}, Lakes: []model.Lake{}, Springs: []terrain.HydroSpring{}, Explanation: "River flows are saved model estimates, never added across connected reaches. Dry-season flows and aquifer recharge are procedural proxies, not potable water or sustainable abstraction yields. Spring discharge is a regional depth-flow proxy, not m³/s."}
	out.Biomes = map[string]float64{}
	populated := map[string]bool{}
	for _, key := range strings.Fields("elevation bathymetry temperature precipitation moisture latitude waterLevel waterDepth flow drainageElevation vegetation fertility aridity windX windY biome plate boundary convergence tectonicStress volcano crustAge mountainCore highland shelf slope sediment permeability lithology geothermal recharge runoff groundFlow baseflow waterTableDepth groundwaterHead dryDischarge river wetland salinity summer winter groundwater terminalBasin waterBody ocean lake windStrength accumulation catchmentArea spring pet") {
		populated[key] = true
	}
	if e.NaturalSource != "" {
		out.Notes = append(out.Notes, "native.* fields retain native-grid coordinates and units; surface.* describes continuous samples. Only populated compatibility fields are included; unimplemented legacy placeholders are omitted.")
	}
	provs, forms, faults := map[int]bool{}, map[int]bool{}, map[int]bool{}
	for _, i := range cells {
		c := e.Geology.Cells[i]
		provs[c.Province] = true
		for _, f := range []int{c.Cover, c.Basement, c.Intrusion} {
			if f >= 0 {
				forms[f] = true
			}
		}
		if c.Fault >= 0 {
			faults[c.Fault] = true
		}
	}
	for i, v := range e.Geology.Provinces {
		if provs[i] {
			v.Geometry = terrain.NaturalGeometry{}
			out.Geology = append(out.Geology, v)
		}
	}
	for i, v := range e.Geology.Formations {
		if forms[i] {
			v.Geometry = terrain.NaturalGeometry{}
			out.Formations = append(out.Formations, v)
		}
	}
	for i, v := range e.Geology.Structures {
		if faults[i] {
			v.Geometry = terrain.NaturalGeometry{}
			out.Structures = append(out.Structures, v)
		}
	}
	systems, districts := map[string]bool{}, map[string]bool{}
	for _, o := range e.Resources.Occurrences {
		overlap := false
		for _, i := range o.Geometry.Cells {
			if included[i] {
				overlap = true
				break
			}
		}
		if point := o.Geometry.Point; point != nil {
			overlap = e.PointDistanceKM(x, y, point[0], point[1]) <= radius
		}
		if !overlap {
			continue
		}
		out.TotalResources++
		if len(out.Resources) >= 512 {
			out.Truncated = true
			continue
		}
		systems[o.SystemID] = true
		districts[o.DistrictID] = true
		o.Geometry = terrain.NaturalGeometry{}
		out.Resources = append(out.Resources, o)
	}
	for _, v := range e.Resources.Systems {
		if systems[v.ID] {
			out.Systems = append(out.Systems, v)
		}
	}
	for _, v := range e.Resources.Districts {
		if districts[v.ID] {
			v.Geometry = terrain.NaturalGeometry{}
			v.Occurrences = nil
			out.Districts = append(out.Districts, v)
		}
	}
	var sampler *detail.Sampler
	if p != nil {
		sampler = detail.New(p)
	}
	const count = 256
	stat := func(key string, n float64) {
		v, exists := out.Fields[key]
		if !exists {
			v.Min = n
			v.Max = n
		}
		v.Min = math.Min(v.Min, n)
		v.Max = math.Max(v.Max, n)
		v.Mean += n / count
		out.Fields[key] = v
	}
	for sample := 0; sample < count; sample++ {
		if ctx.Err() != nil {
			break
		}
		// Equal area on a spherical cap, golden-angle bearings.
		radial := planetRadius * math.Acos(1-(float64(sample)+.5)/count*(1-math.Cos(radius/planetRadius)))
		pt := geoDestination(center, radial, float64(sample)*2.399963229728653, planetRadius)
		xx := int(math.Round((pt[0] + math.Pi) / model.Tau * float64(w-1)))
		yy := int(math.Round((.5 - pt[1]/math.Pi) * float64(h-1)))
		i := max(0, min(w*h-1, yy*w+xx))
		physical := map[string]float64{}
		if sampler != nil {
			s := sampler.Sample(pt[0], pt[1])
			if transform != nil {
				s = transform(s, pt[0], pt[1])
			}
			physical = map[string]float64{"elevation": s.Bed, "waterLevel": s.WaterSurface, "waterDepth": s.WaterDepth, "temperature": s.Temperature, "precipitation": s.Precipitation, "moisture": s.Moisture, "pet": s.PET}
			out.Biomes[s.Biome] += 1. / count
			stat("surface.coastDistanceKm", s.CoastDistance)
			stat("surface.roughness", s.Roughness)
			// Preserve native coordinates/units under their own namespace. In
			// particular, native receiver indices are not parent-grid indices.
			for key := range p.Grid.Fields {
				stat("native."+key, p.Grid.At(key, pt[0], pt[1]))
			}
		}
		for key, value := range physical {
			stat(key, value)
		}
		for k, a := range e.Fields {
			if e.NaturalSource != "" && !populated[k] {
				continue
			}
			if len(a) != w*h {
				continue
			}
			if _, ok := physical[k]; ok {
				continue
			}
			stat(k, a[i])
		}
		if i < len(e.Fields["recharge"]) {
			out.FreshWater.RechargeMMYear += e.Fields["recharge"][i] * 1000 / count
		}
		if sampler != nil {
			s := sampler.Sample(pt[0], pt[1])
			if transform != nil {
				s = transform(s, pt[0], pt[1])
			}
			if s.Water != "dry" && s.Water != "ocean" {
				out.FreshWater.FreshSurfaceFraction += 1. / count
			}
		}
	}
	out.FreshWater.RechargeM3Year = out.FreshWater.RechargeMMYear / 1000 * out.AreaKM2 * 1e6
	if p != nil {
		for _, river := range p.Rivers {
			hit := resourceRiver{ID: river.ID, DistanceKM: math.Inf(1), Destination: river.Destination}
			for j := 1; j < len(river.Path); j++ {
				distance := geoSegmentDistance(center, river.Path[j-1], river.Path[j], planetRadius)
				if distance > radius {
					continue
				}
				hit.DistanceKM = math.Min(hit.DistanceKM, distance)
				a, b := river.Path[j-1], river.Path[j]
				lo, hi := geoSegmentInterval(center, a, b, radius, planetRadius)
				for _, t := range []float64{lo, (lo + hi) / 2, hi} {
					pt := geoDestination(a, geoDistance(a, b, planetRadius)*t, geoBearing(a, b), planetRadius)
					if geoDistance(center, pt, planetRadius) > radius+1e-7 {
						continue
					}
					flow := river.Discharge[j-1] + (river.Discharge[j]-river.Discharge[j-1])*t
					hit.DischargeM3s = math.Max(hit.DischargeM3s, flow)
					dry := model.Clamp(p.Grid.At("rainDriest", pt[0], pt[1])*12/math.Max(1, p.Grid.At("precipitation", pt[0], pt[1])), .03, 1)
					hit.DrySeasonProxyM3s = math.Max(hit.DrySeasonProxyM3s, flow*dry)
				}
			}
			if model.Finite(hit.DistanceKM) {
				out.FreshWater.Rivers = append(out.FreshWater.Rivers, hit)
			}
		}
		for _, lake := range p.Lakes {
			if geoDistance(center, lake.Center, planetRadius) <= radius+lake.Radius*planetRadius {
				out.FreshWater.Lakes = append(out.FreshWater.Lakes, lake)
			}
		}
	}
	if e.Hydrology != nil {
		for _, spring := range e.Hydrology.Springs {
			if included[spring.Cell] {
				out.FreshWater.Springs = append(out.FreshWater.Springs, spring)
			}
		}
	}
	habitats, life := map[string]spotHabitat{}, map[string]spotLifeform{}
	for _, i := range cells {
		b := spots[i]
		out.Biosphere.Available = out.Biosphere.Available || b.Available
		out.Biosphere.Stale = out.Biosphere.Stale || b.Stale
		for _, v := range b.Habitats {
			habitats[v.ID] = v
		}
		for _, v := range b.Lifeforms {
			if old, ok := life[v.ID]; !ok || v.Abundance > old.Abundance {
				life[v.ID] = v
			}
		}
	}
	for _, v := range habitats {
		out.Biosphere.Habitats = append(out.Biosphere.Habitats, v)
	}
	for _, v := range life {
		out.Biosphere.Lifeforms = append(out.Biosphere.Lifeforms, v)
	}
	sort.Slice(out.Biosphere.Habitats, func(i, j int) bool { return out.Biosphere.Habitats[i].ID < out.Biosphere.Habitats[j].ID })
	sort.Slice(out.Biosphere.Lifeforms, func(i, j int) bool {
		a, b := out.Biosphere.Lifeforms[i], out.Biosphere.Lifeforms[j]
		if a.Group != b.Group {
			return a.Group < b.Group
		}
		return a.ID < b.ID
	})
	return out
}
