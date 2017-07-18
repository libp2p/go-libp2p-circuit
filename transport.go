package relay

import (
	"fmt"

	tpt "github.com/libp2p/go-libp2p-transport"
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

var _ tpt.Transport = (*RelayTransport)(nil)

type RelayTransport Relay

func (t *RelayTransport) Relay() *Relay {
	return (*Relay)(t)
}

func (r *Relay) Transport() *RelayTransport {
	return (*RelayTransport)(r)
}

func (t *RelayTransport) Dialer(laddr ma.Multiaddr, opts ...tpt.DialOpt) (tpt.Dialer, error) {
	if !t.Matches(laddr) {
		return nil, fmt.Errorf("%s is not a relay address", laddr)
	}
	return t.Relay().Dialer(), nil
}

func (t *RelayTransport) Listen(laddr ma.Multiaddr) (tpt.Listener, error) {
	if !t.Matches(laddr) {
		return nil, fmt.Errorf("%s is not a relay address", laddr)
	}
	return t.Relay().Listener(), nil
}

func (t *RelayTransport) Matches(a ma.Multiaddr) bool {
	return t.Relay().Dialer().Matches(a)
}
