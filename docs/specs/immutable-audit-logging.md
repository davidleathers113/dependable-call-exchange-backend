# Immutable Audit Logging System - Technical Specification

**Feature ID**: COMPLIANCE-003  
**Risk Score**: 88/100  
**Priority**: Critical  
**Compliance Requirements**: TCPA, GDPR, CCPA, SOX  

## Table of Contents

1. [Feature Overview](#feature-overview)
2. [Technical Architecture](#technical-architecture)
3. [Audit Event Design](#audit-event-design)
4. [Infrastructure Design](#infrastructure-design)
5. [Service Implementation](#service-implementation)
6. [Security Measures](#security-measures)
7. [Performance Optimization](#performance-optimization)
8. [Implementation Plan](#implementation-plan)

## Feature Overview

### Requirements

The immutable audit logging system provides a tamper-proof, cryptographically secured audit trail for all compliance-relevant events in the Dependable Call Exchange platform. This system is critical for:

- **Regulatory Compliance**: Prove adherence to TCPA, GDPR, CCPA requirements
- **Legal Protection**: Maintain defensible records for litigation
- **Security Forensics**: Investigate incidents and breaches
- **Business Intelligence**: Analyze patterns and trends

### Key Features

1. **Tamper-Proof Storage**: Cryptographic hash chaining prevents modification
2. **Complete Audit Trail**: Captures all compliance-relevant events
3. **7-Year Retention**: Automated archival with lifecycle management
4. **High Performance**: < 5ms write latency, sub-second query response
5. **Regulatory Export**: Standard formats for auditor access

### Compliance Event Categories

```
1. Consent Management
   - Consent granted/revoked
   - Preference updates
   - Opt-out requests
   
2. Data Access
   - PII access logs
   - Export requests
   - Deletion requests
   
3. Call Events
   - Call initiation
   - Routing decisions
   - Recording consent
   
4. Configuration Changes
   - Compliance rules
   - System settings
   - User permissions
   
5. Security Events
   - Authentication
   - Authorization failures
   - Suspicious activities
```

## Technical Architecture

### System Design

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ Domain Event │  │   Service    │  │     API      │     │
│  │  Publishers  │  │   Loggers    │  │   Handlers   │     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │
└─────────┼──────────────────┼──────────────────┼─────────────┘
          │                  │                  │
┌─────────▼──────────────────▼──────────────────▼─────────────┐
│                    Audit Service Layer                       │
│  ┌────────────────────────────────────────────────────┐    │
│  │            AuditLogger (Write Path)                │    │
│  │  - Event validation                                │    │
│  │  - Hash computation                                │    │
│  │  - Async buffering                                 │    │
│  └────────────────┬───────────────────────────────────┘    │
│  ┌────────────────▼───────────────────────────────────┐    │
│  │            AuditQuery (Read Path)                  │    │
│  │  - Filter builder                                  │    │
│  │  - Pagination                                      │    │
│  │  - Export formats                                  │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
          │
┌─────────▼────────────────────────────────────────────────────┐
│                    Storage Layer                             │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐       │
│  │  PostgreSQL │  │    Kafka    │  │      S3      │       │
│  │  (Hot Data) │  │  (Streaming) │  │ (Cold Archive)│       │
│  └─────────────┘  └─────────────┘  └──────────────┘       │
└──────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **Event Generation**: Domain events trigger audit log creation
2. **Hash Chain**: Each event includes hash of previous event
3. **Write Path**: Async write to PostgreSQL with buffering
4. **Stream Path**: Real-time stream to Kafka for processing
5. **Archive Path**: Daily batch to S3 with compression
6. **Query Path**: Read from appropriate storage tier

## Audit Event Design

### Standard Event Schema

```go
// Package audit provides immutable audit logging
package audit

import (
    "crypto/sha256"
    "encoding/json"
    "time"
    
    "github.com/google/uuid"
)

// EventType represents the category of audit event
type EventType string

const (
    // Consent events
    EventConsentGranted    EventType = "consent.granted"
    EventConsentRevoked    EventType = "consent.revoked"
    EventConsentUpdated    EventType = "consent.updated"
    EventOptOutRequested   EventType = "consent.opt_out"
    
    // Data access events
    EventDataAccessed      EventType = "data.accessed"
    EventDataExported      EventType = "data.exported"
    EventDataDeleted       EventType = "data.deleted"
    EventDataModified      EventType = "data.modified"
    
    // Call events
    EventCallInitiated     EventType = "call.initiated"
    EventCallRouted        EventType = "call.routed"
    EventCallCompleted     EventType = "call.completed"
    EventCallFailed        EventType = "call.failed"
    EventRecordingStarted  EventType = "call.recording_started"
    
    // Configuration events
    EventConfigChanged     EventType = "config.changed"
    EventRuleUpdated       EventType = "config.rule_updated"
    EventPermissionChanged EventType = "config.permission_changed"
    
    // Security events
    EventAuthSuccess       EventType = "security.auth_success"
    EventAuthFailure       EventType = "security.auth_failure"
    EventAccessDenied      EventType = "security.access_denied"
    EventAnomalyDetected   EventType = "security.anomaly_detected"
)

// Severity levels for audit events
type Severity string

const (
    SeverityInfo     Severity = "INFO"
    SeverityWarning  Severity = "WARNING"
    SeverityError    Severity = "ERROR"
    SeverityCritical Severity = "CRITICAL"
)

// Event represents an immutable audit log entry
type Event struct {
    // Immutable fields (set once, never modified)
    ID            uuid.UUID              `json:"id"`
    SequenceNum   int64                  `json:"sequence_num"`
    Timestamp     time.Time              `json:"timestamp"`
    TimestampNano int64                  `json:"timestamp_nano"`
    
    // Event classification
    Type          EventType              `json:"type"`
    Severity      Severity               `json:"severity"`
    Category      string                 `json:"category"`
    
    // Actor information
    ActorID       string                 `json:"actor_id"`
    ActorType     string                 `json:"actor_type"` // user, system, api
    ActorIP       string                 `json:"actor_ip"`
    ActorAgent    string                 `json:"actor_agent"`
    
    // Target information
    TargetID      string                 `json:"target_id"`
    TargetType    string                 `json:"target_type"`
    TargetOwner   string                 `json:"target_owner,omitempty"`
    
    // Event details
    Action        string                 `json:"action"`
    Result        string                 `json:"result"` // success, failure, partial
    ErrorCode     string                 `json:"error_code,omitempty"`
    ErrorMessage  string                 `json:"error_message,omitempty"`
    
    // Contextual data
    RequestID     string                 `json:"request_id"`
    SessionID     string                 `json:"session_id,omitempty"`
    CorrelationID string                 `json:"correlation_id,omitempty"`
    Environment   string                 `json:"environment"`
    Service       string                 `json:"service"`
    Version       string                 `json:"version"`
    
    // Compliance metadata
    ComplianceFlags map[string]bool      `json:"compliance_flags,omitempty"`
    DataClasses     []string             `json:"data_classes,omitempty"`
    LegalBasis      string               `json:"legal_basis,omitempty"`
    RetentionDays   int                  `json:"retention_days"`
    
    // Additional context
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
    Tags          []string               `json:"tags,omitempty"`
    
    // Cryptographic integrity
    PreviousHash  string                 `json:"previous_hash"`
    EventHash     string                 `json:"event_hash"`
    Signature     string                 `json:"signature,omitempty"`
}

// ComputeHash calculates the SHA-256 hash of the event
func (e *Event) ComputeHash(previousHash string) (string, error) {
    e.PreviousHash = previousHash
    
    // Create deterministic JSON representation
    data := map[string]interface{}{
        "id":             e.ID.String(),
        "sequence_num":   e.SequenceNum,
        "timestamp_nano": e.TimestampNano,
        "type":           e.Type,
        "actor_id":       e.ActorID,
        "target_id":      e.TargetID,
        "action":         e.Action,
        "result":         e.Result,
        "previous_hash":  e.PreviousHash,
    }
    
    jsonBytes, err := json.Marshal(data)
    if err != nil {
        return "", err
    }
    
    hash := sha256.Sum256(jsonBytes)
    e.EventHash = fmt.Sprintf("%x", hash)
    
    return e.EventHash, nil
}

// EventBuilder provides fluent interface for creating events
type EventBuilder struct {
    event *Event
}

// NewEvent creates a new event builder
func NewEvent(eventType EventType) *EventBuilder {
    return &EventBuilder{
        event: &Event{
            ID:            uuid.New(),
            Timestamp:     time.Now().UTC(),
            TimestampNano: time.Now().UnixNano(),
            Type:          eventType,
            Severity:      SeverityInfo,
            Environment:   getEnvironment(),
            Service:       getServiceName(),
            Version:       getServiceVersion(),
            RetentionDays: 2555, // 7 years default
            Metadata:      make(map[string]interface{}),
            ComplianceFlags: make(map[string]bool),
        },
    }
}

// WithActor sets actor information
func (b *EventBuilder) WithActor(id, actorType, ip, agent string) *EventBuilder {
    b.event.ActorID = id
    b.event.ActorType = actorType
    b.event.ActorIP = ip
    b.event.ActorAgent = agent
    return b
}

// WithTarget sets target information
func (b *EventBuilder) WithTarget(id, targetType, owner string) *EventBuilder {
    b.event.TargetID = id
    b.event.TargetType = targetType
    b.event.TargetOwner = owner
    return b
}

// WithResult sets action result
func (b *EventBuilder) WithResult(action, result string) *EventBuilder {
    b.event.Action = action
    b.event.Result = result
    return b
}

// WithError sets error information
func (b *EventBuilder) WithError(code, message string) *EventBuilder {
    b.event.ErrorCode = code
    b.event.ErrorMessage = message
    b.event.Result = "failure"
    if b.event.Severity < SeverityError {
        b.event.Severity = SeverityError
    }
    return b
}

// WithCompliance sets compliance metadata
func (b *EventBuilder) WithCompliance(flags map[string]bool, dataClasses []string, legalBasis string) *EventBuilder {
    b.event.ComplianceFlags = flags
    b.event.DataClasses = dataClasses
    b.event.LegalBasis = legalBasis
    return b
}

// Build returns the constructed event
func (b *EventBuilder) Build() *Event {
    return b.event
}
```

### Event Examples

```go
// Consent granted event
event := NewEvent(EventConsentGranted).
    WithActor(userID, "user", clientIP, userAgent).
    WithTarget(phoneNumber, "phone_number", userID).
    WithResult("grant_consent", "success").
    WithCompliance(
        map[string]bool{"tcpa_compliant": true},
        []string{"phone_number"},
        "explicit_consent",
    ).
    WithMetadata(map[string]interface{}{
        "consent_type": "calls",
        "duration_days": 365,
        "channel": "web_form",
    }).
    Build()

// Data access event
event := NewEvent(EventDataAccessed).
    WithActor(adminID, "admin", clientIP, userAgent).
    WithTarget(userID, "user_profile", userID).
    WithResult("view_pii", "success").
    WithCompliance(
        map[string]bool{"gdpr_compliant": true},
        []string{"name", "email", "phone"},
        "legitimate_interest",
    ).
    WithMetadata(map[string]interface{}{
        "access_reason": "support_ticket_12345",
        "fields_accessed": []string{"phone", "email"},
    }).
    Build()
```

## Infrastructure Design

### Database Schema

```sql
-- Audit database (separate from main application database)
CREATE DATABASE dce_audit_log
    WITH 
    ENCODING = 'UTF8'
    LC_COLLATE = 'en_US.UTF-8'
    LC_CTYPE = 'en_US.UTF-8'
    TEMPLATE = template0;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create audit schema
CREATE SCHEMA IF NOT EXISTS audit;

-- Audit events table (partitioned by month)
CREATE TABLE audit.events (
    -- Primary identifiers
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sequence_num BIGSERIAL NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    timestamp_nano BIGINT NOT NULL,
    
    -- Event classification
    type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    category VARCHAR(50) NOT NULL,
    
    -- Actor information
    actor_id VARCHAR(100) NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    actor_ip INET,
    actor_agent TEXT,
    
    -- Target information
    target_id VARCHAR(100) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_owner VARCHAR(100),
    
    -- Event details
    action VARCHAR(100) NOT NULL,
    result VARCHAR(50) NOT NULL,
    error_code VARCHAR(50),
    error_message TEXT,
    
    -- Context
    request_id VARCHAR(100),
    session_id VARCHAR(100),
    correlation_id VARCHAR(100),
    environment VARCHAR(50) NOT NULL,
    service VARCHAR(100) NOT NULL,
    version VARCHAR(50) NOT NULL,
    
    -- Compliance metadata
    compliance_flags JSONB,
    data_classes TEXT[],
    legal_basis VARCHAR(100),
    retention_days INTEGER NOT NULL DEFAULT 2555,
    
    -- Additional data
    metadata JSONB,
    tags TEXT[],
    
    -- Cryptographic integrity
    previous_hash VARCHAR(64) NOT NULL,
    event_hash VARCHAR(64) NOT NULL,
    signature TEXT,
    
    -- Constraints
    CONSTRAINT audit_events_sequence_unique UNIQUE (sequence_num),
    CONSTRAINT audit_events_hash_unique UNIQUE (event_hash)
) PARTITION BY RANGE (timestamp);

-- Create monthly partitions (example for 2025)
CREATE TABLE audit.events_2025_01 PARTITION OF audit.events
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE audit.events_2025_02 PARTITION OF audit.events
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- Continue for all months...

-- Indexes for query performance
CREATE INDEX idx_audit_events_timestamp ON audit.events (timestamp DESC);
CREATE INDEX idx_audit_events_type ON audit.events (type);
CREATE INDEX idx_audit_events_actor ON audit.events (actor_id, timestamp DESC);
CREATE INDEX idx_audit_events_target ON audit.events (target_id, timestamp DESC);
CREATE INDEX idx_audit_events_request ON audit.events (request_id);
CREATE INDEX idx_audit_events_compliance ON audit.events USING GIN (compliance_flags);
CREATE INDEX idx_audit_events_tags ON audit.events USING GIN (tags);

-- Hash chain verification table
CREATE TABLE audit.hash_chain (
    id SERIAL PRIMARY KEY,
    block_number BIGINT NOT NULL UNIQUE,
    start_sequence BIGINT NOT NULL,
    end_sequence BIGINT NOT NULL,
    block_hash VARCHAR(64) NOT NULL,
    merkle_root VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    verified_at TIMESTAMPTZ,
    verification_status VARCHAR(50)
);

-- Archive metadata table
CREATE TABLE audit.archives (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    partition_name VARCHAR(100) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    event_count BIGINT NOT NULL,
    s3_bucket VARCHAR(255) NOT NULL,
    s3_key VARCHAR(500) NOT NULL,
    archive_hash VARCHAR(64) NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    verified_at TIMESTAMPTZ,
    size_bytes BIGINT NOT NULL,
    compression_type VARCHAR(50) NOT NULL DEFAULT 'gzip'
);

-- Access control for audit logs
CREATE TABLE audit.access_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    accessed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    accessor_id VARCHAR(100) NOT NULL,
    accessor_type VARCHAR(50) NOT NULL,
    accessor_ip INET,
    query_type VARCHAR(50) NOT NULL, -- read, export, verify
    query_filters JSONB,
    events_accessed INTEGER,
    export_format VARCHAR(50),
    purpose TEXT NOT NULL,
    approved_by VARCHAR(100),
    approval_ticket VARCHAR(100)
);

-- Create read-only role for queries
CREATE ROLE audit_reader;
GRANT USAGE ON SCHEMA audit TO audit_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA audit TO audit_reader;

-- Create write-only role for logging
CREATE ROLE audit_writer;
GRANT USAGE ON SCHEMA audit TO audit_writer;
GRANT INSERT ON audit.events TO audit_writer;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA audit TO audit_writer;

-- Prevent updates and deletes (even by superuser in production)
CREATE OR REPLACE FUNCTION audit.prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Audit logs are immutable and cannot be modified or deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER prevent_update
    BEFORE UPDATE ON audit.events
    FOR EACH ROW EXECUTE FUNCTION audit.prevent_audit_modification();

CREATE TRIGGER prevent_delete
    BEFORE DELETE ON audit.events
    FOR EACH ROW EXECUTE FUNCTION audit.prevent_audit_modification();

-- Function to verify hash chain integrity
CREATE OR REPLACE FUNCTION audit.verify_hash_chain(
    start_sequence BIGINT,
    end_sequence BIGINT
) RETURNS TABLE (
    sequence_num BIGINT,
    is_valid BOOLEAN,
    expected_hash VARCHAR(64),
    actual_hash VARCHAR(64)
) AS $$
DECLARE
    prev_hash VARCHAR(64) := '';
    event_record RECORD;
BEGIN
    FOR event_record IN 
        SELECT * FROM audit.events 
        WHERE sequence_num BETWEEN start_sequence AND end_sequence
        ORDER BY sequence_num
    LOOP
        -- Verify hash calculation
        -- Implementation would recalculate hash and compare
        RETURN QUERY SELECT 
            event_record.sequence_num,
            event_record.previous_hash = prev_hash,
            prev_hash,
            event_record.previous_hash;
            
        prev_hash := event_record.event_hash;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Partition management function
CREATE OR REPLACE FUNCTION audit.create_monthly_partition(
    year INTEGER,
    month INTEGER
) RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := DATE(year || '-' || LPAD(month::TEXT, 2, '0') || '-01');
    end_date := start_date + INTERVAL '1 month';
    partition_name := 'events_' || year || '_' || LPAD(month::TEXT, 2, '0');
    
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS audit.%I PARTITION OF audit.events
         FOR VALUES FROM (%L) TO (%L)',
        partition_name, start_date, end_date
    );
END;
$$ LANGUAGE plpgsql;
```

### S3 Archive Structure

```
s3://dce-audit-logs/
├── events/
│   ├── 2025/
│   │   ├── 01/
│   │   │   ├── events_2025_01_01.parquet.gz
│   │   │   ├── events_2025_01_01.manifest
│   │   │   └── events_2025_01_01.sha256
│   │   ├── 02/
│   │   └── ...
│   └── ...
├── hash-chains/
│   ├── 2025/
│   │   ├── chain_2025_01.json
│   │   └── ...
└── compliance-exports/
    ├── gdpr/
    ├── tcpa/
    └── ccpa/
```

## Service Implementation

### Core Interfaces

```go
// Package auditlog provides immutable audit logging services
package auditlog

import (
    "context"
    "time"
    
    "github.com/davidleathers/dce/internal/domain/audit"
)

// Logger is the main interface for writing audit logs
type Logger interface {
    // Log writes an audit event
    Log(ctx context.Context, event *audit.Event) error
    
    // LogBatch writes multiple events atomically
    LogBatch(ctx context.Context, events []*audit.Event) error
    
    // Flush ensures all buffered events are written
    Flush(ctx context.Context) error
}

// Query provides read access to audit logs
type Query interface {
    // GetByID retrieves a single event
    GetByID(ctx context.Context, id uuid.UUID) (*audit.Event, error)
    
    // GetBySequence retrieves event by sequence number
    GetBySequence(ctx context.Context, seq int64) (*audit.Event, error)
    
    // Search queries events with filters
    Search(ctx context.Context, filter SearchFilter) (*SearchResult, error)
    
    // Export generates compliance report
    Export(ctx context.Context, filter ExportFilter) (io.Reader, error)
    
    // VerifyIntegrity checks hash chain
    VerifyIntegrity(ctx context.Context, startSeq, endSeq int64) (*IntegrityReport, error)
}

// SearchFilter defines query parameters
type SearchFilter struct {
    // Time range
    StartTime time.Time
    EndTime   time.Time
    
    // Event filters
    Types      []audit.EventType
    Severities []audit.Severity
    Categories []string
    
    // Actor/Target filters
    ActorIDs   []string
    TargetIDs  []string
    
    // Context filters
    RequestID     string
    CorrelationID string
    SessionID     string
    
    // Full-text search
    SearchText string
    
    // Compliance filters
    ComplianceFlags map[string]bool
    DataClasses     []string
    
    // Pagination
    Offset int
    Limit  int
    
    // Sorting
    OrderBy   string
    OrderDesc bool
}

// SearchResult contains paginated results
type SearchResult struct {
    Events      []*audit.Event
    TotalCount  int64
    HasMore     bool
    NextOffset  int
}

// ExportFilter defines export parameters
type ExportFilter struct {
    SearchFilter
    Format       ExportFormat
    IncludeHash  bool
    Compress     bool
}

// ExportFormat defines output format
type ExportFormat string

const (
    ExportJSON    ExportFormat = "json"
    ExportCSV     ExportFormat = "csv"
    ExportParquet ExportFormat = "parquet"
    ExportXML     ExportFormat = "xml"
)

// IntegrityReport contains verification results
type IntegrityReport struct {
    StartSequence   int64
    EndSequence     int64
    EventsChecked   int64
    ValidEvents     int64
    InvalidEvents   int64
    BrokenChains    []ChainBreak
    VerificationTime time.Duration
}

// ChainBreak represents a hash chain inconsistency
type ChainBreak struct {
    SequenceNum  int64
    ExpectedHash string
    ActualHash   string
    Event        *audit.Event
}
```

### Implementation Example

```go
// Package auditlog provides the audit logging implementation
package auditlog

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    
    "github.com/davidleathers/dce/internal/domain/audit"
    "github.com/davidleathers/dce/internal/infrastructure/database"
    "github.com/jackc/pgx/v5"
)

// PostgresLogger implements Logger using PostgreSQL
type PostgresLogger struct {
    db           *database.DB
    buffer       chan *audit.Event
    bufferSize   int
    flushTicker  *time.Ticker
    hashCache    *hashCache
    sequencer    *sequencer
    mu           sync.RWMutex
    closed       bool
    wg           sync.WaitGroup
}

// NewPostgresLogger creates a new PostgreSQL-backed logger
func NewPostgresLogger(db *database.DB, opts ...LoggerOption) (*PostgresLogger, error) {
    config := defaultConfig()
    for _, opt := range opts {
        opt(config)
    }
    
    logger := &PostgresLogger{
        db:         db,
        buffer:     make(chan *audit.Event, config.BufferSize),
        bufferSize: config.BufferSize,
        hashCache:  newHashCache(config.HashCacheSize),
        sequencer:  newSequencer(db),
    }
    
    // Start background workers
    logger.startWorkers(config.NumWorkers)
    
    // Start flush ticker
    logger.flushTicker = time.NewTicker(config.FlushInterval)
    go logger.flushLoop()
    
    return logger, nil
}

// Log writes an audit event asynchronously
func (l *PostgresLogger) Log(ctx context.Context, event *audit.Event) error {
    l.mu.RLock()
    if l.closed {
        l.mu.RUnlock()
        return ErrLoggerClosed
    }
    l.mu.RUnlock()
    
    // Assign sequence number
    seq, err := l.sequencer.Next()
    if err != nil {
        return fmt.Errorf("failed to get sequence: %w", err)
    }
    event.SequenceNum = seq
    
    // Get previous hash
    prevHash := l.hashCache.GetPrevious(seq)
    
    // Compute event hash
    if _, err := event.ComputeHash(prevHash); err != nil {
        return fmt.Errorf("failed to compute hash: %w", err)
    }
    
    // Update hash cache
    l.hashCache.Set(seq, event.EventHash)
    
    // Send to buffer (non-blocking with timeout)
    select {
    case l.buffer <- event:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(100 * time.Millisecond):
        // If buffer is full, write directly
        return l.writeEvent(ctx, event)
    }
}

// writeEvent writes a single event to the database
func (l *PostgresLogger) writeEvent(ctx context.Context, event *audit.Event) error {
    query := `
        INSERT INTO audit.events (
            id, sequence_num, timestamp, timestamp_nano,
            type, severity, category,
            actor_id, actor_type, actor_ip, actor_agent,
            target_id, target_type, target_owner,
            action, result, error_code, error_message,
            request_id, session_id, correlation_id,
            environment, service, version,
            compliance_flags, data_classes, legal_basis, retention_days,
            metadata, tags,
            previous_hash, event_hash, signature
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
            $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
            $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
            $31, $32, $33
        )`
    
    _, err := l.db.ExecContext(ctx, query,
        event.ID, event.SequenceNum, event.Timestamp, event.TimestampNano,
        event.Type, event.Severity, event.Category,
        event.ActorID, event.ActorType, event.ActorIP, event.ActorAgent,
        event.TargetID, event.TargetType, event.TargetOwner,
        event.Action, event.Result, event.ErrorCode, event.ErrorMessage,
        event.RequestID, event.SessionID, event.CorrelationID,
        event.Environment, event.Service, event.Version,
        event.ComplianceFlags, event.DataClasses, event.LegalBasis, event.RetentionDays,
        event.Metadata, event.Tags,
        event.PreviousHash, event.EventHash, event.Signature,
    )
    
    return err
}

// LogBatch writes multiple events atomically
func (l *PostgresLogger) LogBatch(ctx context.Context, events []*audit.Event) error {
    if len(events) == 0 {
        return nil
    }
    
    // Begin transaction
    tx, err := l.db.BeginTx(ctx, pgx.TxOptions{
        IsoLevel: pgx.ReadCommitted,
    })
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback(ctx)
    
    // Process events in order
    for _, event := range events {
        // Assign sequence number
        seq, err := l.sequencer.NextTx(tx)
        if err != nil {
            return fmt.Errorf("failed to get sequence: %w", err)
        }
        event.SequenceNum = seq
        
        // Get previous hash
        prevHash := l.hashCache.GetPrevious(seq)
        
        // Compute event hash
        if _, err := event.ComputeHash(prevHash); err != nil {
            return fmt.Errorf("failed to compute hash: %w", err)
        }
        
        // Write event
        if err := l.writeEventTx(ctx, tx, event); err != nil {
            return fmt.Errorf("failed to write event: %w", err)
        }
        
        // Update hash cache
        l.hashCache.Set(seq, event.EventHash)
    }
    
    // Commit transaction
    if err := tx.Commit(ctx); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    
    return nil
}

// Flush ensures all buffered events are written
func (l *PostgresLogger) Flush(ctx context.Context) error {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    // Create temporary channel to collect remaining events
    remaining := make([]*audit.Event, 0, len(l.buffer))
    
    // Drain buffer
    done := false
    for !done {
        select {
        case event := <-l.buffer:
            remaining = append(remaining, event)
        default:
            done = true
        }
    }
    
    // Write remaining events
    if len(remaining) > 0 {
        return l.LogBatch(ctx, remaining)
    }
    
    return nil
}

// Close shuts down the logger gracefully
func (l *PostgresLogger) Close() error {
    l.mu.Lock()
    if l.closed {
        l.mu.Unlock()
        return nil
    }
    l.closed = true
    l.mu.Unlock()
    
    // Stop flush ticker
    l.flushTicker.Stop()
    
    // Flush remaining events
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := l.Flush(ctx); err != nil {
        return err
    }
    
    // Close buffer channel
    close(l.buffer)
    
    // Wait for workers to finish
    l.wg.Wait()
    
    return nil
}

// startWorkers starts background workers for async writes
func (l *PostgresLogger) startWorkers(numWorkers int) {
    for i := 0; i < numWorkers; i++ {
        l.wg.Add(1)
        go l.worker()
    }
}

// worker processes events from the buffer
func (l *PostgresLogger) worker() {
    defer l.wg.Done()
    
    batch := make([]*audit.Event, 0, 100)
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case event, ok := <-l.buffer:
            if !ok {
                // Channel closed, write remaining batch
                if len(batch) > 0 {
                    ctx := context.Background()
                    _ = l.LogBatch(ctx, batch)
                }
                return
            }
            
            batch = append(batch, event)
            
            // Write batch if it reaches size limit
            if len(batch) >= 100 {
                ctx := context.Background()
                if err := l.LogBatch(ctx, batch); err != nil {
                    // Log error but continue
                    // In production, this would alert monitoring
                }
                batch = batch[:0]
            }
            
        case <-ticker.C:
            // Write batch on timeout
            if len(batch) > 0 {
                ctx := context.Background()
                if err := l.LogBatch(ctx, batch); err != nil {
                    // Log error but continue
                }
                batch = batch[:0]
            }
        }
    }
}
```

### Query Service Implementation

```go
// PostgresQuery implements Query interface
type PostgresQuery struct {
    db        *database.DB
    cache     cache.Cache
    exporter  *Exporter
    verifier  *Verifier
}

// Search queries events with filters
func (q *PostgresQuery) Search(ctx context.Context, filter SearchFilter) (*SearchResult, error) {
    // Build query
    query, args := q.buildSearchQuery(filter)
    
    // Execute query
    rows, err := q.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()
    
    // Parse results
    events := make([]*audit.Event, 0, filter.Limit)
    for rows.Next() {
        event := &audit.Event{}
        if err := q.scanEvent(rows, event); err != nil {
            return nil, fmt.Errorf("scan failed: %w", err)
        }
        events = append(events, event)
    }
    
    // Get total count
    countQuery, countArgs := q.buildCountQuery(filter)
    var totalCount int64
    if err := q.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
        return nil, fmt.Errorf("count query failed: %w", err)
    }
    
    return &SearchResult{
        Events:     events,
        TotalCount: totalCount,
        HasMore:    int64(filter.Offset+filter.Limit) < totalCount,
        NextOffset: filter.Offset + filter.Limit,
    }, nil
}

// buildSearchQuery constructs the SQL query
func (q *PostgresQuery) buildSearchQuery(filter SearchFilter) (string, []interface{}) {
    var conditions []string
    var args []interface{}
    argNum := 1
    
    // Time range
    if !filter.StartTime.IsZero() {
        conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argNum))
        args = append(args, filter.StartTime)
        argNum++
    }
    
    if !filter.EndTime.IsZero() {
        conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argNum))
        args = append(args, filter.EndTime)
        argNum++
    }
    
    // Event types
    if len(filter.Types) > 0 {
        conditions = append(conditions, fmt.Sprintf("type = ANY($%d)", argNum))
        args = append(args, filter.Types)
        argNum++
    }
    
    // Actor/Target filters
    if len(filter.ActorIDs) > 0 {
        conditions = append(conditions, fmt.Sprintf("actor_id = ANY($%d)", argNum))
        args = append(args, filter.ActorIDs)
        argNum++
    }
    
    if len(filter.TargetIDs) > 0 {
        conditions = append(conditions, fmt.Sprintf("target_id = ANY($%d)", argNum))
        args = append(args, filter.TargetIDs)
        argNum++
    }
    
    // Build WHERE clause
    whereClause := ""
    if len(conditions) > 0 {
        whereClause = "WHERE " + strings.Join(conditions, " AND ")
    }
    
    // Build ORDER BY clause
    orderBy := "timestamp DESC"
    if filter.OrderBy != "" {
        orderBy = filter.OrderBy
        if filter.OrderDesc {
            orderBy += " DESC"
        } else {
            orderBy += " ASC"
        }
    }
    
    // Build final query
    query := fmt.Sprintf(`
        SELECT 
            id, sequence_num, timestamp, timestamp_nano,
            type, severity, category,
            actor_id, actor_type, actor_ip, actor_agent,
            target_id, target_type, target_owner,
            action, result, error_code, error_message,
            request_id, session_id, correlation_id,
            environment, service, version,
            compliance_flags, data_classes, legal_basis, retention_days,
            metadata, tags,
            previous_hash, event_hash, signature
        FROM audit.events
        %s
        ORDER BY %s
        LIMIT $%d OFFSET $%d
    `, whereClause, orderBy, argNum, argNum+1)
    
    args = append(args, filter.Limit, filter.Offset)
    
    return query, args
}

// Export generates compliance report
func (q *PostgresQuery) Export(ctx context.Context, filter ExportFilter) (io.Reader, error) {
    // Search for events
    searchResult, err := q.Search(ctx, filter.SearchFilter)
    if err != nil {
        return nil, err
    }
    
    // Create exporter
    exporter := NewExporter(filter.Format)
    
    // Configure options
    if filter.IncludeHash {
        exporter.IncludeHashes()
    }
    
    // Export events
    reader, err := exporter.Export(searchResult.Events)
    if err != nil {
        return nil, err
    }
    
    // Compress if requested
    if filter.Compress {
        return compressReader(reader), nil
    }
    
    return reader, nil
}

// VerifyIntegrity checks hash chain
func (q *PostgresQuery) VerifyIntegrity(ctx context.Context, startSeq, endSeq int64) (*IntegrityReport, error) {
    report := &IntegrityReport{
        StartSequence: startSeq,
        EndSequence:   endSeq,
    }
    
    startTime := time.Now()
    defer func() {
        report.VerificationTime = time.Since(startTime)
    }()
    
    // Query events in sequence order
    query := `
        SELECT 
            sequence_num, previous_hash, event_hash,
            id, timestamp, type, actor_id, target_id, action
        FROM audit.events
        WHERE sequence_num BETWEEN $1 AND $2
        ORDER BY sequence_num ASC
    `
    
    rows, err := q.db.QueryContext(ctx, query, startSeq, endSeq)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var previousHash string
    for rows.Next() {
        var event struct {
            SequenceNum  int64
            PreviousHash string
            EventHash    string
            // Other fields for context
        }
        
        if err := rows.Scan(
            &event.SequenceNum,
            &event.PreviousHash,
            &event.EventHash,
            // ... other fields
        ); err != nil {
            return nil, err
        }
        
        report.EventsChecked++
        
        // Verify hash chain
        if event.PreviousHash != previousHash {
            report.InvalidEvents++
            report.BrokenChains = append(report.BrokenChains, ChainBreak{
                SequenceNum:  event.SequenceNum,
                ExpectedHash: previousHash,
                ActualHash:   event.PreviousHash,
            })
        } else {
            report.ValidEvents++
        }
        
        previousHash = event.EventHash
    }
    
    return report, nil
}
```

## Security Measures

### Database Security

```sql
-- 1. Network isolation
-- Audit database should be on separate network segment

-- 2. Connection encryption
-- Require SSL for all connections
ALTER SYSTEM SET ssl = on;
ALTER SYSTEM SET ssl_cert_file = '/etc/postgresql/server.crt';
ALTER SYSTEM SET ssl_key_file = '/etc/postgresql/server.key';

-- 3. Row-level security
ALTER TABLE audit.events ENABLE ROW LEVEL SECURITY;

-- Policy for audit readers (can only read non-sensitive events)
CREATE POLICY audit_reader_policy ON audit.events
    FOR SELECT
    TO audit_reader
    USING (
        -- Exclude sensitive security events
        type NOT IN ('security.auth_failure', 'security.anomaly_detected')
        OR current_user = 'audit_admin'
    );

-- 4. Audit the auditors
CREATE OR REPLACE FUNCTION audit.log_audit_access()
RETURNS event_trigger AS $$
DECLARE
    obj record;
BEGIN
    FOR obj IN SELECT * FROM pg_event_trigger_ddl_commands()
    LOOP
        INSERT INTO audit.access_log (
            accessor_id,
            accessor_type,
            query_type,
            purpose
        ) VALUES (
            current_user,
            'database_user',
            obj.command_tag,
            'DDL operation on audit schema'
        );
    END LOOP;
END;
$$ LANGUAGE plpgsql;

CREATE EVENT TRIGGER audit_ddl_trigger
    ON ddl_command_end
    WHEN TAG IN ('CREATE TABLE', 'ALTER TABLE', 'DROP TABLE')
    EXECUTE FUNCTION audit.log_audit_access();

-- 5. Backup encryption
-- Use pgBackRest with encryption
-- repo1-cipher-type=aes-256-cbc
-- repo1-cipher-pass=<encryption-key>
```

### Application Security

```go
// Cryptographic signing for critical events
type Signer interface {
    Sign(event *audit.Event) error
    Verify(event *audit.Event) (bool, error)
}

// HMAC-based signer implementation
type HMACSigner struct {
    key []byte
}

func (s *HMACSigner) Sign(event *audit.Event) error {
    // Create signing payload
    payload := fmt.Sprintf("%s:%d:%s:%s:%s",
        event.ID,
        event.SequenceNum,
        event.Type,
        event.ActorID,
        event.EventHash,
    )
    
    // Generate HMAC
    h := hmac.New(sha256.New, s.key)
    h.Write([]byte(payload))
    
    event.Signature = base64.StdEncoding.EncodeToString(h.Sum(nil))
    return nil
}

// Access control for audit queries
type AccessController struct {
    permissions map[string][]Permission
}

func (ac *AccessController) CanAccess(user User, filter SearchFilter) error {
    // Check user permissions
    perms, ok := ac.permissions[user.Role]
    if !ok {
        return ErrNoPermission
    }
    
    // Validate filter against permissions
    for _, perm := range perms {
        if !perm.Allows(filter) {
            return ErrInsufficientPermission
        }
    }
    
    return nil
}

// Rate limiting for audit queries
type RateLimiter struct {
    limiter *rate.Limiter
}

func (rl *RateLimiter) Allow(userID string) error {
    if !rl.limiter.Allow() {
        return ErrRateLimitExceeded
    }
    return nil
}
```

## Performance Optimization

### Partitioning Strategy

```sql
-- Automated partition creation
CREATE OR REPLACE FUNCTION audit.auto_create_partitions()
RETURNS VOID AS $$
DECLARE
    start_date DATE;
    end_date DATE;
    partition_name TEXT;
BEGIN
    -- Create partitions for next 3 months
    FOR i IN 0..2 LOOP
        start_date := DATE_TRUNC('month', CURRENT_DATE + (i || ' months')::INTERVAL);
        end_date := start_date + INTERVAL '1 month';
        partition_name := 'events_' || TO_CHAR(start_date, 'YYYY_MM');
        
        -- Check if partition exists
        IF NOT EXISTS (
            SELECT 1 FROM pg_tables 
            WHERE schemaname = 'audit' 
            AND tablename = partition_name
        ) THEN
            EXECUTE format(
                'CREATE TABLE audit.%I PARTITION OF audit.events
                 FOR VALUES FROM (%L) TO (%L)',
                partition_name, start_date, end_date
            );
            
            -- Create indexes on new partition
            EXECUTE format(
                'CREATE INDEX %I ON audit.%I (timestamp DESC)',
                partition_name || '_timestamp_idx', partition_name
            );
            
            EXECUTE format(
                'CREATE INDEX %I ON audit.%I (actor_id, timestamp DESC)',
                partition_name || '_actor_idx', partition_name
            );
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Schedule monthly execution
SELECT cron.schedule('create-audit-partitions', '0 0 1 * *', 'SELECT audit.auto_create_partitions()');
```

### Query Optimization

```go
// Parallel query execution for large exports
type ParallelExporter struct {
    workers   int
    batchSize int
}

func (pe *ParallelExporter) Export(ctx context.Context, filter ExportFilter) (io.Reader, error) {
    // Calculate total events
    count, err := pe.getEventCount(ctx, filter)
    if err != nil {
        return nil, err
    }
    
    // Create channels
    jobs := make(chan exportJob, pe.workers)
    results := make(chan exportResult, pe.workers)
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < pe.workers; i++ {
        wg.Add(1)
        go pe.worker(ctx, jobs, results, &wg)
    }
    
    // Generate jobs
    go func() {
        for offset := 0; offset < int(count); offset += pe.batchSize {
            jobs <- exportJob{
                Filter: filter,
                Offset: offset,
                Limit:  pe.batchSize,
            }
        }
        close(jobs)
    }()
    
    // Collect results
    go func() {
        wg.Wait()
        close(results)
    }()
    
    // Merge results into single reader
    return pe.mergeResults(results), nil
}

// Caching for frequently accessed events
type CachedQuery struct {
    query Query
    cache *ristretto.Cache
}

func (cq *CachedQuery) GetByID(ctx context.Context, id uuid.UUID) (*audit.Event, error) {
    // Check cache
    if val, found := cq.cache.Get(id.String()); found {
        return val.(*audit.Event), nil
    }
    
    // Query database
    event, err := cq.query.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Cache result
    cq.cache.Set(id.String(), event, 1)
    
    return event, nil
}
```

### Archive Optimization

```go
// Intelligent archival with compression
type Archiver struct {
    s3Client    *s3.Client
    compressor  Compressor
    batchSize   int
}

func (a *Archiver) ArchivePartition(ctx context.Context, partition string) error {
    // Query events in batches
    offset := 0
    for {
        events, err := a.queryBatch(ctx, partition, offset, a.batchSize)
        if err != nil {
            return err
        }
        
        if len(events) == 0 {
            break
        }
        
        // Convert to Parquet format
        parquetData, err := a.toParquet(events)
        if err != nil {
            return err
        }
        
        // Compress
        compressed, err := a.compressor.Compress(parquetData)
        if err != nil {
            return err
        }
        
        // Upload to S3
        key := fmt.Sprintf("events/%s/%s_%d.parquet.gz",
            partition, partition, offset/a.batchSize)
        
        if err := a.uploadToS3(ctx, key, compressed); err != nil {
            return err
        }
        
        offset += len(events)
    }
    
    // Create manifest
    manifest := a.createManifest(partition, offset)
    manifestKey := fmt.Sprintf("events/%s/%s.manifest", partition, partition)
    
    return a.uploadToS3(ctx, manifestKey, manifest)
}

// Fast retrieval from archives
type ArchiveReader struct {
    s3Client     *s3.Client
    decompressor Decompressor
    cache        *bigcache.BigCache
}

func (ar *ArchiveReader) ReadArchive(ctx context.Context, partition string, filter SearchFilter) ([]*audit.Event, error) {
    // Check cache first
    cacheKey := ar.getCacheKey(partition, filter)
    if cached, err := ar.cache.Get(cacheKey); err == nil {
        return ar.deserializeEvents(cached)
    }
    
    // Read manifest
    manifest, err := ar.readManifest(ctx, partition)
    if err != nil {
        return nil, err
    }
    
    // Determine which files to read based on filter
    files := ar.selectFiles(manifest, filter)
    
    // Read files in parallel
    results := make(chan []*audit.Event, len(files))
    var wg sync.WaitGroup
    
    for _, file := range files {
        wg.Add(1)
        go func(f string) {
            defer wg.Done()
            events, err := ar.readFile(ctx, f)
            if err == nil {
                results <- events
            }
        }(file)
    }
    
    wg.Wait()
    close(results)
    
    // Merge results
    var allEvents []*audit.Event
    for events := range results {
        allEvents = append(allEvents, events...)
    }
    
    // Apply filter
    filtered := ar.filterEvents(allEvents, filter)
    
    // Cache results
    if serialized, err := ar.serializeEvents(filtered); err == nil {
        _ = ar.cache.Set(cacheKey, serialized)
    }
    
    return filtered, nil
}
```

## Implementation Plan

### Phase 1: Core Infrastructure (Week 1-2)
- [ ] Create audit database and schema
- [ ] Implement basic event model
- [ ] Build hash chain logic
- [ ] Create PostgreSQL logger

### Phase 2: Service Layer (Week 3-4)
- [ ] Implement async logging service
- [ ] Build query service with filters
- [ ] Add export functionality
- [ ] Create integrity verification

### Phase 3: Integration (Week 5-6)
- [ ] Integrate with domain events
- [ ] Add service-layer logging
- [ ] Implement API audit endpoints
- [ ] Create compliance reports

### Phase 4: Archival System (Week 7-8)
- [ ] Build S3 archiver
- [ ] Implement partition management
- [ ] Create archive reader
- [ ] Add lifecycle policies

### Phase 5: Security & Performance (Week 9-10)
- [ ] Implement cryptographic signing
- [ ] Add access control
- [ ] Optimize query performance
- [ ] Load testing and tuning

### Phase 6: Monitoring & Tools (Week 11-12)
- [ ] Build integrity monitoring
- [ ] Create admin tools
- [ ] Add alerting for violations
- [ ] Documentation and training

### Validation Criteria
- [ ] Zero data loss under load
- [ ] < 5ms write latency p99
- [ ] < 1s query response for 1M events
- [ ] Successful integrity verification
- [ ] Compliance report generation
- [ ] 7-year data retrieval test

This comprehensive specification provides a complete blueprint for implementing an immutable audit logging system that meets all compliance requirements while maintaining high performance and security standards.