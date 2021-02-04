package client

import (
	"net"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var _ manet.Listener = (*Listener)(nil)

type Listener Client

func (c *Client) Listener() *Listener {
	return (*Listener)(c)
}

func (l *Listener) Accept() (manet.Conn, error) {
	// TODO
	return nil, nil
}

func (l *Listener) Addr() net.Addr {
	return &NetAddr{
		Relay:  "any",
		Remote: "any",
	}
}

func (l *Listener) Multiaddr() ma.Multiaddr {
	return circuitAddr
}

func (l *Listener) Close() error {
	// noop for now
	return nil
}
