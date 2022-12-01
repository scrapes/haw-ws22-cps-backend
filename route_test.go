package main

import (
	"fmt"
	"math"
	"testing"
)

func TestRoute_Step(t *testing.T) {

	speed := float64(1)
	r := NewRoute(speed, []Coordinate{
		{53.56806700639993, 10.005957405703246},
		{53.55510486980884, 9.995245842723662},
		{53.560388265938016, 9.982796162305084},
		{53.54582728707385, 9.966610988816763},
		{53.541331851157324, 9.984566308969676},
		{53.85735535670074, 8.696964423975643},
	})

	expectedSteps := int(math.Round(r.GetDistance()/speed)) + 1

	for i := 0; i <= expectedSteps; i++ {
		r.Step(0)
	}
	lastDistance := r.Loc.DistanceTo(r.Route[r.RouteIndex])
	if lastDistance > 0.01 {
		t.Error("Last Distance greater than 1cm : ", lastDistance)
	}

	fmt.Println("Done in Steps: ", expectedSteps)
	fmt.Println("Distance is: ", math.Round(r.GetDistance())/1000, " KM")
}
