package relay

import (
	"container/list"
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"math/rand"
	"sync"
	"time"

	asnutil "github.com/libp2p/go-libp2p-asn-util"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var validity = 30 * time.Minute

var (
	errTooManyReservations        = errors.New("too many reservations")
	errTooManyReservationsForPeer = errors.New("too many reservations for peer")
	errTooManyReservationsForIP   = errors.New("too many peers for IP address")
	errTooManyReservationsForASN  = errors.New("too many peers for ASN")
)

type listEntry struct {
	t time.Time
}

// constraints implements various reservation constraints
type constraints struct {
	rc *Resources

	mutex sync.Mutex
	rand  rand.Rand
	total *list.List
	peers map[peer.ID]*list.List
	ips   map[string]*list.List
	asns  map[string]*list.List
}

// newConstraints creates a new constraints object.
// The methods are *not* thread-safe; an external lock must be held if synchronization
// is required.
func newConstraints(rc *Resources) *constraints {
	b := make([]byte, 8)
	if _, err := crand.Read(b); err != nil {
		panic("failed to read from crypto/rand")
	}
	random := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(b))))

	return &constraints{
		rc:    rc,
		rand:  *random,
		total: list.New(),
		peers: make(map[peer.ID]*list.List),
		ips:   make(map[string]*list.List),
		asns:  make(map[string]*list.List),
	}
}

// AddReservation adds a reservation for a given peer with a given multiaddr.
// If adding this reservation violates IP constraints, an error is returned.
func (c *constraints) AddReservation(p peer.ID, a ma.Multiaddr) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	c.cleanup(now)

	if c.total.Len() >= c.rc.MaxReservations {
		return errTooManyReservations
	}

	ip, err := manet.ToIP(a)
	if err != nil {
		return errors.New("no IP address associated with peer")
	}

	peerReservations, ok := c.peers[p]
	if ok && peerReservations.Len() >= c.rc.MaxReservationsPerPeer {
		return errTooManyReservationsForPeer
	}

	ipReservations, ok := c.ips[ip.String()]
	if ok && ipReservations.Len() >= c.rc.MaxReservationsPerIP {
		return errTooManyReservationsForIP
	}

	var asnReservations *list.List
	var asn string
	if ip.To4() == nil {
		asn, _ = asnutil.Store.AsnForIPv6(ip)
		if asn != "" {
			var ok bool
			asnReservations, ok = c.asns[asn]
			if ok && asnReservations.Len() >= c.rc.MaxReservationsPerASN {
				return errTooManyReservationsForASN
			}
		}
	}

	c.total.PushBack(listEntry{t: now})

	if peerReservations == nil {
		peerReservations = list.New()
		c.peers[p] = peerReservations
	}
	peerReservations.PushBack(listEntry{t: now})

	if ipReservations == nil {
		ipReservations = list.New()
		c.ips[ip.String()] = ipReservations
	}
	ipReservations.PushBack(listEntry{t: now})

	if asn != "" {
		if asnReservations == nil {
			asnReservations = list.New()
			c.asns[asn] = asnReservations
		}
		asnReservations.PushBack(listEntry{t: now})
	}

	return nil
}

func (c *constraints) cleanupList(l *list.List, now time.Time) {
	for el := l.Front(); el != nil; {
		entry := el.Value.(listEntry)
		if entry.t.Add(validity).After(now) {
			return
		}
		nextEl := el.Next()
		l.Remove(el)
		el = nextEl
	}
}

func (c *constraints) cleanup(now time.Time) {
	c.cleanupList(c.total, now)
	for _, peerReservations := range c.peers {
		c.cleanupList(peerReservations, now)
	}
	for _, ipReservations := range c.ips {
		c.cleanupList(ipReservations, now)
	}
	for _, asnReservations := range c.asns {
		c.cleanupList(asnReservations, now)
	}
}
