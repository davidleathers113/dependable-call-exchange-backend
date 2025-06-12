package rest

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CORSConfig configures CORS middleware
type CORSConfig struct {
	// AllowedOrigins is a list of origins that are allowed.
	// Can include wildcards (e.g., "https://*.example.com")
	AllowedOrigins []string

	// AllowedMethods is a list of methods the client is allowed to use.
	AllowedMethods []string

	// AllowedHeaders is list of non-simple headers the client is allowed to use.
	AllowedHeaders []string

	// ExposedHeaders indicates which headers are safe to expose.
	ExposedHeaders []string

	// MaxAge indicates how long the results of a preflight request can be cached.
	MaxAge time.Duration

	// AllowCredentials indicates whether the request can include user credentials.
	AllowCredentials bool

	// AllowPrivateNetwork indicates whether to allow private network access.
	AllowPrivateNetwork bool

	// OptionsPassthrough passes preflight requests to the next handler.
	OptionsPassthrough bool

	// OptionsSuccessStatus is the status code for successful OPTIONS requests.
	OptionsSuccessStatus int

	// Debug enables debug logging.
	Debug bool
}

// DefaultCORSConfig returns a secure default configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Accept-Language",
			"Content-Type",
			"Content-Language",
			"Authorization",
			"X-Request-ID",
			"X-CSRF-Token",
		},
		ExposedHeaders: []string{
			"X-Request-ID",
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Reset",
		},
		MaxAge:               12 * time.Hour,
		AllowCredentials:     true,
		AllowPrivateNetwork:  false,
		OptionsPassthrough:   false,
		OptionsSuccessStatus: http.StatusNoContent,
		Debug:                false,
	}
}

// CORSMiddleware provides CORS support
type CORSMiddleware struct {
	config         CORSConfig
	allowedOrigins map[string]bool
	allowedMethods string
	allowedHeaders string
	exposedHeaders string
	maxAge         string
	tracer         trace.Tracer
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(config CORSConfig) *CORSMiddleware {
	// Prepare allowed origins map
	allowedOrigins := make(map[string]bool)
	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			// Wildcard
			allowedOrigins["*"] = true
		} else {
			allowedOrigins[strings.ToLower(origin)] = true
		}
	}

	// Prepare header strings
	allowedMethods := strings.Join(config.AllowedMethods, ", ")
	allowedHeaders := strings.Join(config.AllowedHeaders, ", ")
	exposedHeaders := strings.Join(config.ExposedHeaders, ", ")
	maxAge := strconv.Itoa(int(config.MaxAge.Seconds()))

	return &CORSMiddleware{
		config:         config,
		allowedOrigins: allowedOrigins,
		allowedMethods: allowedMethods,
		allowedHeaders: allowedHeaders,
		exposedHeaders: exposedHeaders,
		maxAge:         maxAge,
		tracer:         otel.Tracer("api.rest.cors"),
	}
}

// Middleware returns the CORS middleware function
func (c *CORSMiddleware) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := c.tracer.Start(r.Context(), "cors.middleware",
				trace.WithAttributes(
					attribute.String("origin", r.Header.Get("Origin")),
					attribute.String("method", r.Method),
				),
			)
			defer span.End()

			origin := r.Header.Get("Origin")
			
			// Handle preflight
			if r.Method == http.MethodOptions {
				c.handlePreflight(w, r, origin)
				if !c.config.OptionsPassthrough {
					return
				}
			}

			// Handle actual request
			c.handleActualRequest(w, r, origin)

			// Update request with new context
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// handlePreflight handles preflight OPTIONS requests
func (c *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request, origin string) {
	headers := w.Header()

	// Check if origin is allowed
	if c.isOriginAllowed(origin) {
		headers.Set("Access-Control-Allow-Origin", origin)
		headers.Add("Vary", "Origin")

		if c.config.AllowCredentials {
			headers.Set("Access-Control-Allow-Credentials", "true")
		}

		// Preflight headers
		headers.Set("Access-Control-Allow-Methods", c.allowedMethods)
		headers.Set("Access-Control-Allow-Headers", c.allowedHeaders)
		headers.Set("Access-Control-Max-Age", c.maxAge)

		// Handle private network access
		if c.config.AllowPrivateNetwork && 
			r.Header.Get("Access-Control-Request-Private-Network") == "true" {
			headers.Set("Access-Control-Allow-Private-Network", "true")
		}
	}

	if !c.config.OptionsPassthrough {
		w.WriteHeader(c.config.OptionsSuccessStatus)
	}
}

// handleActualRequest handles non-preflight requests
func (c *CORSMiddleware) handleActualRequest(w http.ResponseWriter, r *http.Request, origin string) {
	headers := w.Header()

	// Check if origin is allowed
	if c.isOriginAllowed(origin) {
		headers.Set("Access-Control-Allow-Origin", origin)
		headers.Add("Vary", "Origin")

		if c.config.AllowCredentials {
			headers.Set("Access-Control-Allow-Credentials", "true")
		}

		if c.exposedHeaders != "" {
			headers.Set("Access-Control-Expose-Headers", c.exposedHeaders)
		}
	}
}

// isOriginAllowed checks if an origin is allowed
func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	// Check wildcard
	if c.allowedOrigins["*"] {
		return true
	}

	// Check exact match
	normalizedOrigin := strings.ToLower(origin)
	if c.allowedOrigins[normalizedOrigin] {
		return true
	}

	// Check pattern matches
	for allowed := range c.allowedOrigins {
		if strings.Contains(allowed, "*") && c.matchWildcard(normalizedOrigin, allowed) {
			return true
		}
	}

	return false
}

// matchWildcard matches origin against a wildcard pattern
func (c *CORSMiddleware) matchWildcard(origin, pattern string) bool {
	// Simple wildcard matching (e.g., "https://*.example.com")
	parts := strings.Split(pattern, "*")
	if len(parts) != 2 {
		return false
	}

	return strings.HasPrefix(origin, parts[0]) && strings.HasSuffix(origin, parts[1])
}

