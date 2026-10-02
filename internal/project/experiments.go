package project

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

type ExperimentRequest struct {
	Expected  string            `json:"expected"`
	Age       string            `json:"age"`
	Operation string            `json:"operation"`
	IDs       []string          `json:"ids"`
	Values    map[string]string `json:"values"`
}
type ExperimentResult struct {
	AllOrNothing bool                `json:"allOrNothing,omitempty"`
	Revision     string              `json:"revision"`
	Snapshot     string              `json:"snapshot"`
	Algorithm    string              `json:"algorithm"`
	Rows         []map[string]string `json:"rows"`
	Proposals    []Record            `json:"proposals"`
	Warnings     []string            `json:"warnings"`
}

func cloneProperties(r Record) Record {
	p := map[string]any{}
	for k, v := range r.Properties {
		p[k] = v
	}
	r.Properties = p
	return r
}
func (s *Store) Experiment(c ExperimentRequest) (ExperimentResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, e := s.state(c.Age)
	out := ExperimentResult{Rows: []map[string]string{}, Proposals: []Record{}, Warnings: []string{}}
	if e != nil {
		return out, e
	}
	if c.Expected != st.Revision {
		return out, ErrConflict
	}
	out.Revision = st.Revision
	out.Snapshot = st.Age.Snapshot
	out.Algorithm = c.Operation + "-v1"
	if len(c.IDs) > 500 {
		return out, fmt.Errorf("select at most 500 inputs")
	}
	selected := []Record{}
	seen := map[string]bool{}
	for _, id := range c.IDs {
		r, ok := st.Records[id]
		if !ok || seen[id] {
			return out, fmt.Errorf("invalid or repeated input")
		}
		seen[id] = true
		selected = append(selected, r)
	}
	switch c.Operation {
	case "production-network":
		if e := productionExperiment(c, st, selected, &out); e != nil {
			return out, e
		}
	case "phonotactics", "inflection":
		if e := conlangExperiment(c, st, selected, &out); e != nil {
			return out, e
		}
	case "sound-change":
		rules, e := parseSoundRules(c.Values["rules"])
		if e != nil {
			return out, e
		}
		classes, e := soundClasses(c.Values["classes"])
		if e != nil {
			return out, e
		}
		out.Algorithm = "sound-change-v2"
		for _, r := range selected {
			if r.Properties["_domain"] != "lexeme" {
				continue
			}
			source, _ := r.Properties["phonemes"].(string)
			tokens := strings.Fields(source)
			if len(tokens) > 1000 {
				return out, fmt.Errorf("lexeme exceeds token budget")
			}
			tokens, e = applySoundRules(tokens, rules, classes)
			if e != nil {
				return out, e
			}
			result := strings.Join(tokens, " ")
			out.Rows = append(out.Rows, map[string]string{"entry": r.Name, "before": source, "after": result})
			if source != result {
				r = cloneProperties(r)
				r.Properties["phonemes"] = result
				r.Properties["derivationSnapshot"] = st.Age.Snapshot
				r.Properties["soundRules"] = c.Values["rules"]
				r.Properties["soundClasses"] = c.Values["classes"]
				out.Proposals = append(out.Proposals, r)
			}
		}
	case "production":
		supply, e := decimal(c.Values["supply"])
		if e != nil {
			return out, e
		}
		demand, e := decimal(c.Values["demand"])
		if e != nil {
			return out, e
		}
		duration, e := decimal(c.Values["duration"])
		if e != nil {
			return out, e
		}
		if supply.Sign() < 0 || demand.Sign() < 0 || duration.Sign() <= 0 {
			return out, fmt.Errorf("supply and demand must be nonnegative; duration must be positive")
		}
		balance := new(big.Rat).Mul(new(big.Rat).Sub(supply, demand), duration)
		out.Rows = append(out.Rows, map[string]string{"supply per period": supply.RatString(), "demand per period": demand.RatString(), "periods": duration.RatString(), "balance (exact)": balance.RatString(), "unit": c.Values["unit"]})
		out.Warnings = append(out.Warnings, "Assumes constant rates, matching units and unconstrained transport. No population or political consequences are inferred.")
	case "inheritance":
		a, b := c.Values["parentA"], c.Values["parentB"]
		allowed := map[string]bool{"AA": true, "Aa": true, "aa": true}
		if !allowed[a] || !allowed[b] {
			out.Warnings = append(out.Warnings, "Both parental states must be known AA, Aa or aa; unknown states remain unresolved.")
			break
		}
		counts := map[string]int{}
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				pair := string(a[i]) + string(b[j])
				if pair == "aA" {
					pair = "Aa"
				}
				counts[pair]++
			}
		}
		for _, pair := range []string{"AA", "Aa", "aa"} {
			out.Rows = append(out.Rows, map[string]string{"state": pair, "probability": fmt.Sprintf("%d/4", counts[pair])})
		}
		out.Warnings = append(out.Warnings, "A single author-defined two-allele rule. This does not choose a character's traits or determine identity or culture.")
	case "succession":
		if len(selected) != 1 {
			return out, fmt.Errorf("select one founder or office holder")
		}
		roles := strings.Split(c.Values["roles"], ",")
		allowed := map[string]bool{}
		for _, role := range roles {
			allowed[strings.TrimSpace(role)] = true
		}
		queue := []string{selected[0].ID}
		depth := map[string]int{selected[0].ID: 0}
		for len(queue) > 0 {
			at := queue[0]
			queue = queue[1:]
			ids := []string{}
			for _, r := range st.Records {
				if r.Kind == "relation" && r.From == at && allowed[r.Name] {
					ids = append(ids, r.To)
				}
			}
			sort.Strings(ids)
			for _, id := range ids {
				if _, ok := depth[id]; ok {
					continue
				}
				if len(depth) >= 200 {
					return out, fmt.Errorf("succession graph exceeds 200 identities")
				}
				depth[id] = depth[at] + 1
				queue = append(queue, id)
				candidate := st.Records[id]
				if candidate.Status != "destroyed" {
					out.Rows = append(out.Rows, map[string]string{"candidate": candidate.Name, "id": id, "distance": fmt.Sprint(depth[id]), "basis": "reachable via selected parent roles"})
				}
			}
		}
		sort.Slice(out.Rows, func(i, j int) bool {
			a, b := out.Rows[i], out.Rows[j]
			if depth[a["id"]] != depth[b["id"]] {
				return depth[a["id"]] < depth[b["id"]]
			}
			return a["candidate"] < b["candidate"]
		})
		out.Warnings = append(out.Warnings, "Breadth-first candidate list, not an automatic succession decision. Dates, exclusions and disputed claims require author review.")
	case "climate":
		delta, e := decimal(c.Values["delta"])
		if e != nil {
			return out, e
		}
		for _, r := range selected {
			if r.Properties["_domain"] != "climate" {
				continue
			}
			q, ok := r.Properties["temperature"].(map[string]any)
			if !ok || q["mode"] != "exact" || q["unit"] != c.Values["unit"] {
				out.Warnings = append(out.Warnings, r.Name+": exact temperature in the requested unit is missing")
				continue
			}
			before, e := decimal(fmt.Sprint(q["value"]))
			if e != nil {
				return out, e
			}
			after := new(big.Rat).Add(before, delta)
			r = cloneProperties(r)
			r.Properties["temperature"] = map[string]any{"mode": "exact", "value": after.FloatString(6), "unit": c.Values["unit"]}
			r.Properties["calculationSnapshot"] = st.Age.Snapshot
			out.Proposals = append(out.Proposals, r)
			out.Rows = append(out.Rows, map[string]string{"climate": r.Name, "before": before.RatString(), "after": after.RatString(), "unit": c.Values["unit"]})
		}
		out.Warnings = append(out.Warnings, "Uniform temperature offset only, rounded to six decimal places when accepted. Habitats, crops and migration are not simulated.")
	case "erosion":
		steps := 0
		fmt.Sscan(c.Values["steps"], &steps)
		if steps < 1 || steps > 30 {
			return out, fmt.Errorf("use 1–30 smoothing steps")
		}
		for _, r := range selected {
			if r.Kind != "map" || r.Terrain == nil {
				continue
			}
			t := *r.Terrain
			t.Heights = append([]int{}, t.Heights...)
			for n := 0; n < steps; n++ {
				prior := append([]int{}, t.Heights...)
				for i, h := range prior {
					ns := neighbors(&t, i)
					sum := h * 4
					for _, j := range ns {
						sum += prior[j]
					}
					t.Heights[i] = sum / (4 + len(ns))
				}
			}
			derive(&t)
			r.Terrain = &t
			out.Proposals = append(out.Proposals, r)
			out.Rows = append(out.Rows, map[string]string{"map": r.Name, "steps": fmt.Sprint(steps), "scale": "geological editing experiment"})
		}
		out.Warnings = append(out.Warnings, "A bounded smoothing approximation, not a physical erosion or climate model. Review geographic effects before acceptance.")
	case "route":
		origin, destination := c.Values["origin"], c.Values["destination"]
		if st.Records[origin].Kind != "entity" || st.Records[destination].Kind != "entity" {
			return out, fmt.Errorf("select valid route endpoints")
		}
		distance := map[string]*big.Rat{origin: new(big.Rat)}
		previous := map[string]string{}
		done := map[string]bool{}
		reached := false
		for step := 0; step < 1000; step++ {
			at := ""
			for id, d := range distance {
				if !done[id] && (at == "" || d.Cmp(distance[at]) < 0 || (d.Cmp(distance[at]) == 0 && id < at)) {
					at = id
				}
			}
			if at == destination {
				reached = true
				break
			}
			if at == "" {
				break
			}
			done[at] = true
			for _, r := range st.Records {
				if r.Properties["_domain"] != "route" || r.Properties["origin"] != at {
					continue
				}
				to, _ := r.Properties["destination"].(string)
				if done[to] {
					continue
				}
				q, ok := r.Properties["cost"].(map[string]any)
				if !ok || q["mode"] != "exact" || q["unit"] != c.Values["unit"] {
					continue
				}
				cost, e := decimal(fmt.Sprint(q["value"]))
				if e != nil || cost.Sign() < 0 {
					return out, fmt.Errorf("route costs must be nonnegative decimals")
				}
				d := new(big.Rat).Add(distance[at], cost)
				if old, ok := distance[to]; !ok || d.Cmp(old) < 0 {
					distance[to] = d
					previous[to] = at
				}
			}
		}
		if d, ok := distance[destination]; ok && reached {
			path := []string{}
			for at := destination; at != ""; at = previous[at] {
				path = append([]string{st.Records[at].Name}, path...)
				if at == origin {
					break
				}
			}
			out.Rows = append(out.Rows, map[string]string{"path": strings.Join(path, " → "), "cost": d.RatString(), "unit": c.Values["unit"]})
		} else {
			out.Warnings = append(out.Warnings, "No completed route was found using directed edges with exact costs in this unit within the 1000-node search budget.")
		}
	default:
		return out, fmt.Errorf("unknown experiment")
	}
	return out, nil
}
