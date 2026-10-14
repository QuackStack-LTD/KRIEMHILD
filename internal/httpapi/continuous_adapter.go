package httpapi

import (
	"fmt"
	"kriemhild/internal/legacyterrain"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
	"kriemhild/internal/terrain/tectonics"
	"math"
)

// legacyProjection is a read adapter for shared geography/ecology/archive clients.
// The solver holds a completed classification only; it never generates this planet.
func legacyProjection(p *model.Planet) (*terrain.Solver, error) {
	c, err := terrain.DecodeConfig(terrain.Defaults)
	if err != nil {
		return nil, err
	}
	rules, err := terrain.Compile(c)
	if err != nil {
		return nil, err
	}
	w, h := 192, 96
	n := w * h
	s := &terrain.Solver{Rules: rules, W: w, H: h, N: n, T: len(c.Types), K: len(c.Continents), Z: len(c.Climates), Radius2: 1, WrapX: true, Status: "done", Dom: make([]uint32, n), Locked: make([]int, n), Pinned: make([]int, n), InQueue: make([]bool, n), Pos: make([]int, n), Buckets: make([][]int, len(c.Types)+1)}
	e := &terrain.Environment{Version: model.Version, Options: terrain.EnvironmentOptions{Columns: w, Rows: h, Seed: p.Spec.Seed, LatitudeNorth: 90, LatitudeSouth: -90, LandPercent: p.Spec.LandCoverage * 100, Ruggedness: p.Spec.Relief * 100, Rainfall: p.Spec.Rainfall}, Fields: map[string][]float64{}, Mask: make([]int, n), Heights: make([]int, n), WaterBodies: []terrain.WaterBody{}, Reefs: []terrain.ReefRegion{}}
	keys := append(append(append([]string{}, terrain.FieldNames...), terrain.SurfaceFields...), terrain.RealismFields...)
	keys = append(keys, "biome")
	for _, k := range keys {
		e.Fields[k] = make([]float64, n)
	}
	biomes := map[string]string{"grassland": "grass", "rainforest": "jungle", "alpine": "mountain", "water": "water", "steppe": "steppe", "ice": "snow", "tropical-woodland": "forest", "temperate-rainforest": "forest", "woodland": "forest", "shrubland": "scrub", "wetland": "marsh"}
	calendar := []terrain.ClimateSeason{{Name: "Annual cycle", Start: 0, Months: 12}}
	climate := &terrain.ClimateState{Version: terrain.ClimateModelVersion, Source: model.Version, Parameters: terrain.ClimateParameters{Mode: "automatic", MinSeasons: 1, MaxSeasons: 6, AxialTilt: p.Spec.Tilt, OrbitalDays: 365.25, Circulation: 1}, Scale: terrain.ClimateScale{WidthPX: float64(w), HeightPX: float64(h), PixelsPerCell: 1, GridWidth: w, GridHeight: h, CellCount: n, RadiusKM: p.Spec.RadiusKM, LatitudeNorth: 90, LatitudeSouth: -90, LongitudeWest: -180, LongitudeEast: 180, NorthSouthKM: math.Pi * p.Spec.RadiusKM / float64(h-1), EquatorialEastWestKM: model.Tau * p.Spec.RadiusKM / float64(w-1), Coverage: "planet", Approximate: true}, Calendar: calendar, FinalCount: 1, Cells: make([]terrain.ClimateCell, n)}
	zones := map[string]int{}
	sampler := detail.New(p)
	for i := 0; i < n; i++ {
		lon, lat := -math.Pi+model.Tau*float64(i%w)/float64(w-1), math.Pi/2-math.Pi*float64(i/w)/float64(h-1)
		a := sampler.Sample(lon, lat)
		e.Heights[i] = int(a.Bed / 4)
		if a.Water == "dry" {
			e.Mask[i] = 1
		}
		for k, val := range map[string]float64{"elevation": a.Bed, "bathymetry": math.Min(0, a.Bed), "temperature": a.Temperature, "precipitation": a.Precipitation, "moisture": a.Moisture, "latitude": lat * 180 / math.Pi, "waterLevel": a.WaterSurface, "waterDepth": a.WaterDepth, "flow": -1, "vegetation": a.Moisture, "drainageElevation": a.Bed, "fertility": a.Moisture, "aridity": 1 - a.Moisture, "windX": p.Grid.At("windX", lon, lat), "windY": p.Grid.At("windY", lon, lat)} {
			e.Fields[k][i] = val
		}
		if a.Water != "dry" {
			e.Fields["waterBody"][i] = 1
			if a.Water == "ocean" {
				e.Fields["ocean"][i] = 1
			}
		}
		id := a.Biome
		if v, ok := biomes[id]; ok {
			id = v
		}
		bt := 0
		for j, b := range terrain.Biomes {
			if b.ID == id {
				bt = j
				break
			}
		}
		e.Fields["biome"][i] = float64(bt)
		plate := tectonics.At(p, lon, lat)
		e.Fields["plate"][i] = float64(plate.Plate)
		e.Fields["tectonicStress"][i] = plate.Convergence
		e.Fields["mountainCore"][i] = plate.Convergence
		zone, ok := zones[id]
		if !ok {
			zone = len(climate.Zones)
			zones[id] = zone
			climate.Zones = append(climate.Zones, terrain.ClimateRegion{ID: "continuous-" + id, Kind: id, Name: id})
		}
		climate.Zones[zone].Cells = append(climate.Zones[zone].Cells, i)
		cell := terrain.ClimateCell{Biome: id, Zone: zone, Seasons: calendar, Estimate: 1, Modifiers: terrain.ClimateModifiers{Elevation: a.Bed, Latitude: lat * 180 / math.Pi, Maritime: math.Exp(-math.Max(0, a.CoastDistance) / 500)}}
		for month := 0; month < 12; month++ {
			temp := p.Grid.At(fmt.Sprintf("temperature-%02d", month), lon, lat) - (a.Bed-p.Grid.At("bed", lon, lat))*.0065
			rain := p.Grid.At(fmt.Sprintf("rainfall-%02d", month), lon, lat)
			pet := p.Grid.At(fmt.Sprintf("pet-%02d", month), lon, lat)
			moisture := model.Clamp(rain/math.Max(10, pet)*.6, 0, 1)
			cell.Months[month] = terrain.ClimateMonth{Temperature: temp, Rain: rain, Evaporation: pet, Humidity: moisture, SoilMoisture: moisture, Drought: 1 - moisture, WindX: e.Fields["windX"][i], WindY: e.Fields["windY"][i], WindSpeed: 5, Daylight: 12}
		}
		climate.Cells[i] = cell
		t := 0
		for j, v := range c.Types {
			if v.ID == id || v.EnvironmentType == id {
				t = j
				break
			}
		}
		s.Dom[i] = uint32(1) << t
	}
	// Legacy climate parameter ranges are narrower; exotic planet scales still
	// expose their authoritative monthly fields through the continuous model.
	if p.Spec.RadiusKM >= 500 && p.Spec.RadiusKM <= 50000 {
		e.Climate = climate
		g := &terrain.GeographicScale{Version: 1, Projection: "equirectangular", Coverage: "planet", Radius: p.Spec.RadiusKM, WidthPX: float64(w), HeightPX: float64(h), PixelsPerCell: 1, WorldScale: p.Spec.RadiusKM * math.Pi / float64(h), North: 90, South: -90, West: -180, East: 180, WrapX: true, CellAreaKM2: make([]float64, h)}
		for y := range g.CellAreaKM2 {
			north, south := (90-180*float64(y)/float64(h))*math.Pi/180, (90-180*float64(y+1)/float64(h))*math.Pi/180
			g.CellAreaKM2[y] = g.Radius * g.Radius * 360 * math.Pi / 180 / float64(w) * (math.Sin(north) - math.Sin(south))
		}
		e.Geography = g
	}
	s.Environment = e
	if err := terrain.RestoreProjectSolver(s, c, e, s.Snapshot()); err != nil {
		return nil, err
	}
	return s, nil
}
