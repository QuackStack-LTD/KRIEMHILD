package httpapi

import (
	"context"
	"kriemhild/internal/biosphere"
	"kriemhild/internal/history"
	"net/http"
)

func (s *Server) readHistoryView(ctx context.Context, id, view string) (*history.Document, error) {
	switch view {
	case "atlas":
		return s.projects.HistoryAtlas(ctx, id)
	case "metadata":
		return s.projects.HistoryMetadata(ctx, id)
	case "workspace":
		return s.projects.HistoryWorkspace(ctx, id)
	default:
		return s.projects.History(ctx, id)
	}
}

func historyResponseView(d *history.Document, r *http.Request) *history.Document {
	switch r.URL.Query().Get("view") {
	case "atlas":
		copy := *d
		copy.Biospheres = nil
		copy.Entities = make([]history.Entity, 0)
		for _, e := range d.Entities {
			if e.Kind != "lifeform" {
				copy.Entities = append(copy.Entities, e)
			}
		}
		return &copy
	case "metadata":
		return &history.Document{World: d.World, Ages: d.Ages, Terrains: d.Terrains, Maps: d.Maps}
	case "workspace":
		copy := *d
		copy.Biospheres = append([]biosphere.State(nil), d.Biospheres...)
		for i := range copy.Biospheres {
			copy.Biospheres[i].Habitats = append([]biosphere.Habitat(nil), copy.Biospheres[i].Habitats...)
			for j := range copy.Biospheres[i].Habitats {
				copy.Biospheres[i].Habitats[j].Profiles = nil
			}
		}
		return &copy
	default:
		return d
	}
}
