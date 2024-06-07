package main

import (
	"fmt"
	"github.com/scrapes/haw-ws22-cps-crude-go-actors/com"
	"math"
	"testing"
)

/*
	"from":{
	  "lat": 53.569039,
	  "lon": 10.101729
	},

	"to":{
	  "lat": 53.576482,
	  "lon": 10.081263
	}
*/
func TestRoutinator_GetRoute(t *testing.T) {
	println("AA")
	cl := com.NewMqttClient("tcp://localhost:1883", false, 2)
	err := cl.ConnectSync()
	if err != nil {
		return
	}

	fmt.Println("Connected!")

	routinator := NewRoutinator("routinator", cl)

	start := Coordinate{
		Latitude:  53.569039,
		Longitude: 10.101729,
	}

	finish := Coordinate{
		Latitude:  53.576482,
		Longitude: 10.081263,
	}

	route := routinator.GetRoute(start, finish)

	distance := float64(0)
	for i := range route {
		if i == 0 {
			continue
		}
		distance += route[i-1].DistanceTo(route[i])
	}

	distance = math.Round(distance*100) / 100
	fmt.Print("Distance is: ")
	fmt.Print(distance)
	fmt.Println("Meters")
}
