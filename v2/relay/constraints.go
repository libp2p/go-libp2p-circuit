package relay

import (
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

var cleanupInterval = 2 * time.Minute
var validity = 30 * time.Minute

var (
	errTooManyReservations        = errors.New("too many reservations")
	errTooManyReservationsForPeer = errors.New("too many reservations for peer")
	errTooManyReservationsForIP   = errors.New("too many peers for IP address")
	errTooManyReservationsForASN  = errors.New("too many peers for ASN")
)

// constraints implements various reservation constraints
type constraints struct {
	rc *Resources

	closed                  bool
	closing, cleanupRunning chan struct{}

	mutex sync.Mutex
	rand  rand.Rand
	total map[uint64]time.Time
	peers map[peer.ID]map[uint64]time.Time
	ips   map[string]map[uint64]time.Time
	asns  map[string]map[uint64]time.Time
}

// NewConstraints creates a new constraints object.
// The methods are *not* thread-safe; an external lock must be held if synchronization
// is required.
func NewConstraints(rc *Resources) *constraints {
	b := make([]byte, 8)
	crand.Read(b)
	random := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(b))))

	c := &constraints{
		rc:             rc,
		closing:        make(chan struct{}),
		cleanupRunning: make(chan struct{}),
		rand:           *random,
		total:          make(map[uint64]time.Time),
		peers:          make(map[peer.ID]map[uint64]time.Time),
		ips:            make(map[string]map[uint64]time.Time),
		asns:           make(map[string]map[uint64]time.Time),
	}
	go c.cleanup()
	return c
}

// AddReservation adds a reservation for a given peer with a given multiaddr.
// If adding this reservation violates IP constraints, an error is returned.
func (c *constraints) AddReservation(p peer.ID, a ma.Multiaddr) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if len(c.total) >= c.rc.MaxReservations {
		return errTooManyReservations
	}

	ip, err := manet.ToIP(a)
	if err != nil {
		return errors.New("no IP address associated with peer")
	}

	peerReservations := c.peers[p]
	if len(peerReservations) >= c.rc.MaxReservationsPerPeer {
		return errTooManyReservationsForPeer
	}

	ipStr := ip.String()
	ipReservations := c.ips[ipStr]
	if len(ipReservations) >= c.rc.MaxReservationsPerIP {
		return errTooManyReservationsForIP
	}

	var ansReservations map[uint64]time.Time
	var asn string
	if ip.To4() == nil {
		asn, _ = asnutil.Store.AsnForIPv6(ip)
		if asn != "" {
			ansReservations = c.asns[asn]
			if len(ansReservations) >= c.rc.MaxReservationsPerASN {
				return errTooManyReservationsForASN
			}
		}
	}

	now := time.Now()
	id := c.rand.Uint64()

	c.total[id] = now

	if peerReservations == nil {
		peerReservations = make(map[uint64]time.Time)
		c.peers[p] = peerReservations
	}
	peerReservations[id] = now

	if ipReservations == nil {
		ipReservations = make(map[uint64]time.Time)
		c.ips[ipStr] = ipReservations
	}
	ipReservations[id] = now

	if asn != "" {
		if ansReservations == nil {
			ansReservations = make(map[uint64]time.Time)
			c.asns[asn] = ansReservations
		}
		ansReservations[id] = now
	}

	return nil
}

func (c *constraints) cleanup() {
	defer close(c.cleanupRunning)
	closeChan := c.closing
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-closeChan:
			return
		case now := <-ticker.C:
			c.mutex.Lock()
			for id, t := range c.total {
				if t.Add(validity).Before(now) {
					delete(c.total, id)
				}
			}
			for p, values := range c.peers {
				for id, t := range values {
					if t.Add(validity).Before(now) {
						delete(values, id)
					}
				}
				if len(values) == 0 {
					delete(c.peers, p)
				}
			}
			for ip, values := range c.ips {
				for id, t := range values {
					if t.Add(validity).Before(now) {
						delete(values, id)
					}
				}
				if len(values) == 0 {
					delete(c.ips, ip)
				}
			}
			for asn, values := range c.asns {
				for id, t := range values {
					if t.Add(validity).Before(now) {
						delete(values, id)
					}
				}
				if len(values) == 0 {
					delete(c.asns, asn)
				}
			}
			c.mutex.Unlock()
		}
	}
}

func (c *constraints) Close() {
	if !c.closed {
		close(c.closing)
		c.closed = true
		<-c.cleanupRunning
	}
}
