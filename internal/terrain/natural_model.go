package terrain

// These immutable layers describe the generated world, not rendered objects.
// Region cells occupy half-cell footprints in the existing parent-grid CRS.
type NaturalGeometry struct {
	Kind  string       `json:"kind"`
	Cells []int        `json:"cells,omitempty"`
	Path  [][2]float64 `json:"path,omitempty"`
	Point *[2]float64  `json:"point,omitempty"`
}
type GeologicalEvent struct {
	Kind        string  `json:"kind"`
	AgeMa       float64 `json:"ageMa"`
	Environment string  `json:"environment"`
}
type GeologicalHistory struct {
	FormerMarine        bool    `json:"formerMarine"`
	AncientWetland      bool    `json:"ancientWetland"`
	Evaporative         bool    `json:"evaporative"`
	TropicalWeathering  bool    `json:"tropicalWeathering"`
	OrganicPreservation float64 `json:"organicPreservation"`
	MaxBurial           float64 `json:"maxBurialMetres"`
	MaxTemperature      float64 `json:"maxTemperatureC"`
	Trap                string  `json:"trap,omitempty"`
	Paleoclimate        string  `json:"paleoclimate"`
}
type GeologicalProvince struct {
	ID       string            `json:"id"`
	Setting  string            `json:"setting"`
	AgeMa    float64           `json:"ageMa"`
	Rock     string            `json:"dominantRock"`
	Process  string            `json:"process"`
	Geometry NaturalGeometry   `json:"geometry"`
	History  GeologicalHistory `json:"history"`
	Events   []GeologicalEvent `json:"events"`
}
type RockStratum struct {
	ID           string  `json:"id"`
	Rock         string  `json:"rock"`
	AgeMa        float64 `json:"ageMa"`
	Top          float64 `json:"topMetres"`
	Bottom       float64 `json:"bottomMetres"`
	Environment  string  `json:"environment"`
	Permeability float64 `json:"permeability"`
	Organic      float64 `json:"organicFraction"`
}
type GeologicalFormation struct {
	ID         string          `json:"id"`
	ProvinceID string          `json:"provinceId"`
	Kind       string          `json:"kind"`
	Rock       string          `json:"rock"`
	AgeMa      float64         `json:"ageMa"`
	Process    string          `json:"process"`
	Geometry   NaturalGeometry `json:"geometry"`
	Strata     []RockStratum   `json:"strata"`
	References []string        `json:"references,omitempty"`
}
type GeologicalStructure struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	AgeMa      float64         `json:"ageMa"`
	Geometry   NaturalGeometry `json:"geometry"`
	References []string        `json:"references"`
}
type GeologicalCell struct {
	Province   int     `json:"province"`
	Basement   int     `json:"basement"`
	Cover      int     `json:"cover"`
	Intrusion  int     `json:"intrusion"`
	Fault      int     `json:"fault"`
	Weathering float64 `json:"weathering"`
	Erosion    float64 `json:"erosion"`
	Deposition float64 `json:"deposition"`
	Soil       float64 `json:"soil"`
}
type GeologicalState struct {
	Version     int                   `json:"version"`
	Coordinates string                `json:"coordinates"`
	Provinces   []GeologicalProvince  `json:"provinces"`
	Formations  []GeologicalFormation `json:"formations"`
	Structures  []GeologicalStructure `json:"structures"`
	Cells       []GeologicalCell      `json:"cells"`
}
type NaturalReference struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type ResourceOccurrence struct {
	Properties    map[string]float64 `json:"properties,omitempty"`
	ID            string             `json:"id"`
	Type          string             `json:"type"`
	Variant       string             `json:"variant"`
	Geometry      NaturalGeometry    `json:"geometry"`
	Cause         string             `json:"cause"`
	Abundance     float64            `json:"abundance"`
	Quality       float64            `json:"quality"`
	Confidence    float64            `json:"confidence"`
	Accessibility float64            `json:"accessibility"`
	Depth         float64            `json:"depthMetres"`
	References    []NaturalReference `json:"references"`
}
type ResourceState struct {
	Version        int                  `json:"version"`
	GeologyVersion int                  `json:"geologyVersion"`
	Units          string               `json:"units"`
	Occurrences    []ResourceOccurrence `json:"occurrences"`
	// Renewable goods and aquifers are actual generated potential fields,
	// not biome-based guesses performed by the hover UI.
	Potential map[string][]float64 `json:"potential"`
}

func (e *Environment) naturalID(kind string, cell int) string {
	return e.hydroID("natural/"+kind, cell)
}
func rounded(v float64) float64 { return float64(int(v*1000+.5)) / 1000 }
