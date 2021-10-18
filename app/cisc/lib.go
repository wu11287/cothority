package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/ssh"

	"strings"

	"cothority/log"

	"cothority/network"
	"cothority/services/identity"
	"github.com/dedis/cothority/app/lib/config"
	"gopkg.in/codegangsta/cli.v1"
)

func init() {
	network.RegisterPacketType(ciscConfig{})
}

type ciscConfig struct {
	*identity.Identity
	Follow []*identity.Identity
}

// loadConfig will try to load the configuration and fail if it can't load it.
func loadConfig(c *cli.Context) (*ciscConfig, bool) {
	configFile := getConfig(c)
	log.Lvl2("Loading from", configFile)
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &ciscConfig{Identity: &identity.Identity{}},
				true
		}
		log.ErrFatal(err)
	}
	_, msg, err := network.UnmarshalRegistered(buf)
	log.ErrFatal(err)
	cfg, ok := msg.(*ciscConfig)
	if !ok {
		log.Fatal("Wrong message-type in config-file")
	}
	return cfg, false
}

// loadConfigOrFail tries to load the config and fails if it doesn't succeed.
func loadConfigOrFail(c *cli.Context) *ciscConfig {
	cfg, empty := loadConfig(c)
	if cfg == nil || empty {
		log.Fatal("Couldn't load configuration-file")
	}
	log.ErrFatal(cfg.ConfigUpdate())
	log.ErrFatal(cfg.ProposeUpdate())
	return cfg
}

// Saves the clientApp in the configfile - refuses to save an empty file.
func (cfg *ciscConfig) saveConfig(c *cli.Context) error {
	configFile := getConfig(c)
	if cfg == nil {
		return errors.New("Cannot save empty clientApp")
	}
	buf, err := network.MarshalRegisteredType(cfg)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Lvl2("Saving to", configFile)
	return ioutil.WriteFile(configFile, buf, 0660)
}

// convenience function to send and vote a proposition and update.
func (cfg *ciscConfig) proposeSendVoteUpdate(p *identity.Config) {
	log.ErrFatal(cfg.ProposeSend(p))
	log.ErrFatal(cfg.ProposeVote(true))
	log.ErrFatal(cfg.ConfigUpdate())
}

// writes the ssh-keys to an 'authorized_keys'-file
func (cfg *ciscConfig) writeAuthorizedKeys(c *cli.Context) {
	var keys []string
	dir, _ := sshDirConfig(c)
	authFile := dir + "/authorized_keys"
	// Make backup
	b, err := ioutil.ReadFile(authFile)
	log.ErrFatal(err)
	err = ioutil.WriteFile(authFile+".back", b, 0600)
	log.ErrFatal(err)
	log.Info("Made a backup of your", authFile, "before creating new one.")
	for _, f := range cfg.Follow {
		log.Lvlf2("Parsing IC %x", f.ID)
		for _, s := range f.Config.GetIntermediateColumn("ssh", f.DeviceName) {
			pub := f.Config.GetValue("ssh", s, f.DeviceName)
			log.Lvlf2("Value of %s is %s", s, pub)
			log.Info("Writing key for", s, "to authorized_keys")
			keys = append(keys, pub+" "+s+"@"+f.DeviceName)
		}
	}
	err = ioutil.WriteFile(authFile,
		[]byte(strings.Join(keys, "\n")), 0600)
	log.ErrFatal(err)
}

// showDifference compares the propose and the config-part
func (cfg *ciscConfig) showDifference() {
	if cfg.Proposed == nil {
		log.Info("No proposed config found")
		return
	}
	for k, v := range cfg.Proposed.Data {
		orig, ok := cfg.Config.Data[k]
		if !ok || v != orig {
			log.Info("New or changed key:", k)
		}
	}
	for k := range cfg.Config.Data {
		_, ok := cfg.Proposed.Data[k]
		if !ok {
			log.Info("Deleted key:", k)
		}
	}
	for dev, pub := range cfg.Proposed.Device {
		if _, exists := cfg.Config.Device[dev]; !exists {
			log.Infof("New device: %s / %s", dev,
				pub.Point.String())
		}
	}
	for dev := range cfg.Config.Device {
		if _, exists := cfg.Proposed.Device[dev]; !exists {
			log.Info("Deleted device:", dev)
		}
	}
}

// shows only the keys, but not the data
func (cfg *ciscConfig) showKeys() {
	for d := range cfg.Config.Device {
		log.Info("Connected device", d)
	}
	for k := range cfg.Config.Data {
		log.Info("Key set", k)
	}
}

// Returns the config-file from the configuration
func getConfig(c *cli.Context) string {
	configDir := config.TildeToHome(c.GlobalString("config"))
	log.ErrFatal(mkdir(configDir, 0770))
	return configDir + "/config.bin"
}

// Reads the group-file and returns it
func getGroup(c *cli.Context) *config.Group {
	gfile := c.Args().Get(0)
	gr, err := os.Open(gfile)
	log.ErrFatal(err)
	defer gr.Close()
	groups, err := config.ReadGroupDescToml(gr)
	log.ErrFatal(err)
	if groups == nil || groups.Roster == nil || len(groups.Roster.List) == 0 {
		log.Fatal("No servers found in roster from", gfile)
	}
	return groups
}

// retrieves ssh-config-name and ssh-directory
func sshDirConfig(c *cli.Context) (string, string) {
	sshDir := config.TildeToHome(c.GlobalString("cs"))
	log.ErrFatal(mkdir(sshDir, 0700))
	return sshDir, sshDir + "/config"
}

// MakeSSHKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
// StackOverflow: Greg http://stackoverflow.com/users/328645/greg in
// http://stackoverflow.com/questions/21151714/go-generate-an-ssh-public-key
// No licence added
func makeSSHKeyPair(bits int, pubKeyPath, privateKeyPath string) error {
	if bits < 1024 {
		return errors.New("Reject using too few bits for key")
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return err
	}

	// generate and write private key as PEM
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE, 0600)
	defer privateKeyFile.Close()
	if err != nil {
		return err
	}
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(pub), 0600)
}

// mkDir fails only if it is another error than an existing directory
func mkdir(n string, p os.FileMode) error {
	err := os.Mkdir(n, p)
	if !os.IsExist(err) {
		return err
	}
	return nil
}
