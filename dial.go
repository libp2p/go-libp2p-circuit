package relay

import (
	"context"
	"fmt"

	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
)

type Dialer Relay

func (d *Dialer) Relay() *Relay {
	return (*Relay)(d)
}

func (r *Relay) Dialer() *Dialer {
	return (*Dialer)(r)
}

func (d *Dialer) DialPeer(ctx context.Context, p peer.ID, a ma.Multiaddr) (tpt.Conn, error) {
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

	dinfo := pstore.PeerInfo{ID: p, Addrs: []ma.Multiaddr{destaddr}}

	return d.Relay().Dial(ctx, *rinfo, dinfo)
}

func (d *Dialer) Matches(a ma.Multiaddr) bool {
	_, err := a.ValueForProtocol(P_CIRCUIT)
	return err == nil
}
