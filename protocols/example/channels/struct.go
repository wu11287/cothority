package channels

import "cothority/sda"

// Announce is used to pass a message to all children
type Announce struct {
	Message string
}

// StructAnnounce contains Announce and the data necessary to identify the
// message in the sda framework.
type StructAnnounce struct {
	*sda.TreeNode
	Announce
}

// Reply returns the count of all children.
type Reply struct {
	ChildrenCount int
}

// StructReply contains Reply and the data necessary to identify the
// message in the sda framework.
type StructReply struct {
	*sda.TreeNode
	Reply
}
