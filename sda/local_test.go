package sda

import (
	"testing"

	"cothority/log"
)

func TestGenLocalHost(t *testing.T) {
	l := NewLocalTest()
	hosts := l.GenLocalHosts(2, false, false)
	defer l.CloseAll()

	log.Lvl4("Hosts are:", hosts[0].Address(), hosts[1].Address())
	if hosts[0].Address() == hosts[1].Address() {
		t.Fatal("Both addresses are equal")
	}
}
