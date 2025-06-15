package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// AuditRepository implements the audit.EventRepository interface
// Following DCE patterns: PostgreSQL with partitioning, prepared statements, context support
type AuditRepository struct {
	db *pgxpool.Pool
}

// NewAuditRepository creates a new PostgreSQL audit repository
func NewAuditRepository(db *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{db: db}
}

// Store persists a single audit event with integrity checks
// Performance target: < 5ms write latency
func (r *AuditRepository) Store(ctx context.Context, event *audit.Event) error {
	// Validate event
	if err := event.Validate(); err != nil {
		return errors.NewValidationError("INVALID_EVENT", "event validation failed").WithCause(err)
	}

	// Get next sequence number if not set
	if event.SequenceNum == 0 {
		seq, err := r.GetNextSequenceNumber(ctx)
		if err != nil {
			return errors.NewInternalError("failed to get sequence number").WithCause(err)
		}
		event.SequenceNum = int64(seq.Value())
	}

	// Compute hash if not already computed
	if !event.IsImmutable() {
		// Get previous hash
		previousHash, err := r.getLatestHash(ctx)
		if err != nil {
			return errors.NewInternalError("failed to get previous hash").WithCause(err)
		}
		
		if _, err := event.ComputeHash(previousHash); err != nil {
			return errors.NewInternalError("failed to compute event hash").WithCause(err)
		}
	}

	// Marshal metadata and compliance flags
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return errors.NewInternalError("failed to marshal metadata").WithCause(err)
	}

	complianceFlagsJSON, err := json.Marshal(event.ComplianceFlags)
	if err != nil {
		return errors.NewInternalError("failed to marshal compliance flags").WithCause(err)
	}

	// Insert event
	query := `
		INSERT INTO audit_events (
			id, sequence_number, event_type, severity, actor_id, actor_type,
			target_id, target_type, action, result, metadata, compliance_flags,
			ip_address, user_agent, session_id, correlation_id, 
			hash, previous_hash, timestamp, archived
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 
			$13, $14, $15, $16, $17, $18, $19, $20
		)`

	_, err = r.db.Exec(ctx, query,
		event.ID,
		event.SequenceNum,
		string(event.Type),
		string(event.Severity),
		event.ActorID,
		event.ActorType,
		event.TargetID,
		event.TargetType,
		event.Action,
		event.Result,
		metadataJSON,
		complianceFlagsJSON,
		event.ActorIP,
		event.ActorAgent,
		event.SessionID,
		event.CorrelationID,
		event.EventHash,
		event.PreviousHash,
		event.Timestamp,
		false, // archived
	)

	if err != nil {
		// Check for unique constraint violation on sequence number
		if pgErr, ok := err.(*pgx.PgError); ok && pgErr.Code == "23505" {
			return errors.NewConflictError("DUPLICATE_SEQUENCE", 
				"sequence number already exists")
		}
		return errors.NewInternalError("failed to store event").WithCause(err)
	}

	return nil
}

// StoreBatch persists multiple events atomically for high throughput
// Performance target: < 5ms per event in batch
func (r *AuditRepository) StoreBatch(ctx context.Context, events []*audit.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Begin transaction for atomicity
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errors.NewInternalError("failed to begin transaction").WithCause(err)
	}
	defer tx.Rollback(ctx)

	// Get initial sequence number and previous hash
	currentSeq, err := r.GetLatestSequenceNumber(ctx)
	if err != nil {
		return errors.NewInternalError("failed to get latest sequence").WithCause(err)
	}

	previousHash, err := r.getLatestHashTx(ctx, tx)
	if err != nil {
		return errors.NewInternalError("failed to get previous hash").WithCause(err)
	}

	// Prepare batch insert
	stmt, err := tx.Prepare(ctx, "batch_insert_audit_events", `
		INSERT INTO audit_events (
			id, sequence_number, event_type, severity, actor_id, actor_type,
			target_id, target_type, action, result, metadata, compliance_flags,
			ip_address, user_agent, session_id, correlation_id,
			hash, previous_hash, timestamp, archived
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20
		)`)
	if err != nil {
		return errors.NewInternalError("failed to prepare statement").WithCause(err)
	}

	// Process each event
	for i, event := range events {
		// Validate event
		if err := event.Validate(); err != nil {
			return errors.NewValidationError("INVALID_EVENT", 
				fmt.Sprintf("event %d validation failed", i)).WithCause(err)
		}

		// Assign sequence number
		nextSeq, err := currentSeq.Next()
		if err != nil {
			return errors.NewInternalError("sequence number overflow").WithCause(err)
		}
		event.SequenceNum = int64(nextSeq.Value())
		currentSeq = nextSeq

		// Compute hash chain
		if !event.IsImmutable() {
			if _, err := event.ComputeHash(previousHash); err != nil {
				return errors.NewInternalError("failed to compute hash").WithCause(err)
			}
		}
		previousHash = event.EventHash

		// Marshal JSON fields
		metadataJSON, err := json.Marshal(event.Metadata)
		if err != nil {
			return errors.NewInternalError("failed to marshal metadata").WithCause(err)
		}

		complianceFlagsJSON, err := json.Marshal(event.ComplianceFlags)
		if err != nil {
			return errors.NewInternalError("failed to marshal compliance flags").WithCause(err)
		}

		// Execute insert
		_, err = tx.Exec(ctx, stmt.Name,
			event.ID,
			event.SequenceNum,
			string(event.Type),
			string(event.Severity),
			event.ActorID,
			event.ActorType,
			event.TargetID,
			event.TargetType,
			event.Action,
			event.Result,
			metadataJSON,
			complianceFlagsJSON,
			event.ActorIP,
			event.ActorAgent,
			event.SessionID,
			event.CorrelationID,
			event.EventHash,
			event.PreviousHash,
			event.Timestamp,
			false, // archived
		)

		if err != nil {
			return errors.NewInternalError(
				fmt.Sprintf("failed to store event %d", i)).WithCause(err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return errors.NewInternalError("failed to commit transaction").WithCause(err)
	}

	return nil
}

// GetByID retrieves an event by its unique identifier
func (r *AuditRepository) GetByID(ctx context.Context, id uuid.UUID) (*audit.Event, error) {
	query := `
		SELECT 
			id, sequence_number, event_type, severity, actor_id, actor_type,
			target_id, target_type, action, result, metadata, compliance_flags,
			ip_address, user_agent, session_id, correlation_id,
			hash, previous_hash, timestamp
		FROM audit_events
		WHERE id = $1`

	var event audit.Event
	var metadataJSON, complianceFlagsJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&event.ID,
		&event.SequenceNum,
		&event.Type,
		&event.Severity,
		&event.ActorID,
		&event.ActorType,
		&event.TargetID,
		&event.TargetType,
		&event.Action,
		&event.Result,
		&metadataJSON,
		&complianceFlagsJSON,
		&event.ActorIP,
		&event.ActorAgent,
		&event.SessionID,
		&event.CorrelationID,
		&event.EventHash,
		&event.PreviousHash,
		&event.Timestamp,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.NewNotFoundError("EVENT_NOT_FOUND", 
			fmt.Sprintf("event with ID %s not found", id))
	}
	if err != nil {
		return nil, errors.NewInternalError("failed to get event").WithCause(err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
		return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
	}

	if err := json.Unmarshal(complianceFlagsJSON, &event.ComplianceFlags); err != nil {
		return nil, errors.NewInternalError("failed to unmarshal compliance flags").WithCause(err)
	}

	// Calculate timestamp nano
	event.TimestampNano = event.Timestamp.UnixNano()

	return &event, nil
}

// GetBySequence retrieves an event by its sequence number
func (r *AuditRepository) GetBySequence(ctx context.Context, seq values.SequenceNumber) (*audit.Event, error) {
	query := `
		SELECT 
			id, sequence_number, event_type, severity, actor_id, actor_type,
			target_id, target_type, action, result, metadata, compliance_flags,
			ip_address, user_agent, session_id, correlation_id,
			hash, previous_hash, timestamp
		FROM audit_events
		WHERE sequence_number = $1`

	var event audit.Event
	var metadataJSON, complianceFlagsJSON []byte

	err := r.db.QueryRow(ctx, query, seq.Value()).Scan(
		&event.ID,
		&event.SequenceNum,
		&event.Type,
		&event.Severity,
		&event.ActorID,
		&event.ActorType,
		&event.TargetID,
		&event.TargetType,
		&event.Action,
		&event.Result,
		&metadataJSON,
		&complianceFlagsJSON,
		&event.ActorIP,
		&event.ActorAgent,
		&event.SessionID,
		&event.CorrelationID,
		&event.EventHash,
		&event.PreviousHash,
		&event.Timestamp,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.NewNotFoundError("EVENT_NOT_FOUND",
			fmt.Sprintf("event with sequence %s not found", seq))
	}
	if err != nil {
		return nil, errors.NewInternalError("failed to get event").WithCause(err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
		return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
	}

	if err := json.Unmarshal(complianceFlagsJSON, &event.ComplianceFlags); err != nil {
		return nil, errors.NewInternalError("failed to unmarshal compliance flags").WithCause(err)
	}

	// Calculate timestamp nano
	event.TimestampNano = event.Timestamp.UnixNano()

	return &event, nil
}

// GetNextSequenceNumber returns the next available sequence number
// Thread-safe using PostgreSQL sequence
func (r *AuditRepository) GetNextSequenceNumber(ctx context.Context) (values.SequenceNumber, error) {
	var seq int64
	err := r.db.QueryRow(ctx, "SELECT nextval('audit_events_sequence_number_seq')").Scan(&seq)
	if err != nil {
		return values.SequenceNumber{}, errors.NewInternalError("failed to get next sequence").WithCause(err)
	}

	return values.NewSequenceNumber(uint64(seq))
}

// GetLatestSequenceNumber returns the highest sequence number in use
func (r *AuditRepository) GetLatestSequenceNumber(ctx context.Context) (values.SequenceNumber, error) {
	var seq sql.NullInt64
	err := r.db.QueryRow(ctx, "SELECT MAX(sequence_number) FROM audit_events").Scan(&seq)
	if err != nil {
		return values.SequenceNumber{}, errors.NewInternalError("failed to get latest sequence").WithCause(err)
	}

	if !seq.Valid || seq.Int64 == 0 {
		return values.FirstSequenceNumber(), nil
	}

	return values.NewSequenceNumber(uint64(seq.Int64))
}

// GetSequenceRange retrieves events within a sequence number range
func (r *AuditRepository) GetSequenceRange(ctx context.Context, start, end values.SequenceNumber) ([]*audit.Event, error) {
	if start.GreaterThan(end) {
		return nil, errors.NewValidationError("INVALID_RANGE", "start sequence must be <= end sequence")
	}

	query := `
		SELECT 
			id, sequence_number, event_type, severity, actor_id, actor_type,
			target_id, target_type, action, result, metadata, compliance_flags,
			ip_address, user_agent, session_id, correlation_id,
			hash, previous_hash, timestamp
		FROM audit_events
		WHERE sequence_number >= $1 AND sequence_number <= $2
		ORDER BY sequence_number ASC`

	rows, err := r.db.Query(ctx, query, start.Value(), end.Value())
	if err != nil {
		return nil, errors.NewInternalError("failed to query events").WithCause(err)
	}
	defer rows.Close()

	events := make([]*audit.Event, 0)
	for rows.Next() {
		event := &audit.Event{}
		var metadataJSON, complianceFlagsJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.SequenceNum,
			&event.Type,
			&event.Severity,
			&event.ActorID,
			&event.ActorType,
			&event.TargetID,
			&event.TargetType,
			&event.Action,
			&event.Result,
			&metadataJSON,
			&complianceFlagsJSON,
			&event.ActorIP,
			&event.ActorAgent,
			&event.SessionID,
			&event.CorrelationID,
			&event.EventHash,
			&event.PreviousHash,
			&event.Timestamp,
		)
		if err != nil {
			return nil, errors.NewInternalError("failed to scan event").WithCause(err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
		}

		if err := json.Unmarshal(complianceFlagsJSON, &event.ComplianceFlags); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal compliance flags").WithCause(err)
		}

		event.TimestampNano = event.Timestamp.UnixNano()
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating rows").WithCause(err)
	}

	return events, nil
}

// GetEvents retrieves events based on filtering criteria with pagination
func (r *AuditRepository) GetEvents(ctx context.Context, filter audit.EventFilter) (*audit.EventPage, error) {
	// Build query dynamically based on filter
	query, args := r.buildFilterQuery(filter)
	
	// Count total matching records
	countQuery := "SELECT COUNT(*) FROM audit_events WHERE " + strings.TrimPrefix(query, "SELECT * FROM audit_events WHERE ")
	var totalCount int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, errors.NewInternalError("failed to count events").WithCause(err)
	}

	// Execute main query
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("failed to query events").WithCause(err)
	}
	defer rows.Close()

	events := make([]*audit.Event, 0)
	for rows.Next() {
		event := &audit.Event{}
		var metadataJSON, complianceFlagsJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.SequenceNum,
			&event.Type,
			&event.Severity,
			&event.ActorID,
			&event.ActorType,
			&event.TargetID,
			&event.TargetType,
			&event.Action,
			&event.Result,
			&metadataJSON,
			&complianceFlagsJSON,
			&event.ActorIP,
			&event.ActorAgent,
			&event.SessionID,
			&event.CorrelationID,
			&event.EventHash,
			&event.PreviousHash,
			&event.Timestamp,
		)
		if err != nil {
			return nil, errors.NewInternalError("failed to scan event").WithCause(err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
		}

		if err := json.Unmarshal(complianceFlagsJSON, &event.ComplianceFlags); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal compliance flags").WithCause(err)
		}

		event.TimestampNano = event.Timestamp.UnixNano()
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating rows").WithCause(err)
	}

	// Prepare result page
	page := &audit.EventPage{
		Events:     events,
		TotalCount: totalCount,
		HasMore:    int64(filter.Offset+filter.Limit) < totalCount,
	}

	return page, nil
}

// Count returns the total number of events matching the filter
func (r *AuditRepository) Count(ctx context.Context, filter audit.EventFilter) (int64, error) {
	// Remove pagination from filter for counting
	filter.Limit = 0
	filter.Offset = 0
	
	query, args := r.buildFilterQuery(filter)
	countQuery := "SELECT COUNT(*) FROM audit_events WHERE " + strings.TrimPrefix(query, "SELECT * FROM audit_events WHERE ")
	
	var count int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return 0, errors.NewInternalError("failed to count events").WithCause(err)
	}

	return count, nil
}

// GetEventsForActor retrieves all events for a specific actor
func (r *AuditRepository) GetEventsForActor(ctx context.Context, actorID string, filter audit.EventFilter) (*audit.EventPage, error) {
	// Add actor filter
	filter.ActorIDs = append(filter.ActorIDs, actorID)
	return r.GetEvents(ctx, filter)
}

// GetEventsForTarget retrieves all events for a specific target
func (r *AuditRepository) GetEventsForTarget(ctx context.Context, targetID string, filter audit.EventFilter) (*audit.EventPage, error) {
	// Add target filter
	filter.TargetIDs = append(filter.TargetIDs, targetID)
	return r.GetEvents(ctx, filter)
}

// GetEventsByTimeRange retrieves events within a time range
func (r *AuditRepository) GetEventsByTimeRange(ctx context.Context, start, end time.Time, filter audit.EventFilter) (*audit.EventPage, error) {
	// Set time range in filter
	filter.StartTime = &start
	filter.EndTime = &end
	return r.GetEvents(ctx, filter)
}

// GetEventsByType retrieves events of specific types
func (r *AuditRepository) GetEventsByType(ctx context.Context, eventTypes []audit.EventType, filter audit.EventFilter) (*audit.EventPage, error) {
	// Set event types in filter
	filter.Types = eventTypes
	return r.GetEvents(ctx, filter)
}

// GetEventsForCompliance retrieves events relevant for compliance reporting
func (r *AuditRepository) GetEventsForCompliance(ctx context.Context, flags []string, filter audit.EventFilter) (*audit.EventPage, error) {
	// Set compliance flags in filter
	filter.ComplianceFlags = flags
	return r.GetEvents(ctx, filter)
}

// GetExpiredEvents returns events that have exceeded their retention period
func (r *AuditRepository) GetExpiredEvents(ctx context.Context, before time.Time, limit int) ([]*audit.Event, error) {
	query := `
		SELECT 
			id, sequence_number, event_type, severity, actor_id, actor_type,
			target_id, target_type, action, result, metadata, compliance_flags,
			ip_address, user_agent, session_id, correlation_id,
			hash, previous_hash, timestamp
		FROM audit_events
		WHERE timestamp < $1 AND archived = false
		ORDER BY timestamp ASC
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, before, limit)
	if err != nil {
		return nil, errors.NewInternalError("failed to query expired events").WithCause(err)
	}
	defer rows.Close()

	events := make([]*audit.Event, 0)
	for rows.Next() {
		event := &audit.Event{}
		var metadataJSON, complianceFlagsJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.SequenceNum,
			&event.Type,
			&event.Severity,
			&event.ActorID,
			&event.ActorType,
			&event.TargetID,
			&event.TargetType,
			&event.Action,
			&event.Result,
			&metadataJSON,
			&complianceFlagsJSON,
			&event.ActorIP,
			&event.ActorAgent,
			&event.SessionID,
			&event.CorrelationID,
			&event.EventHash,
			&event.PreviousHash,
			&event.Timestamp,
		)
		if err != nil {
			return nil, errors.NewInternalError("failed to scan event").WithCause(err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
		}

		if err := json.Unmarshal(complianceFlagsJSON, &event.ComplianceFlags); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal compliance flags").WithCause(err)
		}

		event.TimestampNano = event.Timestamp.UnixNano()
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating rows").WithCause(err)
	}

	return events, nil
}

// GetGDPRRelevantEvents returns events containing PII or GDPR-relevant data
func (r *AuditRepository) GetGDPRRelevantEvents(ctx context.Context, dataSubject string, filter audit.EventFilter) (*audit.EventPage, error) {
	// Add GDPR compliance flags
	filter.ComplianceFlags = append(filter.ComplianceFlags, "gdpr_relevant", "contains_pii")
	
	// If data subject is provided, search in actor or target
	if dataSubject != "" {
		filter.ActorIDs = append(filter.ActorIDs, dataSubject)
		filter.TargetIDs = append(filter.TargetIDs, dataSubject)
	}
	
	return r.GetEvents(ctx, filter)
}

// GetTCPARelevantEvents returns events relevant for TCPA compliance
func (r *AuditRepository) GetTCPARelevantEvents(ctx context.Context, phoneNumber string, filter audit.EventFilter) (*audit.EventPage, error) {
	// Add TCPA compliance flag
	filter.ComplianceFlags = append(filter.ComplianceFlags, "tcpa_relevant")
	
	// Add phone number to search text if provided
	if phoneNumber != "" {
		filter.SearchText = phoneNumber
	}
	
	return r.GetEvents(ctx, filter)
}

// VerifyEventIntegrity verifies the cryptographic hash of an event
func (r *AuditRepository) VerifyEventIntegrity(ctx context.Context, eventID uuid.UUID) (*audit.IntegrityResult, error) {
	// Get the event
	event, err := r.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}

	// Create a copy to compute hash
	eventCopy := event.Clone()
	
	// Compute hash
	computedHash, err := eventCopy.ComputeHash(event.PreviousHash)
	if err != nil {
		return &audit.IntegrityResult{
			EventID:      eventID,
			IsValid:      false,
			HashValid:    false,
			ComputedHash: "",
			StoredHash:   event.EventHash,
			PreviousHash: event.PreviousHash,
			VerifiedAt:   time.Now().UTC(),
			Errors:       []string{"Failed to compute hash: " + err.Error()},
		}, nil
	}

	// Compare hashes
	isValid := computedHash == event.EventHash

	result := &audit.IntegrityResult{
		EventID:      eventID,
		IsValid:      isValid,
		HashValid:    isValid,
		ComputedHash: computedHash,
		StoredHash:   event.EventHash,
		PreviousHash: event.PreviousHash,
		VerifiedAt:   time.Now().UTC(),
	}

	if !isValid {
		result.Errors = []string{"Hash mismatch: computed hash does not match stored hash"}
	}

	return result, nil
}

// VerifyChainIntegrity verifies the hash chain for a range of events
func (r *AuditRepository) VerifyChainIntegrity(ctx context.Context, start, end values.SequenceNumber) (*audit.ChainIntegrityResult, error) {
	// Get events in range
	events, err := r.GetSequenceRange(ctx, start, end)
	if err != nil {
		return nil, err
	}

	result := &audit.ChainIntegrityResult{
		StartSequence: start,
		EndSequence:   end,
		EventsChecked: int64(len(events)),
		IsValid:       true,
		CheckTime:     0, // Will be calculated
		VerifiedAt:    time.Now().UTC(),
		Errors:        make([]audit.ChainError, 0),
	}

	startTime := time.Now()

	// Verify each event and its chain
	for i, event := range events {
		// Verify individual event integrity
		integrityResult, err := r.VerifyEventIntegrity(ctx, event.ID)
		if err != nil {
			result.Errors = append(result.Errors, audit.ChainError{
				Sequence: values.MustNewSequenceNumber(uint64(event.SequenceNum)),
				EventID:  event.ID,
				Type:     "verification_error",
				Message:  err.Error(),
			})
			result.IsValid = false
			continue
		}

		if !integrityResult.IsValid {
			result.Errors = append(result.Errors, audit.ChainError{
				Sequence: values.MustNewSequenceNumber(uint64(event.SequenceNum)),
				EventID:  event.ID,
				Type:     "hash_mismatch",
				Message:  "Event hash verification failed",
			})
			result.IsValid = false
		}

		// Verify chain continuity (except for first event)
		if i > 0 {
			previousEvent := events[i-1]
			if event.PreviousHash != previousEvent.EventHash {
				result.Errors = append(result.Errors, audit.ChainError{
					Sequence: values.MustNewSequenceNumber(uint64(event.SequenceNum)),
					EventID:  event.ID,
					Type:     "chain_break",
					Message:  fmt.Sprintf("Previous hash mismatch: expected %s, got %s", 
						previousEvent.EventHash, event.PreviousHash),
				})
				result.IsValid = false
				
				if result.BrokenAt == nil {
					seq := values.MustNewSequenceNumber(uint64(event.SequenceNum))
					result.BrokenAt = &seq
					result.BrokenReason = "Hash chain discontinuity detected"
				}
			}
		}
	}

	result.CheckTime = time.Since(startTime)

	return result, nil
}

// GetIntegrityReport generates a comprehensive integrity report
func (r *AuditRepository) GetIntegrityReport(ctx context.Context, criteria audit.IntegrityCriteria) (*audit.IntegrityReport, error) {
	report := &audit.IntegrityReport{
		GeneratedAt:   time.Now().UTC(),
		Criteria:      criteria,
		OverallStatus: "HEALTHY",
		IsHealthy:     true,
	}

	startTime := time.Now()

	// Determine sequence range
	var startSeq, endSeq values.SequenceNumber
	if criteria.StartSequence != nil {
		startSeq = *criteria.StartSequence
	} else {
		startSeq = values.FirstSequenceNumber()
	}

	if criteria.EndSequence != nil {
		endSeq = *criteria.EndSequence
	} else {
		latest, err := r.GetLatestSequenceNumber(ctx)
		if err != nil {
			return nil, err
		}
		endSeq = latest
	}

	// Check hash chain integrity
	if criteria.CheckHashChain {
		chainResult, err := r.VerifyChainIntegrity(ctx, startSeq, endSeq)
		if err != nil {
			report.CriticalErrors = append(report.CriticalErrors, 
				"Failed to verify hash chain: " + err.Error())
			report.OverallStatus = "CRITICAL"
			report.IsHealthy = false
		} else {
			report.ChainResult = chainResult
			report.TotalEvents = chainResult.EventsChecked
			report.VerifiedEvents = chainResult.EventsChecked - int64(len(chainResult.Errors))
			report.FailedEvents = int64(len(chainResult.Errors))
			
			if !chainResult.IsValid {
				report.OverallStatus = "DEGRADED"
				report.IsHealthy = false
			}
		}
	}

	// Check for sequence gaps
	if criteria.CheckSequencing {
		gaps, err := r.findSequenceGaps(ctx, startSeq, endSeq)
		if err != nil {
			report.Warnings = append(report.Warnings, 
				"Failed to check sequence gaps: " + err.Error())
		} else if len(gaps) > 0 {
			report.SequenceGaps = gaps
			report.OverallStatus = "DEGRADED"
			report.IsHealthy = false
		}
	}

	// Add recommendations based on findings
	if !report.IsHealthy {
		if len(report.SequenceGaps) > 0 {
			report.Recommendations = append(report.Recommendations,
				"Investigate missing sequence numbers to ensure no events were lost")
		}
		if report.ChainResult != nil && !report.ChainResult.IsValid {
			report.Recommendations = append(report.Recommendations,
				"Review hash chain breaks and consider re-computing hashes from backup")
		}
	}

	report.VerificationTime = time.Since(startTime)

	return report, nil
}

// GetStats returns repository performance statistics
func (r *AuditRepository) GetStats(ctx context.Context) (*audit.RepositoryStats, error) {
	stats := &audit.RepositoryStats{
		CollectedAt: time.Now().UTC(),
	}

	// Get total event count
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&stats.TotalEvents)
	if err != nil {
		return nil, errors.NewInternalError("failed to count total events").WithCause(err)
	}

	// Get today's event count
	today := time.Now().UTC().Truncate(24 * time.Hour)
	err = r.db.QueryRow(ctx, 
		"SELECT COUNT(*) FROM audit_events WHERE timestamp >= $1", 
		today).Scan(&stats.EventsToday)
	if err != nil {
		return nil, errors.NewInternalError("failed to count today's events").WithCause(err)
	}

	// Get this week's event count
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	err = r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM audit_events WHERE timestamp >= $1",
		weekStart).Scan(&stats.EventsThisWeek)
	if err != nil {
		return nil, errors.NewInternalError("failed to count this week's events").WithCause(err)
	}

	// Get this month's event count
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	err = r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM audit_events WHERE timestamp >= $1",
		monthStart).Scan(&stats.EventsThisMonth)
	if err != nil {
		return nil, errors.NewInternalError("failed to count this month's events").WithCause(err)
	}

	// Get latest sequence number
	latestSeq, err := r.GetLatestSequenceNumber(ctx)
	if err != nil {
		return nil, err
	}
	stats.LatestSequence = latestSeq

	// Get compliance event counts
	err = r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_events 
		 WHERE compliance_flags ? 'gdpr_relevant'`).Scan(&stats.GDPREvents)
	if err != nil {
		return nil, errors.NewInternalError("failed to count GDPR events").WithCause(err)
	}

	err = r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM audit_events 
		 WHERE compliance_flags ? 'tcpa_relevant'`).Scan(&stats.TCPAEvents)
	if err != nil {
		return nil, errors.NewInternalError("failed to count TCPA events").WithCause(err)
	}

	// Note: Average query times would typically come from application metrics,
	// not from the database itself
	stats.AverageInsertTime = 3 * time.Millisecond // Example value
	stats.AverageQueryTime = 5 * time.Millisecond  // Example value

	return stats, nil
}

// GetHealthCheck performs health check on the repository
func (r *AuditRepository) GetHealthCheck(ctx context.Context) (*audit.HealthCheckResult, error) {
	result := &audit.HealthCheckResult{
		Status:    "HEALTHY",
		Healthy:   true,
		CheckedAt: time.Now().UTC(),
		Metrics:   make(map[string]interface{}),
	}

	startTime := time.Now()

	// Check database connectivity
	err := r.db.Ping(ctx)
	if err != nil {
		result.DatabaseHealth = false
		result.Errors = append(result.Errors, "Database ping failed: " + err.Error())
		result.Status = "UNHEALTHY"
		result.Healthy = false
	} else {
		result.DatabaseHealth = true
	}

	// Check sequence health
	latestSeq, err := r.GetLatestSequenceNumber(ctx)
	if err != nil {
		result.SequenceHealth = false
		result.Errors = append(result.Errors, "Failed to get latest sequence: " + err.Error())
		result.Status = "DEGRADED"
		result.Healthy = false
	} else {
		result.SequenceHealth = true
		result.Metrics["latest_sequence"] = latestSeq.Value()
	}

	// Check basic integrity (last 100 events)
	if latestSeq.Value() > 100 {
		start, _ := latestSeq.Subtract(100)
		chainResult, err := r.VerifyChainIntegrity(ctx, start, latestSeq)
		if err != nil {
			result.IntegrityHealth = false
			result.Warnings = append(result.Warnings, "Failed to verify recent integrity: " + err.Error())
		} else {
			result.IntegrityHealth = chainResult.IsValid
			if !chainResult.IsValid {
				result.Status = "DEGRADED"
				result.Warnings = append(result.Warnings, "Recent events have integrity issues")
			}
		}
	} else {
		result.IntegrityHealth = true
	}

	// Check performance (simple query)
	queryStart := time.Now()
	var count int
	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM audit_events LIMIT 1").Scan(&count)
	queryTime := time.Since(queryStart)
	
	if err != nil {
		result.PerformanceHealth = false
		result.Errors = append(result.Errors, "Performance check failed: " + err.Error())
	} else if queryTime > 100*time.Millisecond {
		result.PerformanceHealth = false
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Query performance degraded: %v", queryTime))
		result.Status = "DEGRADED"
	} else {
		result.PerformanceHealth = true
		result.Metrics["query_time_ms"] = queryTime.Milliseconds()
	}

	result.ResponseTime = time.Since(startTime)

	return result, nil
}

// GetStorageInfo returns information about storage usage
func (r *AuditRepository) GetStorageInfo(ctx context.Context) (*audit.StorageInfo, error) {
	info := &audit.StorageInfo{
		CollectedAt: time.Now().UTC(),
		Partitions:  make([]audit.PartitionInfo, 0),
	}

	// Get table size information
	query := `
		SELECT 
			pg_size_pretty(pg_total_relation_size('audit_events')),
			pg_total_relation_size('audit_events'),
			pg_size_pretty(pg_relation_size('audit_events')),
			pg_relation_size('audit_events'),
			pg_size_pretty(pg_indexes_size('audit_events')),
			pg_indexes_size('audit_events')
	`
	
	var totalSizePretty, dataSizePretty, indexSizePretty string
	err := r.db.QueryRow(ctx, query).Scan(
		&totalSizePretty, &info.TotalSize,
		&dataSizePretty, &info.DataSize,
		&indexSizePretty, &info.IndexSize,
	)
	if err != nil {
		return nil, errors.NewInternalError("failed to get storage info").WithCause(err)
	}

	// Get partition information
	partitionQuery := `
		SELECT 
			child.relname AS partition_name,
			pg_get_expr(child.relpartbound, child.oid) AS partition_range,
			pg_size_pretty(pg_total_relation_size(child.oid)) AS size_pretty,
			pg_total_relation_size(child.oid) AS size_bytes,
			(SELECT COUNT(*) FROM audit_events WHERE tableoid = child.oid) AS event_count
		FROM pg_inherits
		JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
		JOIN pg_class child ON pg_inherits.inhrelid = child.oid
		WHERE parent.relname = 'audit_events'
		ORDER BY child.relname`

	rows, err := r.db.Query(ctx, partitionQuery)
	if err != nil {
		// Partitioning might not be set up yet
		info.Partitions = []audit.PartitionInfo{}
	} else {
		defer rows.Close()

		for rows.Next() {
			var partition audit.PartitionInfo
			var partitionRange string
			var sizePretty string

			err := rows.Scan(
				&partition.Name,
				&partitionRange,
				&sizePretty,
				&partition.Size,
				&partition.EventCount,
			)
			if err != nil {
				continue
			}

			// Parse partition range to get time bounds
			// Example: FOR VALUES FROM ('2024-01-01') TO ('2024-02-01')
			partition.Status = "ACTIVE"
			info.Partitions = append(info.Partitions, partition)
		}
	}

	// Calculate growth metrics (simplified - would typically use historical data)
	// For now, estimate based on current size and age
	var oldestTimestamp time.Time
	err = r.db.QueryRow(ctx, "SELECT MIN(timestamp) FROM audit_events").Scan(&oldestTimestamp)
	if err == nil && !oldestTimestamp.IsZero() {
		daysSinceStart := time.Since(oldestTimestamp).Hours() / 24
		if daysSinceStart > 0 {
			info.DailyGrowth = int64(float64(info.TotalSize) / daysSinceStart)
			info.WeeklyGrowth = info.DailyGrowth * 7
			info.MonthlyGrowth = info.DailyGrowth * 30
		}
	}

	return info, nil
}

// Vacuum performs maintenance operations (index optimization, cleanup)
func (r *AuditRepository) Vacuum(ctx context.Context) error {
	// Note: VACUUM cannot be executed inside a transaction block
	// This is a simplified version - production would need more sophisticated handling
	
	_, err := r.db.Exec(ctx, "VACUUM ANALYZE audit_events")
	if err != nil {
		return errors.NewInternalError("failed to vacuum table").WithCause(err)
	}

	return nil
}

// Helper methods

// getLatestHash retrieves the hash of the most recent event
func (r *AuditRepository) getLatestHash(ctx context.Context) (string, error) {
	var hash sql.NullString
	err := r.db.QueryRow(ctx, 
		"SELECT hash FROM audit_events ORDER BY sequence_number DESC LIMIT 1").Scan(&hash)
	if err == pgx.ErrNoRows {
		return "", nil // First event, no previous hash
	}
	if err != nil {
		return "", errors.NewInternalError("failed to get latest hash").WithCause(err)
	}

	if !hash.Valid {
		return "", nil
	}

	return hash.String, nil
}

// getLatestHashTx retrieves the hash of the most recent event within a transaction
func (r *AuditRepository) getLatestHashTx(ctx context.Context, tx pgx.Tx) (string, error) {
	var hash sql.NullString
	err := tx.QueryRow(ctx,
		"SELECT hash FROM audit_events ORDER BY sequence_number DESC LIMIT 1").Scan(&hash)
	if err == pgx.ErrNoRows {
		return "", nil // First event, no previous hash
	}
	if err != nil {
		return "", errors.NewInternalError("failed to get latest hash").WithCause(err)
	}

	if !hash.Valid {
		return "", nil
	}

	return hash.String, nil
}

// buildFilterQuery builds a dynamic SQL query based on the provided filter
func (r *AuditRepository) buildFilterQuery(filter audit.EventFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argCounter := 1

	// Base query
	query := `
		SELECT 
			id, sequence_number, event_type, severity, actor_id, actor_type,
			target_id, target_type, action, result, metadata, compliance_flags,
			ip_address, user_agent, session_id, correlation_id,
			hash, previous_hash, timestamp
		FROM audit_events WHERE 1=1`

	// Event type filter
	if len(filter.Types) > 0 {
		typeStrings := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			typeStrings[i] = string(t)
		}
		conditions = append(conditions, fmt.Sprintf("event_type = ANY($%d)", argCounter))
		args = append(args, pq.Array(typeStrings))
		argCounter++
	}

	// Severity filter
	if len(filter.Severities) > 0 {
		severityStrings := make([]string, len(filter.Severities))
		for i, s := range filter.Severities {
			severityStrings[i] = string(s)
		}
		conditions = append(conditions, fmt.Sprintf("severity = ANY($%d)", argCounter))
		args = append(args, pq.Array(severityStrings))
		argCounter++
	}

	// Actor filter
	if len(filter.ActorIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("actor_id = ANY($%d)", argCounter))
		args = append(args, pq.Array(filter.ActorIDs))
		argCounter++
	}

	// Target filter
	if len(filter.TargetIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("target_id = ANY($%d)", argCounter))
		args = append(args, pq.Array(filter.TargetIDs))
		argCounter++
	}

	// Time range filter
	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argCounter))
		args = append(args, *filter.StartTime)
		argCounter++
	}

	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argCounter))
		args = append(args, *filter.EndTime)
		argCounter++
	}

	// Compliance flags filter
	if len(filter.ComplianceFlags) > 0 {
		for _, flag := range filter.ComplianceFlags {
			conditions = append(conditions, fmt.Sprintf("compliance_flags ? $%d", argCounter))
			args = append(args, flag)
			argCounter++
		}
	}

	// Text search
	if filter.SearchText != "" {
		searchCondition := fmt.Sprintf(`(
			actor_id ILIKE $%d OR 
			target_id ILIKE $%d OR 
			action ILIKE $%d OR 
			metadata::text ILIKE $%d
		)`, argCounter, argCounter, argCounter, argCounter)
		conditions = append(conditions, searchCondition)
		args = append(args, "%"+filter.SearchText+"%")
		argCounter++
	}

	// Add conditions to query
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	// Add ordering
	orderBy := "sequence_number"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	
	if filter.OrderDesc {
		query += fmt.Sprintf(" ORDER BY %s DESC", orderBy)
	} else {
		query += fmt.Sprintf(" ORDER BY %s ASC", orderBy)
	}

	// Add pagination
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCounter)
		args = append(args, filter.Limit)
		argCounter++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCounter)
		args = append(args, filter.Offset)
		argCounter++
	}

	return query, args
}

// findSequenceGaps finds gaps in the sequence number range
func (r *AuditRepository) findSequenceGaps(ctx context.Context, start, end values.SequenceNumber) ([]*audit.SequenceGap, error) {
	query := `
		WITH sequence_numbers AS (
			SELECT sequence_number,
				   LAG(sequence_number) OVER (ORDER BY sequence_number) AS prev_seq
			FROM audit_events
			WHERE sequence_number >= $1 AND sequence_number <= $2
		)
		SELECT prev_seq + 1 AS gap_start, sequence_number - 1 AS gap_end
		FROM sequence_numbers
		WHERE sequence_number - prev_seq > 1`

	rows, err := r.db.Query(ctx, query, start.Value(), end.Value())
	if err != nil {
		return nil, errors.NewInternalError("failed to find sequence gaps").WithCause(err)
	}
	defer rows.Close()

	gaps := make([]*audit.SequenceGap, 0)
	gapCounter := 1
	for rows.Next() {
		var gapStart, gapEnd int64
		if err := rows.Scan(&gapStart, &gapEnd); err != nil {
			return nil, errors.NewInternalError("failed to scan gap").WithCause(err)
		}

		startSeq, _ := values.NewSequenceNumber(uint64(gapStart))
		endSeq, _ := values.NewSequenceNumber(uint64(gapEnd))
		gapSize := endSeq.Value() - startSeq.Value() + 1

		// Determine severity based on gap size
		severity := "low"
		if gapSize > 100 {
			severity = "high"
		} else if gapSize > 10 {
			severity = "medium"
		}

		gaps = append(gaps, &audit.SequenceGap{
			GapID:          fmt.Sprintf("gap-%d", gapCounter),
			StartSequence:  startSeq,
			EndSequence:    endSeq,
			GapSize:        int64(gapSize),
			ExpectedEvents: int64(gapSize),
			Severity:       severity,
			PossibleCause:  "Events may have been lost during processing or storage",
			RepairAction:   "Investigate logs and backups for missing events",
		})
		gapCounter++
	}

	return gaps, nil
}