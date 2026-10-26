package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kriemhild/internal/languages"
	"kriemhild/internal/languages/spike"
	"kriemhild/internal/storage"
	"net/http"
	"strings"
	"time"
)

func isLanguageCompute(r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	return r.Method == "POST" && len(p) == 6 && p[0] == "api" && p[1] == "worlds" && p[3] == "languages" && (p[5] == "compile" || p[5] == "run" || p[5] == "validate" || p[5] == "history-preview" || p[5] == "history-apply" || p[5] == "render-script" || p[5] == "import-svg")
}
func (s *Server) languageRoutes() {
	s.mux.HandleFunc("GET /api/worlds/{world}/languages/capabilities", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"profiles": languages.Capabilities(), "scripts": languages.ScriptCapabilities()})
	})
	s.mux.HandleFunc("POST /api/worlds/{world}/languages", s.saveLanguage)
	s.mux.HandleFunc("GET /api/worlds/{world}/languages/{language}", s.getLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/activate", s.activateLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/restore", s.restoreLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/compile", s.computeLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/validate", s.computeLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/run", s.computeLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/history-preview", s.computeLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/history-apply", s.computeLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/render-script", s.computeLanguage)
	s.mux.HandleFunc("POST /api/worlds/{world}/languages/{language}/import-svg", s.computeLanguage)
}

type languageComputeRequest struct {
	AgeID              string                        `json:"ageId"`
	Revision           int                           `json:"revision"`
	DefinitionRevision string                        `json:"definitionRevision"`
	Definition         json.RawMessage               `json:"definition,omitempty"`
	Request            languages.Request             `json:"request"`
	ScriptRequest      languages.ScriptRenderRequest `json:"scriptRequest,omitempty"`
	SVG                string                        `json:"svg,omitempty"`
	GlyphID            string                        `json:"glyphId,omitempty"`
	GlyphName          string                        `json:"glyphName,omitempty"`
	PreviewHash        string                        `json:"previewHash,omitempty"`
	Approved           bool                          `json:"approved,omitempty"`
}

func languageDecode(w http.ResponseWriter, r *http.Request, v any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, languages.MaxDefinitionBytes))
	if err != nil {
		return fmt.Errorf("request exceeds 2 MiB")
	}
	return spike.DecodeJSON(raw, v)
}
func (s *Server) getLanguage(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	raw, err := s.projects.LinguisticDefinition(r.Context(), r.PathValue("world"), r.URL.Query().Get("age"), r.PathValue("language"), r.URL.Query().Get("revision"))
	if err != nil {
		fail(w, 404, "Language or script revision not found")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="language-definition.json"`)
	}
	w.WriteHeader(200)
	_, _ = w.Write(raw)
}
func (s *Server) saveLanguage(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req struct {
		AgeID          string          `json:"ageId"`
		Revision       int             `json:"revision"`
		Kind           string          `json:"kind"`
		Definition     json.RawMessage `json:"definition"`
		DefinitionText string          `json:"definitionText,omitempty"`
		ExpectedDraft  *string         `json:"expectedDraft,omitempty"`
		DisplayName    *string         `json:"displayName,omitempty"`
		Metadata       languages.State `json:"metadata"`
		Note           string          `json:"note"`
	}
	if err := languageDecode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	if req.DefinitionText != "" {
		if len(req.Definition) > 0 {
			fail(w, 400, "Provide a definition object or import text, not both")
			return
		}
		req.Definition = []byte(req.DefinitionText)
	}
	if req.Metadata.Draft != "" || req.Metadata.Active != "" || len(req.Metadata.Revisions) > 0 || len(req.Metadata.Compiled) > 0 {
		fail(w, 400, "Definition heads, revision references and compile reports are server-managed; use the explicit compile, activate or restore actions")
		return
	}
	d, err := s.projects.SaveLinguisticDefinition(storage.WithOperation(r.Context(), "language-definition-saved"), r.PathValue("world"), req.AgeID, req.Kind, req.Revision, req.Definition, req.Metadata, req.Note, storage.LinguisticSaveOptions{ExpectedDraft: req.ExpectedDraft, DisplayName: req.DisplayName})
	if err != nil {
		status := 422
		if strings.Contains(err.Error(), "changed;") {
			status = 409
		}
		fail(w, status, err.Error())
		return
	}
	respond(w, 200, d)
}
func (s *Server) restoreLanguage(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req struct {
		AgeID         string `json:"ageId"`
		Revision      int    `json:"revision"`
		Definition    string `json:"definitionRevision"`
		ExpectedDraft string `json:"expectedDraft"`
	}
	if err := languageDecode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	d, err := s.projects.RestoreLinguisticDraft(storage.WithOperation(r.Context(), "language-draft-restored"), r.PathValue("world"), req.AgeID, r.PathValue("language"), req.Definition, req.ExpectedDraft, req.Revision)
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	respond(w, 200, d)
}
func (s *Server) activateLanguage(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	var req struct {
		AgeID      string `json:"ageId"`
		Revision   int    `json:"revision"`
		Definition string `json:"definitionRevision"`
	}
	if err := languageDecode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	d, err := s.projects.ActivateLanguage(storage.WithOperation(r.Context(), "language-activated"), r.PathValue("world"), req.AgeID, r.PathValue("language"), req.Definition, req.Revision)
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	respond(w, 200, d)
}

// publishLanguage is the single publication boundary after isolated work. The
// same gate serializes membership changes, session revocation and this commit.
func (s *Server) publishLanguage(w http.ResponseWriter, r *http.Request, publish func()) {
	s.accessMu.Lock()
	defer s.accessMu.Unlock()
	if s.auth != nil {
		if _, _, err := s.auth.User(r); err != nil {
			fail(w, 401, "Sign in to continue")
			return
		}
		if !s.authorize(w, r) {
			return
		}
	}
	if r.Context().Err() != nil {
		return
	}
	publish()
}
func (s *Server) computeLanguage(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	select {
	case s.languageSlots <- struct{}{}:
		defer func() { <-s.languageSlots }()
	default:
		fail(w, 429, "Two linguistic jobs are already running; retry after one finishes")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var req languageComputeRequest
	if err := languageDecode(w, r, &req); err != nil {
		fail(w, 400, err.Error())
		return
	}
	world, id := r.PathValue("world"), r.PathValue("language")
	op := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	var raw []byte
	var err error
	if (op == "validate" || op == "render-script" || op == "import-svg") && len(req.Definition) > 0 {
		raw = req.Definition
	} else {
		if len(req.Definition) > 0 {
			fail(w, 400, "Compile and run use saved immutable revisions")
			return
		}
		raw, err = s.projects.LinguisticDefinition(ctx, world, req.AgeID, id, req.DefinitionRevision)
		if err != nil {
			fail(w, 404, "Language revision not found")
			return
		}
	}
	var header struct {
		ScriptID string `json:"scriptId"`
	}
	if json.Unmarshal(raw, &header) == nil && header.ScriptID != "" {
		s.computeScript(ctx, w, r, req, raw, op)
		return
	}
	if op == "render-script" || op == "import-svg" {
		fail(w, 400, "This action requires an independent Script revision")
		return
	}
	definition, err := languages.Decode(raw)
	if err != nil || definition.LanguageID != id {
		fail(w, 400, "Invalid language definition or mismatched identity")
		return
	}
	if op == "history-preview" || op == "history-apply" {
		source, err := s.projects.LanguageHistoricalSource(ctx, definition)
		if err != nil {
			fail(w, 409, "Historical source revision is missing or invalid")
			return
		}
		preview := languages.PreviewEvolution(ctx, source, definition)
		s.publishLanguage(w, r, func() {
			if op == "history-preview" {
				respond(w, 200, map[string]any{"preview": preview})
				return
			}
			if !req.Approved || preview.Status != "success" || preview.Target == nil || req.PreviewHash != preview.PreviewHash {
				fail(w, 409, "Review and explicitly approve the exact successful historical preview before applying it")
				return
			}
			d, err := s.projects.History(r.Context(), world)
			if err != nil {
				fail(w, 404, "World not found")
				return
			}
			entity := d.Entity(req.AgeID, id)
			if entity == nil || entity.Language == nil {
				fail(w, 404, "Target language not found")
				return
			}
			next, err := s.projects.SaveLinguisticDefinition(storage.WithOperation(r.Context(), "language-history-applied"), world, req.AgeID, "language", req.Revision, languages.Canonical(preview.Target), *entity.Language, "Applied historical preview "+preview.PreviewHash[:12], storage.LinguisticSaveOptions{ExpectedDraft: &preview.TargetRevision})
			if err != nil {
				fail(w, 409, err.Error())
				return
			}
			respond(w, 200, map[string]any{"document": next, "preview": preview})
		})
		return
	}
	deps := map[languages.ScriptRef]languages.ScriptDefinition{}
	if op == "validate" && len(req.Definition) > 0 {
		// Unsaved input has not passed Save's dependency-ownership check.
		if o := definition.Orthography; o != nil {
			body, e := s.projects.LinguisticDefinition(ctx, world, req.AgeID, o.Script.ID, o.Script.Revision)
			if e == nil {
				script, e := languages.DecodeScript(body)
				if e == nil {
					deps[o.Script] = script
				}
			}
		}
	} else {
		deps, err = s.projects.LanguageDependencies(ctx, definition)
		if err != nil {
			fail(w, 409, "A pinned script revision is unavailable or corrupt")
			return
		}
	}
	key := languages.ArtifactKey(world, req.AgeID, definition)
	var artifact *languages.Artifact
	if op != "validate" {
		artifact = s.languageCache.Get(key)
	}
	cacheHit := artifact != nil
	if artifact == nil {
		artifact = languages.Compile(ctx, definition, deps)
		if op != "validate" {
			s.languageCache.Put(key, artifact)
		}
	}
	report := artifact.Report()
	if ctx.Err() != nil {
		s.publishLanguage(w, r, func() { respond(w, 422, map[string]any{"status": "execution_limit", "report": report}) })
		return
	}
	if op == "run" {
		d, e := s.projects.History(ctx, world)
		if e != nil {
			fail(w, 404, "World not found")
			return
		}
		entity := d.Entity(req.AgeID, id)
		compiled := false
		if entity != nil && entity.Language != nil {
			for _, ref := range entity.Language.Compiled {
				if ref.Revision == report.Revision && ref.Engine == report.Engine && ref.Adapter == report.Adapter && ref.Compiler == report.Compiler {
					compiled = true
				}
			}
		}
		if !compiled {
			s.publishLanguage(w, r, func() { fail(w, 409, "Compile this exact saved revision before running it") })
			return
		}
		result := artifact.Execute(ctx, req.Request)
		s.publishLanguage(w, r, func() {
			envelope := map[string]any{"result": result, "report": report, "runtime": map[string]any{"cacheHit": cacheHit, "accountedArtifactBytes": artifact.AccountedBytes()}}
			if languages.ProductiveSchema(definition.Schema) {
				envelope["context"] = map[string]string{"worldId": world, "ageId": req.AgeID, "languageId": id}
			}
			respond(w, 200, envelope)
		})
		return
	}
	s.publishLanguage(w, r, func() {
		if op == "validate" {
			respond(w, 200, map[string]any{"report": report})
			return
		}
		d, err := s.projects.RecordLanguageCompile(storage.WithOperation(r.Context(), "language-compiled"), world, req.AgeID, id, req.Revision, report)
		if err != nil {
			fail(w, 409, err.Error())
			return
		}
		respond(w, 200, map[string]any{"document": d, "report": report})
	})
}
