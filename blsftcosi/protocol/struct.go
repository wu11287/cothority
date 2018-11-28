package protocol

import (
	"errors"
	"fmt"
	"time"

	"github.com/dedis/kyber"
	"github.com/dedis/kyber/pairing"
	"github.com/dedis/kyber/sign/bls"
	"github.com/dedis/kyber/sign/cosi"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
)

// DefaultProtocolName can be used from other packages to refer to this protocol.
// If this name is used, then the suite used to verify signatures must be
// the default cothority.Suite.
const DefaultProtocolName = "blsftCoSiProtoDefault"

// DefaultSubProtocolName the name of the default sub protocol, started by the
// main protocol.
const DefaultSubProtocolName = "blsftSubCoSiProtoDefault"

func init() {
	network.RegisterMessages(&Announcement{}, &Response{}, &Stop{})
}

// BlsSignature contains the message and its aggregated signature
type BlsSignature []byte

// GetMask creates and returns the mask associated with the signature
func (sig BlsSignature) GetMask(suite pairing.Suite, publics []kyber.Point) (*cosi.Mask, error) {
	mask, err := cosi.NewMask(suite.(cosi.Suite), publics, nil)
	if err != nil {
		return nil, err
	}

	lenCom := suite.G1().PointLen()
	mask.SetMask(sig[lenCom:])

	return mask, nil
}

// Point creates the point associated with the signature in G1
func (sig BlsSignature) Point(suite pairing.Suite) (kyber.Point, error) {
	pointSig := suite.G1().Point()

	if err := pointSig.UnmarshalBinary(sig); err != nil {
		return nil, err
	}

	return pointSig, nil
}

// Verify checks the signature over the message using the given public keys and policy
func (sig BlsSignature) Verify(ps pairing.Suite, msg []byte, publics []kyber.Point, policy cosi.Policy) error {
	if publics == nil {
		return errors.New("no public keys provided")
	}
	if msg == nil {
		return errors.New("no message provided")
	}
	if sig == nil {
		return errors.New("no signature provided")
	}

	lenCom := ps.G1().PointLen()
	signature := sig[:lenCom]

	// Unpack the participation mask and get the aggregate public key
	mask, err := sig.GetMask(ps, publics)

	err = bls.Verify(ps, mask.AggregatePublic, msg, signature)
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}

	log.Lvl3("Signature verified and is correct!")
	log.Lvl3("m.CountEnabled():", mask.CountEnabled())

	if !policy.Check(mask) {
		return errors.New("the policy is not fulfilled")
	}

	return nil
}

// Announcement is the blsftcosi annoucement message
type Announcement struct {
	Msg       []byte // statement to be signed
	Data      []byte
	Timeout   time.Duration
	Threshold int
}

// StructAnnouncement just contains Announcement and the data necessary to identify and
// process the message in the onet framework.
type StructAnnouncement struct {
	*onet.TreeNode
	Announcement
}

// Response is the blsftcosi response message
type Response struct {
	Signature BlsSignature
	Mask      []byte
	Refusals  map[int][]byte
}

// StructResponse just contains Response and the data necessary to identify and
// process the message in the onet framework.
type StructResponse struct {
	*onet.TreeNode
	Response
}

// Stop is a message used to instruct a node to stop its protocol
type Stop struct{}

// StructStop is a wrapper around Stop for it to work with onet
type StructStop struct {
	*onet.TreeNode
	Stop
}
