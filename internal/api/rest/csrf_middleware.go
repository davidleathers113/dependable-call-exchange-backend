package rest

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CSRFConfig configures CSRF protection
type CSRFConfig struct {
	TokenLength     int
	TokenExpiry     time.Duration
	CookieName      string
	HeaderName      string
	FieldName       string
	CookiePath      string
	CookieDomain    string
	CookieSecure    bool
	CookieHTTPOnly  bool
	CookieSameSite  http.SameSite
	TrustedOrigins  []string
	ExemptPaths     []string
	ExemptMethods   []string
	DoubleSubmit    bool // Use double-submit cookie pattern
	SessionBased    bool // Use session-based tokens
}

// DefaultCSRFConfig returns secure default configuration
func DefaultCSRFConfig() CSRFConfig {
	return CSRFConfig{
		TokenLength:     32,
		TokenExpiry:     24 * time.Hour,
		CookieName:      "csrf_token",
		HeaderName:      "X-CSRF-Token",
		FieldName:       "csrf_token",
		CookiePath:      "/",
		CookieSecure:    true,
		CookieHTTPOnly:  true,
		CookieSameSite:  http.SameSiteStrictMode,
		ExemptMethods:   []string{"GET", "HEAD", "OPTIONS"},
		DoubleSubmit:    false,
		SessionBased:    true,
	}
}

// CSRFMiddleware provides CSRF protection
type CSRFMiddleware struct {
	config       CSRFConfig
	store        CSRFTokenStore
	tracer       trace.Tracer
	trustedHosts map[string]bool
	exemptPaths  map[string]bool
}

// CSRFTokenStore manages CSRF tokens
type CSRFTokenStore interface {
	GetToken(sessionID string) (string, error)
	SetToken(sessionID string, token string, expiry time.Duration) error
	ValidateToken(sessionID string, token string) (bool, error)
	DeleteToken(sessionID string) error
}

// InMemoryCSRFStore provides in-memory CSRF token storage
type InMemoryCSRFStore struct {
	tokens map[string]*csrfToken
	mu     sync.RWMutex
}

type csrfToken struct {
	Token     string
	ExpiresAt time.Time
}

// NewCSRFMiddleware creates a new CSRF middleware
func NewCSRFMiddleware(config CSRFConfig, store CSRFTokenStore) *CSRFMiddleware {
	// Prepare trusted hosts map
	trustedHosts := make(map[string]bool)
	for _, origin := range config.TrustedOrigins {
		trustedHosts[origin] = true
	}

	// Prepare exempt paths map
	exemptPaths := make(map[string]bool)
	for _, path := range config.ExemptPaths {
		exemptPaths[path] = true
	}

	return &CSRFMiddleware{
		config:       config,
		store:        store,
		tracer:       otel.Tracer("api.rest.csrf"),
		trustedHosts: trustedHosts,
		exemptPaths:  exemptPaths,
	}
}

// Middleware returns the CSRF protection middleware
func (c *CSRFMiddleware) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := c.tracer.Start(r.Context(), "csrf.middleware",
				trace.WithAttributes(
					attribute.String("method", r.Method),
					attribute.String("path", r.URL.Path),
				),
			)
			defer span.End()

			// Check if method is exempt
			methodExempt := false
			for _, method := range c.config.ExemptMethods {
				if r.Method == method {
					methodExempt = true
					break
				}
			}

			// Check if path is exempt
			if c.exemptPaths[r.URL.Path] {
				span.SetAttributes(attribute.Bool("exempt_path", true))
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Get or create token for safe methods
			if methodExempt {
				c.ensureToken(w, r)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Validate CSRF token for unsafe methods
			if !c.validateRequest(r) {
				span.SetAttributes(attribute.Bool("csrf_valid", false))
				c.writeCSRFError(w)
				return
			}

			span.SetAttributes(attribute.Bool("csrf_valid", true))

			// Refresh token
			c.refreshToken(w, r)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GenerateToken generates a new CSRF token
func (c *CSRFMiddleware) GenerateToken() (string, error) {
	b := make([]byte, c.config.TokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetToken retrieves the CSRF token for a request
func (c *CSRFMiddleware) GetToken(r *http.Request) string {
	// Try cookie first
	cookie, err := r.Cookie(c.config.CookieName)
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Try session-based token if configured
	if c.config.SessionBased {
		sessionID := c.getSessionID(r)
		if sessionID != "" {
			token, err := c.store.GetToken(sessionID)
			if err == nil {
				return token
			}
		}
	}

	return ""
}

// Private methods

func (c *CSRFMiddleware) ensureToken(w http.ResponseWriter, r *http.Request) {
	// Check if token already exists
	existingToken := c.GetToken(r)
	if existingToken != "" {
		return
	}

	// Generate new token
	token, err := c.GenerateToken()
	if err != nil {
		return
	}

	// Store token
	if c.config.SessionBased {
		sessionID := c.getSessionID(r)
		if sessionID != "" {
			c.store.SetToken(sessionID, token, c.config.TokenExpiry)
		}
	}

	// Set cookie
	c.setTokenCookie(w, token)
}

func (c *CSRFMiddleware) validateRequest(r *http.Request) bool {
	// Check origin/referer
	if !c.validateOrigin(r) {
		return false
	}

	// Get token from cookie or session
	cookieToken := c.GetToken(r)
	if cookieToken == "" {
		return false
	}

	// Get token from request (header or form)
	requestToken := c.getRequestToken(r)
	if requestToken == "" {
		return false
	}

	// Compare tokens
	if c.config.DoubleSubmit {
		// Simple comparison for double-submit pattern
		return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(requestToken)) == 1
	}

	// Session-based validation
	if c.config.SessionBased {
		sessionID := c.getSessionID(r)
		if sessionID == "" {
			return false
		}

		valid, _ := c.store.ValidateToken(sessionID, requestToken)
		return valid
	}

	return false
}

func (c *CSRFMiddleware) validateOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}

	if origin == "" {
		// No origin/referer for same-origin requests is okay
		return true
	}

	// Check against trusted origins
	return c.trustedHosts[origin]
}

func (c *CSRFMiddleware) getRequestToken(r *http.Request) string {
	// Check header first
	token := r.Header.Get(c.config.HeaderName)
	if token != "" {
		return token
	}

	// Check form data
	if r.Method == "POST" {
		r.ParseForm()
		token = r.FormValue(c.config.FieldName)
	}

	return token
}

func (c *CSRFMiddleware) refreshToken(w http.ResponseWriter, r *http.Request) {
	if !c.config.SessionBased {
		return
	}

	// Generate new token
	token, err := c.GenerateToken()
	if err != nil {
		return
	}

	// Update in store
	sessionID := c.getSessionID(r)
	if sessionID != "" {
		c.store.SetToken(sessionID, token, c.config.TokenExpiry)
	}

	// Update cookie
	c.setTokenCookie(w, token)
}

func (c *CSRFMiddleware) setTokenCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     c.config.CookieName,
		Value:    token,
		Path:     c.config.CookiePath,
		Domain:   c.config.CookieDomain,
		Expires:  time.Now().Add(c.config.TokenExpiry),
		Secure:   c.config.CookieSecure,
		HttpOnly: c.config.CookieHTTPOnly,
		SameSite: c.config.CookieSameSite,
	}
	http.SetCookie(w, cookie)
}

func (c *CSRFMiddleware) getSessionID(r *http.Request) string {
	// Try to get session ID from context (set by auth middleware)
	if sessionID, ok := r.Context().Value(contextKey("session_id")).(string); ok {
		return sessionID
	}

	// Try session cookie
	cookie, err := r.Cookie("session_id")
	if err == nil {
		return cookie.Value
	}

	return ""
}

func (c *CSRFMiddleware) writeCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "CSRF_VALIDATION_FAILED",
			"message": "CSRF token validation failed",
		},
	})
}

// InMemoryCSRFStore implementation

// NewInMemoryCSRFStore creates a new in-memory CSRF store
func NewInMemoryCSRFStore() *InMemoryCSRFStore {
	store := &InMemoryCSRFStore{
		tokens: make(map[string]*csrfToken),
	}

	// Start cleanup goroutine
	go store.cleanup()

	return store
}

func (s *InMemoryCSRFStore) GetToken(sessionID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, exists := s.tokens[sessionID]
	if !exists {
		return "", fmt.Errorf("token not found")
	}

	if time.Now().After(token.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}

	return token.Token, nil
}

func (s *InMemoryCSRFStore) SetToken(sessionID string, token string, expiry time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[sessionID] = &csrfToken{
		Token:     token,
		ExpiresAt: time.Now().Add(expiry),
	}

	return nil
}

func (s *InMemoryCSRFStore) ValidateToken(sessionID string, token string) (bool, error) {
	storedToken, err := s.GetToken(sessionID)
	if err != nil {
		return false, err
	}

	return subtle.ConstantTimeCompare([]byte(storedToken), []byte(token)) == 1, nil
}

func (s *InMemoryCSRFStore) DeleteToken(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, sessionID)
	return nil
}

func (s *InMemoryCSRFStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for sessionID, token := range s.tokens {
			if now.After(token.ExpiresAt) {
				delete(s.tokens, sessionID)
			}
		}
		s.mu.Unlock()
	}
}

// RedisCSRFStore provides Redis-backed CSRF token storage
type RedisCSRFStore struct {
	client *redis.Client
	prefix string
	tracer trace.Tracer
}

// NewRedisCSRFStore creates a new Redis-backed CSRF store
func NewRedisCSRFStore(client *redis.Client, prefix string) *RedisCSRFStore {
	if prefix == "" {
		prefix = "csrf"
	}
	return &RedisCSRFStore{
		client: client,
		prefix: prefix,
		tracer: otel.Tracer("api.rest.csrf.redis"),
	}
}

func (s *RedisCSRFStore) GetToken(sessionID string) (string, error) {
	ctx, span := s.tracer.Start(context.Background(), "csrf.redis.get")
	defer span.End()

	key := fmt.Sprintf("%s:%s", s.prefix, sessionID)
	token, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("token not found")
		}
		return "", err
	}

	return token, nil
}

func (s *RedisCSRFStore) SetToken(sessionID string, token string, expiry time.Duration) error {
	ctx, span := s.tracer.Start(context.Background(), "csrf.redis.set")
	defer span.End()

	key := fmt.Sprintf("%s:%s", s.prefix, sessionID)
	return s.client.Set(ctx, key, token, expiry).Err()
}

func (s *RedisCSRFStore) ValidateToken(sessionID string, token string) (bool, error) {
	storedToken, err := s.GetToken(sessionID)
	if err != nil {
		return false, err
	}

	return subtle.ConstantTimeCompare([]byte(storedToken), []byte(token)) == 1, nil
}

func (s *RedisCSRFStore) DeleteToken(sessionID string) error {
	ctx, span := s.tracer.Start(context.Background(), "csrf.redis.delete")
	defer span.End()

	key := fmt.Sprintf("%s:%s", s.prefix, sessionID)
	return s.client.Del(ctx, key).Err()
}