package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/history"
	terrain "kriemhild/internal/legacyterrain"
	"kriemhild/internal/resourcesurvey"
	"kriemhild/internal/storage"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func (s *Server) resourceArchiveRoutes() {
	s.mux.HandleFunc("GET /api/sessions/{id}/terrain/resource-archives", s.resourceArchiveChoices)
	s.mux.HandleFunc("POST /api/sessions/{id}/terrain/resource-reports", s.archiveResourceReport)
	s.mux.HandleFunc("GET /api/worlds/{world}/settlements/{entity}/resource-archives", s.resourceArchiveChoices)
	s.mux.HandleFunc("POST /api/worlds/{world}/settlements/{entity}/resource-reports", s.archiveResourceReport)
	s.mux.HandleFunc("GET /api/worlds/{world}/resource-surveys", s.reviewResourceSurveys)
}

func (s *Server) captureSavedResourceInput(w http.ResponseWriter, r *http.Request) *resourceInput {
	if !s.historyStore(w) {
		return nil
	}
	d, err := s.projects.AgeMapProjection(r.Context(), r.PathValue("world"), r.URL.Query().Get("age"))
	if err != nil {
		fail(w, 404, "World or Age not found")
		return nil
	}
	m, _, err := settlementResourceMap(d, r.URL.Query().Get("age"), r.PathValue("entity"))
	if err != nil {
		fail(w, 404, err.Error())
		return nil
	}
	snapshot, err := s.savedResourceSnapshot(r.Context(), m.Snapshot)
	if err != nil {
		fail(w, 409, err.Error())
		return nil
	}
	return &resourceInput{p: snapshot.p, e: snapshot.e, transform: snapshot.transform, derived: snapshot.derived, stale: snapshot.stale, context: &resourceReportContext{WorldID: d.World.ID, AgeID: m.AgeID, TerrainID: m.TerrainID, MapID: m.ID, Snapshot: m.Snapshot}}
}

type resourceArchiveChoice struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Location   string   `json:"location,omitempty"`
	DistanceKM *float64 `json:"distanceKm"`
}

func resourceArchiveLocations(d *history.Document, age, terrainID, id string, seen map[string]bool) []history.Entity {
	if seen[id] {
		return nil
	}
	seen[id] = true
	e := d.Entity(age, id)
	if e == nil || e.Deleted {
		return nil
	}
	out := []history.Entity{}
	if history.Registry.Types[e.Kind].Category == "settlements" && (e.TerrainID == "" || e.TerrainID == terrainID) {
		out = append(out, *e)
	}
	for _, r := range d.Relations {
		if r.AgeID == age && r.Source == id && (r.Kind == "archive-located-at" || r.Kind == "located-within" || r.Kind == "headquartered-in" || r.Kind == "resides-in") {
			out = append(out, resourceArchiveLocations(d, age, terrainID, r.Destination, seen)...)
		}
	}
	if e.Archive != nil && e.Archive.OwnerID != "" {
		out = append(out, resourceArchiveLocations(d, age, terrainID, e.Archive.OwnerID, seen)...)
	}
	if e.Kind == "archive" && history.ArchiveParent(*e) != "" {
		out = append(out, resourceArchiveLocations(d, age, terrainID, history.ArchiveParent(*e), seen)...)
	}
	return out
}
func resourceArchiveChoices(d *history.Document, age, terrainID string, circle resourcesurvey.Circle) []resourceArchiveChoice {
	out := []resourceArchiveChoice{}
	var width, height int
	for _, t := range d.Terrains {
		if t.AgeID == age && t.ID == terrainID {
			width, height = t.Width, t.Height
		}
	}
	for _, a := range d.Entities {
		if a.AgeID != age || a.Deleted || a.Kind != "archive" {
			continue
		}
		var props struct {
			ResourceFolder string `json:"resourceFolder"`
		}
		_ = json.Unmarshal(a.Properties, &props)
		if props.ResourceFolder != "" {
			continue
		}
		c := resourceArchiveChoice{ID: a.ID, Name: a.Name, Path: a.Name}
		seen := map[string]bool{a.ID: true}
		parent := history.ArchiveParent(a)
		for parent != "" && !seen[parent] {
			seen[parent] = true
			p := d.Entity(age, parent)
			if p == nil {
				break
			}
			c.Path = p.Name + " / " + c.Path
			parent = history.ArchiveParent(*p)
		}
		for _, place := range resourceArchiveLocations(d, age, terrainID, a.ID, map[string]bool{}) {
			_, points, err := settlementResourceMap(d, age, place.ID)
			if err != nil || width < 2 || height < 2 {
				continue
			}
			center, _, err := settlementCircle(points, width, height, circle.PlanetRadiusKM)
			if err != nil {
				continue
			}
			distance := resourcesurvey.Distance(circle.Longitude, circle.Latitude, center[0]*180/math.Pi, center[1]*180/math.Pi, circle.PlanetRadiusKM)
			if c.DistanceKM == nil || distance < *c.DistanceKM {
				c.DistanceKM = &distance
				c.Location = place.Name
			}
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.DistanceKM == nil) != (b.DistanceKM == nil) {
			return a.DistanceKM != nil
		}
		if a.DistanceKM != nil && *a.DistanceKM != *b.DistanceKM {
			return *a.DistanceKM < *b.DistanceKM
		}
		return a.Path < b.Path
	})
	return out
}
func (s *Server) resourceArchiveChoices(w http.ResponseWriter, r *http.Request) {
	in := s.captureResourceInput(w, r)
	if in == nil {
		return
	}
	if in.context == nil || s.projects == nil {
		fail(w, 409, "Save this terrain in a World before archiving reports")
		return
	}
	center, radius, err := readResourceCircle(r, model.Point{}, .05, math.Min(5000, math.Pi*in.e.Geography.Radius))
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	d, err := s.projects.ResourceArchives(r.Context(), in.context.WorldID, in.context.AgeID, true)
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	c := resourcesurvey.Circle{Longitude: center[0] * 180 / math.Pi, Latitude: center[1] * 180 / math.Pi, RadiusKM: radius, PlanetRadiusKM: in.e.Geography.Radius}
	respond(w, 200, map[string]any{"archives": resourceArchiveChoices(d, in.context.AgeID, in.context.TerrainID, c), "age": d.Age(in.context.AgeID)})
}

func canonicalResourceSamples(ctx context.Context, in *resourceInput, report terrainResourceReport, spots map[int]spotBiosphere) (resourcesurvey.Manifest, map[string][]byte, error) {
	c := resourcesurvey.Circle{Longitude: report.Longitude, Latitude: report.Latitude, RadiusKM: report.RadiusKM, PlanetRadiusKM: in.e.Geography.Radius}
	m := resourcesurvey.Manifest{Version: resourcesurvey.Version, Level: resourcesurvey.Level(c), Cells: map[string]string{}, Resources: map[string]string{}}
	blobs := map[string][]byte{}
	stored := report
	stored.Context = nil
	stored.Fingerprint = ""
	if report.Biosphere.Available {
		bio := report.Biosphere
		bio.WorldID = ""
		bio.AgeID = ""
		m.ReportBiosphere = resourcesurvey.Put(blobs, bio)
		stored.Biosphere.Habitats = nil
		stored.Biosphere.Lifeforms = nil
		stored.Biosphere.WorldID = ""
		stored.Biosphere.AgeID = ""
	}
	m.Report = resourcesurvey.Put(blobs, stored)
	cells := resourcesurvey.Cells(c, m.Level)
	if len(cells) > 32768 {
		return m, nil, fmt.Errorf("Observation sample budget exceeded")
	}
	w, h := in.e.Options.Columns, in.e.Options.Rows
	parents := map[int]bool{}
	for _, cell := range cells {
		lon, lat := cell.Center()
		x := int(math.Round((lon + 180) / 360 * float64(w-1)))
		y := int(math.Round((.5 - lat/180) * float64(h-1)))
		parents[max(0, min(w*h-1, y*w+x))] = true
	}
	byParent := map[int]map[string]string{}
	communities := map[string]bool{}
	bioParents := map[int]string{}
	for i := range parents {
		bio := spots[i]
		if !bio.Available {
			continue
		}
		bio.WorldID = ""
		bio.AgeID = ""
		h := resourcesurvey.Put(blobs, bio)
		bioParents[i] = h
		communities[h] = true
	}
	for h := range communities {
		m.Communities = append(m.Communities, h)
	}
	sort.Strings(m.Communities)
	byCell := map[string]map[string]string{}
	includedCells := map[string]bool{}
	for _, cell := range cells {
		includedCells[cell.Key()] = true
	}
	for _, o := range in.e.Resources.Occurrences {
		geometry := o.Geometry
		o.Geometry = terrain.NaturalGeometry{}
		hash := ""
		add := func(i int) {
			if !parents[i] {
				return
			}
			if hash == "" {
				hash = resourcesurvey.Put(blobs, o)
			}
			if byParent[i] == nil {
				byParent[i] = map[string]string{}
			}
			byParent[i][o.ID] = hash
		}
		if geometry.Point != nil {
			lon, lat := geometry.Point[0]/float64(w-1)*360-180, 90-geometry.Point[1]/float64(h-1)*180
			key := resourcesurvey.At(m.Level, lon, lat).Key()
			if includedCells[key] {
				if byCell[key] == nil {
					byCell[key] = map[string]string{}
				}
				byCell[key][o.ID] = resourcesurvey.Put(blobs, o)
			}
		} else {
			for _, i := range geometry.Cells {
				add(i)
			}
		}
	}
	// Keep only occurrence values referenced by observed cells in the manifest.
	// Unreferenced temporary values are discarded before the transaction.
	var sampler *detail.Sampler
	if in.p != nil {
		sampler = detail.New(in.p)
	}
	for _, cell := range cells {
		if err := ctx.Err(); err != nil {
			return m, nil, err
		}
		lon, lat := cell.Center()
		x := int(math.Round((lon + 180) / 360 * float64(w-1)))
		y := int(math.Round((.5 - lat/180) * float64(h-1)))
		i := max(0, min(w*h-1, y*w+x))
		refs := map[string]string{}
		for id, h := range byParent[i] {
			refs[id] = h
		}
		for id, h := range byCell[cell.Key()] {
			refs[id] = h
		}
		state := resourcesurvey.State{Fields: map[string]float64{}, Resources: refs, Biosphere: bioParents[i]}
		for _, key := range strings.Fields("elevation temperature precipitation pet moisture fertility aridity vegetation permeability sediment geothermal runoff baseflow waterTableDepth groundwaterHead salinity groundwater waterDepth") {
			field := in.e.Fields[key]
			if len(field) == w*h {
				state.Fields[key] = field[i]
			}
		}
		if sampler != nil {
			a := sampler.Sample(lon*math.Pi/180, lat*math.Pi/180)
			if in.transform != nil {
				a = in.transform(a, lon*math.Pi/180, lat*math.Pi/180)
			}
			state.Biome = a.Biome
			state.FreshWater = a.Water != "dry" && a.Water != "ocean"
			for k, v := range map[string]float64{"elevation": a.Bed, "waterDepth": a.WaterDepth, "temperature": a.Temperature, "precipitation": a.Precipitation, "pet": a.PET, "moisture": a.Moisture} {
				state.Fields[k] = v
			}
		}
		if a := in.e.Fields["recharge"]; i < len(a) {
			state.RechargeMMYear = a[i] * 1000
		}
		for id, h := range state.Resources {
			m.Resources[id] = h
		}
		m.Cells[cell.Key()] = resourcesurvey.Put(blobs, state)
	}
	keep := map[string][]byte{}
	for _, h := range m.References() {
		keep[h] = blobs[h]
	}
	return m, keep, nil
}

func readableKey(s string) string {
	var out strings.Builder
	for i, r := range s {
		if r == '_' || r == '.' || r == '-' {
			out.WriteByte(' ')
		} else {
			if i > 0 && unicode.IsUpper(r) {
				out.WriteByte(' ')
			}
			out.WriteRune(r)
		}
	}
	return out.String()
}
func resourceReportDocument(report terrainResourceReport, age string, year *int64, format string) []byte {
	var b strings.Builder
	b.WriteString("# Terrain resource report\n\nAge: " + age + "\n\nYear: ")
	if year == nil {
		b.WriteString("Undated")
	} else {
		b.WriteString(strconv.FormatInt(*year, 10))
	}
	b.WriteString("\n\n")
	raw, _ := json.Marshal(report)
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	delete(value, "fingerprint")
	delete(value, "context")
	var write func(string, any, int)
	write = func(key string, v any, depth int) {
		prefix := strings.Repeat("  ", depth)
		switch x := v.(type) {
		case map[string]any:
			if key != "" {
				b.WriteString(prefix + "- **" + readableKey(key) + "**:\n")
			}
			keys := []string{}
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				write(k, x[k], depth+1)
			}
		case []any:
			if len(x) == 0 {
				return
			}
			b.WriteString(prefix + "- **" + readableKey(key) + "**:\n")
			for i, item := range x {
				write(fmt.Sprintf("%d", i+1), item, depth+1)
			}
		default:
			if v != nil {
				b.WriteString(prefix + "- " + readableKey(key) + ": " + fmt.Sprint(v) + "\n")
			}
		}
	}
	write("Observed information", value, 0)
	b.WriteString("\nArchive statistics use retained geographic observations. Resource quantities describe whole deposits and must not be added for overlapping circles. Sampling is approximate; no calendar relationship between Ages is assumed.\n")
	text := b.String()
	if format == "txt" {
		text = strings.ReplaceAll(strings.ReplaceAll(text, "**", ""), "# ", "")
	}
	return []byte(text)
}
func (s *Server) archiveResourceReport(w http.ResponseWriter, r *http.Request) {
	var a struct {
		ArchiveID   string  `json:"archiveId"`
		Year        *int64  `json:"year"`
		Format      string  `json:"format"`
		Longitude   float64 `json:"longitude"`
		Latitude    float64 `json:"latitude"`
		RadiusKM    float64 `json:"radiusKm"`
		Fingerprint string  `json:"fingerprint"`
	}
	if err := decode(w, r, &a); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if a.Year != nil && (*a.Year < -9007199254740991 || *a.Year > 9007199254740991) {
		fail(w, 400, "Enter a whole year within the supported range")
		return
	}
	if a.Format == "" {
		a.Format = "md"
	}
	if a.Format != "md" && a.Format != "txt" {
		fail(w, 400, "Choose Markdown or text")
		return
	}
	in := s.captureResourceInput(w, r)
	if in == nil {
		return
	}
	if in.context == nil || s.projects == nil {
		fail(w, 409, "Save this terrain in a World before archiving reports")
		return
	}
	circle := resourcesurvey.Circle{Longitude: a.Longitude, Latitude: a.Latitude, RadiusKM: a.RadiusKM, PlanetRadiusKM: in.e.Geography.Radius}
	if !circle.Valid() || a.RadiusKM < .05 || a.RadiusKM > 5000 {
		fail(w, 400, "Invalid observation circle")
		return
	}
	report, err := s.resourceReport(r, in, model.Point{a.Longitude * math.Pi / 180, a.Latitude * math.Pi / 180}, a.RadiusKM)
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	if a.Fingerprint == "" || report.Fingerprint != a.Fingerprint {
		fail(w, 409, "Terrain information changed. Refresh the resource preview before archiving it.")
		return
	}
	if report.Truncated {
		spots, err := s.ecologySpots(r, in.context.WorldID, in.context.AgeID, in.context.TerrainID, true)
		if err != nil {
			fail(w, 503, err.Error())
			return
		}
		complete := buildResourceReport(r.Context(), in.p, in.e, model.Point{a.Longitude * math.Pi / 180, a.Latitude * math.Pi / 180}, a.RadiusKM, spots, in.transform, 0)
		complete.Context = report.Context
		complete.Derived = report.Derived
		complete.Stale = report.Stale
		report = complete
	}
	spots, err := s.ecologySpots(r, in.context.WorldID, in.context.AgeID, in.context.TerrainID, true)
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	manifest, blobs, err := canonicalResourceSamples(r.Context(), in, report, spots)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	d, err := s.projects.ResourceArchives(r.Context(), in.context.WorldID, in.context.AgeID, false)
	if err != nil {
		fail(w, 503, err.Error())
		return
	}
	now := time.Now().UnixMilli()
	survey := history.ResourceSurvey{Version: resourcesurvey.Version, TerrainID: in.context.TerrainID, MapID: in.context.MapID, SourceAge: in.context.AgeID, SourceSnapshot: in.context.Snapshot, TerrainRevision: in.context.Revision, Circle: circle, Year: a.Year, RecordedAt: now}
	name := fmt.Sprintf("Resources %.3f, %.3f — %.2f km²", a.Latitude, a.Longitude, circle.Area())
	saved, err := s.projects.SaveResourceSurvey(r.Context(), in.context.WorldID, in.context.AgeID, storage.ResourceArchiveSave{ArchiveID: a.ArchiveID, Survey: survey, Manifest: manifest, Blobs: blobs, Name: name, Format: a.Format, Document: resourceReportDocument(report, d.Age(in.context.AgeID).Name, a.Year, a.Format)})
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	s.mu.Lock()
	v := s.sessions[r.PathValue("id")]
	s.mu.Unlock()
	if v != nil {
		v.mu.Lock()
		if v.historyWorld == in.context.WorldID && v.historyRevision == d.World.Revision {
			v.historyRevision = saved.Revision
		}
		v.mu.Unlock()
	}
	respond(w, 201, saved)
}
