package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/api/rest"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
)

func main() {
	// Parse flags
	var (
		configPath = flag.String("config", "", "Path to configuration file")
		migrate    = flag.Bool("migrate", false, "Run database migrations")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize telemetry
	ctx := context.Background()
	telConfig := &telemetry.Config{
		ServiceName:    "dce-api",
		ServiceVersion: cfg.Version,
		Environment:    cfg.Environment,
		OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
		Enabled:        cfg.Telemetry.Enabled,
		SamplingRate:   cfg.Telemetry.SamplingRate,
		ExportTimeout:  cfg.Telemetry.ExportTimeout,
		BatchTimeout:   cfg.Telemetry.BatchTimeout,
	}
	
	provider, err := telemetry.InitializeOpenTelemetry(ctx, telConfig)
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer func() {
		if err := provider.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown telemetry: %v", err)
		}
	}()

	// Run migrations if requested
	if *migrate {
		if err := runMigrations(cfg); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
		log.Println("Migrations completed successfully")
		return
	}

	// Create and start server
	server, err := rest.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func runMigrations(cfg *config.Config) error {
	// TODO: Implement migration logic
	// This should use the migrate package to run database migrations
	fmt.Println("Running database migrations...")
	return nil
}