package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/api/rest"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/repository"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
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

	// Initialize OpenTelemetry
	otelProvider, err := telemetry.InitializeOpenTelemetry(ctx, &telemetry.Config{
		ServiceName:    "dce-backend",
		ServiceVersion: cfg.Version,
		Environment:    cfg.Environment,
		Enabled:        cfg.Telemetry.Enabled,
		OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
		SamplingRate:   cfg.Telemetry.SamplingRate,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown OpenTelemetry", "error", err)
		}
	}()

	// Initialize database connection
	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Initialize repositories
	repositories := initializeRepositories(db)

	// Initialize services
	services := initializeServices(repositories, cfg)

	// Initialize HTTP server
	server := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      initializeAPIHandlers(services),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("starting HTTP server", "port", cfg.Server.Port)
		serverErrors <- server.ListenAndServe()
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("failed to shutdown server gracefully", "error", err)
		return err
	}

	slog.Info("server shut down successfully")
	return nil
}

// initializeRepositories creates and configures all repository instances
func initializeRepositories(db *pgxpool.Pool) *repository.Repositories {
	return repository.NewRepositories(db)
}

// initializeServices creates and configures all service instances
func initializeServices(repos *repository.Repositories, cfg *config.Config) *rest.Services {
	// Create service factories with proper dependency injection
	factories := service.NewServiceFactories(repos)

	// Initialize all core services using factory methods
	return &rest.Services{
		CallRouting:  factories.CreateCallRoutingService(),
		Bidding:      factories.CreateBiddingService(),
		Telephony:    factories.CreateTelephonyService(),
		Fraud:        factories.CreateFraudService(),
		Repositories: repos,
	}
}

// initializeAPIHandlers sets up the HTTP request routing
func initializeAPIHandlers(services *rest.Services) http.Handler {
	// Create REST API handler with comprehensive routing and middleware
	return rest.NewHandler(services)
}
