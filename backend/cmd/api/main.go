package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/auth"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/measurement"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/logging"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/user"
)

func main() {
	if err := run(); err != nil {
		bootstrapLogger().Error("application stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger, err := logging.New(os.Stdout, cfg.LogLevel, cfg.AppName, cfg.Environment)
	if err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}

	pool, err := database.OpenPool(context.Background(), cfg.Database)
	if err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}
	defer pool.Close()
	logger.Info(
		"PostgreSQL pool ready",
		slog.Int("min_connections", int(cfg.Database.MinConnections)),
		slog.Int("max_connections", int(cfg.Database.MaxConnections)),
	)

	userRepository := user.NewRepository(pool)
	measurementRepository := measurement.NewRepository()
	measurementWriter := user.InitialMeasurementWriterFunc(func(ctx context.Context, tx pgx.Tx, value user.InitialMeasurement) error {
		return measurementRepository.InsertInitial(ctx, tx, measurement.InitialMeasurement{
			ID: value.ID, UserID: value.UserID, MeasuredAt: value.MeasuredAt,
			WeightKG: value.WeightKG, ChestCM: value.ChestCM, WaistCM: value.WaistCM,
			HipsCM: value.HipsCM, NeckCM: value.NeckCM, BicepsCM: value.BicepsCM,
		})
	})
	userService := user.NewService(pool, userRepository, measurementWriter)
	authRepository := auth.NewRepository(pool)
	authService, err := auth.NewService(pool, authRepository, userRepository, auth.NewTokenManager(cfg.Auth))
	if err != nil {
		return fmt.Errorf("initialize auth service: %w", err)
	}
	authHandler := auth.NewHandler(authService, cfg.Auth, logger)
	userHandler := user.NewHandler(userService, logger)
	registerAPI := func(router chi.Router) {
		authHandler.RegisterRoutes(router)
		router.Group(func(private chi.Router) {
			private.Use(authService.Middleware)
			userHandler.RegisterRoutes(private)
		})
	}

	readiness := httpserver.NewReadiness(pool, cfg.Database.PingTimeout)
	server := &http.Server{
		Addr:              cfg.HTTPAddress(),
		Handler:           httpserver.NewHandler(logger, readiness, registerAPI),
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on HTTP address: %w", err)
	}

	shutdownSignal, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	serverErrors := make(chan error, 1)
	readiness.SetReady(true)
	logger.Info(
		"HTTP server started",
		slog.String("address", listener.Addr().String()),
	)

	go func() {
		serverErrors <- server.Serve(listener)
	}()

	select {
	case err := <-serverErrors:
		readiness.SetReady(false)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-shutdownSignal.Done():
		readiness.SetReady(false)
		logger.Info("graceful shutdown started")
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownContext); err != nil {
		_ = server.Close()
		return fmt.Errorf("graceful HTTP shutdown: %w", err)
	}

	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("stop HTTP server: %w", err)
	}

	logger.Info("graceful shutdown completed")
	return nil
}

func bootstrapLogger() *slog.Logger {
	return logging.Bootstrap(os.Stderr)
}
