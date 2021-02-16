package relay

import (
	"errors"
	"net"

	"github.com/libp2p/go-libp2p-core/peer"

	asnutil "github.com/libp2p/go-libp2p-asn-util"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var (
	ErrNoIP              = errors.New("no IP address associated with peer")
	ErrTooManyPeersInIP  = errors.New("too many peers in IP address")
	ErrTooManyPeersInASN = errors.New("too many peers in ASN")
)

// IPConstraints implements reservation constraints per IP
type IPConstraints struct {
	iplimit, asnlimit int

	peers map[peer.ID]net.IP
	ips   map[string]map[peer.ID]struct{}
	asns  map[string]map[peer.ID]struct{}
}

// NewIPConstraints creates a new IPConstraints object.
// The methods are *not* thread-safe; an external lock must be held if synchronization
// is required.
func NewIPConstraints(rc Resources) *IPConstraints {
	return &IPConstraints{
		iplimit:  rc.MaxReservationsPerIP,
		asnlimit: rc.MaxReservationsPerASN,

		peers: make(map[peer.ID]net.IP),
		ips:   make(map[string]map[peer.ID]struct{}),
		asns:  make(map[string]map[peer.ID]struct{}),
	}
}

// AddReservation adds a reservation for a given peer with a given multiaddr.
// If adding this reservation violates IP constraints, an error is returned.
func (ipcs *IPConstraints) AddReservation(p peer.ID, a ma.Multiaddr) error {
	ip, err := manet.ToIP(a)
	if err != nil {
		return ErrNoIP
	}

	ips := ip.String()
	peersInIP := ipcs.ips[ips]
	if len(peersInIP) >= ipcs.iplimit {
		return ErrTooManyPeersInIP
	}

	var peersInAsn map[peer.ID]struct{}
	asn, _ := asnutil.Store.AsnForIPv6(ip)
	peersInAsn = ipcs.asns[asn]
	if len(peersInAsn) >= ipcs.asnlimit {
		return ErrTooManyPeersInASN
	}

	ipcs.peers[p] = ip

	if peersInIP == nil {
		peersInIP = make(map[peer.ID]struct{})
		ipcs.ips[ips] = peersInIP
	}
	peersInIP[p] = struct{}{}

	if asn != "" {
		if peersInAsn == nil {
			peersInAsn = make(map[peer.ID]struct{})
			ipcs.asns[asn] = peersInAsn
		}
		peersInAsn[p] = struct{}{}
	}

	return nil
}

// RemoveReservation removes a peer from the constraints.
func (ipcs *IPConstraints) RemoveReservation(p peer.ID) {
	ip, ok := ipcs.peers[p]
	if !ok {
		return
	}

	ips := ip.String()
	asn, _ := asnutil.Store.AsnForIPv6(ip)

	delete(ipcs.peers, p)

	peersInIP, ok := ipcs.ips[ips]
	if ok {
		delete(peersInIP, p)
		if len(peersInIP) == 0 {
			delete(ipcs.ips, ips)
		}
	}

	peersInAsn, ok := ipcs.asns[asn]
	if ok {
		delete(peersInAsn, p)
		if len(peersInAsn) == 0 {
			delete(ipcs.asns, asn)
		}
	}
}
