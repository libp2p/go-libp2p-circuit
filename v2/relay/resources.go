package relay

import (
	"time"
)

type Resources struct {
	Limit *RelayLimit

	ReservationTTL        time.Duration
	ReservationRefreshTTL time.Duration

	MaxReservations int
	MaxCircuits     int
	BufferSize      int
}

type RelayLimit struct {
	Duration time.Duration
	Data     int64
}

func DefaultResources() Resources {
	return Resources{
		Limit: DefaultLimit(),

		ReservationTTL:        time.Hour,
		ReservationRefreshTTL: 15 * time.Minute,

		MaxReservations: 1024,
		MaxCircuits:     16,
		BufferSize:      2048,
	}
}

func DefaultLimit() *RelayLimit {
	return &RelayLimit{
		Duration: time.Minute,
		Data:     65536,
	}
}
