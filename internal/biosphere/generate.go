package biosphere

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

func random(key string) float64 {
	s := Stable(key)
	v, _ := strconv.ParseUint(s[:8], 16, 32)
	return float64(v) / 4294967296
}
func selected(h Habitat, s Settings) bool {
	return s.Scope != "selected" || slices.Contains(s.HabitatIDs, h.ID)
}
func ruleMatches(r Rule, h Habitat) bool {
	return r.Enabled && (len(r.Habitats) == 0 || slices.Contains(r.Habitats, h.Kind)) && (len(r.HabitatIDs) == 0 || slices.Contains(r.HabitatIDs, h.ID)) && (r.Substrate == "" || slices.Contains(h.Conditions.Substrates, r.Substrate))
}
func ApplyRules(b *Lifeform, h Habitat, rules []Rule) error {
	rules = append([]Rule(nil), rules...)
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Priority < rules[j].Priority })
	changed := map[string]string{}
	for _, r := range rules {
		roleMatches := len(r.TargetRoles) == 0
		for _, role := range b.Roles {
			roleMatches = roleMatches || slices.Contains(r.TargetRoles, role)
		}
		if r.TargetGroup != b.Group || !roleMatches || !ruleMatches(r, h) {
			continue
		}
		if b.Traits == nil {
			b.Traits = map[string]string{}
		}
		for k, v := range r.Traits {
			if old, ok := changed[k]; ok && old != v && r.Conflict == "reject" {
				return fmt.Errorf("rule conflict on trait %s", k)
			}
			if _, ok := b.Traits[k]; !ok || r.Conflict != "merge" {
				b.Traits[k] = v
			}
			changed[k] = v
		}
		for _, k := range r.RemoveTraits {
			delete(b.Traits, k)
		}
		if r.Tolerances != nil {
			b.Requirements = Clone(*r.Tolerances)
		}
		if r.Strict && r.Substrate != "" {
			b.Requirements.Substrates = []string{r.Substrate}
		}
		if r.Strict && len(r.Habitats) > 0 {
			b.Requirements.Habitats = append([]string(nil), r.Habitats...)
		}
		if r.FoodGroup != "" {
			kind := "feeds-on"
			if b.Group == "virus" {
				kind = "infects"
			}
			b.Dependencies = []Dependency{{Group: r.FoodGroup, Kind: kind, Description: "Required by biological rule " + r.Name}}
		}
		if r.Lifecycle != "" {
			b.Lifecycle = r.Lifecycle
		}
		b.RuleIDs = append(b.RuleIDs, r.ID)
	}
	return nil
}
func depsFor(b Lifeform, available []Species) ([]Species, bool) {
	targets := []Species{}
	for _, dep := range b.Dependencies {
		found := false
		for _, s := range available {
			if (dep.Group == "" || s.Biology.Group == dep.Group) && (dep.Role == "" || slices.Contains(s.Biology.Roles, dep.Role)) {
				targets = append(targets, s)
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return targets, true
}
func Generate(base State, settings Settings, existing []Species) (Preview, error) {
	if err := ValidateSettings(settings); err != nil {
		return Preview{}, err
	}
	oldOccurrences := append([]Occurrence(nil), base.Occurrences...)
	base.Settings = Clone(settings)
	base.Version = Version
	base.Diagnostics = []string{}
	base.Occurrences = []Occurrence{}
	base.Interactions = []Interaction{}
	preview := Preview{State: base, Species: []Species{}, Exclusions: []string{}}
	byID := map[string]Species{}
	for _, s := range existing {
		byID[s.ID] = s
	}
	for _, r := range settings.Rules {
		if r.SpeciesID != "" {
			s, ok := byID[r.SpeciesID]
			if !ok || s.Biology.Group != r.TargetGroup {
				return Preview{}, fmt.Errorf("rule %s targets a missing lifeform or incompatible group", r.Name)
			}
		}
	}
	templates := append(Catalogue(), settings.Catalogue...)
	for _, candidate := range existing {
		for _, r := range settings.Rules {
			if r.Enabled && r.SpeciesID == candidate.ID {
				templates = append(templates, candidate)
				break
			}
		}
	}
	limit := map[string]int{"sparse": 1, "moderate": 2, "diverse": 3}[settings.Richness]
	roles := []string{"producer", "decomposer", "decomposer", "herbivore", "predator", "pollinator", "omnivore", "host-dependent"}
	nativeRanges := map[string]map[string]bool{}
	used := map[string]bool{}
	allCandidates := map[string]Species{}
	for _, h := range base.Habitats {
		if !selected(h, settings) {
			continue
		}
		available := []Species{}
		// Confirmed species in this region can supply dependencies to missing entries.
		for _, o := range oldOccurrences {
			if o.HabitatID == h.ID && o.Status != "absent" && o.Status != "locally-extinct" {
				if s, ok := byID[o.SpeciesID]; ok {
					available = append(available, s)
				}
			}
		}
		for stage, role := range roles {
			if settings.Scope == "missing" {
				filled := false
				for _, s := range available {
					filled = filled || slices.Contains(s.Biology.Roles, role) && (stage != 2 || s.Biology.Group == "bacterium")
				}
				if filled {
					continue
				}
			}
			if settings.Richness == "sparse" && stage > 4 {
				continue
			}
			localSeed := settings.Seed + fmt.Sprint(settings.Variants[h.ID+"/"+role])
			candidates := []Species{}
			for _, t := range templates {
				if !slices.Contains(t.Biology.Roles, role) || (stage == 2 && t.Biology.Group != "bacterium") {
					continue
				}
				ref := Clone(t)
				specific := []Rule{}
				for _, r := range settings.Rules {
					if r.SpeciesID == t.ID && ruleMatches(r, h) {
						specific = append(specific, r)
					}
				}
				if len(specific) > 0 {
					if err := ApplyRules(&ref.Biology, h, specific); err != nil {
						preview.Exclusions = append(preview.Exclusions, err.Error())
						continue
					}
					ref.Biology.Origin = "fictional"
					ref.Biology.ParentID = t.ID
					ref.Name = t.Name + " - adapted variant"
					ref.ID = Stable("variant", t.ID, h.ID, localSeed)
					ref.Biology.Locked = false
				}
				score, _ := Suitability(ref.Biology, h)
				if score > 0 {
					if len(specific) == 0 {
						ref.ID = Stable("earth", t.ID)
					}
					if t.Biology.Origin == "custom-created" && len(specific) == 0 {
						ref.ID = t.ID
					}
					candidates = append(candidates, ref)
				}
			}
			// Adapted archetypes keep a functional foundation even without an Earth analogue.
			group := map[string]string{"producer": "plant", "decomposer": "fungus", "herbivore": "animal", "predator": "animal", "pollinator": "animal", "omnivore": "protist", "host-dependent": "virus"}[role]
			if h.Conditions.Medium != "terrestrial" && role == "producer" {
				group = "algae"
			}
			if role == "decomposer" && (h.Conditions.Medium != "terrestrial" || h.Kind == "desert" || h.Kind == "polar") {
				group = "bacterium"
			}
			if h.Kind == "geothermal" && role == "producer" {
				group = "archaea"
			}
			if stage == 2 {
				group = "bacterium"
			}
			groups := []string{group}
			for _, r := range settings.Rules {
				if ruleMatches(r, h) && r.SpeciesID == "" && r.TargetGroup != group && ((role == "decomposer" && r.TargetGroup == "fungus") || (role == "producer" && r.TargetGroup == "plant")) {
					groups = append(groups, r.TargetGroup)
				}
			}
			for _, g := range groups {
				b := Lifeform{Version: 1, Group: g, Origin: "fictional", Roles: []string{role}, Traits: map[string]string{"adaptation": "Adapted to " + h.Kind}, Dispersal: "regional", Completeness: "draft", Generator: "biosphere-v1", Requirements: Requirements{Medium: h.Conditions.Medium, Temperature: Range{math.Floor(h.Conditions.Temperature.Min/20)*20 - 5, math.Ceil(h.Conditions.Temperature.Max/20)*20 + 5}, Moisture: Range{0, 1}, Elevation: Range{math.Floor(h.Conditions.Elevation.Min/2000)*2000 - 500, math.Ceil(h.Conditions.Elevation.Max/2000)*2000 + 500}, Depth: Range{0, math.Ceil(h.Conditions.Depth.Max/1000)*1000 + 50}, Salinity: Range{math.Floor(h.Conditions.Salinity.Min/10) * 10, math.Ceil(h.Conditions.Salinity.Max/10)*10 + 1}, Habitats: []string{h.Kind}}}
				if g == "plant" {
					b.Traits["growth"] = "Low spreading growth with seasonal seed production"
					if strings.Contains(h.Kind, "forest") {
						b.Traits["growth"] = "Layered woody growth with wind-dispersed seeds"
					}
					if h.Kind == "desert" || h.Kind == "desert-dunes" {
						b.Traits["physiology"] = "Water-storing tissues and a waxy surface"
					} else if h.Conditions.Temperature.Min < 0 {
						b.Traits["physiology"] = "Dormant cold-season buds and protected storage tissues"
					}
				}
				if g == "animal" {
					b.Traits["locomotion"] = "Padded ground locomotion"
					if h.Conditions.Medium != "terrestrial" {
						b.Traits["locomotion"] = "Aquatic propulsion and dissolved-oxygen exchange"
					} else if h.Kind == "alpine" {
						b.Traits["locomotion"] = "Gripping feet for uneven slopes"
					}
					if h.Conditions.Temperature.Min < 0 {
						b.Lifecycle = "Cold-season dormancy or seasonal movement within the native range"
					}
				}
				if g == "fungus" {
					b.Traits["growth"] = "Filament networks process local organic matter; spores disperse during moist periods"
				}
				if g == "archaea" {
					b.Traits["energy"] = "Chemical energy from supplied geothermal substrates"
				}
				if role == "producer" && g != "archaea" {
					b.Traits["energy"] = "Photosynthesis"
					b.Requirements.MinLight = .02
					if h.Conditions.Medium == "marine" {
						b.Requirements.MinLight = 0
						b.Traits["verticalRange"] = "Photic surface"
					}
				}
				if role == "decomposer" {
					b.Dependencies = []Dependency{{Role: "producer", Kind: "decomposes"}}
				}
				if role == "herbivore" {
					b.Dependencies = []Dependency{{Role: "producer", Kind: "feeds-on"}}
				}
				if role == "predator" {
					b.Dependencies = []Dependency{{Role: "herbivore", Kind: "predator-of"}}
				}
				if role == "pollinator" {
					b.Dependencies = []Dependency{{Group: "plant", Kind: "pollinates"}}
				}
				if role == "omnivore" {
					b.Dependencies = []Dependency{{Group: "bacterium", Kind: "feeds-on"}}
				}
				if role == "host-dependent" {
					b.Dependencies = []Dependency{{Group: "bacterium", Kind: "infects"}}
					b.Traits["transmission"] = "Local host contact; interaction is not necessarily pathogenic"
				}
				genericRules := []Rule{}
				for _, r := range settings.Rules {
					if r.SpeciesID == "" {
						genericRules = append(genericRules, r)
					}
				}
				if err := ApplyRules(&b, h, genericRules); err != nil {
					preview.Exclusions = append(preview.Exclusions, h.Name+": "+err.Error())
					continue
				}
				eco := fmt.Sprintf("%s/%s/%s", h.Kind, g, strings.Join(b.RuleIDs, "/"))
				profileKey, _ := json.Marshal(b)
				id := Stable("fiction", localSeed, eco, role, string(profileKey))
				syllables := []string{"Silver", "Ember", "Veil", "Dawn", "Moss", "Aster", "Glass", "Amber"}
				name := syllables[int(random(settings.Seed+id)*8)] + " " + strings.ReplaceAll(h.Kind, "-", " ") + " " + role
				if settings.Mode == "quick" {
					b.Origin = "placeholder"
					b.Completeness = "placeholder"
					name = strings.ReplaceAll(h.Kind, "-", " ") + " " + g + " · " + role
					id = Stable("placeholder", localSeed, eco, role, string(profileKey))
				}
				candidates = append(candidates, Species{ID: id, Name: name, Summary: "Environment-adapted " + g + " providing the " + role + " role.", Biology: b})
			}
			sort.SliceStable(candidates, func(i, j int) bool {
				rank := func(s Species) float64 {
					earth := s.Biology.Origin == "earth-reference"
					bias := settings.Familiarity
					if !earth {
						bias = 1 - bias
					}
					if settings.Mode == "quick" && !earth {
						bias = 2
					}
					if len(s.Biology.RuleIDs) > 0 {
						bias += 1
					}
					return bias + random(localSeed+h.ID+s.ID)*.7
				}
				return rank(candidates[i]) > rank(candidates[j])
			})
			count := 0
			for _, s := range candidates {
				if count >= limit {
					break
				}
				if old, ok := byID[s.ID]; ok {
					s = old
				}
				if s.Biology.Origin == "earth-reference" {
					ranges, found := nativeRanges[s.ID]
					if !found {
						ranges = nativeRange(s, base.Habitats, settings.Seed, base.Width)
						nativeRanges[s.ID] = ranges
					}
					if !ranges[h.ID] {
						continue
					}
				}
				if used[h.ID+s.ID] {
					continue
				}
				score, reason := Suitability(s.Biology, h)
				if score < map[string]float64{"realistic": .58, "flexible": .43, "speculative": .25}[settings.Coherence] {
					continue
				}
				targets, ok := depsFor(s.Biology, available)
				if !ok {
					if len(preview.Exclusions) < 80 {
						preview.Exclusions = append(preview.Exclusions, s.Name+" in "+h.Name+": missing compatible food or host")
					}
					continue
				}
				// Mobile/dispersive species occupy more patches, but local reference populations
				// are restricted to seeded regional refugia instead of filling every suitable cell.
				if s.Biology.Origin == "earth-reference" && count > 0 && random(s.ID+h.ID) > .6 {
					continue
				}
				abundance := score
				for _, r := range settings.Rules {
					if slices.Contains(s.Biology.RuleIDs, r.ID) {
						abundance *= r.Abundance
					}
				}
				abundance = clamp(abundance, 0, 1)
				seasons := append([]string(nil), h.Conditions.Seasons...)
				status := "native"
				if s.Biology.Group == "animal" && h.Conditions.Temperature.Min < 0 {
					status = "seasonally-present"
					seasons = []string{"warm / active season"}
				}
				o := Occurrence{ID: Stable(h.ID, s.ID), SpeciesID: s.ID, HabitatID: h.ID, TerrainID: base.TerrainID, Abundance: abundance, Status: status, Seasons: seasons, Suitability: score, Reason: reason}
				for j, t := range targets {
					dep := s.Biology.Dependencies[j]
					preview.State.Interactions = append(preview.State.Interactions, Interaction{ID: Stable(h.ID, s.ID, t.ID, dep.Kind), Source: s.ID, Destination: t.ID, Kind: dep.Kind, HabitatID: h.ID, Description: dep.Description})
				}
				preview.State.Occurrences = append(preview.State.Occurrences, o)
				allCandidates[s.ID] = s
				available = append(available, s)
				used[h.ID+s.ID] = true
				count++
			}
			if count == 0 && stage < 2 {
				preview.State.Diagnostics = append(preview.State.Diagnostics, h.Name+": no compatible "+role+"; habitat remains incomplete")
			}
		}
	}
	ids := []string{}
	for id := range allCandidates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		preview.Species = append(preview.Species, allCandidates[id])
	}
	return preview, nil
}

// Native reference ranges are selected independently of preview scope and generation
// order. Neighbouring suitable regions around a seeded refugium form a clustered
// range; disconnected continents can remain uncolonized even if climatically suitable.
func nativeRange(s Species, habitats []Habitat, seed string, width int) map[string]bool {
	suitable := []Habitat{}
	for _, h := range habitats {
		if score, _ := Suitability(s.Biology, h); score > 0 {
			suitable = append(suitable, h)
		}
	}
	result := map[string]bool{}
	if len(suitable) == 0 {
		return result
	}
	anchor := suitable[int(random(seed+s.ID)*float64(len(suitable)))].Cells[0]
	distance := func(h Habitat) int {
		c := h.Cells[0]
		dx, dy := c%width-anchor%width, c/width-anchor/width
		return dx*dx + dy*dy
	}
	sort.SliceStable(suitable, func(i, j int) bool { return distance(suitable[i]) < distance(suitable[j]) })
	limit := map[string]int{"local": 1, "regional": 12, "mobile": 80, "waterborne": 60}[s.Biology.Dispersal]
	for i, h := range suitable {
		if i >= limit {
			break
		}
		result[h.ID] = true
	}
	return result
}

func ValidateState(s State, species map[string]Lifeform) error {
	if s.Version != Version || s.ID == "" || s.ID != s.TerrainID || s.Width < 1 || s.Height < 1 || s.Width > 256 || s.Height > 256 {
		return fmt.Errorf("invalid biosphere state")
	}
	if err := ValidateSettings(s.Settings); err != nil {
		return err
	}
	habitats := map[string]Habitat{}
	for _, h := range s.Habitats {
		if h.ID == "" || len(h.Cells) == 0 {
			return fmt.Errorf("invalid habitat geometry")
		}
		if _, ok := habitats[h.ID]; ok {
			return fmt.Errorf("duplicate habitat")
		}
		seen := map[int]bool{}
		for _, c := range h.Cells {
			if c < 0 || c >= s.Width*s.Height || seen[c] {
				return fmt.Errorf("invalid habitat cell")
			}
			seen[c] = true
		}
		habitats[h.ID] = h
	}
	occurrences := map[string]bool{}
	present := map[string]bool{}
	for _, o := range s.Occurrences {
		b, ok := species[o.SpeciesID]
		h, exists := habitats[o.HabitatID]
		if !ok || !exists || o.ID == "" || occurrences[o.ID] || o.TerrainID != s.TerrainID || !slices.Contains([]string{"native", "introduced", "cultivated", "invasive", "rare", "locally-threatened", "threatened", "seasonally-present", "migratory", "locally-extinct", "absent"}, o.Status) || !finite(o.Suitability) || o.Suitability < 0 || o.Suitability > 1 || !finite(o.Abundance) || o.Abundance < 0 || o.Abundance > 1 {
			return fmt.Errorf("invalid occurrence %s", o.ID)
		}
		occurrences[o.ID] = true
		if !slices.Contains([]string{"absent", "locally-extinct"}, o.Status) {
			present[o.HabitatID+o.SpeciesID] = true
		}
		if !s.Stale && o.Exception == "" && o.Status != "absent" && o.Status != "locally-extinct" {
			if score, reason := Suitability(b, h); score == 0 {
				return fmt.Errorf("%s: %s; record an authored exception or revise its distribution", o.ID, reason)
			}
		}
	}
	seenLinks := map[string]bool{}
	linksBySource := map[string][]Interaction{}
	for _, r := range s.Interactions {
		if seenLinks[r.ID] || r.ID == "" || !slices.Contains(InteractionKinds, r.Kind) || r.Source == r.Destination {
			return fmt.Errorf("invalid ecological interaction")
		}
		seenLinks[r.ID] = true
		linksBySource[r.HabitatID+"/"+r.Source] = append(linksBySource[r.HabitatID+"/"+r.Source], r)
		if _, ok := species[r.Source]; !ok {
			return fmt.Errorf("ecological source missing")
		}
		if _, ok := species[r.Destination]; !ok {
			return fmt.Errorf("ecological target missing")
		}
		if _, ok := habitats[r.HabitatID]; !ok {
			return fmt.Errorf("ecological habitat missing")
		}
	}
	for _, o := range s.Occurrences {
		if !present[o.HabitatID+o.SpeciesID] || o.Exception != "" || s.Stale {
			continue
		}
		b := species[o.SpeciesID]
		for _, dep := range b.Dependencies {
			ok := false
			for _, r := range linksBySource[o.HabitatID+"/"+o.SpeciesID] {
				target := species[r.Destination]
				if r.Source == o.SpeciesID && r.HabitatID == o.HabitatID && r.Kind == dep.Kind && present[o.HabitatID+r.Destination] && (dep.Group == "" || dep.Group == target.Group) && (dep.Role == "" || slices.Contains(target.Roles, dep.Role)) {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("%s missing %s dependency in %s", o.SpeciesID, dep.Kind, o.HabitatID)
			}
		}
	}
	return nil
}
