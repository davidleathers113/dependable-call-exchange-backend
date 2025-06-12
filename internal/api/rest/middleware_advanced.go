package rest

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

// MiddlewareChain builds a chain of middleware
type MiddlewareChain struct {
	middlewares []Middleware
}

// Middleware represents a function that wraps an HTTP handler
type Middleware func(http.Handler) http.Handler

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain(middlewares ...Middleware) *MiddlewareChain {
	return &MiddlewareChain{middlewares: middlewares}
}

// Then wraps the handler with all middleware in the chain
func (c *MiddlewareChain) Then(h http.Handler) http.Handler {
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}
	return h
}

// Advanced middleware implementations

// CircuitBreakerMiddleware implements circuit breaker pattern
func CircuitBreakerMiddleware(threshold int, timeout time.Duration) Middleware {
	type circuitBreaker struct {
		failures    int
		lastFailure time.Time
		mu          sync.RWMutex
		state       string // "closed", "open", "half-open"
	}

	breakers := make(map[string]*circuitBreaker)
	var mu sync.RWMutex

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			mu.RLock()
			cb, exists := breakers[path]
			mu.RUnlock()

			if !exists {
				mu.Lock()
				cb = &circuitBreaker{state: "closed"}
				breakers[path] = cb
				mu.Unlock()
			}

			cb.mu.RLock()
			state := cb.state
			cb.mu.RUnlock()

			if state == "open" {
				cb.mu.RLock()
				if time.Since(cb.lastFailure) > timeout {
					cb.mu.RUnlock()
					cb.mu.Lock()
					cb.state = "half-open"
					cb.mu.Unlock()
				} else {
					cb.mu.RUnlock()
					writeServiceUnavailable(w)
					return
				}
			}

			// Wrap response writer to capture status
			wrapped := &statusRecorder{ResponseWriter: w, statusCode: 200}
			next.ServeHTTP(wrapped, r)

			// Update circuit breaker state
			cb.mu.Lock()
			defer cb.mu.Unlock()

			if wrapped.statusCode >= 500 {
				cb.failures++
				cb.lastFailure = time.Now()
				if cb.failures >= threshold {
					cb.state = "open"
				}
			} else if cb.state == "half-open" {
				cb.state = "closed"
				cb.failures = 0
			}
		})
	}
}

// CompressionMiddleware adds gzip compression support
func CompressionMiddleware(minSize int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if client accepts gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Create gzip response writer
			gz := &gzipResponseWriter{
				ResponseWriter: w,
				minSize:        minSize,
			}
			defer gz.Close()

			// Set content encoding
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Del("Content-Length") // Content-Length is not valid with compression

			next.ServeHTTP(gz, r)
		})
	}
}

// SecurityHeadersMiddleware adds comprehensive security headers
func SecurityHeadersMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline';")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware ensures every request has a unique ID
func RequestIDMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			// Add to response headers
			w.Header().Set("X-Request-ID", requestID)

			// Add to context
			ctx := context.WithValue(r.Context(), contextKey("request_id"), requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MetricsMiddleware tracks detailed metrics
func MetricsMiddleware() Middleware {
	// Define metrics
	httpDuration := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	httpRequests := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	httpRequestSize := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_size_bytes",
		Help:    "Size of HTTP requests.",
		Buckets: prometheus.ExponentialBuckets(100, 10, 8),
	}, []string{"method", "path"})

	httpResponseSize := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_response_size_bytes",
		Help:    "Size of HTTP responses.",
		Buckets: prometheus.ExponentialBuckets(100, 10, 8),
	}, []string{"method", "path", "status"})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Track request size
			if r.ContentLength > 0 {
				httpRequestSize.WithLabelValues(r.Method, r.URL.Path).Observe(float64(r.ContentLength))
			}

			// Wrap response writer to capture metrics
			wrapped := &metricsResponseWriter{
				ResponseWriter: w,
				statusCode:     200,
			}

			next.ServeHTTP(wrapped, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			status := fmt.Sprintf("%d", wrapped.statusCode)

			httpDuration.WithLabelValues(r.Method, r.URL.Path, status).Observe(duration)
			httpRequests.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			httpResponseSize.WithLabelValues(r.Method, r.URL.Path, status).Observe(float64(wrapped.bytesWritten))
		})
	}
}

// AdvancedRateLimitMiddleware provides sophisticated rate limiting
type RateLimiter struct {
	limiters sync.Map
	config   RateLimitConfig
}

type RateLimitConfig struct {
	RequestsPerSecond int
	Burst             int
	ByIP              bool
	ByUser            bool
	ByEndpoint        bool
	CustomKeyFunc     func(r *http.Request) string
}

func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	return &RateLimiter{config: config}
}

func (rl *RateLimiter) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rl.getKey(r)
			
			limiterInterface, _ := rl.limiters.LoadOrStore(key, rate.NewLimiter(
				rate.Limit(rl.config.RequestsPerSecond),
				rl.config.Burst,
			))
			limiter := limiterInterface.(*rate.Limiter)

			if !limiter.Allow() {
				// Get wait time
				reservation := limiter.Reserve()
				wait := reservation.Delay()
				reservation.Cancel()

				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.RequestsPerSecond))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(wait).Unix()))
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(wait.Seconds())))

				writeRateLimitExceeded(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) getKey(r *http.Request) string {
	if rl.config.CustomKeyFunc != nil {
		return rl.config.CustomKeyFunc(r)
	}

	var parts []string

	if rl.config.ByIP {
		parts = append(parts, getClientIP(r))
	}

	if rl.config.ByUser {
		if userID, ok := r.Context().Value(contextKeyUserID).(string); ok {
			parts = append(parts, userID)
		}
	}

	if rl.config.ByEndpoint {
		parts = append(parts, r.Method, r.URL.Path)
	}

	return strings.Join(parts, ":")
}

// CacheMiddleware provides intelligent caching
type CacheMiddleware struct {
	cache     sync.Map
	ttl       time.Duration
	keyFunc   func(r *http.Request) string
	condition func(r *http.Request) bool
}

func NewCacheMiddleware(ttl time.Duration) *CacheMiddleware {
	return &CacheMiddleware{
		ttl: ttl,
		keyFunc: func(r *http.Request) string {
			return fmt.Sprintf("%s:%s:%s", r.Method, r.URL.Path, r.URL.RawQuery)
		},
		condition: func(r *http.Request) bool {
			return r.Method == "GET"
		},
	}
}

func (cm *CacheMiddleware) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cm.condition(r) {
				next.ServeHTTP(w, r)
				return
			}

			key := cm.keyFunc(r)

			// Check cache
			if cached, ok := cm.cache.Load(key); ok {
				entry := cached.(*cacheEntry)
				if time.Since(entry.timestamp) < cm.ttl {
					// Serve from cache
					for k, v := range entry.headers {
						w.Header()[k] = v
					}
					w.Header().Set("X-Cache", "HIT")
					w.WriteHeader(entry.statusCode)
					w.Write(entry.body)
					return
				}
			}

			// Cache miss - capture response
			recorder := &cacheRecorder{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
			}

			next.ServeHTTP(recorder, r)

			// Store in cache if successful
			if recorder.statusCode >= 200 && recorder.statusCode < 300 {
				cm.cache.Store(key, &cacheEntry{
					statusCode: recorder.statusCode,
					headers:    recorder.Header().Clone(),
					body:       recorder.body.Bytes(),
					timestamp:  time.Now(),
				})
			}
		})
	}
}

// TracingMiddleware provides comprehensive distributed tracing
func TracingMiddleware(tracer trace.Tracer) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from headers
			ctx := r.Context()
			
			// Start span
			ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.scheme", r.URL.Scheme),
					attribute.String("http.host", r.Host),
					attribute.String("http.user_agent", r.UserAgent()),
					attribute.String("http.remote_addr", r.RemoteAddr),
				),
			)
			defer span.End()

			// Wrap response writer to capture status
			wrapped := &tracingResponseWriter{
				ResponseWriter: w,
				statusCode:     200,
			}

			// Add trace ID to response headers
			if span.SpanContext().HasTraceID() {
				w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
			}

			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Record response attributes
			span.SetAttributes(
				attribute.Int("http.status_code", wrapped.statusCode),
				attribute.Int64("http.response_size", wrapped.bytesWritten),
			)

			// Set span status based on HTTP status
			if wrapped.statusCode >= 400 {
				span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
			}
		})
	}
}

// RequestLoggingMiddleware provides structured request/response logging
func RequestLoggingMiddleware(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create request ID if not present
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			// Log request
			logger.InfoContext(r.Context(), "request_started",
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("query", r.URL.RawQuery),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
			)

			// Wrap response writer
			wrapped := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     200,
			}

			next.ServeHTTP(wrapped, r)

			// Log response
			duration := time.Since(start)
			logger.InfoContext(r.Context(), "request_completed",
				slog.String("request_id", requestID),
				slog.Int("status", wrapped.statusCode),
				slog.Int64("bytes", wrapped.bytesWritten),
				slog.Duration("duration", duration),
				slog.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
			)
		})
	}
}

// ContentNegotiationMiddleware handles multiple response formats
func ContentNegotiationMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse Accept header
			accept := r.Header.Get("Accept")
			
			// Determine preferred content type
			var contentType string
			switch {
			case strings.Contains(accept, "application/json"):
				contentType = "application/json"
			case strings.Contains(accept, "application/xml"):
				contentType = "application/xml"
			case strings.Contains(accept, "text/csv"):
				contentType = "text/csv"
			case strings.Contains(accept, "application/msgpack"):
				contentType = "application/msgpack"
			default:
				contentType = "application/json" // Default
			}

			// Store in context for handlers to use
			ctx := context.WithValue(r.Context(), contextKey("content_type"), contentType)
			
			// Set response content type
			w.Header().Set("Content-Type", contentType)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper types and functions

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer  *gzip.Writer
	minSize int
	buf     bytes.Buffer
	written bool
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.written {
		g.buf.Write(b)
		if g.buf.Len() >= g.minSize {
			g.initGzip()
		}
		return len(b), nil
	}
	return g.writer.Write(b)
}

func (g *gzipResponseWriter) initGzip() {
	g.written = true
	g.writer = gzip.NewWriter(g.ResponseWriter)
	g.writer.Write(g.buf.Bytes())
	g.buf.Reset()
}

func (g *gzipResponseWriter) Close() {
	if !g.written && g.buf.Len() > 0 {
		// Not enough data for compression, write directly
		g.ResponseWriter.Header().Del("Content-Encoding")
		g.ResponseWriter.Write(g.buf.Bytes())
	} else if g.writer != nil {
		g.writer.Close()
	}
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (m *metricsResponseWriter) WriteHeader(code int) {
	m.statusCode = code
	m.ResponseWriter.WriteHeader(code)
}

func (m *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := m.ResponseWriter.Write(b)
	m.bytesWritten += int64(n)
	return n, err
}

type cacheEntry struct {
	statusCode int
	headers    http.Header
	body       []byte
	timestamp  time.Time
}

type cacheRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (c *cacheRecorder) WriteHeader(code int) {
	c.statusCode = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *cacheRecorder) Write(b []byte) (int, error) {
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}

type tracingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (t *tracingResponseWriter) WriteHeader(code int) {
	t.statusCode = code
	t.ResponseWriter.WriteHeader(code)
}

func (t *tracingResponseWriter) Write(b []byte) (int, error) {
	n, err := t.ResponseWriter.Write(b)
	t.bytesWritten += int64(n)
	return n, err
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.statusCode = code
	l.ResponseWriter.WriteHeader(code)
}

func (l *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := l.ResponseWriter.Write(b)
	l.bytesWritten += int64(n)
	return n, err
}

// Error responses
func writeServiceUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "SERVICE_UNAVAILABLE",
			"message": "Service temporarily unavailable",
		},
	})
}

func writeRateLimitExceeded(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "RATE_LIMIT_EXCEEDED",
			"message": "Too many requests",
		},
	})
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	
	return ip
}

// Context keys are defined in handler_base.go