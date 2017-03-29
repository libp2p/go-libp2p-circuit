package relay

import (
	"encoding/binary"
	"fmt"
	"io"
)

const maxStatusMessageLength = 1024

const (
	StatusOK = 100

	StatusRelayAddrErr      = 250
	StatusRelayNotConnected = 260
	StatusRelayDialFailed   = 261
	StatusRelayStreamFailed = 262
	StatusRelayHopToSelf    = 270

	StatusDstAddrErr      = 350
	StatusDstRelayRefused = 380
	StatusDstWrongDst     = 381
)

type RelayStatus struct {
	Code    uint64
	Message string
}

func (s *RelayStatus) WriteTo(w io.Writer) error {
	outbuf := make([]byte, 2*binary.MaxVarintLen64+len(s.Message))
	n := binary.PutUvarint(outbuf, s.Code)
	n += binary.PutUvarint(outbuf[n:], uint64(len(s.Message)))
	n += copy(outbuf[n:], s.Message)
	_, err := w.Write(outbuf[:n])
	return err
}

func (s *RelayStatus) ReadFrom(r io.Reader) error {
	br := &singleByteReader{r}
	code, err := binary.ReadUvarint(br)
	if err != nil {
		return err
	}
	l, err := binary.ReadUvarint(br)
	if err != nil {
		return err
	}
	buf := make([]byte, l)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return err
	}

	s.Code = code
	s.Message = string(buf)
	return nil
}

func (s *RelayStatus) Error() string {
	return fmt.Sprintf("%d: %s", s.Code, s.Message)
}
