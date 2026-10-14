package biosphere

import (
	"container/heap"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Surface category names come from terrain's append-only biome catalogue. The
// habitat names remain compatible with v1 reference packs.
func habitatKind(b string) string {
	return map[string]string{"grass": "grassland", "forest": "temperate-forest", "jungle": "tropical-forest", "taiga": "boreal-forest", "steppe": "steppe", "scrubland": "xeric-scrub", "savanna": "savanna", "mediterranean": "mediterranean-woodland", "desert": "desert", "cold_desert": "cold-desert", "tundra": "tundra", "snow": "polar", "alpine": "alpine", "mountain": "alpine", "swamp": "wetland", "dunes": "desert-dunes", "oasis": "oasis", "lava": "volcanic-barren", "ash": "volcanic-barren"}[b]
}

// Depth is organism depth below the water surface; bathymetry is bottom depth.
// These semantics apply only to v2 derived habitats. Stored v1 habitats are read
// as stored, never silently converted into water-column niches.
func layerConditions(c Conditions, stratum string) Conditions {
	if c.Medium == "terrestrial" {
		if stratum == "understory" {
			c.Light *= .4
		}
		if stratum == "floor" {
			c.Light *= .15
		}
		return c
	}
	c.Bathymetry = c.Depth
	switch stratum {
	case "photic":
		c.Depth = Range{0, math.Min(80, c.Bathymetry.Min)}
	case "twilight":
		c.Depth = Range{200, math.Min(1000, c.Bathymetry.Min)}
	case "aphotic":
		c.Depth = Range{1000, c.Bathymetry.Min}
	}
	depth := (c.Depth.Min + c.Depth.Max) / 2
	c.Light = math.Exp(-depth / 35)
	if depth >= 1000 {
		c.Light = 0
	}
	if depth > 200 {
		c.Temperature = Range{math.Max(-1, c.Temperature.Min*.15), math.Max(2, c.Temperature.Max*.25)}
	}
	if stratum == "benthic" {
		c.Substrates = []string{"sediment"}
	} else {
		c.Substrates = []string{"water"}
	}
	// Sinking organic material is an explicit subsidy, not a dark-water plant.
	c.DetritalSupply = .18 * math.Exp(-depth/4000)
	return c
}

func compatibleDependency(source Lifeform, dep Dependency, target Lifeform) bool {
	if dep.Group != "" && dep.Group != target.Group || dep.Role != "" && !slices.Contains(target.Roles, dep.Role) {
		return false
	}
	if dep.Medium != "" && dep.Medium != "any" && target.Requirements.Medium != dep.Medium {
		return false
	}
	if !seasonsOverlap(traitList(source, "activeSeasons"), traitList(target, "activeSeasons"), dep.Seasons) {
		return false
	}
	if dep.LifeStage != "" && !slices.Contains(traitList(target, "lifeStages"), strings.ToLower(dep.LifeStage)) {
		return false
	}
	switch dep.Kind {
	case "pollinates":
		return target.Group == "plant" && target.Traits["reproduction"] == "flowering"
	case "disperses-seeds-for":
		return target.Group == "plant" && target.Traits["reproduction"] == "flowering"
	case "predator-of":
		return target.Group == "animal" || target.Group == "protist"
	case "feeds-on":
		return target.Group != "virus" && target.Traits["inedible"] != "true"
	case "infects", "pathogen-of", "parasite-of":
		return target.Group != "virus" && (source.Traits["hostGroup"] == "" || source.Traits["hostGroup"] == target.Group)
	}
	return true
}

func traitList(b Lifeform, name string) []string {
	out := []string{}
	for _, value := range strings.Split(b.Traits[name], ",") {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			out = append(out, value)
		}
	}
	return out
}

// Missing phenology is unrestricted; explicit seasonal restrictions must overlap.
func seasonsOverlap(groups ...[]string) bool {
	var common []string
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		next := []string{}
		for _, season := range group {
			season = strings.ToLower(strings.TrimSpace(season))
			if common == nil || slices.Contains(common, season) {
				next = append(next, season)
			}
		}
		if len(next) == 0 {
			return false
		}
		common = next
	}
	return true
}

func matchesDependency(source Lifeform, dep Dependency, target Species) bool {
	return (len(dep.SpeciesIDs) == 0 || slices.Contains(dep.SpeciesIDs, target.ID)) && compatibleDependency(source, dep, target.Biology)
}

// Obligate dependency cycles cannot establish themselves without an independent
// foundation. Mutualistic and seasonal/stage-specific food webs are not rejected
// as circular merely because they contain reciprocal ecological relationships.
func validateObligateCycles(links []Interaction) error {
	graph := map[string][]string{}
	for _, link := range links {
		if link.Authored || link.Kind != "depends-on" || link.Season != "" || link.LifeStage != "" {
			continue
		}
		from, to := link.HabitatID+"/"+link.Source, link.HabitatID+"/"+link.Destination
		graph[from] = append(graph[from], to)
	}
	seen := map[string]int{}
	var visit func(string) bool
	visit = func(at string) bool {
		if seen[at] != 0 {
			return seen[at] == 1
		}
		seen[at] = 1
		for _, next := range graph[at] {
			if visit(next) {
				return true
			}
		}
		seen[at] = 2
		return false
	}
	for at := range graph {
		if visit(at) {
			return fmt.Errorf("circular obligate dependency without a seasonal, life-stage or authored explanation")
		}
	}
	return nil
}

type rangeStep struct {
	cell int
	cost float64
}

func mediumBit(medium string) uint8 {
	switch medium {
	case "terrestrial":
		return 1
	case "freshwater":
		return 2
	case "marine":
		return 4
	}
	return 0
}

type rangeQueue []rangeStep

func (q rangeQueue) Len() int { return len(q) }
func (q rangeQueue) Less(i, j int) bool {
	if q[i].cost == q[j].cost {
		return q[i].cell < q[j].cell
	}
	return q[i].cost < q[j].cost
}
func (q rangeQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *rangeQueue) Push(v any)   { *q = append(*q, v.(rangeStep)) }
func (q *rangeQueue) Pop() any     { a := *q; v := a[len(a)-1]; *q = a[:len(a)-1]; return v }

func connectedNativeRange(s Species, state State, seed string, origin ...string) map[string]bool {
	result := map[string]bool{}
	if state.Width < 1 || state.Height < 1 {
		return result
	}
	n := state.Width * state.Height
	media := make([]uint8, n)
	suitable := []Habitat{}
	for _, h := range state.Habitats {
		for _, c := range h.Cells {
			if c >= 0 && c < n {
				media[c] |= mediumBit(h.Conditions.Medium)
			}
		}
		if score, _ := Suitability(s.Biology, h); score > 0 {
			suitable = append(suitable, h)
		}
	}
	if len(suitable) == 0 {
		return result
	}
	anchor := suitable[int(random(seed+s.ID)*float64(len(suitable)))]
	// Locally generated adaptations originate where they were generated. Their
	// descendants still follow exactly the same connected dispersal constraints.
	if len(origin) > 0 {
		for _, h := range suitable {
			if h.ID == origin[0] {
				anchor = h
				break
			}
		}
	}
	starts := []int{}
	for k, cell := range anchor.Cells {
		if k < len(anchor.Profiles) {
			local := anchor
			local.Profiles, local.Conditions = nil, anchor.Profiles[k]
			if score, _ := profileSuitability(s.Biology, local); score == 0 {
				continue
			}
		}
		starts = append(starts, cell)
	}
	if len(starts) == 0 {
		return result
	}
	start := starts[int(random(s.ID+seed)*float64(len(starts)))]
	radius := s.Biology.Requirements.DispersalKM
	if radius == 0 {
		radius = map[string]float64{"local": 350, "regional": 2400, "mobile": 7000, "waterborne": 4500}[s.Biology.Dispersal]
	}
	scale := state.CellKM
	if scale <= 0 {
		scale = 100
	}
	distances := make([]float64, n)
	for i := range distances {
		distances[i] = math.Inf(1)
	}
	distances[start] = 0
	q := &rangeQueue{{start, 0}}
	heap.Init(q)
	for q.Len() > 0 {
		step := heap.Pop(q).(rangeStep)
		i := step.cell
		if step.cost > distances[i] || step.cost > radius {
			continue
		}
		x, y := i%state.Width, i/state.Width
		ns := []int{}
		if x > 0 {
			ns = append(ns, i-1)
		} else if state.WrapX {
			ns = append(ns, i+state.Width-1)
		}
		if x+1 < state.Width {
			ns = append(ns, i+1)
		} else if state.WrapX {
			ns = append(ns, i-state.Width+1)
		}
		if y > 0 {
			ns = append(ns, i-state.Width)
		}
		if y+1 < state.Height {
			ns = append(ns, i+state.Width)
		}
		for _, j := range ns {
			medium := media[j]
			// Even "any" organisms disperse through the anchor's medium unless an
			// explicit airborne trait authorizes crossing water/land barriers.
			traversable := medium&mediumBit(anchor.Conditions.Medium) != 0
			if s.Biology.Requirements.Medium == "amphibious" {
				traversable = medium&3 != 0
			}
			airborne := s.Biology.Traits["dispersalMode"] == "airborne"
			if medium == 0 || (!traversable && !airborne) {
				continue
			}
			distance := scale
			if state.CellEastWestKM > 0 && i/state.Width == j/state.Width {
				latitude := state.LatitudeNorth + (state.LatitudeSouth-state.LatitudeNorth)*(float64(y)+.5)/float64(state.Height)
				distance = state.CellEastWestKM * math.Max(.02, math.Cos(latitude*math.Pi/180))
			}
			cost := step.cost + distance
			if !traversable {
				cost += distance * 3
			}
			if cost <= radius && cost < distances[j] {
				distances[j] = cost
				heap.Push(q, rangeStep{j, cost})
			}
		}
	}
	for _, h := range suitable {
		for k, c := range h.Cells {
			if distances[c] <= radius {
				if k < len(h.Profiles) {
					local := h
					local.Profiles, local.Conditions = nil, h.Profiles[k]
					if score, _ := profileSuitability(s.Biology, local); score == 0 {
						continue
					}
				}
				result[h.ID] = true
				break
			}
		}
	}
	return result
}
