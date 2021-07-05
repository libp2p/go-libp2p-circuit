package relay

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	pbv2 "github.com/libp2p/go-libp2p-circuit/v2/pb"
	"github.com/libp2p/go-libp2p-circuit/v2/util"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	pool "github.com/libp2p/go-buffer-pool"

	logging "github.com/ipfs/go-log"
)

const (
	ProtoIDv2Hop  = "/libp2p/circuit/relay/0.2.0/hop"
	ProtoIDv2Stop = "/libp2p/circuit/relay/0.2.0/stop"

	ReservationTagWeight = 10

	StreamTimeout    = time.Minute
	ConnectTimeout   = 30 * time.Second
	HandshakeTimeout = time.Minute

	maxMessageSize = 4096
)

var log = logging.Logger("relay")

type Relay struct {
	ctx    context.Context
	cancel func()

	host host.Host
	rc   Resources
	acl  ACLFilter

	mx      sync.Mutex
	rsvp    map[peer.ID]time.Time
	refresh map[peer.ID]time.Time
	conns   map[peer.ID]int
}

func New(ctx context.Context, h host.Host, opts ...Option) (*Relay, error) {
	ctx, cancel := context.WithCancel(ctx)

	r := &Relay{
		ctx:     ctx,
		cancel:  cancel,
		host:    h,
		rc:      DefaultResources(),
		acl:     nil,
		rsvp:    make(map[peer.ID]time.Time),
		refresh: make(map[peer.ID]time.Time),
		conns:   make(map[peer.ID]int),
	}

	for _, opt := range opts {
		err := opt(r)
		if err != nil {
			return nil, fmt.Errorf("error applying relay option: %w", err)
		}
	}

	h.SetStreamHandler(ProtoIDv2Hop, r.handleStream)
	h.Network().Notify(
		&network.NotifyBundle{
			DisconnectedF: r.disconnected,
		})
	go r.background()

	return r, nil
}

func (r *Relay) Close() error {
	select {
	case <-r.ctx.Done():
	default:
		r.cancel()
	}
	return nil
}

func (r *Relay) handleStream(s network.Stream) {
	select {
	case <-r.ctx.Done():
		s.Reset()
		return
	default:
	}

	s.SetReadDeadline(time.Now().Add(StreamTimeout))

	log.Infof("new relay stream from: %s", s.Conn().RemotePeer())

	rd := util.NewDelimitedReader(s, maxMessageSize)
	defer rd.Close()

	var msg pbv2.HopMessage

	err := rd.ReadMsg(&msg)
	if err != nil {
		r.handleError(s, pbv2.Status_MALFORMED_MESSAGE)
		return
	}
	// reset stream deadline as message has been read
	s.SetReadDeadline(time.Time{})

	switch msg.GetType() {
	case pbv2.HopMessage_RESERVE:
		r.handleReserve(s, &msg)

	case pbv2.HopMessage_CONNECT:
		r.handleConnect(s, &msg)

	default:
		r.handleError(s, pbv2.Status_MALFORMED_MESSAGE)
	}
}

func (r *Relay) handleReserve(s network.Stream, msg *pbv2.HopMessage) {
	defer s.Close()

	p := s.Conn().RemotePeer()
	a := s.Conn().RemoteMultiaddr()

	if r.acl != nil && !r.acl.AllowReserve(p, a) {
		log.Debugf("refusing relay reservation for %s; permission denied", p)
		r.handleError(s, pbv2.Status_PERMISSION_DENIED)
		return
	}

	r.mx.Lock()
	now := time.Now()

	refresh, exists := r.refresh[p]
	if exists && refresh.After(now) {
		// extend refresh time, peer is trying too fast
		r.refresh[p] = refresh.Add(r.rc.ReservationRefreshTTL)
		r.mx.Unlock()
		log.Debugf("refusing relay reservation for %s; refreshing too fast", p)
		r.handleError(s, pbv2.Status_RESERVATION_REFUSED)
		return
	}

	active := len(r.rsvp)
	if active >= r.rc.MaxReservations {
		r.mx.Unlock()
		log.Debugf("refusing relay reservation for %s; too many reservations", p)
		r.handleError(s, pbv2.Status_RESOURCE_LIMIT_EXCEEDED)
		return
	}

	r.rsvp[p] = now.Add(r.rc.ReservationTTL)
	r.refresh[p] = now.Add(r.rc.ReservationRefreshTTL)
	r.host.ConnManager().TagPeer(p, "relay-reservation", ReservationTagWeight)
	r.mx.Unlock()

	log.Debugf("reserving relay slot for %s", p)

	err := r.writeResponse(s, pbv2.Status_OK, r.makeReservationMsg(p), r.makeLimitMsg(p))
	if err != nil {
		s.Reset()
		log.Debugf("error writing reservation response; retracting reservation for %s", p)
		r.mx.Lock()
		delete(r.rsvp, p)
		r.host.ConnManager().UntagPeer(p, "relay-reservation")
		r.mx.Unlock()
	}
}

func (r *Relay) handleConnect(s network.Stream, msg *pbv2.HopMessage) {
	src := s.Conn().RemotePeer()
	dest, err := util.PeerToPeerInfoV2(msg.GetPeer())
	if err != nil {
		r.handleError(s, pbv2.Status_MALFORMED_MESSAGE)
		return
	}

	if r.acl != nil && !r.acl.AllowConnect(src, s.Conn().RemoteMultiaddr(), dest.ID) {
		log.Debugf("refusing connection from %s to %s; permission denied", src, dest.ID)
		r.handleError(s, pbv2.Status_PERMISSION_DENIED)
		return
	}

	r.mx.Lock()
	_, rsvp := r.rsvp[dest.ID]
	if !rsvp {
		r.mx.Unlock()
		log.Debugf("refusing connection from %s to %s; no reservation", src, dest.ID)
		r.handleError(s, pbv2.Status_NO_RESERVATION)
		return
	}

	srcConns := r.conns[src]
	if srcConns >= r.rc.MaxCircuits {
		r.mx.Unlock()
		log.Debugf("refusing connection from %s to %s; too many connections from %s", src, dest.ID, src)
		r.handleError(s, pbv2.Status_RESOURCE_LIMIT_EXCEEDED)
		return
	}
	r.conns[src]++

	destConns := r.conns[dest.ID]
	if destConns >= r.rc.MaxCircuits {
		r.conns[src]--
		r.mx.Unlock()
		log.Debugf("refusing connection from %s to %s; too many connecitons to %s", src, dest.ID, dest.ID)
		r.handleError(s, pbv2.Status_RESOURCE_LIMIT_EXCEEDED)
		return
	}
	r.conns[dest.ID]++
	r.mx.Unlock()

	cleanup := func() {
		r.mx.Lock()
		r.conns[src]--
		r.conns[dest.ID]--
		r.mx.Unlock()
	}

	ctx, cancel := context.WithTimeout(r.ctx, ConnectTimeout)
	defer cancel()

	ctx = network.WithNoDial(ctx, "relay connect")

	bs, err := r.host.NewStream(ctx, dest.ID, ProtoIDv2Stop)
	if err != nil {
		log.Debugf("error opening relay stream to %s: %s", dest.ID, err)
		cleanup()
		r.handleError(s, pbv2.Status_CONNECTION_FAILED)
		return
	}

	// handshake
	rd := util.NewDelimitedReader(bs, maxMessageSize)
	wr := util.NewDelimitedWriter(bs)
	defer rd.Close()

	var stopmsg pbv2.StopMessage
	stopmsg.Type = pbv2.StopMessage_CONNECT.Enum()
	stopmsg.Peer = util.PeerInfoToPeerV2(peer.AddrInfo{ID: src})
	stopmsg.Limit = r.makeLimitMsg(dest.ID)

	bs.SetDeadline(time.Now().Add(HandshakeTimeout))

	err = wr.WriteMsg(&stopmsg)
	if err != nil {
		log.Debugf("error writing stop handshake")
		bs.Reset()
		cleanup()
		r.handleError(s, pbv2.Status_CONNECTION_FAILED)
		return
	}

	stopmsg.Reset()

	err = rd.ReadMsg(&stopmsg)
	if err != nil {
		log.Debugf("error reading stop response: %s", err.Error())
		bs.Reset()
		cleanup()
		r.handleError(s, pbv2.Status_CONNECTION_FAILED)
		return
	}

	if t := stopmsg.GetType(); t != pbv2.StopMessage_STATUS {
		log.Debugf("unexpected stop response; not a status message (%d)", t)
		bs.Reset()
		cleanup()
		r.handleError(s, pbv2.Status_CONNECTION_FAILED)
		return
	}

	if status := stopmsg.GetStatus(); status != pbv2.Status_OK {
		log.Debugf("relay stop failure: %d", status)
		bs.Reset()
		cleanup()
		r.handleError(s, pbv2.Status_CONNECTION_FAILED)
		return
	}

	var response pbv2.HopMessage
	response.Type = pbv2.HopMessage_STATUS.Enum()
	response.Status = pbv2.Status_OK.Enum()
	response.Limit = r.makeLimitMsg(dest.ID)

	wr = util.NewDelimitedWriter(s)
	err = wr.WriteMsg(&response)
	if err != nil {
		log.Debugf("error writing relay response: %s", err)
		bs.Reset()
		s.Reset()
		cleanup()
		return
	}

	// reset deadline
	bs.SetDeadline(time.Time{})

	log.Infof("relaying connection from %s to %s", src, dest.ID)

	goroutines := new(int32)
	*goroutines = 2

	done := func() {
		if atomic.AddInt32(goroutines, -1) == 0 {
			s.Close()
			bs.Close()
			cleanup()
		}
	}

	if r.rc.Limit != nil {
		deadline := time.Now().Add(r.rc.Limit.Duration)
		s.SetDeadline(deadline)
		bs.SetDeadline(deadline)
		go r.relayLimited(s, bs, src, dest.ID, r.rc.Limit.Data, done)
		go r.relayLimited(bs, s, dest.ID, src, r.rc.Limit.Data, done)
	} else {
		go r.relayUnlimited(s, bs, src, dest.ID, done)
		go r.relayUnlimited(bs, s, dest.ID, src, done)
	}
}

func (r *Relay) relayLimited(src, dest network.Stream, srcID, destID peer.ID, limit int64, done func()) {
	defer done()

	buf := pool.Get(r.rc.BufferSize)
	defer pool.Put(buf)

	limitedSrc := io.LimitReader(src, limit)

	count, err := io.CopyBuffer(dest, limitedSrc, buf)
	if err != nil {
		log.Debugf("relay copy error: %s", err)
		// Reset both.
		src.Reset()
		dest.Reset()
	} else {
		// propagate the close
		dest.CloseWrite()
		if count == limit {
			// we've reached the limit, discard further input
			src.CloseRead()
		}
	}

	log.Debugf("relayed %d bytes from %s to %s", count, srcID, destID)
}

func (r *Relay) relayUnlimited(src, dest network.Stream, srcID, destID peer.ID, done func()) {
	defer done()

	buf := pool.Get(r.rc.BufferSize)
	defer pool.Put(buf)

	count, err := io.CopyBuffer(dest, src, buf)
	if err != nil {
		log.Debugf("relay copy error: %s", err)
		// Reset both.
		src.Reset()
		dest.Reset()
	} else {
		// propagate the close
		dest.CloseWrite()
	}

	log.Debugf("relayed %d bytes from %s to %s", count, srcID, destID)
}

func (r *Relay) handleError(s network.Stream, status pbv2.Status) {
	log.Debugf("relay error: %s (%d)", pbv2.Status_name[int32(status)], status)
	err := r.writeResponse(s, status, nil, nil)
	if err != nil {
		s.Reset()
		log.Debugf("error writing relay response: %s", err.Error())
	} else {
		s.Close()
	}
}

func (r *Relay) writeResponse(s network.Stream, status pbv2.Status, rsvp *pbv2.Reservation, limit *pbv2.Limit) error {
	wr := util.NewDelimitedWriter(s)

	var msg pbv2.HopMessage
	msg.Type = pbv2.HopMessage_STATUS.Enum()
	msg.Status = status.Enum()
	msg.Reservation = rsvp
	msg.Limit = limit

	return wr.WriteMsg(&msg)
}

func (r *Relay) makeReservationMsg(p peer.ID) *pbv2.Reservation {
	// TODO signed reservation vouchers

	ttl := int32(r.rc.ReservationTTL / time.Second)
	// TODO cache this
	ai := peer.AddrInfo{r.host.ID(), r.host.Addrs()}

	return &pbv2.Reservation{
		Ttl:   &ttl,
		Relay: util.PeerInfoToPeerV2(ai),
	}
}

func (r *Relay) makeLimitMsg(p peer.ID) *pbv2.Limit {
	if r.rc.Limit == nil {
		return nil
	}

	duration := int32(r.rc.Limit.Duration / time.Second)
	data := int64(r.rc.Limit.Data)

	return &pbv2.Limit{
		Duration: &duration,
		Data:     &data,
	}
}

func (r *Relay) background() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.gc()
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *Relay) gc() {
	r.mx.Lock()
	defer r.mx.Unlock()

	now := time.Now()

	for p, expire := range r.rsvp {
		if expire.Before(now) {
			delete(r.rsvp, p)
			r.host.ConnManager().UntagPeer(p, "relay-reservation")
		}
	}

	for p, expire := range r.refresh {
		_, rsvp := r.rsvp[p]
		if !rsvp && expire.Before(now) {
			delete(r.refresh, p)
		}
	}

	for p, count := range r.conns {
		if count == 0 {
			delete(r.conns, p)
		}
	}
}

func (r *Relay) disconnected(n network.Network, c network.Conn) {
	p := c.RemotePeer()
	if n.Connectedness(p) == network.Connected {
		return
	}

	r.mx.Lock()
	defer r.mx.Unlock()

	delete(r.rsvp, p)
}
