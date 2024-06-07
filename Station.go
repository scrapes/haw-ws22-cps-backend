package main

import (
	"github.com/google/uuid"
	"github.com/scrapes/haw-ws22-cps-crude-go-actors/actor"
	"github.com/scrapes/haw-ws22-cps-crude-go-actors/com"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"time"
)

const POP_TRIGGER_VAL = 2

type StationID uuid.UUID
type StationUpdateData struct {
	ID         uuid.UUID
	Location   Coordinate
	Name       string
	Capacity   int
	Occupation int
	Popularity Popularity
}
type StationInfo struct {
	ID       StationID
	Location Coordinate
}

type Station struct {
	Info                StationInfo
	Name                string
	Capacity            int
	Bias                float64
	Popularity          Popularity // Between 0..1 as weight
	Slots               []BikeID
	PoiPopularities     map[uuid.UUID]Popularity
	StationPopularities map[uuid.UUID]Popularity
	SlotsMutex          sync.Mutex
	PopMutex            sync.Mutex
	PopCounter          float64
	Actor               *actor.Actor
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
		Logger.Error("Failed to send bike", zap.Error(err))
	}
	Logger.Info("sent bike", zap.String("station", s.Name), zap.String("stationID", (uuid.UUID)(s.Info.ID).String()), zap.String("pod", msg.Receiver.String()))
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

func (s *Station) CalculatePopularity() Popularity {
	popularity := Popularity{
		ToAttraction:   0,
		FromAttraction: 0,
	}
	for _, pop := range s.PoiPopularities {
		popularity = pop.Add(popularity)
	}
	return popularity
}

func (s *Station) RequestPopularity() {
	err := actor.ActorSendMessage[Coordinate](s.Actor, com.NewGroupMessage(PoiRequestPop, s.Actor.GetGroup("SimControlGroup").ID, &s.Info.Location))
	if err != nil {
		Logger.Error("failed to send pop request", zap.Error(err))
		return
	}
}

func (s *Station) SendGroupUpdate(self *actor.Actor) {
	data := StationUpdateData{
		ID:         uuid.UUID(s.Info.ID),
		Location:   s.Info.Location,
		Name:       s.Name,
		Capacity:   s.Capacity,
		Occupation: s.GetLoad(),
		Popularity: s.Popularity,
	}

	Logger.Info("occupation", zap.Int("occupation", data.Occupation), zap.String("stationID", data.ID.String()))

	reply := com.NewGroupMessage[StationUpdateData]("StationUpdateData", self.GetGroup("SimControlGroup").ID, &data)
	err := actor.ActorSendMessageJson(self, reply)
	if err != nil {
		Logger.Error("failed to send group update", zap.Error(err))
	}
}

func (s *Station) RequestStationPopularity(self *actor.Actor) {
	b := byte(0)
	s.StationPopularities = make(map[uuid.UUID]Popularity)
	err := actor.ActorSendMessage[byte](self, com.NewGroupMessage("StationRequestPopularity", self.GetGroup("SimControlGroup").ID, &b))
	if err != nil {
		Logger.Error("failed to send group popularity", zap.Error(err))
		return
	}
}

func (s *Station) MakeBikeDrive(self *actor.Actor, bike BikeID, tostation uuid.UUID) {
	KPI.Mutex.Lock()
	KPI.BikeInDrive++
	KPI.Mutex.Unlock()
	Logger.Info("bike event", zap.String("station", tostation.String()), zap.String("type", "lent"))

	time.Sleep(8 * time.Second)

	KPI.Mutex.Lock()
	KPI.BikeInDrive--
	KPI.Mutex.Unlock()
	Logger.Info("bike event", zap.String("station", tostation.String()), zap.String("type", "received"))

	msg := com.NewDirectMessage[BikeID]("ReceiveBike", tostation, &bike)
	err := actor.ActorSendMessage(self, msg)
	if err != nil {
		Logger.Error("failed to send bike", zap.Error(err))
	}
}
func NewStation(sim *Simulation, name string, loc Coordinate, capacity int, bias float64, pop Popularity) *Station {
	s := Station{
		Info:                StationInfo{},
		Name:                name,
		Capacity:            capacity,
		Bias:                bias,
		Popularity:          pop,
		Slots:               make([]BikeID, capacity),
		SlotsMutex:          sync.Mutex{},
		Actor:               actor.NewActor(sim.MqttClient, name),
		PoiPopularities:     make(map[uuid.UUID]Popularity),
		StationPopularities: make(map[uuid.UUID]Popularity),
		PopMutex:            sync.Mutex{},
		PopCounter:          0,
	}

	rand.Seed(time.Now().UnixNano())

	v := rand.Intn(capacity-3) + 3
	for i := 0; i < v; i++ {
		s.Slots[i] = BikeID(uuid.New())
	}

	KPI.Mutex.Lock()
	KPI.BikesTotal += v
	KPI.Mutex.Unlock()

	s.Info.Location = loc
	s.Info.ID = (StationID)(s.Actor.ID)

	s.Actor.Become(&s)
	s.Actor.JoinGroup(sim.SimGroup)

	err := s.Actor.AddBehaviour(actor.NewBehaviourJson[SimTickMessage]("SimTick", func(self *actor.Actor, message com.Message[SimTickMessage]) {
		station, ok := self.GetState().(*Station)
		if !ok {
			Logger.Error("error getting state")
		} else {
			rand.Seed(time.Now().UnixNano())
			station.PopCounter += rand.Float64() * station.Popularity.FromAttraction
			if station.PopCounter > POP_TRIGGER_VAL {
				station.SendGroupUpdate(self)
				station.PopCounter = 0

				go func() {
				req:
					station.RequestStationPopularity(self)
					time.Sleep(3 * time.Second)

					if station.GetLoad() <= 0 {
						return
					}

					trigger := rand.Float64()
					pops := float64(0)
					backup := uuid.Nil
					id := uuid.Nil
					for _, popularity := range station.StationPopularities {
						pops += popularity.ToAttraction
					}

					trigger *= pops / float64(len(station.StationPopularities)) / 2
					for i, popularity := range station.StationPopularities {
						backup = i
						if popularity.ToAttraction > trigger {
							id = i
						}
					}

					if id == uuid.Nil {
						id = backup
					}

					if id == uuid.Nil {
						goto req
					}

					station.SlotsMutex.Lock()
					bike := BikeID(uuid.Nil)
					for i, slot := range station.Slots {
						if slot != BikeID(uuid.Nil) {
							bike = slot
							station.Slots[i] = BikeID(uuid.Nil)
							break
						}
					}

					station.SlotsMutex.Unlock()
					station.SendGroupUpdate(self)
					station.MakeBikeDrive(self, bike, id)
					station.SendGroupUpdate(self)
				}()
			}
		}

	}))
	if err != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err))
	}

	err1 := s.Actor.AddBehaviour(actor.NewBehaviour[int]("RequestLoad", func(self *actor.Actor, message com.Message[int]) {
		station, ok := self.GetState().(*Station)
		if ok {
			load := float64(station.GetLoad()) / float64(station.Capacity)
			msg := com.NewDirectMessage[float64]("ReceiveLoad", message.Sender, &load)
			err := actor.ActorSendMessage(self, msg)
			if err != nil {
				Logger.Error("Error in Sending Load", zap.Error(err))
			}
			Logger.Info("Station Load", zap.Float64("load", load), zap.String("name", station.Name))
		} else {
			Logger.Error("error getting state")
		}
	}))
	if err1 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err1))
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
			Logger.Error("error getting state")
		}
	}))
	if err2 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err2))
	}

	err3 := s.Actor.AddBehaviour(actor.NewBehaviour[int]("RequestLocation", func(self *actor.Actor, message com.Message[int]) {
		station, ok := self.GetState().(*Station)
		if ok {
			msg := com.NewDirectMessage("ReceiveLocation", message.Sender, &station.Info.Location)
			err := actor.ActorSendMessage(self, msg)
			if err != nil {
				Logger.Error("Error in Sending Location", zap.Error(err))
			}
		} else {
			Logger.Error("error getting state")
		}
	}))
	if err3 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err3))
	}

	err4 := s.Actor.AddBehaviour(actor.NewBehaviour[int]("RequestBikes", func(self *actor.Actor, message com.Message[int]) {
		station, ok := self.GetState().(*Station)
		if ok {
			soll := station.Capacity / 3
			dSoll := station.GetLoad() - soll
			if dSoll < 0 {
				dSoll = 0
			}
			Logger.Info("sending bike", zap.String("station", station.Name), zap.String("stationID", (uuid.UUID)(station.Info.ID).String()), zap.Int("count", dSoll))
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
			Logger.Error("error getting state")
		}
	}))
	if err4 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err4))
	}

	err5 := s.Actor.AddBehaviour(actor.NewBehaviour[Popularity]("ReceivePopularity", func(self *actor.Actor, message com.Message[Popularity]) {
		station, ok := self.State.(*Station)
		if ok {
			station.PopMutex.Lock()
			defer station.PopMutex.Unlock()
			station.PoiPopularities[message.Sender] = message.Data
			station.Popularity = station.CalculatePopularity()
			Logger.Info("Station Popularity", zap.String("name", station.Name), zap.Float64("Attraction", station.Popularity.ToAttraction), zap.Float64("Repulsion", station.Popularity.ToAttraction))
		}
	}))
	if err5 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err5))
	}

	err6 := s.Actor.AddBehaviour(actor.NewBehaviour[int]("UpdatePopularity", func(self *actor.Actor, message com.Message[int]) {
		station, ok := self.State.(*Station)
		if ok {
			station.RequestPopularity()
		}
	}))
	if err6 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err6))
	}

	err7 := s.Actor.AddBehaviour(actor.NewBehaviour[byte]("StationRequestPopularity", func(self *actor.Actor, message com.Message[byte]) {
		station, ok := self.State.(*Station)
		if ok {
			reply := com.NewDirectMessage[Popularity]("StationReceivePopularity", message.Sender, &station.Popularity)
			err := actor.ActorSendMessage(self, reply)
			if err != nil {
				Logger.Error("Error in Sending Popularity", zap.Error(err))
			}
		}
	}))
	if err7 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err7))
	}

	err8 := s.Actor.AddBehaviour(actor.NewBehaviour[Popularity]("StationReceivePopularity", func(self *actor.Actor, message com.Message[Popularity]) {
		station, ok := self.State.(*Station)
		if ok {
			station.PopMutex.Lock()
			defer station.PopMutex.Unlock()
			station.StationPopularities[message.Sender] = message.Data
		}
	}))
	if err8 != nil {
		Logger.Error("Error in Adding behaviour to actor", zap.Error(err8))
	}

	s.SendGroupUpdate(s.Actor)

	return &s
}
