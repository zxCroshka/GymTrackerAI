package auth

import (
	"sync"
	"time"
)

type rateEntry struct {
	start time.Time
	count int
}

// RateLimiter is a bounded, per-process fixed-window limiter. Distributed
// deployments must replace it with a shared edge/store policy.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
	limit   int
	window  time.Duration
	now     func() time.Time
	calls   uint64
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{entries: make(map[string]rateEntry), limit: limit, window: window, now: time.Now}
}

func (l *RateLimiter) Allow(key string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls++
	if l.calls%256 == 0 {
		for candidate, entry := range l.entries {
			if now.Sub(entry.start) >= l.window {
				delete(l.entries, candidate)
			}
		}
	}
	entry, exists := l.entries[key]
	if !exists || now.Sub(entry.start) >= l.window {
		l.entries[key] = rateEntry{start: now, count: 1}
		return true, 0
	}
	if entry.count >= l.limit {
		return false, l.window - now.Sub(entry.start)
	}
	entry.count++
	l.entries[key] = entry
	return true, 0
}
