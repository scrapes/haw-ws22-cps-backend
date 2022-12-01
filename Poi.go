package main

import (
	"fmt"
	"github.com/google/uuid"
	"gitlab.com/anwski/crude-go-actors/actor"
	"gitlab.com/anwski/crude-go-actors/com"
)

const (
	PoiRequestPop    string = "POI-Request-Pop"
	PoiSetAttraction string = "POI-Set-Attraction"
	PoiUpdateData    string = "PoiUpdateData"
)

type PoiID uuid.UUID
type PoiUpdate struct {
	ID         uuid.UUID
	Name       string
	Location   Coordinate
	Attraction float64
}
type Poi struct {
	ID         PoiID
	Name       string
	GeoLoc     Coordinate
	Attraction float64
	Actor      *actor.Actor
}

func (poi *Poi) GetWeight(loc Coordinate) float64 {
	distance := loc.DistanceTo(poi.GeoLoc)
	distanceWeight := float64(0)
	if distance <= CONST_poi_max_distance {
		distanceWeight = 1 - (distance / CONST_poi_max_distance) // reverse bias to get closest
	}
	return distanceWeight * poi.Attraction
}

func NewPOI(mqttclient *com.MqttClient, grp *actor.Group, Name string, GeoLoc Coordinate, Attraction float64) *Poi {
	p := Poi{
		ID:         PoiID(uuid.New()),
		Name:       Name,
		GeoLoc:     GeoLoc,
		Attraction: Attraction,
		Actor:      actor.NewActor(mqttclient, "POI-Actor"),
	}

	p.Actor.Become(&p)
	p.Actor.JoinGroup(grp)

	err1 := p.Actor.AddBehaviour(actor.NewBehaviourJson[SimTickMessage]("SimTick", func(self *actor.Actor, message com.Message[SimTickMessage]) {
		poi, ok := self.GetState().(*Poi)
		if !ok {
			_ = fmt.Errorf("assertion of State not okay")
		} else {
			data := PoiUpdate{
				ID:         uuid.UUID(poi.ID),
				Name:       poi.Name,
				Location:   poi.GeoLoc,
				Attraction: poi.Attraction,
			}
			reply := com.NewGroupMessage[PoiUpdate](PoiUpdateData, grp.ID, &data)
			err := actor.ActorSendMessageJson(self, reply)
			if err != nil {
				fmt.Println(err)
			}
		}

	}))
	if err1 != nil {
		fmt.Println("Error in Adding behaviour to actor")
	}

	err := p.Actor.AddBehaviour(actor.NewBehaviour[Coordinate](PoiRequestPop, func(self *actor.Actor, message com.Message[Coordinate]) {
		poi, ok := self.State.(*Poi)
		if ok {
			weight := poi.GetWeight(message.Data)
			backMessage := com.NewDirectMessage[float64]("ReceivePopularity", message.Sender, &weight)
			err := actor.ActorSendMessage(self, backMessage)
			if err != nil {
				return
			}
		}
	}))
	if err != nil {
		return nil
	}

	err2 := p.Actor.AddBehaviour(actor.NewBehaviour[float64](PoiSetAttraction, func(self *actor.Actor, message com.Message[float64]) {
		poi, ok := self.State.(*Poi)
		if ok {
			poi.Attraction = message.Data
		}
	}))
	if err2 != nil {
		return nil
	}

	return &p
}
