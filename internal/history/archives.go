package history

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const ArchiveModelVersion = 1

var ArchiveCategories = []string{"chronicles", "administrative", "economic", "cultural", "geographical", "personal", "genealogies", "compilations"}
var ArchivePurposes = []string{"personal", "household", "religious", "civic", "military", "diplomatic", "commercial", "royal", "scholarly", "general"}

// Knowledge sources are author-established evidence, not automatically traversed graph edges.
// Event dates belong to SourceAge; delivery belongs to ArrivalAge. These clocks
// are never compared numerically across Ages.
type KnowledgeSource struct {
	ID          string   `json:"id"`
	EntityID    string   `json:"entityId"`
	SourceAge   string   `json:"sourceAge"`
	Channel     string   `json:"channel"`
	Evidence    string   `json:"evidence"`
	Strength    string   `json:"strength"`
	FromYear    int64    `json:"fromYear"`
	ToYear      int64    `json:"toYear"`
	ArrivalYear int64    `json:"arrivalYear"`
	ArrivalAge  string   `json:"arrivalAge"`
	Categories  []string `json:"categories"`
}
type ArchiveConfig struct {
	Version        int               `json:"version"`
	OwnerID        string            `json:"ownerId"`
	Purpose        string            `json:"purpose"`
	Intensity      string            `json:"intensity"`
	Categories     []string          `json:"categories"`
	KnowledgeScope string            `json:"knowledgeScope"`
	Sources        []KnowledgeSource `json:"sources,omitempty"`
	Retention      string            `json:"retention"`
	Coverage       []ArchiveCoverage `json:"coverage,omitempty"`
}
type ArchiveCoverage struct {
	Key        string `json:"key"`
	DocumentID string `json:"documentId"`
	SourceAge  string `json:"sourceAge"`
}
type ArchiveRecord struct {
	Version         int      `json:"version"`
	Artifact        string   `json:"artifact"`
	Category        string   `json:"category"`
	AuthorID        string   `json:"authorId"`
	AuthorName      string   `json:"authorName"`
	RecordingAge    string   `json:"recordingAge"`
	RecordingYear   int64    `json:"recordingYear"`
	CreationYear    int64    `json:"creationYear"`
	SourceAge       string   `json:"sourceAge"`
	FromYear        int64    `json:"fromYear"`
	ToYear          int64    `json:"toYear"`
	RelatedEntities []string `json:"relatedEntities"`
	SourceIDs       []string `json:"sourceIds"`
	CoverageKey     string   `json:"coverageKey"`
	Reliability     string   `json:"reliability"`
	Knowledge       string   `json:"knowledge"`
	Evidence        string   `json:"evidence"`
	SourceSnapshot  string   `json:"sourceSnapshot"`
	Revised         bool     `json:"revised,omitempty"`
	AuthoredEdits   bool     `json:"authoredEdits,omitempty"`
}
type CustodyEvent struct {
	Action string `json:"action"`
	AgeID  string `json:"ageId"`
	Year   int64  `json:"year"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

func cloneArchive(a *ArchiveConfig) *ArchiveConfig {
	if a == nil {
		return nil
	}
	b, _ := json.Marshal(a)
	var c ArchiveConfig
	_ = json.Unmarshal(b, &c)
	return &c
}
func cloneArchiveRecord(a *ArchiveRecord) *ArchiveRecord {
	if a == nil {
		return nil
	}
	b, _ := json.Marshal(a)
	var c ArchiveRecord
	_ = json.Unmarshal(b, &c)
	return &c
}
func archiveKey(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}
func validArchiveYear(y int64) bool { return y >= -9007199254740991 && y <= 9007199254740991 }
func (d *Document) validateArchives() error {
	for _, e := range d.Entities {
		c := e.Archive
		if c == nil {
			continue
		}
		owner := d.Entity(e.AgeID, c.OwnerID)
		if e.Kind != "archive" || c.Version != ArchiveModelVersion || c.OwnerID == e.ID || owner == nil || Registry.Types[owner.Kind].Category == "records" {
			return fmt.Errorf("archive %s requires a valid owner", e.Name)
		}
		if !slices.Contains(ArchivePurposes, c.Purpose) || !slices.Contains([]string{"none", "minimal", "moderate", "detailed", "exhaustive"}, c.Intensity) || !slices.Contains([]string{"direct", "institutional", "transmitted"}, c.KnowledgeScope) || !slices.Contains([]string{"preserve", "review"}, c.Retention) {
			return fmt.Errorf("invalid archive settings")
		}
		for _, category := range c.Categories {
			if !slices.Contains(ArchiveCategories, category) {
				return fmt.Errorf("unknown archive document category")
			}
		}
		seen := map[string]bool{}
		for _, s := range c.Sources {
			if s.ID == "" || d.Age(s.ArrivalAge) == nil || seen[s.ID] || d.Entity(s.SourceAge, s.EntityID) == nil || strings.TrimSpace(s.Evidence) == "" || !slices.Contains([]string{"observation", "messenger", "diplomacy", "trade", "expedition", "intelligence", "inherited-record"}, s.Channel) || !slices.Contains([]string{"established", "corroborated", "incidental"}, s.Strength) || s.FromYear > s.ToYear || !validArchiveYear(s.FromYear) || !validArchiveYear(s.ToYear) || !validArchiveYear(s.ArrivalYear) || (s.SourceAge == s.ArrivalAge && s.ArrivalYear < s.ToYear) {
				return fmt.Errorf("invalid knowledge source: supply evidence, covered dates and a plausible arrival year")
			}
			seen[s.ID] = true
			for _, category := range s.Categories {
				if !slices.Contains(ArchiveCategories, category) {
					return fmt.Errorf("unknown knowledge source category")
				}
			}
		}
	}
	for _, f := range d.LibraryFiles {
		if f.Record != nil {
			r := f.Record
			if r.Version != ArchiveModelVersion || len(r.CoverageKey) != 64 || r.SourceAge == "" || r.AuthorID == "" || r.RecordingAge == "" || !slices.Contains(ArchiveCategories, r.Category) || len(r.SourceIDs) == 0 || !validArchiveYear(r.RecordingYear) || !validArchiveYear(r.CreationYear) || !validArchiveYear(r.FromYear) || !validArchiveYear(r.ToYear) || r.FromYear > r.ToYear {
				return fmt.Errorf("invalid archive provenance")
			}
		}
		if !slices.Contains([]string{"", "present", "lost", "destroyed"}, f.Status) {
			return fmt.Errorf("invalid document preservation state")
		}
	}
	return nil
}
