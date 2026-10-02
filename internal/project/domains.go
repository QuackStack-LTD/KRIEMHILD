package project

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

type DomainField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}
type DomainType struct {
	ID     string        `json:"id"`
	Group  string        `json:"group"`
	Name   string        `json:"name"`
	Fields []DomainField `json:"fields"`
}

// These are optional starting templates. Original properties and custom entity
// types remain available; adopting a template never executes a simulation.
func DomainCatalog() []DomainType {
	definitions := [][4]string{
		{"settlement", "Societies", "Settlement", "population:quantity,government:text,industries:text,supply:text,defenses:text,parent:entity"},
		{"population", "Societies", "Population cohort", "size:quantity,residence:entity,culture:entity,language:entity,religion:entity,migration:text"},
		{"culture", "Societies", "Culture", "values:text,traditions:text,kinship:text,food:text,architecture:text,education:text"},
		{"polity", "Politics", "Polity", "form:text,capital:entity,ruler:entity,jurisdiction:text,claims:text,diplomacy:text"},
		{"institution", "Politics", "Institution", "mandate:text,authority:text,membership:text,resources:quantity,offices:text"},
		{"law", "Politics", "Law", "jurisdiction:entity,institution:entity,terms:text,exceptions:text,effective:text,repeal:text"},
		{"class", "Politics", "Social class", "rights:text,obligations:text,mobility:text,taxes:quantity,terminology:text"},
		{"religion", "Beliefs", "Religion or philosophy", "doctrine:text,rituals:text,taboos:text,clergy:entity,sacredSites:text,sects:text"},
		{"pantheon", "Beliefs", "Pantheon", "tradition:entity,cosmology:text,accounts:text"},
		{"deity", "Beliefs", "Deity", "domains:text,pantheon:entity,interpretations:text,beliefStatus:text"},
		{"lore", "Beliefs", "Lore or sourced assertion", "form:text,author:text,audience:text,perspective:text,truth:text,source:text,rights:text"},
		{"language", "Languages", "Language", "family:entity,phonemes:text,grammar:text,script:text,orthography:text,geography:text"},
		{"lexeme", "Languages", "Dictionary entry", "language:entity,form:text,phonemes:text,senses:text,partOfSpeech:text,etymology:text,borrowedFrom:entity,gloss:text"},
		{"name", "Languages", "Historical name", "subject:entity,language:entity,form:text,meaning:text,usage:text"},
		{"rulebook", "Rules", "Rulebook", "sources:text,access:text,capabilities:text,costs:quantity,limitations:text,prohibitions:text,prerequisites:text,exceptions:text"},
		{"technology", "Rules", "Technology", "capability:text,discovery:text,adoption:text,prerequisite:entity,resources:text,availability:text"},
		{"resource", "Economy", "Resource or good", "quantity:quantity,location:entity,extraction:text,uses:text"},
		{"recipe", "Economy", "Production recipe", "inputs:text,output:entity,capacity:quantity,requirements:text"},
		{"route", "Economy", "Trade or transport route", "origin:entity,destination:entity,mode:text,capacity:quantity,cost:quantity,seasonality:text,access:text,dependencies:text"},
		{"currency", "Economy", "Currency", "issuer:entity,denomination:text,conversion:quantity,conversionDate:text"},
		{"person", "Families", "Person", "biography:text,traits:text,abilities:text,affiliations:text,birth:text,death:text,residence:entity"},
		{"dynasty", "Families", "Dynasty", "founder:entity,membership:text,titles:text,claims:text,succession:text"},
		{"war", "Warfare", "War or campaign", "objectives:text,causes:text,participants:text,outcome:text,treaty:entity"},
		{"unit", "Warfare", "Force or unit", "commander:entity,organization:entity,location:entity,strength:quantity,consumption:quantity,supply:text,movement:text"},
		{"battle", "Warfare", "Battle", "campaign:entity,location:entity,plans:text,outcome:text,losses:quantity"},
		{"climate", "Nature", "Climate", "temperature:quantity,precipitation:quantity,seasonality:text,winds:text,location:entity"},
		{"biome", "Nature", "Biome or habitat", "climate:entity,soil:text,vegetation:text,suitability:text,location:entity"},
		{"species", "Nature", "Species or creature", "habitat:entity,behavior:text,ecologicalRole:text,domestication:text,population:quantity"},
		{"work", "Writing", "Work or series", "premise:text,themes:text,stakes:text,conflict:text,targetWords:quantity"},
		{"arc", "Writing", "Character arc or plot thread", "work:entity,character:entity,goal:text,conflict:text,turns:text,resolution:text"},
		{"chapter", "Writing", "Act, chapter or sequence", "work:entity,parent:entity,purpose:text,order:quantity"},
		{"storyboard", "Writing", "Storyboard panel", "work:entity,shot:text,framing:text,action:text,dialogue:text,caption:text"},
	}
	out := []DomainType{}
	for _, d := range definitions {
		v := DomainType{ID: d[0], Group: d[1], Name: d[2], Fields: []DomainField{}}
		for _, raw := range strings.Split(d[3], ",") {
			p := strings.Split(raw, ":")
			v.Fields = append(v.Fields, DomainField{Key: p[0], Label: p[0], Kind: p[1]})
		}
		out = append(out, v)
	}
	return out
}

type Quantity struct {
	Mode  string `json:"mode"`
	Value string `json:"value,omitempty"`
	Min   string `json:"min,omitempty"`
	Max   string `json:"max,omitempty"`
	Unit  string `json:"unit,omitempty"`
	Text  string `json:"text,omitempty"`
}

var domainDefinitions = func() map[string]DomainType {
	out := map[string]DomainType{}
	for _, definition := range DomainCatalog() {
		out[definition.ID] = definition
	}
	return out
}()

func decimal(s string) (*big.Rat, error) {
	if len(s) > 100 || strings.ContainsAny(s, "/eE") || strings.TrimSpace(s) != s || s == "" {
		return nil, fmt.Errorf("use a decimal number of at most 100 characters")
	}
	text := s
	if text[0] == '-' || text[0] == '+' {
		text = text[1:]
	}
	digits, dots := 0, 0
	for _, r := range text {
		if r == '.' {
			dots++
		} else if r >= '0' && r <= '9' {
			digits++
		} else {
			return nil, fmt.Errorf("use decimal digits with an optional sign and decimal point")
		}
	}
	if digits == 0 || dots > 1 {
		return nil, fmt.Errorf("invalid decimal")
	}
	v, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("invalid decimal")
	}
	return v, nil
}
func validateQuantity(value any) error {
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	var q Quantity
	if e = json.Unmarshal(b, &q); e != nil {
		return fmt.Errorf("quantity must include a mode")
	}
	switch q.Mode {
	case "unknown":
	case "qualitative":
		if strings.TrimSpace(q.Text) == "" {
			return fmt.Errorf("describe the qualitative quantity")
		}
	case "exact":
		if _, e = decimal(q.Value); e != nil {
			return e
		}
	case "range":
		var lo, hi *big.Rat
		lo, e = decimal(q.Min)
		if e != nil {
			return e
		}
		hi, e = decimal(q.Max)
		if e != nil {
			return e
		}
		if lo.Cmp(hi) > 0 {
			return fmt.Errorf("quantity range is reversed")
		}
	default:
		return fmt.Errorf("unsupported quantity mode")
	}
	return nil
}
func validateDomains(r Record, records map[string]Record) error {
	domain, _ := r.Properties["_domain"].(string)
	if domain != "" {
		if r.Kind != "entity" {
			return fmt.Errorf("domain templates belong to entities")
		}
		definition, ok := domainDefinitions[domain]
		if !ok {
			return fmt.Errorf("unknown domain template")
		}
		for _, f := range definition.Fields {
			v, ok := r.Properties[f.Key]
			if !ok || v == nil || v == "" {
				continue
			}
			switch f.Kind {
			case "quantity":
				if e := validateQuantity(v); e != nil {
					return fmt.Errorf("%s / %s: %w", r.Name, f.Key, e)
				}
			case "entity":
				id, ok := v.(string)
				if !ok || records[id].Kind != "entity" {
					return fmt.Errorf("%s / %s: referenced entity is missing", r.Name, f.Key)
				}
			case "text":
				if _, ok := v.(string); !ok {
					return fmt.Errorf("%s / %s requires text", r.Name, f.Key)
				}
			}
		}
	}
	if r.Kind == "scene" {
		for _, key := range []string{"workID", "chapterID", "povID", "locationID", "arcID"} {
			if v, ok := r.Properties[key]; ok && v != "" && v != nil {
				if id, ok := v.(string); !ok || !validID(id) {
					return fmt.Errorf("invalid scene planning reference")
				} else if key != "povID" && key != "locationID" && records[id].Kind != "entity" {
					return fmt.Errorf("scene planning entity is missing")
				}
			}
		}
	}
	return nil
}

type GraphResult struct {
	Nodes     []Record `json:"nodes"`
	Edges     []Record `json:"edges"`
	Truncated bool     `json:"truncated"`
}

func Related(records map[string]Record, id string, depth int) GraphResult {
	depth = max(0, min(depth, 8))
	out := GraphResult{Nodes: []Record{}, Edges: []Record{}}
	seen := map[string]bool{id: true}
	front := []string{id}
	relations := []Record{}
	for _, r := range records {
		if r.Kind == "relation" {
			relations = append(relations, r)
		}
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].ID < relations[j].ID })
	for level := 0; level < depth; level++ {
		next := []string{}
		for _, at := range front {
			for _, r := range relations {
				to := ""
				if r.From == at {
					to = r.To
				} else if r.To == at {
					to = r.From
				}
				if to != "" && !seen[to] {
					if len(seen) >= 200 {
						out.Truncated = true
						continue
					}
					seen[to] = true
					next = append(next, to)
				}
			}
		}
		front = next
	}
	for id := range seen {
		if r, ok := records[id]; ok {
			out.Nodes = append(out.Nodes, r)
		}
	}
	for _, r := range relations {
		if seen[r.From] && seen[r.To] {
			out.Edges = append(out.Edges, r)
		}
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].Name < out.Nodes[j].Name })
	return out
}
