package client

import (
	"context"
	"sync"

	"github.com/libp2p/go-libp2p-circuit/v2/proto"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	logging "github.com/ipfs/go-log"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
)

var log = logging.Logger("p2p-circuit")

// Client implements the client-side of the p2p-circuit/v2 protocol:
// - it implements dialing through v2 relays
// - it listens for incoming connections through v2 relays.
//
// For backwards compatibility with v1 relays and older nodes, the client will
// also accept relay connections through v1 relays and fallback dial peers using p2p-circuit/v1.
// This allows us to use the v2 code as drop in replacement for v1 in a host without breaking
// existing code and interoperability with older nodes.
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
	c.host.SetStreamHandler(proto.ProtoIDv1, c.handleStreamV1)
	c.host.SetStreamHandler(proto.ProtoIDv2Stop, c.handleStreamV2)
}
