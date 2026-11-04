package httpapi

import (
	"context"
	"kriemhild/internal/languages"
	"kriemhild/internal/storage"
	"net/http"
)

// Called after the same permission capture/admission gate as language compute.
// All disclosure and revision writes go through publishLanguage's recheck.
func (s *Server) computeScript(ctx context.Context, w http.ResponseWriter, r *http.Request, req languageComputeRequest, raw []byte, operation string) {
	d, err := languages.DecodeScript(raw)
	if err != nil || d.ScriptID != r.PathValue("language") {
		fail(w, 400, "Invalid Script definition or mismatched identity")
		return
	}
	if d.Schema != 2 {
		fail(w, 422, "Custom rendering requires explicit Script schema 2; schema 1 remains Unicode metadata")
		return
	}
	world, id := r.PathValue("world"), r.PathValue("language")
	if operation == "import-svg" {
		result, err := languages.ImportGlyphSVG(ctx, req.SVG, req.GlyphID, req.GlyphName)
		s.publishLanguage(w, r, func() {
			if err != nil {
				fail(w, 422, err.Error())
				return
			}
			respond(w, 200, map[string]any{"import": result})
		})
		return
	}
	report := languages.CompileScript(ctx, d)
	if ctx.Err() != nil {
		s.publishLanguage(w, r, func() { respond(w, 422, map[string]any{"status": "execution_limit", "report": report}) })
		return
	}
	if operation == "render-script" || operation == "run" {
		if operation == "run" {
			doc, err := s.projects.History(ctx, world)
			if err != nil {
				fail(w, 404, "World not found")
				return
			}
			entity := doc.Entity(req.AgeID, id)
			compiled := false
			if entity != nil && entity.Script != nil {
				for _, ref := range entity.Script.Compiled {
					if ref.Revision == report.Revision && languages.CompatibleScriptVersions(d.Schema, ref.Engine, ref.Adapter, ref.Compiler) {
						compiled = true
					}
				}
			}
			if !compiled {
				s.publishLanguage(w, r, func() { fail(w, 409, "Compile this exact saved Script revision before rendering it") })
				return
			}
		}
		q := req.ScriptRequest
		// Draft preview explicitly captures and pins this canonical authored draft;
		// saved execution never supplies a missing revision or version pin.
		if operation == "render-script" && len(req.Definition) > 0 && q.Revision == "" {
			q.Revision = report.Revision
		}
		result := languages.RenderScript(ctx, d, q)
		s.publishLanguage(w, r, func() {
			respond(w, 200, map[string]any{"scriptResult": result, "report": report, "context": map[string]string{"worldId": world, "ageId": req.AgeID, "scriptId": id}})
		})
		return
	}
	s.publishLanguage(w, r, func() {
		if operation == "validate" {
			respond(w, 200, map[string]any{"report": report})
			return
		}
		if operation != "compile" {
			fail(w, 422, "Unsupported Script operation; historical evolution and spelling execution are not implemented")
			return
		}
		doc, err := s.projects.RecordLanguageCompile(storage.WithOperation(r.Context(), "script-compiled"), world, req.AgeID, id, req.Revision, report)
		if err != nil {
			fail(w, 409, err.Error())
			return
		}
		respond(w, 200, map[string]any{"document": doc, "report": report})
	})
}
