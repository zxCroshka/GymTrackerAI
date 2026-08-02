package database

import (
	"strings"
	"testing"
	"time"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
)

func TestPoolConfigAppliesLimitsTimeoutAndUTC(t *testing.T) {
	cfg := config.DatabaseConfig{
		URL:               "postgres://gymtracker:secret@localhost:5432/gymtracker",
		ConnectTimeout:    4 * time.Second,
		PingTimeout:       time.Second,
		MinConnections:    2,
		MaxConnections:    9,
		MaxConnLifetime:   20 * time.Minute,
		MaxConnIdleTime:   3 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}

	poolCfg, err := poolConfig(cfg)
	if err != nil {
		t.Fatalf("configure pool: %v", err)
	}

	if poolCfg.MinConns != 2 || poolCfg.MaxConns != 9 {
		t.Fatalf("pool limits = %d/%d", poolCfg.MinConns, poolCfg.MaxConns)
	}
	if poolCfg.ConnConfig.ConnectTimeout != 4*time.Second {
		t.Fatalf("connect timeout = %v", poolCfg.ConnConfig.ConnectTimeout)
	}
	if poolCfg.ConnConfig.RuntimeParams["timezone"] != "UTC" {
		t.Fatalf("timezone = %q", poolCfg.ConnConfig.RuntimeParams["timezone"])
	}
	if poolCfg.MaxConnLifetime != 20*time.Minute || poolCfg.MaxConnIdleTime != 3*time.Minute {
		t.Fatalf("unexpected connection lifecycle: %+v", poolCfg)
	}
}

func TestPoolConfigDoesNotEchoInvalidURL(t *testing.T) {
	secretURL := "postgres://user:private-value@%gh&%ij"
	_, err := poolConfig(config.DatabaseConfig{URL: secretURL})
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
	if strings.Contains(err.Error(), "private-value") {
		t.Fatalf("error leaked URL credential: %v", err)
	}
}
