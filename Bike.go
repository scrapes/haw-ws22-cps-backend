package main

import "github.com/google/uuid"

type BikeID uuid.UUID

type Bike struct {
	ID BikeID
}
