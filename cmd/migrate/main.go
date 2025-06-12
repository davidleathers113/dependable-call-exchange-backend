package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
)

const (
	migrationsTable = "schema_migrations"
	migrationsDir   = "migrations"
)

type Migration struct {
	ID        string
	Filename  string
	AppliedAt time.Time
}

func main() {
	var (
		action = flag.String("action", "up", "Migration action: up, down, status, create")
		name   = flag.String("name", "", "Migration name (for create action)")
		steps  = flag.Int("steps", 0, "Number of migrations to run (0 = all)")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	migrator := &Migrator{
		db:  db,
		cfg: cfg,
	}

	ctx := context.Background()

	switch *action {
	case "up":
		err = migrator.Up(ctx, *steps)
	case "down":
		err = migrator.Down(ctx, *steps)
	case "status":
		err = migrator.Status(ctx)
	case "create":
		if *name == "" {
			slog.Error("migration name is required for create action")
			os.Exit(1)
		}
		err = migrator.Create(*name)
	default:
		slog.Error("unknown action", "action", *action)
		os.Exit(1)
	}

	if err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

type Migrator struct {
	db  *sql.DB
	cfg *config.Config
}

func (m *Migrator) ensureMigrationsTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(255) PRIMARY KEY,
			filename VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, migrationsTable)

	_, err := m.db.ExecContext(ctx, query)
	return err
}

func (m *Migrator) getAppliedMigrations(ctx context.Context) (map[string]Migration, error) {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure migrations table: %w", err)
	}

	query := fmt.Sprintf("SELECT id, filename, applied_at FROM %s ORDER BY applied_at", migrationsTable)
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]Migration)
	for rows.Next() {
		var m Migration
		if err := rows.Scan(&m.ID, &m.Filename, &m.AppliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration row: %w", err)
		}
		applied[m.ID] = m
	}

	return applied, rows.Err()
}

func (m *Migrator) getPendingMigrations(ctx context.Context) ([]string, error) {
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("failed to list migration files: %w", err)
	}

	var pending []string
	for _, file := range files {
		id := extractMigrationID(filepath.Base(file))
		if _, exists := applied[id]; !exists {
			pending = append(pending, file)
		}
	}

	return pending, nil
}

func (m *Migrator) Up(ctx context.Context, steps int) error {
	pending, err := m.getPendingMigrations(ctx)
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		slog.Info("no pending migrations")
		return nil
	}

	if steps > 0 && steps < len(pending) {
		pending = pending[:steps]
	}

	for _, file := range pending {
		if err := m.applyMigration(ctx, file); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}
		slog.Info("applied migration", "file", file)
	}

	slog.Info("migrations completed", "count", len(pending))
	return nil
}

func (m *Migrator) Down(ctx context.Context, steps int) error {
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		slog.Info("no migrations to rollback")
		return nil
	}

	// Get migrations in reverse order
	var migrations []Migration
	for _, migration := range applied {
		migrations = append(migrations, migration)
	}

	// Sort by applied_at descending (most recent first)
	for i := 0; i < len(migrations)-1; i++ {
		for j := i + 1; j < len(migrations); j++ {
			if migrations[i].AppliedAt.Before(migrations[j].AppliedAt) {
				migrations[i], migrations[j] = migrations[j], migrations[i]
			}
		}
	}

	if steps > 0 && steps < len(migrations) {
		migrations = migrations[:steps]
	}

	for _, migration := range migrations {
		if err := m.rollbackMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", migration.Filename, err)
		}
		slog.Info("rolled back migration", "file", migration.Filename)
	}

	slog.Info("rollback completed", "count", len(migrations))
	return nil
}

func (m *Migrator) Status(ctx context.Context) error {
	applied, err := m.getAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	pending, err := m.getPendingMigrations(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Applied migrations: %d\n", len(applied))
	for _, migration := range applied {
		fmt.Printf("  %s - %s (applied at %s)\n",
			migration.ID, migration.Filename, migration.AppliedAt.Format(time.RFC3339))
	}

	fmt.Printf("\nPending migrations: %d\n", len(pending))
	for _, file := range pending {
		id := extractMigrationID(filepath.Base(file))
		fmt.Printf("  %s - %s\n", id, filepath.Base(file))
	}

	return nil
}

func (m *Migrator) Create(name string) error {
	timestamp := time.Now().Format("20060102150405")
	id := fmt.Sprintf("%s_%s", timestamp, name)
	filename := fmt.Sprintf("%s.sql", id)
	filepath := filepath.Join(migrationsDir, filename)

	// Ensure migrations directory exists
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	content := fmt.Sprintf(`-- Migration: %s
-- Created at: %s

-- Add your migration SQL here

`, name, time.Now().Format(time.RFC3339))

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to create migration file: %w", err)
	}

	slog.Info("created migration", "file", filepath)
	return nil
}

func (m *Migrator) applyMigration(ctx context.Context, file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration SQL
	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration as applied
	id := extractMigrationID(filepath.Base(file))
	query := fmt.Sprintf("INSERT INTO %s (id, filename) VALUES ($1, $2)", migrationsTable)
	if _, err := tx.ExecContext(ctx, query, id, filepath.Base(file)); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return tx.Commit()
}

func (m *Migrator) rollbackMigration(ctx context.Context, migration Migration) error {
	// Remove migration record
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", migrationsTable)
	if _, err := m.db.ExecContext(ctx, query, migration.ID); err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	slog.Warn("migration rolled back - manual cleanup may be required",
		"migration", migration.Filename)
	return nil
}

func extractMigrationID(filename string) string {
	// Remove .sql extension
	if len(filename) > 4 && filename[len(filename)-4:] == ".sql" {
		return filename[:len(filename)-4]
	}
	return filename
}
