package relay

import (
	ma "github.com/multiformats/go-multiaddr"
)

const P_CIRCUIT = 290

var RelayMaddrProtocol = ma.Protocol{
	Code: P_CIRCUIT,
	Name: "p2p-circuit",
	Size: 0,
}

func init() {
	ma.AddProtocol(RelayMaddrProtocol)
}
