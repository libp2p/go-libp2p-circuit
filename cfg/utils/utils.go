package utils

import (
	"github.com/libp2p/go-libp2p-circuit"

	"github.com/libp2p/go-libp2p-core/network"
)

// Or return an acceptor oring of the 2 given acceptor.
func Or(a, b relay.Acceptor) relay.Acceptor {
	return func(s network.Stream) bool {
		return a(s) || b(s)
	}
}

// And return an acceptor anding of the 2 given acceptor.
func And(a, b relay.Acceptor) relay.Acceptor {
	return func(s network.Stream) bool {
		return a(s) && b(s)
	}
}

// Not return an acceptor noting result of the given one.
func Not(a relay.Acceptor) relay.Acceptor {
	return func(s network.Stream) bool {
		return !a(s)
	}
}
