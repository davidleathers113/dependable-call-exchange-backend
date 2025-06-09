package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer wraps testcontainers postgres with our specific needs
type PostgresContainer struct {
	*postgres.PostgresContainer
	ConnectionString string
}

// NewPostgresContainer creates a new PostgreSQL test container
func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("dce_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}
	
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}
	
	return &PostgresContainer{
		PostgresContainer: pgContainer,
		ConnectionString:  connStr,
	}, nil
}

// WithSnapshot enables database snapshots for fast restoration
func (p *PostgresContainer) WithSnapshot(name string) error {
	// Note: Full snapshot support requires additional setup
	// For now, we'll use schema recreation as our "snapshot"
	return nil
}

// RestoreSnapshot quickly restores database to clean state
func (p *PostgresContainer) RestoreSnapshot(ctx context.Context) error {
	// For now, just return nil - truncation will be handled by TestDB
	// Real snapshot support would require docker commit/restore
	return nil
}