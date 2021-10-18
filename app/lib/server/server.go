package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"cothority/log"
	"github.com/dedis/cothority/app/lib/config"
	"github.com/dedis/cothority/crypto"
	"github.com/dedis/cothority/network"
	"github.com/dedis/cothority/sda"

	"regexp"

	// Empty imports to have the init-functions called which should
	// register the protocol
	"github.com/dedis/crypto/cosi"
	// For the moment, the server only serves CoSi requests
	_ "github.com/dedis/cosi/protocol"
	s "github.com/dedis/cosi/service"
	"github.com/dedis/crypto/abstract"
	crypconf "github.com/dedis/crypto/config"
)

// DefaultServerConfig is the name of the default file to lookup for server
// configuration file
const DefaultServerConfig = "config.toml"

// DefaultGroupFile is the name of the default file to lookup for group
// definition
const DefaultGroupFile = "group.toml"

// DefaultPort where to listen; At time of writing, this port is not listed in
// /etc/services
const DefaultPort = 6879

// DefaultAddress where to be contacted by other servers
const DefaultAddress = "127.0.0.1"

// Service used to get the port connection service
const whatsMyIP = "http://www.whatsmyip.org/"

// RequestTimeOut is how long we're willing to wait for a signature
var RequestTimeOut = time.Second * 1

// InteractiveConfig will ask through the command line to create a Private / Public
// key, what is the listening address
func InteractiveConfig(binaryName string) {
	log.Info("Setting up a cothority-server.")
	str := config.Inputf(strconv.Itoa(DefaultPort), "Please enter the [address:]PORT for incoming requests")
	// let's dissect the port / IP
	var hostStr string
	var ipProvided = true
	var portStr string
	var serverBinding string
	if !strings.Contains(str, ":") {
		str = ":" + str
	}
	host, port, err := net.SplitHostPort(str)
	log.ErrFatal(err, "Couldn't interpret", str)

	if str == "" {
		portStr = strconv.Itoa(DefaultPort)
		hostStr = "0.0.0.0"
		ipProvided = false
	} else if host == "" {
		// one element provided
		// ip
		ipProvided = false
		hostStr = "0.0.0.0"
		portStr = port
	} else {
		hostStr = host
		portStr = port
	}

	serverBinding = hostStr + ":" + portStr
	if net.ParseIP(hostStr) == nil {
		log.Fatal("Invalid connection  information for", serverBinding)
	}

	log.Info("We now need to get a reachable address for other CoSi servers")
	log.Info("and clients to contact you. This address will be put in a group definition")
	log.Info("file that you can share and combine with others to form a Cothority roster.")

	var publicAddress string
	var failedPublic bool
	// if IP was not provided then let's get the public IP address
	if !ipProvided {
		resp, err := http.Get("http://myexternalip.com/raw")
		// cant get the public ip then ask the user for a reachable one
		if err != nil {
			log.Error("Could not get your public IP address")
			failedPublic = true
		} else {
			buff, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				log.Error("Could not parse your public IP address", err)
				failedPublic = true
			} else {
				publicAddress = strings.TrimSpace(string(buff)) + ":" + portStr
			}
		}
	} else {
		publicAddress = serverBinding
	}

	// Let's directly ask the user for a reachable address
	if failedPublic {
		publicAddress = askReachableAddress(portStr)
	} else {
		if isPublicIP(publicAddress) {
			// try  to connect to ipfound:portgiven
			tryIP := publicAddress
			log.Info("Check if the address", tryIP, "is reachable from Internet...")
			if err := tryConnect(tryIP, serverBinding); err != nil {
				log.Error("Could not connect to your public IP")
				publicAddress = askReachableAddress(portStr)
			} else {
				publicAddress = tryIP
				log.Info("Address", publicAddress, "is publicly available from Internet.")
			}
		}
	}

	// create the keys
	privStr, pubStr := createKeyPair()
	conf := &config.CothoritydConfig{
		Public:    pubStr,
		Private:   privStr,
		Addresses: []string{serverBinding},
	}

	var configDone bool
	var configFolder string
	var defaultFolder = path.Dir(getDefaultConfigFile(binaryName))
	var configFile string
	var groupFile string

	for !configDone {
		// get name of config file and write to config file
		configFolder = config.Input(defaultFolder, "Please enter a folder for the configuration files")
		configFile = path.Join(configFolder, DefaultServerConfig)
		groupFile = path.Join(configFolder, DefaultGroupFile)

		// check if the directory exists
		if _, err := os.Stat(configFolder); os.IsNotExist(err) {
			log.Info("Creating inexistant directory configuration", configFolder)
			if err = os.MkdirAll(configFolder, 0744); err != nil {
				log.Fatalf("Could not create directory configuration %s %v", configFolder, err)
			}
		}

		if checkOverwrite(configFile) && checkOverwrite(groupFile) {
			break
		}
	}

	public, err := crypto.ReadPubHex(network.Suite, pubStr)
	if err != nil {
		log.Fatal("Impossible to parse public key:", err)
	}

	server := config.NewServerToml(network.Suite, public, publicAddress)
	group := config.NewGroupToml(server)

	saveFiles(conf, configFile, group, groupFile)
	log.Info("All configurations saved, ready to serve signatures now.")
}

// CheckConfig contacts all servers and verifies if it receives a valid
// signature from each.
func CheckConfig(tomlFileName string) error {
	f, err := os.Open(tomlFileName)
	log.ErrFatal(err, "Couldn't open group definition file")
	group, err := config.ReadGroupDescToml(f)
	log.ErrFatal(err, "Error while reading group definition file", err)
	if len(group.Roster.List) == 0 {
		log.ErrFatalf(err, "Empty entity or invalid group defintion in: %s",
			tomlFileName)
	}
	log.Info("Checking the availability and responsiveness of the servers in the group...")
	return CheckServers(group)
}

// CheckServers contacts all servers in the entity-list and then makes checks
// on each pair. If 'descs' is 'nil', it doesn't print the description.
func CheckServers(g *config.Group) error {
	success := true
	// First check all servers individually
	for _, e := range g.Roster.List {
		desc := []string{"none", "none"}
		if d := g.GetDescription(e); d != "" {
			desc = []string{d, d}
		}
		el := sda.NewRoster([]*network.ServerIdentity{e})
		success = success && checkList(el, desc) == nil
	}
	if len(g.Roster.List) > 1 {
		// Then check pairs of servers
		for i, first := range g.Roster.List {
			for _, second := range g.Roster.List[i+1:] {
				desc := []string{"none", "none"}
				if d1 := g.GetDescription(first); d1 != "" {
					desc = []string{d1, g.GetDescription(second)}
				}
				es := []*network.ServerIdentity{first, second}
				success = success && checkList(sda.NewRoster(es), desc) == nil
				es[0], es[1] = es[1], es[0]
				desc[0], desc[1] = desc[1], desc[0]
				success = success && checkList(sda.NewRoster(es), desc) == nil
			}
		}
	}

	if !success {
		return errors.New("At least one of the tests failed")
	}
	return nil
}

// checkList sends a message to the list and waits for the reply
func checkList(list *sda.Roster, descs []string) error {
	serverStr := ""
	for i, s := range list.List {
		name := strings.Split(descs[i], " ")[0]
		serverStr += fmt.Sprintf("%s_%s ", s.Addresses[0], name)
	}
	log.Lvl3("Sending message to: " + serverStr)
	msg := "verification"
	log.Info("Checking server(s) ", serverStr, ": ")
	sig, err := signStatement(strings.NewReader(msg), list)
	if err != nil {
		log.Error(err)
		return err
	}
	err = verifySignatureHash([]byte(msg), sig, list)
	if err != nil {
		log.Errorf("Invalid signature: %v", err)
		return err
	}
	log.Info("Success")
	return nil
}

// signStatement can be used to sign the contents passed in the io.Reader
// (pass an io.File or use an strings.NewReader for strings)
func signStatement(read io.Reader, el *sda.Roster) (*s.SignatureResponse,
	error) {
	//publics := entityListToPublics(el)
	client := s.NewClient()
	msg, _ := crypto.HashStream(network.Suite.Hash(), read)

	pchan := make(chan *s.SignatureResponse)
	var err error
	go func() {
		log.Lvl3("Waiting for the response on SignRequest")
		response, e := client.SignMsg(el, msg)
		if e != nil {
			err = e
			close(pchan)
			return
		}
		pchan <- response
	}()

	select {
	case response, ok := <-pchan:
		log.Lvl5("Response:", response)
		if !ok || err != nil {
			return nil, errors.New("Received an invalid repsonse.")
		}
		err = cosi.VerifySignature(network.Suite, el.Publics(), msg, response.Signature)
		if err != nil {
			return nil, err
		}
		return response, nil
	case <-time.After(RequestTimeOut):
		return nil, errors.New("timeout on signing request")
	}
}

func verifySignatureHash(b []byte, sig *s.SignatureResponse, el *sda.Roster) error {
	// We have to hash twice, as the hash in the signature is the hash of the
	// message sent to be signed
	//publics := entityListToPublics(el)
	fHash, _ := crypto.HashBytes(network.Suite.Hash(), b)
	hashHash, _ := crypto.HashBytes(network.Suite.Hash(), fHash)
	if !bytes.Equal(hashHash, sig.Sum) {
		return errors.New("You are trying to verify a signature " +
			"belonging to another file. (The hash provided by the signature " +
			"doesn't match with the hash of the file.)")
	}
	err := cosi.VerifySignature(network.Suite, el.Publics(), fHash, sig.Signature)
	if err != nil {
		return errors.New("Invalid sig:" + err.Error())
	}
	return nil
}
func entityListToPublics(el *sda.Roster) []abstract.Point {
	publics := make([]abstract.Point, len(el.List))
	for i, e := range el.List {
		publics[i] = e.Public
	}
	return publics
}

func isPublicIP(ip string) bool {
	public, err := regexp.MatchString("(^127\\.)|(^10\\.)|"+
		"(^172\\.1[6-9]\\.)|(^172\\.2[0-9]\\.)|"+
		"(^172\\.3[0-1]\\.)|(^192\\.168\\.)", ip)
	if err != nil {
		log.Error(err)
	}
	return !public
}

// Returns true if file exists and user is OK to overwrite, or file dont exists
// Return false if file exists and user is NOT OK to overwrite.
func checkOverwrite(file string) bool {
	// check if the file exists and ask for override
	if _, err := os.Stat(file); err == nil {
		return config.InputYN(true, "Configuration file "+file+" already exists. Override?")
	}
	return true
}

// createKeyPair returns the private and public key hexadecimal representation
func createKeyPair() (string, string) {
	log.Info("Creating ed25519 private and public keys.")
	kp := crypconf.NewKeyPair(network.Suite)
	privStr, err := crypto.ScalarHex(network.Suite, kp.Secret)
	if err != nil {
		log.Fatal("Error formating private key to hexadecimal. Abort.")
	}
	var point abstract.Point
	// use the transformation for EdDSA signatures
	//point = cosi.Ed25519Public(network.Suite, kp.Secret)
	point = kp.Public
	pubStr, err := crypto.PubHex(network.Suite, point)
	if err != nil {
		log.Fatal("Could not parse public key. Abort.")
	}

	log.Info("Public key: ", pubStr, "\n")
	return privStr, pubStr
}

func saveFiles(conf *config.CothoritydConfig, fileConf string, group *config.GroupToml, fileGroup string) {
	if err := conf.Save(fileConf); err != nil {
		log.Fatal("Unable to write the config to file:", err)
	}
	log.Info("Sucess! You can now use the CoSi server with the config file", fileConf)
	// group definition part
	if err := group.Save(fileGroup); err != nil {
		log.Fatal("Could not write your group file snippet:", err)
	}

	log.Info("Saved a group definition snippet for your server at", fileGroup,
		group.String())

}

func getDefaultConfigFile(binaryName string) string {
	u, err := user.Current()
	// can't get the user dir, so fallback to current working dir
	if err != nil {
		log.Error("Could not get your home-directory (", err.Error(), "). Switching back to current dir.")
		if curr, err := os.Getwd(); err != nil {
			log.Fatal("Impossible to get the current directory:", err)
		} else {
			return path.Join(curr, DefaultServerConfig)
		}
	}
	// let's try to stick to usual OS folders
	switch runtime.GOOS {
	case "darwin":
		return path.Join(u.HomeDir, "Library", binaryName, DefaultServerConfig)
	default:
		return path.Join(u.HomeDir, ".config", binaryName, DefaultServerConfig)
		// TODO WIndows ? FreeBSD ?
	}
}

func askReachableAddress(port string) string {
	ipStr := config.Input(DefaultAddress, "IP-address where your server can be reached")

	splitted := strings.Split(ipStr, ":")
	if len(splitted) == 2 && splitted[1] != port {
		// if the client gave a port number, it must be the same
		log.Fatal("The port you gave is not the same as the one your server will be listening. Abort.")
	} else if len(splitted) == 2 && net.ParseIP(splitted[0]) == nil {
		// of if the IP address is wrong
		log.Fatal("Invalid IP:port address given:", ipStr)
	} else if len(splitted) == 1 {
		// check if the ip is valid
		if net.ParseIP(ipStr) == nil {
			log.Fatal("Invalid IP address given:", ipStr)
		}
		// add the port
		ipStr = ipStr + ":" + port
	}
	return ipStr
}

// tryConnect will bind to the ip address and ask a internet service to try to
// connect to it. binding is the address where we must listen (needed because
// the reachable address might not be the same as the binding address => NAT, ip
// rules etc).
func tryConnect(ip string, binding string) error {

	stopCh := make(chan bool, 1)
	// let's bind
	go func() {
		ln, err := net.Listen("tcp", binding)
		if err != nil {
			log.Error("Trouble with binding to the address:", err)
			return
		}
		con, _ := ln.Accept()
		<-stopCh
		con.Close()
	}()
	defer func() { stopCh <- true }()

	_, port, err := net.SplitHostPort(ip)
	if err != nil {
		return err
	}
	values := url.Values{}
	values.Set("port", port)
	values.Set("timeout", "default")

	// ask the check
	url := whatsMyIP + "port-scanner/scan.php"
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Host", "www.whatsmyip.org")
	req.Header.Set("Referer", "http://www.whatsmyip.org/port-scanner/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:46.0) Gecko/20100101 Firefox/46.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	buffer, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if !bytes.Contains(buffer, []byte("1")) {
		return errors.New("Address unreachable")
	}
	return nil
}
