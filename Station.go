package main

import (
	"fmt"
	"github.com/google/uuid"
	"gitlab.com/anwski/crude-go-actors/actor"
	"gitlab.com/anwski/crude-go-actors/com"
	"math/rand"
	"sync"
	"time"
)

type StationID uuid.UUID
type StationUpdateData struct {
	ID         uuid.UUID
	Location   Coordinate
	Name       string
	Capacity   int
	Occupation int
	Popularity float64
}
type StationInfo struct {
	ID       StationID
	Location Coordinate
}

type Station struct {
	Info        StationInfo
	Name        string
	Capacity    int
	Bias        float64
	Popularity  float64 // Between 0..1 as weight
	Slots       []BikeID
	Popularises map[uuid.UUID]float64
	SlotsMutex  sync.Mutex
	PopMutex    sync.Mutex
	Actor       *actor.Actor
}

func (s *Station) GetLoad() int {
	load := 0
	for _, slot := range s.Slots {
		if slot != (BikeID)(uuid.Nil) {
			load++
		}
	}
	return load
}

func (s *Station) StoreBike(id BikeID) bool {
	for _, bikeId := range s.Slots {
		if bikeId == id {
			return true // bike already stored
		}
	}

	for i, slot := range s.Slots {
		if slot == (BikeID)(uuid.Nil) {
			s.Slots[i] = id
			return true // bike stored
		}
	}

	return false
}

func (s *Station) SendBike(id PodID, bikeID BikeID) {
	msg := com.NewDirectMessage("ReceiveBike", (uuid.UUID)(id), &bikeID)
	err := actor.ActorSendMessage(s.Actor, msg)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Station) SendABike(id PodID) {
	for i, slot := range s.Slots {
		if slot != (BikeID)(uuid.Nil) {
			s.SendBike(id, slot)
			s.Slots[i] = (BikeID)(uuid.Nil)
			break
		}
	}
}

func (s *Station) CalculatePopularity() float64 {
	popularity := float64(0)
	for _, pop := range s.Popularises {
		popularity += pop
	}
	return popularity
}

func (s *Station) RequestPopularity() {
	err := actor.ActorSendMessage[Coordinate](s.Actor, com.NewGroupMessage(PoiRequestPop, s.Actor.GetGroup("SimControlGroup").ID, &s.Info.Location))
	if err != nil {
		fmt.Println(err)
		return
	}
}
func NewStation(sim *Simulation, name string, loc Coordinate, capacity int, bias float64, pop float64) *Station {
	s := Station{
		Info:        StationInfo{},
		Name:        name,
		Capacity:    capacity,
		Bias:        bias,
		Popularity:  pop,
		Slots:       make([]BikeID, capacity),
		SlotsMutex:  sync.Mutex{},
		Actor:       actor.NewActor(sim.MqttClient, name),
		Popularises: make(map[uuid.UUID]float64),
		PopMutex:    sync.Mutex{},
	}

	rand.Seed(time.Now().UnixNano())
	v := rand.Intn(capacity)
	for i := 0; i < v; i++ {
		s.Slots[i] = BikeID(uuid.New())
	}
	s.Info.Location = loc
	s.Info.ID = (StationID)(s.Actor.ID)

	s.Actor.Become(&s)
	s.Actor.JoinGroup(sim.SimGroup)

	err := s.Actor.AddBehaviour(actor.NewBehaviourJson[SimTickMessage]("SimTick", func(self *actor.Actor, message com.Message[SimTickMessage]) {
		station, ok := self.GetState().(*Station)
		if !ok {
			_ = fmt.Errorf("assertion of State not okay")
		} else {
			data := StationUpdateData{
				ID:         uuid.UUID(station.Info.ID),
				Location:   station.Info.Location,
				Name:       station.Name,
				Capacity:   station.Capacity,
				Occupation: station.GetLoad(),
				Popularity: station.Popularity,
			}
			reply := com.NewGroupMessage[StationUpdateData]("StationUpdateData", sim.SimGroup.ID, &data)
			err := actor.ActorSendMessageJson(self, reply)
			if err != nil {
				fmt.Println(err)
			}
		}

	}))
	if err != nil {
		fmt.Println("Error in Adding behaviour to actor")
	}

	err1 := s.Actor.AddBehaviour(actor.NewBehaviour[int]("RequestLoad", func(self *actor.Actor, message com.Message[int]) {
		station, ok := self.GetState().(*Station)
		if ok {
			load := float64(station.GetLoad()) / float64(station.Capacity)
			msg := com.NewDirectMessage[float64]("ReceiveLoad", message.Sender, &load)
			err := actor.ActorSendMessage(self, msg)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(station.Name, " : ", load)
		} else {
			_ = fmt.Errorf("error in station type conversion")
		}
	}))
	if err1 != nil {
		fmt.Println(err)
	}

	err2 := s.Actor.AddBehaviour(actor.NewBehaviour[BikeID]("ReceiveBike", func(self *actor.Actor, message com.Message[BikeID]) {
		station, ok := self.GetState().(*Station)
		if ok {
			station.SlotsMutex.Lock()
			if !station.StoreBike(message.Data) {
				station.SendBike((PodID)(message.Sender), message.Data) // if cant store Bike, send it back to pod
			}
			station.SlotsMutex.Unlock()
		} else {
			_ = fmt.Errorf("error in station type conversion")
		}
	}))
	if err2 != nil {
		fmt.Println(err2)
	}

	err3 := s.Actor.AddBehaviour(actor.NewBehaviour[int]("RequestLocation", func(self *actor.Actor, message com.Message[int]) {
		station, ok := self.GetState().(*Station)
		if ok {
			msg := com.NewDirectMessage("ReceiveLocation", message.Sender, &station.Info.Location)
			err := actor.ActorSendMessage(self, msg)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			_ = fmt.Errorf("error in station type conversion")
		}
	}))
	if err3 != nil {
		fmt.Println(err3)
	}

	err4 := s.Actor.AddBehaviour(actor.NewBehaviour[int]("RequestBikes", func(self *actor.Actor, message com.Message[int]) {
		station, ok := self.GetState().(*Station)
		if ok {
			soll := station.Capacity / 3
			dSoll := station.GetLoad() - soll
			if dSoll < 0 {
				dSoll = 0
			}
			fmt.Println(station.Name, " Sending ", dSoll, " Bikes")
			if dSoll > message.Data {
				dSoll = message.Data
			} else if dSoll <= 0 {
				dSoll = 1
			}

			station.SlotsMutex.Lock()
			for i := 0; i < dSoll; i++ {
				station.SendABike((PodID)(message.Sender))
			}
			station.SlotsMutex.Unlock()
		} else {
			_ = fmt.Errorf("error in station type conversion")
		}
	}))
	if err4 != nil {
		fmt.Println(err4)
	}

	err5 := s.Actor.AddBehaviour(actor.NewBehaviour[float64]("ReceivePopularity", func(self *actor.Actor, message com.Message[float64]) {
		station, ok := self.State.(*Station)
		if ok {
			station.PopMutex.Lock()
			defer station.PopMutex.Unlock()
			station.Popularises[message.Sender] = message.Data
			station.Popularity = station.CalculatePopularity()
			fmt.Println("[ST]", station.Name, ": ", station.Popularity)
		}
	}))
	if err5 != nil {
		fmt.Println(err4)
	}

	err6 := s.Actor.AddBehaviour(actor.NewBehaviour[int]("UpdatePopularity", func(self *actor.Actor, message com.Message[int]) {
		station, ok := self.State.(*Station)
		if ok {
			station.RequestPopularity()
		}
	}))
	if err6 != nil {
		fmt.Println(err4)
	}
	return &s
}
