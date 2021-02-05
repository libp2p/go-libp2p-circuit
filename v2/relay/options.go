package relay

type Option func(*Relay) error

func WithResources(rc Resources) Option {
	return func(r *Relay) error {
		r.rc = rc
		return nil
	}
}

func WithLimit(limit *RelayLimit) Option {
	return func(r *Relay) error {
		r.rc.Limit = limit
		return nil
	}
}
