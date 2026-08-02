//go:build integration

package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
)

func TestPostgreSQLFoundation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required and must point to a disposable PostgreSQL database")
	}

	migrationsPath, err := filepath.Abs("../../../migrations")
	if err != nil {
		t.Fatalf("resolve migrations path: %v", err)
	}
	migrator, err := NewMigrator(databaseURL, "file://"+filepath.ToSlash(migrationsPath))
	if err != nil {
		t.Fatalf("initialize migrator: %v", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) && !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("reset disposable database: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("apply forward migration: %v", err)
	}

	cfg := integrationDatabaseConfig(databaseURL)
	pool, err := OpenPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}

	t.Run("pool sessions use UTC and complete schema exists", func(t *testing.T) {
		var timezone string
		if err := pool.QueryRow(context.Background(), "SHOW timezone").Scan(&timezone); err != nil {
			t.Fatalf("read session timezone: %v", err)
		}
		if timezone != "UTC" {
			t.Fatalf("session timezone = %q, want UTC", timezone)
		}

		var tableCount int
		if err := pool.QueryRow(context.Background(), `
			SELECT count(*)
			FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = ANY($1)
		`, expectedFoundationTables()).Scan(&tableCount); err != nil {
			t.Fatalf("count foundation tables: %v", err)
		}
		if tableCount != len(expectedFoundationTables()) {
			t.Fatalf("foundation table count = %d, want %d", tableCount, len(expectedFoundationTables()))
		}

		var timestampWithoutTimeZoneCount int
		if err := pool.QueryRow(context.Background(), `
			SELECT count(*)
			FROM information_schema.columns
			WHERE table_schema = 'public'
			AND table_name = ANY($1)
			AND data_type = 'timestamp without time zone'
		`, expectedFoundationTables()).Scan(&timestampWithoutTimeZoneCount); err != nil {
			t.Fatalf("inspect timestamp columns: %v", err)
		}
		if timestampWithoutTimeZoneCount != 0 {
			t.Fatalf("found %d timestamp columns without time zone", timestampWithoutTimeZoneCount)
		}
	})

	t.Run("transaction helper commits and rolls back", func(t *testing.T) {
		committedID := "10000000-0000-4000-8000-000000000001"
		if err := WithinTransaction(context.Background(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(context.Background(), `
				INSERT INTO users (id, email, password_hash)
				VALUES ($1, 'commit@example.test', 'argon2id-test-hash')
			`, committedID)
			return err
		}); err != nil {
			t.Fatalf("commit transaction: %v", err)
		}

		rollbackID := "10000000-0000-4000-8000-000000000002"
		rollbackMarker := errors.New("rollback marker")
		err := WithinTransaction(context.Background(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			if _, err := tx.Exec(context.Background(), `
				INSERT INTO users (id, email, password_hash)
				VALUES ($1, 'rollback@example.test', 'argon2id-test-hash')
			`, rollbackID); err != nil {
				return err
			}
			return rollbackMarker
		})
		if !errors.Is(err, rollbackMarker) {
			t.Fatalf("rollback error = %v", err)
		}

		var rollbackRows int
		if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE id = $1", rollbackID).Scan(&rollbackRows); err != nil {
			t.Fatalf("verify rollback: %v", err)
		}
		if rollbackRows != 0 {
			t.Fatalf("rolled-back user count = %d", rollbackRows)
		}
	})

	t.Run("tenant-safe foreign key rejects cross-user child", func(t *testing.T) {
		ownerID := "20000000-0000-4000-8000-000000000001"
		otherID := "20000000-0000-4000-8000-000000000002"
		programID := "20000000-0000-4000-8000-000000000003"
		dayID := "20000000-0000-4000-8000-000000000004"
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO users (id, email, password_hash) VALUES
			($1, 'owner@example.test', 'hash'),
			($2, 'other@example.test', 'hash')
		`, ownerID, otherID); err != nil {
			t.Fatalf("insert tenant fixtures: %v", err)
		}
		if _, err := pool.Exec(context.Background(), `
			INSERT INTO programs (id, user_id, name) VALUES ($1, $2, 'Program')
		`, programID, ownerID); err != nil {
			t.Fatalf("insert program fixture: %v", err)
		}

		_, err := pool.Exec(context.Background(), `
			INSERT INTO program_days (id, user_id, program_id, position, name)
			VALUES ($1, $2, $3, 1, 'Invalid day')
		`, dayID, otherID, programID)
		if err == nil {
			t.Fatal("cross-user program day unexpectedly succeeded")
		}
	})

	t.Run("system exercise seed is idempotent", func(t *testing.T) {
		firstAffected, err := SeedSystemExercises(context.Background(), pool)
		if err != nil {
			t.Fatalf("first seed: %v", err)
		}
		if firstAffected != int64(len(systemExerciseSeeds)) {
			t.Fatalf("first affected rows = %d, want %d", firstAffected, len(systemExerciseSeeds))
		}
		secondAffected, err := SeedSystemExercises(context.Background(), pool)
		if err != nil {
			t.Fatalf("second seed: %v", err)
		}
		if secondAffected != 0 {
			t.Fatalf("second affected rows = %d, want 0", secondAffected)
		}
	})

	pool.Close()
	if err := migrator.Down(); err != nil {
		t.Fatalf("apply rollback migration: %v", err)
	}

	verificationPool, err := OpenPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("reopen pool after rollback: %v", err)
	}
	defer verificationPool.Close()
	var usersTable *string
	if err := verificationPool.QueryRow(context.Background(), "SELECT to_regclass('public.users')").Scan(&usersTable); err != nil {
		t.Fatalf("verify rollback: %v", err)
	}
	if usersTable != nil {
		t.Fatalf("users table still exists after rollback: %s", *usersTable)
	}
}

func integrationDatabaseConfig(databaseURL string) config.DatabaseConfig {
	return config.DatabaseConfig{
		URL:               databaseURL,
		ConnectTimeout:    5 * time.Second,
		PingTimeout:       2 * time.Second,
		MinConnections:    1,
		MaxConnections:    4,
		MaxConnLifetime:   10 * time.Minute,
		MaxConnIdleTime:   time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}
}

func expectedFoundationTables() []string {
	return []string{
		"users",
		"user_profiles",
		"user_profile_notes",
		"refresh_tokens",
		"exercises",
		"programs",
		"program_days",
		"program_day_exercises",
		"workouts",
		"workout_exercises",
		"workout_sets",
		"body_measurements",
		"daily_wellness",
		"personal_records",
		"coach_conversations",
		"coach_messages",
		"coach_tool_calls",
		"coach_recommendations",
		"weekly_reports",
		"idempotency_keys",
		"audit_events",
	}
}
