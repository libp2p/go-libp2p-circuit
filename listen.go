package relay

import (
	"net"

	peer "github.com/libp2p/go-libp2p-peer"
	tpt "github.com/libp2p/go-libp2p-transport"
	filter "github.com/libp2p/go-maddr-filter"
	ma "github.com/multiformats/go-multiaddr"
)

var _ tpt.Listener = (*RelayListener)(nil)

type RelayListener Relay

func (l *RelayListener) Relay() *Relay {
	return (*Relay)(l)
}

func (r *Relay) Listener() (tpt.Listener, error) {
	return (*RelayListener)(r), nil
}

func (r *Relay) Matches(a ma.Multiaddr) bool {
	return false
}

func (l *RelayListener) Accept() (tpt.Conn, error) {
	ctx := l.Relay().ctx
	select {
	case c := <-l.incoming:
		log.Infof("accepted relay connection: %s", c.ID())
		s := RelayStatus{
			Code:    StatusOK,
			Message: "OK",
		}
		if err := s.WriteTo(c); err != nil {
			return nil, err
		}
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *RelayListener) Addr() net.Addr {
	panic("oh no")
}

func (l *RelayListener) Multiaddr() ma.Multiaddr {
	panic("oh no")
}

func (l *RelayListener) LocalPeer() peer.ID {
	return l.Relay().self
}

func (l *RelayListener) SetAddrFilters(f *filter.Filters) {
	// noop ?
}

func (l *RelayListener) Close() error {
	// TODO: noop?
	return nil
}
