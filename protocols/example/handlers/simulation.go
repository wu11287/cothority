package handlers

import (
	"errors"
	"strconv"

	"cothority/log"

	"cothority/monitor"
	"cothority/sda"

	"github.com/BurntSushi/toml"
)

/*
This is a simple ExampleHandlers-protocol with two steps:
- announcement - which sends a message to all children
- reply - used for counting the number of children
*/

func init() {
	sda.SimulationRegister("ExampleHandlers", NewSimulation)
}

// Simulation implements sda.Simulation.
type Simulation struct {
	sda.SimulationBFTree
}

// NewSimulation is used internally to register the simulation (see the init()
// function above).
func NewSimulation(config string) (sda.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements sda.Simulation.
func (e *Simulation) Setup(dir string, hosts []string) (
	*sda.SimulationConfig, error) {
	sc := &sda.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Run implements sda.Simulation.
func (e *Simulation) Run(config *sda.SimulationConfig) error {
	size := config.Tree.Size()
	log.Lvl2("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		log.Lvl1("Starting round", round)
		round := monitor.NewTimeMeasure("round")
		p, err := config.Overlay.CreateProtocolSDA("ExampleHandlers", config.Tree)
		if err != nil {
			return err
		}
		go p.Start()
		children := <-p.(*ProtocolExampleHandlers).ChildCount
		round.Record()
		if children != size {
			return errors.New("Didn't get " + strconv.Itoa(size) +
				" children")
		}
	}
	return nil
}
