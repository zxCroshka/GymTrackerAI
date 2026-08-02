package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/logging"
)

func main() {
	logger := logging.Bootstrap(os.Stderr)
	if err := run(logger); err != nil {
		logger.Error("database seed failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	databaseConfig, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("load database configuration: %w", err)
	}

	pool, err := database.OpenPool(context.Background(), databaseConfig)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer pool.Close()

	affected, err := database.SeedSystemExercises(context.Background(), pool)
	if err != nil {
		return err
	}
	logger.Info("system exercise seed completed", slog.Int64("affected_rows", affected))
	return nil
}
