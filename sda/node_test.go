package sda

import (
	"testing"
	"time"

	"cothority/log"

	"github.com/dedis/cothority/network"
)

func init() {
	ProtocolRegisterName("ProtocolHandlers", NewProtocolHandlers)
	ProtocolRegisterName("ProtocolBlocking", NewProtocolBlocking)
	ProtocolRegisterName("ProtocolChannels", NewProtocolChannels)
	ProtocolRegisterName(testProto, NewProtocolTest)
	Incoming = make(chan struct {
		*TreeNode
		NodeTestMsg
	})
}

func TestNodeChannelCreateSlice(t *testing.T) {
	local := NewLocalTest()
	_, _, tree := local.GenTree(2, false, true, true)
	defer local.CloseAll()

	p, err := local.CreateProtocol("ProtocolChannels", tree)
	if err != nil {
		t.Fatal("Couldn't create new node:", err)
	}

	var c chan []struct {
		*TreeNode
		NodeTestMsg
	}
	tni := p.(*ProtocolChannels).TreeNodeInstance
	err = tni.RegisterChannel(&c)
	if err != nil {
		t.Fatal("Couldn't register channel:", err)
	}
}

func TestNodeChannelCreate(t *testing.T) {
	local := NewLocalTest()
	_, _, tree := local.GenTree(2, false, true, true)
	defer local.CloseAll()

	p, err := local.CreateProtocol("ProtocolChannels", tree)
	if err != nil {
		t.Fatal("Couldn't create new node:", err)
	}
	var c chan struct {
		*TreeNode
		NodeTestMsg
	}
	tni := p.(*ProtocolChannels).TreeNodeInstance
	err = tni.RegisterChannel(&c)
	if err != nil {
		t.Fatal("Couldn't register channel:", err)
	}
	err = tni.DispatchChannel([]*ProtocolMsg{&ProtocolMsg{
		Msg:     NodeTestMsg{3},
		MsgType: network.RegisterPacketType(NodeTestMsg{}),
		From: &Token{
			TreeID:     tree.ID,
			TreeNodeID: tree.Root.ID,
		}},
	})
	if err != nil {
		t.Fatal("Couldn't dispatch to channel:", err)
	}
	msg := <-c
	if msg.I != 3 {
		t.Fatal("Message should contain '3'")
	}
}

func TestNodeChannel(t *testing.T) {
	local := NewLocalTest()
	_, _, tree := local.GenTree(2, false, true, true)
	defer local.CloseAll()

	p, err := local.CreateProtocol("ProtocolChannels", tree)
	if err != nil {
		t.Fatal("Couldn't create new node:", err)
	}
	c := make(chan struct {
		*TreeNode
		NodeTestMsg
	}, 1)
	tni := p.(*ProtocolChannels).TreeNodeInstance
	err = tni.RegisterChannel(c)
	if err != nil {
		t.Fatal("Couldn't register channel:", err)
	}
	err = tni.DispatchChannel([]*ProtocolMsg{&ProtocolMsg{
		Msg:     NodeTestMsg{3},
		MsgType: network.RegisterPacketType(NodeTestMsg{}),
		From: &Token{
			TreeID:     tree.ID,
			TreeNodeID: tree.Root.ID,
		}},
	})
	if err != nil {
		t.Fatal("Couldn't dispatch to channel:", err)
	}
	msg := <-c
	if msg.I != 3 {
		t.Fatal("Message should contain '3'")
	}
}

// Test instantiation of Node
func TestNewNode(t *testing.T) {
	h1, h2 := SetupTwoHosts(t, false)
	// Add tree + entitylist
	el := NewRoster([]*network.ServerIdentity{h1.ServerIdentity, h2.ServerIdentity})
	h1.AddRoster(el)
	tree := el.GenerateBinaryTree()
	h1.AddTree(tree)

	// Try directly StartNewNode
	proto, err := h1.StartProtocol(testProto, tree)
	if err != nil {
		t.Fatal("Could not start new protocol", err)
	}
	p := proto.(*ProtocolTest)
	m := <-p.DispMsg
	if m != "Dispatch" {
		t.Fatal("Dispatch() not called - msg is:", m)
	}
	m = <-p.StartMsg
	if m != "Start" {
		t.Fatal("Start() not called - msg is:", m)
	}
	h1.Close()
	h2.Close()
}

func TestServiceChannels(t *testing.T) {
	sc1 := &ServiceChannels{}
	sc2 := &ServiceChannels{}
	var count int
	RegisterNewService("ChannelsService", func(c *Context, path string) Service {
		var sc *ServiceChannels
		if count == 0 {
			sc = sc1
		} else {
			sc = sc2
		}
		count++
		sc.ctx = c
		sc.path = path
		return sc
	})
	h1, h2 := SetupTwoHosts(t, true)
	defer h1.Close()
	defer h2.Close()
	// Add tree + entitylist
	el := NewRoster([]*network.ServerIdentity{h1.ServerIdentity, h2.ServerIdentity})
	tree := el.GenerateBinaryTree()
	sc1.tree = *tree
	h1.AddRoster(el)
	h1.AddTree(tree)
	h1.StartProcessMessages()

	sc1.ProcessClientRequest(nil, nil)
	select {
	case msg := <-Incoming:
		if msg.I != 12 {
			t.Fatal("Child should receive 12")
		}
	case <-time.After(time.Second * 3):
		t.Fatal("Timeout")
	}
}

func TestProtocolHandlers(t *testing.T) {
	local := NewLocalTest()
	_, _, tree := local.GenTree(3, false, true, true)
	defer local.CloseAll()
	log.Lvl2("Sending to children")
	IncomingHandlers = make(chan *TreeNodeInstance, 2)
	p, err := local.CreateProtocol("ProtocolHandlers", tree)
	if err != nil {
		t.Fatal(err)
	}
	go p.Start()
	log.Lvl2("Waiting for responses")
	child1 := <-IncomingHandlers
	child2 := <-IncomingHandlers

	if child1.ServerIdentity().ID == child2.ServerIdentity().ID {
		t.Fatal("Both entities should be different")
	}

	log.Lvl2("Sending to parent")

	tni := p.(*ProtocolHandlers).TreeNodeInstance
	child1.SendTo(tni.TreeNode(), &NodeTestAggMsg{})
	if len(IncomingHandlers) > 0 {
		t.Fatal("This should not trigger yet")
	}
	child2.SendTo(tni.TreeNode(), &NodeTestAggMsg{})
	final := <-IncomingHandlers
	if final.ServerIdentity().ID != tni.ServerIdentity().ID {
		t.Fatal("This should be the same ID")
	}
}

func TestMsgAggregation(t *testing.T) {
	local := NewLocalTest()
	_, _, tree := local.GenTree(3, false, true, true)
	defer local.CloseAll()
	root, err := local.StartProtocol("ProtocolChannels", tree)
	if err != nil {
		t.Fatal("Couldn't create new node:", err)
	}
	proto := root.(*ProtocolChannels)
	// Wait for both children to be up
	<-Incoming
	<-Incoming
	log.Lvl3("Both children are up")
	child1 := local.GetNodes(tree.Root.Children[0])[0]
	child2 := local.GetNodes(tree.Root.Children[1])[0]

	err = local.SendTreeNode("ProtocolChannels", child1, proto.TreeNodeInstance, &NodeTestAggMsg{3})
	if err != nil {
		t.Fatal(err)
	}
	if len(proto.IncomingAgg) > 0 {
		t.Fatal("Messages should NOT be there")
	}
	err = local.SendTreeNode("ProtocolChannels", child2, proto.TreeNodeInstance, &NodeTestAggMsg{4})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case msgs := <-proto.IncomingAgg:
		if msgs[0].I != 3 {
			t.Fatal("First message should be 3")
		}
		if msgs[1].I != 4 {
			t.Fatal("Second message should be 4")
		}
	case <-time.After(time.Second):
		t.Fatal("Messages should BE there")
	}
}

func TestFlags(t *testing.T) {
	testType := network.PacketTypeID(uuid.Nil)
	local := NewLocalTest()
	_, _, tree := local.GenTree(3, false, false, true)
	defer local.CloseAll()
	p, err := local.CreateProtocol("ProtocolChannels", tree)
	if err != nil {
		t.Fatal("Couldn't create node.")
	}
	tni := p.(*ProtocolChannels).TreeNodeInstance
	if tni.HasFlag(testType, AggregateMessages) {
		t.Fatal("Should NOT have AggregateMessages-flag")
	}
	tni.SetFlag(testType, AggregateMessages)
	if !tni.HasFlag(testType, AggregateMessages) {
		t.Fatal("Should HAVE AggregateMessages-flag cleared")
	}
	tni.ClearFlag(testType, AggregateMessages)
	if tni.HasFlag(testType, AggregateMessages) {
		t.Fatal("Should NOT have AggregateMessages-flag")
	}
}

// Protocol/service Channels test code:
type NodeTestMsg struct {
	I int
}

var Incoming chan struct {
	*TreeNode
	NodeTestMsg
}

type NodeTestAggMsg struct {
	I int
}

type ProtocolChannels struct {
	*TreeNodeInstance
	IncomingAgg chan []struct {
		*TreeNode
		NodeTestAggMsg
	}
}

func NewProtocolChannels(n *TreeNodeInstance) (ProtocolInstance, error) {
	p := &ProtocolChannels{
		TreeNodeInstance: n,
	}
	p.RegisterChannel(Incoming)
	p.RegisterChannel(&p.IncomingAgg)
	return p, nil
}

func (p *ProtocolChannels) Start() error {
	for _, c := range p.Children() {
		err := p.SendTo(c, &NodeTestMsg{12})
		if err != nil {
			return err
		}
	}
	return nil
}

// release resources ==> call Done()
func (p *ProtocolChannels) Release() {
	p.Done()
}

type ServiceChannels struct {
	ctx  *Context
	path string
	tree Tree
}

// implement services interface
func (c *ServiceChannels) ProcessClientRequest(si *network.ServerIdentity, r *ClientRequest) {

	tni := c.ctx.NewTreeNodeInstance(&c.tree, c.tree.Root, "ProtocolChannels")
	pi, err := NewProtocolChannels(tni)
	if err != nil {
		return
	}

	if err := c.ctx.RegisterProtocolInstance(pi); err != nil {
		return
	}
	pi.Start()
}

func (c *ServiceChannels) NewProtocol(tn *TreeNodeInstance, conf *GenericConfig) (ProtocolInstance, error) {
	log.Lvl1("Cosi Service received New Protocol event")
	return NewProtocolChannels(tn)
}

func (c *ServiceChannels) Process(packet *network.Packet) {
	return
}

// End: protocol/service channels

type ProtocolHandlers struct {
	*TreeNodeInstance
}

var IncomingHandlers chan *TreeNodeInstance

func NewProtocolHandlers(n *TreeNodeInstance) (ProtocolInstance, error) {
	p := &ProtocolHandlers{
		TreeNodeInstance: n,
	}
	p.RegisterHandler(p.HandleMessageOne)
	p.RegisterHandler(p.HandleMessageAggregate)
	return p, nil
}

func (p *ProtocolHandlers) Start() error {
	for _, c := range p.Children() {
		err := p.SendTo(c, &NodeTestMsg{12})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *ProtocolHandlers) HandleMessageOne(msg struct {
	*TreeNode
	NodeTestMsg
}) {
	IncomingHandlers <- p.TreeNodeInstance
}

func (p *ProtocolHandlers) HandleMessageAggregate(msg []struct {
	*TreeNode
	NodeTestAggMsg
}) {
	log.Lvl3("Received message")
	IncomingHandlers <- p.TreeNodeInstance
}

func (p *ProtocolHandlers) Dispatch() error {
	return nil
}

// release resources ==> call Done()
func (p *ProtocolHandlers) Release() {
	p.Done()
}

func TestBlocking(t *testing.T) {
	l := NewLocalTest()
	_, _, tree := l.GenTree(2, true, true, true)
	defer l.CloseAll()

	n1, err := l.StartProtocol("ProtocolBlocking", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol")
	}
	n2, err := l.StartProtocol("ProtocolBlocking", tree)
	if err != nil {
		t.Fatal("Couldn't start protocol")
	}

	p1 := n1.(*BlockingProtocol)
	p2 := n2.(*BlockingProtocol)
	tn1 := p1.TreeNodeInstance
	tn2 := p2.TreeNodeInstance
	go func() {
		// Send two messages to n1, which blocks the old interface
		err := l.SendTreeNode("", tn2, tn1, &NodeTestMsg{})
		if err != nil {
			t.Fatal("Couldn't send message:", err)
		}
		err = l.SendTreeNode("", tn2, tn1, &NodeTestMsg{})
		if err != nil {
			t.Fatal("Couldn't send message:", err)
		}
		// Now send a message to n2, but in the old interface this
		// blocks.
		err = l.SendTreeNode("", tn1, tn2, &NodeTestMsg{})
		if err != nil {
			t.Fatal("Couldn't send message:", err)
		}
	}()
	// Release p2
	p2.stopBlockChan <- true
	select {
	case <-p2.doneChan:
		log.Lvl2("Node 2 done")
		p1.stopBlockChan <- true
		<-p1.doneChan
	case <-time.After(time.Second):
		t.Fatal("Node 2 didn't receive")
	}
}

// BlockingProtocol is a protocol that will block until it receives a "continue"
// signal on the continue channel. It is used for testing the asynchronous
// & non blocking handling of the messages in
type BlockingProtocol struct {
	*TreeNodeInstance
	// the protocol will signal on this channel that it is done
	doneChan chan bool
	// stopBLockChan is used to signal the protocol to stop blocking the
	// incoming messages on the Incoming chan
	stopBlockChan chan bool
	Incoming      chan struct {
		*TreeNode
		NodeTestMsg
	}
}

func NewProtocolBlocking(node *TreeNodeInstance) (ProtocolInstance, error) {
	bp := &BlockingProtocol{
		TreeNodeInstance: node,
		doneChan:         make(chan bool),
		stopBlockChan:    make(chan bool),
	}

	node.RegisterChannel(&bp.Incoming)
	return bp, nil
}

func (bp *BlockingProtocol) Start() error {
	return nil
}

func (bp *BlockingProtocol) Dispatch() error {
	// first wait on stopBlockChan
	<-bp.stopBlockChan
	log.Lvl2("BlockingProtocol: will continue")
	// Then wait on the actual message
	<-bp.Incoming
	log.Lvl2("BlockingProtocol: received message => signal Done")
	// then signal that you are done
	bp.doneChan <- true
	return nil
}
