// Package history owns fictional worlds, historical graphs, and logical map identities.
package history

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/languages"
	"kriemhild/internal/world"
	"math"
	"slices"
	"sort"
	"strings"
	"time"
)

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

type World struct {
	Networks    []NetworkView `json:"networks,omitempty"`
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	CurrentAge  string        `json:"currentAge"`
	Revision    int           `json:"revision"`
	UpdatedAt   int64         `json:"updatedAt"`
}
type Age struct {
	StartedFromYear *YearOrigin `json:"startedFromYear,omitempty"`
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	TimelineID      string      `json:"timelineId"`
	InheritedFrom   string      `json:"inheritedFrom,omitempty"`
	MergePolicy     string      `json:"mergePolicy,omitempty"`
	Notes           string      `json:"notes,omitempty"`
}
type Timeline struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Ages  []string `json:"ages"`
	Order int      `json:"order"`
}
type Edge struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Kind        string `json:"kind"`
}
type Terrain struct {
	ID      string `json:"id"`
	AgeID   string `json:"ageId"`
	Name    string `json:"name"`
	Setting string `json:"setting"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}
type Map struct {
	ID        string       `json:"id"`
	AgeID     string       `json:"ageId"`
	TerrainID string       `json:"terrainId"`
	ProjectID string       `json:"projectId"`
	Snapshot  string       `json:"snapshot"`
	Name      string       `json:"name"`
	ParentID  string       `json:"parentId,omitempty"`
	EntityID  string       `json:"entityId,omitempty"`
	Bounds    world.Bounds `json:"bounds"`
	Kind      string       `json:"kind"`
}
type Entity struct {
	Language     *languages.State    `json:"language,omitempty"`
	Script       *languages.State    `json:"script,omitempty"`
	Archive      *ArchiveConfig      `json:"archive,omitempty"`
	System       *SystemRecord       `json:"system,omitempty"`
	Biology      *biosphere.Lifeform `json:"biology,omitempty"`
	Content      *ConceptContent     `json:"content,omitempty"`
	CreatedAt    int64               `json:"createdAt,omitempty"`
	UpdatedAt    int64               `json:"updatedAt,omitempty"`
	ContinuityID string              `json:"continuityId,omitempty"`
	Names        []EntityName        `json:"names,omitempty"`
	Deleted      bool                `json:"deleted,omitempty"`
	ID           string              `json:"id"`
	AgeID        string              `json:"ageId"`
	TerrainID    string              `json:"terrainId"`
	HomeMap      string              `json:"homeMap"`
	Name         string              `json:"name"`
	Kind         string              `json:"kind"`
	Natural      bool                `json:"natural"`
	SourceID     string              `json:"sourceId,omitempty"`
	Properties   json.RawMessage     `json:"properties"`
}
type Representation struct {
	ID       string       `json:"id"`
	AgeID    string       `json:"ageId"`
	MapID    string       `json:"mapId"`
	EntityID string       `json:"entityId"`
	Shape    world.Entity `json:"shape"`
}
type Relation struct {
	Stance            string `json:"stance,omitempty"`
	Authority         string `json:"authority,omitempty"`
	Prevalence        string `json:"prevalence,omitempty"`
	Interpretation    string `json:"interpretation,omitempty"`
	Provenance        string `json:"provenance,omitempty"`
	Status            string `json:"status,omitempty"`
	Role              string `json:"role,omitempty"`
	Rank              string `json:"rank,omitempty"`
	UnitID            string `json:"unitId,omitempty"`
	AppointmentSource string `json:"appointmentSource,omitempty"`
	SourcePort        string `json:"sourcePort,omitempty"`
	DestinationPort   string `json:"destinationPort,omitempty"`
	Description       string `json:"description,omitempty"`
	Notes             string `json:"notes,omitempty"`
	ContextID         string `json:"contextId,omitempty"`
	ValidFrom         string `json:"validFrom,omitempty"`
	ValidTo           string `json:"validTo,omitempty"`
	ID                string `json:"id"`
	AgeID             string `json:"ageId"`
	Source            string `json:"source"`
	Destination       string `json:"destination"`
	Kind              string `json:"kind"`
	Automatic         bool   `json:"automatic"`
}
type Document struct {
	LibraryFiles    []LibraryFile     `json:"libraryFiles,omitempty"`
	Biospheres      []biosphere.State `json:"biospheres,omitempty"`
	Years           []Year            `json:"years,omitempty"`
	World           World             `json:"world"`
	Ages            []Age             `json:"ages"`
	Timelines       []Timeline        `json:"timelines"`
	Edges           []Edge            `json:"edges"`
	Terrains        []Terrain         `json:"terrains"`
	Maps            []Map             `json:"maps"`
	Entities        []Entity          `json:"entities"`
	Representations []Representation  `json:"representations"`
	Relations       []Relation        `json:"relations"`
}
type Command struct {
	Biosphere         *biosphere.State  `json:"biosphere,omitempty"`
	BiosphereEdit     *BiosphereEdit    `json:"biosphereEdit,omitempty"`
	EntityID          string            `json:"entityId,omitempty"`
	ComponentID       string            `json:"componentId,omitempty"`
	Commands          []Command         `json:"commands,omitempty"`
	SourceYearID      string            `json:"sourceYearId,omitempty"`
	Relations         []Relation        `json:"relations,omitempty"`
	RemoveRelationIDs []string          `json:"removeRelationIds,omitempty"`
	EntityNames       map[string]string `json:"entityNames,omitempty"`
	Entity            *Entity           `json:"entity,omitempty"`
	Relation          *Relation         `json:"relation,omitempty"`
	EntityIDs         []string          `json:"entityIds,omitempty"`
	ExcludeIDs        []string          `json:"excludeIds,omitempty"`
	Network           *NetworkView      `json:"network,omitempty"`
	Kind              string            `json:"kind"`
	AgeID             string            `json:"ageId"`
	OtherAge          string            `json:"otherAge"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	Mode              string            `json:"mode"`
	Policy            string            `json:"policy"`
	EdgeID            string            `json:"edgeId"`
	MapID             string            `json:"mapId"`
	Revision          int               `json:"revision"`
}

func New(name string) *Document {
	a, t := ID(), ID()
	return &Document{World: World{ID: ID(), Name: strings.TrimSpace(name), CurrentAge: a, UpdatedAt: time.Now().UnixMilli()}, Ages: []Age{{ID: a, Name: "First Age", TimelineID: t}}, Timelines: []Timeline{{ID: t, Name: "Original timeline", Ages: []string{a}}}, Edges: []Edge{}, Terrains: []Terrain{}, Maps: []Map{}, Entities: []Entity{}, Representations: []Representation{}, Relations: []Relation{}}
}
func (d *Document) Age(id string) *Age {
	for i := range d.Ages {
		if d.Ages[i].ID == id {
			return &d.Ages[i]
		}
	}
	return nil
}
func (d *Document) Map(age, id string) *Map {
	for i := range d.Maps {
		if d.Maps[i].AgeID == age && d.Maps[i].ID == id {
			return &d.Maps[i]
		}
	}
	return nil
}
func (d *Document) Project(id string) *Map {
	for i := range d.Maps {
		if d.Maps[i].ProjectID == id {
			return &d.Maps[i]
		}
	}
	return nil
}
func (d *Document) AddEdge(from, to, kind string) error {
	if d.Age(from) == nil || d.Age(to) == nil {
		return fmt.Errorf("choose Ages in this World")
	}
	for _, e := range d.Edges {
		if e.Source == from && e.Destination == to {
			return fmt.Errorf("these Ages are already connected")
		}
	}
	d.Edges = append(d.Edges, Edge{ID(), from, to, kind})
	if err := d.ValidateGraph(); err != nil {
		d.Edges = d.Edges[:len(d.Edges)-1]
		return err
	}
	return nil
}
func (d *Document) ValidateGraph() error {
	ages := map[string]bool{}
	for _, a := range d.Ages {
		if ages[a.ID] || a.ID == "" || strings.TrimSpace(a.Name) == "" || len(a.Name) > 160 {
			return fmt.Errorf("invalid Age")
		}
		ages[a.ID] = true
	}
	if !ages[d.World.CurrentAge] {
		return fmt.Errorf("current Age does not exist")
	}
	count := map[string]int{}
	timelines := map[string]bool{}
	for _, t := range d.Timelines {
		if t.ID == "" || timelines[t.ID] || strings.TrimSpace(t.Name) == "" || len(t.Ages) == 0 {
			return fmt.Errorf("invalid timeline identity")
		}
		timelines[t.ID] = true
		for _, id := range t.Ages {
			if !ages[id] || d.Age(id).TimelineID != t.ID {
				return fmt.Errorf("invalid timeline membership")
			}
			count[id]++
		}
	}
	for id := range ages {
		if count[id] != 1 {
			return fmt.Errorf("each Age needs exactly one timeline")
		}
	}
	unique := map[string]bool{}
	edgeIDs := map[string]bool{}
	for _, e := range d.Edges {
		key := e.Source + "/" + e.Destination
		if e.ID == "" || edgeIDs[e.ID] || !ages[e.Source] || !ages[e.Destination] || unique[key] || (e.Kind != "linear" && e.Kind != "branch" && e.Kind != "merge") {
			return fmt.Errorf("invalid timeline relationship")
		}
		edgeIDs[e.ID] = true
		unique[key] = true
	}
	return nil
}
func (d *Document) inherit(from, to string, overwrite bool) {
	// File inheritance follows entity cloning so dated archive coverage can be
	// reconciled in the destination Age without mutating the source records.
	defer d.inheritLibrary(from, to, overwrite)
	for _, v := range append([]NetworkView(nil), d.World.Networks...) {
		if v.AgeID == from {
			v.AgeID = to
			v.RelationTypes = append([]string(nil), v.RelationTypes...)
			v.MemberIDs = append([]string(nil), v.MemberIDs...)
			v.LegacyData = append(json.RawMessage(nil), v.LegacyData...)
			found := -1
			for i, x := range d.World.Networks {
				if x.AgeID == to && x.ID == v.ID {
					found = i
					break
				}
			}
			if found < 0 {
				d.World.Networks = append(d.World.Networks, v)
			} else if overwrite {
				d.World.Networks[found] = v
			}
		}
	}
	for _, v := range append([]biosphere.State(nil), d.Biospheres...) {
		if v.AgeID != from {
			continue
		}
		v = biosphere.Clone(v)
		v.AgeID = to
		found := false
		for i, old := range d.Biospheres {
			if old.AgeID == to && old.ID == v.ID {
				if overwrite {
					d.Biospheres[i] = v
				}
				found = true
			}
		}
		if !found {
			d.Biospheres = append(d.Biospheres, v)
		}
	}
	for _, v := range append([]Terrain(nil), d.Terrains...) {
		if v.AgeID != from {
			continue
		}
		v.AgeID = to
		found := -1
		for i, x := range d.Terrains {
			if x.AgeID == to && x.ID == v.ID {
				found = i
				break
			}
		}
		if found < 0 {
			d.Terrains = append(d.Terrains, v)
		} else if overwrite {
			d.Terrains[found] = v
		}
	}
	for _, v := range append([]Map(nil), d.Maps...) {
		if v.AgeID != from {
			continue
		}
		v.AgeID = to
		v.ProjectID = ID()
		if old := d.Map(to, v.ID); old == nil {
			d.Maps = append(d.Maps, v)
		} else if overwrite {
			*old = v
		}
	}
	for _, v := range append([]Entity(nil), d.Entities...) {
		v.Language = languages.Clone(v.Language)
		v.Script = languages.Clone(v.Script)
		v.Properties = append(json.RawMessage(nil), v.Properties...)
		v.Content = cloneContent(v.Content)
		v.System = cloneSystem(v.System)
		v.Archive = cloneArchive(v.Archive)
		v.Biology = biosphere.Clone(v.Biology)
		v.Names = append([]EntityName(nil), v.Names...)
		if v.ContinuityID == "" {
			v.ContinuityID = v.ID
		}
		if v.AgeID != from {
			continue
		}
		v.AgeID = to
		found := -1
		for i, x := range d.Entities {
			if x.AgeID == to && x.ID == v.ID {
				found = i
				break
			}
		}
		if found < 0 {
			d.Entities = append(d.Entities, v)
		} else if overwrite {
			d.Entities[found] = v
		}
	}
	for _, v := range append([]Representation(nil), d.Representations...) {
		shapeData, _ := json.Marshal(v.Shape)
		var clonedShape world.Entity
		_ = json.Unmarshal(shapeData, &clonedShape)
		v.Shape = clonedShape
		v.Shape.Points = append([][2]float64(nil), v.Shape.Points...)
		v.Shape.Cells = append([]int(nil), v.Shape.Cells...)
		if v.AgeID != from {
			continue
		}
		v.AgeID = to
		found := -1
		for i, x := range d.Representations {
			if x.AgeID == to && x.ID == v.ID {
				found = i
				break
			}
		}
		if found < 0 {
			d.Representations = append(d.Representations, v)
		} else if overwrite {
			d.Representations[found] = v
		}
	}
	for _, v := range append([]Relation(nil), d.Relations...) {
		if v.AgeID == from {
			v.AgeID = to
			exists := false
			for i, x := range d.Relations {
				if x.AgeID == to && x.ID == v.ID {
					if overwrite {
						d.Relations[i] = v
					}
					exists = true
				}
			}
			if !exists {
				d.Relations = append(d.Relations, v)
			}
		}
	}
}
func (d *Document) Apply(c Command) (err error) {
	if c.Kind == "biosphere" {
		return d.applyBiosphere(c)
	}
	if c.Kind == "promote-component" {
		return d.promoteComponent(c)
	}
	if c.Kind == "batch" {
		return d.applyBatch(c)
	}
	defer func() {
		if err == nil {
			d.SyncYears()
		}
	}()
	a := d.Age(c.AgeID)
	if handled, err := d.applyEntities(c); handled {
		return err
	}
	switch c.Kind {
	case "delete-age":
		if a == nil {
			return fmt.Errorf("Age not found")
		}
		if len(d.Ages) == 1 {
			return fmt.Errorf("keep at least one Age in a World")
		}
		d.Biospheres = slices.DeleteFunc(d.Biospheres, func(b biosphere.State) bool { return b.AgeID == c.AgeID })
		d.LibraryFiles = slices.DeleteFunc(d.LibraryFiles, func(f LibraryFile) bool { return f.AgeID == c.AgeID })
		d.World.Networks = slices.DeleteFunc(d.World.Networks, func(v NetworkView) bool { return v.AgeID == c.AgeID })
		previous, next := "", ""
		for _, t := range d.Timelines {
			if t.ID != a.TimelineID {
				continue
			}
			for i, id := range t.Ages {
				if id != a.ID {
					continue
				}
				if i > 0 {
					previous = t.Ages[i-1]
				}
				if i+1 < len(t.Ages) {
					next = t.Ages[i+1]
				}
			}
		}
		joinBefore, joinAfter := false, false
		for _, e := range d.Edges {
			joinBefore = joinBefore || e.Kind == "linear" && e.Source == previous && e.Destination == a.ID
			joinAfter = joinAfter || e.Kind == "linear" && e.Source == a.ID && e.Destination == next
		}
		id := a.ID
		d.Edges = slices.DeleteFunc(d.Edges, func(e Edge) bool { return e.Source == id || e.Destination == id })
		d.Ages = slices.DeleteFunc(d.Ages, func(v Age) bool { return v.ID == id })
		for i := range d.Ages {
			if d.Ages[i].InheritedFrom == id {
				d.Ages[i].InheritedFrom = ""
			}
		}
		for i := range d.Timelines {
			d.Timelines[i].Ages = slices.DeleteFunc(d.Timelines[i].Ages, func(v string) bool { return v == id })
		}
		d.Timelines = slices.DeleteFunc(d.Timelines, func(v Timeline) bool { return len(v.Ages) == 0 })
		d.Terrains = slices.DeleteFunc(d.Terrains, func(v Terrain) bool { return v.AgeID == id })
		d.Maps = slices.DeleteFunc(d.Maps, func(v Map) bool { return v.AgeID == id })
		d.Entities = slices.DeleteFunc(d.Entities, func(v Entity) bool { return v.AgeID == id })
		for i := range d.Entities {
			if c := d.Entities[i].Archive; c != nil {
				c.Sources = slices.DeleteFunc(c.Sources, func(s KnowledgeSource) bool { return s.SourceAge == id || s.ArrivalAge == id })
			}
		}
		d.Representations = slices.DeleteFunc(d.Representations, func(v Representation) bool { return v.AgeID == id })
		d.Relations = slices.DeleteFunc(d.Relations, func(v Relation) bool { return v.AgeID == id })
		if joinBefore && joinAfter {
			exists := false
			for _, e := range d.Edges {
				exists = exists || e.Source == previous && e.Destination == next
			}
			if !exists {
				d.Edges = append(d.Edges, Edge{ID: ID(), Source: previous, Destination: next, Kind: "linear"})
			}
		}
		if d.World.CurrentAge == id {
			d.World.CurrentAge = previous
			if previous == "" {
				d.World.CurrentAge = next
			}
			if d.World.CurrentAge == "" {
				d.World.CurrentAge = d.Timelines[0].Ages[0]
			}
		}
		return nil
	case "delete-map":
		m := d.Map(c.AgeID, c.MapID)
		if m == nil {
			return fmt.Errorf("map not found")
		}
		remove := map[string]bool{m.ID: true}
		for changed := true; changed; {
			changed = false
			for _, x := range d.Maps {
				if x.AgeID == c.AgeID && remove[x.ParentID] && !remove[x.ID] {
					remove[x.ID] = true
					changed = true
				}
			}
		}
		maps := d.Maps[:0]
		for _, x := range d.Maps {
			if x.AgeID != c.AgeID || !remove[x.ID] {
				maps = append(maps, x)
			}
		}
		d.Maps = maps
		// A map is a representation, not the lifetime owner of its logical entities.
		for i := range d.Entities {
			e := &d.Entities[i]
			if e.AgeID != c.AgeID || !remove[e.HomeMap] {
				continue
			}
			e.HomeMap = ""
			e.TerrainID = ""
			for _, r := range d.Representations {
				if r.AgeID == c.AgeID && r.EntityID == e.ID && !remove[r.MapID] {
					if other := d.Map(c.AgeID, r.MapID); other != nil {
						e.HomeMap = other.ID
						e.TerrainID = other.TerrainID
						break
					}
				}
			}
		}
		reps := d.Representations[:0]
		for _, r := range d.Representations {
			if r.AgeID != c.AgeID || !remove[r.MapID] {
				reps = append(reps, r)
			}
		}
		d.Representations = reps
		terrains := d.Terrains[:0]
		for _, t := range d.Terrains {
			keep := t.AgeID != c.AgeID
			for _, m := range d.Maps {
				if m.AgeID == t.AgeID && m.TerrainID == t.ID {
					keep = true
				}
			}
			if keep {
				terrains = append(terrains, t)
			}
		}
		d.Terrains = terrains
		d.pruneSystems(c.AgeID)
		d.Biospheres = slices.DeleteFunc(d.Biospheres, func(b biosphere.State) bool {
			for _, t := range d.Terrains {
				if t.ID == b.TerrainID && t.AgeID == b.AgeID {
					return false
				}
			}
			return true
		})
		d.Resolve(c.AgeID)
	case "select":
		if a == nil {
			return fmt.Errorf("Age not found")
		}
		d.World.CurrentAge = a.ID
	case "rename-world":
		if strings.TrimSpace(c.Name) == "" || len(c.Name) > 160 || len(c.Description) > 8192 {
			return fmt.Errorf("invalid World metadata")
		}
		d.World.Name = strings.TrimSpace(c.Name)
		d.World.Description = c.Description
	case "rename-age":
		if a == nil || strings.TrimSpace(c.Name) == "" || len(c.Name) > 160 {
			return fmt.Errorf("invalid Age name")
		}
		a.Name = strings.TrimSpace(c.Name)
	case "disconnect":
		found := false
		edges := d.Edges[:0]
		for _, e := range d.Edges {
			if e.ID == c.EdgeID {
				if e.Kind == "linear" {
					return fmt.Errorf("linear chronology cannot be disconnected; manage branch and merge links instead")
				}
				found = true
			} else {
				edges = append(edges, e)
			}
		}
		if !found {
			return fmt.Errorf("relationship not found")
		}
		d.Edges = edges
	case "connect":
		return d.AddEdge(c.AgeID, c.OtherAge, "merge")
	case "age":
		if a == nil || strings.TrimSpace(c.Name) == "" || len(c.Name) > 160 {
			return fmt.Errorf("choose a source Age and a name")
		}
		source := *a
		var origin *YearOrigin
		if c.SourceYearID != "" {
			y := d.Year(source.ID, c.SourceYearID)
			if y == nil || (c.Mode != "after" && c.Mode != "branch" && c.Mode != "parallel") {
				return fmt.Errorf("choose a year in the source Age and a continuation or branch")
			}
			origin = &YearOrigin{AgeID: source.ID, YearID: y.ID, Number: y.Number}
		}
		if c.Mode == "merge" && (d.Age(c.OtherAge) == nil || c.OtherAge == source.ID || (c.Policy != "primary" && c.Policy != "secondary")) {
			return fmt.Errorf("merge requires two Ages and an explicit conflict policy")
		}
		if c.Mode != "after" && c.Mode != "before" && c.Mode != "branch" && c.Mode != "parallel" && c.Mode != "merge" {
			return fmt.Errorf("unknown Age operation")
		}
		age := Age{ID: ID(), Name: strings.TrimSpace(c.Name), TimelineID: source.TimelineID, InheritedFrom: source.ID, MergePolicy: c.Policy}
		age.StartedFromYear = origin
		if c.Mode == "branch" || c.Mode == "parallel" || c.Mode == "merge" {
			age.TimelineID = ID()
			d.Timelines = append(d.Timelines, Timeline{ID: age.TimelineID, Name: age.Name + " timeline", Ages: []string{age.ID}, Order: len(d.Timelines)})
		} else {
			for i := range d.Timelines {
				t := &d.Timelines[i]
				if t.ID != source.TimelineID {
					continue
				}
				for j, id := range t.Ages {
					if id != source.ID {
						continue
					}
					at := j
					if c.Mode == "after" {
						at++
					}
					t.Ages = append(t.Ages, "")
					copy(t.Ages[at+1:], t.Ages[at:])
					t.Ages[at] = age.ID
					break
				}
			}
		}
		d.Ages = append(d.Ages, age)
		d.inherit(source.ID, age.ID, false)
		if c.Mode == "merge" {
			d.inherit(c.OtherAge, age.ID, c.Policy == "secondary")
			if err := d.AddEdge(c.OtherAge, age.ID, "merge"); err != nil {
				return err
			}
		}
		if c.Mode == "after" || c.Mode == "before" {
			edges := append([]Edge(nil), d.Edges...)
			d.Edges = nil
			for _, e := range edges {
				if e.Kind == "linear" && ((c.Mode == "after" && e.Source == source.ID) || (c.Mode == "before" && e.Destination == source.ID)) {
					if c.Mode == "after" {
						e.Source = age.ID
					} else {
						e.Destination = age.ID
					}
				}
				d.Edges = append(d.Edges, e)
			}
			from, to := source.ID, age.ID
			if c.Mode == "before" {
				from, to = to, from
			}
			if err := d.AddEdge(from, to, "linear"); err != nil {
				return err
			}
		} else if c.Mode != "parallel" {
			if err := d.AddEdge(source.ID, age.ID, c.Mode); err != nil {
				return err
			}
		}
		for id, name := range c.EntityNames {
			if slices.Contains(c.ExcludeIDs, id) {
				continue
			}
			e := d.Entity(age.ID, id)
			if e == nil {
				return fmt.Errorf("inherited entity not found")
			}
			name = strings.TrimSpace(name)
			if name == "" || len(name) > 200 {
				return fmt.Errorf("invalid inherited name")
			}
			e.Name = name
			for i := range d.Representations {
				r := &d.Representations[i]
				if r.AgeID == age.ID && r.EntityID == id {
					r.Shape.Name = name
				}
			}
		}
		d.World.CurrentAge = age.ID
		for _, id := range c.ExcludeIDs {
			if e := d.Entity(age.ID, id); e != nil {
				e.Deleted = true
			}
		}
		d.pruneBiosphere(age.ID)
		d.pruneSystems(age.ID)
		d.PruneDeleted(age.ID)
		d.Resolve(age.ID)
	default:
		return fmt.Errorf("unknown historical action")
	}
	return d.ValidateGraph()
}
func (d *Document) Validate() error {
	if err := d.validateLibrary(); err != nil {
		return err
	}
	if err := d.validateBiospheres(); err != nil {
		return err
	}
	if err := d.validateYears(); err != nil {
		return err
	}
	if d.World.ID == "" || strings.TrimSpace(d.World.Name) == "" || len(d.World.Name) > 160 {
		return fmt.Errorf("invalid World")
	}
	if err := d.ValidateGraph(); err != nil {
		return err
	}
	seen := map[string]bool{}
	terrains := map[string]Terrain{}
	for _, t := range d.Terrains {
		k := t.AgeID + "/" + t.ID
		if d.Age(t.AgeID) == nil || t.ID == "" || terrains[k].ID != "" || t.Width < 16 || t.Height < 16 || t.Width > 256 || t.Height > 256 {
			return fmt.Errorf("invalid terrain")
		}
		terrains[k] = t
	}
	for _, m := range d.Maps {
		k := m.AgeID + "/" + m.ID
		t, ok := terrains[m.AgeID+"/"+m.TerrainID]
		if !ok || seen[k] || seen["project/"+m.ProjectID] || m.ID == "" || m.ProjectID == "" || m.Bounds.Width <= 0 || m.Bounds.Height <= 0 || !finite(m.Bounds.X+m.Bounds.Y+m.Bounds.Width+m.Bounds.Height) || m.Bounds.X < 0 || m.Bounds.Y < 0 || m.Bounds.X+m.Bounds.Width > float64(t.Width-1)+1e-6 || m.Bounds.Y+m.Bounds.Height > float64(t.Height-1)+1e-6 {
			return fmt.Errorf("invalid map extent or identity")
		}
		seen[k] = true
		seen["project/"+m.ProjectID] = true
		visited := map[string]bool{m.ID: true}
		for p := m.ParentID; p != ""; {
			parent := d.Map(m.AgeID, p)
			if parent == nil || parent.TerrainID != m.TerrainID || visited[p] {
				return fmt.Errorf("invalid map ancestry")
			}
			visited[p] = true
			p = parent.ParentID
		}
	}
	entities := map[string]bool{}
	continuities := map[string]bool{}
	deleted := map[string]bool{}

	for _, e := range d.Entities {
		k := e.AgeID + "/" + e.ID
		home := d.Map(e.AgeID, e.HomeMap)
		if entities[k] || e.ID == "" || d.Age(e.AgeID) == nil || (e.HomeMap != "" && (home == nil || home.TerrainID != e.TerrainID)) || (e.TerrainID != "" && terrains[e.AgeID+"/"+e.TerrainID].ID == "") || !json.Valid(e.Properties) {
			return fmt.Errorf("invalid logical entity")
		}
		if err := d.validateEntity(e); err != nil {
			return err
		}
		continuity := e.ContinuityID
		if continuity == "" {
			continuity = e.ID
		}
		ck := e.AgeID + "/" + continuity
		if continuities[ck] {
			return fmt.Errorf("duplicate entity continuity in Age")
		}
		continuities[ck] = true
		entities[k] = true
		deleted[k] = e.Deleted

	}
	reps := map[string]bool{}
	for _, r := range d.Representations {
		m := d.Map(r.AgeID, r.MapID)
		k := r.AgeID + "/" + r.ID
		if m == nil || r.ID == "" || reps[k] || !entities[r.AgeID+"/"+r.EntityID] || deleted[r.AgeID+"/"+r.EntityID] || r.Shape.Deleted || (r.Shape.ID != r.EntityID && r.Shape.LogicalID != r.EntityID) {
			return fmt.Errorf("invalid map representation")
		}
		t := terrains[m.AgeID+"/"+m.TerrainID]
		check := world.New(t.Width, t.Height)
		shape := r.Shape
		shape.ParentID = ""
		if err := check.ValidateEntity(shape); err != nil {
			return err
		}
		reps[k] = true
	}
	relations := map[string]bool{}
	for _, r := range d.Relations {
		key := r.AgeID + "/" + r.ID
		if r.ID == "" || relations[key] || !entities[r.AgeID+"/"+r.Source] || !entities[r.AgeID+"/"+r.Destination] || deleted[r.AgeID+"/"+r.Source] || deleted[r.AgeID+"/"+r.Destination] || r.Source == r.Destination {
			return fmt.Errorf("invalid logical relationship")
		}
		if err := d.validateRelation(r); err != nil {
			return err
		}
		relations[key] = true
	}
	networks := map[string]bool{}
	for _, v := range d.World.Networks {
		key := v.AgeID + "/" + v.ID
		if v.ID == "" || d.Age(v.AgeID) == nil || networks[key] || strings.TrimSpace(v.Name) == "" {
			return fmt.Errorf("invalid saved network")
		}
		networks[key] = true
		for _, id := range append([]string{v.CenterID, v.FamilyID}, v.MemberIDs...) {
			if id != "" && !entities[v.AgeID+"/"+id] {
				return fmt.Errorf("saved network subject not found")
			}
		}
	}
	return nil
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
func (d *Document) Sort() {
	sort.Slice(d.Ages, func(i, j int) bool { return d.Ages[i].ID < d.Ages[j].ID })
}
