package generate

import (
	"context"
	"fmt"
	"kriemhild/internal/terrain/landmass"
	"kriemhild/internal/terrain/model"
	"math"
)

// A reduced moisture budget: ocean replenishment, conservative upwind transport,
// bounded orographic condensation, storm rainfall, and monthly PET. It is not a GCM.
func climate(ctx context.Context, p *model.Planet) error {
	g := &p.Grid
	n := g.Width * g.Height
	for month := 0; month < 12; month++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		ts, rs, es := make([]float64, n), make([]float64, n), make([]float64, n)
		vapor, next := make([]float64, n), make([]float64, n)
		decl := p.Spec.Tilt * math.Pi / 180 * math.Sin(model.Tau*(float64(month)-2)/12)
		for i := range ts {
			pt := g.Point(i%g.Width, i/g.Width)
			lat := pt[1]
			coast := g.Fields["coast"][i]
			bed := g.Fields["bed"][i]
			continental := 1 - math.Exp(-math.Max(0, coast)/700)
			ts[i] = 29 - 48*math.Pow(math.Sin(lat), 2) - math.Max(0, bed)*.0065 + decl*math.Sin(lat)*(12+26*continental)
			g.Fields["windX"][i] = math.Tanh((math.Abs(lat-decl*.4)-.53)*8) * math.Cos(lat)
			g.Fields["windY"][i] = .25*math.Sin(3*(lat-decl)) + .12*landmass.Noise(p.Seed+412, pt[0], lat, 4)
			vapor[i] = .5
		}
		for pass := 0; pass < 64; pass++ {
			if pass%8 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			for i := range vapor {
				x, y := i%g.Width, i/g.Width
				wx := g.Fields["windX"][i]
				wy := g.Fields["windY"][i]
				dx := 1
				if wx > 0 {
					dx = -1
				}
				dy := 1
				if wy > 0 {
					dy = -1
				}
				ix := y*g.Width + (x+dx+g.Width)%g.Width
				iy := max(0, min(g.Height-1, y+dy))*g.Width + x
				incoming := vapor[ix]*.7 + vapor[iy]*.15 + vapor[i]*.15
				if g.Fields["bed"][i] <= 0 {
					next[i] = .96
					continue
				}
				uplift := math.Max(0, g.Fields["bed"][i]-g.Fields["bed"][ix])
				loss := model.Clamp(.025+uplift/12000, .025, .32)
				next[i] = incoming * (1 - loss)
			}
			vapor, next = next, vapor
		}
		for i := range ts {
			pt := g.Point(i%g.Width, i/g.Width)
			lat := pt[1]
			coast := g.Fields["coast"][i]
			convection := math.Exp(-math.Pow((lat-decl)/.27, 2))
			storm := math.Exp(-math.Pow((math.Abs(lat)-.85)/.3, 2))
			monsoon := .2 + .8*math.Max(0, math.Sin(pt[0]*2+decl*3))*math.Exp(-math.Pow((math.Abs(lat)-.4)/.35, 2))
			supply := vapor[i] * (.42 + .9*convection + .7*storm + .4*monsoon)
			rs[i] = math.Max(0, (150*supply+12*math.Exp(-math.Max(0, coast)/250))*p.Spec.Rainfall)
			es[i] = math.Max(4, 38+ts[i]*3.8) * (1 + .15*math.Cos(lat-decl))
			g.Fields["temperature"][i] += ts[i] / 12
			g.Fields["precipitation"][i] += rs[i]
			g.Fields["pet"][i] += es[i]
		}
		g.Fields[fmt.Sprintf("temperature-%02d", month)] = ts
		g.Fields[fmt.Sprintf("rainfall-%02d", month)] = rs
		g.Fields[fmt.Sprintf("pet-%02d", month)] = es
	}
	for i := range g.Fields["moisture"] {
		g.Fields["moisture"][i] = model.Clamp(g.Fields["precipitation"][i]/math.Max(100, g.Fields["pet"][i])*0.6, 0, 1)
	}
	return nil
}
