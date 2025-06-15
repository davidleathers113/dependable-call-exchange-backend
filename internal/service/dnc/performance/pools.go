package performance

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// ConnectionPool manages database connections for optimal performance
type ConnectionPool struct {
	logger *zap.Logger
	config *ConnectionPoolConfig
	
	// Connection management
	connections chan *PooledConnection
	active      map[*PooledConnection]bool
	activeMutex sync.RWMutex
	
	// Statistics
	stats       *ConnectionPoolStats
	statsMutex  sync.RWMutex
	
	// State management
	running int32
	closed  chan struct{}
	wg      sync.WaitGroup
}

// ConnectionPoolConfig configures the connection pool
type ConnectionPoolConfig struct {
	MaxConnections     int
	MinIdleConnections int
	ConnectionTimeout  time.Duration
	ConnectionMaxAge   time.Duration
	PrewarmEnabled     bool
	HealthCheckInterval time.Duration
	DSN                string
}

// PooledConnection wraps a database connection with metadata
type PooledConnection struct {
	*sql.Conn
	created   time.Time
	lastUsed  time.Time
	useCount  int64
	healthy   bool
	pool      *ConnectionPool
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *ConnectionPoolConfig, logger *zap.Logger) *ConnectionPool {
	pool := &ConnectionPool{
		logger:      logger,
		config:      config,
		connections: make(chan *PooledConnection, config.MaxConnections),
		active:      make(map[*PooledConnection]bool),
		stats: &ConnectionPoolStats{
			MaxConnections: config.MaxConnections,
		},
		closed: make(chan struct{}),
	}
	
	return pool
}

// Start initializes the connection pool
func (cp *ConnectionPool) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&cp.running, 0, 1) {
		return fmt.Errorf("connection pool already running")
	}
	
	cp.logger.Info("Starting connection pool",
		zap.Int("max_connections", cp.config.MaxConnections),
		zap.Int("min_idle_connections", cp.config.MinIdleConnections),
		zap.Bool("prewarm_enabled", cp.config.PrewarmEnabled),
	)
	
	// Prewarm connections if enabled
	if cp.config.PrewarmEnabled {
		if err := cp.prewarmConnections(ctx); err != nil {
			return fmt.Errorf("failed to prewarm connections: %w", err)
		}
	}
	
	// Start health checker
	cp.wg.Add(1)
	go cp.runHealthChecker(ctx)
	
	// Start connection reaper
	cp.wg.Add(1)
	go cp.runConnectionReaper(ctx)
	
	cp.logger.Info("Connection pool started successfully")
	return nil
}

// Stop gracefully shuts down the connection pool
func (cp *ConnectionPool) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&cp.running, 1, 0) {
		return nil
	}
	
	cp.logger.Info("Stopping connection pool")
	
	close(cp.closed)
	
	// Wait for background routines
	done := make(chan struct{})
	go func() {
		cp.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
	case <-ctx.Done():
		cp.logger.Warn("Connection pool stop timed out")
	}
	
	// Close all connections
	cp.closeAllConnections()
	
	cp.logger.Info("Connection pool stopped")
	return nil
}

// GetConnection retrieves a connection from the pool
func (cp *ConnectionPool) GetConnection(ctx context.Context) (*PooledConnection, error) {
	if atomic.LoadInt32(&cp.running) == 0 {
		return nil, fmt.Errorf("connection pool not running")
	}
	
	start := time.Now()
	defer func() {
		cp.updateWaitTime(time.Since(start))
	}()
	
	// Try to get an existing connection
	select {
	case conn := <-cp.connections:
		if cp.isConnectionHealthy(conn) {
			cp.markConnectionActive(conn)
			atomic.AddInt64(&cp.stats.TotalCreated, 0) // No increment for reused
			return conn, nil
		} else {
			// Connection is unhealthy, close it and try again
			cp.closeConnection(conn)
			return cp.GetConnection(ctx)
		}
	case <-ctx.Done():
		atomic.AddInt64(&cp.stats.TotalErrors, 1)
		return nil, ctx.Err()
	default:
		// No connections available, try to create new one
		return cp.createNewConnection(ctx)
	}
}

// ReturnConnection returns a connection to the pool
func (cp *ConnectionPool) ReturnConnection(conn *PooledConnection) {
	if conn == nil {
		return
	}
	
	conn.lastUsed = time.Now()
	atomic.AddInt64(&conn.useCount, 1)
	
	cp.markConnectionIdle(conn)
	
	// Check if connection should be kept
	if cp.shouldKeepConnection(conn) {
		select {
		case cp.connections <- conn:
			// Successfully returned to pool
		default:
			// Pool is full, close the connection
			cp.closeConnection(conn)
		}
	} else {
		cp.closeConnection(conn)
	}
}

// GetStats returns current pool statistics
func (cp *ConnectionPool) GetStats() *ConnectionPoolStats {
	cp.statsMutex.RLock()
	defer cp.statsMutex.RUnlock()
	
	stats := *cp.stats
	stats.Active = len(cp.active)
	stats.Idle = len(cp.connections)
	stats.Total = stats.Active + stats.Idle
	
	return &stats
}

// prewarmConnections creates initial connections
func (cp *ConnectionPool) prewarmConnections(ctx context.Context) error {
	for i := 0; i < cp.config.MinIdleConnections; i++ {
		conn, err := cp.createNewConnection(ctx)
		if err != nil {
			cp.logger.Warn("Failed to prewarm connection",
				zap.Int("connection_number", i),
				zap.Error(err),
			)
			continue
		}
		cp.ReturnConnection(conn)
	}
	
	cp.logger.Info("Prewarmed connections",
		zap.Int("count", cp.config.MinIdleConnections),
	)
	
	return nil
}

// createNewConnection creates a new database connection
func (cp *ConnectionPool) createNewConnection(ctx context.Context) (*PooledConnection, error) {
	// Check if we can create more connections
	currentTotal := cp.getCurrentTotal()
	if currentTotal >= cp.config.MaxConnections {
		atomic.AddInt64(&cp.stats.TotalErrors, 1)
		return nil, fmt.Errorf("connection pool exhausted (max: %d)", cp.config.MaxConnections)
	}
	
	// Create database connection
	db, err := sql.Open("postgres", cp.config.DSN)
	if err != nil {
		atomic.AddInt64(&cp.stats.TotalErrors, 1)
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	connCtx, cancel := context.WithTimeout(ctx, cp.config.ConnectionTimeout)
	defer cancel()
	
	conn, err := db.Conn(connCtx)
	if err != nil {
		db.Close()
		atomic.AddInt64(&cp.stats.TotalErrors, 1)
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	
	pooledConn := &PooledConnection{
		Conn:     conn,
		created:  time.Now(),
		lastUsed: time.Now(),
		healthy:  true,
		pool:     cp,
	}
	
	atomic.AddInt64(&cp.stats.TotalCreated, 1)
	
	cp.logger.Debug("Created new connection",
		zap.Int("total_active", currentTotal+1),
	)
	
	return pooledConn, nil
}

// isConnectionHealthy checks if a connection is healthy
func (cp *ConnectionPool) isConnectionHealthy(conn *PooledConnection) bool {
	if !conn.healthy {
		return false
	}
	
	// Check age
	if time.Since(conn.created) > cp.config.ConnectionMaxAge {
		return false
	}
	
	// Ping the connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	
	if err := conn.PingContext(ctx); err != nil {
		conn.healthy = false
		return false
	}
	
	return true
}

// shouldKeepConnection determines if a connection should be kept in the pool
func (cp *ConnectionPool) shouldKeepConnection(conn *PooledConnection) bool {
	// Check if connection is too old
	if time.Since(conn.created) > cp.config.ConnectionMaxAge {
		return false
	}
	
	// Check if connection is unhealthy
	if !conn.healthy {
		return false
	}
	
	return true
}

// markConnectionActive marks a connection as active
func (cp *ConnectionPool) markConnectionActive(conn *PooledConnection) {
	cp.activeMutex.Lock()
	cp.active[conn] = true
	cp.activeMutex.Unlock()
}

// markConnectionIdle marks a connection as idle
func (cp *ConnectionPool) markConnectionIdle(conn *PooledConnection) {
	cp.activeMutex.Lock()
	delete(cp.active, conn)
	cp.activeMutex.Unlock()
}

// closeConnection closes a single connection
func (cp *ConnectionPool) closeConnection(conn *PooledConnection) {
	if conn != nil && conn.Conn != nil {
		conn.Close()
		atomic.AddInt64(&cp.stats.TotalClosed, 1)
	}
	
	cp.markConnectionIdle(conn)
}

// closeAllConnections closes all connections
func (cp *ConnectionPool) closeAllConnections() {
	// Close idle connections
	for {
		select {
		case conn := <-cp.connections:
			cp.closeConnection(conn)
		default:
			goto closeActive
		}
	}
	
closeActive:
	// Close active connections
	cp.activeMutex.Lock()
	for conn := range cp.active {
		cp.closeConnection(conn)
	}
	cp.activeMutex.Unlock()
}

// getCurrentTotal returns the current total number of connections
func (cp *ConnectionPool) getCurrentTotal() int {
	cp.activeMutex.RLock()
	active := len(cp.active)
	cp.activeMutex.RUnlock()
	
	idle := len(cp.connections)
	
	return active + idle
}

// updateWaitTime updates the average wait time statistics
func (cp *ConnectionPool) updateWaitTime(waitTime time.Duration) {
	cp.statsMutex.Lock()
	defer cp.statsMutex.Unlock()
	
	// Simple moving average
	if cp.stats.AverageWaitTime == 0 {
		cp.stats.AverageWaitTime = waitTime
	} else {
		cp.stats.AverageWaitTime = (cp.stats.AverageWaitTime + waitTime) / 2
	}
}

// runHealthChecker periodically checks connection health
func (cp *ConnectionPool) runHealthChecker(ctx context.Context) {
	defer cp.wg.Done()
	
	ticker := time.NewTicker(cp.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cp.closed:
			return
		case <-ticker.C:
			cp.performHealthCheck()
		}
	}
}

// runConnectionReaper periodically removes old connections
func (cp *ConnectionPool) runConnectionReaper(ctx context.Context) {
	defer cp.wg.Done()
	
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cp.closed:
			return
		case <-ticker.C:
			cp.reapOldConnections()
		}
	}
}

// performHealthCheck checks all connections in the pool
func (cp *ConnectionPool) performHealthCheck() {
	// Check idle connections
	idleConnections := make([]*PooledConnection, 0, len(cp.connections))
	
	// Drain the channel to check all connections
	for {
		select {
		case conn := <-cp.connections:
			idleConnections = append(idleConnections, conn)
		default:
			goto checkConnections
		}
	}
	
checkConnections:
	// Check each connection and return healthy ones
	for _, conn := range idleConnections {
		if cp.isConnectionHealthy(conn) {
			cp.connections <- conn
		} else {
			cp.closeConnection(conn)
		}
	}
}

// reapOldConnections removes connections that are too old
func (cp *ConnectionPool) reapOldConnections() {
	minIdle := cp.config.MinIdleConnections
	currentIdle := len(cp.connections)
	
	if currentIdle <= minIdle {
		return // Keep minimum idle connections
	}
	
	// Remove excess old connections
	toRemove := currentIdle - minIdle
	removed := 0
	
	for removed < toRemove {
		select {
		case conn := <-cp.connections:
			if time.Since(conn.lastUsed) > time.Hour || time.Since(conn.created) > cp.config.ConnectionMaxAge {
				cp.closeConnection(conn)
				removed++
			} else {
				// Connection is still good, put it back
				cp.connections <- conn
				break
			}
		default:
			break
		}
	}
	
	if removed > 0 {
		cp.logger.Debug("Reaped old connections",
			zap.Int("removed", removed),
			zap.Int("remaining_idle", len(cp.connections)),
		)
	}
}