package rest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/repository"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"log/slog"
	"strconv"
	"strings"
)


// RateLimiterMiddleware wraps a rate limiter for use as middleware
type RateLimiterMiddleware struct {
	rateLimiter cache.RateLimiter
	config      RateLimitConfig
}

// Middleware returns the middleware function
func (rlm *RateLimiterMiddleware) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get rate limit key based on IP, user, and endpoint
			key := rlm.getRateLimitKey(r)
			
			// Check rate limit
			allowed, err := rlm.rateLimiter.Allow(r.Context(), key, rlm.config.RequestsPerSecond, time.Second)
			if err != nil {
				// On error, allow the request but log it
				next.ServeHTTP(w, r)
				return
			}
			
			if !allowed {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rlm.config.RequestsPerSecond))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))
				w.Header().Set("Retry-After", "1")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

func (rlm *RateLimiterMiddleware) getRateLimitKey(r *http.Request) string {
	var parts []string
	
	// By IP
	if rlm.config.ByIP {
		ip := r.Header.Get("X-Real-IP")
		if ip == "" {
			ip = r.Header.Get("X-Forwarded-For")
			if ip == "" {
				ip = r.RemoteAddr
			}
		}
		parts = append(parts, "ip:"+ip)
	}
	
	// By User
	if rlm.config.ByUser {
		if userID := r.Context().Value(contextKeyUserID); userID != nil {
			parts = append(parts, "user:"+userID.(string))
		}
	}
	
	// By Endpoint
	if rlm.config.ByEndpoint {
		parts = append(parts, "endpoint:"+r.URL.Path)
	}
	
	return strings.Join(parts, ":")
}

// Server represents the API server
type Server struct {
	config       *config.Config
	httpServer   *http.Server
	handler      *Handler
	logger       *slog.Logger
	tracer       trace.Tracer
	db           *sql.DB
	redis        *redis.Client
	services     *Services
	middlewares  []Middleware
	healthService *HealthService
}

// NewServer creates a new API server with all dependencies
func NewServer(cfg *config.Config) (*Server, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	
	// Create zap logger for dependencies that need it
	zapLogger, _ := zap.NewProduction()

	// Initialize database
	db, err := database.Connect(cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Convert to sql.DB for compatibility
	// db is already a *pgxpool.Pool, need to wrap it
	sqlDB := stdlib.OpenDBFromPool(db)

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Address,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Initialize repositories
	repos := &repository.Repositories{
		Account:    repository.NewAccountRepository(sqlDB),
		Bid:        repository.NewBidRepository(sqlDB),
		Call:       repository.NewCallRepository(sqlDB),
		Compliance: repository.NewComplianceRepository(db),  // Uses pgxpool
		Financial:  repository.NewFinancialRepository(db),   // Uses pgxpool
	}

	// Initialize cache manager
	cacheManager, err := cache.NewCacheManager(&cfg.Redis, zapLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache manager: %w", err)
	}

	// Initialize services using factories
	factories := service.NewServiceFactories(repos)
	
	// Create consent service first as it's needed by call routing
	consentService := factories.CreateConsentService()
	
	services := &Services{
		Repositories: repos,
		Consent:      consentService,
		CallRouting:  factories.CreateCallRoutingService(consentService),
		Bidding:      factories.CreateBiddingService(),
		Fraud:        factories.CreateFraudService(),
		Telephony:    factories.CreateTelephonyService(),
	}

	// Initialize authentication
	authConfig := &AuthConfig{
		JWTSecret:          []byte(cfg.Security.JWTSecret),
		TokenExpiry:        time.Duration(cfg.Security.TokenExpiryMinutes) * time.Minute,
		RefreshTokenExpiry: time.Duration(cfg.Security.RefreshTokenExpiryDays) * 24 * time.Hour,
		Issuer:             "dependable-call-exchange",
		Audience:           []string{"api"},
		UseRSA:             false,
	}

	// Initialize session store
	sessionStore := NewRedisSessionStore(redisClient, "session")

	// Create auth middleware
	authMiddleware := NewAuthMiddleware(authConfig, sessionStore, &mockUserService{})

	// Initialize rate limiter
	rateLimitConfig := RateLimitConfig{
		RequestsPerSecond: cfg.Security.RateLimit.RequestsPerSecond,
		Burst:             cfg.Security.RateLimit.Burst,
		ByIP:              true,
		ByUser:            true,
		ByEndpoint:        true,
	}
	rateLimiterMiddleware := &RateLimiterMiddleware{
		rateLimiter: cacheManager.RateLimiter,
		config:      rateLimitConfig,
	}

	// Initialize CORS
	corsConfig := DefaultCORSConfig()
	corsConfig.AllowedOrigins = cfg.CORS.AllowedOrigins
	corsMiddleware := NewCORSMiddleware(corsConfig)

	// Initialize CSRF
	csrfConfig := DefaultCSRFConfig()
	csrfConfig.TrustedOrigins = cfg.CORS.AllowedOrigins
	csrfStore := NewRedisCSRFStore(redisClient, "csrf")
	csrfMiddleware := NewCSRFMiddleware(csrfConfig, csrfStore)

	// Security headers middleware is already defined in middleware.go

	// Initialize health service
	healthService := NewHealthService(DefaultHealthConfig())
	healthService.RegisterChecker("database", NewDatabaseHealthChecker(sqlDB, "postgres"))
	healthService.RegisterChecker("redis", NewRedisHealthChecker(redisClient, "redis"))
	healthService.RegisterChecker("system", NewSystemHealthChecker())

	// Create handler
	handler := NewHandler(services)

	// Initialize tracer
	tracer := otel.Tracer("api.rest.server")
	
	// Build middleware chain
	middlewares := []Middleware{
		// Observability
		requestIDMiddleware,
		loggingMiddleware,
		MetricsMiddleware(),
		TracingMiddleware(tracer),
		
		// Recovery
		recoveryMiddleware,
		
		// Security
		SecurityHeadersMiddleware(),
		corsMiddleware.Middleware(),
		
		// Rate limiting (before auth to prevent brute force)
		rateLimiterMiddleware.Middleware(),
		
		// Timeout
		timeoutMiddleware(30 * time.Second),
		
		// Authentication (skip for health endpoints)
		ConditionalMiddleware(
			authMiddleware.Middleware(),
			func(r *http.Request) bool {
				// Skip auth for health checks and docs
				return !isPublicAPIEndpoint(r.URL.Path)
			},
		),
		
		// CSRF (after auth, skip for API endpoints)
		ConditionalMiddleware(
			csrfMiddleware.Middleware(),
			func(r *http.Request) bool {
				// Skip CSRF for API endpoints (they use JWT)
				return !isAPIEndpoint(r.URL.Path)
			},
		),
		
		// Compression
		CompressionMiddleware(6),
	}

	// Create server
	server := &Server{
		config:        cfg,
		handler:       handler,
		logger:        logger,
		tracer:        tracer,
		db:            sqlDB,
		redis:         redisClient,
		services:      services,
		middlewares:   middlewares,
		healthService: healthService,
	}

	// Create HTTP server
	mux := server.setupRoutes()
	
	// Apply middleware chain
	var h http.Handler = mux
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}

	server.httpServer = &http.Server{
		Addr:           cfg.Server.Address,
		Handler:        h,
		ReadTimeout:    time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:   time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:    time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	return server, nil
}

// setupRoutes configures all routes
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Health checks
	mux.HandleFunc("/health", s.healthService.ReadinessHandler())
	mux.HandleFunc("/healthz", s.healthService.LivenessHandler())
	mux.HandleFunc("/ready", s.healthService.ReadinessHandler())
	mux.HandleFunc("/startup", s.healthService.StartupHandler())

	// API documentation
	mux.HandleFunc("/docs", s.handler.handleDocs)
	mux.HandleFunc("/docs/openapi.json", s.handler.handleOpenAPISpec)

	// API v1 routes
	v1 := http.NewServeMux()
	
	// Account routes
	v1.HandleFunc("POST /accounts", s.handler.handleCreateAccount)
	v1.HandleFunc("GET /accounts", s.handler.handleGetAccounts)
	v1.HandleFunc("GET /accounts/{id}", s.handler.handleGetAccount)
	v1.HandleFunc("PUT /accounts/{id}", s.handler.handleUpdateAccount)
	v1.HandleFunc("DELETE /accounts/{id}", s.handler.handleDeleteAccount)

	// Call routes
	v1.HandleFunc("POST /calls", s.handler.handleCreateCall)
	v1.HandleFunc("GET /calls", s.handler.handleGetCalls)
	v1.HandleFunc("GET /calls/{id}", s.handler.handleGetCall)
	v1.HandleFunc("PUT /calls/{id}", s.handler.handleUpdateCall)
	v1.HandleFunc("DELETE /calls/{id}", s.handler.handleDeleteCall)
	v1.HandleFunc("GET /calls/{id}/status", s.handler.handleGetCallStatus)

	// Bid routes
	v1.HandleFunc("POST /bids", s.handler.handleCreateBid)
	v1.HandleFunc("GET /bids", s.handler.handleGetBids)
	v1.HandleFunc("GET /bids/{id}", s.handler.handleGetBid)
	v1.HandleFunc("PUT /bids/{id}", s.handler.handleUpdateBid)
	v1.HandleFunc("DELETE /bids/{id}", s.handler.handleDeleteBid)

	// Auction routes
	v1.HandleFunc("POST /auctions", s.handler.handleCreateAuction)
	v1.HandleFunc("GET /auctions", s.handler.handleGetAuction)
	v1.HandleFunc("GET /auctions/{id}", s.handler.handleGetAuction)
	v1.HandleFunc("POST /auctions/{id}/close", s.handler.handleCloseAuction)

	// WebSocket
	v1.HandleFunc("/ws", s.handler.handleWebSocket)

	// Mount v1 routes
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	return mux
}

// Start starts the server
func (s *Server) Start() error {
	s.logger.Info("starting API server",
		"address", s.httpServer.Addr,
		"environment", s.config.Environment,
	)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server failed to start: %w", err)
	case sig := <-sigCh:
		s.logger.Info("received shutdown signal", "signal", sig)
		return s.Shutdown()
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.logger.Info("shutting down server")

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("failed to shutdown server", "error", err)
		return err
	}

	// Close database
	if err := s.db.Close(); err != nil {
		s.logger.Error("failed to close database", "error", err)
	}

	// Close Redis
	if err := s.redis.Close(); err != nil {
		s.logger.Error("failed to close Redis", "error", err)
	}

	s.logger.Info("server shutdown complete")
	return nil
}

// Helper functions

func isPublicAPIEndpoint(path string) bool {
	// Check if it's one of the exact public paths
	publicPaths := map[string]bool{
		"/health":                true,
		"/healthz":               true,
		"/ready":                 true,
		"/startup":               true,
		"/docs":                  true,
		"/api/v1/auth/login":     true,
		"/api/v1/auth/register":  true,
		"/api/v1/auth/refresh":   true,
	}
	
	return publicPaths[path]
}

func isAPIEndpoint(path string) bool {
	return len(path) > 4 && path[:4] == "/api"
}

// ConditionalMiddleware applies middleware conditionally
func ConditionalMiddleware(mw Middleware, condition func(*http.Request) bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if condition(r) {
				mw(next).ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// mockUserService is a temporary implementation
type mockUserService struct{}

func (m *mockUserService) GetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	// TODO: Implement real user service
	return &User{
		ID:          userID,
		Email:       "user@example.com",
		AccountID:   uuid.New(),
		AccountType: "buyer",
		Permissions: []string{"read", "write"},
		Active:      true,
		MFAEnabled:  false,
	}, nil
}

func (m *mockUserService) ValidatePermissions(ctx context.Context, userID uuid.UUID, required []string) (bool, error) {
	// TODO: Implement real permission validation
	return true, nil
}