package filter

import (
	ds "github.com/jonhoo/drwmutex"

	"github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type PeerFilter struct {
	// Allowed store who can hop.
	allowed map[peer.ID]struct{}
	// An drwmutex is used here because even if drwmutex is slower in monocore
	// we expect very few Lock but a lot of RLock on a lot of core.
	dmx ds.DRWMutex
}

// Create a new PeerFilter, each peer id passed will be allowed to hop.
func New(ids ...peer.ID) *PeerFilter {
	pf := PeerFilter{
		dmx: ds.New(),
	}

	for _, p := range ids {
		pf.allowed[p] = struct{}{}
	}
	return &pf
}

// Allow, allow a peer to hop.
func (pf *PeerFilter) Allow(p peer.ID) {
	pf.dmx.Lock()
	pf.allowed[p] = struct{}{}
	pf.dmx.Unlock()
}

// Unallow, unallow a peer to hop, can be called freely with peer not allowed to hop.
// Note: that will still be a bit costy, that will lock in write the whole map for a short period of time.
// Note: unallowing an node will not kill current hopping.
func (pf *PeerFilter) Unallow(p peer.ID) {
	pf.dmx.Lock()
	delete(pf.allowed, p)
	pf.dmx.Unlock()
}

// IsAllowed, Check if a peer can hop.
func (pf *PeerFilter) IsAllowed(p peer.ID) bool {
	l := pf.dmx.RLock()
	_, is := pf.allowed[p]
	l.Unlock()
	return is
}

// GetAcceptor return the acceptor, the list can be edited after and change will be made.
// Use `relay.OptApplyAcceptor` to transform it into an RelayOpt.
// Note: unallowing an node will not kill current hopping.
// Note: you can use the same acceptor or multiple acceptor from the same peerfilter on multile relay.
func (pf *PeerFilter) GetAcceptor() relay.Acceptor {
	return func(s network.Stream) bool {
		return pf.IsAllowed(s.Conn().RemotePeer())
	}
}
