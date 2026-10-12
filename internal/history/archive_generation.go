package history

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type GatherRequest struct {
	Revision      int    `json:"revision"`
	AgeID         string `json:"ageId"`
	ArchiveID     string `json:"archiveId"`
	Scope         string `json:"scope"`
	SourceAge     string `json:"sourceAge"`
	FromYear      *int64 `json:"fromYear,omitempty"`
	ToYear        *int64 `json:"toYear,omitempty"`
	RecordingYear int64  `json:"recordingYear"`
	Preview       bool   `json:"preview"`
	ReviseID      string `json:"reviseId,omitempty"`
}
type GatherDecision struct {
	SourceID   string `json:"sourceId"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
	DocumentID string `json:"documentId,omitempty"`
}
type GatherResult struct {
	Created []GatherDecision `json:"created"`
	Skipped []GatherDecision `json:"skipped"`
	Preview bool             `json:"preview"`
}
type GeneratedDocument struct {
	Entity     Entity
	Record     ArchiveRecord
	Name, MIME string
	Data       []byte
}
type archiveFact struct {
	age, id, entity, title, body, category, artifact string
	year                                             int64
	significance                                     int
	related                                          []string
	rows                                             [][]string
}

func yearLabel(y int64) string { return strconv.FormatInt(y, 10) }
func archiveSourceIdentity(id string) string {
	if strings.HasPrefix(id, "fact/") || strings.HasPrefix(id, "component/") || strings.HasPrefix(id, "survey/") || strings.HasPrefix(id, "portrait/") {
		if n := strings.LastIndex(id, "/"); n > 0 {
			return id[:n]
		}
	}
	return id
}
func existedBy(e *Entity, year int64) bool {
	if e == nil {
		return false
	}
	var p map[string]any
	_ = json.Unmarshal(e.Properties, &p)
	start, known := parseYear(dateValue(p, true))
	return !known || start <= year
}
func (d *Document) archiveStateYear(age string) int64 {
	latest := int64(-9007199254740991)
	for _, y := range d.Years {
		if y.AgeID == age && len(y.Events) > 0 && y.Number > latest {
			latest = y.Number
		}
	}
	return latest
}
func relationCovers(r Relation, y int64) bool {
	start, ok := parseYear(r.ValidFrom)
	if !ok || y < start {
		return false
	}
	if r.ValidTo != "" {
		end, ok := parseYear(r.ValidTo)
		if !ok || y > end {
			return false
		}
	}
	return true
}

// One dated, explicitly authored administrative edge; never spatial containment or
// unrestricted reachability. Allies/enemies/trade require an explicit delivery record.
func (d *Document) archiveKnowledge(c *ArchiveConfig, age string, f archiveFact, now int64) (string, string, string, bool) {
	if f.age == age && f.entity == c.OwnerID && f.year <= now {
		return "direct", "Owner's own record", "first-hand", true
	}
	if f.age == age && f.year <= now {
		for _, r := range d.Relations {
			if r.AgeID == age && !r.Automatic && r.Source == c.OwnerID && r.Destination == f.entity && r.Kind == "participated-in" && relationCovers(r, f.year) {
				return "direct", "Dated participation " + r.ID, "participant account", true
			}
		}
	}
	if c.KnowledgeScope != "direct" && f.age == age && f.year <= now {
		for _, r := range d.Relations {
			if r.AgeID != age || r.Automatic || !relationCovers(r, f.year) {
				continue
			}
			administration := r.Source == c.OwnerID && r.Destination == f.entity && slices.Contains([]string{"governs", "controls"}, r.Kind)
			institution := r.Destination == c.OwnerID && r.Source == f.entity && slices.Contains([]string{"member-of", "subordinate-to"}, r.Kind)
			// Institutional access covers official facts, not private biographies and doctrine.
			if (administration || institution) && slices.Contains([]string{"chronicles", "administrative", "economic"}, f.category) {
				return "institutional", "Dated relationship " + r.ID + " (" + r.Kind + ")", "institutional record", true
			}
		}
	}
	if c.KnowledgeScope == "transmitted" {
		for _, s := range c.Sources {
			if s.SourceAge == f.age && s.EntityID == f.entity && s.Strength != "incidental" && d.sourceHasArrived(s, age, now) && f.year >= s.FromYear && f.year <= s.ToYear && slices.Contains(s.Categories, f.category) {
				return "transmitted", s.Channel + ": " + s.Evidence + "; received year " + yearLabel(s.ArrivalYear), s.Strength + " report", true
			}
		}
	}
	return "", "", "", false
}
func (d *Document) archiveFacts(ages map[string]bool, now int64, currentAge string) []archiveFact {
	facts := []archiveFact{}
	latestStateYear := d.archiveStateYear(currentAge)
	for _, y := range d.Years {
		if !ages[y.AgeID] {
			continue
		}
		for _, event := range y.Events {
			e := d.Entity(y.AgeID, event.EntityID)
			if e == nil || e.Deleted || Registry.Types[e.Kind].Category == "records" {
				continue
			}
			significance := 2
			if Registry.Types[e.Kind].Category == "politics" || Registry.Types[e.Kind].Category == "history" {
				significance = 3
			}
			facts = append(facts, archiveFact{age: y.AgeID, id: "event/" + event.ID, entity: e.ID, title: event.Title, body: event.Description, category: "chronicles", artifact: "chronicle", year: y.Number, significance: significance, related: []string{e.ID}})
		}
	}
	fields := []struct{ key, category string }{{"population", "administrative"}, {"taxation", "administrative"}, {"inventory", "administrative"}, {"laws", "administrative"}, {"harvest", "economic"}, {"production", "economic"}, {"prices", "economic"}, {"debts", "economic"}, {"shipments", "economic"}, {"summary", "personal"}, {"biography", "personal"}, {"traditions", "cultural"}, {"customs", "cultural"}, {"rituals", "cultural"}}
	for _, e := range d.Entities {
		if !ages[e.AgeID] || e.Deleted || Registry.Types[e.Kind].Category == "records" {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal(e.Properties, &p)
		// Undated state is a present observation, never backdated into another Age.
		if e.AgeID == currentAge && existedBy(&e, now) && now >= latestStateYear {
			for _, field := range fields {
				v, ok := p[field.key]
				if !ok || v == nil || v == "" {
					continue
				}
				b, _ := json.Marshal(v)
				value := string(b)
				if s, ok := v.(string); ok {
					value = s
				}
				facts = append(facts, archiveFact{age: e.AgeID, id: "fact/" + e.ID + "/" + field.key + "/" + archiveKey(value), entity: e.ID, title: e.Name + " — " + field.key, body: value, category: field.category, artifact: "register", year: now, significance: 1, related: []string{e.ID}, rows: [][]string{{"Entity", "Field", "Recorded value"}, {e.Name, field.key, value}}})
			}
			if e.Content != nil {
				for _, part := range e.Content.Components {
					if part.EntityID != "" || part.Description == "" {
						continue
					}
					facts = append(facts, archiveFact{age: e.AgeID, id: "component/" + e.ID + "/" + part.ID + "/" + archiveKey(part.Description), entity: e.ID, title: part.Title, body: part.Description, category: "cultural", artifact: "account", year: now, significance: 0, related: []string{e.ID}})
				}
			}
		}
	}
	for _, r := range d.Relations {
		if !ages[r.AgeID] {
			continue
		}
		from, ok := parseYear(r.ValidFrom)
		if !ok {
			continue
		}
		a, b := d.Entity(r.AgeID, r.Source), d.Entity(r.AgeID, r.Destination)
		if a == nil || b == nil || a.Deleted || b.Deleted || !existedBy(a, from) || !existedBy(b, from) {
			continue
		}
		family := Registry.Relations[r.Kind].Family
		category := ""
		switch family {
		case "genealogical":
			category = "genealogies"
		case "political", "organizational":
			category = "administrative"
		}
		if category == "" || r.Automatic {
			continue
		}
		facts = append(facts, archiveFact{age: r.AgeID, id: "relation/" + r.ID, entity: a.ID, title: a.Name + " — " + Registry.Relations[r.Kind].Label + " — " + b.Name, body: r.Description, category: category, artifact: "register", year: from, significance: 2, related: []string{a.ID, b.ID}, rows: [][]string{{"Source", "Relationship", "Destination", "From year", "Description"}, {a.Name, Registry.Relations[r.Kind].Label, b.Name, r.ValidFrom, r.Description}}})
		if end, known := parseYear(r.ValidTo); known {
			facts = append(facts, archiveFact{age: r.AgeID, id: "relation-end/" + r.ID, entity: a.ID, title: "End of " + a.Name + " / " + b.Name + " relationship", body: r.Description, category: category, artifact: "register", year: end, significance: 2, related: []string{a.ID, b.ID}, rows: [][]string{{"Source", "Relationship", "Destination", "End year"}, {a.Name, Registry.Relations[r.Kind].Label, b.Name, r.ValidTo}}})
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		a, b := facts[i], facts[j]
		if a.age != b.age {
			return a.age < b.age
		}
		if a.year != b.year {
			return a.year < b.year
		}
		return a.id < b.id
	})
	return facts
}
func safeCSV(value string) string {
	if len(value) > 0 && strings.ContainsAny(value[:1], "=+-@\t\r") {
		return "'" + value
	}
	return value
}
func renderArchiveDocument(f archiveFact, r ArchiveRecord) ([]byte, string, string) {
	if f.artifact == "illustration" {
		b, ext, mime, _ := archivePortrait(f.body)
		return b, ext, mime
	}
	if f.artifact == "survey" {
		return []byte(f.body), "svg", "image/svg+xml"
	}
	if f.artifact == "register" {
		var out bytes.Buffer
		w := csv.NewWriter(&out)
		rows := [][]string{{"Title", f.title}, {"Attributed source", r.AuthorName}, {"Event / observed year", yearLabel(f.year)}, {"Recording year", yearLabel(r.RecordingYear)}, {"Evidence", r.Evidence}, {"Reliability", r.Reliability}}
		rows = append(rows, f.rows...)
		for _, row := range rows {
			clean := append([]string(nil), row...)
			for i := range clean {
				clean[i] = safeCSV(clean[i])
			}
			_ = w.Write(clean)
		}
		w.Flush()
		return out.Bytes(), "csv", "text/csv"
	}
	account := fmt.Sprintf("In year %d, the following was recorded: %s.", f.year, f.title)
	if r.Knowledge == "transmitted" {
		account = fmt.Sprintf("An account received in year %d reports this occurrence in year %d: %s.", r.RecordingYear, f.year, f.title)
	}
	if strings.TrimSpace(f.body) != "" {
		account += "\n\n" + f.body
	}
	text := fmt.Sprintf("# %s\n\nAttributed to: %s\n\nEvent / observed year: %d\nRecording year: %d\n\n%s\n\n## Source note\n\n%s\n\nReliability: %s. This account records the archive's available information; it does not establish world truth.\n", f.title, r.AuthorName, f.year, r.RecordingYear, account, r.Evidence, r.Reliability)
	return []byte(text), "md", "text/markdown"
}
func (d *Document) GatherDocuments(req GatherRequest) ([]GeneratedDocument, GatherResult, error) {
	result := GatherResult{Created: []GatherDecision{}, Skipped: []GatherDecision{}, Preview: req.Preview}
	archive := d.Entity(req.AgeID, req.ArchiveID)
	if archive == nil || archive.Deleted || archive.Kind != "archive" || archive.Archive == nil {
		return nil, result, fmt.Errorf("configure the archive owner and documentation settings first")
	}
	c := archive.Archive
	var archiveDates map[string]any
	_ = json.Unmarshal(archive.Properties, &archiveDates)
	if start, known := parseYear(dateValue(archiveDates, true)); known && req.RecordingYear < start {
		return nil, result, fmt.Errorf("recording year precedes the archive's foundation")
	}
	if end, known := parseYear(dateValue(archiveDates, false)); known && req.RecordingYear > end {
		return nil, result, fmt.Errorf("recording year follows the archive's closure")
	}
	owner := d.Entity(req.AgeID, c.OwnerID)
	if owner == nil || owner.Deleted {
		return nil, result, fmt.Errorf("the archive needs an active owner in this Age")
	}
	if err := d.validateArchives(); err != nil {
		return nil, result, err
	}
	if !validArchiveYear(req.RecordingYear) || (req.FromYear != nil && !validArchiveYear(*req.FromYear)) || (req.ToYear != nil && !validArchiveYear(*req.ToYear)) || (req.FromYear != nil && req.ToYear != nil && *req.FromYear > *req.ToYear) {
		return nil, result, fmt.Errorf("invalid gathering dates")
	}
	ages := map[string]bool{}
	switch req.Scope {
	case "age", "range":
		if d.Age(req.SourceAge) == nil {
			return nil, result, fmt.Errorf("select a source Age")
		}
		ages[req.SourceAge] = true
	case "history":
		// Historical lineage, not every alternative universe on the Time Graph.
		for a := d.Age(req.AgeID); a != nil && !ages[a.ID]; a = d.Age(a.InheritedFrom) {
			ages[a.ID] = true
		}
		for _, s := range c.Sources {
			ages[s.SourceAge] = true
		}
	default:
		return nil, result, fmt.Errorf("select age, range or history scope")
	}
	covered := map[string]bool{}
	for _, x := range c.Coverage {
		covered[x.Key] = true
	}
	for _, f := range d.LibraryFiles {
		if f.AgeID == req.AgeID && f.ArchiveID == archive.ID && f.Record != nil {
			covered[f.Record.CoverageKey] = true
		}
	}
	var revision *LibraryFile
	if req.ReviseID != "" {
		revision = d.LibraryFile(req.AgeID, req.ReviseID)
		if revision == nil || revision.ArchiveID != archive.ID || revision.Record == nil || revision.Status == "lost" || revision.Status == "destroyed" {
			return nil, result, fmt.Errorf("select a preserved generated document to revise")
		}
	}
	output := []GeneratedDocument{}
	if req.RecordingYear < d.archiveStateYear(req.AgeID) {
		result.Skipped = append(result.Skipped, GatherDecision{Title: "Undated current-state observations", Reason: "Current population, economic figures, profiles and surveys cannot be backdated before the latest dated state in this Age. Dated events are still considered."})
	}
	for _, f := range d.allArchiveFacts(ages, req, c.OwnerID) {
		if req.Scope == "range" && ((req.FromYear != nil && f.year < *req.FromYear) || (req.ToYear != nil && f.year > *req.ToYear)) {
			continue
		}
		// Stable across inherited Ages and repeated state surveys; unchanged facts aren't new facts.
		key := archiveKey(f.id, yearLabel(f.year))
		if strings.HasPrefix(f.id, "fact/") || strings.HasPrefix(f.id, "component/") || strings.HasPrefix(f.id, "survey/") || strings.HasPrefix(f.id, "portrait/") || strings.HasPrefix(f.id, "compilation/") {
			key = archiveKey(f.id)
		}
		if revision != nil {
			if revision.Record.SourceAge != f.age || len(revision.Record.SourceIDs) == 0 || archiveSourceIdentity(revision.Record.SourceIDs[0]) != archiveSourceIdentity(f.id) {
				continue
			}
		}
		reason := ""
		knowledge, evidence, reliability, known := d.archiveKnowledge(c, req.AgeID, f, req.RecordingYear)
		if !known && len(f.related) == 2 {
			other := f
			other.entity = f.related[1]
			knowledge, evidence, reliability, known = d.archiveKnowledge(c, req.AgeID, other, req.RecordingYear)
		}
		preferred := map[string][]string{"personal": {"personal", "genealogies"}, "household": {"genealogies", "administrative"}, "religious": {"cultural"}, "civic": {"administrative", "economic"}, "military": {"chronicles", "geographical"}, "diplomatic": {"chronicles", "genealogies"}, "commercial": {"economic"}, "royal": {"administrative", "chronicles"}, "scholarly": {"geographical", "cultural"}}
		significance := f.significance
		if slices.Contains(preferred[c.Purpose], f.category) {
			significance++
		}
		if !known {
			reason = "No dated observation, administrative relationship or received information source"
		} else if !slices.Contains(c.Categories, f.category) {
			reason = "Document category disabled"
		} else if c.Intensity == "none" {
			reason = "Documentation disabled"
		} else if c.Intensity == "minimal" && significance < 3 {
			reason = "Below minimal recording significance"
		} else if c.Intensity == "moderate" && significance < 2 {
			reason = "Detailed observation omitted at moderate intensity"
		} else if c.Intensity == "detailed" && significance < 1 {
			reason = "Fine-grained component reserved for exhaustive documentation"
		} else if covered[key] && revision == nil {
			reason = "Already documented (including retained coverage of lost or transferred records)"
		} else if len(output) >= 256 {
			reason = "Batch limit reached; gather again to continue"
		}
		if reason != "" {
			result.Skipped = append(result.Skipped, GatherDecision{SourceID: f.id, Title: f.title, Reason: reason})
			continue
		}
		artifact := f.artifact
		if knowledge == "transmitted" && artifact != "register" {
			artifact = "report"
		}
		record := ArchiveRecord{Version: ArchiveModelVersion, Artifact: artifact, Category: f.category, AuthorID: owner.ID, AuthorName: owner.Name, RecordingAge: req.AgeID, RecordingYear: req.RecordingYear, CreationYear: req.RecordingYear, SourceAge: f.age, FromYear: f.year, ToYear: f.year, RelatedEntities: f.related, SourceIDs: []string{f.id}, CoverageKey: key, Reliability: reliability, Knowledge: knowledge, Evidence: evidence, SourceSnapshot: f.body, Revised: revision != nil}
		if f.artifact == "survey" || f.artifact == "illustration" {
			record.Artifact = f.artifact
			record.SourceSnapshot = "Visual source preserved in immutable file; source digest " + archiveKey(f.body)
		}
		data, ext, mime := renderArchiveDocument(f, record)
		id := "archive-" + archiveKey(archive.ID, key)[:32]
		if revision != nil {
			id = revision.ID
		}
		// Generated records deliberately omit entity startYear: gathering must not recursively create events.
		p, _ := json.Marshal(map[string]any{"summary": f.category + "; recorded year " + yearLabel(req.RecordingYear)})
		e := Entity{ID: id, AgeID: req.AgeID, Name: f.title, Kind: "document", Properties: p}
		if len([]rune(e.Name)) > 150 {
			e.Name = string([]rune(e.Name)[:150])
		}
		covered[key] = true
		output = append(output, GeneratedDocument{Entity: e, Record: record, Name: id + "." + ext, MIME: mime, Data: data})
		result.Created = append(result.Created, GatherDecision{SourceID: f.id, Title: f.title, Reason: knowledge + ": " + evidence, DocumentID: id})
	}
	if revision != nil && len(output) == 0 && len(result.Skipped) == 0 {
		result.Skipped = append(result.Skipped, GatherDecision{Title: "Original source unavailable in this selection", Reason: "Select the document's source Age and period. Removed source facts remain preserved in the original file; they are never invented during revision."})
	}
	return output, result, nil
}
