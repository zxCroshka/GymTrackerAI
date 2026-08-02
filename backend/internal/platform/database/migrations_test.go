package database

import "testing"

func TestMigrationDatabaseURLUsesPGXV5Driver(t *testing.T) {
	tests := map[string]string{
		"postgres://host/database?sslmode=disable":   "pgx5://host/database?sslmode=disable",
		"postgresql://host/database?sslmode=require": "pgx5://host/database?sslmode=require",
		"pgx5://host/database":                       "pgx5://host/database",
	}

	for input, want := range tests {
		if got := migrationDatabaseURL(input); got != want {
			t.Errorf("migrationDatabaseURL(%q) = %q, want %q", input, got, want)
		}
	}
}
