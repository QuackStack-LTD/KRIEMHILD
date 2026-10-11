package terrain

import "math"

func (e *Environment) geographicPoint(x, y float64) (latitude, longitude float64) {
	g := e.Geography
	return (g.North - (y+.5)/float64(e.Options.Rows)*(g.North-g.South)) * math.Pi / 180,
		(g.West + (x+.5)/float64(e.Options.Columns)*(g.East-g.West)) * math.Pi / 180
}

func sphericalDistance(lat, lon, otherLat, otherLon, radius float64) float64 {
	a := math.Pow(math.Sin((otherLat-lat)/2), 2) + math.Cos(lat)*math.Cos(otherLat)*math.Pow(math.Sin((otherLon-lon)/2), 2)
	return 2 * radius * math.Asin(math.Sqrt(clamp(a, 0, 1)))
}

func (e *Environment) PointDistanceKM(x, y, otherX, otherY float64) float64 {
	if e.Geography == nil {
		return 0
	}
	lat, lon := e.geographicPoint(x, y)
	otherLat, otherLon := e.geographicPoint(otherX, otherY)
	return sphericalDistance(lat, lon, otherLat, otherLon, e.Geography.Radius)
}

// Closest point on a spherical latitude/longitude cell, including the wrapped
// seam. Regions occupy cell areas; point occurrences keep their exact location.
func (e *Environment) distanceToCellKM(x, y float64, column, row int) float64 {
	lat, lon := e.geographicPoint(x, y)
	north, west := e.geographicPoint(float64(column)-.5, float64(row)-.5)
	south, east := e.geographicPoint(float64(column)+.5, float64(row)+.5)
	centre := (west + east) / 2
	lon = centre + math.Remainder(lon-centre, 2*math.Pi)
	if lon >= west && lon <= east && lat >= south && lat <= north {
		return 0
	}
	closestLon := clamp(lon, west, east)
	closestLat := clamp(math.Atan2(math.Sin(lat), math.Cos(lat)*math.Cos(lon-closestLon)), south, north)
	return math.Min(sphericalDistance(lat, lon, closestLat, closestLon, e.Geography.Radius),
		math.Min(sphericalDistance(lat, lon, south, closestLon, e.Geography.Radius), sphericalDistance(lat, lon, north, closestLon, e.Geography.Radius)))
}
