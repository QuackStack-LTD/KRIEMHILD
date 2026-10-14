package history

import (
	"encoding/json"
	"fmt"
	"kriemhild/internal/biosphere"
	"reflect"
	"slices"
)

func (d *Document) Biosphere(age, terrain string) *biosphere.State {
	for i := range d.Biospheres {
		if d.Biospheres[i].AgeID == age && d.Biospheres[i].TerrainID == terrain {
			return &d.Biospheres[i]
		}
	}
	return nil
}
func (d *Document) Lifeforms(age string) []biosphere.Species {
	out := []biosphere.Species{}
	for _, e := range d.Entities {
		if e.AgeID == age && !e.Deleted && e.Biology != nil {
			var p map[string]any
			_ = json.Unmarshal(e.Properties, &p)
			summary, _ := p["summary"].(string)
			out = append(out, biosphere.Species{ID: e.ID, Name: e.Name, Summary: summary, Biology: *e.Biology})
		}
	}
	return out
}
func (d *Document) validateBiospheres() error {
	seen := map[string]bool{}
	for _, s := range d.Biospheres {
		if d.Age(s.AgeID) == nil || seen[s.AgeID+s.ID] {
			return fmt.Errorf("invalid biosphere Age or duplicate state")
		}
		seen[s.AgeID+s.ID] = true
		found := false
		for _, t := range d.Terrains {
			found = found || t.ID == s.TerrainID && t.AgeID == s.AgeID && t.Width == s.Width && t.Height == s.Height
		}
		if !found {
			return fmt.Errorf("biosphere terrain not found")
		}
		species := map[string]biosphere.Lifeform{}
		for _, e := range d.Entities {
			if e.AgeID == s.AgeID && !e.Deleted && e.Biology != nil {
				species[e.ID] = *e.Biology
			}
		}
		for _, r := range s.Settings.Rules {
			if r.SpeciesID != "" {
				target, ok := species[r.SpeciesID]
				if !ok || target.Group != r.TargetGroup {
					return fmt.Errorf("invalid biological rule target")
				}
			}
		}
		if err := biosphere.ValidateState(s, species); err != nil {
			return err
		}
	}
	return nil
}
func (d *Document) validateBiologicalDeletion(age string, ids []string) error {
	for _, s := range d.Biospheres {
		if s.AgeID != age {
			continue
		}
		for _, o := range s.Occurrences {
			if slices.Contains(ids, o.SpeciesID) {
				return fmt.Errorf("remove lifeform distributions or mark them locally extinct instead of deleting historical identity")
			}
		}
	}
	for _, e := range d.Entities {
		if e.AgeID == age && !e.Deleted && e.Biology != nil && slices.Contains(ids, e.Biology.ParentID) && !slices.Contains(ids, e.ID) {
			return fmt.Errorf("lifeform has dependent taxa")
		}
	}
	return nil
}

type BiosphereEdit struct {
	TerrainID   string                 `json:"terrainId"`
	Settings    *biosphere.Settings    `json:"settings,omitempty"`
	Occurrence  *biosphere.Occurrence  `json:"occurrence,omitempty"`
	Interaction *biosphere.Interaction `json:"interaction,omitempty"`
}

func (d *Document) applyBiosphere(c Command) error {
	if patch := c.BiosphereEdit; patch != nil {
		old := d.Biosphere(c.AgeID, patch.TerrainID)
		if old == nil {
			return fmt.Errorf("generate habitats before editing biosphere records")
		}
		next := biosphere.Clone(*old)
		if patch.Settings != nil {
			next.Settings = biosphere.Clone(*patch.Settings)
		}
		if patch.Occurrence != nil {
			next.Occurrences = slices.DeleteFunc(next.Occurrences, func(o biosphere.Occurrence) bool { return o.ID == patch.Occurrence.ID })
			next.Occurrences = append(next.Occurrences, *patch.Occurrence)
		}
		if patch.Interaction != nil {
			next.Interactions = slices.DeleteFunc(next.Interactions, func(r biosphere.Interaction) bool { return r.ID == patch.Interaction.ID })
			next.Interactions = append(next.Interactions, *patch.Interaction)
		}
		c.Biosphere = &next
	}
	if c.Biosphere == nil || c.Biosphere.AgeID != c.AgeID {
		return fmt.Errorf("biosphere state in this Age required")
	}
	old := d.Biosphere(c.AgeID, c.Biosphere.TerrainID)
	if old == nil {
		return fmt.Errorf("generate habitats before editing biosphere records")
	}
	next := biosphere.Clone(*c.Biosphere)
	if next.Source != old.Source || !reflect.DeepEqual(next.Habitats, old.Habitats) {
		return fmt.Errorf("habitats are generated from saved terrain; use generation to refresh them")
	}
	for _, o := range next.Occurrences {
		previous := false
		for _, p := range old.Occurrences {
			if reflect.DeepEqual(o, p) {
				previous = true
				break
			}
		}
		if previous || o.Exception != "" || o.Status == "locally-extinct" || o.Status == "absent" {
			continue
		}
		e := d.Entity(c.AgeID, o.SpeciesID)
		if e == nil || e.Biology == nil {
			return fmt.Errorf("lifeform does not exist")
		}
		for _, h := range next.Habitats {
			if h.ID == o.HabitatID {
				if score, reason := biosphere.Suitability(*e.Biology, h); score == 0 {
					return fmt.Errorf("%s; record an explicit supporting exception", reason)
				}
			}
		}
	}
	next.Stale = old.Stale
	if !reflect.DeepEqual(next.Occurrences, old.Occurrences) || !reflect.DeepEqual(next.Interactions, old.Interactions) {
		next.Stale = true
		next.Diagnostics = append(next.Diagnostics, "Authored occurrence or ecological interaction changed; review dependent ranges before regeneration.")
	}
	if err := biosphere.ValidateSettings(next.Settings); err != nil {
		return err
	}
	copy := biosphere.Clone(*d)
	*copy.Biosphere(c.AgeID, next.TerrainID) = next
	if err := copy.validateBiospheres(); err != nil {
		return err
	}
	*d = copy
	return nil
}

// Accept generated additions atomically. Confirmed ranges and authored/locked
// lifeforms always win over a regeneration, including one from another terrain.
func (d *Document) AcceptBiosphere(p biosphere.Preview, ids, locks []string, names map[string]string) error {
	next := biosphere.Clone(*d)
	s := biosphere.Clone(p.State)
	old := next.Biosphere(s.AgeID, s.TerrainID)
	selected := map[string]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	oldOccurrences := []biosphere.Occurrence{}
	oldLinks := []biosphere.Interaction{}
	if old != nil {
		oldOccurrences = old.Occurrences
		oldLinks = old.Interactions
		s.Stale = old.Stale || old.Source != s.Source
		seen := map[string]bool{}
		for _, h := range s.Habitats {
			seen[h.ID] = true
		}
		for _, h := range old.Habitats {
			if !seen[h.ID] {
				s.Habitats = append(s.Habitats, h)
			}
		}
	}
	occurrences := map[string]biosphere.Occurrence{}
	for _, o := range oldOccurrences {
		occurrences[o.ID] = o
	}
	candidateSpecies := map[string]biosphere.Species{}
	for _, e := range p.Species {
		candidateSpecies[e.ID] = e
	}
	added := map[string]bool{}
	for _, o := range s.Occurrences {
		if !selected[o.ID] {
			continue
		}
		if prev, ok := occurrences[o.ID]; ok && (prev.Confirmed || prev.Locked) {
			continue
		}
		o.Confirmed = true
		o.Locked = slices.Contains(locks, o.ID)
		occurrences[o.ID] = o
		added[o.ID] = true
	}
	links := map[string]biosphere.Interaction{}
	for _, r := range oldLinks {
		links[r.ID] = r
	}
	for _, r := range s.Interactions {
		if _, exists := links[r.ID]; !exists {
			links[r.ID] = r
		}
	}
	linksBySource := map[string][]biosphere.Interaction{}
	for _, r := range links {
		key := r.HabitatID + "/" + r.Source
		linksBySource[key] = append(linksBySource[key], r)
	}
	// Discarding a host/food entry also discards dependent new entries, never saves
	// viruses without hosts or predators without supported food populations.
	removedDependents := 0
	for changed := true; changed; {
		changed = false
		present := map[string]bool{}
		for _, o := range occurrences {
			if o.Status != "absent" && o.Status != "locally-extinct" {
				present[o.HabitatID+o.SpeciesID] = true
			}
		}
		for id, o := range occurrences {
			if !added[id] {
				continue
			}
			b := candidateSpecies[o.SpeciesID].Biology
			if e := next.Entity(s.AgeID, o.SpeciesID); e != nil && e.Biology != nil {
				b = *e.Biology
			}
			for _, dep := range b.Dependencies {
				supported := false
				for _, r := range linksBySource[o.HabitatID+"/"+o.SpeciesID] {
					if r.Source == o.SpeciesID && r.HabitatID == o.HabitatID && r.Kind == dep.Kind && present[o.HabitatID+r.Destination] {
						supported = true
					}
				}
				if !supported {
					delete(occurrences, id)
					delete(added, id)
					removedDependents++
					changed = true
					break
				}
			}
		}
	}
	if removedDependents > 0 {
		s.Diagnostics = append(s.Diagnostics, fmt.Sprintf("Omitted %d selected populations because their food or host populations were not accepted.", removedDependents))
	}
	used := map[string]bool{}
	// Record a completed acceptance only when every preview population survived
	// selection and dependency checks. Partial acceptance must remain resumable.
	s.AcceptedGeneration = biosphere.GenerationKey(s.Source, s.Settings)
	for _, occurrence := range p.State.Occurrences {
		if _, ok := occurrences[occurrence.ID]; !ok {
			s.AcceptedGeneration = ""
			break
		}
	}
	s.Occurrences = nil
	for _, o := range occurrences {
		s.Occurrences = append(s.Occurrences, o)
		used[o.SpeciesID] = true
	}
	sortedSpecies := []string{}
	for id := range used {
		sortedSpecies = append(sortedSpecies, id)
	}
	slices.Sort(sortedSpecies)
	for _, id := range sortedSpecies {
		if next.Entity(s.AgeID, id) != nil {
			continue
		}
		v, ok := candidateSpecies[id]
		if !ok {
			return fmt.Errorf("preview lifeform missing")
		}
		if name := names[id]; name != "" {
			v.Name = name
		}
		props, _ := json.Marshal(map[string]string{"summary": v.Summary})
		e := Entity{ID: id, Name: v.Name, Kind: "lifeform", Properties: props, Biology: &v.Biology}
		if _, err := next.applyEntities(Command{Kind: "entity", AgeID: s.AgeID, Entity: &e}); err != nil {
			return err
		}
		next.Entity(s.AgeID, id).Biology.Locked = slices.Contains(locks, id)
	}
	present := map[string]bool{}
	for _, o := range s.Occurrences {
		present[o.HabitatID+o.SpeciesID] = true
	}
	s.Interactions = nil
	for _, r := range links {
		if present[r.HabitatID+r.Source] && present[r.HabitatID+r.Destination] {
			s.Interactions = append(s.Interactions, r)
		}
	}
	slices.SortFunc(s.Occurrences, func(a, b biosphere.Occurrence) int { return compareID(a.ID, b.ID) })
	slices.SortFunc(s.Interactions, func(a, b biosphere.Interaction) int { return compareID(a.ID, b.ID) })
	if old == nil {
		next.Biospheres = append(next.Biospheres, s)
	} else {
		*old = s
	}
	if err := next.validateBiospheres(); err != nil {
		return err
	}
	*d = next
	d.SyncYears()
	return nil
}
func compareID(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Explicit inheritance exclusions remove only the new Age's occurrences. Remaining
// dependent populations are retained with a review flag instead of erased.
func (d *Document) pruneBiosphere(age string) {
	active := func(id string) bool { e := d.Entity(age, id); return e != nil && !e.Deleted }
	for i := range d.Biospheres {
		s := &d.Biospheres[i]
		if s.AgeID != age {
			continue
		}
		for j := range s.Settings.Rules {
			r := &s.Settings.Rules[j]
			if r.SpeciesID != "" && !active(r.SpeciesID) {
				r.SpeciesID = ""
				r.Enabled = false
			}
		}
		n := len(s.Occurrences)
		s.Occurrences = slices.DeleteFunc(s.Occurrences, func(o biosphere.Occurrence) bool { return !active(o.SpeciesID) })
		s.Interactions = slices.DeleteFunc(s.Interactions, func(r biosphere.Interaction) bool { return !active(r.Source) || !active(r.Destination) })
		if len(s.Occurrences) != n {
			s.Stale = true
			s.Diagnostics = append(s.Diagnostics, "Inherited lifeform excluded; review dependent populations.")
		}
	}
	for i := range d.Entities {
		e := &d.Entities[i]
		if e.AgeID == age && !e.Deleted && e.Biology != nil && e.Biology.ParentID != "" && !active(e.Biology.ParentID) {
			e.Biology.ParentID = ""
		}
	}
}
