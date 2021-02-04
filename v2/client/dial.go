package client

import (
	"context"
	"fmt"
	"time"

	pbv1 "github.com/libp2p/go-libp2p-circuit/pb"
	pbv2 "github.com/libp2p/go-libp2p-circuit/v2/pb"
	"github.com/libp2p/go-libp2p-circuit/v2/util"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"

	ma "github.com/multiformats/go-multiaddr"
)

const maxMessageSize = 4096

func (c *Client) dial(ctx context.Context, a ma.Multiaddr, p peer.ID) (*Conn, error) {
	// split /a/p2p-circuit/b into (/a, /p2p-circuit/b)
	relayaddr, destaddr := ma.SplitFunc(a, func(c ma.Component) bool {
		return c.Protocol().Code == ma.P_CIRCUIT
	})

	// If the address contained no /p2p-circuit part, the second part is nil.
	if destaddr == nil {
		return nil, fmt.Errorf("%s is not a relay address", a)
	}

	if relayaddr == nil {
		return nil, fmt.Errorf("can't dial a p2p-circuit without specifying a relay: %s", a)
	}

	dinfo := peer.AddrInfo{ID: p}

	// Strip the /p2p-circuit prefix from the destaddr so that we can pass the destination address
	// (if present) for active relays
	_, destaddr = ma.SplitFirst(destaddr)
	if destaddr != nil {
		dinfo.Addrs = append(dinfo.Addrs, destaddr)
	}

	rinfo, err := peer.AddrInfoFromP2pAddr(relayaddr)
	if err != nil {
		return nil, fmt.Errorf("error parsing relay multiaddr '%s': %w", relayaddr, err)
	}

	return c.dialPeer(ctx, *rinfo, dinfo)
}

func (c *Client) dialPeer(ctx context.Context, relay, dest peer.AddrInfo) (*Conn, error) {
	log.Debugf("dialing peer %s through relay %s", dest.ID, relay.ID)

	if len(relay.Addrs) > 0 {
		c.host.Peerstore().AddAddrs(relay.ID, relay.Addrs, peerstore.TempAddrTTL)
	}

	s, err := c.host.NewStream(ctx, relay.ID, ProtoIDv2Hop, ProtoIDv1)
	if err != nil {
		return nil, fmt.Errorf("error opening hop stream to relay: %w", err)
	}

	switch s.Protocol() {
	case ProtoIDv2Hop:
		return c.connectV2(s, dest)

	case ProtoIDv1:
		return c.connectV1(s, dest)

	default:
		s.Reset()
		return nil, fmt.Errorf("unexpected stream protocol: %s", s.Protocol())
	}
}

func (c *Client) connectV2(s network.Stream, dest peer.AddrInfo) (*Conn, error) {
	rd := util.NewDelimitedReader(s, maxMessageSize)
	wr := util.NewDelimitedWriter(s)
	defer rd.Close()

	var msg pbv2.HopMessage

	msg.Type = pbv2.HopMessage_CONNECT.Enum()
	msg.Peer = util.PeerInfoToPeerV2(dest)

	err := wr.WriteMsg(&msg)
	if err != nil {
		s.Reset()
		return nil, err
	}

	msg.Reset()

	err = rd.ReadMsg(&msg)
	if err != nil {
		s.Reset()
		return nil, err
	}

	if msg.GetType() != pbv2.HopMessage_STATUS {
		s.Reset()
		return nil, fmt.Errorf("unexpected relay response; not a status message (%d)", msg.GetType())
	}

	status := msg.GetStatus()
	if status != pbv2.Status_OK {
		s.Reset()
		return nil, fmt.Errorf("error opening relay circuit: %s (%d)", pbv2.Status_name[int32(status)], status)
	}

	var stat network.Stat
	if limit := msg.GetLimit(); limit != nil {
		stat.Transient = true
		stat.Extra = make(map[interface{}]interface{})
		stat.Extra[StatLimitDuration] = time.Duration(limit.GetDuration()) * time.Second
		stat.Extra[StatLimitData] = limit.GetData()
	}

	return &Conn{stream: s, remote: dest, stat: stat, client: c}, nil
}

func (c *Client) connectV1(s network.Stream, dest peer.AddrInfo) (*Conn, error) {
	rd := util.NewDelimitedReader(s, maxMessageSize)
	wr := util.NewDelimitedWriter(s)
	defer rd.Close()

	var msg pbv1.CircuitRelay

	msg.Type = pbv1.CircuitRelay_HOP.Enum()
	msg.SrcPeer = util.PeerInfoToPeerV1(c.host.Peerstore().PeerInfo(c.host.ID()))
	msg.DstPeer = util.PeerInfoToPeerV1(dest)

	err := wr.WriteMsg(&msg)
	if err != nil {
		s.Reset()
		return nil, err
	}

	msg.Reset()

	err = rd.ReadMsg(&msg)
	if err != nil {
		s.Reset()
		return nil, err
	}

	if msg.GetType() != pbv1.CircuitRelay_STATUS {
		s.Reset()
		return nil, fmt.Errorf("unexpected relay response; not a status message (%d)", msg.GetType())
	}

	status := msg.GetCode()
	if status != pbv1.CircuitRelay_SUCCESS {
		s.Reset()
		return nil, fmt.Errorf("error opening relay circuit: %s (%d)", pbv1.CircuitRelay_Status_name[int32(status)], status)
	}

	return &Conn{stream: s, remote: dest, client: c}, nil
}
