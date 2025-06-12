package rest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HealthChecker checks the health of a dependency
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthCheckResult
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Status      HealthStatus           `json:"status"`
	Message     string                 `json:"message,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ResponseTime time.Duration         `json:"response_time"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	LastChecked time.Time             `json:"last_checked"`
}

// HealthStatus represents the health status
type HealthStatus string

const (
	HealthStatusPass HealthStatus = "pass"
	HealthStatusWarn HealthStatus = "warn"
	HealthStatusFail HealthStatus = "fail"
)

// HealthService manages health checks
type HealthService struct {
	checkers  map[string]HealthChecker
	cache     sync.Map
	config    HealthConfig
	tracer    trace.Tracer
	startTime time.Time
}

// HealthConfig configures the health service
type HealthConfig struct {
	// CacheDuration is how long to cache health check results
	CacheDuration time.Duration

	// Timeout is the maximum time for a health check
	Timeout time.Duration

	// IncludeDetails includes detailed information
	IncludeDetails bool

	// ServiceName is the name of the service
	ServiceName string

	// ServiceVersion is the version of the service
	ServiceVersion string

	// Environment is the deployment environment
	Environment string
}

// DefaultHealthConfig returns default configuration
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		CacheDuration:  10 * time.Second,
		Timeout:        5 * time.Second,
		IncludeDetails: true,
		ServiceName:    "api",
		ServiceVersion: "1.0.0",
		Environment:    "production",
	}
}

// NewHealthService creates a new health service
func NewHealthService(config HealthConfig) *HealthService {
	return &HealthService{
		checkers:  make(map[string]HealthChecker),
		config:    config,
		tracer:    otel.Tracer("api.rest.health"),
		startTime: time.Now(),
	}
}

// RegisterChecker registers a health checker
func (h *HealthService) RegisterChecker(name string, checker HealthChecker) {
	h.checkers[name] = checker
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status      HealthStatus                    `json:"status"`
	Version     string                          `json:"version"`
	ServiceID   string                          `json:"service_id"`
	Description string                          `json:"description,omitempty"`
	Checks      map[string]HealthCheckResult    `json:"checks,omitempty"`
	Output      string                          `json:"output,omitempty"`
	ServiceName string                          `json:"service_name"`
	ReleaseID   string                          `json:"release_id,omitempty"`
	Notes       []string                        `json:"notes,omitempty"`
	Links       map[string]string               `json:"links,omitempty"`
	Metadata    map[string]interface{}          `json:"metadata,omitempty"`
}

// LivenessHandler returns a simple liveness check handler
func (h *HealthService) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, span := h.tracer.Start(r.Context(), "health.liveness")
		defer span.End()

		// Simple liveness check - service is running
		response := HealthResponse{
			Status:      HealthStatusPass,
			Version:     h.config.ServiceVersion,
			ServiceID:   uuid.New().String(),
			ServiceName: h.config.ServiceName,
			Metadata: map[string]interface{}{
				"uptime_seconds": time.Since(h.startTime).Seconds(),
				"timestamp":      time.Now().UTC(),
			},
		}

		w.Header().Set("Content-Type", "application/health+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

		span.SetAttributes(
			attribute.String("health.status", string(response.Status)),
			attribute.Float64("health.uptime", time.Since(h.startTime).Seconds()),
		)
	}
}

// ReadinessHandler returns a readiness check handler
func (h *HealthService) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := h.tracer.Start(r.Context(), "health.readiness")
		defer span.End()

		// Run all health checks
		checks := h.runChecks(ctx)
		
		// Determine overall status
		status := HealthStatusPass
		statusCode := http.StatusOK
		
		for _, result := range checks {
			if result.Status == HealthStatusFail {
				status = HealthStatusFail
				statusCode = http.StatusServiceUnavailable
				break
			} else if result.Status == HealthStatusWarn && status == HealthStatusPass {
				status = HealthStatusWarn
			}
		}

		response := HealthResponse{
			Status:      status,
			Version:     h.config.ServiceVersion,
			ServiceID:   uuid.New().String(),
			ServiceName: h.config.ServiceName,
			Description: fmt.Sprintf("%s health check", h.config.ServiceName),
			Checks:      checks,
			Links: map[string]string{
				"about": "/health",
				"docs":  "/docs/health",
			},
			Metadata: map[string]interface{}{
				"uptime_seconds": time.Since(h.startTime).Seconds(),
				"timestamp":      time.Now().UTC(),
				"environment":    h.config.Environment,
			},
		}

		if h.config.IncludeDetails {
			response.Notes = []string{
				"All critical dependencies must be healthy for readiness",
				"Warnings indicate degraded but functional state",
			}
		}

		w.Header().Set("Content-Type", "application/health+json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)

		span.SetAttributes(
			attribute.String("health.status", string(response.Status)),
			attribute.Int("health.checks_count", len(checks)),
			attribute.Int("http.status_code", statusCode),
		)
	}
}

// StartupHandler returns a startup check handler
func (h *HealthService) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, span := h.tracer.Start(r.Context(), "health.startup")
		defer span.End()

		// Check if service has been up for minimum time
		uptime := time.Since(h.startTime)
		minUptime := 10 * time.Second

		status := HealthStatusPass
		statusCode := http.StatusOK
		output := "Service started successfully"

		if uptime < minUptime {
			status = HealthStatusFail
			statusCode = http.StatusServiceUnavailable
			output = fmt.Sprintf("Service starting up, please wait %v", minUptime-uptime)
		}

		response := HealthResponse{
			Status:      status,
			Version:     h.config.ServiceVersion,
			ServiceID:   uuid.New().String(),
			ServiceName: h.config.ServiceName,
			Output:      output,
			Metadata: map[string]interface{}{
				"uptime_seconds":     uptime.Seconds(),
				"min_uptime_seconds": minUptime.Seconds(),
				"timestamp":          time.Now().UTC(),
			},
		}

		w.Header().Set("Content-Type", "application/health+json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

// runChecks runs all registered health checks
func (h *HealthService) runChecks(ctx context.Context) map[string]HealthCheckResult {
	results := make(map[string]HealthCheckResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checker := range h.checkers {
		wg.Add(1)
		go func(n string, c HealthChecker) {
			defer wg.Done()

			// Check cache
			if cached, ok := h.getCachedResult(n); ok {
				mu.Lock()
				results[n] = cached
				mu.Unlock()
				return
			}

			// Run check with timeout
			checkCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
			defer cancel()

			result := c.Check(checkCtx)
			result.LastChecked = time.Now()

			// Cache result
			h.cacheResult(n, result)

			mu.Lock()
			results[n] = result
			mu.Unlock()
		}(name, checker)
	}

	wg.Wait()
	return results
}

// getCachedResult gets a cached health check result
func (h *HealthService) getCachedResult(name string) (HealthCheckResult, bool) {
	if val, ok := h.cache.Load(name); ok {
		cached := val.(cachedHealthResult)
		if time.Since(cached.timestamp) < h.config.CacheDuration {
			return cached.result, true
		}
	}
	return HealthCheckResult{}, false
}

// cacheResult caches a health check result
func (h *HealthService) cacheResult(name string, result HealthCheckResult) {
	h.cache.Store(name, cachedHealthResult{
		result:    result,
		timestamp: time.Now(),
	})
}

type cachedHealthResult struct {
	result    HealthCheckResult
	timestamp time.Time
}

// Built-in health checkers

// DatabaseHealthChecker checks database health
type DatabaseHealthChecker struct {
	db   *sql.DB
	name string
}

// NewDatabaseHealthChecker creates a new database health checker
func NewDatabaseHealthChecker(db *sql.DB, name string) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		db:   db,
		name: name,
	}
}

func (d *DatabaseHealthChecker) Name() string {
	return d.name
}

func (d *DatabaseHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	
	// Ping database
	err := d.db.PingContext(ctx)
	responseTime := time.Since(start)

	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusFail,
			Error:        err.Error(),
			ResponseTime: responseTime,
			LastChecked:  time.Now(),
		}
	}

	// Get connection stats
	stats := d.db.Stats()
	
	status := HealthStatusPass
	message := "Database is healthy"
	
	// Check connection pool health
	if stats.OpenConnections > stats.MaxOpenConnections*9/10 {
		status = HealthStatusWarn
		message = "Connection pool near capacity"
	}

	return HealthCheckResult{
		Status:       status,
		Message:      message,
		ResponseTime: responseTime,
		Metadata: map[string]interface{}{
			"open_connections": stats.OpenConnections,
			"in_use":          stats.InUse,
			"idle":            stats.Idle,
			"wait_count":      stats.WaitCount,
			"wait_duration":   stats.WaitDuration.String(),
		},
		LastChecked: time.Now(),
	}
}

// RedisHealthChecker checks Redis health
type RedisHealthChecker struct {
	client *redis.Client
	name   string
}

// NewRedisHealthChecker creates a new Redis health checker
func NewRedisHealthChecker(client *redis.Client, name string) *RedisHealthChecker {
	return &RedisHealthChecker{
		client: client,
		name:   name,
	}
}

func (r *RedisHealthChecker) Name() string {
	return r.name
}

func (r *RedisHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	
	// Ping Redis
	_, err := r.client.Ping(ctx).Result()
	responseTime := time.Since(start)

	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusFail,
			Error:        err.Error(),
			ResponseTime: responseTime,
			LastChecked:  time.Now(),
		}
	}

	// Get Redis info
	info, err := r.client.Info(ctx, "server", "memory", "stats").Result()
	
	metadata := make(map[string]interface{})
	if err == nil {
		// Parse some basic info
		metadata["info_sample"] = len(info) > 100
	}

	return HealthCheckResult{
		Status:       HealthStatusPass,
		Message:      "Redis is healthy",
		ResponseTime: responseTime,
		Metadata:     metadata,
		LastChecked:  time.Now(),
	}
}

// HTTPHealthChecker checks external HTTP service health
type HTTPHealthChecker struct {
	name   string
	url    string
	client *http.Client
}

// NewHTTPHealthChecker creates a new HTTP health checker
func NewHTTPHealthChecker(name, url string) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (h *HTTPHealthChecker) Name() string {
	return h.name
}

func (h *HTTPHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, nil)
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusFail,
			Error:        err.Error(),
			ResponseTime: time.Since(start),
			LastChecked:  time.Now(),
		}
	}

	resp, err := h.client.Do(req)
	responseTime := time.Since(start)
	
	if err != nil {
		return HealthCheckResult{
			Status:       HealthStatusFail,
			Error:        err.Error(),
			ResponseTime: responseTime,
			LastChecked:  time.Now(),
		}
	}
	defer resp.Body.Close()

	status := HealthStatusPass
	message := fmt.Sprintf("Service responded with %d", resp.StatusCode)
	
	if resp.StatusCode >= 500 {
		status = HealthStatusFail
	} else if resp.StatusCode >= 400 {
		status = HealthStatusWarn
	}

	return HealthCheckResult{
		Status:       status,
		Message:      message,
		ResponseTime: responseTime,
		Metadata: map[string]interface{}{
			"status_code": resp.StatusCode,
			"url":         h.url,
		},
		LastChecked: time.Now(),
	}
}

// SystemHealthChecker checks system resources
type SystemHealthChecker struct{}

// NewSystemHealthChecker creates a new system health checker
func NewSystemHealthChecker() *SystemHealthChecker {
	return &SystemHealthChecker{}
}

func (s *SystemHealthChecker) Name() string {
	return "system"
}

func (s *SystemHealthChecker) Check(ctx context.Context) HealthCheckResult {
	start := time.Now()
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	status := HealthStatusPass
	message := "System resources are healthy"
	
	// Check memory usage
	heapUsagePercent := float64(m.HeapAlloc) / float64(m.HeapSys) * 100
	if heapUsagePercent > 90 {
		status = HealthStatusFail
		message = "Memory usage critical"
	} else if heapUsagePercent > 75 {
		status = HealthStatusWarn
		message = "Memory usage high"
	}
	
	// Check goroutines
	numGoroutines := runtime.NumGoroutine()
	if numGoroutines > 10000 {
		status = HealthStatusWarn
		message = "High number of goroutines"
	}

	return HealthCheckResult{
		Status:       status,
		Message:      message,
		ResponseTime: time.Since(start),
		Metadata: map[string]interface{}{
			"goroutines":        numGoroutines,
			"heap_alloc_mb":     m.HeapAlloc / 1024 / 1024,
			"heap_sys_mb":       m.HeapSys / 1024 / 1024,
			"heap_usage_percent": fmt.Sprintf("%.2f", heapUsagePercent),
			"gc_runs":           m.NumGC,
			"go_version":        runtime.Version(),
		},
		LastChecked: time.Now(),
	}
}