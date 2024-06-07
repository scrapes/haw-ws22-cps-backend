package main

import (
	"github.com/google/uuid"
	"github.com/scrapes/haw-ws22-cps-crude-go-actors/com"
	"go.uber.org/zap"
	"reflect"
	"time"
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
	subid := uuid.New()

	Logger.Info("Calculating route", zap.Float64("from lat", from.Latitude), zap.Float64("from long", from.Longitude), zap.Float64("to lat", to.Latitude), zap.Float64("to long", to.Longitude), zap.String("subid", subid.String()))
	ch := make(chan ResponseMessage)
	defer close(ch)

	closed := false
	defer func() { closed = true }()

	req := RequestMessage{
		UUID: uuid.NewString(),
		From: from,
		To:   to,
	}

	Logger.Info("Subscribing", zap.String("subid", req.UUID))
	err := route.mqttClient.SubscribeJson(route.getResponseTopic(req.UUID), reflect.TypeOf(ResponseMessage{}), com.SubCallback{ID: subid, Callback: func(msg reflect.Value) {
		Logger.Info("Got Route", zap.String("subid", req.UUID))
		reflect.ValueOf(func(message *ResponseMessage) {
			if !closed {
				ch <- *message
			}
		}).Call([]reflect.Value{msg})

		route.mqttClient.Unsubscribe(route.getResponseTopic(req.UUID), subid)
	}})

	if err != nil {
		Logger.Error("Error subscribing", zap.String("subid", req.UUID), zap.Error(err))
		return nil
	}

	success := 20
	for success > 0 {
		Logger.Info("Waiting for response", zap.String("subid", req.UUID))
		err2 := route.mqttClient.PublishJson(route.getRequestTopic(), req)
		if err2 != nil {
			return nil
		}
		select {
		case msg := <-ch:
			return msg.Route
		case <-time.After(5 * time.Second):
			success--
			Logger.Error("Timed out waiting for response", zap.String("subid", req.UUID))
		}
	}
	return nil
}
