package httpapi

import (
	"image"
	"image/color"
	"image/png"
	"kriemhild/internal/legacyterrain"
	"math"
	"net/http"
)

// Atlas previews read immutable saved terrain, without opening or regenerating it.
func (s *Server) atlasPreview(w http.ResponseWriter, r *http.Request) {
	if !s.historyStore(w) {
		return
	}
	d, err := s.projects.History(r.Context(), r.PathValue("world"))
	if err != nil {
		fail(w, 404, "World not found")
		return
	}
	m := d.Map(r.URL.Query().Get("age"), r.PathValue("map"))
	if m == nil {
		fail(w, 404, "Map not found")
		return
	}
	var width, height int
	for _, t := range d.Terrains {
		if t.AgeID == m.AgeID && t.ID == m.TerrainID {
			width, height = t.Width, t.Height
		}
	}
	raw, err := s.projects.SnapshotPart(r.Context(), m.Snapshot, "environment")
	if err != nil {
		fail(w, 404, "Terrain preview unavailable")
		return
	}
	var env *terrain.Environment
	if expandJSON(raw, &env) != nil || env == nil {
		fail(w, 404, "Terrain preview unavailable")
		return
	}
	get := func(k string, i int) float64 {
		if i < len(env.Fields[k]) {
			return env.Fields[k][i]
		}
		return 0
	}
	highlight := habitatCells(d, m.AgeID, m.TerrainID, r.URL.Query().Get("habitat"))
	if resourceID := r.URL.Query().Get("resource"); resourceID != "" {
		highlight = map[int]bool{}
		found := false
		if env.Resources != nil {
			for _, o := range env.Resources.Occurrences {
				if o.ID != resourceID {
					continue
				}
				found = true
				for _, cell := range o.Geometry.Cells {
					highlight[cell] = true
				}
				mark := func(p [2]float64) {
					x, y := int(math.Round(p[0])), int(math.Round(p[1]))
					if x >= 0 && y >= 0 && x < width && y < height {
						highlight[y*width+x] = true
					}
				}
				if o.Geometry.Point != nil {
					mark(*o.Geometry.Point)
				}
				for i, p := range o.Geometry.Path {
					mark(p)
					if i > 0 {
						a := o.Geometry.Path[i-1]
						n := int(math.Ceil(math.Max(math.Abs(p[0]-a[0]), math.Abs(p[1]-a[1]))))
						for j := 1; j < n; j++ {
							t := float64(j) / float64(n)
							mark([2]float64{a[0] + (p[0]-a[0])*t, a[1] + (p[1]-a[1])*t})
						}
					}
				}
			}
		}
		if !found {
			fail(w, 404, "Stored resource occurrence not found")
			return
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			z := get("elevation", i)
			var c color.RGBA
			switch {
			case get("ocean", i) > .5 || get("waterDepth", i) > 0:
				c = color.RGBA{42, 83, 103, 255}
			case get("temperature", i) < -8:
				c = color.RGBA{205, 215, 207, 255}
			case z > 2300:
				c = color.RGBA{141, 139, 124, 255}
			case get("aridity", i) > .7:
				c = color.RGBA{182, 166, 119, 255}
			default:
				c = color.RGBA{103, 135, 95, 255}
			}
			shade := math.Max(.7, math.Min(1.2, 1+z/18000))
			c.R = uint8(math.Min(255, float64(c.R)*shade))
			c.G = uint8(math.Min(255, float64(c.G)*shade))
			c.B = uint8(math.Min(255, float64(c.B)*shade))
			if len(highlight) > 0 {
				if highlight[i] {
					c = color.RGBA{120, 240, 185, 255}
				} else {
					c.R /= 2
					c.G /= 2
					c.B /= 2
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=60")
	_ = png.Encode(w, img)
}
