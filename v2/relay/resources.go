package relay

import (
	"time"
)

type Resources struct {
	Limit *RelayLimit

	ReservationTTL  time.Duration
	MaxReservations int
	MaxCircuits     int
	BufferSize      int
}

type RelayLimit struct {
	Duration time.Duration
	Data     int64
}
