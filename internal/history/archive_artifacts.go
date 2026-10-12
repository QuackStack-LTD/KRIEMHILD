package history

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"sort"
	"strings"
)

func (d *Document) sourceHasArrived(s KnowledgeSource, age string, now int64) bool {
	if s.ArrivalAge == age {
		return s.ArrivalYear <= now
	}
	seen := map[string]bool{}
	for a := d.Age(age); a != nil && !seen[a.ID]; a = d.Age(a.InheritedFrom) {
		seen[a.ID] = true
		if a.InheritedFrom == s.ArrivalAge {
			if a.StartedFromYear != nil && a.StartedFromYear.AgeID == s.ArrivalAge && s.ArrivalYear > a.StartedFromYear.Number {
				return false
			}
			return true
		}
	}
	return false
}

// Surveys use the known entity's exact representation, never the whole omniscient map.
func (d *Document) allArchiveFacts(ages map[string]bool, req GatherRequest, owner string) []archiveFact {
	facts := d.archiveFacts(ages, req.RecordingYear, req.AgeID)
	if !ages[req.AgeID] {
		return facts
	}
	for _, rep := range d.Representations {
		if req.RecordingYear < d.archiveStateYear(req.AgeID) || rep.AgeID != req.AgeID || rep.Shape.Deleted || len(rep.Shape.Points) == 0 {
			continue
		}
		e := d.Entity(rep.AgeID, rep.EntityID)
		if e == nil || e.Deleted {
			continue
		}
		svg := surveySVG(e.Name, rep.Shape.Geometry, rep.Shape.Points)
		signature, _ := json.Marshal(rep.Shape.Points)
		facts = append(facts, archiveFact{age: req.AgeID, id: "survey/" + rep.ID + "/" + archiveKey(string(signature), rep.Shape.Geometry), entity: e.ID, title: e.Name + " — territorial survey", body: svg, category: "geographical", artifact: "survey", year: req.RecordingYear, significance: 1, related: []string{e.ID}})
	}
	if req.RecordingYear >= d.archiveStateYear(req.AgeID) {
		for _, e := range d.Entities {
			if e.AgeID != req.AgeID || e.Deleted || !existedBy(&e, req.RecordingYear) {
				continue
			}
			var p struct {
				Portrait string `json:"portrait"`
			}
			_ = json.Unmarshal(e.Properties, &p)
			if _, _, _, ok := archivePortrait(p.Portrait); !ok {
				continue
			}
			facts = append(facts, archiveFact{age: req.AgeID, id: "portrait/" + e.ID + "/" + archiveKey(p.Portrait), entity: e.ID, title: e.Name + " — preserved portrait or emblem", body: p.Portrait, category: "personal", artifact: "illustration", year: req.RecordingYear, significance: 1, related: []string{e.ID}})
		}
	}
	// An index is itself a dated artifact. It indexes preserved sources already in
	// this archive, not everything in the world's Library, and never indexes indexes.
	ids := []string{}
	rows := [][]string{{"Document", "Type", "Source Age", "Covered year", "Recording year", "File"}}
	fs := append([]LibraryFile(nil), d.LibraryFiles...)
	sort.Slice(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
	for _, f := range fs {
		if f.AgeID != req.AgeID || f.ArchiveID != req.ArchiveID || f.Status == "lost" || f.Status == "destroyed" || f.Record == nil || f.Record.Category == "compilations" {
			continue
		}
		if f.Record.RecordingAge == req.AgeID && f.Record.RecordingYear > req.RecordingYear {
			continue
		}
		e := d.Entity(req.AgeID, f.ID)
		if e == nil || e.Deleted {
			continue
		}
		ids = append(ids, f.ID+"/"+f.Versions[len(f.Versions)-1].Hash)
		rows = append(rows, []string{e.Name, f.Record.Artifact, f.Record.SourceAge, yearLabel(f.Record.FromYear), yearLabel(f.Record.RecordingYear), f.Versions[len(f.Versions)-1].Name})
	}
	if len(ids) > 0 {
		facts = append(facts, archiveFact{age: req.AgeID, id: "compilation/" + archiveKey(ids...), entity: owner, title: "Index of preserved archive documents", body: strings.Join(ids, "\n"), category: "compilations", artifact: "register", year: req.RecordingYear, significance: 1, related: []string{owner}, rows: rows})
	}
	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].age != facts[j].age {
			return facts[i].age < facts[j].age
		}
		if facts[i].year != facts[j].year {
			return facts[i].year < facts[j].year
		}
		return facts[i].id < facts[j].id
	})
	return facts
}
func archivePortrait(data string) ([]byte, string, string, bool) {
	for _, format := range []struct{ prefix, extension, mime string }{{"data:image/png;base64,", "png", "image/png"}, {"data:image/jpeg;base64,", "jpg", "image/jpeg"}, {"data:image/webp;base64,", "webp", "image/webp"}} {
		if !strings.HasPrefix(data, format.prefix) || len(data) > 2<<20 {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, format.prefix))
		if err != nil || len(b) == 0 {
			return nil, "", "", false
		}
		return b, format.extension, format.mime, true
	}
	return nil, "", "", false
}
func surveySVG(title, kind string, points [][2]float64) string {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range points {
		minX = math.Min(minX, p[0])
		maxX = math.Max(maxX, p[0])
		minY = math.Min(minY, p[1])
		maxY = math.Max(maxY, p[1])
	}
	w, h := math.Max(maxX-minX, 1), math.Max(maxY-minY, 1)
	scale := math.Min(700/w, 430/h)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 800 550"><rect width="800" height="550" fill="white"/><text x="40" y="35" font-family="serif" font-size="20">%s</text>`, html.EscapeString(title))
	if len(points) == 1 {
		fmt.Fprintf(&b, `<circle cx="400" cy="270" r="5" fill="#35382f"/>`)
	} else {
		tag := "polyline"
		if kind == "polygon" {
			tag = "polygon"
		}
		fmt.Fprintf(&b, `<%s fill="none" stroke="#35382f" stroke-width="2" points="`, tag)
		for _, p := range points {
			fmt.Fprintf(&b, "%.3f,%.3f ", 50+(p[0]-minX)*scale, 60+(p[1]-minY)*scale)
		}
		b.WriteString(`"/>`)
	}
	b.WriteString(`<text x="40" y="525" font-family="serif" font-size="13">Surveyed representation; local terrain coordinates. Surroundings not surveyed.</text></svg>`)
	return b.String()
}
