package generate

import (
	"fmt"
	"kriemhild/internal/terrain/model"
	"math"
)

// ClimateMeasurements is read-only and may also measure historical versions.
// No distribution measurement is an input to the climate solver.
func ClimateMeasurements(p *model.Planet) map[string]model.ClimateBandDiagnostics {
	out := map[string]model.ClimateBandDiagnostics{}
	g := p.Grid
	for i, bed := range g.Fields["bed"] {
		if !p.Emerged(i) {
			continue
		}
		pt := g.Point(i%g.Width, i/g.Width)
		area := model.Tau / float64(g.Width) * math.Pi / float64(g.Height) * math.Cos(pt[1]) * p.Spec.RadiusKM * p.Spec.RadiusKM
		c := ClimateRegimeAt(p, pt[0], pt[1], bed)
		if p.Spec.ClimateVersion < 4 {
			for m := 0; m < 12; m++ {
				c.WaterDeficit += math.Max(0, g.Fields[fmt.Sprintf("pet-%02d", m)][i]-g.Fields[rainfallMonths[m]][i])
				if g.Fields[temperatureMonths[m]][i] >= 10 {
					c.GrowingMonths++
				}
			}
		}
		wet := g.Fields["lake"][i] > 0 && g.Fields["lake"][i] > bed
		biome := "water"
		if !wet {
			biome = vegetationAt(p, c, pt[0], pt[1], bed)
			if p.Spec.ClimateVersion == 0 || p.Spec.Version != model.Version {
				biome = BiomeAt(p, pt[0], pt[1], bed, false)
			}
		}
		latitude := int(math.Abs(pt[1])*180/math.Pi) / 15 * 15
		hemisphere := "north"
		if pt[1] < 0 {
			hemisphere = "south"
		}
		coast := "interior"
		if g.Fields["coast"][i] < 250 {
			coast = "coastal"
		}
		elevation := "highland"
		if bed < 500 {
			elevation = "lowland"
		}
		exposure := "neutral"
		if g.At("exposure", pt[0], pt[1]) > .1 {
			exposure = "windward"
		} else if g.At("rainShadow", pt[0], pt[1]) > .1 {
			exposure = "leeward"
		}
		band := fmt.Sprintf("%02d-%02d", latitude, latitude+15)
		for _, key := range []string{"all", "latitude/" + band, "hemisphere/" + hemisphere + "/" + band, "latitudeCoast/" + band + "/" + coast, "elevation/" + elevation, "exposure/" + exposure} {
			d := out[key]
			if d.Biomes == nil {
				d.Biomes = map[string]float64{}
				d.Thermal = map[string]float64{}
			}
			d.LandAreaKM2 += area
			d.Temperature += g.Fields["temperature"][i] * area
			d.Precipitation += g.Fields["precipitation"][i] * area
			d.PET += g.Fields["pet"][i] * area
			d.Aridity += c.Aridity * area
			d.DryMonths += c.DryMonths * area
			d.WaterDeficit += c.WaterDeficit * area
			d.GrowingMonths += c.GrowingMonths * area
			d.TemperatureRange += (c.Warmest - c.Coldest) * area
			if !wet {
				d.DryLandAreaKM2 += area
				d.Biomes[biome] += area
				d.Thermal[c.Thermal] += area
				if c.Perhumid {
					d.Perhumid += area
				}
			}
			out[key] = d
		}
	}
	for key, d := range out {
		d.PrecipitationKM3 = d.Precipitation / 1e6
		d.IntegratedAridity = d.Precipitation / math.Max(1, d.PET)
		d.Temperature /= d.LandAreaKM2
		d.Precipitation /= d.LandAreaKM2
		d.PET /= d.LandAreaKM2
		d.Aridity /= d.LandAreaKM2
		d.DryMonths /= d.LandAreaKM2
		d.WaterDeficit /= d.LandAreaKM2
		d.GrowingMonths /= d.LandAreaKM2
		d.TemperatureRange /= d.LandAreaKM2
		for k := range d.Biomes {
			d.Biomes[k] /= math.Max(1, d.DryLandAreaKM2)
		}
		for k := range d.Thermal {
			d.Thermal[k] /= math.Max(1, d.DryLandAreaKM2)
		}
		d.Perhumid /= math.Max(1, d.DryLandAreaKM2)
		out[key] = d
	}
	return out
}
