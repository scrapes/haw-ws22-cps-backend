package main

import (
	"github.com/google/uuid"
	"gitlab.com/anwski/crude-go-actors/actor"
	"gitlab.com/anwski/crude-go-actors/com"
	"go.uber.org/zap"
	"sync"
)

type PodID uuid.UUID
type PodInfo struct {
	ID       uuid.UUID
	Location Coordinate
	State    string
	Capacity int
	Used     int
}
type PodWorkData struct {
	StationLoad      map[float64]StationID
	StationLoadMutex sync.Mutex
	TickTimeout      int
	LastStation      StationID
}
type Pod struct {
	Info         PodInfo
	Capacity     int
	Slots        []BikeID // slots of UUIDs of Bikes
	SlotMutex    sync.Mutex
	NextStop     StationInfo
	Route        Route
	Actor        *actor.Actor
	Routinator   *Routinator
	Data         PodWorkData
	RoutineMutex sync.Mutex
}

func (pod *Pod) NavigateTo(loc Coordinate, rtn Routinator) {
	pod.Route = *NewRoute(pod.Route.Speed, rtn.GetRoute(pod.Route.Loc, loc))
	pod.Info.Location = pod.Route.Loc
}

func (pod *Pod) IsEmpty() bool {
	usage := 0
	empty := true
	for _, slot := range pod.Slots {
		if slot != (BikeID)(uuid.Nil) {
			empty = false
			usage += 1
		}
	}
	return empty
}

func (pod *Pod) RequestCapacities() {
	group := pod.Actor.GetGroup("SimControlGroup")
	if group != nil {
		msg := com.NewGroupMessage[int]("RequestLoad", group.ID, &pod.Data.TickTimeout)
		err := actor.ActorSendMessage(pod.Actor, msg)
		if err != nil {
			Logger.Error("Error sending RequestLoad Message", zap.Error(err))
		}
		pod.Info.State = "WaitingForLoad"
	}
}

func (pod *Pod) CalculateTick(currentTick int) {
	pod.RoutineMutex.Lock()
	defer pod.RoutineMutex.Unlock()

	if pod.Info.State == "MovingToLoad" || pod.Info.State == "MovingToDump" {
		pod.Route.Step(0)
		pod.Info.Location = pod.Route.Loc
		if pod.Route.GetRemainingDistance() < 0.01 {
			if pod.Info.State == "MovingToDump" {
				pod.SendBikes(pod.NextStop.ID)
				pod.Info.State = "Dumped"
			} else {
				msg := com.NewDirectMessage("RequestBikes", (uuid.UUID)(pod.NextStop.ID), &pod.Capacity)
				err := actor.ActorSendMessage(pod.Actor, msg)
				if err != nil {
					Logger.Error("Error sending RequestBikes Message", zap.Error(err))
				}
				pod.Data.TickTimeout = currentTick + (5 * (1 / CONST_speed))
				pod.Info.State = "WaitingForLoading"
			}

		}
	} else if pod.Info.State == "Dumped" {
		pod.Data.TickTimeout = currentTick + (5 * (1 / CONST_speed)) // timeout to 5 seconds
		pod.RequestCapacities()

	} else if pod.Info.State == "WaitingForLoad" {
		if currentTick >= pod.Data.TickTimeout {
			station := StationID{}
			if pod.IsEmpty() {
				// find fullest station
				capacity := float64(-1)
				for sCapacity, id := range pod.Data.StationLoad {
					if sCapacity > capacity && pod.Data.LastStation != id {
						capacity = sCapacity
						station = id
					}
				}
			} else {
				// find emptiest station
				capacity := float64(2)
				for sCapacity, id := range pod.Data.StationLoad {
					if sCapacity < capacity && id != pod.Data.LastStation {
						capacity = sCapacity
						station = id
					}
				}
			}

			pod.Data.StationLoad = make(map[float64]StationID)

			Logger.Info("Next Station", zap.String("PodID", pod.Info.ID.String()), zap.String("StationID", (uuid.UUID)(station).String()))

			msg := com.NewDirectMessage[int]("RequestLocation", (uuid.UUID)(station), &currentTick)
			err := actor.ActorSendMessage(pod.Actor, msg)
			if err != nil {
				Logger.Error("Error sending RequestLocation Message", zap.Error(err))
			}

			pod.Info.State = "WaitingForCoords"
		}
	} else if pod.Info.State == "WaitingForLoading" {
		if currentTick >= pod.Data.TickTimeout {
			pod.RequestCapacities()
		}
	} else {
		pod.Data.TickTimeout = currentTick + (5 * (1 / CONST_speed)) // timeout to 5 seconds
		pod.RequestCapacities()
	}

	grp := pod.Actor.GetGroup("SimControlGroup")
	if grp != nil {
		msg := com.NewGroupMessage("PodUpdate", grp.ID, &pod.Info)
		err := actor.ActorSendMessageJson(pod.Actor, msg)
		if err != nil {
			Logger.Error("Error sending PodUpdate Message", zap.Error(err))
		}
	}

}

func (pod *Pod) SendBike(bid BikeID, sid StationID) {
	Logger.Info("sending bike", zap.String("podID", pod.Info.ID.String()), zap.String("stationID", (uuid.UUID)(sid).String()), zap.String("bikeID", (uuid.UUID)(bid).String()))
	msg := com.NewDirectMessage[BikeID]("ReceiveBike", (uuid.UUID)(sid), &bid)
	err := actor.ActorSendMessage(pod.Actor, msg)
	if err != nil {
		Logger.Error("Error sending Bike", zap.Error(err))
	}
}

func (pod *Pod) SendBikes(id StationID) {
	pod.SlotMutex.Lock()
	for _, slot := range pod.Slots {
		if slot != BikeID(uuid.Nil) {
			pod.SendBike(slot, id)
		}
	}
	pod.Slots = make([]BikeID, pod.Capacity)
	pod.SlotMutex.Unlock()
}

func (pod *Pod) VisualizeSlots() string {
	str := "[ "
	pod.SlotMutex.Lock()
	for _, slot := range pod.Slots {
		if slot == (BikeID)(uuid.Nil) {
			str += "0 "
		} else {
			str += "1 "
		}
	}
	str += "]"
	pod.SlotMutex.Unlock()
	return str
}

func (pod *Pod) StoreBike(id BikeID) bool {
	for _, bikeID := range pod.Slots {
		if bikeID == id {
			return true // bike already stored
		}
	}

	for i, bikeID := range pod.Slots {
		if bikeID == (BikeID)(uuid.Nil) {
			pod.Slots[i] = id
			return true // bike stored
		}
	}

	return false // no slots available
}

func NewPod(sim *Simulation, speed float64, where Coordinate, capacity int) *Pod {
	p := Pod{
		Info: PodInfo{
			ID:       uuid.UUID{},
			Location: where,
		},
		Data: PodWorkData{
			StationLoad:      make(map[float64]StationID),
			StationLoadMutex: sync.Mutex{},
			TickTimeout:      0,
		},
		Capacity:  capacity,
		Slots:     make([]BikeID, capacity),
		SlotMutex: sync.Mutex{},
		NextStop: StationInfo{
			ID:       StationID{},
			Location: Coordinate{},
		},
		Route: Route{
			Route:      nil,
			RouteIndex: -1,
			RouteAngle: -1,
			Loc:        where,
			Speed:      speed,
		},
		Actor:      actor.NewActor(sim.MqttClient, "PodActor"),
		Routinator: sim.Routinator,
	}
	p.Info.ID = (uuid.UUID)(p.Actor.ID)
	p.Actor.Become(&p)
	p.Actor.JoinGroup(sim.SimGroup)

	// Tick Behaviour for routing and maintainance
	err := p.Actor.AddBehaviour(actor.NewBehaviourJson[SimTickMessage]("SimTick", func(self *actor.Actor, message com.Message[SimTickMessage]) {
		pod, ok := self.GetState().(*Pod)
		if !ok {
			Logger.Error("state is not pod")
		} else {
			pod.CalculateTick(message.Data.Tick)
			Logger.Info("Pod Slots", zap.String("slots", pod.VisualizeSlots()), zap.String("state", pod.Info.State), zap.Float64("distance", pod.Route.GetRemainingDistance()))
		}
	}))

	if err != nil {
		Logger.Error("Error adding behaviour to actor", zap.Error(err))
	}

	// GetLocation Behaviour
	err2 := p.Actor.AddBehaviour(actor.NewBehaviour[Coordinate]("ReceiveLocation", func(self *actor.Actor, message com.Message[Coordinate]) {
		// message designated for me e.g. not for Group
		if message.Receiver == self.ID {
			pod, ok := self.GetState().(*Pod)
			if !ok {
				Logger.Error("state is not pod")
			} else {
				pod.Data.LastStation = (StationID)(message.Sender)
				pod.NextStop.Location = message.Data
				pod.NextStop.ID = (StationID)(message.Sender)
				// be aware that routinator is an block but async operation
				pod.Route = *NewRoute(pod.Route.Speed, pod.Routinator.GetRoute(pod.Info.Location, message.Data))
				pod.Info.Location = pod.Route.Loc

				if pod.IsEmpty() {
					pod.Info.State = "MovingToLoad"
				} else {
					pod.Info.State = "MovingToDump"
				}
			}
		}
	}))

	if err2 != nil {
		Logger.Error("Error adding behaviour to actor", zap.Error(err2))
	}

	err3 := p.Actor.AddBehaviour(actor.NewBehaviour[float64]("ReceiveLoad", func(self *actor.Actor, message com.Message[float64]) {
		// message designated for me e.g. not for Group
		if message.Receiver == self.ID {
			pod, ok := self.GetState().(*Pod)
			if !ok {
				Logger.Error("state is not pod")
			} else {
				data := message.Data
				pod.Data.StationLoadMutex.Lock()
				pod.Data.StationLoad[data] = (StationID)(message.Sender)
				pod.Data.StationLoadMutex.Unlock()
			}
		}
	}))
	if err3 != nil {
		Logger.Error("Error adding behaviour to actor", zap.Error(err3))
	}

	err4 := p.Actor.AddBehaviour(actor.NewBehaviour[BikeID]("ReceiveBike", func(self *actor.Actor, message com.Message[BikeID]) {
		Logger.Info("receive bike", zap.String("pod", message.Receiver.String()), zap.String("bike", (uuid.UUID)(message.Data).String()))
		if message.Receiver == self.ID {
			pod, ok := self.GetState().(*Pod)
			if !ok {
				Logger.Error("state is not pod")
			} else {
				pod.SlotMutex.Lock()
				if !pod.StoreBike(message.Data) {
					// if pod is full, send back bike
					pod.SendBike(message.Data, StationID(message.Sender))
				} else {
					Logger.Info("Received bike", zap.String("podID", self.ID.String()), zap.String("bikeID", (uuid.UUID)(message.Data).String()))
				}
				pod.Data.LastStation = StationID(message.Sender)
				pod.SlotMutex.Unlock()
			}
		}
	}))

	if err4 != nil {
		Logger.Error("Error adding behaviour to actor", zap.Error(err4))
	}

	return &p
}
