package alice

import (
	"github.com/btcsuite/btcd/btcec"
)

var (
	curve = btcec.S256()
	msg   = []byte{"abracadabra"}
)
