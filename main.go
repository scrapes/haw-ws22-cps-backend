package main

import (
	"gitlab.com/anwski/crude-go-actors/com"
	"time"
)

func main() {
	client := com.NewMqttClient("wss://mqtt.cps.datenspieker.de/mqtt", false, 2)
	err := client.ConnectSync()
	if err != nil {
		return
	}

	sim := CreateSimulation(client)
	sim.Start()

	for true {
		time.Sleep(time.Second * 1)
	}
}
