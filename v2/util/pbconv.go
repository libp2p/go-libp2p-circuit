package util

import (
	"errors"

	pbv1 "github.com/libp2p/go-libp2p-circuit/pb"

	"github.com/libp2p/go-libp2p-core/peer"

	ma "github.com/multiformats/go-multiaddr"
)

func PeerToPeerInfoV1(p *pbv1.CircuitRelay_Peer) (peer.AddrInfo, error) {
	if p == nil {
		return peer.AddrInfo{}, errors.New("nil peer")
	}

	id, err := peer.IDFromBytes(p.Id)
	if err != nil {
		return peer.AddrInfo{}, err
	}

	addrs := make([]ma.Multiaddr, 0, len(p.Addrs))
	for _, addrBytes := range p.Addrs {
		a, err := ma.NewMultiaddrBytes(addrBytes)
		if err == nil {
			addrs = append(addrs, a)
		}
	}

	return peer.AddrInfo{ID: id, Addrs: addrs}, nil
}

func PeerInfoToPeerV1(pi peer.AddrInfo) *pbv1.CircuitRelay_Peer {
	addrs := make([][]byte, len(pi.Addrs))
	for i, addr := range pi.Addrs {
		addrs[i] = addr.Bytes()
	}

	p := new(pbv1.CircuitRelay_Peer)
	p.Id = []byte(pi.ID)
	p.Addrs = addrs

	return p
}
