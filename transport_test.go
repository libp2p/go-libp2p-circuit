package relay_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	. "github.com/libp2p/go-libp2p-circuit"

	host "github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

const TestProto = "test/relay-transport"

func TestRelayTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	err := AddRelayTransport(ctx, hosts[0])
	if err != nil {
		t.Fatal(err)
	}

	err = AddRelayTransport(ctx, hosts[1], OptHop)
	if err != nil {
		t.Fatal(err)
	}

	err = AddRelayTransport(ctx, hosts[2])
	if err != nil {
		t.Fatal(err)
	}

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])

	time.Sleep(100 * time.Millisecond)

	msg := []byte("relay works!")
	handler := func(s inet.Stream) {
		_, err := s.Write(msg)
		if err != nil {
			t.Error(err)
		}
		s.Close()
	}

	hosts[2].SetStreamHandler(TestProto, handler)

	testFullAddressDial(t, hosts, msg)
	testSpecificRelayDial(t, hosts, msg)
	testUnspecificRelayDial(t, hosts, msg)
}

func testFullAddressDial(t *testing.T, hosts []host.Host, msg []byte) {
	addr, err := ma.NewMultiaddr(fmt.Sprintf("%s/ipfs/%s/p2p-circuit/ipfs/%s", hosts[1].Addrs()[0].String(), hosts[1].ID().Pretty(), hosts[2].ID().Pretty()))
	if err != nil {
		t.Fatal(err)
	}

	rctx, rcancel := context.WithTimeout(context.Background(), time.Second)
	defer rcancel()

	hosts[0].Peerstore().AddAddrs(hosts[2].ID(), []ma.Multiaddr{addr}, pstore.TempAddrTTL)

	s, err := hosts[0].NewStream(rctx, hosts[2].ID(), TestProto)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(s)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, msg) {
		t.Fatal("message was incorrect:", string(data))
	}
}

func testSpecificRelayDial(t *testing.T, hosts []host.Host, msg []byte) {
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s/p2p-circuit/ipfs/%s", hosts[1].ID().Pretty(), hosts[2].ID().Pretty()))
	if err != nil {
		t.Fatal(err)
	}

	rctx, rcancel := context.WithTimeout(context.Background(), time.Second)
	defer rcancel()

	hosts[0].Peerstore().AddAddrs(hosts[2].ID(), []ma.Multiaddr{addr}, pstore.TempAddrTTL)

	s, err := hosts[0].NewStream(rctx, hosts[2].ID(), TestProto)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(s)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, msg) {
		t.Fatal("message was incorrect:", string(data))
	}
}

func testUnspecificRelayDial(t *testing.T, hosts []host.Host, msg []byte) {
	addr, err := ma.NewMultiaddr(fmt.Sprintf("/p2p-circuit/ipfs/%s", hosts[2].ID().Pretty()))
	if err != nil {
		t.Fatal(err)
	}

	rctx, rcancel := context.WithTimeout(context.Background(), time.Second)
	defer rcancel()

	hosts[0].Peerstore().AddAddrs(hosts[2].ID(), []ma.Multiaddr{addr}, pstore.TempAddrTTL)

	s, err := hosts[0].NewStream(rctx, hosts[2].ID(), TestProto)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(s)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, msg) {
		t.Fatal("message was incorrect:", string(data))
	}
}
