package sda

import (
	"errors"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"net"

	"cothority/log"

	"cothority/network"

	"github.com/BurntSushi/toml"
	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

type simulationCreate func(string) (Simulation, error)

var simulationRegistered map[string]simulationCreate

// SimulationFileName is the name of the (binary encoded) file containing the
// simulation config.
const SimulationFileName = "simulation.bin"

// Simulation is an interface needed by every protocol that wants to be available
// to be used in a simulation.
type Simulation interface {
	// This has to initialise all necessary files and copy them to the
	// 'dir'-directory. This directory will be accessible to all simulated
	// hosts.
	// Setup also gets a slice of all available hosts. In turn it has
	// to return a tree using one or more of these hosts. It can create
	// the Roster as desired, putting more than one ServerIdentity/Host on the same host.
	// The 'config'-argument holds all arguments read from the runfile in
	// toml-format.
	Setup(dir string, hosts []string) (*SimulationConfig, error)

	// Node will be run for every node and might be used to setup load-
	// creation. It is started once the Host is set up and running, but before
	// 'Run'
	Node(config *SimulationConfig) error

	// Run will begin with the simulation or return an error. It is sure
	// to be run on the host where 'tree.Root' is. It should only return
	// when all rounds are done.
	Run(config *SimulationConfig) error
}

// SimulationConfig has to be returned from 'Setup' and will be passed to
// 'Run'.
type SimulationConfig struct {
	// Represents the tree that has to be used
	Tree *Tree
	// The Roster used by the tree
	Roster *Roster
	// All private keys generated by 'Setup', indexed by the complete addresses
	PrivateKeys map[string]abstract.Scalar
	// If non-nil, points to our overlay
	Overlay *Overlay
	// If non-nil, points to our host
	Host *Host
	// Additional configuration used to run
	Config string
}

// SimulationConfigFile stores the state of the simulation's config.
// Only used internally.
type SimulationConfigFile struct {
	TreeMarshal *TreeMarshal
	Roster      *Roster
	PrivateKeys map[string]abstract.Scalar
	Config      string
}

// LoadSimulationConfig gets all configuration from dir + SimulationFileName and instantiates the
// corresponding host 'ha'.
func LoadSimulationConfig(dir, ha string) ([]*SimulationConfig, error) {
	network.RegisterPacketType(SimulationConfigFile{})
	bin, err := ioutil.ReadFile(dir + "/" + SimulationFileName)
	if err != nil {
		return nil, err
	}
	_, msg, err := network.UnmarshalRegisteredType(bin,
		network.DefaultConstructors(network.Suite))
	if err != nil {
		return nil, err
	}
	scf := msg.(SimulationConfigFile)
	sc := &SimulationConfig{
		Roster:      scf.Roster,
		PrivateKeys: scf.PrivateKeys,
		Config:      scf.Config,
	}
	sc.Tree, err = scf.TreeMarshal.MakeTree(sc.Roster)
	if err != nil {
		return nil, err
	}

	var ret []*SimulationConfig
	if ha != "" {
		if !strings.Contains(ha, ":") {
			// to correctly match hosts a column is needed, else
			// 10.255.0.1 would also match 10.255.0.10 and others
			ha += ":"
		}
		for _, e := range sc.Roster.List {
			for _, a := range e.Addresses {
				log.Lvl4("Searching for", ha, "in", a)
				// If we are started in Deterlab- or other
				// big-server-needs-multiple-hosts, we might
				// want to initialise all hosts in one instance
				// of 'cothority' so as to minimize memory
				// footprint
				if strings.Contains(a, ha) {
					log.Lvl3("Found host", a, "to match", ha)
					host := NewHost(e, scf.PrivateKeys[a])
					scNew := *sc
					scNew.Host = host
					scNew.Overlay = host.overlay
					ret = append(ret, &scNew)
				}

			}
		}
		if len(ret) == 0 {
			return nil, errors.New("Didn't find address: " + ha)
		}
	} else {
		ret = append(ret, sc)
	}
	if strings.Contains(sc.Roster.List[0].Addresses[0], "localhost") {
		// Now strip all superfluous numbers of localhost
		for i := range sc.Roster.List {
			_, port, _ := net.SplitHostPort(sc.Roster.List[i].Addresses[0])
			sc.Roster.List[i].Addresses[0] = "localhost:" + port
		}
	}
	return ret, nil
}

// Save takes everything in the SimulationConfig structure and saves it to
// dir + SimulationFileName
func (sc *SimulationConfig) Save(dir string) error {
	network.RegisterPacketType(&SimulationConfigFile{})
	scf := &SimulationConfigFile{
		TreeMarshal: sc.Tree.MakeTreeMarshal(),
		Roster:      sc.Roster,
		PrivateKeys: sc.PrivateKeys,
		Config:      sc.Config,
	}
	buf, err := network.MarshalRegisteredType(scf)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(dir+"/"+SimulationFileName, buf, 0660)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

// GetService returns the service with the given name.
func (sc *SimulationConfig) GetService(name string) Service {
	return sc.Host.serviceManager.Service(name)
}

// SimulationRegister is must to be called to register a simulation.
// Protocol or simulation developers must not forget to call this function
// with the protocol's name.
func SimulationRegister(name string, sim simulationCreate) {
	if simulationRegistered == nil {
		simulationRegistered = make(map[string]simulationCreate)
	}
	simulationRegistered[name] = sim
}

// NewSimulation returns a simulation and decodes the 'conf' into the
// simulation-structure
func NewSimulation(name string, conf string) (Simulation, error) {
	sim, ok := simulationRegistered[name]
	if !ok {
		return nil, errors.New("Didn't find simulation " + name)
	}
	simInst, err := sim(conf)
	if err != nil {
		return nil, err
	}
	_, err = toml.Decode(conf, simInst)
	if err != nil {
		return nil, err
	}
	return simInst, nil
}

// SimulationBFTree is the main struct storing the data for all the simulations
// which use a tree with a certain branching factor or depth.
type SimulationBFTree struct {
	Rounds     int
	BF         int
	Hosts      int
	SingleHost bool
	Depth      int
}

// CreateRoster creates an Roster with the host-names in 'addresses'.
// It creates 's.Hosts' entries, starting from 'port' for each round through
// 'addresses'
func (s *SimulationBFTree) CreateRoster(sc *SimulationConfig, addresses []string, port int) {
	start := time.Now()
	nbrAddr := len(addresses)
	if sc.PrivateKeys == nil {
		sc.PrivateKeys = make(map[string]abstract.Scalar)
	}
	hosts := s.Hosts
	if s.SingleHost {
		// If we want to work with a single host, we only make one
		// host per server
		log.Fatal("Not supported yet")
		hosts = nbrAddr
		if hosts > s.Hosts {
			hosts = s.Hosts
		}
	}
	localhosts := false
	listeners := make([]net.Listener, hosts)
	if strings.Contains(addresses[0], "localhost") {
		localhosts = true
	}
	entities := make([]*network.ServerIdentity, hosts)
	log.Lvl3("Doing", hosts, "hosts")
	key := config.NewKeyPair(network.Suite)
	for c := 0; c < hosts; c++ {
		key.Secret.Add(key.Secret,
			key.Suite.Scalar().One())
		key.Public.Add(key.Public,
			key.Suite.Point().Base())
		address := addresses[c%nbrAddr] + ":"
		if localhosts {
			// If we have localhosts, we have to search for an empty port
			var err error
			listeners[c], err = net.Listen("tcp", ":0")
			if err != nil {
				log.Fatal("Couldn't search for empty port:", err)
			}
			_, p, _ := net.SplitHostPort(listeners[c].Addr().String())
			address += p
			log.Lvl4("Found free port", address)
		} else {
			address += strconv.Itoa(port + c/nbrAddr)
		}
		entities[c] = network.NewServerIdentity(key.Public, address)
		sc.PrivateKeys[entities[c].Addresses[0]] = key.Secret
	}
	// And close all our listeners
	if localhosts {
		for _, l := range listeners {
			err := l.Close()
			if err != nil {
				log.Fatal("Couldn't close port:", l, err)
			}
		}
	}

	sc.Roster = NewRoster(entities)
	log.Lvl3("Creating entity List took: " + time.Now().Sub(start).String())
}

// CreateTree the tree as defined in SimulationBFTree and stores the result
// in 'sc'
func (s *SimulationBFTree) CreateTree(sc *SimulationConfig) error {
	log.Lvl3("CreateTree strarted")
	start := time.Now()
	if sc.Roster == nil {
		return errors.New("Empty Roster")
	}
	sc.Tree = sc.Roster.GenerateBigNaryTree(s.BF, s.Hosts)
	log.Lvl3("Creating tree took: " + time.Now().Sub(start).String())
	return nil
}

// Node - standard registers the entityList and the Tree with that Overlay,
// so we don't have to pass that around for the experiments.
func (s *SimulationBFTree) Node(sc *SimulationConfig) error {
	sc.Overlay.RegisterRoster(sc.Roster)
	sc.Overlay.RegisterTree(sc.Tree)
	return nil
}

// GetSingleHost returns the 'SingleHost'-flag
func (sc SimulationConfig) GetSingleHost() bool {
	var sh struct{ SingleHost bool }
	_, err := toml.Decode(sc.Config, &sh)
	if err != nil {
		log.Error("Couldn't decode string", sc.Config, "into toml.")
		return false
	}
	return sh.SingleHost
}
