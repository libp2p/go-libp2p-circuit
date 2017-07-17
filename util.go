package relay

import (
	"errors"

	pb "github.com/libp2p/go-libp2p-circuit/pb"

	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
)

func peerToPeerInfo(p *pb.CircuitRelay_Peer) (empty pstore.PeerInfo, err error) {
	if p == nil {
		return empty, errors.New("nil peer")
	}

	h, err := mh.Cast(p.Id)
	if err != nil {
		return empty, err
	}

	addrs := make([]ma.Multiaddr, len(p.Addrs))
	for i := 0; i < len(addrs); i++ {
		a, err := ma.NewMultiaddrBytes(p.Addrs[i])
		if err != nil {
			return empty, err
		}
		addrs[i] = a
	}

	return pstore.PeerInfo{ID: peer.ID(h), Addrs: addrs}, nil
}

func peerInfoToPeer(pi pstore.PeerInfo) *pb.CircuitRelay_Peer {
	addrs := make([][]byte, len(pi.Addrs))
	for i := 0; i < len(addrs); i++ {
		addrs[i] = pi.Addrs[i].Bytes()
	}

	p := new(pb.CircuitRelay_Peer)
	p.Id = []byte(pi.ID)
	p.Addrs = addrs

	return p
}
