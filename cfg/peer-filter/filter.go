package filter

import (
	"sync"

	"github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-circuit/cfg/utils"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

var _ relay.Acceptor = (*PeerFilter)(nil)
var _ utils.HalfAcceptor = (*PeerFilter)(nil)

type PeerFilter struct {
	// Allowed store who can hop.
	allowed map[peer.ID]struct{}
	mx      sync.RWMutex
}

// Create a new PeerFilter, each peer id passed will be allowed to hop.
func New(ids ...peer.ID) *PeerFilter {
	pf := PeerFilter{}

	for _, p := range ids {
		pf.allowed[p] = struct{}{}
	}
	return &pf
}

// Allow allows a peer to hop.
// Note: its way faster to allow multiple peers at once due to locking time.
func (pf *PeerFilter) Allow(ps ...peer.ID) {
	pf.mx.Lock()
	for _, p := range ps {
		pf.allowed[p] = struct{}{}
	}
	pf.mx.Unlock()
}

// Unallow unallows some peers to hop.
// Note: can be called freely with peer not allowed to hop, that will still be a
// bit costy, that will lock in write the whole map for a short period of time.
// Note: unallowing an node will not kill current hopping.
// Note: its way faster to unallow multiple peers at once due to locking time.
func (pf *PeerFilter) Unallow(ps ...peer.ID) {
	pf.mx.Lock()
	for _, p := range ps {
		delete(pf.allowed, p)
	}
	pf.mx.Unlock()
}

// IsAllowed checks if a peer can hop.
func (pf *PeerFilter) IsAllowed(p peer.ID) bool {
	pf.mx.RLock()
	_, is := pf.allowed[p]
	pf.mx.RUnlock()
	return is
}

// Used by the utils.
func (pf *PeerFilter) In(s network.Stream) bool {
	return pf.IsAllowed(s.Conn().RemotePeer())
}

// Used by the utils.
func (pf *PeerFilter) Out(dst peer.AddrInfo) bool {
	return pf.IsAllowed(dst.ID)
}

// Used by the relay.
func (pf *PeerFilter) HopConn(s network.Stream, dst peer.AddrInfo) bool {
	return pf.In(s) || pf.Out(dst)
}

// Used by the relay.
func (pf *PeerFilter) CanHop(s network.Stream) bool {
	return pf.In(s)
}
