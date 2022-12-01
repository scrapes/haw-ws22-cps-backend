package main

import "math"

type Coordinate struct {
	Latitude  float64
	Longitude float64
}

func ToRadians(x float64) float64 {
	return x * math.Pi / 180
}

// DistanceTo Calculate distance in Meters
func (from Coordinate) DistanceTo(to Coordinate) float64 {
	earthRadiusKm := float64(6371)

	dLat := ToRadians(to.Latitude - from.Latitude)
	dLon := ToRadians(to.Longitude - from.Longitude)

	lat1 := ToRadians(from.Latitude)
	lat2 := ToRadians(to.Latitude)

	var a = math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
	var c = 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c * 1000
}

func (from Coordinate) GetAngle(to Coordinate) float64 {
	dy := to.Latitude - from.Latitude
	dx := math.Cos(math.Pi/180*to.Latitude) * (to.Longitude - from.Longitude)
	angle := math.Atan2(dy, dx)
	return angle
}

func (from Coordinate) Move(to Coordinate, meters float64) (Coordinate, float64) {
	distance := from.DistanceTo(to)
	multiplier := meters / distance

	// use .99 to account float rounding errors
	if multiplier > 0.99 {
		return to, meters - distance
	} else {

		//get Delta
		dLat := to.Latitude - from.Latitude
		dLon := to.Longitude - from.Longitude

		//set Movement
		from.Latitude += dLat * multiplier
		from.Longitude += dLon * multiplier

		return from, 0
	}
}
