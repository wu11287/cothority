package status

import (
	"testing"

	"cothority/log"

	"github.com/dedis/cothority/protocols/example/channels"
	"github.com/dedis/cothority/sda"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestServiceStatus(t *testing.T) {
	local := sda.NewLocalTest()
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, el, tr := local.GenTree(5, false, true, false)
	defer local.CloseAll()

	// Send a request to the service
	client := NewClient()
	log.Lvl1("Sending request to service...")
	stat, err := client.GetStatus(el.List[0])
	log.Lvl1(el.List[0])
	log.ErrFatal(err)
	log.Lvl1(stat)
	assert.Equal(t, "2", stat.Msg["Status"]["Total"])
	pi, err := local.CreateProtocol("ExampleChannels", tr)
	if err != nil {
		t.Fatal("Couldn't start protocol:", err)
	}
	go pi.Start()
	<-pi.(*channels.ProtocolExampleChannels).ChildCount
	stat, err = client.GetStatus(el.List[0])
	log.ErrFatal(err)
	log.Lvl1(stat)
	assert.Equal(t, "4", stat.Msg["Status"]["Total"])
}
