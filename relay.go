package relay

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	logging "github.com/ipfs/go-log"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("relay")

const HopID = "/libp2p/relay/circuit/1.0.0/hop"
const StopID = "/libp2p/relay/circuit/1.0.0/stop"

const maxAddrLen = 1024

var RelayAcceptTimeout = time.Minute

type Relay struct {
	host host.Host
	ctx  context.Context
	self peer.ID

	active bool

	incoming chan *Conn

	arLk         sync.Mutex
	activeRelays []*Conn
}

type RelayOpt int

var (
	OptActive = RelayOpt(0)
	OptHop    = RelayOpt(1)
)

func NewRelay(ctx context.Context, h host.Host, opts ...RelayOpt) (*Relay, error) {
	r := &Relay{
		host:     h,
		ctx:      ctx,
		self:     h.ID(),
		incoming: make(chan *Conn),
	}

	h.SetStreamHandler(StopID, r.HandleNewStopStream)

	for _, opt := range opts {
		switch opt {
		case OptActive:
			r.active = true
		case OptHop:
			h.SetStreamHandler(HopID, r.HandleNewHopStream)
		default:
			return nil, fmt.Errorf("unrecognized option: %d", opt)
		}
	}

	return r, nil
}

func (r *Relay) Dial(ctx context.Context, relay peer.ID, dest ma.Multiaddr) (*Conn, error) {
	s, err := r.host.NewStream(ctx, relay, HopID)
	if err != nil {
		return nil, err
	}

	if err := writeLpMultiaddr(s, dest); err != nil {
		return nil, err
	}

	var stat RelayStatus
	if err := stat.ReadFrom(s); err != nil {
		return nil, err
	}

	if stat.Code != StatusOK {
		return nil, &stat
	}

	return &Conn{Stream: s}, nil
}

func (r *Relay) HandleNewStopStream(s inet.Stream) {
	log.Infof("new stop stream from: %s", s.Conn().RemotePeer())
	status := r.handleNewStopStream(s)
	if status != nil {
		if err := status.WriteTo(s); err != nil {
			log.Info("problem writing error status:", err)
		}
		s.Close()
		return
	}
}

func (r *Relay) handleNewStopStream(s inet.Stream) *RelayStatus {
	info, err := r.readInfo(s)
	if err != nil {
		return &RelayStatus{
			Code:    StatusDstAddrErr,
			Message: err.Error(),
		}
	}

	log.Infof("relay connection from: %s", info.ID)
	select {
	case r.incoming <- &Conn{Stream: s, remoteMaddr: info.Addrs[0], remotePeer: info.ID}:
		return nil
	case <-time.After(RelayAcceptTimeout):
		return &RelayStatus{
			Code:    StatusDstRelayRefused,
			Message: "timed out waiting for relay to be accepted",
		}
	}
}

func (r *Relay) HandleNewHopStream(s inet.Stream) {
	log.Infof("new hop stream from: %s", s.Conn().RemotePeer())
	status := r.handleNewHopStream(s)
	if status != nil {
		if err := status.WriteTo(s); err != nil {
			log.Debugf("problem writing error status back: %s", err)
			s.Close()
			return
		}
	}
}

func (r *Relay) handleNewHopStream(s inet.Stream) *RelayStatus {
	info, err := r.readInfo(s)
	if err != nil {
		return &RelayStatus{
			Code:    StatusRelayAddrErr,
			Message: err.Error(),
		}
	}

	if info.ID == r.self {
		return &RelayStatus{
			Code:    StatusRelayHopToSelf,
			Message: "relay hop attempted to self",
		}
	}

	ctp := r.host.Network().ConnsToPeer(info.ID)
	if len(ctp) == 0 {
		return &RelayStatus{
			Code:    StatusRelayNotConnected,
			Message: "refusing to make new connection for relay",
		}
	}

	bs, err := r.host.NewStream(r.ctx, info.ID, StopID)
	if err != nil {
		return &RelayStatus{
			Code:    StatusRelayStreamFailed,
			Message: err.Error(),
		}
	}

	// TODO: add helper method 'PeerID to multiaddr'
	paddr, err := ma.NewMultiaddr("/ipfs/" + s.Conn().RemotePeer().Pretty())
	if err != nil {
		return &RelayStatus{
			Code:    StatusRelayAddrErr,
			Message: err.Error(),
		}
	}

	p2pa := s.Conn().RemoteMultiaddr().Encapsulate(paddr)
	if err := writeLpMultiaddr(bs, p2pa); err != nil {
		return &RelayStatus{
			Code:    StatusRelayStreamFailed,
			Message: err.Error(),
		}
	}

	go func() {
		_, err := io.Copy(s, bs)
		if err != io.EOF && err != nil {
			log.Debugf("relay copy error: %s", err)
		}
		s.Close()
	}()
	go func() {
		_, err := io.Copy(bs, s)
		if err != io.EOF && err != nil {
			log.Debugf("relay copy error: %s", err)
		}
		bs.Close()
	}()

	return nil
}

func (r *Relay) readInfo(s inet.Stream) (*pstore.PeerInfo, error) {
	addr, err := readLpMultiaddr(s)
	if err != nil {
		return nil, err
	}

	info, err := pstore.InfoFromP2pAddr(addr)
	if err != nil {
		return nil, err
	}

	return info, nil
}
