package sda

import (
	"errors"
	"os"
	"strconv"
	"testing"

	"io/ioutil"

	"cothority/log"
)

func TestSimulationBF(t *testing.T) {
	sc, _, err := createBFTree(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	addresses := []string{
		"local1:2000", "local2:2000",
		"local1:2001", "local2:2001",
		"local1:2002", "local2:2002",
		"local1:2003",
	}
	for i, a := range sc.Roster.List {
		if a.Addresses[0] != addresses[i] {
			t.Fatal("Address", a.Addresses[0], "should be", addresses[i])
		}
	}
	if !sc.Tree.IsBinary(sc.Tree.Root) {
		t.Fatal("Created tree is not binary")
	}

	sc, _, err = createBFTree(13, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Tree.Root.Children) != 3 {
		t.Fatal("Branching-factor 3 tree has not 3 children")
	}
	if !sc.Tree.IsNary(sc.Tree.Root, 3) {
		t.Fatal("Created tree is not binary")
	}
}

func TestBigTree(t *testing.T) {
	for i := uint(12); i < 15; i++ {
		_, _, err := createBFTree(1<<i-1, 2)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadSave(t *testing.T) {
	sc, _, err := createBFTree(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := ioutil.TempDir("", "example")
	log.ErrFatal(err)
	defer os.RemoveAll(dir)
	sc.Save(dir)
	sc2, err := LoadSimulationConfig(dir, "local1:2000")
	if err != nil {
		t.Fatal(err)
	}
	if sc2[0].Tree.ID != sc.Tree.ID {
		t.Fatal("Tree-id is not correct")
	}
}

func TestMultipleInstances(t *testing.T) {
	sc, _, err := createBFTree(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := ioutil.TempDir("", "example")
	log.ErrFatal(err)
	defer os.RemoveAll(dir)
	sc.Save(dir)
	sc2, err := LoadSimulationConfig(dir, "local1")
	if err != nil {
		t.Fatal(err)
	}
	if len(sc2) != 4 {
		t.Fatal("We should have 4 local1-hosts but have", len(sc2))
	}
	if sc2[0].Host.ServerIdentity.ID == sc2[1].Host.ServerIdentity.ID {
		t.Fatal("Hosts are not copies")
	}
}

func createBFTree(hosts, bf int) (*SimulationConfig, *SimulationBFTree, error) {
	sc := &SimulationConfig{}
	sb := &SimulationBFTree{
		Hosts: hosts,
		BF:    bf,
	}
	sb.CreateRoster(sc, []string{"local1", "local2"}, 2000)
	if len(sc.Roster.List) != hosts {
		return nil, nil, errors.New("Didn't get correct number of entities")
	}
	err := sb.CreateTree(sc)
	if err != nil {
		return nil, nil, err
	}
	if !sc.Tree.IsNary(sc.Tree.Root, bf) {
		return nil, nil, errors.New("Tree isn't " + strconv.Itoa(bf) + "-ary")
	}

	return sc, sb, nil
}
