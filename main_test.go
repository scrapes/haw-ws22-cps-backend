package main

import (
	"gitlab.com/anwski/crude-go-actors/com"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	cl := com.NewMqttClient("mqtt://127.0.0.1:1883", true, 2)
	err := cl.ConnectSync()
	if err != nil {
		return
	}
	sim := CreateSimulation(cl)
	sim.Start()

	for true {
		time.Sleep(time.Second)
	}
}