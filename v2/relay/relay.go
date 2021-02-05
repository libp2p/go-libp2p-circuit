package relay

import (
	"fmt"
	"sync"
	"time"

	pbv2 "github.com/libp2p/go-libp2p-circuit/v2/pb"
	"github.com/libp2p/go-libp2p-circuit/v2/util"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	logging "github.com/ipfs/go-log"
)

const (
	ProtoIDv2Hop  = "/libp2p/circuit/relay/0.2.0/hop"
	ProtoIDv2Stop = "/libp2p/circuit/relay/0.2.0/stop"

	ReservationTagWeight = 10

	maxMessageSize = 4096
	streamTimeout  = time.Minute
)

var log = logging.Logger("relay")

type Relay struct {
	host host.Host
	rc   Resources
	acl  ACLFilter

	mx      sync.Mutex
	rsvp    map[peer.ID]time.Time
	refresh map[peer.ID]time.Time
}

func New(h host.Host, opts ...Option) (*Relay, error) {
	r := &Relay{
		host: h,
		// TODO
	}

	for _, opt := range opts {
		err := opt(r)
		if err != nil {
			return nil, fmt.Errorf("error applying relay option: %w", err)
		}
	}

	h.SetStreamHandler(ProtoIDv2Hop, r.handleStream)

	// TODO network notifee for handling peer disconns and removing from rsvp table
	// TODO start background goroutine for cleaning up reservations

	return r, nil
}

func (r *Relay) Close() error {
	// TODO
	return nil
}

func (r *Relay) handleStream(s network.Stream) {
	s.SetReadDeadline(time.Now().Add(streamTimeout))

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
		r.refresh[p] = refresh.Add(r.rc.ReservationTTL / 2)
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
	r.refresh[p] = now.Add(r.rc.ReservationTTL / 2)
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
	// TODO
}

func (r *Relay) handleError(s network.Stream, status pbv2.Status) {
	log.Warnf("relay error: %s (%d)", pbv2.Status_name[int32(status)], status)
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
