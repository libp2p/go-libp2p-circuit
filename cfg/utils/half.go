package utils

import (
	"github.com/libp2p/go-libp2p-circuit"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type InQuarterAcceptor interface {
	In(s network.Stream) bool
}

type OutQuarterAcceptor interface {
	Out(dst peer.AddrInfo) bool
}

type HalfAcceptor interface {
	InQuarterAcceptor
	OutQuarterAcceptor
}

// Used for sharing CanHop.
type base struct {
	h HalfAcceptor
}

func (se base) CanHop(s network.Stream) bool {
	return se.h.In(s)
}

type any struct {
	base
}

func (se any) HopConn(s network.Stream, p peer.AddrInfo) bool {
	return se.h.In(s) || se.h.Out(p)
}

// FullifyAny returns an acceptor where at least one of In or Out of the
// original HalfAcceptor must be true to pass HopConn.
func FullifyAny(h HalfAcceptor) relay.Acceptor {
	return any{base{h: h}}
}

type both struct {
	base
}

func (se both) HopConn(s network.Stream, p peer.AddrInfo) bool {
	return se.h.In(s) && se.h.Out(p)
}

// FullifyBoth returns an acceptor where both, In or Out of the original
// HalfAcceptor must be true to pass HopConn.
func FullifyBoth(h HalfAcceptor) relay.Acceptor {
	return both{base{h: h}}
}

type merged struct {
	InQuarterAcceptor
	OutQuarterAcceptor
}

// Merge merges the 2 given HalfAcceptor, so one do In and the other do Out.
func Merge(in InQuarterAcceptor, out OutQuarterAcceptor) HalfAcceptor {
	return merged{in, out}
}
