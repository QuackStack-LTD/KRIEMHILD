package httpapi

import (
	"image"
	"image/color"
	"image/png"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/model"
	"math"
	"net/http"
)

func renderResourceCircle(w http.ResponseWriter, r *http.Request, s *resourceSnapshot, center model.Point, radius float64, outlines [][][2]float64) {
	const size = 384
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	planetRadius := s.e.Geography.Radius
	var sampler *detail.Sampler
	if s.p != nil {
		sampler = detail.New(s.p)
	}
	get := func(k string, i int) float64 {
		if i < len(s.e.Fields[k]) {
			return s.e.Fields[k][i]
		}
		return 0
	}
	for y := 0; y < size; y++ {
		if r.Context().Err() != nil {
			return
		}
		for x := 0; x < size; x++ {
			lx, ly := (float64(x)+.5-size/2)*radius/(size/2), (float64(y)+.5-size/2)*radius/(size/2)
			if math.Hypot(lx, ly) > radius {
				continue
			}
			pt := localInverse(center, [2]float64{lx, ly}, planetRadius)
			cellX := int(math.Round((pt[0] + math.Pi) / model.Tau * float64(s.e.Options.Columns-1)))
			cellY := int(math.Round((.5 - pt[1]/math.Pi) * float64(s.e.Options.Rows-1)))
			i := max(0, min(len(s.e.Mask)-1, cellY*s.e.Options.Columns+cellX))
			bed, temp, moist, water := get("elevation", i), get("temperature", i), get("moisture", i), get("waterDepth", i) > 0
			if sampler != nil {
				a := sampler.Sample(pt[0], pt[1])
				if s.transform != nil {
					a = s.transform(a, pt[0], pt[1])
				}
				bed, temp, moist, water = a.Bed, a.Temperature, a.Moisture, a.Water != "dry"
			}
			c := color.NRGBA{103, 135, 95, 255}
			switch {
			case water:
				c = color.NRGBA{43, 104, 133, 255}
			case temp < -8:
				c = color.NRGBA{216, 224, 217, 255}
			case bed > 2300:
				c = color.NRGBA{144, 140, 128, 255}
			case moist < .22:
				c = color.NRGBA{186, 166, 116, 255}
			}
			shade := model.Clamp(1+bed/18000, .7, 1.2)
			c.R = uint8(model.Clamp(float64(c.R)*shade, 0, 255))
			c.G = uint8(model.Clamp(float64(c.G)*shade, 0, 255))
			c.B = uint8(model.Clamp(float64(c.B)*shade, 0, 255))
			img.SetNRGBA(x, y, c)
		}
	}
	pixel := func(p [2]float64) [2]float64 {
		pt := model.Point{p[0]/float64(s.e.Options.Columns-1)*model.Tau - math.Pi, math.Pi/2 - p[1]/float64(s.e.Options.Rows-1)*math.Pi}
		q := localPoint(center, pt, planetRadius)
		return [2]float64{size/2 + q[0]/radius*(size/2), size/2 + q[1]/radius*(size/2)}
	}
	mark := func(x, y int) {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				xx, yy := x+dx, y+dy
				if xx >= 0 && xx < size && yy >= 0 && yy < size && math.Hypot(float64(xx)-size/2, float64(yy)-size/2) < size/2 {
					img.SetNRGBA(xx, yy, color.NRGBA{255, 216, 126, 255})
				}
			}
		}
	}
	for _, outline := range outlines {
		for j, a := range outline {
			b := outline[(j+1)%len(outline)]
			previous := pixel(a)
			for step := 1; step <= 16; step++ {
				t := float64(step) / 16
				dx := b[0] - a[0]
				width := float64(s.e.Options.Columns - 1)
				dx -= width * math.Floor(dx/width+.5)
				q := pixel([2]float64{a[0] + dx*t, a[1] + (b[1]-a[1])*t})
				n := max(1, int(math.Ceil(math.Hypot(q[0]-previous[0], q[1]-previous[1]))))
				for k := 0; k <= n; k++ {
					v := float64(k) / float64(n)
					mark(int(math.Round(previous[0]+(q[0]-previous[0])*v)), int(math.Round(previous[1]+(q[1]-previous[1])*v)))
				}
				previous = q
			}
		}
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = png.Encode(w, img)
}
