package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

func TestAuditRepository_Store(t *testing.T) {
	// Skip if no database connection
	db := testutil.NewTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db.Pool)
	ctx := context.Background()

	// Create test event
	event, err := audit.NewEvent(
		audit.EventCallInitiated,
		"user-123",
		"call-456",
		"initiate_call",
	)
	require.NoError(t, err)

	// Set additional fields
	event.ActorType = "user"
	event.TargetType = "call"
	event.SessionID = "session-789"
	event.CorrelationID = "corr-abc"
	event.ActorIP = "192.168.1.100"
	event.ActorAgent = "Mozilla/5.0"

	// Store event
	err = repo.Store(ctx, event)
	assert.NoError(t, err)

	// Verify event was stored
	retrieved, err := repo.GetByID(ctx, event.ID)
	require.NoError(t, err)
	assert.Equal(t, event.ID, retrieved.ID)
	assert.Equal(t, event.Type, retrieved.Type)
	assert.Equal(t, event.ActorID, retrieved.ActorID)
	assert.Equal(t, event.TargetID, retrieved.TargetID)
	assert.Equal(t, event.Action, retrieved.Action)
	assert.NotEmpty(t, retrieved.EventHash)
	assert.True(t, retrieved.SequenceNum > 0)
}

func TestAuditRepository_StoreBatch(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db.Pool)
	ctx := context.Background()

	// Create batch of events
	events := make([]*audit.Event, 5)
	for i := 0; i < 5; i++ {
		event, err := audit.NewEvent(
			audit.EventBidPlaced,
			"buyer-123",
			"call-789",
			"place_bid",
		)
		require.NoError(t, err)
		
		event.ActorType = "buyer"
		event.TargetType = "call"
		event.Metadata["bid_amount"] = 2.50 + float64(i)*0.25
		events[i] = event
	}

	// Store batch
	err := repo.StoreBatch(ctx, events)
	assert.NoError(t, err)

	// Verify all events were stored with sequential numbers
	for i, event := range events {
		retrieved, err := repo.GetByID(ctx, event.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, retrieved.EventHash)
		assert.NotEmpty(t, retrieved.PreviousHash)
		
		// Verify sequence numbers are sequential
		if i > 0 {
			assert.Equal(t, events[i-1].SequenceNum+1, event.SequenceNum)
		}
	}
}

func TestAuditRepository_GetBySequence(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db.Pool)
	ctx := context.Background()

	// Create and store event
	event, err := audit.NewEvent(
		audit.EventConsentGranted,
		"consumer-123",
		"business-456",
		"grant_consent",
	)
	require.NoError(t, err)

	err = repo.Store(ctx, event)
	require.NoError(t, err)

	// Get by sequence number
	seq, err := values.NewSequenceNumber(uint64(event.SequenceNum))
	require.NoError(t, err)

	retrieved, err := repo.GetBySequence(ctx, seq)
	require.NoError(t, err)
	assert.Equal(t, event.ID, retrieved.ID)
}

func TestAuditRepository_GetSequenceRange(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db.Pool)
	ctx := context.Background()

	// Store multiple events
	var firstSeq, lastSeq values.SequenceNumber
	for i := 0; i < 10; i++ {
		event, err := audit.NewEvent(
			audit.EventAPICall,
			"system",
			"api-endpoint",
			"api_call",
		)
		require.NoError(t, err)

		err = repo.Store(ctx, event)
		require.NoError(t, err)

		if i == 0 {
			firstSeq, _ = values.NewSequenceNumber(uint64(event.SequenceNum))
		}
		if i == 9 {
			lastSeq, _ = values.NewSequenceNumber(uint64(event.SequenceNum))
		}
	}

	// Get range
	events, err := repo.GetSequenceRange(ctx, firstSeq, lastSeq)
	require.NoError(t, err)
	assert.Len(t, events, 10)

	// Verify ordering
	for i := 1; i < len(events); i++ {
		assert.Greater(t, events[i].SequenceNum, events[i-1].SequenceNum)
	}
}

func TestAuditRepository_GetEvents_WithFilters(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db.Pool)
	ctx := context.Background()

	// Store events of different types
	eventTypes := []audit.EventType{
		audit.EventCallInitiated,
		audit.EventCallCompleted,
		audit.EventBidPlaced,
		audit.EventBidWon,
		audit.EventAuthSuccess,
		audit.EventAuthFailure,
	}

	for _, eventType := range eventTypes {
		event, err := audit.NewEvent(
			eventType,
			"actor-123",
			"target-456",
			"test_action",
		)
		require.NoError(t, err)

		if eventType == audit.EventAuthFailure {
			event.Severity = audit.SeverityWarning
		}

		err = repo.Store(ctx, event)
		require.NoError(t, err)
	}

	// Test type filter
	t.Run("FilterByType", func(t *testing.T) {
		filter := audit.EventFilter{
			Types: []audit.EventType{audit.EventCallInitiated, audit.EventCallCompleted},
			Limit: 10,
		}

		page, err := repo.GetEvents(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page.Events, 2)
		for _, event := range page.Events {
			assert.Contains(t, []audit.EventType{audit.EventCallInitiated, audit.EventCallCompleted}, event.Type)
		}
	})

	// Test severity filter
	t.Run("FilterBySeverity", func(t *testing.T) {
		filter := audit.EventFilter{
			Severities: []audit.Severity{audit.SeverityWarning},
			Limit:      10,
		}

		page, err := repo.GetEvents(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page.Events, 1)
		assert.Equal(t, audit.SeverityWarning, page.Events[0].Severity)
	})

	// Test actor filter
	t.Run("FilterByActor", func(t *testing.T) {
		filter := audit.EventFilter{
			ActorIDs: []string{"actor-123"},
			Limit:    10,
		}

		page, err := repo.GetEventsForActor(ctx, "actor-123", filter)
		require.NoError(t, err)
		assert.Greater(t, len(page.Events), 0)
		for _, event := range page.Events {
			assert.Equal(t, "actor-123", event.ActorID)
		}
	})
}

func TestAuditRepository_HashChainIntegrity(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db.Pool)
	ctx := context.Background()

	// Store a chain of events
	events := make([]*audit.Event, 5)
	for i := 0; i < 5; i++ {
		event, err := audit.NewEvent(
			audit.EventDataAccessed,
			"user-123",
			"resource-456",
			"read_data",
		)
		require.NoError(t, err)
		events[i] = event
	}

	err := repo.StoreBatch(ctx, events)
	require.NoError(t, err)

	// Verify individual event integrity
	for _, event := range events {
		result, err := repo.VerifyEventIntegrity(ctx, event.ID)
		require.NoError(t, err)
		assert.True(t, result.IsValid)
		assert.True(t, result.HashValid)
		assert.Empty(t, result.Errors)
	}

	// Verify chain integrity
	firstSeq, _ := values.NewSequenceNumber(uint64(events[0].SequenceNum))
	lastSeq, _ := values.NewSequenceNumber(uint64(events[4].SequenceNum))

	chainResult, err := repo.VerifyChainIntegrity(ctx, firstSeq, lastSeq)
	require.NoError(t, err)
	assert.True(t, chainResult.IsValid)
	assert.Empty(t, chainResult.Errors)
	assert.Equal(t, int64(5), chainResult.EventsChecked)
}

func TestAuditRepository_ComplianceEvents(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db.Pool)
	ctx := context.Background()

	// Create GDPR-relevant event
	gdprEvent, err := audit.NewEvent(
		audit.EventDataExported,
		"admin-123",
		"user-456",
		"export_user_data",
	)
	require.NoError(t, err)
	gdprEvent.ComplianceFlags["gdpr_relevant"] = true
	gdprEvent.ComplianceFlags["contains_pii"] = true
	gdprEvent.DataClasses = []string{"personal_data", "email"}

	err = repo.Store(ctx, gdprEvent)
	require.NoError(t, err)

	// Create TCPA-relevant event
	tcpaEvent, err := audit.NewEvent(
		audit.EventConsentGranted,
		"consumer-789",
		"business-123",
		"grant_call_consent",
	)
	require.NoError(t, err)
	tcpaEvent.ComplianceFlags["tcpa_relevant"] = true
	tcpaEvent.Metadata["phone_number"] = "+14155551234"

	err = repo.Store(ctx, tcpaEvent)
	require.NoError(t, err)

	// Test GDPR query
	t.Run("GDPREvents", func(t *testing.T) {
		filter := audit.EventFilter{Limit: 10}
		page, err := repo.GetGDPRRelevantEvents(ctx, "", filter)
		require.NoError(t, err)
		assert.Greater(t, len(page.Events), 0)
		
		found := false
		for _, event := range page.Events {
			if event.ID == gdprEvent.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	// Test TCPA query
	t.Run("TCPAEvents", func(t *testing.T) {
		filter := audit.EventFilter{Limit: 10}
		page, err := repo.GetTCPARelevantEvents(ctx, "+14155551234", filter)
		require.NoError(t, err)
		assert.Greater(t, len(page.Events), 0)
		
		found := false
		for _, event := range page.Events {
			if event.ID == tcpaEvent.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestAuditRepository_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	db := testutil.NewTestDB(t)
	defer db.Close()

	repo := NewAuditRepository(db.Pool)
	ctx := context.Background()

	// Measure single insert performance
	t.Run("SingleInsertPerformance", func(t *testing.T) {
		event, err := audit.NewEvent(
			audit.EventCallRouted,
			"router",
			"call-123",
			"route_call",
		)
		require.NoError(t, err)

		start := time.Now()
		err = repo.Store(ctx, event)
		elapsed := time.Since(start)

		assert.NoError(t, err)
		assert.Less(t, elapsed, 5*time.Millisecond, "Single insert should be < 5ms")
	})

	// Measure batch insert performance
	t.Run("BatchInsertPerformance", func(t *testing.T) {
		events := make([]*audit.Event, 100)
		for i := 0; i < 100; i++ {
			event, err := audit.NewEvent(
				audit.EventBidPlaced,
				"buyer-123",
				"call-456",
				"place_bid",
			)
			require.NoError(t, err)
			events[i] = event
		}

		start := time.Now()
		err := repo.StoreBatch(ctx, events)
		elapsed := time.Since(start)

		assert.NoError(t, err)
		perEventTime := elapsed / time.Duration(len(events))
		assert.Less(t, perEventTime, 5*time.Millisecond, "Batch insert should be < 5ms per event")
	})
}

// Helper function to create test database with audit schema
func createAuditSchema(t *testing.T, db *pgxpool.Pool) {
	ctx := context.Background()
	
	// Create sequence
	_, err := db.Exec(ctx, `CREATE SEQUENCE IF NOT EXISTS audit_events_sequence_number_seq`)
	require.NoError(t, err)

	// Create audit_events table with partitioning support
	_, err = db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS audit_events (
			id UUID PRIMARY KEY,
			sequence_number BIGINT NOT NULL UNIQUE DEFAULT nextval('audit_events_sequence_number_seq'),
			event_type VARCHAR(100) NOT NULL,
			severity VARCHAR(20) NOT NULL,
			actor_id VARCHAR(255) NOT NULL,
			actor_type VARCHAR(50),
			target_id VARCHAR(255) NOT NULL,
			target_type VARCHAR(50),
			action VARCHAR(100) NOT NULL,
			result VARCHAR(20) NOT NULL,
			metadata JSONB,
			compliance_flags JSONB,
			ip_address VARCHAR(45),
			user_agent TEXT,
			session_id VARCHAR(255),
			correlation_id VARCHAR(255),
			hash VARCHAR(64) NOT NULL,
			previous_hash VARCHAR(64),
			timestamp TIMESTAMPTZ NOT NULL,
			archived BOOLEAN DEFAULT FALSE,
			
			-- Indexes for performance
			CONSTRAINT audit_events_timestamp_idx CHECK (timestamp IS NOT NULL)
		) PARTITION BY RANGE (timestamp)`)
	require.NoError(t, err)

	// Create initial partition for current month
	currentMonth := time.Now().Format("2006_01")
	startOfMonth := time.Now().Format("2006-01-01")
	startOfNextMonth := time.Now().AddDate(0, 1, 0).Format("2006-01-01")
	
	_, err = db.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS audit_events_%s PARTITION OF audit_events
		FOR VALUES FROM ('%s') TO ('%s')`,
		currentMonth, startOfMonth, startOfNextMonth))
	require.NoError(t, err)

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events (timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_event_type ON audit_events (event_type)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_actor_id ON audit_events (actor_id)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_target_id ON audit_events (target_id)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_correlation_id ON audit_events (correlation_id)",
		"CREATE INDEX IF NOT EXISTS idx_audit_events_compliance_flags ON audit_events USING GIN (compliance_flags)",
	}

	for _, index := range indexes {
		_, err = db.Exec(ctx, index)
		require.NoError(t, err)
	}
}