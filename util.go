package relay

import (
	"encoding/binary"
	"fmt"
	"io"

	ma "github.com/multiformats/go-multiaddr"
)

type singleByteReader struct {
	r io.Reader
}

func (s *singleByteReader) ReadByte() (byte, error) {
	var b [1]byte
	n, err := s.r.Read(b[:])
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.ErrNoProgress
	}

	return b[0], nil
}

func writeLpMultiaddr(w io.Writer, a ma.Multiaddr) error {
	buf := make([]byte, binary.MaxVarintLen32+len(a.Bytes()))
	n := binary.PutUvarint(buf, uint64(len(a.Bytes())))
	n += copy(buf[n:], a.Bytes())
	nw, err := w.Write(buf[:n])
	if err != nil {
		return err
	}
	if n != nw {
		return fmt.Errorf("failed to write all bytes to writer")
	}
	return nil
}

func readLpMultiaddr(r io.Reader) (ma.Multiaddr, error) {
	l, err := binary.ReadUvarint(&singleByteReader{r})
	if err != nil {
		return nil, err
	}

	if l > maxAddrLen {
		return nil, fmt.Errorf("address length was too long: %d > %d", l, maxAddrLen)
	}

	if l == 0 {
		return nil, fmt.Errorf("zero length multiaddr is invalid")
	}

	buf := make([]byte, l)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	return ma.NewMultiaddrBytes(buf)
}
