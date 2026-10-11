package world

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Artwork is an immutable rendered snapshot, never a source of geography.
// Map/Age record its origin; the owning map snapshot also carries it when an
// Age inherits that map. Geometry edits and object undo do not alter artwork.
type Artwork struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	MapID           string `json:"mapId"`
	AgeID           string `json:"ageId,omitempty"`
	WorldID         string `json:"worldId,omitempty"`
	ParentMapID     string `json:"parentMapId,omitempty"`
	SourceRevision  int    `json:"sourceRevision"`
	CreatedAt       int64  `json:"createdAt"`
	RendererVersion string `json:"rendererVersion"`
	Style           string `json:"style"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	Bounds          Bounds `json:"bounds"`
	SVG             string `json:"svg"`
}

func (a Artwork) Validate() error {
	if !validID(a.ID) || strings.TrimSpace(a.Name) == "" || len(a.Name) > 160 || a.MapID == "" || a.SourceRevision < 0 || a.Width < 320 || a.Width > 8192 || a.Height < 240 || a.Height > 8192 || a.Style != "medieval-white" || a.RendererVersion != "medieval-1" || len(a.SVG) > 12<<20 {
		return fmt.Errorf("invalid artwork metadata")
	}
	if !finite(a.Bounds.X) || !finite(a.Bounds.Y) || !finite(a.Bounds.Width) || !finite(a.Bounds.Height) || a.Bounds.Width <= 0 || a.Bounds.Height <= 0 {
		return fmt.Errorf("invalid artwork extent")
	}
	// Portable, inert SVG only. No scripts, event handlers, foreign HTML, remote
	// resources, CSS imports, or image payloads in saved/downloaded artwork.
	allowed := map[string]bool{}
	for _, tag := range strings.Fields("svg title desc defs clipPath g path rect circle ellipse line polyline polygon text tspan use symbol") {
		allowed[tag] = true
	}
	d := xml.NewDecoder(strings.NewReader(a.SVG))
	depth, roots := 0, 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid artwork SVG: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if !allowed[t.Name.Local] || (t.Name.Space != "" && t.Name.Space != "http://www.w3.org/2000/svg") {
				return fmt.Errorf("unsupported artwork element")
			}
			if depth == 0 {
				roots++
				if t.Name.Local != "svg" {
					return fmt.Errorf("artwork needs an SVG root")
				}
			}
			depth++
			for _, attr := range t.Attr {
				k := strings.ToLower(attr.Name.Local)
				v := strings.ToLower(strings.TrimSpace(attr.Value))
				if strings.HasPrefix(k, "on") || k == "style" || k == "base" || (k == "href" && !strings.HasPrefix(v, "#")) || (strings.Contains(v, "url(") && !strings.HasPrefix(v, "url(#")) {
					return fmt.Errorf("unsafe artwork attribute")
				}
			}
		case xml.EndElement:
			depth--
		case xml.Directive, xml.ProcInst:
			return fmt.Errorf("unsupported artwork directive")
		}
	}
	if roots != 1 || depth != 0 {
		return fmt.Errorf("invalid artwork document")
	}
	return nil
}

func (s *State) AddArtwork(a Artwork) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if s.Artworks == nil {
		s.Artworks = map[string]Artwork{}
	}
	if _, ok := s.Artworks[a.ID]; ok {
		return fmt.Errorf("artwork snapshots cannot be overwritten")
	}
	s.Artworks[a.ID] = a
	s.dirty["artwork/"+a.ID] = true
	return nil
}

// DiscardArtwork rolls back an uncommitted artifact after a failed transaction.
func (s *State) DiscardArtwork(id string) { delete(s.Artworks, id); delete(s.dirty, "artwork/"+id) }
