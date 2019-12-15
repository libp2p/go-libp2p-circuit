package filter

import (
	"sync"

	"github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type PeerFilter struct {
	// Allowed store who can hop.
	allowed map[peer.ID]struct{}
	// An drwmutex is used here because even if drwmutex is slower in monocore
	// we expect very few Lock but a lot of RLock on a lot of core.
	mx sync.RWMutex
}

// Create a new PeerFilter, each peer id passed will be allowed to hop.
func New(ids ...peer.ID) *PeerFilter {
	pf := PeerFilter{}

	for _, p := range ids {
		pf.allowed[p] = struct{}{}
	}
	return &pf
}

// Allow, allow a peer to hop.
func (pf *PeerFilter) Allow(p peer.ID) {
	pf.mx.Lock()
	pf.allowed[p] = struct{}{}
	pf.mx.Unlock()
}

// Unallow, unallow a peer to hop, can be called freely with peer not allowed to hop.
// Note: that will still be a bit costy, that will lock in write the whole map for a short period of time.
// Note: unallowing an node will not kill current hopping.
func (pf *PeerFilter) Unallow(p peer.ID) {
	pf.mx.Lock()
	delete(pf.allowed, p)
	pf.mx.Unlock()
}

// IsAllowed, Check if a peer can hop.
func (pf *PeerFilter) IsAllowed(p peer.ID) bool {
	pf.mx.RLock()
	_, is := pf.allowed[p]
	pf.mx.RUnlock()
	return is
}

func (pf *PeerFilter) isStreamAllowed(s network.Stream) bool {
	return pf.IsAllowed(s.Conn().RemotePeer())
}

// GetAcceptor return the acceptor, the list can be edited after and change will be made.
// Use `relay.OptApplyAcceptor` to transform it into an RelayOpt.
// Note: unallowing an node will not kill current hopping.
// Note: you can use the same acceptor or multiple acceptor from the same peerfilter on multiple relay.
func (pf *PeerFilter) GetAcceptor() *relay.Acceptor {
	return &relay.Acceptor{HopConn: pf.isStreamAllowed, CanHop: pf.isStreamAllowed}
}
