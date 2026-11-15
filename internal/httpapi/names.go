package httpapi

import (
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http"
	"strconv"
)

func (s *Server) sessionNames(w http.ResponseWriter, r *http.Request) {
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.historical != nil && s.projects != nil {
		revision, err := s.projects.CurrentRevision(r.Context(), v.historyWorld)
		if err != nil {
			fail(w, 409, err.Error())
			return
		}
		if revision != v.historyRevision {
			if s.projects.MapUnchangedSince(r.Context(), v.historyWorld, v.historyRevision, revision) {
				v.historyRevision = revision
			} else if change, e := s.projects.NameChangesSince(r.Context(), v.historyWorld, v.historyRevision, revision); e == nil && change != nil {
				applySessionNameChange(v, change)
			} else if err = s.applyHistoricalMap(r.Context(), v); err != nil {
				fail(w, 409, err.Error())
				return
			} else {
				v.authored().Header.Revision++
			}
		}
	}
	requested, parseErr := strconv.Atoi(r.URL.Query().Get("revision"))
	if parseErr == nil && (requested == v.historyRevision || v.historical != nil && s.projects != nil && s.projects.MapUnchangedSince(r.Context(), v.historyWorld, requested, v.historyRevision)) {
		respond(w, 200, map[string]any{"unchanged": true, "historyRevision": v.historyRevision, "historicalMap": v.historical})
		return
	}
	type name struct {
		ID        string `json:"id"`
		LogicalID string `json:"logicalId,omitempty"`
		SourceID  string `json:"sourceId,omitempty"`
		Name      string `json:"name"`
		LabelName string `json:"labelName,omitempty"`
	}
	names := []name{}
	for _, e := range v.authored().Entities {
		if !e.Deleted {
			names = append(names, name{e.ID, e.LogicalID, e.SourceID, e.Name, e.LabelName})
		}
	}
	namesOnly := false
	ecologyNamesChanged := false
	if parseErr == nil && v.historyWorld != "" && s.projects != nil {
		change, err := s.projects.NameChangesSince(r.Context(), v.historyWorld, requested, v.historyRevision)
		namesOnly = err == nil && change != nil
		if namesOnly {
			for _, e := range change.Entities {
				ecologyNamesChanged = ecologyNamesChanged || e.Kind == "lifeform"
			}
		}
	}
	respond(w, 200, map[string]any{"ecologyNamesChanged": ecologyNamesChanged, "namesOnly": namesOnly, "names": names, "name": v.authored().Header.Name, "revision": v.authored().Header.Revision, "historyRevision": v.historyRevision, "historicalMap": v.historical, "illustrations": v.authored().Header.Illustrations, "terrainSource": continuousFeatureSource(v.planet), "analysisStale": v.terrainAnalysisStale()})
}

func (s *Server) worldNames(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	revision, err := s.projects.CurrentRevision(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	if r.URL.Query().Get("revision") == strconv.Itoa(revision) {
		respond(w, 200, map[string]any{"unchanged": true})
		return
	}
	if r.URL.Query().Get("delta") == "names" {
		from, err := strconv.Atoi(r.URL.Query().Get("revision"))
		if err == nil {
			change, err := s.projects.NameChangesSince(r.Context(), r.PathValue("world"), from, revision)
			if err == nil && change != nil {
				respondHistory(w, r, 200, map[string]any{"nameChange": change})
				return
			}
		}
	}
	d, err := s.readHistoryView(r.Context(), r.PathValue("world"), r.URL.Query().Get("view"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	respondHistory(w, r, 200, map[string]any{"document": d})
}

func applySessionNameChange(v *session, change *storage.NameChange) {
	if v.retired || v.historyWorld != change.World.ID || v.historyRevision != change.Base || v.historical == nil {
		return
	}
	model := v.authored()
	byID := map[string]history.Entity{}
	for _, e := range change.Entities {
		if e.AgeID == v.historical.AgeID {
			byID[e.ID] = e
		}
	}
	names := map[string]world.Entity{}
	for id, shape := range model.Entities {
		logical := shape.LogicalID
		if logical == "" {
			logical = id
		}
		if e, ok := byID[logical]; ok && shape.RecordStatus != "draft" {
			if shape.LabelName == shape.Name {
				shape.LabelName = e.Name
			}
			shape.Name = e.Name
			names[id] = shape
		}
	}
	model.RefreshNames(names)
	for _, m := range change.Maps {
		if m.ID == v.historical.ID && m.AgeID == v.historical.AgeID {
			copy := *v.historical
			copy.Name = m.Name
			v.historical = &copy
			model.Header.Name = m.Name
		}
	}
	v.historyRevision = change.World.Revision
	v.composed = nil
}

func (s *Server) refreshNameSessions(change *storage.NameChange) {
	s.mu.Lock()
	views := make([]*session, 0, len(s.sessions))
	for _, v := range s.sessions {
		views = append(views, v)
	}
	s.mu.Unlock()
	for _, v := range views {
		// A busy editor catches up from the committed patch on its next
		// request. Its terrain computation must not delay a profile rename.
		if !v.mu.TryLock() {
			continue
		}
		applySessionNameChange(v, change)
		v.mu.Unlock()
	}
}

// A logical-record edit updates live map projections, never terrain snapshots.
// Called without a session lock; each session retains its pending local edits.
func (s *Server) refreshHistorySessions(d *history.Document, refreshMaps bool) {
	s.mu.Lock()
	views := make([]*session, 0)
	for _, v := range s.sessions {
		views = append(views, v)
	}
	s.mu.Unlock()
	for _, v := range views {
		v.mu.Lock()
		if !v.retired && v.historyWorld == d.World.ID {
			if m := d.Project(v.worldID); m != nil {
				if refreshMaps {
					s.projectHistoricalMap(v, d, m)
					v.authored().Header.Revision++
					v.collabRevision++
				} else {
					// A new Age changes no existing map. Advance its history token
					// without rebuilding the old projection or invalidating edits.
					v.historical = m
					v.historyRevision = d.World.Revision
				}
			}
		}
		v.mu.Unlock()
	}
}
