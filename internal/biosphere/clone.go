package biosphere

import (
	"maps"
	"slices"
)

func cloneRequirements(v Requirements) Requirements {
	v.Strata = slices.Clone(v.Strata)
	v.Substrates = slices.Clone(v.Substrates)
	v.Habitats = slices.Clone(v.Habitats)
	copyRange := func(r *Range) *Range {
		if r == nil {
			return nil
		}
		v := *r
		return &v
	}
	v.SoilPH = copyRange(v.SoilPH)
	v.DroughtMonths = copyRange(v.DroughtMonths)
	v.GrowingMonths = copyRange(v.GrowingMonths)
	return v
}

// CloneLifeform preserves independent Age edits without serializing biological
// profiles to an intermediate JSON buffer for every inherited species.
func CloneLifeform(source *Lifeform) *Lifeform {
	if source == nil {
		return nil
	}
	v := *source
	v.Roles = slices.Clone(v.Roles)
	v.Traits = maps.Clone(v.Traits)
	v.RuleIDs = slices.Clone(v.RuleIDs)
	v.Requirements = cloneRequirements(v.Requirements)
	v.Dependencies = slices.Clone(v.Dependencies)
	for i := range v.Dependencies {
		v.Dependencies[i].SpeciesIDs = slices.Clone(v.Dependencies[i].SpeciesIDs)
		v.Dependencies[i].Seasons = slices.Clone(v.Dependencies[i].Seasons)
	}
	return &v
}

func cloneConditions(v Conditions) Conditions {
	v.Substrates = slices.Clone(v.Substrates)
	v.ClimateZones = slices.Clone(v.ClimateZones)
	v.Seasons = slices.Clone(v.Seasons)
	return v
}

// CloneState copies habitat geometry, cell profiles, populations and settings
// directly. Large Biospheres must not require a second serialized copy in RAM.
func CloneState(v State) State {
	v.Habitats = slices.Clone(v.Habitats)
	for i := range v.Habitats {
		h := &v.Habitats[i]
		h.Cells = slices.Clone(h.Cells)
		h.Overlays = slices.Clone(h.Overlays)
		h.Conditions = cloneConditions(h.Conditions)
		h.Profiles = slices.Clone(h.Profiles)
		for j := range h.Profiles {
			h.Profiles[j] = cloneConditions(h.Profiles[j])
		}
	}
	v.Occurrences = slices.Clone(v.Occurrences)
	for i := range v.Occurrences {
		v.Occurrences[i].Seasons = slices.Clone(v.Occurrences[i].Seasons)
	}
	v.Interactions = slices.Clone(v.Interactions)
	v.Diagnostics = slices.Clone(v.Diagnostics)
	v.Settings.Variants = maps.Clone(v.Settings.Variants)
	v.Settings.HabitatIDs = slices.Clone(v.Settings.HabitatIDs)
	v.Settings.Rules = slices.Clone(v.Settings.Rules)
	for i := range v.Settings.Rules {
		r := &v.Settings.Rules[i]
		r.TargetRoles = slices.Clone(r.TargetRoles)
		r.Habitats = slices.Clone(r.Habitats)
		r.HabitatIDs = slices.Clone(r.HabitatIDs)
		r.Traits = maps.Clone(r.Traits)
		r.RemoveTraits = slices.Clone(r.RemoveTraits)
		if r.Tolerances != nil {
			copy := cloneRequirements(*r.Tolerances)
			r.Tolerances = &copy
		}
	}
	v.Settings.Catalogue = slices.Clone(v.Settings.Catalogue)
	for i := range v.Settings.Catalogue {
		v.Settings.Catalogue[i].Biology = *CloneLifeform(&v.Settings.Catalogue[i].Biology)
	}
	return v
}
