# Webhook Management System Specification

## Executive Summary

### Problem Statement
The Dependable Call Exchange Backend currently lacks webhook infrastructure, as identified in the API audit. This critical gap prevents seamless integration with partners who need real-time notifications for call events, bid updates, and compliance alerts.

### Business Impact
- **Partner Integration Blocked**: Cannot notify partners of critical events in real-time
- **Manual Polling Required**: Partners must constantly poll APIs, increasing load and latency
- **Competitive Disadvantage**: Modern pay-per-call platforms require webhook capabilities
- **Revenue Impact**: Cannot onboard enterprise partners requiring event-driven integrations

### Proposed Solution
Implement an enterprise-grade webhook platform with:
- Reliable event delivery with automatic retries
- Security features including HMAC signatures and SSL enforcement
- Comprehensive monitoring and analytics
- Flexible event filtering and routing

## Webhook Architecture

### Event-Driven Delivery System

```
┌─────────────────────────────────────────────────────────────┐
│                     Event Sources                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │   Call   │  │   Bid    │  │Compliance│  │Financial │  │
│  │  Events  │  │  Events  │  │  Events  │  │  Events  │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  │
└───────┼──────────────┼──────────────┼──────────────┼───────┘
        │              │              │              │
        └──────────────┴──────────────┴──────────────┘
                              │
                    ┌─────────▼─────────┐
                    │  Event Dispatcher │
                    │  (Kafka/Redis)    │
                    └─────────┬─────────┘
                              │
                    ┌─────────▼─────────┐
                    │  Webhook Service  │
                    ├───────────────────┤
                    │ • Event Filtering │
                    │ • Queue Management│
                    │ • Retry Logic    │
                    │ • Rate Limiting  │
                    └─────────┬─────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼─────┐     ┌─────────▼─────┐     ┌────────▼──────┐
│  Partner A  │     │   Partner B   │     │   Partner C   │
│  Endpoint   │     │   Endpoint    │     │   Endpoint    │
└─────────────┘     └───────────────┘     └───────────────┘
```

### Delivery Components

1. **Event Publisher**
   - Publishes domain events to message queue
   - Ensures event ordering per aggregate
   - Includes event metadata and correlation IDs

2. **Webhook Dispatcher**
   - Consumes events from queue
   - Matches events to webhook subscriptions
   - Manages delivery queues per endpoint

3. **Delivery Worker Pool**
   - Concurrent HTTP delivery workers
   - Connection pooling and keep-alive
   - Timeout management (30s default)

4. **Retry Manager**
   - Exponential backoff: 1s, 2s, 4s, 8s, 16s, 32s, 64s
   - Max 7 retry attempts over 24 hours
   - Dead letter queue after max retries

## Domain Model

```go
package webhook

import (
    "time"
    "github.com/google/uuid"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Webhook represents a webhook subscription
type Webhook struct {
    ID              uuid.UUID
    AccountID       uuid.UUID
    URL             string
    Events          []EventType
    Active          bool
    Secret          string // For HMAC signing
    Config          WebhookConfig
    CreatedAt       time.Time
    UpdatedAt       time.Time
    LastDeliveredAt *time.Time
    Stats           DeliveryStats
}

// EventType represents types of events that can trigger webhooks
type EventType string

const (
    // Call Events
    EventCallCreated    EventType = "call.created"
    EventCallAnswered   EventType = "call.answered"
    EventCallCompleted  EventType = "call.completed"
    EventCallFailed     EventType = "call.failed"
    
    // Bid Events
    EventBidPlaced      EventType = "bid.placed"
    EventBidWon         EventType = "bid.won"
    EventBidLost        EventType = "bid.lost"
    EventBidExpired     EventType = "bid.expired"
    
    // Compliance Events
    EventComplianceViolation EventType = "compliance.violation"
    EventDNCListUpdated      EventType = "compliance.dnc_updated"
    EventConsentExpired      EventType = "compliance.consent_expired"
    
    // Financial Events
    EventPaymentProcessed    EventType = "payment.processed"
    EventPaymentFailed       EventType = "payment.failed"
    EventInvoiceGenerated    EventType = "invoice.generated"
)

// WebhookEvent represents an event to be delivered
type WebhookEvent struct {
    ID           uuid.UUID
    WebhookID    uuid.UUID
    EventType    EventType
    EventID      string // Original event ID
    Payload      json.RawMessage
    CreatedAt    time.Time
    ScheduledAt  time.Time
    Status       DeliveryStatus
    Attempts     []DeliveryAttempt
}

// DeliveryAttempt represents a single delivery attempt
type DeliveryAttempt struct {
    ID            uuid.UUID
    WebhookEventID uuid.UUID
    AttemptNumber  int
    RequestHeaders map[string]string
    RequestBody    []byte
    ResponseStatus int
    ResponseBody   []byte
    ResponseHeaders map[string][]string
    Duration       time.Duration
    Error          *string
    AttemptedAt    time.Time
}

// WebhookConfig contains webhook configuration
type WebhookConfig struct {
    // Retry Configuration
    MaxRetries      int           `json:"max_retries"`      // Default: 7
    RetryBackoff    time.Duration `json:"retry_backoff"`    // Default: exponential
    TimeoutSeconds  int           `json:"timeout_seconds"`  // Default: 30
    
    // Security Configuration
    IPAllowlist     []string      `json:"ip_allowlist"`     // Optional IP restrictions
    RequireHTTPS    bool          `json:"require_https"`    // Default: true
    VerifySSL       bool          `json:"verify_ssl"`       // Default: true
    
    // Delivery Configuration
    CustomHeaders   map[string]string `json:"custom_headers"`
    BasicAuthUser   string           `json:"basic_auth_user"`
    BasicAuthPass   string           `json:"basic_auth_pass"`
    
    // Rate Limiting
    RateLimit       int           `json:"rate_limit"`       // Events per minute
    BurstLimit      int           `json:"burst_limit"`      // Max burst size
}

// DeliveryStatus represents the status of event delivery
type DeliveryStatus string

const (
    StatusPending    DeliveryStatus = "pending"
    StatusDelivering DeliveryStatus = "delivering"
    StatusDelivered  DeliveryStatus = "delivered"
    StatusFailed     DeliveryStatus = "failed"
    StatusExpired    DeliveryStatus = "expired"
)

// DeliveryStats tracks webhook performance
type DeliveryStats struct {
    TotalEvents      int64
    SuccessfulEvents int64
    FailedEvents     int64
    AvgLatencyMs     float64
    LastSuccess      *time.Time
    LastFailure      *time.Time
}
```

## Delivery Guarantees

### At-Least-Once Delivery
- Events are persisted before delivery attempt
- Acknowledgment required for successful delivery
- Failed deliveries are retried according to policy
- Events may be delivered multiple times

### Duplicate Detection
- Include unique `X-Event-ID` header in all requests
- Partners should implement idempotency using this ID
- Event IDs are globally unique UUIDs
- Retain processed IDs for at least 7 days

### Ordered Delivery
- Events are delivered in order per webhook endpoint
- Failed events block subsequent events for that endpoint
- Different endpoints process independently
- Option to disable ordering for higher throughput

### Dead Letter Queue
- Events that fail all retry attempts go to DLQ
- Manual intervention required for DLQ events
- DLQ events retained for 30 days
- API to requeue DLQ events

## Management Features

### Webhook Registration API

```http
POST /api/v1/webhooks
{
  "url": "https://partner.example.com/webhooks",
  "events": ["call.completed", "bid.won"],
  "config": {
    "max_retries": 5,
    "timeout_seconds": 30,
    "require_https": true,
    "custom_headers": {
      "X-Partner-ID": "12345"
    }
  }
}

Response:
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://partner.example.com/webhooks",
  "events": ["call.completed", "bid.won"],
  "secret": "whsec_8fj3k4l5m6n7o8p9", // For HMAC signing
  "active": true,
  "created_at": "2025-01-15T10:30:00Z"
}
```

### Event Filtering and Routing
- Subscribe to specific event types
- Filter by additional criteria (e.g., geography, buyer/seller)
- Transform payloads with JSONPath expressions
- Route to different endpoints based on event data

### Delivery Status Tracking

```http
GET /api/v1/webhooks/{id}/deliveries?status=failed&limit=100

Response:
{
  "deliveries": [
    {
      "event_id": "evt_123",
      "event_type": "call.completed",
      "status": "failed",
      "attempts": 7,
      "last_attempt": {
        "attempted_at": "2025-01-15T10:30:00Z",
        "response_status": 500,
        "error": "Internal Server Error"
      }
    }
  ],
  "pagination": {
    "total": 42,
    "page": 1,
    "per_page": 100
  }
}
```

### Analytics and Monitoring

**Metrics Exposed**:
- Delivery success rate by endpoint
- Average delivery latency
- Retry distribution
- Event volume by type
- Error rate by response code

**Dashboards**:
- Real-time delivery status
- Partner health scores
- Event throughput graphs
- Error rate trends

## Security Features

### HMAC Signature Validation

Every webhook request includes signature headers:

```http
POST /webhooks
X-DCE-Signature: sha256=8fj3k4l5m6n7o8p9q0r1s2t3u4v5w6x7
X-DCE-Timestamp: 1705315800
X-DCE-Event-ID: evt_550e8400-e29b-41d4-a716-446655440000

{
  "event_type": "call.completed",
  "data": { ... }
}
```

Signature calculation:
```go
func generateSignature(secret, timestamp, body string) string {
    message := fmt.Sprintf("%s.%s", timestamp, body)
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(message))
    return hex.EncodeToString(mac.Sum(nil))
}
```

### IP Allowlisting
- Optional IP restriction per webhook
- Support for CIDR notation
- Bypass for development/testing
- Automatic IP detection from X-Forwarded-For

### SSL/TLS Enforcement
- HTTPS required by default
- TLS 1.2 minimum
- Certificate validation
- Option to disable for testing only

### Rate Limiting
- Per-endpoint rate limits
- Token bucket algorithm
- Configurable limits and burst
- Graceful degradation when limited

## Implementation Plan

### Phase 1: Core Infrastructure (1.5 days)
- Event publishing framework
- Webhook domain model
- Database schema and migrations
- Basic CRUD operations

### Phase 2: Delivery Engine (1.5 days)
- HTTP delivery worker pool
- Retry logic implementation
- Queue management
- Error handling

### Phase 3: Security Features (1 day)
- HMAC signature generation
- SSL/TLS validation
- IP allowlisting
- Rate limiting

### Phase 4: Management API (1 day)
- Registration endpoints
- Status tracking API
- Analytics endpoints
- Admin dashboard

### Total Effort: 4-5 developer days

## Testing Strategy

### Unit Tests
- Domain model validation
- Signature generation/validation
- Retry logic scenarios
- Rate limiting algorithms

### Integration Tests
- End-to-end delivery flow
- Retry behavior validation
- Concurrent delivery handling
- Error scenario coverage

### Performance Tests
- 10,000+ webhooks/second throughput
- < 100ms delivery latency (p99)
- Graceful degradation under load
- Memory efficiency validation

## Monitoring and Alerting

### Key Metrics
- `webhook_deliveries_total{status, endpoint}`
- `webhook_delivery_duration_seconds{endpoint}`
- `webhook_retry_count{attempt, endpoint}`
- `webhook_queue_depth{priority}`

### Critical Alerts
- Delivery success rate < 95%
- Queue depth > 10,000 events
- Any endpoint failing for > 1 hour
- Certificate expiration warnings

## Migration Path

1. **Beta Program**: Select partners for initial rollout
2. **Parallel Delivery**: Deliver via webhooks and existing polling
3. **Gradual Migration**: Move partners to webhooks incrementally
4. **Deprecation**: Phase out polling after full migration

## Success Criteria

- **Reliability**: 99.9% successful delivery rate
- **Performance**: < 100ms delivery latency (p99)
- **Scale**: Support 1000+ webhook endpoints
- **Security**: Zero security incidents
- **Adoption**: 80% of partners using webhooks within 6 months