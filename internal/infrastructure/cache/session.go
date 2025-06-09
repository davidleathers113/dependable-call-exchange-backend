package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// redisSessionStore implements the SessionStore interface using Redis
type redisSessionStore struct {
	cache  Cache
	client *redis.Client
	logger *zap.Logger
}

// NewRedisSessionStore creates a new Redis-based session store
func NewRedisSessionStore(cache Cache, client *redis.Client, logger *zap.Logger) SessionStore {
	return &redisSessionStore{
		cache:  cache,
		client: client,
		logger: logger,
	}
}

// CreateSession creates a new session with the given data
func (s *redisSessionStore) CreateSession(ctx context.Context, userID uuid.UUID, data map[string]interface{}) (string, error) {
	// Generate unique session ID
	sessionID := uuid.New().String()
	sessionKey := SessionPrefix + sessionID
	userSessionsKey := UserPrefix + userID.String() + ":sessions"
	
	// Add user ID to session data
	sessionData := make(map[string]interface{})
	for k, v := range data {
		sessionData[k] = v
	}
	sessionData["user_id"] = userID.String()
	sessionData["created_at"] = time.Now().Unix()
	
	// Use pipeline for atomic operations
	pipe := s.client.Pipeline()
	
	// Store session data
	pipe.HMSet(ctx, sessionKey, sessionData)
	pipe.Expire(ctx, sessionKey, SessionTTL)
	
	// Add session to user's session list
	pipe.SAdd(ctx, userSessionsKey, sessionID)
	pipe.Expire(ctx, userSessionsKey, SessionTTL+time.Hour) // Slightly longer TTL for cleanup
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("session creation failed",
			zap.String("session_id", sessionID),
			zap.String("user_id", userID.String()),
			zap.Error(err))
		return "", fmt.Errorf("session creation failed: %w", err)
	}
	
	s.logger.Debug("session created",
		zap.String("session_id", sessionID),
		zap.String("user_id", userID.String()))
	
	return sessionID, nil
}

// GetSession retrieves session data by session ID
func (s *redisSessionStore) GetSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	sessionKey := SessionPrefix + sessionID
	
	result, err := s.client.HGetAll(ctx, sessionKey).Result()
	if err != nil {
		s.logger.Error("session get failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
		return nil, fmt.Errorf("session get failed: %w", err)
	}
	
	if len(result) == 0 {
		return nil, ErrSessionExpired{SessionID: sessionID}
	}
	
	// Convert string map to interface{} map
	sessionData := make(map[string]interface{})
	for k, v := range result {
		// Try to convert numeric strings back to numbers
		if k == "created_at" {
			if timestamp, err := strconv.ParseInt(v, 10, 64); err == nil {
				sessionData[k] = timestamp
				continue
			}
		}
		sessionData[k] = v
	}
	
	return sessionData, nil
}

// UpdateSession updates existing session data
func (s *redisSessionStore) UpdateSession(ctx context.Context, sessionID string, data map[string]interface{}) error {
	sessionKey := SessionPrefix + sessionID
	
	// Check if session exists first
	exists, err := s.client.Exists(ctx, sessionKey).Result()
	if err != nil {
		s.logger.Error("session exists check failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
		return fmt.Errorf("session exists check failed: %w", err)
	}
	
	if exists == 0 {
		return ErrSessionExpired{SessionID: sessionID}
	}
	
	// Update session data
	err = s.client.HMSet(ctx, sessionKey, data).Err()
	if err != nil {
		s.logger.Error("session update failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
		return fmt.Errorf("session update failed: %w", err)
	}
	
	s.logger.Debug("session updated", zap.String("session_id", sessionID))
	return nil
}

// DeleteSession removes a session
func (s *redisSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	sessionKey := SessionPrefix + sessionID
	
	// Get user ID first to clean up user sessions list
	userID, err := s.client.HGet(ctx, sessionKey, "user_id").Result()
	if err != nil && err != redis.Nil {
		s.logger.Error("failed to get user_id for session cleanup",
			zap.String("session_id", sessionID),
			zap.Error(err))
		// Continue with deletion even if we can't clean up user sessions
	}
	
	// Use pipeline for atomic operations
	pipe := s.client.Pipeline()
	
	// Delete session
	pipe.Del(ctx, sessionKey)
	
	// Remove from user's session list if we got the user ID
	if err == nil && userID != "" {
		userSessionsKey := UserPrefix + userID + ":sessions"
		pipe.SRem(ctx, userSessionsKey, sessionID)
	}
	
	_, err = pipe.Exec(ctx)
	if err != nil {
		s.logger.Error("session deletion failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
		return fmt.Errorf("session deletion failed: %w", err)
	}
	
	s.logger.Debug("session deleted", zap.String("session_id", sessionID))
	return nil
}

// ExtendSession updates the session TTL
func (s *redisSessionStore) ExtendSession(ctx context.Context, sessionID string, ttl time.Duration) error {
	sessionKey := SessionPrefix + sessionID
	
	// Check if session exists
	exists, err := s.client.Exists(ctx, sessionKey).Result()
	if err != nil {
		s.logger.Error("session exists check failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
		return fmt.Errorf("session exists check failed: %w", err)
	}
	
	if exists == 0 {
		return ErrSessionExpired{SessionID: sessionID}
	}
	
	// Extend TTL
	err = s.client.Expire(ctx, sessionKey, ttl).Err()
	if err != nil {
		s.logger.Error("session extend failed",
			zap.String("session_id", sessionID),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return fmt.Errorf("session extend failed: %w", err)
	}
	
	s.logger.Debug("session extended",
		zap.String("session_id", sessionID),
		zap.Duration("ttl", ttl))
	
	return nil
}

// ListSessions returns all active sessions for a user
func (s *redisSessionStore) ListSessions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	userSessionsKey := UserPrefix + userID.String() + ":sessions"
	
	sessionIDs, err := s.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		s.logger.Error("list sessions failed",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		return nil, fmt.Errorf("list sessions failed: %w", err)
	}
	
	// Filter out expired sessions
	var activeSessions []string
	for _, sessionID := range sessionIDs {
		sessionKey := SessionPrefix + sessionID
		exists, err := s.client.Exists(ctx, sessionKey).Result()
		if err != nil {
			s.logger.Warn("failed to check session existence",
				zap.String("session_id", sessionID),
				zap.Error(err))
			continue
		}
		
		if exists > 0 {
			activeSessions = append(activeSessions, sessionID)
		} else {
			// Clean up orphaned session ID
			s.client.SRem(ctx, userSessionsKey, sessionID)
		}
	}
	
	return activeSessions, nil
}

// CleanupExpired removes expired sessions (called by background job)
func (s *redisSessionStore) CleanupExpired(ctx context.Context) (int64, error) {
	pattern := SessionPrefix + "*"
	
	var cursor uint64
	var deletedCount int64
	
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			s.logger.Error("session cleanup scan failed", zap.Error(err))
			return deletedCount, fmt.Errorf("session cleanup scan failed: %w", err)
		}
		
		for _, key := range keys {
			// Check if key has expired
			ttl, err := s.client.TTL(ctx, key).Result()
			if err != nil {
				continue
			}
			
			// If TTL is -2, the key doesn't exist (already expired)
			// If TTL is -1, the key exists but has no expiration (shouldn't happen)
			if ttl == -2 || ttl == -1 {
				// Extract session ID from key
				sessionID := key[len(SessionPrefix):]
				s.DeleteSession(ctx, sessionID)
				deletedCount++
			}
		}
		
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	
	if deletedCount > 0 {
		s.logger.Info("session cleanup completed",
			zap.Int64("deleted_sessions", deletedCount))
	}
	
	return deletedCount, nil
}