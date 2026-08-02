package database

import (
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// NewMigrator constructs the shared golang-migrate runner used by the one-shot
// migration command and PostgreSQL integration tests.
func NewMigrator(databaseURL, sourceURL string) (*migrate.Migrate, error) {
	migrator, err := migrate.New(sourceURL, migrationDatabaseURL(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("initialize migrations: %w", err)
	}
	return migrator, nil
}

func migrationDatabaseURL(databaseURL string) string {
	switch {
	case strings.HasPrefix(databaseURL, "postgres://"):
		return "pgx5://" + strings.TrimPrefix(databaseURL, "postgres://")
	case strings.HasPrefix(databaseURL, "postgresql://"):
		return "pgx5://" + strings.TrimPrefix(databaseURL, "postgresql://")
	default:
		return databaseURL
	}
}
