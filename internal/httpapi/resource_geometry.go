package httpapi

import (
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
)

func geoDistance(a, b model.Point, radius float64) float64 {
	h := math.Pow(math.Sin((b[1]-a[1])/2), 2) + math.Cos(a[1])*math.Cos(b[1])*math.Pow(math.Sin(model.Wrap(b[0]-a[0])/2), 2)
	return 2 * radius * math.Asin(math.Sqrt(model.Clamp(h, 0, 1)))
}
func geoBearing(a, b model.Point) float64 {
	d := model.Wrap(b[0] - a[0])
	return math.Atan2(math.Sin(d)*math.Cos(b[1]), math.Cos(a[1])*math.Sin(b[1])-math.Sin(a[1])*math.Cos(b[1])*math.Cos(d))
}
func geoDestination(a model.Point, distance, bearing, radius float64) model.Point {
	d := distance / radius
	lat := math.Asin(model.Clamp(math.Sin(a[1])*math.Cos(d)+math.Cos(a[1])*math.Sin(d)*math.Cos(bearing), -1, 1))
	return model.Point{model.Wrap(a[0] + math.Atan2(math.Sin(bearing)*math.Sin(d)*math.Cos(a[1]), math.Cos(d)-math.Sin(a[1])*math.Sin(lat))), lat}
}
func localPoint(a, b model.Point, radius float64) [2]float64 {
	d, t := geoDistance(a, b, radius), geoBearing(a, b)
	return [2]float64{d * math.Sin(t), -d * math.Cos(t)}
}
func localInverse(a model.Point, p [2]float64, radius float64) model.Point {
	return geoDestination(a, math.Hypot(p[0], p[1]), math.Atan2(p[0], -p[1]), radius)
}

type enclosingDisk struct{ x, y, r float64 }

func (c enclosingDisk) contains(p [2]float64) bool { return math.Hypot(p[0]-c.x, p[1]-c.y) <= c.r+1e-8 }
func pairDisk(a, b [2]float64) enclosingDisk {
	return enclosingDisk{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, math.Hypot(a[0]-b[0], a[1]-b[1]) / 2}
}
func tripleDisk(a, b, c [2]float64) enclosingDisk {
	bx, by, cx, cy := b[0]-a[0], b[1]-a[1], c[0]-a[0], c[1]-a[1]
	d := 2 * (bx*cy - by*cx)
	if math.Abs(d) < 1e-12 {
		best := pairDisk(a, b)
		for _, v := range []enclosingDisk{pairDisk(a, c), pairDisk(b, c)} {
			if v.r > best.r {
				best = v
			}
		}
		return best
	}
	u, v := bx*bx+by*by, cx*cx+cy*cy
	x, y := (cy*u-by*v)/d, (bx*v-cx*u)/d
	return enclosingDisk{a[0] + x, a[1] + y, math.Hypot(x, y)}
}

// Deterministic shuffled smallest enclosing disk in a local azimuthal projection.
// Rechecking the geographic outline after projection ensures the preview fits
// irregular and seam-crossing settlements, rather than fitting a bounding box.
func settlementCircle(points [][2]float64, w, h int, radius float64) (model.Point, float64, error) {
	if len(points) == 0 || len(points) > 16384 || w < 2 || h < 2 {
		return model.Point{}, 0, fmt.Errorf("settlement has no supported terrain footprint")
	}
	outline := make([]model.Point, 0, len(points)*8)
	toGeo := func(p [2]float64) model.Point {
		return model.Point{p[0]/float64(w-1)*model.Tau - math.Pi, math.Pi/2 - p[1]/float64(h-1)*math.Pi}
	}
	origin := toGeo(points[0])
	for i, p := range points {
		a, b := toGeo(p), toGeo(points[(i+1)%len(points)])
		for j := 0; j < 8; j++ {
			t := float64(j) / 8
			outline = append(outline, model.Point{model.Wrap(a[0] + model.Wrap(b[0]-a[0])*t), a[1] + (b[1]-a[1])*t})
		}
	}
	local := make([][2]float64, len(outline))
	for i, p := range outline {
		local[i] = localPoint(origin, p, radius)
	}
	seed := uint64(0x54ed92)
	for i := len(local) - 1; i > 0; i-- {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		j := int(seed % uint64(i+1))
		local[i], local[j] = local[j], local[i]
	}
	disk := enclosingDisk{r: -1}
	for i, a := range local {
		if disk.contains(a) {
			continue
		}
		disk = enclosingDisk{a[0], a[1], 0}
		for j := 0; j < i; j++ {
			b := local[j]
			if disk.contains(b) {
				continue
			}
			disk = pairDisk(a, b)
			for k := 0; k < j; k++ {
				c := local[k]
				if !disk.contains(c) {
					disk = tripleDisk(a, b, c)
				}
			}
		}
	}
	center := localInverse(origin, [2]float64{disk.x, disk.y}, radius)
	fit := 0.
	for _, p := range outline {
		fit = math.Max(fit, geoDistance(center, p, radius))
	}
	// A small stroke margin prevents drawing at the clipping boundary. A point
	// settlement has a 50 m minimum rather than a zero-sized image.
	return center, math.Max(.05, fit*1.025), nil
}

// Distance to a minor great-circle segment, with endpoint clamping.
func geoSegmentDistance(center, a, b model.Point, radius float64) float64 {
	length := geoDistance(a, b, radius) / radius
	if length < 1e-12 {
		return geoDistance(center, a, radius)
	}
	d, angle := geoDistance(a, center, radius)/radius, geoBearing(a, center)-geoBearing(a, b)
	along := math.Atan2(math.Sin(d)*math.Cos(angle), math.Cos(d))
	if along < 0 || along > length {
		return math.Min(geoDistance(center, a, radius), geoDistance(center, b, radius))
	}
	return math.Abs(math.Asin(model.Clamp(math.Sin(d)*math.Sin(angle), -1, 1))) * radius
}

// Clip a saved river segment to the observed cap before interpolating flow.
// Endpoint flow outside a small circle must not be reported as local supply.
func geoSegmentInterval(center, a, b model.Point, capRadius, planetRadius float64) (float64, float64) {
	length := geoDistance(a, b, planetRadius)
	if length < 1e-10 {
		return 0, 1
	}
	bearing := geoBearing(a, b)
	inside := func(t float64) bool {
		return geoDistance(center, geoDestination(a, t*length, bearing, planetRadius), planetRadius) <= capRadius+1e-8
	}
	d, angle := geoDistance(a, center, planetRadius)/planetRadius, geoBearing(a, center)-bearing
	nearest := model.Clamp(math.Atan2(math.Sin(d)*math.Cos(angle), math.Cos(d))*planetRadius/length, 0, 1)
	lo, hi := 0., 1.
	if !inside(0) {
		outside, in := 0., nearest
		for i := 0; i < 36; i++ {
			m := (outside + in) / 2
			if inside(m) {
				in = m
			} else {
				outside = m
			}
		}
		lo = in
	}
	if !inside(1) {
		in, outside := nearest, 1.
		for i := 0; i < 36; i++ {
			m := (in + outside) / 2
			if inside(m) {
				in = m
			} else {
				outside = m
			}
		}
		hi = in
	}
	return lo, hi
}
