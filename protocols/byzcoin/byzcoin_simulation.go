package byzcoin

import (
	"errors"
	"sync"

	"cothority/log"

	"cothority/monitor"

	"cothority/sda"

	"cothority/protocols/byzcoin/blockchain"
	"cothority/protocols/byzcoin/cosi"
	"cothority/protocols/manage"

	"github.com/BurntSushi/toml"
	"github.com/dedis/crypto/abstract"
)

func init() {
	sda.SimulationRegister("ByzCoin", NewSimulation)
	sda.ProtocolRegisterName("ByzCoin", func(n *sda.TreeNodeInstance) (sda.ProtocolInstance, error) {
		return NewByzCoinProtocol(n)
	})
}

// Simulation implements da.Simulation interface
type Simulation struct {
	// sda fields:
	sda.SimulationBFTree
	// your simulation specific fields:
	SimulationConfig
}

// SimulationConfig is the config used by the simulation for byzcoin
type SimulationConfig struct {
	// Blocksize is the number of transactions in one block:
	Blocksize int
	// timeout the leader after TimeoutMs milliseconds
	TimeoutMs uint64
	// Fail:
	// 0  do not fail
	// 1 fail by doing nothing
	// 2 fail by sending wrong blocks
	Fail uint
}

// NewSimulation returns a fresh byzcoin simulation out of the toml config
func NewSimulation(config string) (sda.Simulation, error) {
	es := &Simulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements sda.Simulation interface. It checks on the availability
// of the block-file and downloads it if missing. Then the block-file will be
// copied to the simulation-directory
func (e *Simulation) Setup(dir string, hosts []string) (*sda.SimulationConfig, error) {
	err := blockchain.EnsureBlockIsAvailable(dir)
	if err != nil {
		log.Fatal("Couldn't get block:", err)
	}
	sc := &sda.SimulationConfig{}
	e.CreateRoster(sc, hosts, 2000)
	err = e.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

type monitorMut struct {
	*monitor.TimeMeasure
	sync.Mutex
}

func (m *monitorMut) NewMeasure(id string) {
	m.Lock()
	defer m.Unlock()
	m.TimeMeasure = monitor.NewTimeMeasure(id)
}
func (m *monitorMut) Record() {
	m.Lock()
	defer m.Unlock()
	m.TimeMeasure.Record()
	m.TimeMeasure = nil
}

// Run implements sda.Simulation interface
func (e *Simulation) Run(sdaConf *sda.SimulationConfig) error {
	log.Lvl2("Simulation starting with: Rounds=", e.Rounds)
	server := NewByzCoinServer(e.Blocksize, e.TimeoutMs, e.Fail)
	pi, err := sdaConf.Overlay.CreateProtocolSDA("Broadcast", sdaConf.Tree)
	if err != nil {
		return err
	}
	proto, _ := pi.(*manage.Broadcast)
	// channel to notify we are done
	broadDone := make(chan bool)
	proto.RegisterOnDone(func() {
		broadDone <- true
	})
	// ignore error on purpose: Broadcast.Start() always returns nil
	_ = proto.Start()
	// wait
	<-broadDone

	for round := 0; round < e.Rounds; round++ {
		client := NewClient(server)
		err := client.StartClientSimulation(blockchain.GetBlockDir(), e.Blocksize)
		if err != nil {
			log.Error("Error in ClientSimulation:", err)
			return err
		}

		log.Lvl1("Starting round", round)
		// create an empty node
		tni := sdaConf.Overlay.NewTreeNodeInstanceFromProtoName(sdaConf.Tree, "ByzCoin")
		if err != nil {
			return err
		}
		// instantiate a byzcoin protocol
		rComplete := monitor.NewTimeMeasure("round")
		pi, err := server.Instantiate(tni)
		if err != nil {
			return err
		}
		sdaConf.Overlay.RegisterProtocolInstance(pi)

		bz := pi.(*ByzCoin)
		// Register callback for the generation of the signature !
		bz.RegisterOnSignatureDone(func(sig *BlockSignature) {
			if err := verifyBlockSignature(tni.Suite(), tni.Roster().Aggregate, sig); err != nil {
				log.Error("Round", round, "failed:", err)
			} else {
				log.Lvl2("Round", round, "success")
			}

		})

		// Register when the protocol is finished (all the nodes have finished)
		done := make(chan bool)
		bz.RegisterOnDone(func() {
			done <- true
		})
		if e.Fail > 0 {
			go func() {
				err := bz.startAnnouncementPrepare()
				if err != nil {
					log.Error("Error while starting "+
						"announcment prepare:", err)
				}
			}()
			// do not run bz.startAnnouncementCommit()
		} else {
			go func() {
				if err := bz.Start(); err != nil {
					log.Error("Couldn't start protocol",
						err)
				}
			}()
		}
		// wait for the end
		<-done
		log.Lvl3("Round", round, "finished")
		rComplete.Record()

	}
	return nil
}

func verifyBlockSignature(suite abstract.Suite, aggregate abstract.Point, sig *BlockSignature) error {
	if sig == nil || sig.Sig == nil || sig.Block == nil {
		return errors.New("Empty block signature")
	}
	marshalled := sig.Block.HashSum()
	return cosi.VerifySignatureWithException(suite, aggregate, marshalled, sig.Sig.Challenge, sig.Sig.Response, sig.Exceptions)
}
