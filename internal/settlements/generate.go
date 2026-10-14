// Package settlements generates geographic footprints independently of map styling.
package settlements

import (
	"fmt"
	"kriemhild/internal/terrain/detail"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
	"math/rand/v2"
)

type Spec struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Archetype string  `json:"archetype"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Seed      uint64  `json:"seed"`
}
type Feature struct {
	ID        string        `json:"id"`
	Parent    string        `json:"parent"`
	Kind      string        `json:"kind"`
	Name      string        `json:"name"`
	Geometry  string        `json:"geometry"`
	Points    []model.Point `json:"points"`
	MinDetail float64       `json:"minDetail"`
}
type Preview struct {
	Score        float64   `json:"score"`
	Warnings     []string  `json:"warnings"`
	Features     []Feature `json:"features"`
	RadiusMetres float64   `json:"radiusMetres"`
}

func Generate(p *model.Planet, s Spec) (Preview, error) {
	out := Preview{Warnings: []string{}, Features: []Feature{}}
	if len(s.Name) < 1 || len(s.Name) > 160 || !model.Finite(s.Longitude) || !model.Finite(s.Latitude) || math.Abs(s.Latitude) > 80 {
		return out, fmt.Errorf("provide a name and a site between 80°S and 80°N")
	}
	radius := map[string]float64{"village": 350, "town": 1000, "city": 2400, "capital": 4000}[s.Kind]
	if radius == 0 {
		return out, fmt.Errorf("choose village, town, city or capital")
	}
	switch s.Archetype {
	case "organic", "market", "fortified", "port", "planned":
	default:
		return out, fmt.Errorf("unknown settlement archetype")
	}
	lon, lat := s.Longitude*math.Pi/180, s.Latitude*math.Pi/180
	a := detail.Sample(p, lon, lat)
	if a.Water != "dry" {
		return out, fmt.Errorf("the site is submerged; choose a dry location")
	}
	r := radius / (p.Spec.RadiusKM * 1000)
	toWorld := func(x, y float64) model.Point { return model.Point{model.Wrap(lon + x/math.Cos(lat)), lat + y} }
	maxSlope := 0.
	for _, v := range [][2]float64{{r, 0}, {-r, 0}, {0, r}, {0, -r}} {
		pt := toWorld(v[0], v[1])
		b := detail.Sample(p, pt[0], pt[1])
		maxSlope = math.Max(maxSlope, math.Abs(b.Bed-a.Bed)/radius)
	}
	out.Score = model.Clamp(1-maxSlope*4-math.Max(0, .4-a.Moisture), 0, 1)
	if maxSlope > .3 {
		return out, fmt.Errorf("site is too steep for this settlement size")
	}
	if maxSlope > .1 {
		out.Warnings = append(out.Warnings, "Steep approaches; inspect street gradients before accepting.")
	}
	if a.Moisture < .3 {
		out.Warnings = append(out.Warnings, "Dry site: an authored water supply may be needed.")
	}
	if s.Archetype == "port" && a.CoastDistance > radius/1000*3 {
		return out, fmt.Errorf("port sites must be within three settlement radii of the coast")
	}
	random := rand.New(rand.NewPCG(s.Seed, landmass.Hash(s.Seed+1)))
	add := func(id, parent, kind, name, geometry string, points []model.Point, lod float64) {
		out.Features = append(out.Features, Feature{id, parent, kind, name, geometry, points, lod})
	}
	footprint := []model.Point{}
	for j := 0; j < 18; j++ {
		angle := model.Tau * float64(j) / 18
		d := r * (.85 + random.Float64()*.25)
		footprint = append(footprint, toWorld(math.Cos(angle)*d, math.Sin(angle)*d))
	}
	add("settlement", "", s.Kind, s.Name, "polygon", footprint, 0)
	roads := 5
	if s.Kind == "city" || s.Kind == "capital" {
		roads = 8
	}
	for j := 0; j < roads; j++ {
		angle := model.Tau*float64(j)/float64(roads) + (random.Float64()-.5)*.25
		path := []model.Point{toWorld(0, 0)}
		for k := 1; k <= 6; k++ {
			d := r * float64(k) / 6
			bend := math.Sin(float64(k)*.65) * .09
			if s.Archetype == "planned" {
				bend = 0
			}
			path = append(path, toWorld(math.Cos(angle+bend)*d, math.Sin(angle+bend)*d))
		}
		passable := true
		previous := a.Bed
		for _, pt := range path[1:] {
			sample := detail.Sample(p, pt[0], pt[1])
			if sample.Water != "dry" || math.Abs(sample.Bed-previous)/(radius/6) > .2 {
				passable = false
				break
			}
			previous = sample.Bed
		}
		if !passable {
			out.Warnings = append(out.Warnings, fmt.Sprintf("Approach %d omitted because of water or excessive gradient.", j+1))
			continue
		}
		add(fmt.Sprintf("street-%d", j), "settlement", "road", fmt.Sprintf("%s street %d", s.Name, j+1), "line", path, 3)
		district := []model.Point{toWorld(0, 0), toWorld(math.Cos(angle-.3)*r*.75, math.Sin(angle-.3)*r*.75), toWorld(math.Cos(angle+.3)*r*.75, math.Sin(angle+.3)*r*.75)}
		add(fmt.Sprintf("district-%d", j), "settlement", "district", fmt.Sprintf("District %d", j+1), "polygon", district, 3)
		for k := 1; k <= 10; k++ {
			d := r * (.12 + float64(k)*.066)
			for side := -1; side <= 1; side += 2 {
				bend := math.Sin(d/r*6*.65) * .09
				if s.Archetype == "planned" {
					bend = 0
				}
				a := angle + bend
				offset := r * (.035 + random.Float64()*.012) * float64(side)
				cx, cy := math.Cos(a)*d-math.Sin(a)*offset, math.Sin(a)*d+math.Cos(a)*offset
				sx, sy := r*(.012+random.Float64()*.008), r*(.015+random.Float64()*.01)
				ps := []model.Point{}
				dry := true
				for _, v := range [][2]float64{{-sx, -sy}, {sx, -sy}, {sx, sy}, {-sx, sy}} {
					pt := toWorld(cx+v[0]*math.Cos(a)-v[1]*math.Sin(a), cy+v[0]*math.Sin(a)+v[1]*math.Cos(a))
					if detail.Sample(p, pt[0], pt[1]).Water != "dry" {
						dry = false
					}
					ps = append(ps, pt)
				}
				if dry {
					add(fmt.Sprintf("building-%d-%d-%d", j, k, side+1), fmt.Sprintf("district-%d", j), "house", "", "polygon", ps, 6)
				}
			}
		}
	}
	if s.Archetype == "fortified" {
		add("wall", "settlement", "fortification", s.Name+" walls", "polygon", footprint, 4)
	}
	if s.Archetype == "market" {
		square := []model.Point{toWorld(-r*.08, -r*.08), toWorld(r*.08, -r*.08), toWorld(r*.08, r*.08), toWorld(-r*.08, r*.08)}
		add("market", "settlement", "district", s.Name+" market", "polygon", square, 4)
	}
	out.RadiusMetres = radius
	return out, nil
}
