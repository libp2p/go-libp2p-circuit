package client

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	logging "github.com/ipfs/go-log"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
)

const (
	ProtoIDv1     = "/libp2p/circuit/relay/0.1.0"
	ProtoIDv2Hop  = "/libp2p/circuit/relay/0.2.0/hop"
	ProtoIDv2Stop = "/libp2p/circuit/relay/0.2.0/stop"
)

var log = logging.Logger("p2p-circuit")

type Client struct {
	ctx      context.Context
	host     host.Host
	upgrader *tptu.Upgrader

	incoming chan accept

	mx       sync.Mutex
	hopCount map[peer.ID]int
}

type accept struct {
	conn          *Conn
	writeResponse func() error
}

// New constructs a new p2p-circuit/v2 client, attached to the given host and using the given
// upgrader to perform connection upgrades.
func New(ctx context.Context, h host.Host, upgrader *tptu.Upgrader) (*Client, error) {
	return &Client{
		ctx:      ctx,
		host:     h,
		upgrader: upgrader,
		incoming: make(chan accept),
		hopCount: make(map[peer.ID]int),
	}, nil
}

// Start registers the circuit (client) protocol stream handlers
func (c *Client) Start() {
	c.host.SetStreamHandler(ProtoIDv1, c.handleStreamV1)
	c.host.SetStreamHandler(ProtoIDv2Stop, c.handleStreamV2)
}
