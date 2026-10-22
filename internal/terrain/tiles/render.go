// Package tiles renders directly sampled continuous terrain with a shared halo.
package tiles

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"kriemhild/internal/terrain/model"
	"math"
	"time"
)

type Transform func(s model.Sample, lon, lat float64) model.Sample

// Raster fixes invalidate derived disk images without changing saved terrain.
const RenderingVersion = "raster-2"

func Valid(z, x, y, size int) bool {
	return z >= 0 && z <= 18 && x >= 0 && x < (2<<z) && y >= 0 && y < (1<<z) && (size == 256 || size == 512)
}
func Coordinate(z, x, y, size, i, j int) (float64, float64) {
	return -math.Pi + model.Tau*(float64(x*size+i)+.5)/float64((2<<z)*size), math.Pi/2 - math.Pi*(float64(y*size+j)+.5)/float64((1<<z)*size)
}

// Canonicalize the integer sample address before floating-point conversion.
// Wrapping radians after conversion can produce different values for the same
// halo pixel at +/-pi, enough to cross a discrete biome threshold. Coordinate
// stays unwrapped for geographic bounding boxes and authored-operation queries.
func sampleCoordinate(z, x, y, size, i, j int) (float64, float64) {
	width := (2 << z) * size
	pixel := ((x*size+i)%width + width) % width
	return Coordinate(z, 0, y, size, pixel, j)
}
func Render(ctx context.Context, p *model.Planet, z, x, y, size int, style string, transform Transform) ([]byte, error) {
	return RenderMeasured(ctx, p, z, x, y, size, style, transform, nil)
}

// Optional timings include cache wait in sampling, and keep raster/PNG costs separate.
type RenderTiming struct{ SamplingMS, TransformMS, RasterMS, EncodeMS float64 }

func RenderMeasured(ctx context.Context, p *model.Planet, z, x, y, size int, style string, transform Transform, timing *RenderTiming) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !Valid(z, x, y, size) {
		return nil, fmt.Errorf("invalid tile address")
	}
	switch style {
	case "natural", "elevation", "biome", "mask", "coastline", "bathymetry", "temperature", "precipitation", "moisture", "atlas", "hillshade", "pet", "aridity", "soil", "seasonality", "rainShadow", "exposure", "slope", "contours", "watersheds", "flow-accumulation", "river-network", "wetlands", "shore-ink":
	default:
		return nil, fmt.Errorf("unknown terrain style")
	}
	stride := size + 2
	started := time.Now()
	base, err := physicalSamples(ctx, p, z, x, y, size)
	if timing != nil {
		timing.SamplingMS = float64(time.Since(started).Microseconds()) / 1000
	}
	if err != nil {
		return nil, err
	}
	samples := base
	started = time.Now()
	if transform != nil {
		samples = append([]model.Sample(nil), base...)
	}
	for j := -1; transform != nil && j <= size; j++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for i := -1; i <= size; i++ {
			lon, lat := sampleCoordinate(z, x, y, size, i, j)
			s := base[(j+1)*stride+i+1]
			if transform != nil {
				s = transform(s, lon, lat)
			}
			samples[(j+1)*stride+i+1] = s
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	if timing != nil {
		timing.TransformMS = float64(time.Since(started).Microseconds()) / 1000
	}
	started = time.Now()
	for j := 0; j < size; j++ {
		for i := 0; i < size; i++ {
			at := (j+1)*stride + i + 1
			s := samples[at]
			lon, lat := Coordinate(z, x, y, size, i, j)
			_ = lon
			metres := p.Spec.RadiusKM * 1000 * math.Pi / float64((1<<z)*size)
			dx := (samples[at+1].Bed - samples[at-1].Bed) / (metres * 2 * math.Max(.12, math.Cos(lat)))
			dy := (samples[at+stride].Bed - samples[at-stride].Bed) / (metres * 2)
			dx *= 5
			dy *= 5
			shade := model.Clamp((.72 + (-dx*.45-dy*.55)/math.Sqrt(1+dx*dx+dy*dy)), .32, 1.18)
			c := palette(s, style, shade)
			if s.Water != "ocean" {
				switch style {
				case "watersheds":
					g := p.Grid
					x := int((model.Wrap(lon)+math.Pi)/model.Tau*float64(g.Width)+.5) % g.Width
					y := int(model.Clamp((.5-lat/math.Pi)*float64(g.Height), 0, float64(g.Height-1)))
					if ids := g.Fields["basin"]; len(ids) > 0 {
						id := uint32(ids[y*g.Width+x]) * 2654435761
						c = color.NRGBA{uint8(60 + id%160), uint8(60 + (id>>8)%160), uint8(60 + (id>>16)%160), 255}
					}
				case "flow-accumulation":
					v := model.Clamp(math.Log1p(p.Grid.At("discharge", lon, lat))/12, 0, 1)
					c = color.NRGBA{uint8(220 - 200*v), uint8(220 - 100*v), uint8(190 + 60*v), 255}
				case "river-network":
					v := p.Grid.At("network", lon, lat)
					c = color.NRGBA{uint8(225 - 200*v), uint8(220 - 110*v), uint8(195 + 50*v), 255}
				case "wetlands":
					v := p.Grid.At("wetland", lon, lat)
					c = color.NRGBA{uint8(220 - 180*v), uint8(210 - 50*v), uint8(180 - 60*v), 255}
				}
			}
			if style == "pet" || style == "aridity" || style == "soil" || style == "seasonality" || style == "rainShadow" || style == "exposure" || style == "slope" {
				maximum := map[string]float64{"pet": 2200, "aridity": 2, "soil": 200, "seasonality": 1.5, "rainShadow": .3, "exposure": .3, "slope": .1}[style]
				v := model.Clamp(p.Grid.At(style, lon, lat)/maximum, 0, 1)
				c = color.NRGBA{uint8(210 - 175*v), uint8(160 - 30*v), uint8(90 + 85*v), 255}
			}
			if style == "contours" && s.Water == "dry" {
				interval := math.Max(20, math.Pow(2, math.Ceil(math.Log2(metres))))
				if math.Mod(math.Abs(s.Bed), interval) < math.Max(1, metres*.04) {
					c = color.NRGBA{70, 55, 40, 255}
				}
			}
			if p.Surface == nil && style != "coastline" && style != "mask" && style != "hillshade" && math.Abs(s.CoastDistance*1000) < metres*.8 && (s.Water == "dry" || s.Water == "ocean") {
				wet, dry := s, s
				wet.Water = "ocean"
				wet.WaterDepth = 30
				dry.Water = "dry"
				dry.WaterDepth = 0
				a, b := palette(wet, style, 1), palette(dry, style, shade)
				f := model.Clamp(.5+s.CoastDistance*1000/(metres*1.6), 0, 1)
				c = color.NRGBA{uint8(float64(a.R)*(1-f) + float64(b.R)*f), uint8(float64(a.G)*(1-f) + float64(b.G)*f), uint8(float64(a.B)*(1-f) + float64(b.B)*f), 255}
			}
			if style == "coastline" || style == "shore-ink" {
				c = color.NRGBA{16, 28, 43, 255}
				if style == "shore-ink" {
					c = color.NRGBA{}
				}
				wet := s.Water != "dry"
				for _, offset := range []int{-1, 1, -stride, stride} {
					if (samples[at+offset].Water != "dry") != wet {
						c = color.NRGBA{216, 232, 218, 255}
						if style == "shore-ink" {
							c = color.NRGBA{80, 61, 42, 255}
						}
						break
					}
				}
			}
			img.SetNRGBA(i, j, c)
		}
	}
	var b bytes.Buffer
	if timing != nil {
		timing.RasterMS = float64(time.Since(started).Microseconds()) / 1000
	}
	started = time.Now()
	err = png.Encode(&b, img)
	if timing != nil {
		timing.EncodeMS = float64(time.Since(started).Microseconds()) / 1000
	}
	return b.Bytes(), err
}
func palette(s model.Sample, style string, shade float64) color.NRGBA {
	mix := func(a, b [3]float64, t float64) [3]float64 {
		t = model.Clamp(t, 0, 1)
		return [3]float64{a[0]*(1-t) + b[0]*t, a[1]*(1-t) + b[1]*t, a[2]*(1-t) + b[2]*t}
	}
	var c [3]float64
	if s.Water != "dry" {
		c = mix([3]float64{64, 146, 162}, [3]float64{13, 39, 69}, s.WaterDepth/3700)
		shade = .95
	} else {
		dry := [3]float64{192, 160, 106}
		green := [3]float64{55, 100, 63}
		c = mix(dry, green, s.Moisture)
		c = mix(c, [3]float64{123, 128, 109}, model.Clamp((s.Bed-1700)/3000, 0, .8))
		c = mix(c, [3]float64{228, 235, 230}, model.Clamp((-s.Temperature+3)/14, 0, 1))
	}
	switch style {
	case "mask":
		if s.Water == "dry" {
			c = [3]float64{230, 235, 224}
		} else {
			c = [3]float64{16, 28, 43}
		}
		shade = 1
	case "elevation":
		if s.Water == "dry" {
			c = mix([3]float64{76, 127, 87}, [3]float64{217, 199, 159}, s.Bed/3000)
			c = mix(c, [3]float64{245, 244, 236}, (s.Bed-3200)/2500)
		}
	case "bathymetry":
		if s.Water == "dry" {
			c = [3]float64{170, 166, 143}
		}
	case "temperature":
		c = mix([3]float64{61, 102, 177}, [3]float64{226, 91, 49}, (s.Temperature+25)/60)
		shade = 1
	case "precipitation":
		c = mix([3]float64{211, 179, 112}, [3]float64{30, 104, 137}, s.Precipitation/2400)
		shade = 1
	case "moisture":
		c = mix([3]float64{192, 139, 79}, [3]float64{48, 130, 105}, s.Moisture)
		shade = 1
	case "biome":
		if rgb, ok := BiomeColors[s.Biome]; ok {
			c = [3]float64{float64(rgb[0]), float64(rgb[1]), float64(rgb[2])}
		} else {
			c = [3]float64{255, 0, 255}
		}
		shade = 1
	case "hillshade":
		c = [3]float64{255, 255, 255}
	case "natural":
		if s.Water == "dry" {
			if rgb, ok := BiomeColors[s.Biome]; ok {
				c = mix(c, [3]float64{float64(rgb[0]), float64(rgb[1]), float64(rgb[2])}, .45)
			}
		}
		shade = 1
	case "atlas":
		if s.Water == "dry" {
			c = mix([3]float64{220, 206, 165}, [3]float64{139, 155, 113}, s.Moisture*.7)
		} else {
			c = mix([3]float64{158, 190, 188}, [3]float64{103, 143, 155}, s.WaterDepth/4000)
		}
		shade = .8 + shade*.2
	}
	return color.NRGBA{uint8(model.Clamp(c[0]*shade, 0, 255)), uint8(model.Clamp(c[1]*shade, 0, 255)), uint8(model.Clamp(c[2]*shade, 0, 255)), 255}
}
