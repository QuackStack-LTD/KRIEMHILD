package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"kriemhild/internal/history"
	"kriemhild/internal/storage"
	"kriemhild/internal/world"
	"net/http"
	"reflect"
	"time"
)

func (s *Server) saveIllustratedMap(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, history.MaxLibraryFileSize+(1<<20))
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		fail(w, 400, "The illustration could not be read or exceeds 64 MiB")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	var req struct {
		SourceRevision int                `json:"sourceRevision"`
		Illustration   world.Illustration `json:"illustration"`
	}
	if err := json.Unmarshal([]byte(r.FormValue("metadata")), &req); err != nil {
		fail(w, 400, "Invalid illustration metadata")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "The rendered illustration is required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, history.MaxLibraryFileSize+1))
	if err != nil || len(data) > history.MaxLibraryFileSize {
		fail(w, 413, "The illustration exceeds 64 MiB")
		return
	}
	var banner []byte
	if image, _, e := r.FormFile("banner"); e == nil {
		defer image.Close()
		banner, err = io.ReadAll(io.LimitReader(image, history.MaxLibraryFileSize+1))
		if err != nil || len(banner) > history.MaxLibraryFileSize {
			fail(w, 413, "The banner exceeds 64 MiB")
			return
		}
	}
	v := s.worldSession(w, r)
	if v == nil {
		return
	}
	defer v.mu.Unlock()
	if v.historical == nil || s.projects == nil {
		fail(w, 409, "Save this terrain to its World before saving an illustration")
		return
	}
	saved, ok := v.authored().Header.Illustrations[req.Illustration.ID]
	if req.Illustration.LandKey != "" && req.Illustration.LandKey != landKey(v) {
		fail(w, 409, "The landscape changed. Select the connected land again before saving this map.")
		return
	}
	if !ok || req.SourceRevision != v.authored().Header.Revision || !reflect.DeepEqual(saved, req.Illustration) {
		fail(w, 409, "The map or illustration changed while rendering; save the illustration again")
		return
	}
	focusID := ""
	if req.Illustration.Scope == "country" || req.Illustration.Scope == "city" {
		if req.Illustration.Focus != "" {
			shape, found := v.authored().Entities[req.Illustration.Focus]
			if !found || shape.Deleted {
				fail(w, 409, "The selected place is no longer available")
				return
			}
			focusID = shape.LogicalID
			if focusID == "" {
				focusID = shape.ID
			}
		} else {
			focusID = v.historical.EntityID
		}
	}
	doc, id, folder, err := s.projects.SaveIllustratedMap(r.Context(), v.historyWorld, v.historyRevision, storage.IllustratedMapRequest{MapID: v.historical.ID, AgeID: v.historical.AgeID, FocusID: focusID, SourceRevision: req.SourceRevision, Illustration: saved, PNG: data, BannerPNG: banner})
	if err != nil {
		fail(w, 409, err.Error())
		return
	}
	v.historyRevision = doc.World.Revision
	w.Header().Set("Server-Timing", fmt.Sprintf("illustration-save;dur=%.2f", float64(time.Since(started).Microseconds())/1000))
	respond(w, 201, map[string]any{"document": doc, "fileId": id, "archiveId": folder, "saved": true})
}
