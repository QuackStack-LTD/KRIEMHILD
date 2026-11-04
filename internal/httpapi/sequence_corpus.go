package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kriemhild/internal/languages"
	"net/http"
	"time"
)

// Verification uses the existing capture/compute/final-publication boundary.
// It does not record a compile, activate a head or write corpus files to a World.
func (s *Server) sequenceCorpus(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	select {
	case s.languageSlots <- struct{}{}:
		defer func() { <-s.languageSlots }()
	default:
		fail(w, 429, "Two linguistic jobs are already running")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var req struct {
		AgeID  string          `json:"ageId"`
		Mode   string          `json:"mode"`
		Corpus json.RawMessage `json:"corpus"`
	}
	if err := languageDecode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if !languages.ValidID(req.AgeID) || req.Mode != "validate" && req.Mode != "execute" {
		fail(w, 400, "Use an explicit Age and validate or execute mode")
		return
	}
	c, err := languages.DecodeSequenceCorpus(req.Corpus)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	for _, test := range c.Cases {
		if test.Pin.LanguageID != r.PathValue("language") {
			fail(w, 400, "Every case must belong to this language identity")
			return
		}
	}
	world := r.PathValue("world")
	resolve := func(ctx context.Context, kind, id, revision string) ([]byte, error) {
		if kind != "language" && kind != "script" {
			return nil, fmt.Errorf("unsupported dependency")
		}
		return s.projects.LinguisticDefinition(ctx, world, req.AgeID, id, revision)
	}
	report, err := languages.RunSequenceCorpus(ctx, c, resolve, req.Mode == "validate")
	if err != nil {
		s.publishLanguage(w, r, func() { fail(w, 422, err.Error()) })
		return
	}
	s.publishLanguage(w, r, func() {
		// Recheck retained ownership after bounded work, as well as the session
		// and membership checked by publishLanguage. No compile under the gate.
		for _, test := range c.Cases {
			if _, err := resolve(r.Context(), "language", test.Pin.LanguageID, test.Pin.Revision); err != nil {
				fail(w, 409, "Pinned language is no longer retained in this World/Age")
				return
			}
			if _, err := resolve(r.Context(), "script", test.Pin.Script.ID, test.Pin.Script.Revision); err != nil {
				fail(w, 409, "Pinned Script is no longer retained in this World/Age")
				return
			}
		}
		respond(w, 200, map[string]any{"corpus": report, "summary": languages.CorpusSummary(report)})
	})
}
