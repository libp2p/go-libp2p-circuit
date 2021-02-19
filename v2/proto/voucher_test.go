package proto

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

func TestReservationVoucher(t *testing.T) {
	relayPrivk, relayPubk, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Fatal(err)
	}

	_, peerPubk, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Fatal(err)
	}

	relayID, err := peer.IDFromPublicKey(relayPubk)
	if err != nil {
		t.Fatal(err)
	}

	peerID, err := peer.IDFromPublicKey(peerPubk)
	if err != nil {
		t.Fatal(err)
	}

	rsvp := &ReservationVoucher{
		Relay:      relayID,
		Peer:       peerID,
		Expiration: time.Now().Add(time.Hour),
	}

	err = rsvp.Sign(relayPrivk)
	if err != nil {
		t.Fatal(err)
	}

	blob, err := rsvp.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	rsvp2 := new(ReservationVoucher)
	err = rsvp2.Unmarshal(blob)
	if err != nil {
		t.Fatal(err)
	}

	err = rsvp2.Verify(relayPubk)
	if err != nil {
		t.Fatal(err)
	}

	if rsvp.Relay != rsvp2.Relay {
		t.Fatal("relay IDs don't match")
	}
	if rsvp.Peer != rsvp2.Peer {
		t.Fatal("peer IDs don't match")
	}
	if rsvp.Expiration.Unix() != rsvp2.Expiration.Unix() {
		t.Fatal("expirations don't match")
	}
}
