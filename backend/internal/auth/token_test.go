package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
)

func TestTokenManagerSeparatesPurposeAndExpires(t *testing.T) {
	cfg := config.AuthConfig{
		AccessSecret: "access-secret-with-at-least-32-characters", RefreshSecret: "refresh-secret-with-at-least-32-characters",
		Issuer: "issuer", AccessAudience: "access-audience", RefreshAudience: "refresh-audience", AccessTTL: time.Minute, RefreshTTL: time.Hour,
	}
	manager := NewTokenManager(cfg)
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	access, expiresAt, err := manager.issueAccess("user-id", "access-jti", "family-id", 2)
	if err != nil {
		t.Fatalf("issueAccess: %v", err)
	}
	if !expiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("expiresAt = %s", expiresAt)
	}
	claims, err := manager.parseAccess(access)
	if err != nil || claims.Subject != "user-id" || claims.AuthVersion != 2 {
		t.Fatalf("parseAccess = %+v, %v", claims, err)
	}
	if _, err := manager.parseRefresh(access); !errors.Is(err, errInvalidToken) {
		t.Fatalf("access accepted as refresh: %v", err)
	}
	manager.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := manager.parseAccess(access); !errors.Is(err, errInvalidToken) {
		t.Fatalf("expired access accepted: %v", err)
	}
}
