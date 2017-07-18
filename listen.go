package relay

import (
	"net"

	pb "github.com/libp2p/go-libp2p-circuit/pb"

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
	select {
	case c := <-l.incoming:
		err := l.Relay().writeResponse(c.Stream, pb.CircuitRelay_SUCCESS)
		if err != nil {
			log.Debugf("error writing relay response: %s", err.Error())
			// this won't prevent the other side from continuing to write
			// TODO fully close the stream when Reset is implemented
			c.Stream.Close()
			return nil, err
		}

		log.Infof("accepted relay connection: %s", c.ID())

		return c, nil
	case <-l.ctx.Done():
		return nil, l.ctx.Err()
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
