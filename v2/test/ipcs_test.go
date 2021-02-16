package test

import (
	"fmt"
	"net"
	"testing"

	"github.com/libp2p/go-libp2p-circuit/v2/relay"

	"github.com/libp2p/go-libp2p-core/peer"

	ma "github.com/multiformats/go-multiaddr"
)

func TestIPConstraints(t *testing.T) {
	ipcs := relay.NewIPConstraints(relay.Resources{
		MaxReservationsPerIP:  1,
		MaxReservationsPerASN: 2,
	})

	peerA := peer.ID("A")
	peerB := peer.ID("B")
	peerC := peer.ID("C")
	peerD := peer.ID("D")
	peerE := peer.ID("E")

	ipA := net.ParseIP("1.2.3.4")
	ipB := ipA
	ipC := net.ParseIP("2001:200::1")
	ipD := net.ParseIP("2001:200::2")
	ipE := net.ParseIP("2001:200::3")

	err := ipcs.AddReservation(peerA, ma.StringCast(fmt.Sprintf("/ip4/%s/tcp/1234", ipA)))
	if err != nil {
		t.Fatal(err)
	}

	err = ipcs.AddReservation(peerB, ma.StringCast(fmt.Sprintf("/ip4/%s/tcp/1234", ipB)))
	if err != relay.ErrTooManyPeersInIP {
		t.Fatalf("unexpected error: %s", err)
	}

	ipcs.RemoveReservation(peerA)
	err = ipcs.AddReservation(peerB, ma.StringCast(fmt.Sprintf("/ip4/%s/tcp/1234", ipB)))
	if err != nil {
		t.Fatal(err)
	}

	err = ipcs.AddReservation(peerC, ma.StringCast(fmt.Sprintf("/ip6/%s/tcp/1234", ipC)))
	if err != nil {
		t.Fatal(err)
	}

	err = ipcs.AddReservation(peerD, ma.StringCast(fmt.Sprintf("/ip6/%s/tcp/1234", ipD)))
	if err != nil {
		t.Fatal(err)
	}

	err = ipcs.AddReservation(peerE, ma.StringCast(fmt.Sprintf("/ip6/%s/tcp/1234", ipE)))
	if err != relay.ErrTooManyPeersInASN {
		t.Fatalf("unexpected error: %s", err)
	}

	ipcs.RemoveReservation(peerD)
	err = ipcs.AddReservation(peerE, ma.StringCast(fmt.Sprintf("/ip6/%s/tcp/1234", ipE)))
	if err != nil {
		t.Fatal(err)
	}
}
