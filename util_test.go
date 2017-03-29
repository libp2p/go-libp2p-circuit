package relay

import (
	"bytes"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

func TestMultiaddrSerialization(t *testing.T) {
	buf := new(bytes.Buffer)

	addr, err := ma.NewMultiaddr("/ip4/1.2.3.4/tcp/1234")
	if err != nil {
		t.Fatal(err)
	}

	if err := writeLpMultiaddr(buf, addr); err != nil {
		t.Fatal(err)
	}

	out, err := readLpMultiaddr(buf)
	if err != nil {
		t.Fatal(err)
	}

	if !addr.Equal(out) {
		t.Fatal("addresses didnt match")
	}
}

func TestDecodeInvalid(t *testing.T) {
	buf := bytes.NewBuffer([]byte{72, 0, 0})
	_, err := readLpMultiaddr(buf)
	if err == nil {
		t.Fatal("shouldnt have parsed correctly")
	}

	buf = bytes.NewBuffer([]byte{0})
	_, err = readLpMultiaddr(buf)
	if err == nil {
		t.Fatal("shouldnt have parsed correctly")
	}
}
