package resourcesurvey

import (
	"fmt"
	"math"
)

// Six cube faces avoid longitude-grid singularities at poles and the date
// line. Exact spherical cell areas supply the weights for aggregate statistics.
type Cell struct{ Level, Face, X, Y int }

func (c Cell) Key() string { return fmt.Sprintf("%d/%d/%d/%d", c.Level, c.Face, c.X, c.Y) }
func cubeVector(face int, u, v float64) [3]float64 {
	switch face {
	case 0:
		return [3]float64{1, u, v}
	case 1:
		return [3]float64{-1, u, v}
	case 2:
		return [3]float64{u, 1, v}
	case 3:
		return [3]float64{u, -1, v}
	case 4:
		return [3]float64{u, v, 1}
	default:
		return [3]float64{u, v, -1}
	}
}
func cubePoint(face int, u, v float64) (float64, float64) {
	p := cubeVector(face, u, v)
	return math.Atan2(p[1], p[0]) * 180 / math.Pi, math.Atan2(p[2], math.Hypot(p[0], p[1])) * 180 / math.Pi
}
func At(level int, lon, lat float64) Cell {
	lon *= math.Pi / 180
	lat *= math.Pi / 180
	x, y, z := math.Cos(lat)*math.Cos(lon), math.Cos(lat)*math.Sin(lon), math.Sin(lat)
	face := 0
	u, v := 0., 0.
	a, b, c := math.Abs(x), math.Abs(y), math.Abs(z)
	if a >= b && a >= c {
		if x < 0 {
			face = 1
		}
		u, v = y/a, z/a
	} else if b >= c {
		face = 2
		if y < 0 {
			face = 3
		}
		u, v = x/b, z/b
	} else {
		face = 4
		if z < 0 {
			face = 5
		}
		u, v = x/c, y/c
	}
	n := 1 << level
	return Cell{level, face, max(0, min(n-1, int((u+1)/2*float64(n)))), max(0, min(n-1, int((v+1)/2*float64(n))))}
}
func (c Cell) Center() (float64, float64) {
	n := math.Exp2(float64(c.Level))
	return cubePoint(c.Face, (float64(c.X)+.5)/n*2-1, (float64(c.Y)+.5)/n*2-1)
}
func (c Cell) Area(radius float64) float64 {
	n := math.Exp2(float64(c.Level))
	u0, v0 := float64(c.X)/n*2-1, float64(c.Y)/n*2-1
	u1, v1 := u0+2/n, v0+2/n
	primitive := func(u, v float64) float64 { return math.Atan2(u*v, math.Sqrt(1+u*u+v*v)) }
	return math.Abs(primitive(u1, v1)-primitive(u0, v1)-primitive(u1, v0)+primitive(u0, v0)) * radius * radius
}
func CellArea(c Circle, level int) float64 {
	return 4 * math.Pi * c.PlanetRadiusKM * c.PlanetRadiusKM / (6 * math.Exp2(float64(2*level)))
}
func Level(c Circle) int {
	l := 13
	for l > 1 && c.Area()/CellArea(c, l) > 1400 {
		l--
	}
	for l < 22 && c.Area()/CellArea(c, l) < 24 {
		l++
	}
	return l
}

// Traverse conservative spherical cell caps. Include boundary cells, while the
// observation's circle remains the sole authority for recorded coverage.
func Cells(circle Circle, level int) []Cell {
	out := []Cell{}
	var visit func(Cell)
	visit = func(c Cell) {
		lon, lat := c.Center()
		n := math.Exp2(float64(c.Level))
		u0, v0 := float64(c.X)/n*2-1, float64(c.Y)/n*2-1
		pad := 0.
		for _, uv := range [][2]float64{{u0, v0}, {u0 + 2/n, v0}, {u0, v0 + 2/n}, {u0 + 2/n, v0 + 2/n}} {
			x, y := cubePoint(c.Face, uv[0], uv[1])
			pad = math.Max(pad, Distance(lon, lat, x, y, circle.PlanetRadiusKM))
		}
		if Distance(circle.Longitude, circle.Latitude, lon, lat, circle.PlanetRadiusKM) > circle.RadiusKM+pad+1e-7 {
			return
		}
		if c.Level == level {
			out = append(out, c)
			return
		}
		for y := 0; y < 2; y++ {
			for x := 0; x < 2; x++ {
				visit(Cell{c.Level + 1, c.Face, c.X*2 + x, c.Y*2 + y})
			}
		}
	}
	for f := 0; f < 6; f++ {
		visit(Cell{0, f, 0, 0})
	}
	return out
}
