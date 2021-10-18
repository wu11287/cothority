package services

import (
	// Importing the services so they register their services to SDA
	// automatically when importing cothority/services
	_ "cothority/services/byzcoin_ng"
	_ "cothority/services/guard"
	_ "cothority/services/identity"
	_ "cothority/services/skipchain"
	_ "cothority/services/status"
	_ "github.com/dedis/cosi/service"
)
