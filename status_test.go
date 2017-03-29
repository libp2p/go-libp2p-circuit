package relay

import (
	"bytes"
	"testing"
)

func TestStatusParsing(t *testing.T) {
	s := &RelayStatus{
		Code:    StatusDstAddrErr,
		Message: "foo bar",
	}

	buf := new(bytes.Buffer)

	if err := s.WriteTo(buf); err != nil {
		t.Fatal(err)
	}

	var ns RelayStatus
	if err := ns.ReadFrom(buf); err != nil {
		t.Fatal(err)
	}

	if ns.Code != s.Code {
		t.Fatal("codes didnt match")
	}

	if ns.Message != s.Message {
		t.Fatal("messages didnt match")
	}
}
