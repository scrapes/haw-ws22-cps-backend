package main

import (
	"gitlab.com/anwski/crude-go-actors/com"
	"time"
)

func main() {
	client := com.NewMqttClient("tcp://127.0.0.1:1883", false, 2)
	err := client.ConnectSync()
	if err != nil {
		return
	}

	sim := CreateSimulation(client)
	sim.Start()

	tick := 0

	for true {
		time.Sleep(time.Second * 1)
		sim.Tick(tick)
		tick++
	}
}
