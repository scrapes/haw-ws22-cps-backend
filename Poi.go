package main

import (
	"github.com/google/uuid"
	"github.com/scrapes/haw-ws22-cps-crude-go-actors/actor"
	"github.com/scrapes/haw-ws22-cps-crude-go-actors/com"
	"go.uber.org/zap"
)

const (
	PoiRequestPop    string = "POI-Request-Pop"
	PoiSetAttraction string = "POI-Set-Attraction"
	PoiUpdateData    string = "PoiUpdateData"
)

type PoiID uuid.UUID

type Popularity struct {
	ToAttraction   float64
	FromAttraction float64
}

func (p Popularity) Multiply(multiplier float64) Popularity {
	p.ToAttraction *= multiplier
	p.FromAttraction *= multiplier
	return p
}

func (p Popularity) Add(b Popularity) Popularity {
	p.FromAttraction += b.FromAttraction
	p.ToAttraction += b.ToAttraction
	return p
}

type PoiUpdate struct {
	ID         uuid.UUID
	Name       string
	Location   Coordinate
	Popularity Popularity
}

type Poi struct {
	ID         PoiID
	Name       string
	GeoLoc     Coordinate
	Popularity Popularity
	Actor      *actor.Actor
}

func (poi *Poi) GetWeight(loc Coordinate) Popularity {
	distance := loc.DistanceTo(poi.GeoLoc)
	distanceWeight := float64(0)
	if distance <= CONST_poi_max_distance {
		distanceWeight = 1 - (distance / CONST_poi_max_distance) // reverse bias to get closest
	}
	return poi.Popularity.Multiply(distanceWeight)
}

func NewPOI(mqttclient *com.MqttClient, grp *actor.Group, Name string, GeoLoc Coordinate, ToA float64, FromA float64) *Poi {
	p := Poi{
		ID:     PoiID(uuid.New()),
		Name:   Name,
		GeoLoc: GeoLoc,
		Popularity: Popularity{
			FromAttraction: FromA,
			ToAttraction:   ToA,
		},
		Actor: actor.NewActor(mqttclient, "POI-Actor"),
	}

	p.Actor.Become(&p)
	p.Actor.JoinGroup(grp)

	err1 := p.Actor.AddBehaviour(actor.NewBehaviourJson[SimTickMessage]("SimTick", func(self *actor.Actor, message com.Message[SimTickMessage]) {
		poi, ok := self.GetState().(*Poi)
		if !ok {
			Logger.Error("state ist not POI")
		} else {
			data := PoiUpdate{
				ID:         uuid.UUID(poi.ID),
				Name:       poi.Name,
				Location:   poi.GeoLoc,
				Popularity: poi.Popularity,
			}
			reply := com.NewGroupMessage[PoiUpdate](PoiUpdateData, grp.ID, &data)
			err := actor.ActorSendMessageJson(self, reply)
			if err != nil {
				Logger.Error("send poi update fail", zap.Error(err))
			}
		}

	}))
	if err1 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err1))
		return nil
	}

	err := p.Actor.AddBehaviour(actor.NewBehaviour[Coordinate](PoiRequestPop, func(self *actor.Actor, message com.Message[Coordinate]) {
		poi, ok := self.State.(*Poi)
		if ok {
			weight := poi.GetWeight(message.Data)
			backMessage := com.NewDirectMessage[Popularity]("ReceivePopularity", message.Sender, &weight)
			err := actor.ActorSendMessage(self, backMessage)
			if err != nil {
				Logger.Error("send popularity fail", zap.Error(err))
				return
			}
		}
	}))
	if err != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err))
		return nil
	}

	err2 := p.Actor.AddBehaviour(actor.NewBehaviour[Popularity](PoiSetAttraction, func(self *actor.Actor, message com.Message[Popularity]) {
		poi, ok := self.State.(*Poi)
		if ok {
			poi.Popularity = message.Data
		}
	}))
	if err2 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err2))
		return nil
	}

	return &p
}
