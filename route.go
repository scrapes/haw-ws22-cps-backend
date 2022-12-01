package main

type Route struct {
	// Routing data for UI
	Loc        Coordinate
	Route      []Coordinate
	RouteIndex int
	RouteAngle float64
	Speed      float64 // Speed in meter/second
}

func NewRoute(speed float64, coords []Coordinate) *Route {
	r := Route{
		Route:      coords,
		Loc:        coords[0],
		RouteIndex: 1,
		RouteAngle: 0,
		Speed:      speed,
	}
	r.RouteAngle = r.Loc.GetAngle(r.Route[0])
	return &r
}

func (r *Route) GetDistance() float64 {
	distance := float64(0)
	lastCoordinate := r.Route[0]
	for _, coordinate := range r.Route {
		distance += lastCoordinate.DistanceTo(coordinate)
		lastCoordinate = coordinate
	}
	return distance
}

func (r *Route) GetRemainingDistance() float64 {
	if r.RouteIndex > 1 {
		distance := float64(0)
		lastCoordinate := r.Loc
		for i := r.RouteIndex; i < len(r.Route); i++ {
			distance += lastCoordinate.DistanceTo(r.Route[i])
			lastCoordinate = r.Route[i]
		}
		return distance
	} else {
		return float64(0)
	}
}

func (r *Route) NextPoint() *Coordinate {
	return &r.Route[r.RouteIndex]
}

func (r *Route) Step(carry float64) {
	speed := r.Speed / CONST_speed
	if r.Route == nil || r.RouteIndex >= len(r.Route) {
		return
	}
	if carry > 0 {
		move := carry
		carry = 0
		r.Loc, carry = r.Loc.Move(r.Route[r.RouteIndex], move)
		r.RouteAngle = r.Loc.GetAngle(r.Route[r.RouteIndex])
	} else {
		r.Loc, carry = r.Loc.Move(r.Route[r.RouteIndex], speed)
	}

	if carry > 0 && r.RouteIndex < len(r.Route)-1 {
		r.RouteIndex++
		r.Step(carry)
	}
}
