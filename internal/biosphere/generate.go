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
			if matchesDependency(b, dep, s) {
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
	oldInteractions := append([]Interaction(nil), base.Interactions...)
	previousSeed := base.Settings.Seed
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
	type guild struct {
		role, group string
		foundation  bool
	}
	guilds := []guild{{"producer", "", true}, {"decomposer", "", true}, {"decomposer", "bacterium", false}, {"detritivore", "animal", false}, {"herbivore", "animal", false}, {"predator", "animal", false}, {"pollinator", "animal", false}, {"omnivore", "protist", false}, {"scavenger", "animal", false}, {"seed-disperser", "animal", false}, {"mutualist", "fungus", false}, {"parasite", "animal", false}, {"pathogen", "bacterium", false}, {"host-dependent", "virus", false}}
	nativeRanges := map[string]map[string]bool{}
	nativeOrigins := map[string]string{}
	used := map[string]bool{}
	allCandidates := map[string]Species{}
	for _, h := range base.Habitats {
		if !selected(h, settings) {
			continue
		}
		if settings.Scope != "missing" && previousSeed == settings.Seed {
			confirmed, variant := false, false
			for _, o := range oldOccurrences {
				if o.HabitatID == h.ID && (o.Confirmed || o.Locked) {
					confirmed = true
					break
				}
			}
			for _, guild := range guilds {
				variant = variant || settings.Variants[h.ID+"/"+guild.role] > 0
			}
			if confirmed && !variant {
				continue
			}
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
		for _, guild := range guilds {
			role := guild.role
			// Accepted guilds are stable across identical previews. Dependencies
			// created later in the previous run must not silently expand them.
			retained := false
			if settings.Variants[h.ID+"/"+role] == 0 {
				for _, occurrence := range oldOccurrences {
					if occurrence.HabitatID != h.ID || (!occurrence.Confirmed && !occurrence.Locked) {
						continue
					}
					if species, ok := byID[occurrence.SpeciesID]; ok && slices.Contains(species.Biology.Roles, role) && (guild.group == "" || species.Biology.Group == guild.group) {
						retained = true
						break
					}
				}
			}
			if retained {
				continue
			}
			if settings.Scope == "missing" {
				filled := false
				for _, s := range available {
					filled = filled || slices.Contains(s.Biology.Roles, role) && (guild.group == "" || s.Biology.Group == guild.group)
				}
				if filled {
					continue
				}
			}
			localSeed := settings.Seed + fmt.Sprint(settings.Variants[h.ID+"/"+role])
			candidates := []Species{}
			for _, t := range templates {
				if !slices.Contains(t.Biology.Roles, role) || (guild.group != "" && t.Biology.Group != guild.group) {
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
			group := map[string]string{"producer": "plant", "decomposer": "fungus", "herbivore": "animal", "predator": "animal", "pollinator": "animal", "omnivore": "protist", "scavenger": "animal", "seed-disperser": "animal", "parasite": "animal", "mutualist": "fungus", "pathogen": "bacterium", "host-dependent": "virus"}[role]
			if h.Conditions.Medium != "terrestrial" && role == "producer" {
				group = "algae"
			}
			if role == "detritivore" {
				group = "animal"
			}
			if role == "decomposer" && (h.Conditions.Medium != "terrestrial" || h.Kind == "desert" || h.Kind == "polar") {
				group = "bacterium"
			}
			if (h.Kind == "geothermal" || h.Stratum == "benthic" && h.Conditions.Geothermal > 150) && role == "producer" {
				group = "archaea"
			}
			if guild.group != "" {
				group = guild.group
			}
			groups := []string{group}
			for _, r := range settings.Rules {
				if ruleMatches(r, h) && r.SpeciesID == "" && r.TargetGroup != group && ((role == "decomposer" && r.TargetGroup == "fungus") || (role == "producer" && r.TargetGroup == "plant")) {
					groups = append(groups, r.TargetGroup)
				}
			}
			for _, g := range groups {
				b := Lifeform{Version: Version, Group: g, Origin: "fictional", Roles: []string{role}, Traits: map[string]string{"adaptation": "Adapted to " + h.Kind}, Dispersal: "regional", Completeness: "draft", Generator: "biosphere-v2", Requirements: Requirements{Medium: h.Conditions.Medium, Temperature: Range{h.Conditions.Temperature.Min - 6, h.Conditions.Temperature.Max + 6}, Moisture: Range{math.Max(0, h.Conditions.Moisture.Min-.15), math.Min(1, h.Conditions.Moisture.Max+.15)}, Elevation: Range{math.Floor(h.Conditions.Elevation.Min/2000)*2000 - 500, math.Ceil(h.Conditions.Elevation.Max/2000)*2000 + 500}, Depth: Range{0, math.Ceil(h.Conditions.Depth.Max/1000)*1000 + 50}, Salinity: Range{math.Floor(h.Conditions.Salinity.Min/10) * 10, math.Ceil(h.Conditions.Salinity.Max/10)*10 + 1}, Habitats: []string{h.Kind}}}
				// Nearby patches share ecological archetypes. Exact floating-point
				// envelopes made almost every small patch invent a separate species.
				b.Requirements.Temperature = Range{math.Floor(h.Conditions.Temperature.Min/10)*10 - 5, math.Ceil(h.Conditions.Temperature.Max/10)*10 + 5}
				b.Requirements.Moisture = Range{math.Max(0, math.Floor(h.Conditions.Moisture.Min*4)/4-.1), math.Min(1, math.Ceil(h.Conditions.Moisture.Max*4)/4+.1)}
				b.Requirements.Strata = []string{h.Stratum}
				b.Requirements.DroughtMonths = &Range{math.Max(0, math.Floor(h.Conditions.DroughtMonths/4)*4-2), math.Min(12, math.Floor(h.Conditions.DroughtMonths/4)*4+6)}
				b.Requirements.GrowingMonths = &Range{math.Max(0, math.Floor(h.Conditions.GrowingMonths/4)*4-2), math.Min(12, math.Floor(h.Conditions.GrowingMonths/4)*4+6)}
				if h.Conditions.Medium == "terrestrial" {
					b.Requirements.SoilPH = &Range{math.Max(0, math.Floor(h.Conditions.SoilPH/2)*2-1), math.Min(14, math.Floor(h.Conditions.SoilPH/2)*2+3)}
				}

				if g == "plant" {
					b.Traits["reproduction"] = "flowering"
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
						b.Lifecycle = "Resident; cold-season dormancy"
						b.Traits["dormancy"] = "cold-season"
					}
				}
				if g == "fungus" {
					b.Traits["growth"] = "Filament networks process local organic matter; spores disperse during moist periods"
				}
				if role == "decomposer" && h.Conditions.Light < .01 && h.Conditions.DetritalSupply > 0 {
					b.Traits["energy"] = "Sinking organic detritus from photic production"
				}
				if g == "archaea" {
					b.Traits["energy"] = "Chemical energy from supplied geothermal substrates"
				}
				if role == "producer" && g != "archaea" {
					b.Traits["energy"] = "Photosynthesis"
					b.Requirements.MinLight = .02
					b.Requirements.Strata = []string{h.Stratum}
				}
				if role == "decomposer" {
					b.Dependencies = []Dependency{{Role: "producer", Kind: "decomposes"}}
				}
				if role == "decomposer" && h.Conditions.Light < .01 && h.Conditions.DetritalSupply > 0 {
					b.Dependencies = nil
				}
				if role == "herbivore" {
					b.Dependencies = []Dependency{{Role: "producer", Kind: "feeds-on"}}
				}
				if role == "detritivore" {
					b.Dependencies = []Dependency{{Role: "decomposer", Kind: "feeds-on", Description: "Microbe-conditioned organic detritus and biofilm"}}
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
				if role == "scavenger" {
					b.Dependencies = []Dependency{{Group: "animal", Kind: "feeds-on", Description: "Locally produced carrion subsidy"}}
				}
				if role == "seed-disperser" {
					b.Dependencies = []Dependency{{Group: "plant", Role: "producer", Kind: "disperses-seeds-for"}}
				}
				if role == "parasite" {
					b.Dependencies = []Dependency{{Group: "animal", Kind: "parasite-of"}}
					b.Traits["hostGroup"] = "animal"
				}
				if role == "mutualist" {
					b.Dependencies = []Dependency{{Role: "producer", Kind: "symbiotic-with", Description: "Resource exchange with a local primary producer"}}
				}
				if role == "pathogen" {
					b.Dependencies = []Dependency{{Group: "animal", Kind: "pathogen-of"}}
					b.Traits["hostGroup"] = "animal"
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
				{ // All native origins use the same spatial dispersal constraints.
					ranges, found := nativeRanges[s.ID]
					if !found {
						if s.Biology.Generator == "biosphere-v2" {
							origin := s.Biology.NativeOriginHabitat
							if origin == "" {
								origin = h.ID
							}
							nativeOrigins[s.ID] = origin
							ranges = connectedNativeRange(s, base, settings.Seed, origin)
						} else {
							ranges = connectedNativeRange(s, base, settings.Seed)
						}
						nativeRanges[s.ID] = ranges
					}
					if !ranges[h.ID] {
						continue
					}
					if origin := nativeOrigins[s.ID]; origin != "" {
						s.Biology.NativeOriginHabitat = origin
					}
				}
				if used[h.ID+s.ID] {
					continue
				}
				score, reason := Suitability(s.Biology, h)
				if score < map[string]float64{"realistic": .58, "flexible": .43, "speculative": .25}[settings.Coherence] {
					continue
				}
				partners := make([]Species, 0, len(available))
				for _, partner := range available {
					if partner.ID != s.ID {
						partners = append(partners, partner)
					}
				}
				targets, ok := depsFor(s.Biology, partners)
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
				abundance := score * clamp(.25+.5*h.Conditions.Fertility+.25*h.Conditions.Light+h.Conditions.DetritalSupply, .1, 1)
				if role == "predator" || role == "parasite" {
					abundance *= .3
				}
				for _, r := range settings.Rules {
					if slices.Contains(s.Biology.RuleIDs, r.ID) {
						abundance *= r.Abundance
					}
				}
				abundance = clamp(abundance, 0, 1)
				seasons := append([]string(nil), h.Conditions.Seasons...)
				if active := traitList(s.Biology, "activeSeasons"); len(active) > 0 {
					seasons = active
				}
				status := "native"
				if s.Biology.Traits["seasonalPresence"] == "warm-season" {
					seasons = []string{"warm / active season"}
					if s.Biology.Dispersal == "mobile" && len(nativeRanges[s.ID]) > 1 && s.Biology.Traits["migration"] != "" {
						status = "seasonally-present"
						reason += "; seasonal visitor with an explicit migration trait and connected alternative habitat"
					} else {
						reason += "; seasonal activity of a resident, without an inferred migration"
					}
				}
				if authored := s.Biology.Traits["occurrenceStatus"]; slices.Contains([]string{"introduced", "cultivated", "invasive", "rare", "locally-threatened"}, authored) {
					status = authored
					reason += "; explicit biological origin/history rule"
				}
				o := Occurrence{ID: Stable(h.ID, s.ID), SpeciesID: s.ID, HabitatID: h.ID, TerrainID: base.TerrainID, Abundance: abundance, Status: status, Seasons: seasons, Suitability: score, Reason: reason}
				for j, t := range targets {
					dep := s.Biology.Dependencies[j]
					season := ""
					if len(dep.Seasons) > 0 {
						season = strings.Join(dep.Seasons, ",")
					}
					preview.State.Interactions = append(preview.State.Interactions, Interaction{ID: Stable(h.ID, s.ID, t.ID, dep.Kind), Source: s.ID, Destination: t.ID, Kind: dep.Kind, HabitatID: h.ID, Season: season, LifeStage: dep.LifeStage, Description: dep.Description})
				}
				preview.State.Occurrences = append(preview.State.Occurrences, o)
				allCandidates[s.ID] = s
				available = append(available, s)
				used[h.ID+s.ID] = true
				count++
			}
			if count == 0 && guild.foundation && !(role == "producer" && h.Conditions.Light < .01 && h.Conditions.DetritalSupply > 0) {
				preview.State.Diagnostics = append(preview.State.Diagnostics, h.Name+": no compatible "+role+"; habitat remains incomplete")
			}
		}
	}
	// Preview preserves authored/confirmed records and the links they need.
	for _, o := range oldOccurrences {
		if !o.Locked && !o.Confirmed {
			continue
		}
		found := false
		for k, newOccurrence := range preview.State.Occurrences {
			if newOccurrence.ID == o.ID {
				preview.State.Occurrences[k] = o
				found = true
				break
			}
		}
		if !found {
			preview.State.Occurrences = append(preview.State.Occurrences, o)
		}
		if s, ok := byID[o.SpeciesID]; ok {
			allCandidates[s.ID] = s
		}
	}
	for _, link := range oldInteractions {
		retained := link.Authored
		for _, occurrence := range oldOccurrences {
			if occurrence.HabitatID == link.HabitatID && occurrence.SpeciesID == link.Source && (occurrence.Locked || occurrence.Confirmed) {
				retained = true
				break
			}
		}
		if !retained {
			continue
		}
		found := false
		for k, fresh := range preview.State.Interactions {
			if fresh.ID == link.ID {
				preview.State.Interactions[k] = link
				found = true
				break
			}
		}
		if !found {
			preview.State.Interactions = append(preview.State.Interactions, link)
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

func ValidateState(s State, species map[string]Lifeform) error {
	if (s.Version < 1 || s.Version > Version) || s.ID == "" || s.ID != s.TerrainID || s.Width < 1 || s.Height < 1 || s.Width > 256 || s.Height > 256 {
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
		if len(h.Profiles) > 0 && len(h.Profiles) != len(h.Cells) {
			return fmt.Errorf("habitat cell profiles do not match its geometry")
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
	presenceSeasons := map[string][]string{}
	for _, o := range s.Occurrences {
		b, ok := species[o.SpeciesID]
		h, exists := habitats[o.HabitatID]
		if !ok || !exists || o.ID == "" || occurrences[o.ID] || o.TerrainID != s.TerrainID || !slices.Contains([]string{"native", "introduced", "cultivated", "invasive", "rare", "locally-threatened", "threatened", "seasonally-present", "migratory", "locally-extinct", "absent"}, o.Status) || !finite(o.Suitability) || o.Suitability < 0 || o.Suitability > 1 || !finite(o.Abundance) || o.Abundance < 0 || o.Abundance > 1 {
			return fmt.Errorf("invalid occurrence %s", o.ID)
		}
		occurrences[o.ID] = true
		if !slices.Contains([]string{"absent", "locally-extinct"}, o.Status) {
			present[o.HabitatID+o.SpeciesID] = true
			// Listing every local season describes year-round residency, not a
			// restriction to those labels when a partner has explicit phenology.
			active := traitList(b, "activeSeasons")
			if len(active) == 0 && (o.Status == "seasonally-present" || o.Status == "migratory" || !slices.Equal(o.Seasons, h.Conditions.Seasons)) {
				active = o.Seasons
			}
			presenceSeasons[o.HabitatID+o.SpeciesID] = active
		}
		if !s.Stale && o.Exception == "" && o.Status != "absent" && o.Status != "locally-extinct" {
			if score, reason := Suitability(b, h); score == 0 {
				return fmt.Errorf("%s: %s; record an authored exception or revise its distribution", o.ID, reason)
			}
		}
	}
	seenLinks := map[string]bool{}
	if s.Version >= 2 && !s.Stale {
		modern := []Interaction{}
		for _, link := range s.Interactions {
			if species[link.Source].Version >= 2 && species[link.Destination].Version >= 2 {
				modern = append(modern, link)
			}
		}
		if err := validateObligateCycles(modern); err != nil {
			return err
		}
	}
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
		if s.Version >= 2 && !s.Stale && !r.Authored {
			if !present[r.HabitatID+r.Source] || !present[r.HabitatID+r.Destination] {
				return fmt.Errorf("ecological link has no co-occurring populations")
			}
			var requiredSeasons []string
			if r.Season != "" {
				requiredSeasons = strings.Split(r.Season, ",")
			}
			if !seasonsOverlap(presenceSeasons[r.HabitatID+r.Source], presenceSeasons[r.HabitatID+r.Destination], requiredSeasons) {
				return fmt.Errorf("ecological link has no seasonal overlap")
			}
			if !compatibleDependency(species[r.Source], Dependency{Kind: r.Kind}, species[r.Destination]) {
				return fmt.Errorf("incompatible %s interaction", r.Kind)
			}
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
				compatible := matchesDependency(b, dep, Species{ID: r.Destination, Biology: target})
				if s.Version == 1 {
					compatible = (dep.Group == "" || dep.Group == target.Group) && (dep.Role == "" || slices.Contains(target.Roles, dep.Role))
				}
				if r.Source == o.SpeciesID && r.HabitatID == o.HabitatID && r.Kind == dep.Kind && present[o.HabitatID+r.Destination] && compatible {
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
