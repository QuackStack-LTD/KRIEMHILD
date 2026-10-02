package project

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

type Month struct {
	Name string `json:"name"`
	Days int    `json:"days"`
}
type Calendar struct {
	Epoch     string  `json:"epoch"`
	Era       string  `json:"era"`
	Months    []Month `json:"months"`
	Week      int     `json:"week"`
	YearZero  bool    `json:"yearZero"`
	LeapEvery int     `json:"leapEvery"`
	LeapDays  int     `json:"leapDays"`
}
type Chronology struct {
	Age      string `json:"age"`
	Baseline string `json:"baseline"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	Branch   string `json:"branch"`
	Calendar string `json:"calendar,omitempty"`
}
type FictionalDate struct {
	Precision string `json:"precision"`
	Tick      string `json:"tick,omitempty"`
	End       string `json:"end,omitempty"`
	Relative  string `json:"relative,omitempty"`
}
type Change struct {
	Target string `json:"target"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}
type Event struct {
	Age     string        `json:"age"`
	Date    FictionalDate `json:"date"`
	Until   string        `json:"until,omitempty"`
	Track   string        `json:"track,omitempty"`
	Causes  []string      `json:"causes,omitempty"`
	Changes []Change      `json:"changes,omitempty"`
}
type HistoricalState struct {
	Records    map[string]Record `json:"records"`
	Unresolved []string          `json:"unresolved"`
	Tick       string            `json:"tick"`
	Label      string            `json:"label"`
}

func tick(s string) (*big.Int, error) {
	if len(s) == 0 || len(s) > 100 {
		return nil, fmt.Errorf("date must be a signed decimal integer of at most 100 digits")
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.String() != s {
		return nil, fmt.Errorf("use a canonical integer tick, e.g. -20, 0 or 120")
	}
	return v, nil
}
func cmp(a, b string) int { av, _ := tick(a); bv, _ := tick(b); return av.Cmp(bv) }
func validateDate(d FictionalDate) error {
	switch d.Precision {
	case "exact", "approximate", "before", "after", "range":
		if _, e := tick(d.Tick); e != nil {
			return e
		}
		if d.Precision == "range" {
			if _, e := tick(d.End); e != nil {
				return e
			}
			if cmp(d.Tick, d.End) > 0 {
				return fmt.Errorf("date range is reversed")
			}
		}
	case "unknown":
	case "relative":
		if !nameOK(d.Relative) {
			return fmt.Errorf("describe the relative date")
		}
	default:
		return fmt.Errorf("invalid date precision")
	}
	return nil
}
func validateCalendar(c *Calendar) error {
	if c == nil {
		return fmt.Errorf("calendar definition is required")
	}
	if _, e := tick(c.Epoch); e != nil {
		return e
	}
	if !nameOK(c.Era) || len(c.Months) == 0 || len(c.Months) > 60 || c.Week < 1 || c.Week > 100 || c.LeapEvery < 0 || c.LeapEvery > 1000 || c.LeapDays < 0 || c.LeapDays > 100 || ((c.LeapEvery == 0) != (c.LeapDays == 0)) {
		return fmt.Errorf("invalid calendar rules")
	}
	seen := map[string]bool{}
	for _, m := range c.Months {
		if !nameOK(m.Name) || seen[m.Name] || m.Days < 1 || m.Days > 1000 {
			return fmt.Errorf("months need unique names and 1–1000 days")
		}
		seen[m.Name] = true
	}
	return nil
}

// Tick is a day. Euclidean division preserves BCE dates and arbitrary precision.
func calendarLabel(c *Calendar, at string) string {
	day, _ := tick(at)
	epoch, _ := tick(c.Epoch)
	day.Sub(day, epoch)
	weekday := new(big.Int).Mod(new(big.Int).Set(day), big.NewInt(int64(c.Week))).Int64() + 1
	yearDays := 0
	for _, m := range c.Months {
		yearDays += m.Days
	}
	cycleYears := max(1, c.LeapEvery)
	cycleDays := yearDays*cycleYears + c.LeapDays
	cycles, remainder := new(big.Int), new(big.Int)
	cycles.DivMod(day, big.NewInt(int64(cycleDays)), remainder)
	year := new(big.Int).Mul(cycles, big.NewInt(int64(cycleYears)))
	remaining := int(remainder.Int64())
	within := 0
	for {
		length := yearDays
		if c.LeapEvery > 0 && within == cycleYears-1 {
			length += c.LeapDays
		}
		if remaining < length {
			break
		}
		remaining -= length
		within++
	}
	year.Add(year, big.NewInt(int64(within)))
	month := c.Months[len(c.Months)-1].Name
	for i, m := range c.Months {
		length := m.Days
		if i == len(c.Months)-1 && c.LeapEvery > 0 && within == cycleYears-1 {
			length += c.LeapDays
		}
		if remaining < length {
			month = m.Name
			break
		}
		remaining -= length
	}
	if !c.YearZero && year.Sign() >= 0 {
		year.Add(year, big.NewInt(1))
	}
	return fmt.Sprintf("%s %s, day %d · %s · weekday %d", year.String(), month, remaining+1, c.Era, weekday)
}
func chronology(records map[string]Record) *Chronology {
	for _, r := range records {
		if r.Kind == "chronology" {
			return r.Chronology
		}
	}
	return nil
}
func validateP2(r Record, records map[string]Record) error {
	if (r.Event != nil && r.Kind != "event") || (r.Calendar != nil && r.Kind != "calendar") || (r.Chronology != nil && r.Kind != "chronology") {
		return fmt.Errorf("temporal payload does not match record kind")
	}
	if r.SettingDate != "" {
		if r.Kind != "scene" {
			return fmt.Errorf("only scenes have setting dates")
		}
		if _, e := tick(r.SettingDate); e != nil {
			return e
		}
	}
	if r.Terrain != nil && r.Kind != "map" {
		return fmt.Errorf("only maps have terrain")
	}
	if r.Kind == "map" {
		if e := validateTerrain(r.Terrain); e != nil {
			return e
		}
		ids := map[string]bool{}
		for _, f := range r.Features {
			if !validID(f.ID) || ids[f.ID] || !nameOK(f.Name) || len(f.Points) < 2 || len(f.Points) > 10000 {
				return fmt.Errorf("invalid map feature")
			}
			ids[f.ID] = true
			if f.Kind != "river" && f.Kind != "route" && f.Kind != "border" && f.Kind != "lake" && f.Kind != "climate" && f.Kind != "biome" {
				return fmt.Errorf("unsupported map feature")
			}
			if (f.Kind == "border" || f.Kind == "lake" || f.Kind == "climate" || f.Kind == "biome") && len(f.Points) < 3 {
				return fmt.Errorf("area features need at least three points")
			}
			if f.Entity != "" && records[f.Entity].Kind != "entity" {
				return fmt.Errorf("feature owner must exist")
			}
			for _, p := range f.Points {
				if !normalized(p) {
					return fmt.Errorf("feature coordinates outside map")
				}
			}
		}
	}
	if r.Kind == "calendar" {
		return validateCalendar(r.Calendar)
	}
	if r.Kind == "chronology" {
		c := r.Chronology
		if c == nil || !validID(c.Age) || !validHash(c.Baseline) {
			return fmt.Errorf("invalid chronology")
		}
		count := 0
		for _, v := range records {
			if v.Kind == "chronology" {
				count++
			}
		}
		if count != 1 {
			return fmt.Errorf("one chronology is allowed per Age")
		}
		if c.Start != "" {
			if _, e := tick(c.Start); e != nil {
				return e
			}
		}
		if c.End != "" {
			if _, e := tick(c.End); e != nil {
				return e
			}
			if c.Start != "" && cmp(c.Start, c.End) >= 0 {
				return fmt.Errorf("Age end must follow its start")
			}
		}
		if c.Branch != "canon" && c.Branch != "alternate" && c.Branch != "experiment" && c.Branch != "abandoned" {
			return fmt.Errorf("invalid branch label")
		}
		if c.Calendar != "" && records[c.Calendar].Kind != "calendar" {
			return fmt.Errorf("calendar not found")
		}
	}
	if r.Kind == "event" {
		e := r.Event
		if e == nil || !validID(e.Age) {
			return fmt.Errorf("invalid event")
		}
		if err := validateDate(e.Date); err != nil {
			return err
		}
		if len(e.Changes) > 0 && e.Date.Precision != "exact" {
			return fmt.Errorf("changes require an exact date; uncertain events remain narrative")
		}
		if e.Until != "" {
			if _, err := tick(e.Until); err != nil {
				return err
			}
			if e.Date.Precision != "exact" || cmp(e.Date.Tick, e.Until) >= 0 {
				return fmt.Errorf("validity end must follow exact event date")
			}
		}
		seen := map[string]bool{}
		for _, ch := range e.Changes {
			if !validID(ch.Target) || seen[ch.Target] || !validHash(ch.After) || (ch.Before != "" && !validHash(ch.Before)) {
				return fmt.Errorf("invalid event change")
			}
			seen[ch.Target] = true
		}
		visiting := map[string]bool{}
		visited := map[string]bool{}
		var walk func(string) error
		walk = func(id string) error {
			if visiting[id] {
				return fmt.Errorf("event causes cannot contain cycles")
			}
			if visited[id] {
				return nil
			}
			v := records[id]
			if v.Kind != "event" || v.Event == nil {
				return fmt.Errorf("cause event not found")
			}
			visiting[id] = true
			for _, cause := range v.Event.Causes {
				if err := walk(cause); err != nil {
					return err
				}
			}
			delete(visiting, id)
			visited[id] = true
			return nil
		}
		if err := walk(r.ID); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) Historical(snapshot, at string) (HistoricalState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.historical(snapshot, at)
}
func (s *Store) historical(snapshot, at string) (HistoricalState, error) {
	if _, e := tick(at); e != nil {
		return HistoricalState{}, e
	}
	records, e := s.records(snapshot)
	if e != nil {
		return HistoricalState{}, e
	}
	out := HistoricalState{Records: map[string]Record{}, Unresolved: []string{}, Tick: at, Label: "Tick " + at}
	c := chronology(records)
	if c == nil {
		out.Unresolved = append(out.Unresolved, "No opening baseline has been asserted for this Age.")
		return out, nil
	}
	if c.Calendar != "" {
		if cal := records[c.Calendar].Calendar; cal != nil {
			out.Label = calendarLabel(cal, at)
		}
	}
	if c.Start != "" && cmp(at, c.Start) < 0 || c.End != "" && cmp(at, c.End) >= 0 {
		out.Unresolved = append(out.Unresolved, "This date falls outside the configured Age interval.")
		return out, nil
	}
	if c.Start != "" {
		base, err := s.records(c.Baseline)
		if err != nil {
			return out, err
		}
		for id, r := range base {
			if r.Kind == "entity" || r.Kind == "relation" || r.Kind == "map" || r.Kind == "note" {
				out.Records[id] = r
			}
		}
	} else {
		out.Unresolved = append(out.Unresolved, "Opening baseline has no start date; undated records are unresolved.")
	}
	events := []Record{}
	for _, r := range records {
		if r.Kind == "event" && r.Event.Age == c.Age {
			events = append(events, r)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })
	applied := map[string]bool{}
	for _, r := range events {
		ev := r.Event
		if ev.Date.Precision != "exact" {
			out.Unresolved = append(out.Unresolved, r.Name+": "+ev.Date.Precision+" date; no state inferred.")
			continue
		}
		if cmp(at, ev.Date.Tick) < 0 || ev.Until != "" && cmp(at, ev.Until) >= 0 {
			continue
		}
		for _, ch := range ev.Changes {
			if applied[ch.Target] {
				delete(out.Records, ch.Target)
				out.Unresolved = append(out.Unresolved, "Conflicting validity intervals for "+ch.Target)
				continue
			}
			value, err := s.validationRecord(ch.After)
			if err != nil {
				return out, err
			}
			out.Records[ch.Target] = value
			applied[ch.Target] = true
		}
	}
	for _, r := range records {
		if r.Kind == "map" || r.Kind == "entity" || r.Kind == "relation" {
			if _, ok := out.Records[r.ID]; !ok {
				out.Unresolved = append(out.Unresolved, r.Name+": no dated state asserted.")
			}
		}
	}
	return out, nil
}
func intervalsOverlap(a, b *Event) bool {
	return (a.Until == "" || cmp(b.Date.Tick, a.Until) < 0) && (b.Until == "" || cmp(a.Date.Tick, b.Until) < 0)
}
func validateIntervals(records map[string]Record) error {
	byTarget := map[string][]*Event{}
	for _, r := range records {
		if r.Event == nil {
			continue
		}
		for _, ch := range r.Event.Changes {
			key := r.Event.Age + ":" + ch.Target
			for _, prior := range byTarget[key] {
				if intervalsOverlap(prior, r.Event) {
					return fmt.Errorf("overlapping validity intervals for %s; end the earlier interval first", ch.Target)
				}
			}
			byTarget[key] = append(byTarget[key], r.Event)
		}
	}
	return nil
}
func safeNarrative(r Record) bool {
	return r.Kind == "entity" || r.Kind == "relation" || r.Kind == "map" || r.Kind == "note"
}
func cleanTrack(s string) string { return strings.TrimSpace(s) }
