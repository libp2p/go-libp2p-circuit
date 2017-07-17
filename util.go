package relay

import (
	"encoding/binary"
	"errors"
	"io"

	pb "github.com/libp2p/go-libp2p-circuit/pb"

	ggio "github.com/gogo/protobuf/io"
	proto "github.com/gogo/protobuf/proto"
	peer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
)

func peerToPeerInfo(p *pb.CircuitRelay_Peer) (empty pstore.PeerInfo, err error) {
	if p == nil {
		return empty, errors.New("nil peer")
	}

	h, err := mh.Cast(p.Id)
	if err != nil {
		return empty, err
	}

	addrs := make([]ma.Multiaddr, len(p.Addrs))
	for i := 0; i < len(addrs); i++ {
		a, err := ma.NewMultiaddrBytes(p.Addrs[i])
		if err != nil {
			return empty, err
		}
		addrs[i] = a
	}

	return pstore.PeerInfo{ID: peer.ID(h), Addrs: addrs}, nil
}

func peerInfoToPeer(pi pstore.PeerInfo) *pb.CircuitRelay_Peer {
	addrs := make([][]byte, len(pi.Addrs))
	for i := 0; i < len(addrs); i++ {
		addrs[i] = pi.Addrs[i].Bytes()
	}

	p := new(pb.CircuitRelay_Peer)
	p.Id = []byte(pi.ID)
	p.Addrs = addrs

	return p
}

type delimitedReader struct {
	r   io.Reader
	buf []byte
}

// the gogo protobuf NewDelimitedReader is buffered, which may eat up stream data
func newDelimitedReader(r io.Reader, maxSize int) *delimitedReader {
	return &delimitedReader{r: r, buf: make([]byte, maxSize)}
}

func (d *delimitedReader) ReadByte() (byte, error) {
	buf := d.buf[:1]
	_, err := d.r.Read(buf)
	return buf[0], err
}

func (d *delimitedReader) ReadMsg(msg proto.Message) error {
	mlen, err := binary.ReadUvarint(d)
	if err != nil {
		return err
	}

	if uint64(len(d.buf)) < mlen {
		return errors.New("Message too large")
	}

	buf := d.buf[:mlen]
	_, err = io.ReadFull(d.r, buf)
	if err != nil {
		return err
	}

	return proto.Unmarshal(buf, msg)
}

func newDelimitedWriter(w io.Writer) ggio.WriteCloser {
	return ggio.NewDelimitedWriter(w)
}
