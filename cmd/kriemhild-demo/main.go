// Creates a fresh, disposable reference world through the public command API.
// It never selects, rewrites or migrates an existing world.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"kriemhild/internal/project"
	"log"
	"os"
	"path/filepath"
)

func main() {
	library := flag.String("data", "worlds-dev", "library in which to create a NEW reference world")
	flag.Parse()
	if e := os.MkdirAll(*library, 0700); e != nil {
		log.Fatal(e)
	}
	store, age, e := project.Create(filepath.Join(*library, project.NewID()), "Reference world — The Harbor", "Age of Foundations")
	if e != nil {
		log.Fatal(e)
	}
	defer store.Close()
	state, e := store.State(age)
	if e != nil {
		log.Fatal(e)
	}
	apply := func(c project.Command) {
		c.Expected = state.Revision
		c.Age = state.Age.ID
		next, e := store.Apply(c)
		if e != nil {
			log.Fatal(e)
		}
		state = next
	}
	records := []project.Record{}
	domain := map[string]project.Record{}
	for _, d := range project.DomainCatalog() {
		r := project.Record{ID: project.NewID(), Kind: "entity", Type: d.Name, Name: d.Name + " example", Notes: "An editable reference entry. Replace these examples with your own world.", Properties: map[string]any{"_domain": d.ID}}
		domain[d.ID] = r
	}
	city := domain["settlement"]
	city.Name = "Rivergate"
	city.Properties["population"] = map[string]any{"mode": "range", "min": "1000", "max": "1500", "unit": "people"}
	domain["settlement"] = city
	person := domain["person"]
	person.Name = "Ada of Rivergate"
	person.Properties["residence"] = city.ID
	domain["person"] = person
	lang := domain["language"]
	lang.Name = "Harbor speech"
	lang.Properties["phonemes"] = "p t k a e i"
	domain["language"] = lang
	word := domain["lexeme"]
	word.Name = "pata — gate"
	word.Properties["language"] = lang.ID
	word.Properties["phonemes"] = "p a t a"
	word.Properties["form"] = "pata"
	word.Properties["senses"] = "gate"
	domain["lexeme"] = word
	for _, d := range project.DomainCatalog() {
		r := domain[d.ID]
		records = append(records, r)
		if r.ID != city.ID {
			records = append(records, project.Record{ID: project.NewID(), Kind: "relation", Name: "connected with", From: city.ID, To: r.ID})
		}
	}
	child := project.Record{ID: project.NewID(), Kind: "entity", Type: "Person", Name: "Ren of Rivergate", Properties: map[string]any{"_domain": "person"}}
	records = append(records, child, project.Record{ID: project.NewID(), Kind: "relation", Name: "adoptive parent of", From: person.ID, To: child.ID})
	coast := project.Record{ID: project.NewID(), Kind: "map", Name: "Harbor coast", Width: 1200, Height: 800, Pins: []project.Pin{{Entity: city.ID, X: .5, Y: .4}}, Features: []project.Feature{{ID: project.NewID(), Name: "Coastal habitat", Kind: "biome", Entity: domain["biome"].ID, Points: []project.Point{{X: .25, Y: .2}, {X: .7, Y: .2}, {X: .65, Y: .5}, {X: .3, Y: .6}}}}}
	records = append(records, coast)
	local := project.Record{ID: project.NewID(), Kind: "map", Name: "Gatehouse detail", Width: 1000, Height: 700, Properties: map[string]any{"_localMap": project.LocalDrawing{Parent: coast.ID, Anchor: project.Point{X: .5, Y: .4}, Span: 100, Unit: "metres", Grid: "square", Columns: 20, Shapes: []project.LocalShape{{ID: project.NewID(), Name: "Gate hall", Kind: "room", Level: 0, Points: []project.Point{{X: .2, Y: .2}, {X: .7, Y: .2}, {X: .7, Y: .6}, {X: .2, Y: .6}}}}, Tokens: []project.LocalToken{{ID: project.NewID(), Entity: domain["unit"].ID, X: .5, Y: .4, Level: 0, Facing: 90}}}}}
	records = append(records, local)
	apply(project.Command{Action: "put-many", Records: records})
	terrain, e := store.PreviewTerrain(project.TerrainRequest{Expected: state.Revision, Age: state.Age.ID, Map: coast.ID, Operation: "generate", Seed: "harbor-reference"})
	if e != nil {
		log.Fatal(e)
	}
	apply(project.Command{Action: "put", Record: &terrain.Record})
	apply(project.Command{Action: "configure-time", Chronology: &project.Chronology{Start: "0", Branch: "canon"}})
	scene := project.Record{ID: project.NewID(), Kind: "scene", Name: "At the gate", Order: 1, SettingAge: state.Age.ID, SettingSnapshot: state.Age.Snapshot, SettingDate: "10", References: []string{city.ID, person.ID}, Document: json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","attrs":{"blockId":"reference-opening"},"content":[{"type":"text","text":"Ada waited at the gate. Beyond the harbor, the tide was turning."}]}]}`), Properties: map[string]any{"locationID": city.ID, "povID": person.ID, "workID": domain["work"].ID, "goal": "Enter the city", "draftStatus": "draft"}}
	apply(project.Command{Action: "put", Record: &scene})
	first := state.Age.ID
	apply(project.Command{Action: "copy-age", Name: "Age of Rising Water"})
	apply(project.Command{Action: "configure-time", Chronology: &project.Chronology{Start: "100", Branch: "canon"}})
	renamed := state.Records[city.ID]
	renamed.Name = "High Rivergate"
	event := project.Record{ID: project.NewID(), Kind: "event", Name: "The harbor moves uphill", Event: &project.Event{Age: state.Age.ID, Date: project.FictionalDate{Precision: "exact", Tick: "120"}}}
	apply(project.Command{Action: "record-event", Event: &event, Records: []project.Record{renamed}})
	state, e = store.State(first)
	if e != nil {
		log.Fatal(e)
	}
	apply(project.Command{Action: "copy-age", Name: "Alternate Age — Old Harbor"})
	fmt.Printf("Created %s\nWorld: %s\nOpen http://127.0.0.1:4784/?world=%s&age=%s\n", state.Root.World.Name, state.Root.World.ID, state.Root.World.ID, first)
}
