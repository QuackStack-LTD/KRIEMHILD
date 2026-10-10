package terrain

import "math"

// ChannelSample describes the same cross-section for generated and authored rivers.
type ChannelSample struct {
	Valid                                                      bool
	Distance, Width, Valley, Level, Discharge, Grade, T, Depth float64
}

func SampleChannel(f DetailFeature, x, y float64) ChannelSample {
	best := ChannelSample{Distance: math.Inf(1)}
	for k := 1; k < len(f.Path); k++ {
		a, b := f.Path[k-1], f.Path[k]
		dx, dy := b[0]-a[0], b[1]-a[1]
		if dx*dx+dy*dy == 0 {
			continue
		}
		t := clamp(((x-a[0])*dx+(y-a[1])*dy)/(dx*dx+dy*dy), 0, 1)
		d := math.Hypot(x-a[0]-t*dx, y-a[1]-t*dy)
		width := f.Width
		if len(f.Widths) == len(f.Path) {
			width = f.Widths[k-1]*(1-t) + f.Widths[k]*t
		}
		valley := width*(2.5+3/(1+f.Grade/100)) + .04
		if !best.Valid || d/valley < best.Distance/best.Valley {
			best = ChannelSample{true, d, width, valley, a[2]*(1-t) + b[2]*t, f.Discharge, f.Grade, (float64(k-1) + t) / float64(len(f.Path)-1), f.Depth}
		}
	}
	return best
}

func ChannelBed(r ChannelSample) (target, influence float64) {
	incision := math.Min(110, 7+6*math.Sqrt(r.Discharge)) * (1 + math.Min(1, r.Grade/350))
	if r.Depth > 0 {
		incision = math.Min(incision, r.Depth)
	}
	return r.Level + 18*math.Pow(r.Distance/r.Valley, 2) - incision*math.Exp(-math.Pow(r.Distance/(r.Width*.6+.012), 2)), math.Exp(-math.Pow(r.Distance/r.Valley, 2) * 2)
}
