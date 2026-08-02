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

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/logging"
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

	readiness := httpserver.NewReadiness()
	server := &http.Server{
		Addr:              cfg.HTTPAddress(),
		Handler:           httpserver.NewHandler(logger, readiness),
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
