package main

import (
	"testing"
)

func TestCoordinate_Move(t *testing.T) {
	A := Coordinate{
		53.56806700639993, 10.005957405703246,
	}

	B := Coordinate{
		53.55510486980884, 9.995245842723662,
	}

	divisor := 20
	dDistance := A.DistanceTo(B) / float64(divisor)
	for i := 0; i < divisor; i++ {
		A, _ = A.Move(B, dDistance)
	}

	if A.DistanceTo(B) > 0.01 {
		t.Error("Distance greater than 1 cm")
	}

}
