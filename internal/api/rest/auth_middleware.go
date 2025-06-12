package rest

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret          []byte
	JWTPublicKey       *rsa.PublicKey
	JWTPrivateKey      *rsa.PrivateKey
	TokenExpiry        time.Duration
	RefreshTokenExpiry time.Duration
	Issuer             string
	Audience           []string
	UseRSA             bool
}

// Claims represents JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserID      uuid.UUID `json:"user_id"`
	AccountID   uuid.UUID `json:"account_id"`
	AccountType string    `json:"account_type"`
	Email       string    `json:"email"`
	Permissions []string  `json:"permissions"`
	SessionID   string    `json:"session_id"`
}

// AuthMiddleware provides JWT-based authentication
type AuthMiddleware struct {
	config      *AuthConfig
	tracer      trace.Tracer
	sessionStore SessionStore
	userService  UserService
}

// SessionStore manages sessions
type SessionStore interface {
	ValidateSession(ctx context.Context, sessionID string) (bool, error)
	RevokeSession(ctx context.Context, sessionID string) error
	CreateSession(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error)
}

// UserService provides user information
type UserService interface {
	GetUser(ctx context.Context, userID uuid.UUID) (*User, error)
	ValidatePermissions(ctx context.Context, userID uuid.UUID, required []string) (bool, error)
}

// User represents a user in the system
type User struct {
	ID          uuid.UUID
	Email       string
	AccountID   uuid.UUID
	AccountType string
	Permissions []string
	Active      bool
	MFAEnabled  bool
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(config *AuthConfig, sessionStore SessionStore, userService UserService) *AuthMiddleware {
	return &AuthMiddleware{
		config:       config,
		tracer:       otel.Tracer("api.rest.auth"),
		sessionStore: sessionStore,
		userService:  userService,
	}
}

// Middleware returns the authentication middleware function
func (a *AuthMiddleware) Middleware(requiredPermissions ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := a.tracer.Start(r.Context(), "auth.middleware",
				trace.WithAttributes(
					attribute.StringSlice("required_permissions", requiredPermissions),
				),
			)
			defer span.End()

			// Extract token from header
			token, err := a.extractToken(r)
			if err != nil {
				span.RecordError(err)
				a.writeUnauthorized(w, "Invalid authorization header")
				return
			}

			// Validate token
			claims, err := a.validateToken(ctx, token)
			if err != nil {
				span.RecordError(err)
				a.writeUnauthorized(w, "Invalid or expired token")
				return
			}

			// Validate session
			if claims.SessionID != "" {
				valid, err := a.sessionStore.ValidateSession(ctx, claims.SessionID)
				if err != nil || !valid {
					span.RecordError(err)
					a.writeUnauthorized(w, "Invalid session")
					return
				}
			}

			// Validate user is still active
			user, err := a.userService.GetUser(ctx, claims.UserID)
			if err != nil || !user.Active {
				span.RecordError(err)
				a.writeUnauthorized(w, "User account is not active")
				return
			}

			// Check permissions
			if len(requiredPermissions) > 0 {
				hasPermission := false
				for _, required := range requiredPermissions {
					for _, userPerm := range claims.Permissions {
						if userPerm == required || userPerm == "*" {
							hasPermission = true
							break
						}
					}
					if hasPermission {
						break
					}
				}

				if !hasPermission {
					a.writeForbidden(w, "Insufficient permissions")
					return
				}
			}

			// Add claims to context
			ctx = a.enrichContext(ctx, claims, user)
			
			// Add auth info to span
			span.SetAttributes(
				attribute.String("user_id", claims.UserID.String()),
				attribute.String("account_type", claims.AccountType),
				attribute.StringSlice("permissions", claims.Permissions),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GenerateToken generates a new JWT token
func (a *AuthMiddleware) GenerateToken(user *User, sessionID string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.config.Issuer,
			Subject:   user.ID.String(),
			Audience:  a.config.Audience,
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.TokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
		UserID:      user.ID,
		AccountID:   user.AccountID,
		AccountType: user.AccountType,
		Email:       user.Email,
		Permissions: user.Permissions,
		SessionID:   sessionID,
	}

	var token *jwt.Token
	if a.config.UseRSA {
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		return token.SignedString(a.config.JWTPrivateKey)
	}

	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.config.JWTSecret)
}

// GenerateRefreshToken generates a refresh token
func (a *AuthMiddleware) GenerateRefreshToken(user *User, sessionID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    a.config.Issuer,
		Subject:   user.ID.String(),
		Audience:  []string{"refresh"},
		ExpiresAt: jwt.NewNumericDate(now.Add(a.config.RefreshTokenExpiry)),
		NotBefore: jwt.NewNumericDate(now),
		IssuedAt:  jwt.NewNumericDate(now),
		ID:        sessionID,
	}

	var token *jwt.Token
	if a.config.UseRSA {
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		return token.SignedString(a.config.JWTPrivateKey)
	}

	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.config.JWTSecret)
}

// RefreshToken validates a refresh token and issues new tokens
func (a *AuthMiddleware) RefreshToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error) {
	// Parse refresh token (refresh tokens use RegisteredClaims, not our custom Claims)
	var token *jwt.Token
	if a.config.UseRSA {
		token, err = jwt.ParseWithClaims(refreshToken, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return a.config.JWTPublicKey, nil
		})
	} else {
		token, err = jwt.ParseWithClaims(refreshToken, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return a.config.JWTSecret, nil
		})
	}
	
	if err != nil {
		return "", "", fmt.Errorf("invalid refresh token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return "", "", errors.New("invalid refresh token claims")
	}

	// Validate it's a refresh token
	if len(claims.Audience) == 0 || claims.Audience[0] != "refresh" {
		return "", "", errors.New("not a refresh token")
	}

	// Get user ID and session ID
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return "", "", fmt.Errorf("invalid user ID: %w", err)
	}

	sessionID := claims.ID

	// Validate session
	if sessionID != "" {
		valid, err := a.sessionStore.ValidateSession(ctx, sessionID)
		if err != nil || !valid {
			return "", "", errors.New("invalid session")
		}
	}

	// Get fresh user data
	user, err := a.userService.GetUser(ctx, userID)
	if err != nil || !user.Active {
		return "", "", errors.New("user not found or inactive")
	}

	// Generate new tokens
	accessToken, err = a.GenerateToken(user, sessionID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err = a.GenerateRefreshToken(user, sessionID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, newRefreshToken, nil
}

// Private methods

func (a *AuthMiddleware) extractToken(r *http.Request) (string, error) {
	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// Check cookie as fallback
		cookie, err := r.Cookie("access_token")
		if err != nil {
			return "", errors.New("no authorization token provided")
		}
		return cookie.Value, nil
	}

	// Extract Bearer token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}

func (a *AuthMiddleware) validateToken(ctx context.Context, tokenString string) (*Claims, error) {
	token, err := a.parseToken(tokenString)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

func (a *AuthMiddleware) parseToken(tokenString string) (*jwt.Token, error) {
	if a.config.UseRSA {
		return jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return a.config.JWTPublicKey, nil
		})
	}

	return jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.config.JWTSecret, nil
	})
}

func (a *AuthMiddleware) enrichContext(ctx context.Context, claims *Claims, user *User) context.Context {
	ctx = context.WithValue(ctx, contextKeyUserID, claims.UserID)
	ctx = context.WithValue(ctx, contextKeyAccountType, claims.AccountType)
	ctx = context.WithValue(ctx, contextKey("account_id"), claims.AccountID)
	ctx = context.WithValue(ctx, contextKey("email"), claims.Email)
	ctx = context.WithValue(ctx, contextKey("permissions"), claims.Permissions)
	ctx = context.WithValue(ctx, contextKey("session_id"), claims.SessionID)
	ctx = context.WithValue(ctx, contextKey("user"), user)
	return ctx
}

func (a *AuthMiddleware) writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "UNAUTHORIZED",
			"message": message,
		},
	})
}

func (a *AuthMiddleware) writeForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "FORBIDDEN",
			"message": message,
		},
	})
}

// LoadRSAKeys loads RSA keys from PEM encoded strings
func LoadRSAKeys(publicKeyPEM, privateKeyPEM string) (*rsa.PublicKey, *rsa.PrivateKey, error) {
	// Parse public key
	pubBlock, _ := pem.Decode([]byte(publicKeyPEM))
	if pubBlock == nil {
		return nil, nil, errors.New("failed to parse public key PEM")
	}

	pub, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, nil, errors.New("not an RSA public key")
	}

	// Parse private key
	privBlock, _ := pem.Decode([]byte(privateKeyPEM))
	if privBlock == nil {
		return nil, nil, errors.New("failed to parse private key PEM")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(privBlock.Bytes)
	if err != nil {
		// Try PKCS8 format
		privInterface, err := x509.ParsePKCS8PrivateKey(privBlock.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		privateKey, ok = privInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, errors.New("not an RSA private key")
		}
	}

	return publicKey, privateKey, nil
}