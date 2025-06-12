package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RedisSessionStore implements SessionStore using Redis
type RedisSessionStore struct {
	client *redis.Client
	prefix string
	tracer trace.Tracer
}

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NewRedisSessionStore creates a new Redis-backed session store
func NewRedisSessionStore(client *redis.Client, prefix string) *RedisSessionStore {
	if prefix == "" {
		prefix = "session"
	}
	return &RedisSessionStore{
		client: client,
		prefix: prefix,
		tracer: otel.Tracer("api.rest.session"),
	}
}

// ValidateSession checks if a session is valid
func (s *RedisSessionStore) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	ctx, span := s.tracer.Start(ctx, "session.validate",
		trace.WithAttributes(
			attribute.String("session_id", sessionID),
		),
	)
	defer span.End()

	key := s.key(sessionID)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("failed to check session: %w", err)
	}

	valid := exists > 0
	span.SetAttributes(attribute.Bool("valid", valid))

	return valid, nil
}

// CreateSession creates a new session
func (s *RedisSessionStore) CreateSession(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error) {
	ctx, span := s.tracer.Start(ctx, "session.create",
		trace.WithAttributes(
			attribute.String("user_id", userID.String()),
			attribute.String("ttl", ttl.String()),
		),
	)
	defer span.End()

	session := &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Data:      make(map[string]interface{}),
	}

	// Add request context if available
	if ip := ctx.Value(contextKey("client_ip")); ip != nil {
		session.IP = ip.(string)
	}
	if ua := ctx.Value(contextKey("user_agent")); ua != nil {
		session.UserAgent = ua.(string)
	}

	// Serialize session
	data, err := json.Marshal(session)
	if err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("failed to marshal session: %w", err)
	}

	// Store in Redis
	key := s.key(session.ID)
	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		span.RecordError(err)
		return "", fmt.Errorf("failed to store session: %w", err)
	}

	// Also store in user's session list
	userKey := s.userKey(userID)
	if err := s.client.SAdd(ctx, userKey, session.ID).Err(); err != nil {
		// Log but don't fail
		span.RecordError(err)
	}
	s.client.Expire(ctx, userKey, ttl)

	span.SetAttributes(attribute.String("session_id", session.ID))
	return session.ID, nil
}

// RevokeSession revokes a session
func (s *RedisSessionStore) RevokeSession(ctx context.Context, sessionID string) error {
	ctx, span := s.tracer.Start(ctx, "session.revoke",
		trace.WithAttributes(
			attribute.String("session_id", sessionID),
		),
	)
	defer span.End()

	// Get session to find user ID
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		// Session might already be deleted
		return nil
	}

	// Delete session
	key := s.key(sessionID)
	if err := s.client.Del(ctx, key).Err(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Remove from user's session list
	if session != nil {
		userKey := s.userKey(session.UserID)
		s.client.SRem(ctx, userKey, sessionID)
	}

	return nil
}

// GetSession retrieves a session
func (s *RedisSessionStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	ctx, span := s.tracer.Start(ctx, "session.get",
		trace.WithAttributes(
			attribute.String("session_id", sessionID),
		),
	)
	defer span.End()

	key := s.key(sessionID)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		s.RevokeSession(ctx, sessionID)
		return nil, nil
	}

	return &session, nil
}

// UpdateSession updates session data
func (s *RedisSessionStore) UpdateSession(ctx context.Context, sessionID string, data map[string]interface{}) error {
	ctx, span := s.tracer.Start(ctx, "session.update",
		trace.WithAttributes(
			attribute.String("session_id", sessionID),
		),
	)
	defer span.End()

	// Get existing session
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}

	// Update data
	for k, v := range data {
		session.Data[k] = v
	}

	// Save back
	sessionData, err := json.Marshal(session)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	key := s.key(sessionID)
	ttl := time.Until(session.ExpiresAt)
	if err := s.client.Set(ctx, key, sessionData, ttl).Err(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// ExtendSession extends the expiration time of a session
func (s *RedisSessionStore) ExtendSession(ctx context.Context, sessionID string, extension time.Duration) error {
	ctx, span := s.tracer.Start(ctx, "session.extend",
		trace.WithAttributes(
			attribute.String("session_id", sessionID),
			attribute.String("extension", extension.String()),
		),
	)
	defer span.End()

	// Get existing session
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session not found")
	}

	// Update expiry
	session.ExpiresAt = session.ExpiresAt.Add(extension)

	// Save back
	sessionData, err := json.Marshal(session)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	key := s.key(sessionID)
	ttl := time.Until(session.ExpiresAt)
	if err := s.client.Set(ctx, key, sessionData, ttl).Err(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to extend session: %w", err)
	}

	return nil
}

// RevokeUserSessions revokes all sessions for a user
func (s *RedisSessionStore) RevokeUserSessions(ctx context.Context, userID uuid.UUID) error {
	ctx, span := s.tracer.Start(ctx, "session.revoke_user",
		trace.WithAttributes(
			attribute.String("user_id", userID.String()),
		),
	)
	defer span.End()

	userKey := s.userKey(userID)
	sessionIDs, err := s.client.SMembers(ctx, userKey).Result()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Revoke each session
	for _, sessionID := range sessionIDs {
		if err := s.RevokeSession(ctx, sessionID); err != nil {
			// Log but continue
			span.RecordError(err)
		}
	}

	// Delete user's session list
	s.client.Del(ctx, userKey)

	span.SetAttributes(attribute.Int("sessions_revoked", len(sessionIDs)))
	return nil
}

// GetUserSessions returns all active sessions for a user
func (s *RedisSessionStore) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]*Session, error) {
	ctx, span := s.tracer.Start(ctx, "session.get_user_sessions",
		trace.WithAttributes(
			attribute.String("user_id", userID.String()),
		),
	)
	defer span.End()

	userKey := s.userKey(userID)
	sessionIDs, err := s.client.SMembers(ctx, userKey).Result()
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, err := s.GetSession(ctx, sessionID)
		if err != nil {
			continue
		}
		if session != nil {
			sessions = append(sessions, session)
		}
	}

	span.SetAttributes(attribute.Int("session_count", len(sessions)))
	return sessions, nil
}

// CleanupExpiredSessions removes expired sessions
func (s *RedisSessionStore) CleanupExpiredSessions(ctx context.Context) error {
	// In a production system, this would be more sophisticated
	// For now, Redis TTL handles expiration automatically
	return nil
}

// Helper methods

func (s *RedisSessionStore) key(sessionID string) string {
	return fmt.Sprintf("%s:%s", s.prefix, sessionID)
}

func (s *RedisSessionStore) userKey(userID uuid.UUID) string {
	return fmt.Sprintf("%s:user:%s", s.prefix, userID)
}

// InMemorySessionStore provides a simple in-memory session store for testing
type InMemorySessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewInMemorySessionStore creates a new in-memory session store
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*Session),
	}
}

// ValidateSession checks if a session is valid
func (s *InMemorySessionStore) ValidateSession(ctx context.Context, sessionID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return false, nil
	}

	// Check expiry
	if time.Now().After(session.ExpiresAt) {
		return false, nil
	}

	return true, nil
}

// CreateSession creates a new session
func (s *InMemorySessionStore) CreateSession(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error) {
	session := &Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Data:      make(map[string]interface{}),
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	return session.ID, nil
}

// RevokeSession revokes a session
func (s *InMemorySessionStore) RevokeSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	return nil
}