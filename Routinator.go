package main

import (
	"fmt"
	"github.com/google/uuid"
	"gitlab.com/anwski/crude-go-actors/com"
	"reflect"
)

type RequestMessage struct {
	UUID string
	From Coordinate
	To   Coordinate
}

type ResponseMessage struct {
	UUID  string
	Route []Coordinate
}

type Routinator struct {
	mqttClient *com.MqttClient
	prefix     string
}

func NewRoutinator(prefix string, client *com.MqttClient) *Routinator {
	return &Routinator{
		mqttClient: client,
		prefix:     prefix,
	}
}

func (route *Routinator) getRequestTopic() string {
	return route.prefix + "/request"
}

func (route *Routinator) getResponseTopic(uuid string) string {
	return route.prefix + "/response/by-uuid/" + uuid
}

func (route *Routinator) GetRoute(from Coordinate, to Coordinate) []Coordinate {
	ch := make(chan ResponseMessage)

	req := RequestMessage{
		UUID: uuid.NewString(),
		From: from,
		To:   to,
	}
	subid := uuid.New()
	err := route.mqttClient.SubscribeJson(route.getResponseTopic(req.UUID), reflect.TypeOf(ResponseMessage{}), com.SubCallback{ID: subid, Callback: func(msg reflect.Value) {
		reflect.ValueOf(func(message *ResponseMessage) {
			ch <- *message
		}).Call([]reflect.Value{msg})

		route.mqttClient.Unsubscribe(route.getResponseTopic(req.UUID), subid)
	}})

	if err != nil {
		fmt.Println(err)
		close(ch)
		return nil
	}

	err2 := route.mqttClient.PublishJson(route.getRequestTopic(), req)
	if err2 != nil {
		return nil
	}

	msg := <-ch
	close(ch)
	return msg.Route
}