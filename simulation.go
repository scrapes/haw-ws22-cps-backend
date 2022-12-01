package main

import (
	"fmt"
	"github.com/google/uuid"
	"gitlab.com/anwski/crude-go-actors/actor"
	"gitlab.com/anwski/crude-go-actors/com"
	"time"
)

type SimSetAttractionMessage struct {
	Name       string
	Attraction float64
}
type SimTickMessage struct {
	Tick int
}

type SimControlMessage struct {
	Config SimulationConfig
	Reset  bool
}

type SimulationConfig struct {
	pad   byte
	Speed int // speed, ticks per second
}

type CreateStationData struct {
	Name string
	Loc  Coordinate
	Size int
	Bias float64
}

type CreatePodData struct {
	Loc      Coordinate
	Capacity int
	Speed    float64
}

type Simulation struct {
	Configuration SimulationConfig

	// Data
	Stations map[StationID]*Station
	Pods     map[uuid.UUID]*Pod
	Bikes    map[BikeID]Bike
	Pois     map[string]*Poi

	MqttClient   *com.MqttClient
	Routinator   *Routinator
	SimActor     *actor.Actor
	SimBehaviour *actor.Behaviour
	SimGroup     *actor.Group
}

func (s *Simulation) Start() {
	tick := SimTickMessage{Tick: 0}
	resetData := SimControlMessage{Reset: true}
	resetMsg := com.NewGroupMessage("cmd", s.SimGroup.ID, &resetData)
	reply := com.NewGroupMessage[SimTickMessage]("SimTick", s.SimGroup.ID, &tick)
	time.Sleep(time.Second / time.Duration(s.Configuration.Speed))

	er := resetMsg.SendJsonAsName(s.MqttClient, s.SimGroup.Name)
	if er != nil {
		fmt.Println(er)
	}

	err := actor.ActorSendMessageJson(s.SimActor, reply)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Simulation) SetSpeed(speed int) {
	dat := SimulationConfig{
		Speed: speed,
	}
	reply := com.NewDirectMessage[SimulationConfig]("ReceiveSimConfig", s.SimActor.ID, &dat)
	time.Sleep(time.Second / time.Duration(s.Configuration.Speed))
	err := actor.ActorSendMessageJson(s.SimActor, reply)
	if err != nil {
		fmt.Println(err)
	}
}

func CreateSimulation(client *com.MqttClient) *Simulation {
	sim := Simulation{
		Configuration: SimulationConfig{
			Speed: CONST_speed,
		},

		Stations:   make(map[StationID]*Station),
		Pods:       make(map[uuid.UUID]*Pod),
		Pois:       make(map[string]*Poi),
		Bikes:      make(map[BikeID]Bike),
		MqttClient: client,
		SimGroup:   actor.NewGroup("SimControlGroup"),
	}
	sim.Routinator = NewRoutinator("routinator", sim.MqttClient)
	sim.SimActor = actor.NewActor(sim.MqttClient, "Simulation Actor")
	sim.SimActor.JoinGroup(sim.SimGroup)

	// Abusing State as sim reference
	sim.SimActor.Become(&sim)
	time.Sleep(time.Second * 5)

	// Stupid Tick Behaviour that sends every x second one tick after receiving the last one
	err := sim.SimActor.AddBehaviour(actor.NewBehaviourJson[SimTickMessage]("SimTick", func(self *actor.Actor, message com.Message[SimTickMessage]) {
		message.Data.Tick++
		simulation, ok := self.GetState().(*Simulation)
		if !ok {
			_ = fmt.Errorf("assertion of State not okay")
		} else {
			reply := com.NewGroupMessage[SimTickMessage]("SimTick", simulation.SimGroup.ID, &message.Data)
			time.Sleep(time.Second / time.Duration(simulation.Configuration.Speed))
			err := actor.ActorSendMessageJson(self, reply)
			if err != nil {
				fmt.Println(err)
			}
		}

	}))
	if err != nil {
		fmt.Println("Error in Adding behaviour to sim actor")
	}

	// Configuration behaviour
	err2 := sim.SimActor.AddBehaviour(actor.NewBehaviourJson[SimulationConfig]("ReceiveSimConfig", func(self *actor.Actor, message com.Message[SimulationConfig]) {
		simulation, ok := self.GetState().(*Simulation)
		if !ok {
			_ = fmt.Errorf("assertion of State not okay")
		} else {
			simulation.Configuration = message.Data
		}
	}))
	if err2 != nil {
		fmt.Println("Error in Adding behaviour to sim actor")
	}

	err3 := sim.SimActor.AddBehaviour(actor.NewBehaviourJson[CreateStationData]("SimCreateStation", func(self *actor.Actor, message com.Message[CreateStationData]) {
		simulation, ok := self.GetState().(*Simulation)
		if !ok {
			_ = fmt.Errorf("assertion of State not okay")
		} else {
			d := message.Data
			newStation := NewStation(simulation, d.Name, d.Loc, d.Size, d.Bias, 0)
			sim.Stations[newStation.Info.ID] = newStation
		}
	}))
	if err3 != nil {
		fmt.Println("Error in Adding behaviour to sim actor")
	}

	err4 := sim.SimActor.AddBehaviour(actor.NewBehaviourJson[CreatePodData]("SimCreatePod", func(self *actor.Actor, message com.Message[CreatePodData]) {
		simulation, ok := self.GetState().(*Simulation)
		if !ok {
			_ = fmt.Errorf("assertion of State not okay")
		} else {
			d := message.Data
			pd1 := NewPod(simulation, d.Speed, d.Loc, d.Capacity)
			sim.Pods[pd1.Info.ID] = pd1
		}
	}))
	if err4 != nil {
		fmt.Println("Error in Adding behaviour to sim actor")
	}

	err5 := sim.SimActor.AddBehaviour(actor.NewBehaviourJson[SimSetAttractionMessage]("SimSetAttraction", func(self *actor.Actor, message com.Message[SimSetAttractionMessage]) {
		simulation, ok := self.GetState().(*Simulation)
		if !ok {
			_ = fmt.Errorf("assertion of State not okay")
		} else {
			d := message.Data
			simulation.Pois[d.Name].Attraction = d.Attraction
			for _, station := range simulation.Stations {
				station.RequestPopularity()
			}
		}
	}))
	if err5 != nil {
		fmt.Println("Error in Adding behaviour to sim actor")
	}

	/*
		Load Stations
	*/
	stEidelstedterPlatz := NewStation(&sim, "Eidelstedter Platz", Coordinate{53.60661423204293, 9.903912989786303}, 20, 0, 0)
	sim.Stations[stEidelstedterPlatz.Info.ID] = stEidelstedterPlatz

	stEidelstedt := NewStation(&sim, "S-Eidelstedt", Coordinate{53.59678238412708, 9.90592784469073}, 15, 0, 0)
	sim.Stations[stEidelstedt.Info.ID] = stEidelstedt
	stElbgaustrasse := NewStation(&sim, "S-Elbgaustrasse", Coordinate{53.60284565615403, 9.89179520993729}, 15, 0, 0)
	sim.Stations[stElbgaustrasse.Info.ID] = stElbgaustrasse
	stSLangenfelde := NewStation(&sim, "S-Langenfelde", Coordinate{53.57965878299834, 9.929212934391371}, 15, 0, 0)
	sim.Stations[stSLangenfelde.Info.ID] = stSLangenfelde
	stStellingen := NewStation(&sim, "S-Stellingen", Coordinate{53.590367590915335, 9.919342063891001}, 15, 0, 0)
	sim.Stations[stStellingen.Info.ID] = stStellingen
	stSBahrenfeld := NewStation(&sim, "S-Bahrenfeld", Coordinate{53.55997703177752, 9.909492680964616}, 15, 0, 0)
	sim.Stations[stSBahrenfeld.Info.ID] = stSBahrenfeld

	stStadion := NewStation(&sim, "Volksparkstadion", Coordinate{53.58757994042396, 9.903935555351431}, 25, 0, 0)
	sim.Stations[stStadion.Info.ID] = stStadion

	stBahrenfelderStr := NewStation(&sim, "BahrenfelderStr,Von-Sauer-Str", Coordinate{53.56628625921898, 9.912247578577507}, 10, 0, 0)
	sim.Stations[stBahrenfelderStr.Info.ID] = stBahrenfelderStr
	stEbertallee := NewStation(&sim, "Ebertallee", Coordinate{53.57364223632872, 9.892397307291459}, 10, 0, 0)
	sim.Stations[stEbertallee.Info.ID] = stEbertallee

	stDesy := NewStation(&sim, "Desy Gebauede 25b", Coordinate{53.58051008676595, 9.884356214925655}, 20, 0, 0)
	sim.Stations[stDesy.Info.ID] = stDesy
	/*
		Load Pods
	*/

	pd1 := NewPod(&sim, 150, Coordinate{53.588532121996714, 9.876192892343472}, 10)
	sim.Pods[pd1.Info.ID] = pd1

	pd2 := NewPod(&sim, 150, Coordinate{53.56907457893642, 9.902722536627016}, 10)
	sim.Pods[pd2.Info.ID] = pd2

	/*
		Load Pois (Train Stations, Stadium, Supermarkets)
	*/
	poiVolkspark := NewPOI(sim.MqttClient, sim.SimGroup, "Volksparkstadion", Coordinate{53.5873503764629, 9.898852191846043}, 0.3)
	sim.Pois[poiVolkspark.Name] = poiVolkspark

	poiBarclaysArena := NewPOI(sim.MqttClient, sim.SimGroup, "Barclays Arena", Coordinate{53.589357924013065, 9.899143208616863}, 0.3)
	sim.Pois[poiBarclaysArena.Name] = poiBarclaysArena

	poiSStellingen := NewPOI(sim.MqttClient, sim.SimGroup, "S-Bahn Stellingen", Coordinate{53.589973247084004, 9.918811914968728}, 1)
	sim.Pois[poiSStellingen.Name] = poiSStellingen

	poiSEidelstedt := NewPOI(sim.MqttClient, sim.SimGroup, "S-Bahn Eidelstedt", Coordinate{53.59593368989732, 9.906587127274884}, 1)
	sim.Pois[poiSEidelstedt.Name] = poiSEidelstedt

	poiSElbgaustrasse := NewPOI(sim.MqttClient, sim.SimGroup, "S-Bahn Elbgaustrasse", Coordinate{53.602724437413244, 9.892021694621539}, 1)
	sim.Pois[poiSElbgaustrasse.Name] = poiSElbgaustrasse

	poiSLangenfelde := NewPOI(sim.MqttClient, sim.SimGroup, "S-Bahn Langenfelde", Coordinate{53.579767351704206, 9.930222519436683}, 1)
	sim.Pois[poiSLangenfelde.Name] = poiSLangenfelde

	poiSBahrenfeld := NewPOI(sim.MqttClient, sim.SimGroup, "S-Bahn Bahrenfeld", Coordinate{53.56014781227735, 9.910238735667162}, 1)
	sim.Pois[poiSBahrenfeld.Name] = poiSBahrenfeld

	poiFriedhofAltona := NewPOI(sim.MqttClient, sim.SimGroup, "Friedhof Altona", Coordinate{53.58389166487952, 9.889299677024592}, 0.45)
	sim.Pois[poiFriedhofAltona.Name] = poiFriedhofAltona

	poiTrabrennbahn := NewPOI(sim.MqttClient, sim.SimGroup, "Trabrennbahn Altona", Coordinate{53.576687688196415, 9.892450118626606}, 0.3)
	sim.Pois[poiTrabrennbahn.Name] = poiTrabrennbahn

	poiRabatzz := NewPOI(sim.MqttClient, sim.SimGroup, "rabatzz!", Coordinate{53.59941474737661, 9.914978071492532}, 0.3)
	sim.Pois[poiRabatzz.Name] = poiRabatzz

	poiEdelfettwerk := NewPOI(sim.MqttClient, sim.SimGroup, "Edelfettwerk", Coordinate{53.59503910174099, 9.90556320730452}, 0.3)
	sim.Pois[poiEdelfettwerk.Name] = poiEdelfettwerk

	poiFriedhofHolstenkamp := NewPOI(sim.MqttClient, sim.SimGroup, "Friedhof Holstenkamp", Coordinate{53.59503910174099, 9.90556320730452}, 0.3)
	sim.Pois[poiFriedhofHolstenkamp.Name] = poiFriedhofHolstenkamp

	poiDesy := NewPOI(sim.MqttClient, sim.SimGroup, "Desy", Coordinate{53.57761688576384, 9.881601013238047}, 1.2)
	sim.Pois[poiDesy.Name] = poiDesy
	/*
		Load/Generate Bikes
	*/

	/*
		Generation Popularties
	*/

	for _, station := range sim.Stations {
		station.RequestPopularity()
	}

	return &sim
}
