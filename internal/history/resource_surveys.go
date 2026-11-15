package history

import (
	"encoding/hex"
	"fmt"
	"kriemhild/internal/resourcesurvey"
)

// The readable Library file owns this reference. Custody, loss, deletion and
// inheritance therefore affect statistics exactly as they affect the document.
type ResourceSurvey struct {
	Version         int                   `json:"version"`
	TerrainID       string                `json:"terrainId"`
	MapID           string                `json:"mapId"`
	SourceAge       string                `json:"sourceAge"`
	SourceSnapshot  string                `json:"sourceSnapshot"`
	TerrainRevision int                   `json:"terrainRevision"`
	Circle          resourcesurvey.Circle `json:"circle"`
	Year            *int64                `json:"year"`
	RecordedAt      int64                 `json:"recordedAt"`
	ManifestHash    string                `json:"manifestHash"`
}

func (s ResourceSurvey) Validate(d *Document, age string) error {
	raw, err := hex.DecodeString(s.ManifestHash)
	terrain := false
	for _, t := range d.Terrains {
		if t.AgeID == s.SourceAge && t.ID == s.TerrainID {
			terrain = true
		}
	}
	if s.Version != resourcesurvey.Version || err != nil || len(raw) != 32 || !s.Circle.Valid() || d.Age(s.SourceAge) == nil || !terrain || s.RecordedAt < 1 || s.Year != nil && (*s.Year < -9007199254740991 || *s.Year > 9007199254740991) {
		return fmt.Errorf("invalid archived resource observation")
	}
	return nil
}
