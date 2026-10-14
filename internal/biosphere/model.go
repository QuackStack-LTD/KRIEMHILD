// Package biosphere derives communities from finalized environmental records.
package biosphere

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

const Version = 2

func Stable(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "/")))
	return hex.EncodeToString(h[:12])
}
func Clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }

type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}
type Requirements struct {
	Strata        []string `json:"strata,omitempty"`
	SoilPH        *Range   `json:"soilPH,omitempty"`
	DroughtMonths *Range   `json:"droughtMonths,omitempty"`
	GrowingMonths *Range   `json:"growingMonths,omitempty"`
	DispersalKM   float64  `json:"dispersalKm,omitempty"`
	Medium        string   `json:"medium"` // terrestrial, freshwater, marine, amphibious, any
	Temperature   Range    `json:"temperature"`
	Moisture      Range    `json:"moisture"`
	Elevation     Range    `json:"elevation"`
	Depth         Range    `json:"depth"`
	Salinity      Range    `json:"salinity"`
	Substrates    []string `json:"substrates,omitempty"`
	Habitats      []string `json:"habitats,omitempty"`
	MinLight      float64  `json:"minLight"`
}
type Dependency struct {
	SpeciesIDs  []string `json:"speciesIds,omitempty"`
	Medium      string   `json:"medium,omitempty"`
	Seasons     []string `json:"seasons,omitempty"`
	LifeStage   string   `json:"lifeStage,omitempty"`
	Group       string   `json:"group,omitempty"`
	Role        string   `json:"role,omitempty"`
	Kind        string   `json:"kind"`
	Description string   `json:"description,omitempty"`
}
type Lifeform struct {
	Version             int               `json:"version"`
	Group               string            `json:"group"`
	CustomGroup         string            `json:"customGroup,omitempty"`
	Origin              string            `json:"origin"`
	ScientificName      string            `json:"scientificName,omitempty"`
	Taxonomy            string            `json:"taxonomy,omitempty"`
	ParentID            string            `json:"parentId,omitempty"`
	Roles               []string          `json:"roles"`
	Traits              map[string]string `json:"traits,omitempty"`
	Lifecycle           string            `json:"lifecycle,omitempty"`
	Dispersal           string            `json:"dispersal"`
	Requirements        Requirements      `json:"requirements"`
	Dependencies        []Dependency      `json:"dependencies,omitempty"`
	Completeness        string            `json:"completeness"`
	CatalogueID         string            `json:"catalogueId,omitempty"`
	Generator           string            `json:"generator,omitempty"`
	NativeOriginHabitat string            `json:"nativeOriginHabitat,omitempty"`
	RuleIDs             []string          `json:"ruleIds,omitempty"`
	Locked              bool              `json:"locked"`
}
type Species struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Summary string   `json:"summary"`
	Biology Lifeform `json:"biology"`
}
type Conditions struct {
	SeasonalKnown     bool     `json:"seasonalKnown,omitempty"`
	Bathymetry        Range    `json:"bathymetry,omitempty"`
	SoilPH            float64  `json:"soilPH,omitempty"`
	SoilDepth         float64  `json:"soilDepth,omitempty"`
	Drainage          float64  `json:"drainage,omitempty"`
	DroughtMonths     float64  `json:"droughtMonths,omitempty"`
	GrowingMonths     float64  `json:"growingMonths,omitempty"`
	DetritalSupply    float64  `json:"detritalSupply,omitempty"`
	Geothermal        float64  `json:"geothermal,omitempty"`
	Temperature       Range    `json:"temperature"`
	Moisture          Range    `json:"moisture"`
	Elevation         Range    `json:"elevation"`
	Depth             Range    `json:"depth"`
	Salinity          Range    `json:"salinity"`
	Rain              float64  `json:"rain"`
	Fertility         float64  `json:"fertility"`
	Light             float64  `json:"light"`
	SeasonalAmplitude float64  `json:"seasonalAmplitude"`
	Medium            string   `json:"medium"`
	Substrates        []string `json:"substrates"`
	ClimateZones      []string `json:"climateZones"`
	Seasons           []string `json:"seasons"`
}
type Habitat struct {
	BaseBiome   string       `json:"baseBiome,omitempty"`
	Overlays    []string     `json:"overlays,omitempty"`
	Profiles    []Conditions `json:"profiles,omitempty"` // One profile per cell, same order as Cells.
	RegionID    string       `json:"regionId,omitempty"`
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Kind        string       `json:"kind"`
	Stratum     string       `json:"stratum,omitempty"`
	Cells       []int        `json:"cells"`
	Conditions  Conditions   `json:"conditions"`
	Description string       `json:"description"`
}
type Occurrence struct {
	ID          string   `json:"id"`
	SpeciesID   string   `json:"speciesId"`
	HabitatID   string   `json:"habitatId"`
	TerrainID   string   `json:"terrainId"`
	Abundance   float64  `json:"abundance"`
	Status      string   `json:"status"`
	Seasons     []string `json:"seasons"`
	Suitability float64  `json:"suitability"`
	Reason      string   `json:"reason"`
	Locked      bool     `json:"locked"`
	Confirmed   bool     `json:"confirmed"`
	Exception   string   `json:"exception,omitempty"`
}
type Interaction struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Kind        string `json:"kind"`
	HabitatID   string `json:"habitatId"`
	Season      string `json:"season,omitempty"`
	LifeStage   string `json:"lifeStage,omitempty"`
	Description string `json:"description,omitempty"`
	Authored    bool   `json:"authored"`
}
type Rule struct {
	TargetRoles  []string          `json:"targetRoles,omitempty"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	TargetGroup  string            `json:"targetGroup"`
	SpeciesID    string            `json:"speciesId,omitempty"`
	Habitats     []string          `json:"habitats,omitempty"`
	HabitatIDs   []string          `json:"habitatIds,omitempty"`
	Substrate    string            `json:"substrate,omitempty"`
	Traits       map[string]string `json:"traits"`
	RemoveTraits []string          `json:"removeTraits,omitempty"`
	Tolerances   *Requirements     `json:"tolerances,omitempty"`
	FoodGroup    string            `json:"foodGroup,omitempty"`
	Lifecycle    string            `json:"lifecycle,omitempty"`
	Abundance    float64           `json:"abundance"`
	Priority     int               `json:"priority"`
	Conflict     string            `json:"conflict"`
	Strict       bool              `json:"strict"`
	Provenance   string            `json:"provenance"`
	Enabled      bool              `json:"enabled"`
}
type Settings struct {
	Variants    map[string]int `json:"variants,omitempty"`
	Mode        string         `json:"mode"`
	Preset      string         `json:"preset"`
	Familiarity float64        `json:"familiarity"`
	Richness    string         `json:"richness"`
	Coherence   string         `json:"coherence"`
	Scope       string         `json:"scope"`
	HabitatIDs  []string       `json:"habitatIds,omitempty"`
	Seed        string         `json:"seed"`
	Ideas       string         `json:"ideas,omitempty"`
	Rules       []Rule         `json:"rules"`
	Catalogue   []Species      `json:"catalogue,omitempty"`
}
type State struct {
	AcceptedGeneration string        `json:"acceptedGeneration,omitempty"`
	WrapX              bool          `json:"wrapX,omitempty"`
	CellKM             float64       `json:"cellKm,omitempty"`
	CellEastWestKM     float64       `json:"cellEastWestKm,omitempty"`
	LatitudeNorth      float64       `json:"latitudeNorth,omitempty"`
	LatitudeSouth      float64       `json:"latitudeSouth,omitempty"`
	ID                 string        `json:"id"`
	AgeID              string        `json:"ageId"`
	TerrainID          string        `json:"terrainId"`
	Version            int           `json:"version"`
	Source             string        `json:"source"`
	Width              int           `json:"width"`
	Height             int           `json:"height"`
	Habitats           []Habitat     `json:"habitats"`
	Occurrences        []Occurrence  `json:"occurrences"`
	Interactions       []Interaction `json:"interactions"`
	Settings           Settings      `json:"settings"`
	Diagnostics        []string      `json:"diagnostics"`
	Stale              bool          `json:"stale"`
}
type Preview struct {
	State      State     `json:"state"`
	Species    []Species `json:"species"`
	Exclusions []string  `json:"exclusions"`
}

var Groups = []string{"plant", "animal", "fungus", "algae", "bacterium", "archaea", "protist", "virus", "custom"}
var Roles = []string{"producer", "decomposer", "detritivore", "herbivore", "predator", "omnivore", "scavenger", "pollinator", "seed-disperser", "mutualist", "parasite", "pathogen", "host-dependent"}
var InteractionKinds = []string{"feeds-on", "predator-of", "pollinates", "disperses-seeds-for", "symbiotic-with", "parasite-of", "pathogen-of", "infects", "competes-with", "decomposes", "depends-on", "evolves-from"}

func ValidRequirements(r Requirements) error {
	if !finite(r.DispersalKM) || r.DispersalKM < 0 || r.DispersalKM > 50000 {
		return fmt.Errorf("invalid dispersal distance")
	}
	for _, v := range []*Range{r.SoilPH, r.DroughtMonths, r.GrowingMonths} {
		if v != nil && (!finite(v.Min) || !finite(v.Max) || v.Min > v.Max || v.Min < 0 || v.Max > 14) {
			return fmt.Errorf("invalid ecological niche range")
		}
	}
	for _, v := range []*Range{r.DroughtMonths, r.GrowingMonths} {
		if v != nil && v.Max > 12 {
			return fmt.Errorf("month range exceeds annual cycle")
		}
	}
	if !slices.Contains([]string{"terrestrial", "freshwater", "marine", "amphibious", "any"}, r.Medium) {
		return fmt.Errorf("invalid environmental medium")
	}
	for _, v := range []Range{r.Temperature, r.Moisture, r.Elevation, r.Depth, r.Salinity} {
		if !finite(v.Min) || !finite(v.Max) || v.Min > v.Max {
			return fmt.Errorf("invalid tolerance range")
		}
	}
	if r.Moisture.Min < 0 || r.Moisture.Max > 1 || r.Depth.Min < 0 || r.Salinity.Min < 0 || !finite(r.MinLight) || r.MinLight < 0 || r.MinLight > 1 {
		return fmt.Errorf("invalid moisture, depth, salinity or sunlight requirements")
	}
	return nil
}
func ValidateLifeform(b *Lifeform) error {
	if b == nil {
		return nil
	}
	if (b.Version < 1 || b.Version > Version) || !slices.Contains(Groups, b.Group) || !slices.Contains([]string{"earth-reference", "fictional", "custom-created", "placeholder"}, b.Origin) || !slices.Contains([]string{"placeholder", "draft", "complete"}, b.Completeness) || !slices.Contains([]string{"local", "regional", "mobile", "waterborne"}, b.Dispersal) {
		return fmt.Errorf("invalid lifeform classification")
	}
	if len(b.Roles) == 0 || len(b.Roles) > 12 || len(b.Dependencies) > 20 || len(b.Traits) > 60 {
		return fmt.Errorf("lifeform needs ecological roles and bounded traits")
	}
	for _, r := range b.Roles {
		if !slices.Contains(Roles, r) {
			return fmt.Errorf("unknown ecological role %s", r)
		}
	}
	for k, v := range b.Traits {
		if k == "" || len(k) > 120 || len(v) > 4000 {
			return fmt.Errorf("invalid biological trait")
		}
	}
	if len(b.Lifecycle) > 20000 || len(b.ScientificName) > 200 || len(b.Taxonomy) > 4000 {
		return fmt.Errorf("biological profile too large")
	}
	for _, dep := range b.Dependencies {
		if !slices.Contains(InteractionKinds, dep.Kind) || (dep.Group == "" && dep.Role == "" && len(dep.SpeciesIDs) == 0) || (dep.Group != "" && !slices.Contains(Groups, dep.Group)) || (dep.Role != "" && !slices.Contains(Roles, dep.Role)) || len(dep.SpeciesIDs) > 20 || len(dep.Seasons) > 12 || len(dep.LifeStage) > 80 || dep.Medium != "" && !slices.Contains([]string{"any", "terrestrial", "freshwater", "marine", "amphibious"}, dep.Medium) {
			return fmt.Errorf("invalid ecological dependency")
		}
	}
	if b.Group == "virus" {
		host := false
		for _, dep := range b.Dependencies {
			host = host || dep.Kind == "infects"
		}
		if !host {
			return fmt.Errorf("viruses require a compatible host group and transmission context")
		}
		if b.Traits["transmission"] == "" {
			return fmt.Errorf("virus needs transmission context")
		}
	}
	return ValidRequirements(b.Requirements)
}
func ValidateSettings(s Settings) error {
	if !slices.Contains([]string{"quick", "guided", "advanced"}, s.Mode) || !slices.Contains([]string{"sparse", "moderate", "diverse"}, s.Richness) || !slices.Contains([]string{"realistic", "flexible", "speculative"}, s.Coherence) || !slices.Contains([]string{"entire", "selected", "missing"}, s.Scope) || !finite(s.Familiarity) || s.Familiarity < 0 || s.Familiarity > 1 {
		return fmt.Errorf("invalid biosphere generation settings")
	}
	if s.Scope == "selected" && len(s.HabitatIDs) == 0 {
		return fmt.Errorf("select at least one habitat")
	}
	if len(s.Rules) > 100 || len(s.Catalogue) > 150 || len(s.Ideas) > 10000 || len(s.Seed) > 200 {
		return fmt.Errorf("generation settings too large")
	}
	for key, variant := range s.Variants {
		if len(key) > 300 || variant < 0 || variant > 1000000 {
			return fmt.Errorf("invalid regional generation variant")
		}
	}
	seen := map[string]bool{}
	for _, r := range s.Rules {
		if r.ID == "" || seen[r.ID] || strings.TrimSpace(r.Name) == "" || !slices.Contains(Groups, r.TargetGroup) || !slices.Contains([]string{"merge", "replace", "reject"}, r.Conflict) || !finite(r.Abundance) || r.Abundance < 0 || r.Abundance > 3 {
			return fmt.Errorf("invalid or duplicate biological rule")
		}
		seen[r.ID] = true
		for _, role := range r.TargetRoles {
			if !slices.Contains(Roles, role) {
				return fmt.Errorf("unknown target role")
			}
		}
		for k, v := range r.Traits {
			if strings.TrimSpace(k) == "" || len(k) > 120 || len(v) > 4000 {
				return fmt.Errorf("invalid rule trait")
			}
		}
		if r.Tolerances != nil {
			if err := ValidRequirements(*r.Tolerances); err != nil {
				return err
			}
		}
		if len(r.Traits) > 60 {
			return fmt.Errorf("rule has too many traits")
		}
		if r.FoodGroup != "" && !slices.Contains(Groups, r.FoodGroup) {
			return fmt.Errorf("unknown food group")
		}
	}
	seen = map[string]bool{}
	for _, c := range s.Catalogue {
		if c.ID == "" || seen[c.ID] || len(c.Name) == 0 || len(c.Name) > 200 {
			return fmt.Errorf("invalid reference pack identity")
		}
		seen[c.ID] = true
		if err := ValidateLifeform(&c.Biology); err != nil {
			return err
		}
	}
	return nil
}
func finite(v float64) bool         { return !math.IsInf(v, 0) && !math.IsNaN(v) }
func clamp(v, a, b float64) float64 { return math.Max(a, math.Min(b, v)) }
