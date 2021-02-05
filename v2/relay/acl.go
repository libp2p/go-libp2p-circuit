package relay

import (
	"github.com/libp2p/go-libp2p-core/peer"

	ma "github.com/multiformats/go-multiaddr"
)

type ACLFilter interface {
	AllowReserve(p peer.ID, a ma.Multiaddr) bool
	AllowConnect(src peer.ID, srcAddr ma.Multiaddr, dest peer.ID) bool
}
