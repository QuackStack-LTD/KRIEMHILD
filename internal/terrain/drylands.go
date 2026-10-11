package terrain

// The seasonal climate layer is authoritative when available. Old projects
// retain a deterministic water-balance fallback instead of seeded dry blobs.
func (e *Environment) desertAt(i int) bool {
	if e.Fields["desert"] != nil {
		return e.get("desert", i) > .5
	}
	return e.get("moisture", i) < .25 && e.get("summer", i) >= 10
}
