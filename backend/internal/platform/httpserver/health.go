package httpserver

import (
	"context"
	"sync/atomic"
	"time"
)

// DatabasePinger is the narrow pgxpool capability required by readiness.
type DatabasePinger interface {
	Ping(context.Context) error
}

// Readiness represents whether this application instance can accept traffic.
type Readiness struct {
	ready       atomic.Bool
	database    DatabasePinger
	pingTimeout time.Duration
}

func NewReadiness(database DatabasePinger, pingTimeout time.Duration) *Readiness {
	return &Readiness{database: database, pingTimeout: pingTimeout}
}

func (r *Readiness) SetReady(ready bool) {
	r.ready.Store(ready)
}

func (r *Readiness) Ready(ctx context.Context) bool {
	if !r.ready.Load() || r.database == nil {
		return false
	}

	pingContext, cancelPing := context.WithTimeout(ctx, r.pingTimeout)
	defer cancelPing()
	return r.database.Ping(pingContext) == nil
}
