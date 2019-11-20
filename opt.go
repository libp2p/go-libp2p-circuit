package relay

import (
	"github.com/libp2p/go-libp2p-core/network"
)

// OptActive configures the relay transport to actively establish
// outbound connections on behalf of clients. You probably don't want to
// enable this unless you know what you're doing.
func OptActive(r *Relay) error {
	r.active = true
	return nil
}

// OptHop configures the relay transport to accept requests to relay
// traffic on behalf of third-parties. Unless OptActive is specified,
// this will only relay traffic between peers already connected to this
// node.
// Note: This is not needed to enable hopping with a filter.
func OptHop(r *Relay) error {
	r.isAllowedToHop = func(_ network.Stream) bool {
		return true
	}
	return nil
}

// OptDiscovery configures this relay transport to discover new relays
// by probing every new peer. You almost _certainly_ don't want to
// enable this.
func OptDiscovery(r *Relay) error {
	r.discovery = true
	return nil
}

// ApplyAcceptor will return an applier applying the acceptor
// `func(network.Stream) bool` to the relay, if the acceptor return true the
// peer is allowed to hop over the current node.
func OptApplyAcceptor(f Acceptor) RelayOpt {
	return func(r *Relay) error {
		r.isAllowedToHop = f
		return nil
	}
}
