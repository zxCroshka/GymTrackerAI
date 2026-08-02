package auth

import (
	"testing"
	"time"
)

func TestRateLimiterSeparatesKeysAndResetsWindow(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	if allowed, _ := limiter.Allow("first"); !allowed {
		t.Fatal("first request rejected")
	}
	if allowed, _ := limiter.Allow("first"); !allowed {
		t.Fatal("second request rejected")
	}
	if allowed, retry := limiter.Allow("first"); allowed || retry != time.Minute {
		t.Fatalf("third request = %v, %v", allowed, retry)
	}
	if allowed, _ := limiter.Allow("second"); !allowed {
		t.Fatal("independent key rejected")
	}
	now = now.Add(time.Minute)
	if allowed, _ := limiter.Allow("first"); !allowed {
		t.Fatal("request after window rejected")
	}
}
