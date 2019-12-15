package utils

import (
	"github.com/libp2p/go-libp2p-circuit"

	"github.com/libp2p/go-libp2p-core/network"
)

// Or return an acceptor oring of the 2 given acceptor.
func Or(a, b *relay.Acceptor) *relay.Acceptor {
	return &relay.Acceptor{
		HopConn: func(s network.Stream) bool {
			return a.HopConn(s) || b.HopConn(s)
		},
		CanHop: func(s network.Stream) bool {
			return a.CanHop(s) || b.CanHop(s)
		},
	}
}

// And return an acceptor anding of the 2 given acceptor.
func And(a, b *relay.Acceptor) *relay.Acceptor {
	return &relay.Acceptor{
		HopConn: func(s network.Stream) bool {
			return a.HopConn(s) && b.HopConn(s)
		},
		CanHop: func(s network.Stream) bool {
			return a.CanHop(s) && b.CanHop(s)
		},
	}
}

// Not return an acceptor noting result of the given one.
func Not(a *relay.Acceptor) *relay.Acceptor {
	return &relay.Acceptor{
		HopConn: func(s network.Stream) bool {
			return !a.HopConn(s)
		},
		CanHop: func(s network.Stream) bool {
			return !a.CanHop(s)
		},
	}
}
