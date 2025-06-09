package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger, err := telemetry.SetupLogger(cfg.LogLevel)
	if err != nil {
		slog.Error("failed to setup logger", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(logger)

	if err := run(ctx, cfg); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config) error {
	slog.Info("starting dependable call exchange backend",
		"version", cfg.Version,
		"environment", cfg.Environment,
		"port", cfg.Server.Port)

	// TODO: Initialize services, APIs, and start server
	
	<-ctx.Done()
	slog.Info("shutting down gracefully")
	
	return nil
}