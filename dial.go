package relay

import (
	"context"
	"fmt"

	pstore "github.com/libp2p/go-libp2p-peerstore"
	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
)

var _ tpt.Dialer = (*Dialer)(nil)

type Dialer Relay

func (d *Dialer) Relay() *Relay {
	return (*Relay)(d)
}

func (r *Relay) Dialer() *Dialer {
	return (*Dialer)(r)
}

func (d *Dialer) Dial(a ma.Multiaddr) (tpt.Conn, error) {
	return d.DialContext(d.ctx, a)
}

func (d *Dialer) DialContext(ctx context.Context, a ma.Multiaddr) (tpt.Conn, error) {
	if !d.Matches(a) {
		return nil, fmt.Errorf("%s is not a relay address", a)
	}
	parts := ma.Split(a)

	spl, _ := ma.NewMultiaddr("/p2p-circuit")

	var relayaddr, destaddr ma.Multiaddr
	for i, p := range parts {
		if p.Equal(spl) {
			relayaddr = ma.Join(parts[:i]...)
			destaddr = ma.Join(parts[i+1:]...)
			break
		}
	}

	rinfo, err := pstore.InfoFromP2pAddr(relayaddr)
	if err != nil {
		return nil, err
	}

	dinfo, err := pstore.InfoFromP2pAddr(destaddr)
	if err != nil {
		return nil, err
	}

	return d.Relay().DialPeer(ctx, *rinfo, *dinfo)
}

func (d *Dialer) Matches(a ma.Multiaddr) bool {
	_, err := a.ValueForProtocol(P_CIRCUIT)
	return err == nil
}
