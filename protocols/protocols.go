package protocols

/*
Only used to include the different protocols
*/

import (
	// Don't forget to "register" your protocols here too
	_ "github.com/dedis/cothority/protocols/bizcoin"
	_ "github.com/dedis/cothority/protocols/bizcoin/pbft"
	_ "github.com/dedis/cothority/protocols/cosi"
	_ "github.com/dedis/cothority/protocols/example/channels"
	_ "github.com/dedis/cothority/protocols/example/handlers"
	_ "github.com/dedis/cothority/protocols/jvss"
	_ "github.com/dedis/cothority/protocols/manage"
)
