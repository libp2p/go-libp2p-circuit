package relay

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	pb "github.com/libp2p/go-libp2p-circuit/pb"

	ggio "github.com/gogo/protobuf/io"
	logging "github.com/ipfs/go-log"
	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

var log = logging.Logger("relay")

const ProtoID = "/libp2p/circuit/relay/0.1.0"

const maxMessageSize = 4096

var RelayAcceptTimeout = time.Minute

type Relay struct {
	host host.Host
	ctx  context.Context
	self peer.ID

	active bool
	hop    bool

	incoming chan *Conn

	arLk         sync.Mutex
	activeRelays []*Conn
}

type RelayOpt int

var (
	OptActive = RelayOpt(0)
	OptHop    = RelayOpt(1)
)

type RelayError struct {
	Code pb.CircuitRelay_Status
}

func (e RelayError) Error() string {
	return fmt.Sprintf("error opening relay circuit: %s (%d)", pb.CircuitRelay_Status_name[int32(e.Code)], e.Code)
}

func NewRelay(ctx context.Context, h host.Host, opts ...RelayOpt) (*Relay, error) {
	r := &Relay{
		host:     h,
		ctx:      ctx,
		self:     h.ID(),
		incoming: make(chan *Conn),
	}

	for _, opt := range opts {
		switch opt {
		case OptActive:
			r.active = true
		case OptHop:
			r.hop = true
		default:
			return nil, fmt.Errorf("unrecognized option: %d", opt)
		}
	}

	h.SetStreamHandler(ProtoID, r.handleNewStream)

	return r, nil
}

func (r *Relay) Dial(ctx context.Context, relay pstore.PeerInfo, dest pstore.PeerInfo) (*Conn, error) {
	err := r.host.Connect(ctx, relay)
	if err != nil {
		return nil, err
	}

	s, err := r.host.NewStream(ctx, relay.ID, ProtoID)
	if err != nil {
		return nil, err
	}

	rd := ggio.NewDelimitedReader(s, maxMessageSize)
	wr := ggio.NewDelimitedWriter(s)

	var msg pb.CircuitRelay

	msg.Type = pb.CircuitRelay_HOP.Enum()
	msg.SrcPeer = peerInfoToPeer(pstore.PeerInfo{r.self, r.host.Addrs()})
	msg.DstPeer = peerInfoToPeer(dest)

	err = wr.WriteMsg(&msg)
	if err != nil {
		s.Close()
		return nil, err
	}

	msg.Reset()

	err = rd.ReadMsg(&msg)
	if err != nil {
		s.Close()
		return nil, err
	}

	if msg.GetType() != pb.CircuitRelay_STATUS {
		s.Close()
		return nil, fmt.Errorf("unexpected relay response; not a status message (%d)", msg.GetType())
	}

	if msg.GetCode() != pb.CircuitRelay_SUCCESS {
		s.Close()
		return nil, RelayError{msg.GetCode()}
	}

	return &Conn{Stream: s}, nil
}

func (r *Relay) handleNewStream(s inet.Stream) {
	log.Infof("new relay stream from: %s", s.Conn().RemotePeer())

	rd := ggio.NewDelimitedReader(s, maxMessageSize)

	var msg pb.CircuitRelay

	err := rd.ReadMsg(&msg)
	if err != nil {
		r.handleError(s, pb.CircuitRelay_MALFORMED_MESSAGE)
		return
	}

	switch msg.GetType() {
	case pb.CircuitRelay_HOP:
		r.handleHopStream(s, &msg)
	case pb.CircuitRelay_STOP:
		r.handleStopStream(s, &msg)
	case pb.CircuitRelay_CAN_HOP:
		r.handleCanHop(s, &msg)
	default:
		log.Warningf("unexpected relay handshake: %d", msg.GetType())
		r.handleError(s, pb.CircuitRelay_MALFORMED_MESSAGE)
	}
}

func (r *Relay) handleHopStream(s inet.Stream, msg *pb.CircuitRelay) {
	if !r.hop {
		r.handleError(s, pb.CircuitRelay_HOP_CANT_SPEAK_RELAY)
		return
	}

	src, err := peerToPeerInfo(msg.GetSrcPeer())
	if err != nil {
		r.handleError(s, pb.CircuitRelay_HOP_SRC_MULTIADDR_INVALID)
		return
	}

	if src.ID != s.Conn().RemotePeer() {
		r.handleError(s, pb.CircuitRelay_HOP_SRC_MULTIADDR_INVALID)
		return
	}

	dst, err := peerToPeerInfo(msg.GetDstPeer())
	if err != nil {
		r.handleError(s, pb.CircuitRelay_HOP_DST_MULTIADDR_INVALID)
		return
	}

	if dst.ID == r.self {
		r.handleError(s, pb.CircuitRelay_HOP_CANT_RELAY_TO_SELF)
		return
	}

	// open stream
	ctp := r.host.Network().ConnsToPeer(dst.ID)
	if len(ctp) == 0 {
		if !r.active {
			r.handleError(s, pb.CircuitRelay_HOP_NO_CONN_TO_DST)
			return
		}

		ctx, cancel := context.WithTimeout(r.ctx, time.Second*10)
		defer cancel()
		err = r.host.Connect(ctx, dst)
		if err != nil {
			log.Debugf("error opening relay connection to %s: %s", dst.ID.Pretty(), err.Error())
			r.handleError(s, pb.CircuitRelay_HOP_CANT_DIAL_DST)
			return
		}
	}

	bs, err := r.host.NewStream(r.ctx, dst.ID, ProtoID)
	if err != nil {
		log.Debugf("error opening relay stream to %s: %s", dst.ID.Pretty(), err.Error())
		r.handleError(s, pb.CircuitRelay_HOP_CANT_OPEN_DST_STREAM)
		return
	}

	// stop handshake
	rd := ggio.NewDelimitedReader(bs, maxMessageSize)
	wr := ggio.NewDelimitedWriter(bs)

	msg.Type = pb.CircuitRelay_STOP.Enum()

	err = wr.WriteMsg(msg)
	if err != nil {
		log.Debugf("error writing stop handshake: %s", err.Error())
		bs.Close()
		r.handleError(s, pb.CircuitRelay_STOP_RELAY_REFUSED)
		return
	}

	msg.Reset()

	err = rd.ReadMsg(msg)
	if err != nil {
		log.Debugf("error reading stop response: %s", err.Error())
		bs.Close()
		r.handleError(s, pb.CircuitRelay_STOP_RELAY_REFUSED)
		return
	}

	if msg.GetType() != pb.CircuitRelay_STATUS {
		log.Debugf("unexpected relay stop response: not a status message (%d)", msg.GetType())
		bs.Close()
		r.handleError(s, pb.CircuitRelay_STOP_RELAY_REFUSED)
		return
	}

	if msg.GetCode() != pb.CircuitRelay_SUCCESS {
		log.Debugf("relay stop failure: %d", msg.GetCode())
		bs.Close()
		r.handleError(s, msg.GetCode())
		return
	}

	err = r.writeResponse(s, pb.CircuitRelay_SUCCESS)
	if err != nil {
		log.Debugf("error writing relay response: %s", err.Error())
		s.Close()
		return
	}

	// relay connection
	log.Infof("relaying connection between %s and %s", src.ID.Pretty(), dst.ID.Pretty())

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
}

func (r *Relay) handleStopStream(s inet.Stream, msg *pb.CircuitRelay) {
	src, err := peerToPeerInfo(msg.GetSrcPeer())
	if err != nil || len(src.Addrs) == 0 {
		r.handleError(s, pb.CircuitRelay_STOP_SRC_MULTIADDR_INVALID)
		return
	}

	dst, err := peerToPeerInfo(msg.GetDstPeer())
	if err != nil || dst.ID != r.self {
		r.handleError(s, pb.CircuitRelay_STOP_DST_MULTIADDR_INVALID)
		return
	}

	log.Infof("relay connection from: %s", src.ID)

	r.host.Peerstore().AddAddrs(src.ID, src.Addrs, pstore.TempAddrTTL)

	select {
	case r.incoming <- &Conn{Stream: s, remote: src}:
	case <-time.After(RelayAcceptTimeout):
		r.handleError(s, pb.CircuitRelay_STOP_RELAY_REFUSED)
	}
}

func (r *Relay) handleCanHop(s inet.Stream, msg *pb.CircuitRelay) {
	var err error

	if r.hop {
		err = r.writeResponse(s, pb.CircuitRelay_SUCCESS)
	} else {
		err = r.writeResponse(s, pb.CircuitRelay_HOP_CANT_SPEAK_RELAY)
	}

	if err != nil {
		log.Debugf("error writing relay response: %s", err.Error())
	}

	s.Close()
}

func (r *Relay) handleError(s inet.Stream, code pb.CircuitRelay_Status) {
	log.Warningf("relay error: %s (%d)", pb.CircuitRelay_Status_name[int32(code)], code)
	err := r.writeResponse(s, code)
	if err != nil {
		log.Debugf("error writing relay response: %s", err.Error())
	}
	s.Close()
}

func (r *Relay) writeResponse(s inet.Stream, code pb.CircuitRelay_Status) error {
	wr := ggio.NewDelimitedWriter(s)

	var msg pb.CircuitRelay
	msg.Type = pb.CircuitRelay_STATUS.Enum()
	msg.Code = code.Enum()

	return wr.WriteMsg(&msg)
}
