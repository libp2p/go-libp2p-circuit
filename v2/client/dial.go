package client

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"

	ma "github.com/multiformats/go-multiaddr"
)

func (c *Client) dial(ctx context.Context, a ma.Multiaddr, p peer.ID) (*Conn, error) {
	// TODO
	return nil, nil
}
