package httpserver

import "sync/atomic"

// Readiness represents whether this application instance can accept traffic.
type Readiness struct {
	ready atomic.Bool
}

func NewReadiness() *Readiness {
	return &Readiness{}
}

func (r *Readiness) SetReady(ready bool) {
	r.ready.Store(ready)
}

func (r *Readiness) Ready() bool {
	return r.ready.Load()
}
