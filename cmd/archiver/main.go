package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/archive"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
)

// Command-line flags
var (
	configPath = flag.String("config", "configs/config.yaml", "Path to configuration file")
	mode       = flag.String("mode", "archive", "Operation mode: archive, verify, stats, restore")
	days       = flag.Int("days", 90, "Archive events older than this many days")
	batchSize  = flag.Int("batch", 1000, "Batch size for archival")
	archiveID  = flag.String("archive-id", "", "Archive ID for verify/restore operations")
	dryRun     = flag.Bool("dry-run", false, "Perform dry run without actual archival")
)

func main() {
	flag.Parse()

	// Initialize logger
	logger := telemetry.NewLogger()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to load configuration", "error", err)
	}

	// Connect to database
	db, err := database.Connect(context.Background(), cfg.Database.URL)
	if err != nil {
		logger.Fatal(context.Background(), "Failed to connect to database", "error", err)
	}
	defer db.Close()

	// Create audit repository
	auditRepo := database.NewAuditRepository(db)

	// Create archiver
	factory := archive.NewFactory(db, auditRepo, cfg, logger)
	archiver, err := factory.CreateArchiver(context.Background())
	if err != nil {
		logger.Fatal(context.Background(), "Failed to create archiver", "error", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info(ctx, "Received shutdown signal")
		cancel()
	}()

	// Execute based on mode
	switch *mode {
	case "archive":
		err = runArchive(ctx, archiver, logger)
	case "verify":
		err = runVerify(ctx, archiver, logger)
	case "stats":
		err = runStats(ctx, archiver, logger)
	case "restore":
		err = runRestore(ctx, archiver, logger)
	default:
		err = fmt.Errorf("unknown mode: %s", *mode)
	}

	if err != nil {
		logger.Fatal(ctx, "Operation failed", "error", err)
	}

	logger.Info(ctx, "Operation completed successfully")
}

// runArchive archives old events
func runArchive(ctx context.Context, archiver archive.ArchiverRepository, logger telemetry.Logger) error {
	cutoffDate := time.Now().UTC().AddDate(0, 0, -*days)
	
	logger.Info(ctx, "Starting archive operation",
		"cutoff_date", cutoffDate.Format(time.RFC3339),
		"batch_size", *batchSize,
		"dry_run", *dryRun)

	if *dryRun {
		logger.Info(ctx, "DRY RUN: Would archive events older than", "date", cutoffDate)
		// Could add logic to count events that would be archived
		return nil
	}

	// Start archival
	startTime := time.Now()
	count, err := archiver.ArchiveEvents(ctx, cutoffDate, *batchSize)
	if err != nil {
		return fmt.Errorf("archive failed: %w", err)
	}

	duration := time.Since(startTime)
	eventsPerSecond := float64(count) / duration.Seconds()

	logger.Info(ctx, "Archive operation completed",
		"events_archived", count,
		"duration", duration,
		"events_per_second", fmt.Sprintf("%.2f", eventsPerSecond))

	return nil
}

// runVerify verifies archive integrity
func runVerify(ctx context.Context, archiver archive.ArchiverRepository, logger telemetry.Logger) error {
	if *archiveID == "" {
		return fmt.Errorf("archive-id is required for verify operation")
	}

	logger.Info(ctx, "Verifying archive integrity", "archive_id", *archiveID)

	result, err := archiver.VerifyArchiveIntegrity(ctx, *archiveID)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	logger.Info(ctx, "Verification completed",
		"archive_id", result.ArchiveID,
		"is_valid", result.IsValid,
		"event_count", result.EventCount,
		"hash_chain_valid", result.HashChainValid,
		"metadata_valid", result.MetadataValid,
		"parquet_valid", result.ParquetValid)

	if !result.IsValid {
		logger.Error(ctx, "Archive integrity check failed",
			"errors", result.Errors)
		return fmt.Errorf("archive integrity check failed")
	}

	return nil
}

// runStats displays archive statistics
func runStats(ctx context.Context, archiver archive.ArchiverRepository, logger telemetry.Logger) error {
	logger.Info(ctx, "Retrieving archive statistics")

	stats, err := archiver.GetArchiveStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	// Display statistics
	fmt.Printf("\n=== Archive Statistics ===\n")
	fmt.Printf("Total Archives: %d\n", stats.TotalArchives)
	fmt.Printf("Total Events: %d\n", stats.TotalEvents)
	fmt.Printf("Total Size: %.2f GB\n", float64(stats.TotalSize)/(1024*1024*1024))
	fmt.Printf("Average Archive Size: %.2f MB\n", float64(stats.AverageSize)/(1024*1024))
	fmt.Printf("Compression Ratio: %.2fx\n", stats.CompressionRatio)
	fmt.Printf("Oldest Archive: %s\n", stats.OldestArchive.Format(time.RFC3339))
	fmt.Printf("Newest Archive: %s\n", stats.NewestArchive.Format(time.RFC3339))

	fmt.Printf("\nArchives by Year:\n")
	for year, count := range stats.ArchivesByYear {
		fmt.Printf("  %d: %d archives\n", year, count)
	}

	fmt.Printf("\nEvents by Compliance Type:\n")
	for flag, count := range stats.EventsByCompliance {
		fmt.Printf("  %s: %d events\n", flag, count)
	}

	return nil
}

// runRestore restores an archive
func runRestore(ctx context.Context, archiver archive.ArchiverRepository, logger telemetry.Logger) error {
	if *archiveID == "" {
		return fmt.Errorf("archive-id is required for restore operation")
	}

	logger.Info(ctx, "Restoring archive", "archive_id", *archiveID)

	if *dryRun {
		logger.Info(ctx, "DRY RUN: Would restore archive", "archive_id", *archiveID)
		return nil
	}

	result, err := archiver.RestoreArchive(ctx, *archiveID)
	if err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	logger.Info(ctx, "Restore completed",
		"archive_id", result.ArchiveID,
		"events_restored", result.EventsRestored,
		"verification_status", result.VerificationStatus,
		"restore_time", result.RestoreTime)

	if len(result.Errors) > 0 {
		logger.Warn(ctx, "Restore completed with errors",
			"errors", result.Errors)
	}

	return nil
}