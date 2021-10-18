package network

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	"strconv"

	"cothority/log"

	"github.com/dedis/crypto/abstract"
	"github.com/dedis/crypto/config"
)

type PublicPacket struct {
	Point abstract.Point
}

// Here we registers the packets, so that the decoder can instantiate
// to the right type and then we can do event-driven stuff such as receiving
// new messages without knowing the type and then check on the MsgType field
// to cast to the right packet type (See below)
var PublicType = RegisterPacketType(PublicPacket{})

func init() {
	RegisterPacketType(stressMsg{})
}

type TestRegisterS struct {
	I int
}

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestRegister(t *testing.T) {
	if TypeFromData(&TestRegisterS{}) != ErrorType {
		t.Fatal("TestRegister should not yet be there")
	}

	trType := RegisterPacketType(&TestRegisterS{})
	if uuid.Equal(uuid.UUID(trType), uuid.Nil) {
		t.Fatal("Couldn't register TestRegister-struct")
	}

	if TypeFromData(&TestRegisterS{}) != trType {
		t.Fatal("TestRegister is different now")
	}
	if TypeFromData(TestRegisterS{}) != trType {
		t.Fatal("TestRegister is different now")
	}
}

func TestRegisterReflect(t *testing.T) {
	typ := RegisterPacketType(TestRegisterS{})
	typReflect := RTypeToPacketTypeID(reflect.TypeOf(TestRegisterS{}))
	if typ != typReflect {
		t.Fatal("Register does not work")
	}
}

// Test closing and opening of Host on same address
func TestMultiClose(t *testing.T) {
	gotConnect := make(chan bool)
	fn := func(s Conn) {
		log.Lvl3("Getting connection from", s)
		gotConnect <- true
	}
	h1 := NewTCPHost()
	h2 := NewTCPHost()
	done := make(chan bool)
	go func() {
		err := h1.Listen("localhost:0", fn)
		if err != nil {
			t.Fatal("Couldn't listen:", err)
		}
		done <- true
	}()
	time.Sleep(time.Second)
	log.Lvl3("Opening connection to h1")
	h1Port := <-h1.listeningPort
	_, err := h2.Open("localhost:" + strconv.Itoa(h1Port))
	if err != nil {
		t.Fatal(h2, "couldn't Open() connection to", h1, err)
	}
	// wait for the listener, then close h1 & h2:
	<-gotConnect
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't Close():", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't Close()", err)
	}
	<-done

	h3 := NewTCPHost()
	go func() {
		err := h3.Listen("localhost:0", fn)
		if err != nil {
			t.Fatal("Couldn't re-open listener:", err)
		}
		done <- true
	}()
	h3Port := <-h3.listeningPort
	_, err = h2.Open("localhost:" + strconv.Itoa(h3Port))
	if err != nil {
		t.Fatal(h2, "couldn't Open() connection to", h3, err)
	}
	// wait for the listener and close h3 & h2
	<-gotConnect
	err = h3.Close()
	if err != nil {
		t.Fatal("Couldn't close h3:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close h2:", err)
	}
	<-done
}

// Test closing and opening of SecureHost on same address
func TestSecureMultiClose(t *testing.T) {
	receiverStarted := make(chan bool)
	fn := func(s SecureConn) {
		log.Lvl3("Getting connection from", s.ServerIdentity().First())
		close(receiverStarted)
	}

	kp1 := config.NewKeyPair(Suite)
	si1 := NewServerIdentity(kp1.Public, "localhost:0")
	kp2 := config.NewKeyPair(Suite)
	si2 := NewServerIdentity(kp2.Public, "localhost:0")

	h1 := NewSecureTCPHost(kp1.Secret, si1)
	h2 := NewSecureTCPHost(kp2.Secret, si2)
	done := make(chan bool)
	go func() {
		err := h1.Listen(fn)
		if err != nil {
			t.Fatal("Listening failed for h1:", err)
		}
		done <- true
	}()

	<-done
	_, err := h2.Open(h1.serverIdentity)
	if err != nil {
		t.Fatal("Couldn't open h2:", err)
	}
	<-receiverStarted
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}

	log.Lvl1("Finished first connection, starting 2nd")
	receiverStarted2 := make(chan bool)
	fn2 := func(s SecureConn) {
		log.Lvl3("Getting connection from", s.ServerIdentity().First())
		receiverStarted2 <- true
	}
	done2 := make(chan bool)
	go func() {
		err := h1.Listen(fn2)
		if err != nil {
			t.Fatal("Couldn't re-open listener:", err)
		}
		done2 <- true
	}()
	_, err = h2.Open(h1.serverIdentity)
	if err != nil {
		t.Fatal(err)
	}

	<-receiverStarted2
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close h1:", err)
	}

	<-done2
}

func TestTcpCounterIO(t *testing.T) {
	RegisterPacketType(&TestRegisterS{})
	receiverStarted := make(chan bool)
	fn := func(s Conn) {
		err := s.Send(context.TODO(), &TestRegisterS{10})
		if err != nil {
			t.Fatal("Error while sending message:", err)
		}
		close(receiverStarted)
	}

	h1 := NewTCPHost()
	h2 := NewTCPHost()
	done := make(chan bool)
	go func() {
		err := h1.Listen("localhost:0", fn)
		if err != nil {
			t.Fatal("Listening failed for h1:", err)
		}
		done <- true
	}()

	c2, err := h2.Open("localhost:" + strconv.Itoa(<-h1.listeningPort))
	if err != nil {
		t.Fatal("Couldn't open h2:", err)
	}
	<-receiverStarted
	c2.Receive(context.TODO())
	err = h1.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	err = h2.Close()
	if err != nil {
		t.Fatal("Couldn't close:", err)
	}
	<-done
	// verify the amount of bytes read / written
	if h1.Tx() == 0 || h1.Tx() != h2.Rx() || h2.Rx() == 0 || h2.Rx() != c2.Rx() {
		t.Fatal("stg is wrong with CounterIO implementation of TcpConn / TcpHost")
	}

}

// Testing exchange of entity
func TestSecureTcp(t *testing.T) {
	opened := make(chan bool)
	fn := func(s SecureConn) {
		log.Lvl3("Getting connection from", s)
		opened <- true
	}

	kp1 := config.NewKeyPair(Suite)
	si1 := NewServerIdentity(kp1.Public, "localhost:0")
	kp2 := config.NewKeyPair(Suite)
	si2 := NewServerIdentity(kp2.Public, "localhost:0")

	host1 := NewSecureTCPHost(kp1.Secret, si1)
	host2 := NewSecureTCPHost(kp1.Secret, si2)

	done := make(chan bool)
	go func() {
		err := host1.Listen(fn)
		if err != nil {
			t.Fatal("Couldn't listen:", err)
		}
		done <- true
	}()
	<-done
	conn, err := host2.Open(host1.serverIdentity)
	if err != nil {
		t.Fatal("Couldn't connect to host1:", err)
	}
	if !conn.ServerIdentity().Public.Equal(kp1.Public) {
		t.Fatal("Connection-id is not from host1")
	}
	if !<-opened {
		t.Fatal("Lazy programmers - no select")
	}
	log.Lvl4("Closing connections")
	if err := host1.Close(); err != nil {
		t.Fatal("Couldn't close host", host1)
	}
	if err := host2.Close(); err != nil {
		t.Fatal("Couldn't close host", host2)
	}
}

// Testing a full-blown server/client
func TestTcpNetwork(t *testing.T) {
	// Create one client + one server
	clientHost := NewTCPHost()
	serverHost := NewTCPHost()
	// Give them keys
	clientPub := Suite.Point().Base()
	serverPub := Suite.Point().Add(Suite.Point().Base(), Suite.Point().Base())
	wg := sync.WaitGroup{}
	client := NewSimpleClient(clientHost, clientPub, &wg)
	server := NewSimpleServer(serverHost, serverPub, t, &wg)
	// Make the server listen
	done := make(chan bool)
	go func() {
		err := server.Listen("127.0.0.1:5000", server.ExchangeWithClient)
		if err != nil {
			t.Fatal("Couldn't listen:", err)
		}
		done <- true
	}()
	// Make the client engage with the server
	client.ExchangeWithServer("127.0.0.1:5000", t)
	wg.Wait()
	if err := clientHost.Close(); err != nil {
		t.Fatal("could not close client", err)
	}
	if err := serverHost.Close(); err != nil {
		t.Fatal("could not close server", err)
	}
	<-done
}

// Opens up a lot of connections and sends packets around. Now this is a test
// that fails on MacOSX but not on Travis!
func TestStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test.")
	}
	wait := 100 * time.Millisecond
	for i := 0; i < 100; i++ {
		log.Lvl1("Going for round:", i)
		stressTest(t, wait, 30, 16)
		stressTest(t, wait, 10, 0)
		stressTest(t, 0, 10, 0)
	}
}

// Will start 'nbrHosts' hosts and either start to send 'loadLen' of bytes
// across the connection, or just directly close the connection if
// 'loadLen' == 0.
// If 'waiting' > 0, it will wait for that time before sending
// and/or closing the connection.
func stressTest(t *testing.T, waiting time.Duration, nbrHosts, loadSize int) {
	log.Lvl1("Testing with waiting:", waiting, "hosts:", nbrHosts,
		"load-size:", loadSize)
	wg := sync.WaitGroup{}
	closeIt := func(s SecureConn) {
		if waiting.Seconds() > 0 {
			log.Lvl2("Waiting to close connection", s)
			time.Sleep(waiting)
		}
		if loadSize > 0 {
			log.Lvl2("Sending something")
			err := s.Send(context.TODO(), &stressMsg{make([]byte, loadSize)})
			log.ErrFatal(err)
			log.Lvl2("Receiving")
			p, err := s.Receive(context.TODO())
			log.ErrFatal(err)
			if len(p.Msg.(stressMsg).Load) != loadSize {
				t.Fatal("Didn't receive enough bytes")
			}
		}
		log.Lvl2("Closing connection", s)
		s.Close()
		wg.Done()
	}
	hosts := make([]*SecureTCPHost, nbrHosts)
	for i := range hosts {
		log.Lvl2("Starting connection", i)
		wg.Add(1)
		go func(nbr int) {
			kp := config.NewKeyPair(Suite)
			si := NewServerIdentity(kp.Public, "localhost:0")
			h := NewSecureTCPHost(kp.Secret, si)
			hosts[nbr] = h
			log.ErrFatal(h.Listen(closeIt))
			wg.Done()
		}(i)
	}
	wg.Wait()
	for i := range hosts {
		wg.Add(1)
		go func(nbr int) {
			log.Lvl2("Opening", nbr, hosts[nbr])
			c, err := hosts[nbr].Open(hosts[(nbr+1)%nbrHosts].serverIdentity)
			log.ErrFatal(err)
			if loadSize > 0 {
				p, err := c.Receive(context.TODO())
				log.ErrFatal(err)
				if len(p.Msg.(stressMsg).Load) != loadSize {
					t.Fatal("Didn't receive", loadSize, "bytes")
				}
				err = c.Send(context.TODO(), &stressMsg{make([]byte, loadSize)})
				log.ErrFatal(err)
			}
		}(i)
	}
	wg.Wait()
	if waiting.Seconds() > 0 {
		time.Sleep(waiting)
	}
	log.Lvl2("Closing hosts")
	for _, h := range hosts {
		log.ErrFatal(h.Close())
		log.Lvl2("Closing", h)
	}
	if waiting.Seconds() > 0 {
		time.Sleep(waiting)
	}
}

type stressMsg struct {
	Load []byte
}

type SimpleClient struct {
	Host
	Pub   abstract.Point
	Peers []abstract.Point
	wg    *sync.WaitGroup
}

// The server
type SimpleServer struct {
	Host
	Pub abstract.Point
	t   *testing.T
	wg  *sync.WaitGroup
}

// Create a new simple server
func NewSimpleServer(host Host, pub abstract.Point, t *testing.T, wg *sync.WaitGroup) *SimpleServer {
	s := &SimpleServer{}
	s.Host = host
	s.Pub = pub
	s.t = t
	s.wg = wg
	return s
}

// Createa a new simple client
func NewSimpleClient(host Host, pub abstract.Point, wg *sync.WaitGroup) *SimpleClient {
	return &SimpleClient{
		Host:  host,
		Pub:   pub,
		Peers: make([]abstract.Point, 0),
		wg:    wg,
	}
}

// overridding Name host
func (s *SimpleClient) Name() string {
	return "Client "
}

// Simplest protocol : exchange keys with the server
func (s *SimpleClient) ExchangeWithServer(name string, t *testing.T) {
	s.wg.Add(1)
	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	// open a connection to the peer
	c, err := s.Open(name)
	if err != nil {
		t.Fatal("client connection is nil ><")
	}
	// create pack
	p := PublicPacket{
		Point: s.Pub,
	}
	// Send it
	err = c.Send(ctx, &p)
	if err != nil {
		t.Fatal("error sending from client:", err)
	}
	// Receive the response
	am, err := c.Receive(ctx)
	if err != nil {
		fmt.Printf("error receiving ..")
	}

	// Cast to the right type
	if am.MsgType != PublicType {
		t.Fatal("Received a non-wanted packet.\n")
	}
	err = c.Close()
	if err != nil {
		t.Fatal("error closing connection", err)
	}

	err = c.Close()
	if err != nil && err != ErrClosed {
		t.Fatal("Couldn't close:", err)
	}
	if err == ErrClosed {
		log.Error("Called Close() on alredy closed connetion.")
	}
	s.wg.Done()
}

func (s *SimpleServer) Name() string {
	return "Server "
}

func (s *SimpleServer) ProxySend(c Conn, msg Body) {
	ctx := context.TODO()
	if err := c.Send(ctx, msg); err != nil {
		s.t.Fatal(err)
	}
}

// this is the callback when a new connection is don
func (s *SimpleServer) ExchangeWithClient(c Conn) {
	s.wg.Add(1)
	p := PublicPacket{
		Point: s.Pub,
	}

	ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
	s.ProxySend(c, &p)
	am, err := c.Receive(ctx)
	if err != nil {
		s.t.Error("Server errored when receiving packet ...\n")
	}
	if am.MsgType != PublicType {
		s.t.Error("Server received a non-wanted packet\n")
	}
	p = (am.Msg).(PublicPacket)
	comp := Suite.Point().Base()
	if !p.Point.Equal(comp) {
		s.t.Error("point not equally reconstructed")
	}
	err = c.Close()
	if err != nil {
		s.t.Fatal("error closing connection", err)
	}

	s.wg.Done()
}
