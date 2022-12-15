package main

import "github.com/google/uuid"

// ------- Message types --------

type MapeDistributionRequest struct {
	From     uuid.UUID
	To       uuid.UUID
	Capacity int
}

type MapeDistributionAnswer struct {
	Capacity int
}

// ------- Message types --------

type MapeState struct {
}
