package main

import (
	"gitlab.com/anwski/crude-go-actors/com"
	"os"
	"time"
)

func main() {
	mqttHost := os.Getenv("MQTT_HOST")
	if mqttHost == "" {
		Logger.Error("MQTT_HOST is not set")
		return
	}

	client := com.NewMqttClient(mqttHost, false, 2)
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
