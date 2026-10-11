package history

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Years are Age-scoped historical records, not ordinary profile entities.
type Year struct {
	ID     string      `json:"id"`
	AgeID  string      `json:"ageId"`
	Number int64       `json:"number"`
	Events []YearEvent `json:"events"`
}
type YearEvent struct {
	ComponentID string `json:"componentId,omitempty"`
	ID          string `json:"id"`
	EntityID    string `json:"entityId"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}
type YearOrigin struct {
	AgeID  string `json:"ageId"`
	YearID string `json:"yearId"`
	Number int64  `json:"number"`
}

func (d *Document) Year(age, id string) *Year {
	for i := range d.Years {
		if d.Years[i].AgeID == age && d.Years[i].ID == id {
			return &d.Years[i]
		}
	}
	return nil
}

func parseYear(v any) (int64, bool) {
	var value string
	switch v := v.(type) {
	case string:
		value = strings.TrimSpace(v)
	case float64:
		value = strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return 0, false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	return n, err == nil && n >= -9007199254740991 && n <= 9007199254740991
}
func dateValue(p map[string]any, start bool) any {
	keys := []string{"startYear", "born", "startDate", "foundingYear", "created"}
	if !start {
		keys = []string{"endYear", "died", "endDate", "dissolutionYear", "destroyed"}
	}
	for _, key := range keys {
		if v, ok := p[key]; ok {
			return v
		}
	}
	return nil
}

// Legacy textual dates remain readable. New/changed dates must be whole years;
// blank dates mean unknown, while zero and negative years are valid.
func validateYearEdit(e Entity, old *Entity) error {
	var p, previous map[string]any
	_ = json.Unmarshal(e.Properties, &p)
	if old != nil {
		_ = json.Unmarshal(old.Properties, &previous)
	}
	for _, start := range []bool{true, false} {
		v := dateValue(p, start)
		if v == nil || v == "" || reflect.DeepEqual(v, dateValue(previous, start)) {
			continue
		}
		if _, ok := parseYear(v); !ok {
			return fmt.Errorf("start and end years must be whole numbers (negative years and year zero are allowed)")
		}
	}
	return nil
}

// Reconcile derived events on writes, including builder sync and inherited
// state. Stable source keys make retries, date edits and name edits idempotent.
// Empty year records remain available as historical anchors for new Ages.
func (d *Document) SyncYears() {
	byKey := map[string]int{}
	kept := []Year{}
	for _, y := range d.Years {
		if d.Age(y.AgeID) == nil {
			continue
		}
		y.Events = []YearEvent{}
		key := y.AgeID + "/" + strconv.FormatInt(y.Number, 10)
		if _, exists := byKey[key]; exists {
			continue
		}
		byKey[key] = len(kept)
		kept = append(kept, y)
	}
	d.Years = kept

	for _, e := range d.Entities {
		if e.Deleted || !yearEntityEligible(e.Kind) {
			continue
		}
		var p map[string]any
		if json.Unmarshal(e.Properties, &p) != nil {
			continue
		}
		description := ""
		for _, key := range []string{"summary", "description", "biography", "notes"} {
			if text, ok := p[key].(string); ok && text != "" {
				description = text
				break
			}
		}
		d.addYearEvents(e, "", e.Name, description, p, byKey)
		if e.Content != nil {
			for _, part := range e.Content.Components {
				if part.EntityID != "" {
					continue
				}
				dates := map[string]any{}
				if part.StartYear != nil {
					dates["startYear"] = strconv.FormatInt(*part.StartYear, 10)
				}
				if part.EndYear != nil {
					dates["endYear"] = strconv.FormatInt(*part.EndYear, 10)
				}
				d.addYearEvents(e, part.ID, part.Title, part.Description, dates, byKey)
			}
		}
	}

	for i := range d.Years {
		sort.Slice(d.Years[i].Events, func(a, b int) bool { return d.Years[i].Events[a].ID < d.Years[i].Events[b].ID })
	}
	sort.Slice(d.Years, func(i, j int) bool {
		if d.Years[i].AgeID != d.Years[j].AgeID {
			return d.Years[i].AgeID < d.Years[j].AgeID
		}
		return d.Years[i].Number < d.Years[j].Number
	})
}

func (d *Document) validateYears() error {
	seen, numbers := map[string]bool{}, map[string]bool{}
	for _, y := range d.Years {
		key, numeric := y.AgeID+"/"+y.ID, y.AgeID+"/"+strconv.FormatInt(y.Number, 10)
		if y.ID == "" || d.Age(y.AgeID) == nil || seen[key] || numbers[numeric] || y.Number < -9007199254740991 || y.Number > 9007199254740991 {
			return fmt.Errorf("invalid or duplicate year record")
		}
		seen[key], numbers[numeric] = true, true
		events := map[string]bool{}
		for _, ev := range y.Events {
			e := d.Entity(y.AgeID, ev.EntityID)
			expected := ev.EntityID + "/" + ev.Kind
			if ev.ComponentID != "" {
				expected = ev.EntityID + "/component/" + ev.ComponentID + "/" + ev.Kind
				found := false
				if e != nil && e.Content != nil {
					for _, part := range e.Content.Components {
						if part.ID == ev.ComponentID && part.EntityID == "" {
							found = true
						}
					}
				}
				if !found {
					return fmt.Errorf("year event component not found")
				}
			}
			if e == nil || e.Deleted || !yearEntityEligible(e.Kind) || ev.ID != expected || (ev.Kind != "start" && ev.Kind != "end") || events[ev.ID] {
				return fmt.Errorf("invalid year event")
			}
			events[ev.ID] = true
		}
	}
	for _, a := range d.Ages {
		if o := a.StartedFromYear; o != nil {
			if o.AgeID == "" || o.YearID == "" || o.Number < -9007199254740991 || o.Number > 9007199254740991 {
				return fmt.Errorf("invalid Age year origin")
			}
			// Keep provenance after the source Age is deleted.
			if d.Age(o.AgeID) != nil {
				y := d.Year(o.AgeID, o.YearID)
				if y == nil || y.Number != o.Number {
					return fmt.Errorf("Age origin year not found")
				}
			}
		}
	}
	return nil
}

// Specialized terrains/maps and the author's Writing workspace do not generate
// fictional historical events. All logical entity categories opt in by default.
func yearEntityEligible(kind string) bool {
	return kind != "terrain" && kind != "map" && kind != "writing-document"
}
func normalizeEntityYears(e *Entity, old *Entity) {
	var p, previous map[string]any
	_ = json.Unmarshal(e.Properties, &p)
	if old != nil {
		_ = json.Unmarshal(old.Properties, &previous)
	}
	changed := false
	for _, keys := range [][]string{{"startYear", "born", "startDate", "foundingYear", "created"}, {"endYear", "died", "endDate", "dissolutionYear", "destroyed"}} {
		for _, key := range keys {
			if value, present := p[key]; present && (old == nil || !reflect.DeepEqual(value, previous[key])) {
				p[keys[0]] = value
				changed = true
				break
			}
		}
	}
	if changed {
		e.Properties, _ = json.Marshal(p)
	}
}
func (d *Document) addYearEvents(e Entity, component, title, description string, p map[string]any, byKey map[string]int) {
	for _, start := range []bool{true, false} {
		n, ok := parseYear(dateValue(p, start))
		if !ok {
			continue
		}
		value := strconv.FormatInt(n, 10)
		key := e.AgeID + "/" + value
		i, exists := byKey[key]
		if !exists {
			i = len(d.Years)
			byKey[key] = i
			d.Years = append(d.Years, Year{ID: "year/" + value, AgeID: e.AgeID, Number: n, Events: []YearEvent{}})
		}
		kind, verb := "start", "began"
		if !start {
			kind, verb = "end", "ended"
		}
		if component == "" {
			if e.Kind == "character" {
				if start {
					verb = "was born"
				} else {
					verb = "died"
				}
			} else if Registry.Types[e.Kind].Category == "organizations" {
				if start {
					verb = "was founded"
				} else {
					verb = "was dissolved"
				}
			}
		}
		id := e.ID + "/" + kind
		if component != "" {
			id = e.ID + "/component/" + component + "/" + kind
		}
		d.Years[i].Events = append(d.Years[i].Events, YearEvent{ID: id, EntityID: e.ID, ComponentID: component, Kind: kind, Title: title + " " + verb, Description: description})
	}
}
