package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
)

// Config holds API configuration
type Config struct {
	Version               string
	BaseURL               string
	EnableMetrics         bool
	EnableTracing         bool
	EnableCompression     bool
	CompressionMinSize    int
	EnableRateLimiting    bool
	PublicRateLimit       int
	PublicRateBurst       int
	AuthRateLimit         int
	AuthRateBurst         int
	EnableCircuitBreaker  bool
	CircuitBreakerTimeout time.Duration
	CacheDuration         time.Duration
	EnableWebSocket       bool
	EnableGraphQL         bool
	Logger                *slog.Logger
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Version:               "v1",
		BaseURL:               "https://api.dependablecallexchange.com",
		EnableMetrics:         true,
		EnableTracing:         true,
		EnableCompression:     true,
		CompressionMinSize:    1024, // 1KB
		EnableRateLimiting:    true,
		PublicRateLimit:       10,
		PublicRateBurst:       20,
		AuthRateLimit:         100,
		AuthRateBurst:         200,
		EnableCircuitBreaker:  true,
		CircuitBreakerTimeout: 30 * time.Second,
		CacheDuration:         5 * time.Minute,
		EnableWebSocket:       true,
		EnableGraphQL:         false, // Coming soon
		Logger:                slog.Default(),
	}
}

// NewRouter creates the gold standard API router
func NewRouter(config *Config) http.Handler {
	if config == nil {
		config = DefaultConfig()
	}

	// Create the mux
	mux := http.NewServeMux()

	// Create handlers
	handlers := NewHandlersV2(config.Version, config.BaseURL)

	// Build base middleware chain
	var middlewares []Middleware

	// Always add security headers
	middlewares = append(middlewares, SecurityHeadersMiddleware())

	// Request ID and logging are essential
	middlewares = append(middlewares, RequestIDMiddleware())
	middlewares = append(middlewares, RequestLoggingMiddleware(config.Logger))

	// Add metrics if enabled
	if config.EnableMetrics {
		middlewares = append(middlewares, MetricsMiddleware())
	}

	// Add tracing if enabled
	if config.EnableTracing {
		tracer := otel.Tracer("api.rest")
		middlewares = append(middlewares, TracingMiddleware(tracer))
	}

	// Add compression if enabled
	if config.EnableCompression {
		middlewares = append(middlewares, CompressionMiddleware(config.CompressionMinSize))
	}

	// Content negotiation for flexible response formats
	middlewares = append(middlewares, ContentNegotiationMiddleware())

	// Create the middleware chain
	chain := NewMiddlewareChain(middlewares...)

	// Register all routes
	handlers.RegisterRoutes(mux)

	// Add special endpoints

	// Metrics endpoint (Prometheus format)
	if config.EnableMetrics {
		mux.Handle("GET /metrics", promhttp.Handler())
	}

	// WebSocket endpoint for real-time updates
	if config.EnableWebSocket {
		wsHandler := NewWebSocketHandler(config.Logger)
		mux.Handle("GET /api/v1/ws", chain.Then(wsHandler))
	}

	// GraphQL endpoint (future enhancement)
	if config.EnableGraphQL {
		// graphqlHandler := NewGraphQLHandler()
		// mux.Handle("POST /api/v1/graphql", chain.Then(graphqlHandler))
		// mux.Handle("GET /api/v1/graphql/playground", chain.Then(graphqlPlayground))
	}

	// API documentation
	mux.Handle("GET /api/docs", chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve Swagger UI or ReDoc
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(swaggerUIHTML))
	})))

	// Apply global rate limiting if enabled
	var handler http.Handler = mux
	if config.EnableRateLimiting {
		globalRateLimiter := NewRateLimiter(RateLimitConfig{
			RequestsPerSecond: 1000, // Global limit
			Burst:             2000,
			ByIP:              true,
		})
		handler = globalRateLimiter.Middleware()(handler)
	}

	// Add panic recovery as the outermost middleware
	handler = RecoveryMiddleware(config.Logger)(handler)

	return handler
}

// RecoveryMiddleware handles panics gracefully
func RecoveryMiddleware(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						"error", err,
						"url", r.URL.String(),
						"method", r.Method,
					)

					// Return error response
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"error": map[string]interface{}{
							"code":    "INTERNAL_ERROR",
							"message": "An unexpected error occurred",
						},
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	logger *slog.Logger
	// Add upgrader and hub here
}

func NewWebSocketHandler(logger *slog.Logger) http.Handler {
	return &WebSocketHandler{logger: logger}
}

func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement WebSocket handling
	// - Upgrade connection
	// - Authenticate
	// - Subscribe to events
	// - Send real-time updates
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("WebSocket support coming soon"))
}

// Swagger UI HTML template
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <title>Dependable Call Exchange API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css">
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            background: #fafafa;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist/swagger-ui-standalone-preset.js"></script>
    <script>
    window.onload = function() {
        window.ui = SwaggerUIBundle({
            url: "/api/v1/openapi.json",
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            plugins: [
                SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "StandaloneLayout"
        });
    };
    </script>
</body>
</html>`