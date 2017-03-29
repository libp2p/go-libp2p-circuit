package relay

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	bhost "github.com/libp2p/go-libp2p-blankhost"
	host "github.com/libp2p/go-libp2p-host"
	netutil "github.com/libp2p/go-libp2p-netutil"
	ma "github.com/multiformats/go-multiaddr"
)

/* TODO: add tests
- simple A -[R]-> B
- A tries to relay through R, R doesnt support relay
- A tries to relay through R to B, B doesnt support relay
- A sends invalid multiaddr
- A sends too long multiaddr
- R drops stream mid-message
- A relays through R, R has no connection to B
*/

func getNetHosts(t *testing.T, ctx context.Context, n int) []host.Host {
	var out []host.Host

	for i := 0; i < n; i++ {
		netw := netutil.GenSwarmNetwork(t, ctx)
		h := bhost.NewBlankHost(netw)
		out = append(out, h)
	}

	return out
}

func connect(t *testing.T, a, b host.Host) {
	pinfo := a.Peerstore().PeerInfo(a.ID())
	err := b.Connect(context.Background(), pinfo)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBasicRelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])

	r1, err := NewRelay(ctx, hosts[0])
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewRelay(ctx, hosts[1], OptHop)
	if err != nil {
		t.Fatal(err)
	}

	r3, err := NewRelay(ctx, hosts[2])
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("relay works!")
	go func() {
		list, err := r3.Listener()
		if err != nil {
			t.Error(err)
		}

		con, err := list.Accept()
		if err != nil {
			t.Error(err)
		}

		_, err = con.Write(msg)
		if err != nil {
			t.Error("failed to write", err)
		}
		con.Close()
	}()

	destma, err := ma.NewMultiaddr("/ipfs/" + hosts[2].ID().Pretty())
	if err != nil {
		t.Fatal(err)
	}

	con, err := r1.Dial(ctx, hosts[1].ID(), destma)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(con)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, msg) {
		t.Fatal("message was incorrect:", string(data))
	}
}
