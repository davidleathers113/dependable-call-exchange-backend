package rest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type MockSessionStore struct {
	mock.Mock
}

func (m *MockSessionStore) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	args := m.Called(ctx, sessionID)
	return args.Bool(0), args.Error(1)
}

func (m *MockSessionStore) RevokeSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockSessionStore) CreateSession(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error) {
	args := m.Called(ctx, userID, ttl)
	return args.String(0), args.Error(1)
}

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) GetUser(ctx context.Context, userID uuid.UUID) (*User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockUserService) ValidatePermissions(ctx context.Context, userID uuid.UUID, required []string) (bool, error) {
	args := m.Called(ctx, userID, required)
	return args.Bool(0), args.Error(1)
}

// Test helpers
func generateTestRSAKeys(t *testing.T) (*rsa.PrivateKey, string, string) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Generate private key PEM
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Generate public key PEM
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return privateKey, string(publicKeyPEM), string(privateKeyPEM)
}

func createTestAuthConfig(t *testing.T, useRSA bool) (*AuthConfig, *rsa.PrivateKey) {
	config := &AuthConfig{
		JWTSecret:          []byte("test-secret-key"),
		TokenExpiry:        time.Hour,
		RefreshTokenExpiry: 24 * time.Hour,
		Issuer:             "test-issuer",
		Audience:           []string{"test-audience"},
		UseRSA:             useRSA,
	}

	var privateKey *rsa.PrivateKey
	if useRSA {
		var publicPEM, privatePEM string
		privateKey, publicPEM, privatePEM = generateTestRSAKeys(t)
		
		publicKey, privKey, err := LoadRSAKeys(publicPEM, privatePEM)
		require.NoError(t, err)
		
		config.JWTPublicKey = publicKey
		config.JWTPrivateKey = privKey
	}

	return config, privateKey
}

func createTestUser() *User {
	return &User{
		ID:          uuid.New(),
		Email:       "test@example.com",
		AccountID:   uuid.New(),
		AccountType: "buyer",
		Permissions: []string{"read:calls", "write:calls"},
		Active:      true,
		MFAEnabled:  false,
	}
}

func setupAuthMiddleware(t *testing.T, useRSA bool) (*AuthMiddleware, *MockSessionStore, *MockUserService, *AuthConfig) {
	config, _ := createTestAuthConfig(t, useRSA)
	sessionStore := new(MockSessionStore)
	userService := new(MockUserService)
	
	middleware := NewAuthMiddleware(config, sessionStore, userService)
	
	return middleware, sessionStore, userService, config
}

// Test JWT Token Generation and Validation
func TestAuthMiddleware_TokenGeneration(t *testing.T) {
	t.Run("GenerateToken with HMAC", func(t *testing.T) {
		middleware, _, _, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the token can be parsed
		claims, err := middleware.validateToken(context.Background(), token)
		require.NoError(t, err)
		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, user.AccountID, claims.AccountID)
		assert.Equal(t, user.AccountType, claims.AccountType)
		assert.Equal(t, user.Email, claims.Email)
		assert.Equal(t, user.Permissions, claims.Permissions)
		assert.Equal(t, sessionID, claims.SessionID)
	})

	t.Run("GenerateToken with RSA", func(t *testing.T) {
		middleware, _, _, _ := setupAuthMiddleware(t, true)
		user := createTestUser()
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the token can be parsed
		claims, err := middleware.validateToken(context.Background(), token)
		require.NoError(t, err)
		assert.Equal(t, user.ID, claims.UserID)
		assert.Equal(t, sessionID, claims.SessionID)
	})

	t.Run("GenerateRefreshToken", func(t *testing.T) {
		middleware, _, _, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		sessionID := "test-session-123"

		refreshToken, err := middleware.GenerateRefreshToken(user, sessionID)
		require.NoError(t, err)
		assert.NotEmpty(t, refreshToken)

		// Parse and validate refresh token using jwt.ParseWithClaims directly 
		// since parseToken uses Claims struct, but refresh tokens use RegisteredClaims
		token, err := jwt.ParseWithClaims(refreshToken, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
			return middleware.config.JWTSecret, nil
		})
		require.NoError(t, err)
		
		claims := token.Claims.(*jwt.RegisteredClaims)
		assert.Equal(t, user.ID.String(), claims.Subject)
		assert.Contains(t, claims.Audience, "refresh")
		assert.Equal(t, sessionID, claims.ID)
	})

	t.Run("Token expiry validation", func(t *testing.T) {
		config, _ := createTestAuthConfig(t, false)
		config.TokenExpiry = -time.Hour // Expired token
		
		sessionStore := new(MockSessionStore)
		userService := new(MockUserService)
		middleware := NewAuthMiddleware(config, sessionStore, userService)
		
		user := createTestUser()
		token, err := middleware.GenerateToken(user, "session")
		require.NoError(t, err)

		// Should fail validation due to expiry
		_, err = middleware.validateToken(context.Background(), token)
		assert.Error(t, err)
	})
}

// Test Token Refresh Functionality
func TestAuthMiddleware_RefreshToken(t *testing.T) {
	t.Run("successful token refresh", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		sessionID := "test-session-123"

		// Generate refresh token
		refreshToken, err := middleware.GenerateRefreshToken(user, sessionID)
		require.NoError(t, err)

		// Setup mocks
		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		// Refresh tokens
		newAccessToken, newRefreshToken, err := middleware.RefreshToken(context.Background(), refreshToken)
		require.NoError(t, err)
		assert.NotEmpty(t, newAccessToken)
		assert.NotEmpty(t, newRefreshToken)

		// Validate new access token
		claims, err := middleware.validateToken(context.Background(), newAccessToken)
		require.NoError(t, err)
		assert.Equal(t, user.ID, claims.UserID)

		sessionStore.AssertExpectations(t)
		userService.AssertExpectations(t)
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		middleware, _, _, _ := setupAuthMiddleware(t, false)

		_, _, err := middleware.RefreshToken(context.Background(), "invalid.token.here")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid refresh token")
	})

	t.Run("not a refresh token", func(t *testing.T) {
		middleware, _, _, _ := setupAuthMiddleware(t, false)
		user := createTestUser()

		// Generate regular access token instead of refresh token
		accessToken, err := middleware.GenerateToken(user, "session")
		require.NoError(t, err)

		_, _, err = middleware.RefreshToken(context.Background(), accessToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a refresh token")
	})

	t.Run("inactive user", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		user.Active = false
		sessionID := "test-session-123"

		refreshToken, err := middleware.GenerateRefreshToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		_, _, err = middleware.RefreshToken(context.Background(), refreshToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found or inactive")
	})

	t.Run("invalid session", func(t *testing.T) {
		middleware, sessionStore, _, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		sessionID := "invalid-session"

		refreshToken, err := middleware.GenerateRefreshToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(false, nil)

		_, _, err = middleware.RefreshToken(context.Background(), refreshToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid session")
	})
}

// Test Middleware Authentication
func TestAuthMiddleware_Middleware(t *testing.T) {
	t.Run("successful authentication", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		// Setup mocks
		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		// Create test handler
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify context enrichment
			userID := r.Context().Value(contextKeyUserID)
			accountType := r.Context().Value(contextKeyAccountType)
			assert.Equal(t, user.ID, userID)
			assert.Equal(t, user.AccountType, accountType)
			w.WriteHeader(http.StatusOK)
		})

		// Wrap with auth middleware
		authHandler := middleware.Middleware()(testHandler)

		// Make request with token
		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		sessionStore.AssertExpectations(t)
		userService.AssertExpectations(t)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		middleware, _, _, _ := setupAuthMiddleware(t, false)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
	})

	t.Run("token from cookie", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		// Setup mocks
		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid token format", func(t *testing.T) {
		middleware, _, _, _ := setupAuthMiddleware(t, false)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid authorization header")
	})

	t.Run("malformed JWT token", func(t *testing.T) {
		middleware, _, _, _ := setupAuthMiddleware(t, false)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer invalid.jwt.token")
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid or expired token")
	})

	t.Run("expired token", func(t *testing.T) {
		config, _ := createTestAuthConfig(t, false)
		config.TokenExpiry = -time.Hour // Past expiry
		
		sessionStore := new(MockSessionStore)
		userService := new(MockUserService)
		middleware := NewAuthMiddleware(config, sessionStore, userService)
		
		user := createTestUser()
		token, err := middleware.GenerateToken(user, "session")
		require.NoError(t, err)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid session", func(t *testing.T) {
		middleware, sessionStore, _, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		sessionID := "invalid-session"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(false, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid session")
	})

	t.Run("inactive user", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		user.Active = false
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "User account is not active")
	})
}

// Test Permission-Based Authorization
func TestAuthMiddleware_PermissionAuthorization(t *testing.T) {
	t.Run("sufficient permissions", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		user.Permissions = []string{"read:calls", "write:calls", "admin:users"}
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Require admin:users permission
		authHandler := middleware.Middleware("admin:users")(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("insufficient permissions", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		user.Permissions = []string{"read:calls"}
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Require admin:users permission
		authHandler := middleware.Middleware("admin:users")(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "FORBIDDEN")
		assert.Contains(t, w.Body.String(), "Insufficient permissions")
	})

	t.Run("wildcard permission", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		user.Permissions = []string{"*"} // Admin with all permissions
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware("admin:users", "write:accounts")(testHandler)

		req := httptest.NewRequest("DELETE", "/api/v1/admin/users/123", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("multiple permissions - any match", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		user.Permissions = []string{"read:calls", "admin:calls"}
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// User has admin:calls, which is one of the required permissions
		authHandler := middleware.Middleware("admin:users", "admin:calls")(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("no permissions required", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
		user := createTestUser()
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// No permissions required
		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// Test RSA vs HMAC Token Handling
func TestAuthMiddleware_RSAvsHMAC(t *testing.T) {
	t.Run("RSA token validation", func(t *testing.T) {
		middleware, sessionStore, userService, _ := setupAuthMiddleware(t, true)
		user := createTestUser()
		sessionID := "test-session-123"

		token, err := middleware.GenerateToken(user, sessionID)
		require.NoError(t, err)

		sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
		userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		authHandler := middleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("wrong signing method for RSA", func(t *testing.T) {
		// Create HMAC token but use RSA middleware
		hmacMiddleware, _, _, _ := setupAuthMiddleware(t, false)
		rsaMiddleware, _, _, _ := setupAuthMiddleware(t, true)
		
		user := createTestUser()
		
		// Generate token with HMAC
		hmacToken, err := hmacMiddleware.GenerateToken(user, "session")
		require.NoError(t, err)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Try to validate HMAC token with RSA middleware
		authHandler := rsaMiddleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer "+hmacToken)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("wrong signing method for HMAC", func(t *testing.T) {
		// Create RSA token but use HMAC middleware
		rsaMiddleware, _, _, _ := setupAuthMiddleware(t, true)
		hmacMiddleware, _, _, _ := setupAuthMiddleware(t, false)
		
		user := createTestUser()
		
		// Generate token with RSA
		rsaToken, err := rsaMiddleware.GenerateToken(user, "session")
		require.NoError(t, err)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Try to validate RSA token with HMAC middleware
		authHandler := hmacMiddleware.Middleware()(testHandler)

		req := httptest.NewRequest("GET", "/api/v1/calls", nil)
		req.Header.Set("Authorization", "Bearer "+rsaToken)
		w := httptest.NewRecorder()

		authHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// Test Context Enrichment
func TestAuthMiddleware_ContextEnrichment(t *testing.T) {
	middleware, sessionStore, userService, _ := setupAuthMiddleware(t, false)
	user := createTestUser()
	sessionID := "test-session-123"

	token, err := middleware.GenerateToken(user, sessionID)
	require.NoError(t, err)

	sessionStore.On("ValidateSession", mock.Anything, sessionID).Return(true, nil)
	userService.On("GetUser", mock.Anything, user.ID).Return(user, nil)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Verify all context values are set
		assert.Equal(t, user.ID, ctx.Value(contextKeyUserID))
		assert.Equal(t, user.AccountType, ctx.Value(contextKeyAccountType))
		assert.Equal(t, user.AccountID, ctx.Value(contextKey("account_id")))
		assert.Equal(t, user.Email, ctx.Value(contextKey("email")))
		assert.Equal(t, user.Permissions, ctx.Value(contextKey("permissions")))
		assert.Equal(t, sessionID, ctx.Value(contextKey("session_id")))
		assert.Equal(t, user, ctx.Value(contextKey("user")))
		
		w.WriteHeader(http.StatusOK)
	})

	authHandler := middleware.Middleware()(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/calls", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	authHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	sessionStore.AssertExpectations(t)
	userService.AssertExpectations(t)
}

// Test LoadRSAKeys Function
func TestLoadRSAKeys(t *testing.T) {
	t.Run("valid RSA keys", func(t *testing.T) {
		_, publicPEM, privatePEM := generateTestRSAKeys(t)

		publicKey, privateKey, err := LoadRSAKeys(publicPEM, privatePEM)
		require.NoError(t, err)
		assert.NotNil(t, publicKey)
		assert.NotNil(t, privateKey)
	})

	t.Run("invalid public key PEM", func(t *testing.T) {
		_, _, privatePEM := generateTestRSAKeys(t)

		_, _, err := LoadRSAKeys("invalid-pem", privatePEM)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse public key PEM")
	})

	t.Run("invalid private key PEM", func(t *testing.T) {
		_, publicPEM, _ := generateTestRSAKeys(t)

		_, _, err := LoadRSAKeys(publicPEM, "invalid-pem")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse private key PEM")
	})

	t.Run("non-RSA public key", func(t *testing.T) {
		// Generate an EC key instead of RSA
		_, _, privatePEM := generateTestRSAKeys(t)
		
		// Create invalid public key (just test with empty bytes)
		invalidPublicPEM := "-----BEGIN PUBLIC KEY-----\n" +
			"MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\n" +
			"-----END PUBLIC KEY-----"

		_, _, err := LoadRSAKeys(invalidPublicPEM, privatePEM)
		assert.Error(t, err)
	})
}

// Test Error Response Formatting
func TestAuthMiddleware_ErrorResponses(t *testing.T) {
	middleware, _, _, _ := setupAuthMiddleware(t, false)

	tests := []struct {
		name               string
		setupRequest       func() *http.Request
		expectedStatus     int
		expectedCode       string
		expectedMessage    string
		expectedHeaders    map[string]string
	}{
		{
			name: "unauthorized with WWW-Authenticate header",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/api/v1/calls", nil)
			},
			expectedStatus:  http.StatusUnauthorized,
			expectedCode:    "UNAUTHORIZED",
			expectedMessage: "no authorization token provided",
			expectedHeaders: map[string]string{
				"WWW-Authenticate": `Bearer realm="api"`,
			},
		},
		{
			name: "forbidden without WWW-Authenticate header",
			setupRequest: func() *http.Request {
				// This would trigger in the permission check, but we'll simulate
				// by testing the writeForbidden method indirectly
				req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
				req.Header.Set("Authorization", "Bearer valid.token")
				return req
			},
			expectedStatus:  http.StatusForbidden,
			expectedCode:    "FORBIDDEN",
			expectedMessage: "Insufficient permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			authHandler := middleware.Middleware("admin:users")(testHandler)
			req := tt.setupRequest()
			w := httptest.NewRecorder()

			authHandler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			// Check headers
			for key, expectedValue := range tt.expectedHeaders {
				assert.Equal(t, expectedValue, w.Header().Get(key))
			}

			// Check response body structure
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			errorObj := response["error"].(map[string]interface{})
			assert.Equal(t, tt.expectedCode, errorObj["code"])
			assert.Contains(t, errorObj["message"], tt.expectedMessage)
		})
	}
}