package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

type ReservationVoucher struct {
	// Relay is the ID of the peer providing relay service
	Relay peer.ID
	// Peer is the ID of the peer receiving relay service through Relay
	Peer peer.ID
	// Expiration is the expiration time of the reservation
	Expiration time.Time
	// Signature is the signature of this voucher, as produced by the Relay peer
	Signature []byte
}

func (rv *ReservationVoucher) bytes() []byte {
	buf := make([]byte, 1024)
	relayBytes := []byte(rv.Relay)
	peerBytes := []byte(rv.Peer)
	expireUnix := rv.Expiration.Unix()

	n := binary.PutUvarint(buf, uint64(len(relayBytes)))
	n += copy(buf[n:], relayBytes)
	n += binary.PutUvarint(buf[n:], uint64(len(peerBytes)))
	n += copy(buf[n:], peerBytes)
	n += binary.PutUvarint(buf[n:], uint64(expireUnix))

	return buf[:n]
}

func (rv *ReservationVoucher) Sign(privk crypto.PrivKey) error {
	if rv.Signature != nil {
		return nil
	}

	blob := append([]byte("libp2p-relay-rsvp:"), rv.bytes()...)

	sig, err := privk.Sign(blob)
	if err != nil {
		return err
	}

	rv.Signature = sig
	return nil
}

func (rv *ReservationVoucher) Verify(pubk crypto.PubKey) error {
	if rv.Signature == nil {
		return fmt.Errorf("missing signature")
	}

	blob := append([]byte("libp2p-relay-rsvp:"), rv.bytes()...)

	ok, err := pubk.Verify(blob, rv.Signature)
	if err != nil {
		return fmt.Errorf("signature verification error: %w", err)
	}
	if !ok {
		return fmt.Errorf("signature verifcation failed")
	}

	return nil
}

func (rv *ReservationVoucher) Marshal() ([]byte, error) {
	if rv.Signature == nil {
		return nil, fmt.Errorf("cannot marshal unsigned reservation voucher")
	}

	blob := rv.bytes()
	result := make([]byte, len(blob)+len(rv.Signature))
	copy(result, blob)
	copy(result[len(blob):], rv.Signature)

	return result, nil
}

func (rv *ReservationVoucher) Unmarshal(blob []byte) error {
	rd := bytes.NewReader(blob)

	readID := func() (peer.ID, error) {
		idLen, err := binary.ReadUvarint(rd)
		if err != nil {
			return "", fmt.Errorf("error reading ID length: %w", err)
		}
		if idLen > uint64(rd.Len()) {
			return "", fmt.Errorf("error reading ID: ID length exceeds available bytes")
		}

		idBytes := make([]byte, int(idLen))
		n, err := rd.Read(idBytes)
		if err != nil {
			return "", fmt.Errorf("error reading ID: %w", err)
		}
		if n != len(idBytes) {
			return "", fmt.Errorf("error reading ID: not enough bytes read")
		}

		return peer.IDFromBytes(idBytes)
	}

	var err error

	rv.Relay, err = readID()
	if err != nil {
		return fmt.Errorf("error reading relay ID: %w", err)
	}

	rv.Peer, err = readID()
	if err != nil {
		return fmt.Errorf("error reading peer ID: %w", err)
	}

	expireUnix, err := binary.ReadUvarint(rd)
	if err != nil {
		return fmt.Errorf("error reading reservation expiration: %w", err)
	}
	rv.Expiration = time.Unix(int64(expireUnix), 0)

	sig := make([]byte, rd.Len())
	n, err := rd.Read(sig)
	if err != nil {
		return fmt.Errorf("error reading signature: %w", err)
	}
	if n != len(sig) {
		return fmt.Errorf("error reading signature: not enough bytes read")
	}
	rv.Signature = sig

	return nil
}
