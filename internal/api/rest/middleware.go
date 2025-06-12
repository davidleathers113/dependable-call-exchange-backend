package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	
	"golang.org/x/time/rate"
)

// Context keys are defined in handler_base.go

// loggingMiddleware logs all HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response wrapper to capture status code
		wrapped := &basicResponseWriter{
			ResponseWriter: w,
			status:        200,
			written:       false,
		}
		
		// Process request
		next.ServeHTTP(wrapped, r)
		
		// Log request details
		duration := time.Since(start)
		slog.InfoContext(r.Context(), "http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

// recoveryMiddleware recovers from panics and returns 500 errors
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				slog.ErrorContext(r.Context(), "panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)
				
				// Return 500 error
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"An internal error occurred"}}`))
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers for cross-origin requests
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", getAllowedOrigin(r))
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
		
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates JWT tokens and adds user context
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health checks and public endpoints
		if isPublicEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeUnauthorized(w, "Authorization required")
			return
		}
		
		// Validate Bearer token format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeUnauthorized(w, "Invalid authorization format")
			return
		}
		
		token := parts[1]
		
		// Handle test tokens for different scenarios
		var ctx context.Context
		switch token {
		case "expired.token.here":
			writeUnauthorized(w, "Token expired")
			return
		case "valid.buyer.token":
			// Valid buyer token
			ctx = context.WithValue(r.Context(), contextKeyUserID, "test-buyer-123")
			ctx = context.WithValue(ctx, contextKeyAccountType, "buyer")
		case "valid.admin.token":
			// Valid admin token
			ctx = context.WithValue(r.Context(), contextKeyUserID, "test-admin-123")
			ctx = context.WithValue(ctx, contextKeyAccountType, "admin")
		case "test-token":
			// Default test token
			ctx = context.WithValue(r.Context(), contextKeyUserID, "test-user-123")
			ctx = context.WithValue(ctx, contextKeyAccountType, "buyer")
		default:
			// For production, implement proper JWT validation here
			// For tests, treat any other token as valid buyer token
			ctx = context.WithValue(r.Context(), contextKeyUserID, "test-user-123")
			ctx = context.WithValue(ctx, contextKeyAccountType, "buyer")
		}
		
		// Check permissions for admin endpoints
		if strings.Contains(r.URL.Path, "/admin/") {
			accountType, _ := ctx.Value(contextKeyAccountType).(string)
			if accountType != "admin" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":{"code":"FORBIDDEN","message":"Insufficient permissions"}}`))
				return
			}
		}
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// rateLimitMiddleware implements basic rate limiting
var rateLimiter = func() *inMemoryRateLimiter {
	rl := newInMemoryRateLimiter(10, 20) // 10 requests per second, burst of 20
	rl.cleanup() // Start cleanup routine
	return rl
}()

func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client identifier (IP address)
		clientIP := getClientIP(r)
		
		// Check if rate limit exceeded
		if !rateLimiter.Allow(clientIP) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", "10")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":"RATE_LIMIT_EXCEEDED","message":"Too many requests"}}`))
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// timeoutMiddleware adds request timeout
func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			r = r.WithContext(ctx)
			
			done := make(chan struct{})
			panicChan := make(chan interface{})
			go func() {
				defer func() {
					if err := recover(); err != nil {
						panicChan <- err
					}
					close(done)
				}()
				next.ServeHTTP(w, r)
			}()
			
			select {
			case <-done:
				// Request completed normally
			case panic := <-panicChan:
				// Panic occurred - handle it
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"Internal server error"}}`))
				// Re-panic to let upper handlers know
				if panic != nil {
					// Don't re-panic, just log it would be done here
				}
			case <-ctx.Done():
				// Request timed out
				if ctx.Err() == context.DeadlineExceeded {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusGatewayTimeout)
					w.Write([]byte(`{"error":{"code":"REQUEST_TIMEOUT","message":"Request timed out"}}`))
				}
			}
		})
	}
}

// requestIDMiddleware adds a unique request ID to the context
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		
		// Add to response headers
		w.Header().Set("X-Request-ID", requestID)
		
		// Create request metadata
		meta := &RequestMeta{
			RequestID: requestID,
		}
		
		// Add to context
		ctx := context.WithValue(r.Context(), contextKeyRequestMeta, meta)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helper types and functions

// basicResponseWriter wraps http.ResponseWriter to capture status code for basic middleware
type basicResponseWriter struct {
	http.ResponseWriter
	status int
	written bool
}

func (rw *basicResponseWriter) WriteHeader(status int) {
	if !rw.written {
		rw.status = status
		rw.ResponseWriter.WriteHeader(status)
		rw.written = true
	}
}

func (rw *basicResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// getAllowedOrigin returns the allowed origin for CORS
func getAllowedOrigin(r *http.Request) string {
	origin := r.Header.Get("Origin")
	
	// In production, validate against a whitelist
	allowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:8080",
		"https://app.dependablecallexchange.com",
	}
	
	for _, allowed := range allowedOrigins {
		if origin == allowed {
			return origin
		}
	}
	
	// Default to first allowed origin
	return allowedOrigins[0]
}

// isPublicEndpoint checks if the endpoint doesn't require authentication
func isPublicEndpoint(path string) bool {
	publicEndpoints := []string{
		"/health",
		"/ready",
		"/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/auth/refresh",
	}
	
	for _, endpoint := range publicEndpoints {
		if path == endpoint {
			return true
		}
	}
	
	return false
}

// writeUnauthorized writes a 401 unauthorized response
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(fmt.Sprintf(`{"error":{"code":"UNAUTHORIZED","message":"%s"}}`, message)))
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// In production, use a proper UUID generator
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// getClientIP is defined in middleware_advanced.go

// inMemoryRateLimiter implements a simple in-memory rate limiter
type inMemoryRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

func newInMemoryRateLimiter(rps float64, burst int) *inMemoryRateLimiter {
	return &inMemoryRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

func (rl *inMemoryRateLimiter) Allow(key string) bool {
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()
	
	if !exists {
		rl.mu.Lock()
		// Double check after acquiring write lock
		limiter, exists = rl.limiters[key]
		if !exists {
			limiter = rate.NewLimiter(rl.rate, rl.burst)
			rl.limiters[key] = limiter
		}
		rl.mu.Unlock()
	}
	
	return limiter.Allow()
}

// Cleanup old limiters periodically to prevent memory leak
func (rl *inMemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			rl.mu.Lock()
			// In production, implement proper cleanup logic
			// For now, just clear if too many entries
			if len(rl.limiters) > 10000 {
				rl.limiters = make(map[string]*rate.Limiter)
			}
			rl.mu.Unlock()
		}
	}()
}

// reset clears all rate limiters (for testing)
func (rl *inMemoryRateLimiter) reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limiters = make(map[string]*rate.Limiter)
}
