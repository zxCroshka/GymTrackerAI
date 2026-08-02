package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/logging"
)

const defaultMigrationsPath = "migrations"

var migrationNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func main() {
	logger := logging.Bootstrap(os.Stderr)
	if err := run(os.Args[1:], os.Getenv, time.Now); err != nil {
		logger.Error("migration command failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, now func() time.Time) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: migrate up|down|create <name>")
	}

	migrationsPath := getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = defaultMigrationsPath
	}

	if args[0] == "create" {
		if len(args) != 2 {
			return fmt.Errorf("usage: migrate create <snake_case_name>")
		}
		return createMigration(migrationsPath, args[1], now().UTC())
	}
	if len(args) != 1 || (args[0] != "up" && args[0] != "down") {
		return fmt.Errorf("usage: migrate up|down|create <name>")
	}

	databaseConfig, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("load database configuration: %w", err)
	}

	absolutePath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return fmt.Errorf("resolve migrations path: %w", err)
	}
	migrator, err := database.NewMigrator(databaseConfig.URL, "file://"+filepath.ToSlash(absolutePath))
	if err != nil {
		return err
	}

	switch args[0] {
	case "up":
		err = migrator.Up()
	case "down":
		err = migrator.Steps(-1)
	}
	sourceCloseErr, databaseCloseErr := migrator.Close()
	closeErr := errors.Join(sourceCloseErr, databaseCloseErr)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return errors.Join(fmt.Errorf("migrate %s: %w", args[0], err), closeErr)
	}
	return closeErr
}

func createMigration(directory, name string, now time.Time) error {
	if !migrationNamePattern.MatchString(name) {
		return fmt.Errorf("migration name must be lower snake_case and start with a letter")
	}
	info, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("open migrations directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("migrations path is not a directory")
	}

	version := now.Format("20060102150405")
	base := filepath.Join(directory, version+"_"+name)
	upPath := base + ".up.sql"
	downPath := base + ".down.sql"

	upFile, err := os.OpenFile(upPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create forward migration: %w", err)
	}
	if _, err := upFile.WriteString("SET TIME ZONE 'UTC';\n"); err != nil {
		_ = upFile.Close()
		_ = os.Remove(upPath)
		return fmt.Errorf("initialize forward migration: %w", err)
	}
	if err := upFile.Close(); err != nil {
		return fmt.Errorf("close forward migration: %w", err)
	}

	downFile, err := os.OpenFile(downPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		_ = os.Remove(upPath)
		return fmt.Errorf("create rollback migration: %w", err)
	}
	if _, err := downFile.WriteString("SET TIME ZONE 'UTC';\n"); err != nil {
		_ = downFile.Close()
		_ = os.Remove(upPath)
		_ = os.Remove(downPath)
		return fmt.Errorf("initialize rollback migration: %w", err)
	}
	if err := downFile.Close(); err != nil {
		return fmt.Errorf("close rollback migration: %w", err)
	}

	fmt.Printf("created %s and %s\n", upPath, downPath)
	return nil
}
