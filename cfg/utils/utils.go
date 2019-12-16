package utils

import (
	"github.com/libp2p/go-libp2p-circuit"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type or struct {
	a relay.Acceptor
	b relay.Acceptor
}

func (se or) HopConn(s network.Stream, dst peer.AddrInfo) bool {
	return se.a.HopConn(s, dst) || se.b.HopConn(s, dst)
}
func (se or) CanHop(s network.Stream) bool {
	return se.a.CanHop(s) || se.b.CanHop(s)
}

// Or return an acceptor oring of the 2 given acceptor.
func Or(a, b relay.Acceptor) relay.Acceptor {
	return or{a: a, b: b}
}

type and struct {
	a relay.Acceptor
	b relay.Acceptor
}

func (se and) HopConn(s network.Stream, dst peer.AddrInfo) bool {
	return se.a.HopConn(s, dst) && se.b.HopConn(s, dst)
}
func (se and) CanHop(s network.Stream) bool {
	return se.a.CanHop(s) && se.b.CanHop(s)
}

// And return an acceptor anding of the 2 given acceptor.
func And(a, b relay.Acceptor) relay.Acceptor {
	return and{a: a, b: b}
}

type not struct {
	a relay.Acceptor
}

func (se not) HopConn(s network.Stream, dst peer.AddrInfo) bool {
	return !se.a.HopConn(s, dst)
}
func (se not) CanHop(s network.Stream) bool {
	return !se.a.CanHop(s)
}

// Not return an acceptor noting result of the given one.
func Not(a relay.Acceptor) relay.Acceptor {
	return not{a: a}
}
