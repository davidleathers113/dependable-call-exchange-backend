# Authentication & Authorization System Specification

**Status**: 🚨 CRITICAL SECURITY PRIORITY  
**Version**: 1.0  
**Last Updated**: January 2025  
**Author**: System Architecture Team  
**Effort Estimate**: 3-4 developer days

## Executive Summary

### Critical Security Vulnerability

**Problem**: Authentication middleware exists in the codebase but is NOT applied to any routes, leaving all APIs completely unprotected.

**Impact**: 
- All API endpoints are publicly accessible without authentication
- No user identity verification or access control
- Complete exposure of sensitive call, bid, and financial data
- Violation of security best practices and compliance requirements

**Solution**: Implement comprehensive authentication and authorization system with JWT tokens, role-based access control (RBAC), and API key management.

### Priority Level: P0 - IMMEDIATE

This is the #1 security priority for the DCE platform. No other feature development should proceed until this vulnerability is addressed.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    API Gateway Layer                         │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │   Auth      │  │    Rate      │  │   Request ID    │   │
│  │ Middleware  │→ │   Limiter    │→ │   Middleware    │   │
│  └─────────────┘  └──────────────┘  └─────────────────┘   │
└───────────────────────────┬─────────────────────────────────┘
                           │
┌───────────────────────────┴─────────────────────────────────┐
│                  Authentication Service                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
│  │  Login   │  │  Token   │  │ Session  │  │  API Key  │  │
│  │ Handler  │  │ Service  │  │ Manager  │  │  Service  │  │
│  └──────────┘  └──────────┘  └──────────┘  └───────────┘  │
└─────────────────────────────────────────────────────────────┘
                           │
┌───────────────────────────┴─────────────────────────────────┐
│                  Authorization Service                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────────┐             │
│  │   RBAC   │  │Permission│  │  Resource    │             │
│  │  Engine  │  │  Checker │  │  Validator   │             │
│  └──────────┘  └──────────┘  └──────────────┘             │
└─────────────────────────────────────────────────────────────┘
```

## Technical Requirements

### 1. JWT Token Management

**Algorithm**: RS256 (RSA Signature with SHA-256)

**Token Structure**:
```json
{
  "header": {
    "alg": "RS256",
    "typ": "JWT",
    "kid": "key-id-1"
  },
  "payload": {
    "sub": "user-uuid",
    "email": "user@example.com",
    "roles": ["buyer"],
    "permissions": ["calls:read", "bids:create"],
    "iat": 1704067200,
    "exp": 1704070800,
    "jti": "token-uuid"
  }
}
```

**Token Lifecycle**:
- Access Token: 15 minutes
- Refresh Token: 7 days
- API Keys: No expiration (revocable)

### 2. Role-Based Access Control (RBAC)

**Core Roles**:

| Role | Description | Default Permissions |
|------|-------------|-------------------|
| admin | System administrators | All permissions |
| buyer | Call buyers | calls:read, bids:*, analytics:read |
| seller | Call sellers | calls:*, campaigns:*, analytics:read |
| viewer | Read-only access | *:read |

**Permission Format**: `resource:action`

**Resources**: 
- calls, bids, campaigns, analytics, users, billing, settings

**Actions**: 
- create, read, update, delete, execute

### 3. API Key Management

**Key Features**:
- Cryptographically secure key generation (32 bytes)
- Scoped permissions per key
- Rate limiting per key
- Usage tracking and analytics
- Key rotation support

**Key Format**: `dce_live_sk_[32-character-random-string]`

### 4. Session Management

**Session Storage**: Redis with encryption

**Session Data**:
```json
{
  "session_id": "uuid",
  "user_id": "uuid",
  "created_at": "2025-01-06T10:00:00Z",
  "last_accessed": "2025-01-06T10:30:00Z",
  "ip_address": "192.168.1.1",
  "user_agent": "Mozilla/5.0...",
  "device_fingerprint": "hash"
}
```

### 5. Rate Limiting

**Limits by Authentication Type**:

| Type | Requests/Second | Burst | Daily Limit |
|------|----------------|-------|-------------|
| Unauthenticated | 10 | 20 | 1,000 |
| Authenticated User | 100 | 200 | 100,000 |
| API Key (Standard) | 500 | 1,000 | 1,000,000 |
| API Key (Premium) | 2,000 | 5,000 | 10,000,000 |

## Domain Model

### Core Entities

```go
// User represents an authenticated user in the system
type User struct {
    ID             uuid.UUID      `json:"id"`
    Email          string         `json:"email"`
    PasswordHash   string         `json:"-"`
    FirstName      string         `json:"first_name"`
    LastName       string         `json:"last_name"`
    Status         UserStatus     `json:"status"`
    EmailVerified  bool           `json:"email_verified"`
    TwoFactorEnabled bool         `json:"two_factor_enabled"`
    CreatedAt      time.Time      `json:"created_at"`
    UpdatedAt      time.Time      `json:"updated_at"`
    LastLoginAt    *time.Time     `json:"last_login_at"`
    Roles          []Role         `json:"roles"`
    Metadata       map[string]any `json:"metadata"`
}

// Role represents a collection of permissions
type Role struct {
    ID          uuid.UUID    `json:"id"`
    Name        string       `json:"name"`
    Description string       `json:"description"`
    Permissions []Permission `json:"permissions"`
    IsSystem    bool         `json:"is_system"`
    CreatedAt   time.Time    `json:"created_at"`
}

// Permission represents a specific action on a resource
type Permission struct {
    ID          uuid.UUID `json:"id"`
    Resource    string    `json:"resource"`
    Action      string    `json:"action"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
}

// APIKey represents a programmatic access key
type APIKey struct {
    ID           uuid.UUID      `json:"id"`
    UserID       uuid.UUID      `json:"user_id"`
    Name         string         `json:"name"`
    KeyHash      string         `json:"-"`
    KeyPrefix    string         `json:"key_prefix"` // First 8 chars for identification
    Permissions  []string       `json:"permissions"`
    RateLimit    *RateLimit     `json:"rate_limit"`
    LastUsedAt   *time.Time     `json:"last_used_at"`
    ExpiresAt    *time.Time     `json:"expires_at"`
    Status       APIKeyStatus   `json:"status"`
    CreatedAt    time.Time      `json:"created_at"`
    Metadata     map[string]any `json:"metadata"`
}

// Session represents an active user session
type Session struct {
    ID              uuid.UUID `json:"id"`
    UserID          uuid.UUID `json:"user_id"`
    RefreshToken    string    `json:"-"`
    IPAddress       string    `json:"ip_address"`
    UserAgent       string    `json:"user_agent"`
    DeviceFingerprint string  `json:"device_fingerprint"`
    LastAccessedAt  time.Time `json:"last_accessed_at"`
    ExpiresAt       time.Time `json:"expires_at"`
    CreatedAt       time.Time `json:"created_at"`
}

// Supporting Types
type UserStatus string
const (
    UserStatusActive    UserStatus = "active"
    UserStatusInactive  UserStatus = "inactive"
    UserStatusSuspended UserStatus = "suspended"
)

type APIKeyStatus string
const (
    APIKeyStatusActive   APIKeyStatus = "active"
    APIKeyStatusRevoked  APIKeyStatus = "revoked"
    APIKeyStatusExpired  APIKeyStatus = "expired"
)

type RateLimit struct {
    RequestsPerSecond int `json:"requests_per_second"`
    BurstSize        int `json:"burst_size"`
    DailyLimit       int `json:"daily_limit"`
}
```

## Service Layer Architecture

### 1. AuthenticationService

```go
type AuthenticationService interface {
    // User authentication
    Login(ctx context.Context, email, password string) (*LoginResponse, error)
    Logout(ctx context.Context, sessionID uuid.UUID) error
    RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
    
    // Password management
    ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error
    ResetPassword(ctx context.Context, token, newPassword string) error
    RequestPasswordReset(ctx context.Context, email string) error
    
    // Session management
    GetSession(ctx context.Context, sessionID uuid.UUID) (*Session, error)
    InvalidateAllSessions(ctx context.Context, userID uuid.UUID) error
    
    // Two-factor authentication
    EnableTwoFactor(ctx context.Context, userID uuid.UUID) (*TwoFactorSetup, error)
    VerifyTwoFactor(ctx context.Context, userID uuid.UUID, code string) error
    DisableTwoFactor(ctx context.Context, userID uuid.UUID, password string) error
}
```

### 2. AuthorizationService

```go
type AuthorizationService interface {
    // Permission checking
    HasPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error)
    HasAllPermissions(ctx context.Context, userID uuid.UUID, permissions []string) (bool, error)
    HasAnyPermission(ctx context.Context, userID uuid.UUID, permissions []string) (bool, error)
    
    // Role management
    AssignRole(ctx context.Context, userID, roleID uuid.UUID) error
    RemoveRole(ctx context.Context, userID, roleID uuid.UUID) error
    GetUserRoles(ctx context.Context, userID uuid.UUID) ([]Role, error)
    
    // Resource-based authorization
    CanAccessResource(ctx context.Context, userID uuid.UUID, resourceType string, resourceID uuid.UUID) (bool, error)
    GetAccessibleResources(ctx context.Context, userID uuid.UUID, resourceType string) ([]uuid.UUID, error)
}
```

### 3. TokenService

```go
type TokenService interface {
    // JWT operations
    GenerateAccessToken(user *User) (string, error)
    GenerateRefreshToken(sessionID uuid.UUID) (string, error)
    ValidateAccessToken(token string) (*TokenClaims, error)
    ValidateRefreshToken(token string) (*uuid.UUID, error)
    RevokeToken(jti string) error
    
    // Key management
    RotateSigningKey() error
    GetPublicKeys() ([]PublicKey, error)
}
```

### 4. APIKeyService

```go
type APIKeyService interface {
    // Key management
    CreateAPIKey(ctx context.Context, userID uuid.UUID, name string, permissions []string) (*APIKey, string, error)
    GetAPIKey(ctx context.Context, keyID uuid.UUID) (*APIKey, error)
    ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]APIKey, error)
    RevokeAPIKey(ctx context.Context, keyID uuid.UUID) error
    
    // Key validation
    ValidateAPIKey(ctx context.Context, key string) (*APIKey, error)
    UpdateLastUsed(ctx context.Context, keyID uuid.UUID) error
    
    // Rate limiting
    CheckRateLimit(ctx context.Context, keyID uuid.UUID) error
    GetUsageStats(ctx context.Context, keyID uuid.UUID, period time.Duration) (*UsageStats, error)
}
```

## API Endpoints

### Authentication Endpoints

```yaml
# User Authentication
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
POST   /api/v1/auth/refresh
GET    /api/v1/auth/me
POST   /api/v1/auth/verify-email
POST   /api/v1/auth/resend-verification

# Password Management
POST   /api/v1/auth/change-password
POST   /api/v1/auth/forgot-password
POST   /api/v1/auth/reset-password

# Two-Factor Authentication
POST   /api/v1/auth/2fa/enable
POST   /api/v1/auth/2fa/verify
POST   /api/v1/auth/2fa/disable
GET    /api/v1/auth/2fa/recovery-codes

# API Key Management
POST   /api/v1/api-keys
GET    /api/v1/api-keys
GET    /api/v1/api-keys/:id
DELETE /api/v1/api-keys/:id
POST   /api/v1/api-keys/:id/regenerate

# Session Management
GET    /api/v1/auth/sessions
DELETE /api/v1/auth/sessions/:id
DELETE /api/v1/auth/sessions
```

### Request/Response Examples

#### Login Request
```json
POST /api/v1/auth/login
{
  "email": "user@example.com",
  "password": "SecurePassword123!",
  "device_fingerprint": "unique-device-id"
}

Response:
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "refresh_token": "dce_refresh_...",
  "token_type": "Bearer",
  "expires_in": 900,
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "roles": ["buyer"],
    "permissions": ["calls:read", "bids:create"]
  }
}
```

#### Create API Key Request
```json
POST /api/v1/api-keys
{
  "name": "Production Integration",
  "permissions": ["calls:read", "bids:create", "bids:read"],
  "rate_limit": {
    "requests_per_second": 1000,
    "daily_limit": 5000000
  },
  "expires_in_days": 365
}

Response:
{
  "api_key": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "name": "Production Integration",
    "key_prefix": "dce_live_sk_a1b2c3d4",
    "permissions": ["calls:read", "bids:create", "bids:read"],
    "created_at": "2025-01-06T10:00:00Z"
  },
  "secret_key": "dce_live_sk_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0"
}
```

## Database Schema

### Users Table
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    email_verified BOOLEAN DEFAULT FALSE,
    two_factor_enabled BOOLEAN DEFAULT FALSE,
    two_factor_secret VARCHAR(255),
    metadata JSONB DEFAULT '{}',
    last_login_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_email CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    CONSTRAINT valid_status CHECK (status IN ('active', 'inactive', 'suspended'))
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status) WHERE status != 'active';
```

### Roles Table
```sql
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_role_name CHECK (name ~ '^[a-z_]+$')
);

CREATE INDEX idx_roles_name ON roles(name);
```

### Permissions Table
```sql
CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT unique_permission UNIQUE (resource, action),
    CONSTRAINT valid_resource CHECK (resource ~ '^[a-z_]+$'),
    CONSTRAINT valid_action CHECK (action IN ('create', 'read', 'update', 'delete', 'execute'))
);

CREATE INDEX idx_permissions_resource_action ON permissions(resource, action);
```

### User Roles Junction
```sql
CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    assigned_by UUID REFERENCES users(id),
    
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);
```

### Role Permissions Junction
```sql
CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    granted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX idx_role_permissions_role_id ON role_permissions(role_id);
```

### API Keys Table
```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(16) NOT NULL,
    permissions TEXT[] NOT NULL DEFAULT '{}',
    rate_limit JSONB,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_status CHECK (status IN ('active', 'revoked', 'expired')),
    CONSTRAINT unique_key_hash UNIQUE (key_hash)
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_status ON api_keys(status);
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;
```

### Sessions Table
```sql
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) NOT NULL,
    ip_address INET NOT NULL,
    user_agent TEXT,
    device_fingerprint VARCHAR(255),
    last_accessed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT unique_refresh_token UNIQUE (refresh_token_hash)
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_device ON sessions(user_id, device_fingerprint);
```

### Audit Log Table
```sql
CREATE TABLE auth_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_type VARCHAR(50) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    success BOOLEAN NOT NULL,
    failure_reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_event_type CHECK (event_type IN (
        'login', 'logout', 'password_change', 'password_reset',
        'token_refresh', 'api_key_created', 'api_key_used',
        'permission_denied', '2fa_enabled', '2fa_verified'
    ))
);

CREATE INDEX idx_auth_audit_user_id ON auth_audit_log(user_id);
CREATE INDEX idx_auth_audit_created_at ON auth_audit_log(created_at);
CREATE INDEX idx_auth_audit_event_type ON auth_audit_log(event_type);
```

## Implementation Plan

### Phase 1: Apply Existing Middleware (Day 1)
**Objective**: Immediate security fix by applying auth middleware to all protected routes

**Tasks**:
1. ✅ Audit all API endpoints and classify protection requirements
2. ✅ Apply authentication middleware to protected route groups
3. ✅ Implement basic JWT validation (hardcoded secret for now)
4. ✅ Add integration tests to verify protection
5. ✅ Deploy hotfix to production

**Deliverables**:
- All sensitive endpoints require authentication
- Basic JWT validation working
- No regression in existing functionality

### Phase 2: JWT Token Service (Day 2)
**Objective**: Implement proper JWT token management with RS256

**Tasks**:
1. ⬜ Generate RSA key pairs for token signing
2. ⬜ Implement TokenService with proper JWT creation/validation
3. ⬜ Add token expiration and refresh logic
4. ⬜ Implement token revocation with Redis blacklist
5. ⬜ Add comprehensive unit tests

**Deliverables**:
- Secure JWT implementation with RS256
- Token refresh mechanism
- Token revocation capability

### Phase 3: RBAC Implementation (Day 3)
**Objective**: Add role-based access control with granular permissions

**Tasks**:
1. ⬜ Create database schema for users, roles, permissions
2. ⬜ Implement AuthorizationService
3. ⬜ Create permission checking middleware
4. ⬜ Add role management endpoints
5. ⬜ Seed default roles and permissions

**Deliverables**:
- Complete RBAC system
- Permission-based route protection
- Role management API

### Phase 4: API Key Management (Day 4)
**Objective**: Enable programmatic access with API keys

**Tasks**:
1. ⬜ Implement API key generation and storage
2. ⬜ Add API key authentication middleware
3. ⬜ Implement per-key rate limiting
4. ⬜ Add usage tracking and analytics
5. ⬜ Create API key management endpoints

**Deliverables**:
- API key authentication
- Key management interface
- Usage analytics

## Testing Strategy

### Unit Tests

```go
// Example: TokenService Tests
func TestTokenService_GenerateAccessToken(t *testing.T) {
    tests := []struct {
        name    string
        user    *User
        wantErr bool
    }{
        {
            name: "valid user with roles",
            user: &User{
                ID:    uuid.New(),
                Email: "test@example.com",
                Roles: []Role{{Name: "buyer"}},
            },
            wantErr: false,
        },
        {
            name:    "nil user",
            user:    nil,
            wantErr: true,
        },
    }
    // Test implementation...
}
```

### Integration Tests

```go
// Example: Auth Flow Integration Test
func TestAuthenticationFlow(t *testing.T) {
    // Setup test server and database
    server := setupTestServer(t)
    
    // 1. Login
    loginResp := server.POST("/api/v1/auth/login").
        WithJSON(LoginRequest{
            Email:    "test@example.com",
            Password: "SecurePass123!",
        }).
        Expect(t).
        Status(http.StatusOK)
    
    token := loginResp.JSON().Get("access_token").String()
    
    // 2. Access protected endpoint
    server.GET("/api/v1/calls").
        WithHeader("Authorization", "Bearer "+token).
        Expect(t).
        Status(http.StatusOK)
    
    // 3. Refresh token
    refreshToken := loginResp.JSON().Get("refresh_token").String()
    server.POST("/api/v1/auth/refresh").
        WithJSON(RefreshRequest{Token: refreshToken}).
        Expect(t).
        Status(http.StatusOK)
}
```

### Security Tests

```go
// Example: Token Validation Security Tests
func TestTokenValidation_Security(t *testing.T) {
    tests := []struct {
        name        string
        token       string
        expectError string
    }{
        {
            name:        "expired token",
            token:       generateExpiredToken(),
            expectError: "token is expired",
        },
        {
            name:        "wrong signature",
            token:       generateTokenWithWrongSignature(),
            expectError: "signature is invalid",
        },
        {
            name:        "none algorithm",
            token:       generateTokenWithNoneAlgorithm(),
            expectError: "none algorithm not allowed",
        },
    }
    // Test implementation...
}
```

### Performance Tests

```go
// Benchmark auth middleware overhead
func BenchmarkAuthMiddleware(b *testing.B) {
    handler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    
    token := generateValidToken()
    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        w := httptest.NewRecorder()
        handler.ServeHTTP(w, req)
    }
}

// Target: < 0.1ms overhead per request
```

## Security Considerations

### 1. Token Security
- Use RS256 for token signing (asymmetric)
- Rotate signing keys every 90 days
- Store private keys in secure key management system
- Implement token binding to prevent token theft

### 2. Password Security
- Bcrypt with cost factor 12 (adaptive)
- Enforce password complexity requirements
- Implement password history (last 5)
- Rate limit password attempts

### 3. Session Security
- Secure, HttpOnly, SameSite cookies
- Session fixation protection
- Idle timeout (30 minutes)
- Absolute timeout (24 hours)

### 4. API Key Security
- Generate using cryptographically secure random
- Hash keys before storage (SHA-256)
- Implement key rotation reminders
- Log all key usage for audit

### 5. Rate Limiting & DDoS Protection
```go
// Per-user rate limiting
type RateLimiter struct {
    store        redis.Client
    keyPrefix    string
    window       time.Duration
    maxRequests  int
}

func (rl *RateLimiter) Allow(userID string) (bool, error) {
    key := fmt.Sprintf("%s:%s:%d", rl.keyPrefix, userID, time.Now().Unix()/int64(rl.window.Seconds()))
    
    count, err := rl.store.Incr(key)
    if err != nil {
        return false, err
    }
    
    if count == 1 {
        rl.store.Expire(key, rl.window)
    }
    
    return count <= rl.maxRequests, nil
}
```

### 6. Audit Logging
- Log all authentication events
- Log all authorization failures
- Log all privilege escalations
- Retain logs for 90 days minimum

## Monitoring & Alerting

### Key Metrics

```prometheus
# Authentication metrics
auth_login_attempts_total{status="success|failure",reason=""}
auth_token_validations_total{type="access|refresh",status="valid|invalid"}
auth_api_key_usage_total{key_id="",endpoint=""}

# Authorization metrics
auth_permission_checks_total{permission="",granted="true|false"}
auth_rate_limit_exceeded_total{user_id="",limit_type=""}

# Performance metrics
auth_token_validation_duration_seconds
auth_middleware_overhead_seconds
```

### Critical Alerts

1. **Brute Force Detection**
   - Alert: > 10 failed login attempts from same IP in 5 minutes
   - Action: Automatic IP blocking

2. **Privilege Escalation**
   - Alert: User assigned admin role
   - Action: Manual review required

3. **Mass Token Failures**
   - Alert: > 100 token validation failures in 1 minute
   - Action: Check for key rotation issues

4. **API Key Abuse**
   - Alert: API key exceeding rate limit by 200%
   - Action: Automatic key suspension

## Migration Strategy

### Existing System Compatibility

1. **Gradual Rollout**
   - Phase 1: New endpoints require auth
   - Phase 2: Deprecation warnings on old endpoints
   - Phase 3: Mandatory auth on all endpoints

2. **Backward Compatibility**
   - Support both Bearer tokens and API keys
   - Grace period for old authentication methods
   - Clear migration documentation

3. **Data Migration**
   - Script to create user accounts from existing data
   - Default roles based on account type
   - Temporary passwords with forced reset

## Documentation Requirements

### API Documentation
- OpenAPI specification updates
- Authentication flow diagrams
- Code examples in multiple languages
- Postman collection with auth examples

### Developer Guide
- How to obtain tokens
- How to use API keys
- Permission model explanation
- Rate limit guidelines
- Error handling examples

### Operations Guide
- Key rotation procedures
- User management workflows
- Security incident response
- Monitoring setup

## Success Metrics

### Security Metrics
- 0% unauthorized access to protected endpoints
- < 0.1% false positive rate on auth checks
- 100% of sensitive operations logged

### Performance Metrics
- < 1ms auth middleware overhead (p99)
- < 10ms token generation time
- < 5ms permission check time

### Adoption Metrics
- 100% of APIs protected within 1 week
- 80% of users migrated within 1 month
- 90% of traffic using new auth within 2 months

## Conclusion

This authentication and authorization system specification addresses the critical security vulnerability of unprotected APIs while providing a robust, scalable solution for the DCE platform. The phased implementation approach ensures immediate security improvements while building toward a comprehensive auth system.

The 3-4 day implementation timeline is aggressive but achievable with focused effort. Phase 1 must be completed immediately to close the security gap, with subsequent phases building the full-featured authentication system.

Key success factors:
1. Immediate application of auth middleware (Phase 1)
2. Proper JWT implementation with RS256
3. Granular RBAC for flexible access control
4. API key support for programmatic access
5. Comprehensive testing and monitoring

This system will provide DCE with enterprise-grade authentication and authorization capabilities while maintaining the sub-millisecond performance requirements critical to the platform's success.