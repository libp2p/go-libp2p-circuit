package client

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

type Reservation struct {
}

func Reserve(ctx context.Context, h host.Host, ai peer.AddrInfo) (*Reservation, error) {
	// TODO
	return nil, nil
}
