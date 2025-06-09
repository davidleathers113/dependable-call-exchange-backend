package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
)

// ConnectionPool represents an advanced database connection pool with
// circuit breaker, health checks, and automatic failover capabilities
type ConnectionPool struct {
	primary         *pgxpool.Pool
	replicas        []*pgxpool.Pool
	config          *config.DatabaseConfig
	logger          *zap.Logger
	mu              sync.RWMutex
	healthCheckStop chan struct{}
	metrics         *ConnectionMetrics
	circuitBreaker  *CircuitBreaker
}

// ConnectionMetrics tracks database performance metrics
type ConnectionMetrics struct {
	mu sync.RWMutex

	// Connection metrics
	TotalConnections     int64
	ActiveConnections    int64
	IdleConnections      int64
	WaitingConnections   int64
	MaxLifetimeClosures  int64
	IdleClosures         int64

	// Query metrics
	QueriesExecuted      int64
	QueriesFailed        int64
	TotalQueryTime       time.Duration
	SlowestQuery         time.Duration
	SlowestQuerySQL      string

	// Transaction metrics
	TransactionsStarted  int64
	TransactionsCommitted int64
	TransactionsRolledBack int64

	// Replication lag
	ReplicationLag       time.Duration
	LastHealthCheck      time.Time
}

// CircuitBreaker implements circuit breaker pattern for database connections
type CircuitBreaker struct {
	mu              sync.Mutex
	failureCount    int
	lastFailureTime time.Time
	state           CircuitState
	timeout         time.Duration
	threshold       int
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// ExtendedDatabaseConfig extends the base config with pgx-specific settings
type ExtendedDatabaseConfig struct {
	*config.DatabaseConfig
	PrimaryURL      string   // Primary database URL
	ReplicaURLs     []string // Replica database URLs
	MaxConnections  int      // Max connections per pool
	MinConnections  int      // Min connections per pool
	MaxConnLifetime time.Duration
}

// NewConnectionPool creates a new advanced connection pool
func NewConnectionPool(cfg *config.DatabaseConfig, logger *zap.Logger) (*ConnectionPool, error) {
	// Create extended config with defaults
	extCfg := &ExtendedDatabaseConfig{
		DatabaseConfig:  cfg,
		PrimaryURL:      cfg.URL,
		ReplicaURLs:     []string{}, // No replicas by default
		MaxConnections:  cfg.MaxOpenConns,
		MinConnections:  cfg.MaxIdleConns,
		MaxConnLifetime: cfg.ConnMaxLifetime,
	}

	pool := &ConnectionPool{
		config:          extCfg.DatabaseConfig,
		logger:          logger,
		healthCheckStop: make(chan struct{}),
		metrics:         &ConnectionMetrics{},
		circuitBreaker: &CircuitBreaker{
			timeout:   30 * time.Second,
			threshold: 10, // More reasonable threshold
			state:     CircuitClosed,
		},
	}

	// Configure primary connection
	primaryConfig, err := pgxpool.ParseConfig(extCfg.PrimaryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse primary database URL: %w", err)
	}

	// Apply advanced configuration
	pool.configurePgxPool(primaryConfig, extCfg)

	// Create primary pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool.primary, err = pgxpool.NewWithConfig(ctx, primaryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create primary connection pool: %w", err)
	}

	// Test primary connection
	if err := pool.primary.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping primary database: %w", err)
	}

	// Configure replicas
	pool.replicas = make([]*pgxpool.Pool, 0, len(extCfg.ReplicaURLs))
	for i, replicaURL := range extCfg.ReplicaURLs {
		replicaConfig, err := pgxpool.ParseConfig(replicaURL)
		if err != nil {
			logger.Warn("failed to parse replica URL", 
				zap.Int("replica", i),
				zap.Error(err))
			continue
		}

		pool.configurePgxPool(replicaConfig, extCfg)

		replica, err := pgxpool.NewWithConfig(ctx, replicaConfig)
		if err != nil {
			logger.Warn("failed to create replica connection pool",
				zap.Int("replica", i),
				zap.Error(err))
			continue
		}

		if err := replica.Ping(ctx); err != nil {
			logger.Warn("failed to ping replica database",
				zap.Int("replica", i),
				zap.Error(err))
			replica.Close()
			continue
		}

		pool.replicas = append(pool.replicas, replica)
	}

	// Start health check routine
	go pool.healthCheckRoutine()

	// Start metrics collection
	go pool.metricsCollectionRoutine()

	logger.Info("database connection pool initialized",
		zap.Int("replicas", len(pool.replicas)),
		zap.Int("max_connections", int(primaryConfig.MaxConns)))

	return pool, nil
}

// NewConnectionPoolWithExtended creates a new connection pool with extended config
func NewConnectionPoolWithExtended(cfg *ExtendedDatabaseConfig, logger *zap.Logger) (*ConnectionPool, error) {
	pool := &ConnectionPool{
		config:          cfg.DatabaseConfig,
		logger:          logger,
		healthCheckStop: make(chan struct{}),
		metrics:         &ConnectionMetrics{},
		circuitBreaker: &CircuitBreaker{
			timeout:   30 * time.Second,
			threshold: 10,
			state:     CircuitClosed,
		},
	}

	// Configure primary connection
	primaryConfig, err := pgxpool.ParseConfig(cfg.PrimaryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse primary database URL: %w", err)
	}

	// Apply advanced configuration
	pool.configurePgxPool(primaryConfig, cfg)

	// Create primary pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool.primary, err = pgxpool.NewWithConfig(ctx, primaryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create primary connection pool: %w", err)
	}

	// Test primary connection
	if err := pool.primary.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping primary database: %w", err)
	}

	// Configure replicas
	pool.replicas = make([]*pgxpool.Pool, 0, len(cfg.ReplicaURLs))
	for i, replicaURL := range cfg.ReplicaURLs {
		replicaConfig, err := pgxpool.ParseConfig(replicaURL)
		if err != nil {
			logger.Warn("failed to parse replica URL",
				zap.Int("replica", i),
				zap.Error(err))
			continue
		}

		pool.configurePgxPool(replicaConfig, cfg)

		replica, err := pgxpool.NewWithConfig(ctx, replicaConfig)
		if err != nil {
			logger.Warn("failed to create replica connection pool",
				zap.Int("replica", i),
				zap.Error(err))
			continue
		}

		if err := replica.Ping(ctx); err != nil {
			logger.Warn("failed to ping replica database",
				zap.Int("replica", i),
				zap.Error(err))
			replica.Close()
			continue
		}

		pool.replicas = append(pool.replicas, replica)
	}

	// Start health check routine
	go pool.healthCheckRoutine()

	// Start metrics collection
	go pool.metricsCollectionRoutine()

	logger.Info("database connection pool initialized",
		zap.Int("replicas", len(pool.replicas)),
		zap.Int("max_connections", int(primaryConfig.MaxConns)))

	return pool, nil
}

// configurePgxPool applies advanced configuration to pgx connection pool
func (p *ConnectionPool) configurePgxPool(config *pgxpool.Config, extCfg *ExtendedDatabaseConfig) {
	// Connection pool settings - use config values or sensible defaults
	if extCfg.MaxConnections > 0 {
		config.MaxConns = int32(extCfg.MaxConnections)
	} else {
		config.MaxConns = 25 // Conservative default
	}
	if extCfg.MinConnections > 0 {
		config.MinConns = int32(extCfg.MinConnections)
	} else {
		config.MinConns = 5
	}
	if extCfg.MaxConnLifetime > 0 {
		config.MaxConnLifetime = extCfg.MaxConnLifetime
	} else {
		config.MaxConnLifetime = 30 * time.Minute
	}
	config.MaxConnIdleTime = 10 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	// Connection configuration
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	config.ConnConfig.TLSConfig = nil // Configure TLS if needed

	// Runtime parameters
	config.ConnConfig.RuntimeParams = map[string]string{
		"application_name":              "dce_backend",
		"search_path":                   "core,billing,analytics,public",
		"timezone":                      "UTC",
		"lock_timeout":                  "10s",
		"statement_timeout":             "30s",
		"idle_in_transaction_session_timeout": "60s",
		"default_transaction_isolation": "read committed",
		"synchronous_commit":            "on",
		"jit":                          "on",
	}

	// Before connect callback for connection initialization
	config.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
		p.logger.Debug("establishing database connection",
			zap.String("host", cc.Host),
			zap.Uint16("port", cc.Port))
		return nil
	}

	// After connect callback for connection setup
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// Register custom types
		if err := registerCustomTypes(ctx, conn); err != nil {
			return fmt.Errorf("failed to register custom types: %w", err)
		}

		// Prepare frequently used statements
		if err := prepareStatements(ctx, conn); err != nil {
			return fmt.Errorf("failed to prepare statements: %w", err)
		}

		p.metrics.mu.Lock()
		p.metrics.TotalConnections++
		p.metrics.mu.Unlock()

		return nil
	}

	// Before acquire callback
	config.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
		// Check circuit breaker
		if !p.circuitBreaker.Allow() {
			return false
		}

		// Quick health check
		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		return conn.Ping(ctx) == nil
	}

	// After release callback
	config.AfterRelease = func(conn *pgx.Conn) bool {
		// Reset connection state
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		_, err := conn.Exec(ctx, "DISCARD ALL")
		return err == nil
	}
}

// GetPrimary returns a connection to the primary database
func (p *ConnectionPool) GetPrimary() *pgxpool.Pool {
	return p.primary
}

// GetReplica returns a connection to a read replica using round-robin
func (p *ConnectionPool) GetReplica() *pgxpool.Pool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.replicas) == 0 {
		return p.primary // Fallback to primary if no replicas
	}

	// Simple round-robin selection
	// In production, use more sophisticated load balancing
	now := time.Now().UnixNano()
	index := int(now % int64(len(p.replicas)))
	
	return p.replicas[index]
}

// GetReadConnection returns appropriate connection for read operations
func (p *ConnectionPool) GetReadConnection(preferPrimary bool) *pgxpool.Pool {
	if preferPrimary || len(p.replicas) == 0 {
		return p.primary
	}

	// Check replication lag
	p.metrics.mu.RLock()
	lag := p.metrics.ReplicationLag
	p.metrics.mu.RUnlock()

	// Use primary if replication lag is too high
	if lag > 5*time.Second {
		p.logger.Warn("high replication lag detected, using primary",
			zap.Duration("lag", lag))
		return p.primary
	}

	return p.GetReplica()
}

// Transaction executes a function within a database transaction
func (p *ConnectionPool) Transaction(ctx context.Context, fn func(pgx.Tx) error) error {
	return p.TransactionWithOptions(ctx, pgx.TxOptions{}, fn)
}

// TransactionWithOptions executes a function within a database transaction with options
func (p *ConnectionPool) TransactionWithOptions(ctx context.Context, opts pgx.TxOptions, fn func(pgx.Tx) error) error {
	p.metrics.mu.Lock()
	p.metrics.TransactionsStarted++
	p.metrics.mu.Unlock()

	err := pgx.BeginTxFunc(ctx, p.primary, opts, fn)

	p.metrics.mu.Lock()
	if err != nil {
		p.metrics.TransactionsRolledBack++
		p.circuitBreaker.RecordFailure()
	} else {
		p.metrics.TransactionsCommitted++
		p.circuitBreaker.RecordSuccess()
	}
	p.metrics.mu.Unlock()

	return err
}

// healthCheckRoutine performs periodic health checks
func (p *ConnectionPool) healthCheckRoutine() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performHealthCheck()
		case <-p.healthCheckStop:
			return
		}
	}
}

// performHealthCheck checks the health of all connections
func (p *ConnectionPool) performHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check primary
	if err := p.primary.Ping(ctx); err != nil {
		p.logger.Error("primary database health check failed", zap.Error(err))
		p.circuitBreaker.RecordFailure()
	}

	// Check replicas
	p.mu.Lock()
	healthyReplicas := make([]*pgxpool.Pool, 0, len(p.replicas))
	for i, replica := range p.replicas {
		if err := replica.Ping(ctx); err != nil {
			p.logger.Warn("replica health check failed",
				zap.Int("replica", i),
				zap.Error(err))
			continue
		}

		// Check replication lag
		var lag time.Duration
		row := replica.QueryRow(ctx, `
			SELECT EXTRACT(EPOCH FROM (NOW() - pg_last_xact_replay_timestamp()))::INTEGER
		`)
		
		var lagSeconds sql.NullInt64
		if err := row.Scan(&lagSeconds); err == nil && lagSeconds.Valid {
			lag = time.Duration(lagSeconds.Int64) * time.Second
		}

		if lag > 30*time.Second {
			p.logger.Warn("high replication lag on replica",
				zap.Int("replica", i),
				zap.Duration("lag", lag))
		}

		healthyReplicas = append(healthyReplicas, replica)
	}
	p.replicas = healthyReplicas
	p.mu.Unlock()

	p.metrics.mu.Lock()
	p.metrics.LastHealthCheck = time.Now()
	p.metrics.mu.Unlock()
}

// metricsCollectionRoutine collects performance metrics
func (p *ConnectionPool) metricsCollectionRoutine() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.collectMetrics()
		case <-p.healthCheckStop:
			return
		}
	}
}

// collectMetrics gathers current performance metrics
func (p *ConnectionPool) collectMetrics() {
	stats := p.primary.Stat()

	p.metrics.mu.Lock()
	p.metrics.ActiveConnections = int64(stats.AcquiredConns())
	p.metrics.IdleConnections = int64(stats.IdleConns())
	p.metrics.MaxLifetimeClosures = stats.MaxLifetimeDestroyCount()
	p.metrics.mu.Unlock()
}

// Close closes all database connections
func (p *ConnectionPool) Close() error {
	close(p.healthCheckStop)

	// Close replicas
	for _, replica := range p.replicas {
		replica.Close()
	}

	// Close primary
	p.primary.Close()

	p.logger.Info("database connection pool closed")
	return nil
}

// CircuitBreaker methods
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0
	cb.state = CircuitClosed
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= cb.threshold {
		cb.state = CircuitOpen
	}
}

// registerCustomTypes registers PostgreSQL custom types
func registerCustomTypes(ctx context.Context, conn *pgx.Conn) error {
	// Register custom types here
	// Example: conn.TypeMap().RegisterType(&pgtype.Type{...})
	return nil
}

// prepareStatements prepares frequently used SQL statements
func prepareStatements(ctx context.Context, conn *pgx.Conn) error {
	// Check if the core schema exists before preparing statements
	var schemaExists bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.schemata 
			WHERE schema_name = 'core'
		)
	`).Scan(&schemaExists)
	
	if err != nil || !schemaExists {
		// Skip statement preparation if schema doesn't exist (e.g., during tests)
		return nil
	}

	statements := map[string]string{
		"get_account_by_id": `
			SELECT id, email, name, type, status, balance, created_at, updated_at
			FROM core.accounts
			WHERE id = $1 AND deleted_at IS NULL
		`,
		"update_account_balance": `
			UPDATE core.accounts 
			SET balance = balance + $2, updated_at = NOW()
			WHERE id = $1
			RETURNING balance
		`,
		"get_active_calls": `
			SELECT id, call_sid, from_number, to_number, status, start_time
			FROM core.calls
			WHERE status IN ('queued', 'ringing', 'in_progress')
			ORDER BY start_time DESC
		`,
	}

	for name, sql := range statements {
		if _, err := conn.Prepare(ctx, name, sql); err != nil {
			return fmt.Errorf("failed to prepare statement %s: %w", name, err)
		}
	}

	return nil
}

// GetDB returns a standard database/sql DB for compatibility
func (p *ConnectionPool) GetDB() (*sql.DB, error) {
	return stdlib.OpenDBFromPool(p.primary), nil
}