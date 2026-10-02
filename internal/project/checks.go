package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Finding struct {
	ID          string `json:"id"`
	Rule        string `json:"rule"`
	Severity    string `json:"severity"`
	Record      string `json:"record"`
	Message     string `json:"message"`
	Evidence    string `json:"evidence"`
	Fingerprint string `json:"fingerprint"`
	Excepted    bool   `json:"excepted"`
}

func (s *Store) Checks(age string) ([]Finding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.state(age)
	if err != nil {
		return nil, err
	}
	out := []Finding{}
	add := func(rule, severity string, r Record, message, evidence string) {
		data, _ := json.Marshal([]any{rule, r, evidence})
		hash := sha256.Sum256(data)
		finger := hex.EncodeToString(hash[:])
		excepted := false
		for _, note := range st.Records {
			if note.Kind == "note" && note.Properties["exceptionFingerprint"] == finger && strings.TrimSpace(note.Notes) != "" {
				excepted = true
			}
		}
		out = append(out, Finding{rule + ":" + r.ID, rule, severity, r.ID, message, evidence, finger, excepted})
	}
	names := map[string][]Record{}
	for _, r := range st.Records {
		if r.Kind == "entity" {
			key := strings.ToLower(r.Name)
			names[key] = append(names[key], r)
		}
	}
	for _, items := range names {
		if len(items) > 1 {
			for _, r := range items {
				add("duplicate-name-v1", "information", r, "Several identities share this name; they have not been merged.", fmt.Sprintf("%d separate entities named %s", len(items), r.Name))
			}
		}
	}
	for _, r := range st.Records {
		if r.Kind == "note" {
			if id, ok := r.Properties["commentOn"].(string); ok && id != "" {
				scene, exists := st.Records[id]
				if !exists || scene.Kind != "scene" {
					add("orphan-comment-v1", "information", r, "The commented scene is absent from this Age.", id)
				} else if block, ok := r.Properties["blockID"].(string); ok && block != "" {
					var doc map[string]any
					json.Unmarshal(scene.Document, &doc)
					var has func(map[string]any) bool
					has = func(n map[string]any) bool {
						attrs, _ := n["attrs"].(map[string]any)
						if attrs["blockId"] == block {
							return true
						}
						children, _ := n["content"].([]any)
						for _, raw := range children {
							child, _ := raw.(map[string]any)
							if has(child) {
								return true
							}
						}
						return false
					}
					if !has(doc) {
						add("orphan-comment-anchor-v1", "information", r, "The comment's paragraph anchor is no longer present. The comment was preserved.", id+" / "+block)
					}
				}
			}
		}
		if r.Kind == "map" && r.Terrain != nil {
			for _, p := range r.Pins {
				i := min(r.Terrain.Rows-1, int(p.Y*float64(r.Terrain.Rows)))*r.Terrain.Columns + min(r.Terrain.Columns-1, int(p.X*float64(r.Terrain.Columns)))
				if r.Terrain.Water[i] != 0 {
					add("submerged-pin-v1", "warning", r, "A mapped entity is in water; this may be intentional.", st.Records[p.Entity].Name+" / "+p.Entity)
				}
			}
		}
		if r.Kind == "scene" {
			context, err := s.records(r.SettingSnapshot)
			if err != nil {
				return nil, err
			}
			if r.SettingDate != "" {
				historic, err := s.historical(r.SettingSnapshot, r.SettingDate)
				if err != nil {
					return nil, err
				}
				context = historic.Records
				for _, u := range historic.Unresolved {
					add("unresolved-setting-v1", "information", r, "Some setting information has not been asserted.", u)
				}
			}
			for _, id := range r.References {
				historical := context[id]
				if historical.Status == "destroyed" {
					add("inactive-scene-reference-v1", "warning", r, "This scene references an entity marked destroyed in its setting.", historical.Name+" / "+id)
				}
				if current, ok := st.Records[id]; ok && current.Name != historical.Name {
					add("renamed-reference-v1", "information", r, "The current name differs from the pinned setting. Prose was not changed.", historical.Name+" → "+current.Name)
				}
			}
		}
		if r.Properties["_domain"] == "recipe" && r.Properties["inputs"] == "" && r.Properties["_production"] == nil {
			add("missing-recipe-inputs-v1", "information", r, "Production inputs have not been described.", "Recipe inputs are empty.")
		}
		if r.Properties["_domain"] == "route" {
			if r.Properties["origin"] == nil || r.Properties["destination"] == nil || r.Properties["origin"] == "" || r.Properties["destination"] == "" {
				add("route-endpoints-v1", "information", r, "A transport route has incomplete endpoints.", "Origin and destination are needed for network calculations.")
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID == out[j].ID {
			return out[i].Fingerprint < out[j].Fingerprint
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
