package skipchain

import (
	"cothority/network"
)

func init() {
	var msgs = []interface{}{
		// Propose
		&ProposeSkipBlock{},
		&ProposedSkipBlockReply{},
		&SetChildrenSkipBlock{},
		&SetChildrenSkipBlockReply{},
		// Propagation
		&PropagateSkipBlock{},
		// Requests for data
		&GetUpdateChain{},
		&GetUpdateChainReply{},
		// Data-structures
		&ForwardSignature{},
		&SkipBlockFix{},
		&SkipBlock{},
		// Own service
		&Service{},
	}
	for _, m := range msgs {
		network.RegisterPacketType(m)
	}
}

// VerifierID represents one of the verifications used to accept or
// deny a SkipBlock.
type VerifierID uuid.UUID

var (
	// VerifyNone does only basic syntax checking
	VerifyNone = VerifierID(uuid.Nil)
	// VerifyShard makes sure that the child SkipChain will always be
	// a part of its parent SkipChain
	VerifyShard = VerifierID(uuid.NewV5(uuid.NamespaceURL, "Shard"))
	// VerifySSH makes sure that a given number of client-devices signed
	// off on the changes
	VerifySSH = VerifierID(uuid.NewV5(uuid.NamespaceURL, "SSH-ks"))
	// VerifyTimeVault will make sure that a valid TimeVault asks for an
	// additional SkipBlockData
	VerifyTimeVault = VerifierID(uuid.NewV5(uuid.NamespaceURL, "TimeVault"))
)

// This file holds all messages that can be sent to the SkipChain,
// both from the outside and between instances of this service

// External calls

// ProposeSkipBlock - Requests a new skipblock to be appended to
// the given SkipBlock. If the given SkipBlock has Index 0 (which
// is invalid), a new SkipChain will be created.
// The AppId will be used to call the corresponding verification-
// routines who will have to sign off on the new Tree.
type ProposeSkipBlock struct {
	LatestID SkipBlockID
	Proposed *SkipBlock
}

// ProposedSkipBlockReply - returns the signed SkipBlock with updated backlinks
type ProposedSkipBlockReply struct {
	Previous *SkipBlock
	Latest   *SkipBlock
}

// GetUpdateChain - the client sends the hash of the last known
// Skipblock and will get back a list of all necessary SkipBlocks
// to get to the latest.
type GetUpdateChain struct {
	LatestID SkipBlockID
}

// GetUpdateChainReply - returns the shortest chain to the current SkipBlock,
// starting from the SkipBlock the client sent
type GetUpdateChainReply struct {
	Update []*SkipBlock
}

// SetChildrenSkipBlock adds a link to a child-SkipBlock in the
// parent-SkipBlock
type SetChildrenSkipBlock struct {
	ParentID SkipBlockID
	ChildID  SkipBlockID
}

// SetChildrenSkipBlockReply is the reply from SetChildrenSkipBlock. Only one
// of ChildData and ChildRoster will be non-nil
type SetChildrenSkipBlockReply struct {
	Parent *SkipBlock
	Child  *SkipBlock
}

// GetChildrenSkipList - if the SkipList doesn't exist yet, creates the
// Genesis-block of that SkipList.
// It returns a 'GetUpdateChainReply' with the chain from the first to
// the last SkipBlock.
type GetChildrenSkipList struct {
	Current    *SkipBlock
	VerifierID VerifierID
}

// Internal calls

// PropagateSkipBlock sends a newly signed SkipBlock to all members of
// the Cothority
type PropagateSkipBlock struct {
	SkipBlock *SkipBlock
}

// ForwardSignature asks this responsible for a SkipChain to sign off
// a new ForwardLink. This will probably be sent to all members of any
// SkipChain-definition at time 'n'
type ForwardSignature struct {
	ToUpdate SkipBlockID
	Latest   *SkipBlock
}
