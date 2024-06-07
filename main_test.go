package main

import (
	"github.com/scrapes/haw-ws22-cps-crude-go-actors/com"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	cl := com.NewMqttClient("tcp://localhost:1883", true, 2)
	err := cl.ConnectSync()
	if err != nil {
		return
	}
	sim := CreateSimulation(cl)
	sim.Start()

	for true {
		time.Sleep(time.Second * 100)
	}
}
