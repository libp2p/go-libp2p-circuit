package client

import (
	"context"
	"fmt"
	"time"

	pbv2 "github.com/libp2p/go-libp2p-circuit/v2/pb"
	"github.com/libp2p/go-libp2p-circuit/v2/util"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

var ReserveTimeout = time.Minute

// Reservation is a struct carrying information about a relay/v2 slot reservation.
type Reservation struct {
	// Expiration is the expiration time of the reservation
	Expiration time.Time
	// Relay is the public addresses of the relay, which can be used for constructing
	// and advertising relay specific addresses.
	Relay peer.AddrInfo

	// LimitDuration is the time limit for which the relay will keep a relayed connection
	// open. If 0, there is no limit.
	LimitDuration time.Duration
	// LimitData is the number of bytes that the relay will relay in each direction before
	// resetting a relayed connection.
	LimitData int64

	// TODO reservation voucher
}

// Reserve reserves a slot in a relay and returns the reservation information.
// Clients must reserve slots in order for the relay to relay connections to them.
func Reserve(ctx context.Context, h host.Host, ai peer.AddrInfo) (*Reservation, error) {
	if len(ai.Addrs) > 0 {
		h.Peerstore().AddAddrs(ai.ID, ai.Addrs, peerstore.TempAddrTTL)
	}

	s, err := h.NewStream(ctx, ai.ID, ProtoIDv2Hop)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	rd := util.NewDelimitedReader(s, maxMessageSize)
	wr := util.NewDelimitedWriter(s)
	defer rd.Close()

	var msg pbv2.HopMessage
	msg.Type = pbv2.HopMessage_RESERVE.Enum()

	s.SetDeadline(time.Now().Add(ReserveTimeout))

	if err := wr.WriteMsg(&msg); err != nil {
		s.Reset()
		return nil, fmt.Errorf("error writing reservation message: %w", err)
	}

	msg.Reset()

	if err := rd.ReadMsg(&msg); err != nil {
		s.Reset()
		return nil, fmt.Errorf("error reading reservation response message: %w", err)
	}

	if msg.GetType() != pbv2.HopMessage_STATUS {
		return nil, fmt.Errorf("unexpected relay response: not a status message (%d)", msg.GetType())
	}

	if status := msg.GetStatus(); status != pbv2.Status_OK {
		return nil, fmt.Errorf("reservation failed: %s (%d)", pbv2.Status_name[int32(status)], status)
	}

	rsvp := msg.GetReservation()
	if rsvp == nil {
		return nil, fmt.Errorf("missing reservation info")
	}

	result := &Reservation{}
	result.Expiration = time.Now().Add(time.Duration(rsvp.GetTtl()) * time.Second)

	rinfo, err := util.PeerToPeerInfoV2(rsvp.GetRelay())
	if err != nil {
		return nil, fmt.Errorf("missing relay info")
	}
	result.Relay = rinfo

	limit := msg.GetLimit()
	if limit != nil {
		result.LimitDuration = time.Duration(limit.GetDuration()) * time.Second
		result.LimitData = limit.GetData()
	}

	return result, nil
}
