package audit

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHashChainVerifier tests verifier creation
func TestNewHashChainVerifier(t *testing.T) {
	t.Run("default verifier", func(t *testing.T) {
		verifier := NewHashChainVerifier()
		assert.NotNil(t, verifier)
		assert.True(t, verifier.allowEmptyChain)
		assert.True(t, verifier.requireSequential)
		assert.True(t, verifier.validateTimestamps)
	})

	t.Run("custom options verifier", func(t *testing.T) {
		verifier := NewHashChainVerifierWithOptions(false, false, false)
		assert.NotNil(t, verifier)
		assert.False(t, verifier.allowEmptyChain)
		assert.False(t, verifier.requireSequential)
		assert.False(t, verifier.validateTimestamps)
	})
}

// TestHashChainVerifierEmptyChain tests empty chain handling
func TestHashChainVerifierEmptyChain(t *testing.T) {
	t.Run("empty chain allowed", func(t *testing.T) {
		verifier := NewHashChainVerifier()
		result, err := verifier.VerifySequential([]*Event{})
		
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsValid)
		assert.Equal(t, 0, result.EventsVerified)
		assert.Empty(t, result.ChainBreaks)
		assert.Empty(t, result.AggregateHash)
		assert.Len(t, result.ErrorsEncountered, 0)
	})

	t.Run("empty chain not allowed", func(t *testing.T) {
		verifier := NewHashChainVerifierWithOptions(false, true, true)
		_, err := verifier.VerifySequential([]*Event{})
		
		require.Error(t, err)
		var appErr *errors.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "EMPTY_CHAIN", appErr.Code)
	})
}

// TestHashChainVerifierSingleEvent tests single event chain
func TestHashChainVerifierSingleEvent(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	// Create and hash single event
	event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
	require.NoError(t, err)
	
	event.SequenceNum = 1
	_, err = event.ComputeHash("")
	require.NoError(t, err)
	
	result, err := verifier.VerifySequential([]*Event{event})
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsValid)
	assert.Equal(t, 1, result.EventsVerified)
	assert.Len(t, result.ChainBreaks, 0)
	assert.NotEmpty(t, result.AggregateHash)
	assert.Equal(t, int64(1), result.StartSequence)
	assert.Equal(t, int64(1), result.EndSequence)
}

// TestHashChainVerifierValidSequence tests valid sequential events
func TestHashChainVerifierValidSequence(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	// Create chain of 5 events
	events := createValidEventChain(t, 5)
	
	result, err := verifier.VerifySequential(events)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsValid)
	assert.Equal(t, 5, result.EventsVerified)
	assert.Len(t, result.ChainBreaks, 0)
	assert.NotEmpty(t, result.AggregateHash)
	assert.Equal(t, int64(1), result.StartSequence)
	assert.Equal(t, int64(5), result.EndSequence)
	assert.Len(t, result.ErrorsEncountered, 0)
	assert.Greater(t, result.VerificationTime, time.Duration(0))
}

// TestHashChainVerifierHashMismatch tests hash chain breaks
func TestHashChainVerifierHashMismatch(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	// Create valid chain and break it
	events := createValidEventChain(t, 3)
	
	// Corrupt the second event's previous hash
	events[1].PreviousHash = "corrupted-hash"
	
	result, err := verifier.VerifySequential(events)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsValid)
	assert.Equal(t, 3, result.EventsVerified)
	assert.Len(t, result.ChainBreaks, 1)
	
	chainBreak := result.ChainBreaks[0]
	assert.Equal(t, events[1].ID.String(), chainBreak.EventID)
	assert.Equal(t, int64(2), chainBreak.SequenceNum)
	assert.Equal(t, BreakTypeHashMismatch, chainBreak.BreakType)
	assert.Contains(t, chainBreak.Description, "Hash chain break detected")
}

// TestHashChainVerifierSequenceGap tests sequence number gaps
func TestHashChainVerifierSequenceGap(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	// Create valid chain with sequence gap
	events := createValidEventChain(t, 3)
	events[1].SequenceNum = 5 // Should be 2, creating a gap
	
	result, err := verifier.VerifySequential(events)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsValid)
	assert.Len(t, result.ChainBreaks, 1)
	
	chainBreak := result.ChainBreaks[0]
	assert.Equal(t, events[1].ID.String(), chainBreak.EventID)
	assert.Equal(t, int64(5), chainBreak.SequenceNum)
	assert.Equal(t, BreakTypeSequenceGap, chainBreak.BreakType)
	assert.Contains(t, chainBreak.Description, "Expected sequence 2, got 5")
}

// TestHashChainVerifierTimestampReverse tests reverse timestamp detection
func TestHashChainVerifierTimestampReverse(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	// Create valid chain with reverse timestamp
	events := createValidEventChain(t, 3)
	
	// Make second event timestamp earlier than first
	events[1].Timestamp = events[0].Timestamp.Add(-1 * time.Minute)
	
	result, err := verifier.VerifySequential(events)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsValid)
	assert.Len(t, result.ChainBreaks, 1)
	
	chainBreak := result.ChainBreaks[0]
	assert.Equal(t, events[1].ID.String(), chainBreak.EventID)
	assert.Equal(t, BreakTypeTimestampReverse, chainBreak.BreakType)
	assert.True(t, chainBreak.TimestampIssue)
}

// TestHashChainVerifierInvalidEvent tests invalid event handling
func TestHashChainVerifierInvalidEvent(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	// Create valid chain with one invalid event
	events := createValidEventChain(t, 3)
	
	// Make middle event invalid
	events[1].ActorID = "" // Invalid
	
	result, err := verifier.VerifySequential(events)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsValid)
	assert.Equal(t, 3, result.EventsVerified)
	assert.Len(t, result.ErrorsEncountered, 1)
	assert.Contains(t, result.ErrorsEncountered[0], "validation failed")
}

// TestVerifyEvent tests single event verification
func TestVerifyEvent(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	t.Run("valid event verification", func(t *testing.T) {
		event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		require.NoError(t, err)
		
		prevHash := "previous-hash-123"
		_, err = event.ComputeHash(prevHash)
		require.NoError(t, err)
		
		isValid, err := verifier.VerifyEvent(event, prevHash)
		require.NoError(t, err)
		assert.True(t, isValid)
	})
	
	t.Run("nil event", func(t *testing.T) {
		_, err := verifier.VerifyEvent(nil, "prev-hash")
		require.Error(t, err)
		
		var appErr *errors.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "NIL_EVENT", appErr.Code)
	})
	
	t.Run("event not immutable", func(t *testing.T) {
		event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		require.NoError(t, err)
		
		_, err = verifier.VerifyEvent(event, "prev-hash")
		require.Error(t, err)
		
		var appErr *errors.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "EVENT_NOT_HASHED", appErr.Code)
	})
	
	t.Run("previous hash mismatch", func(t *testing.T) {
		event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		require.NoError(t, err)
		
		_, err = event.ComputeHash("different-hash")
		require.NoError(t, err)
		
		isValid, err := verifier.VerifyEvent(event, "expected-hash")
		require.NoError(t, err)
		assert.False(t, isValid)
	})
}

// TestComputeChainHash tests aggregate hash computation
func TestComputeChainHash(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	t.Run("empty chain hash", func(t *testing.T) {
		hash, err := verifier.ComputeChainHash([]*Event{})
		require.NoError(t, err)
		assert.Empty(t, hash)
	})
	
	t.Run("single event hash", func(t *testing.T) {
		events := createValidEventChain(t, 1)
		hash, err := verifier.ComputeChainHash(events)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Len(t, hash, 64) // SHA-256 hex string
	})
	
	t.Run("multiple events hash deterministic", func(t *testing.T) {
		events1 := createValidEventChain(t, 3)
		events2 := createValidEventChain(t, 3)
		
		// Make events identical
		for i := range events2 {
			events2[i].ID = events1[i].ID
			events2[i].Timestamp = events1[i].Timestamp
			events2[i].TimestampNano = events1[i].TimestampNano
			events2[i].EventHash = events1[i].EventHash
		}
		
		hash1, err := verifier.ComputeChainHash(events1)
		require.NoError(t, err)
		
		hash2, err := verifier.ComputeChainHash(events2)
		require.NoError(t, err)
		
		assert.Equal(t, hash1, hash2)
	})
}

// TestComputeChainStatistics tests chain statistics computation
func TestComputeChainStatistics(t *testing.T) {
	t.Run("empty chain stats", func(t *testing.T) {
		stats := ComputeChainStatistics([]*Event{})
		assert.Equal(t, 0, stats.TotalEvents)
		assert.Equal(t, 0, stats.ValidEvents)
		assert.Equal(t, 0, stats.InvalidEvents)
		assert.NotNil(t, stats.EventTypes)
		assert.NotNil(t, stats.SeverityBreakdown)
		assert.NotNil(t, stats.CategoryBreakdown)
	})
	
	t.Run("valid chain stats", func(t *testing.T) {
		events := createValidEventChain(t, 5)
		
		// Add variety to event types and severities
		events[1].Type = EventBidPlaced
		events[1].Severity = SeverityWarning
		events[2].Type = EventConsentGranted
		events[2].Severity = SeverityError
		
		stats := ComputeChainStatistics(events)
		
		assert.Equal(t, 5, stats.TotalEvents)
		assert.Equal(t, 5, stats.ValidEvents) // All should be valid
		assert.Equal(t, 0, stats.InvalidEvents)
		assert.Equal(t, 3, stats.EventTypes[EventCallInitiated]) // 3 call events
		assert.Equal(t, 1, stats.EventTypes[EventBidPlaced])
		assert.Equal(t, 1, stats.EventTypes[EventConsentGranted])
		assert.Equal(t, 3, stats.SeverityBreakdown[SeverityInfo]) // Default severity
		assert.Equal(t, 1, stats.SeverityBreakdown[SeverityWarning])
		assert.Equal(t, 1, stats.SeverityBreakdown[SeverityError])
		
		assert.True(t, stats.EndTime.After(stats.StartTime) || stats.EndTime.Equal(stats.StartTime))
		assert.GreaterOrEqual(t, stats.TimeSpan, time.Duration(0))
	})
	
	t.Run("chain with invalid events", func(t *testing.T) {
		events := createValidEventChain(t, 3)
		
		// Make one event invalid
		events[1].ActorID = ""
		
		stats := ComputeChainStatistics(events)
		
		assert.Equal(t, 3, stats.TotalEvents)
		assert.Equal(t, 2, stats.ValidEvents)
		assert.Equal(t, 1, stats.InvalidEvents)
	})
}

// TestQuickVerifyChain tests quick verification
func TestQuickVerifyChain(t *testing.T) {
	t.Run("empty chain", func(t *testing.T) {
		isValid, err := QuickVerifyChain([]*Event{})
		require.NoError(t, err)
		assert.True(t, isValid)
	})
	
	t.Run("valid chain", func(t *testing.T) {
		events := createValidEventChain(t, 3)
		
		isValid, err := QuickVerifyChain(events)
		require.NoError(t, err)
		assert.True(t, isValid)
	})
	
	t.Run("broken chain", func(t *testing.T) {
		events := createValidEventChain(t, 3)
		events[1].PreviousHash = "corrupted"
		
		isValid, err := QuickVerifyChain(events)
		require.NoError(t, err)
		assert.False(t, isValid)
	})
	
	t.Run("event without hash", func(t *testing.T) {
		event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		require.NoError(t, err)
		
		_, err = QuickVerifyChain([]*Event{event})
		require.Error(t, err)
		
		var appErr *errors.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "INVALID_EVENT", appErr.Code)
	})
}

// TestVerifyChainIntegrity tests convenience function
func TestVerifyChainIntegrity(t *testing.T) {
	events := createValidEventChain(t, 3)
	
	result, err := VerifyChainIntegrity(events)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsValid)
	assert.Equal(t, 3, result.EventsVerified)
}

// Property-based tests for hash chain verification

// TestPropertyHashChainIntegrityPreservedUnderValidOperations tests that valid operations preserve integrity
func TestPropertyHashChainIntegrityPreservedUnderValidOperations(t *testing.T) {
	for i := 0; i < 1000; i++ {
		// Create random length chain (1-10 events)
		chainLength := rand.Intn(10) + 1
		events := createValidEventChain(t, chainLength)
		
		// Original chain should be valid
		result, err := VerifyChainIntegrity(events)
		require.NoError(t, err, "iteration %d: chain verification failed", i)
		require.True(t, result.IsValid, "iteration %d: chain should be valid", i)
		
		// Test various valid operations that should preserve integrity
		operation := rand.Intn(3)
		switch operation {
		case 0:
			// Reorder events (should still be valid if we disable sequential requirement)
			verifier := NewHashChainVerifierWithOptions(true, false, false)
			result, err = verifier.VerifySequential(events)
			require.NoError(t, err, "iteration %d: reorder verification failed", i)
			
		case 1:
			// Clone all events (should maintain validity)
			clonedEvents := make([]*Event, len(events))
			for j, event := range events {
				clonedEvents[j] = event.Clone()
				clonedEvents[j].ID = event.ID
				clonedEvents[j].EventHash = event.EventHash
				clonedEvents[j].PreviousHash = event.PreviousHash
				clonedEvents[j].immutable = true
			}
			result, err = VerifyChainIntegrity(clonedEvents)
			require.NoError(t, err, "iteration %d: clone verification failed", i)
			require.True(t, result.IsValid, "iteration %d: cloned chain should be valid", i)
			
		case 2:
			// Subset of events (if we take a continuous subsequence)
			if len(events) > 1 {
				start := rand.Intn(len(events))
				end := start + rand.Intn(len(events)-start) + 1
				subset := events[start:end]
				
				// Adjust hashes for subset to be valid
				if len(subset) > 0 {
					// First event in subset has no previous hash in this context
					subset[0] = subset[0].Clone()
					subset[0].PreviousHash = ""
					hash, err := subset[0].ComputeHash("")
					require.NoError(t, err, "iteration %d: subset hash failed", i)
					
					// Recompute subsequent hashes
					prevHash := hash
					for j := 1; j < len(subset); j++ {
						subset[j] = subset[j].Clone()
						hash, err = subset[j].ComputeHash(prevHash)
						require.NoError(t, err, "iteration %d: subset hash %d failed", i, j)
						prevHash = hash
					}
				}
				
				result, err = VerifyChainIntegrity(subset)
				require.NoError(t, err, "iteration %d: subset verification failed", i)
				require.True(t, result.IsValid, "iteration %d: subset chain should be valid", i)
			}
		}
	}
}

// TestPropertyHashChainDetectsAllCorruption tests that all types of corruption are detected
func TestPropertyHashChainDetectsAllCorruption(t *testing.T) {
	corruptionTypes := []struct {
		name      string
		corruptFn func([]*Event) []*Event
		breakType BreakType
	}{
		{
			name: "hash corruption",
			corruptFn: func(events []*Event) []*Event {
				if len(events) > 1 {
					events[1].PreviousHash = "corrupted-hash"
				}
				return events
			},
			breakType: BreakTypeHashMismatch,
		},
		{
			name: "sequence gap",
			corruptFn: func(events []*Event) []*Event {
				if len(events) > 1 {
					events[1].SequenceNum = events[0].SequenceNum + 10 // Create gap
				}
				return events
			},
			breakType: BreakTypeSequenceGap,
		},
		{
			name: "timestamp reversal",
			corruptFn: func(events []*Event) []*Event {
				if len(events) > 1 {
					events[1].Timestamp = events[0].Timestamp.Add(-1 * time.Hour)
				}
				return events
			},
			breakType: BreakTypeTimestampReverse,
		},
	}
	
	for i := 0; i < 1000; i++ {
		// Create valid chain
		chainLength := rand.Intn(5) + 2 // At least 2 events for corruption
		events := createValidEventChain(t, chainLength)
		
		// Randomly select corruption type
		corruption := corruptionTypes[rand.Intn(len(corruptionTypes))]
		
		// Apply corruption
		corruptedEvents := corruption.corruptFn(events)
		
		// Verify corruption is detected
		result, err := VerifyChainIntegrity(corruptedEvents)
		require.NoError(t, err, "iteration %d (%s): verification failed", i, corruption.name)
		assert.False(t, result.IsValid, "iteration %d (%s): corrupted chain should be invalid", i, corruption.name)
		
		// Check that the correct type of break is detected
		foundExpectedBreak := false
		for _, chainBreak := range result.ChainBreaks {
			if chainBreak.BreakType == corruption.breakType {
				foundExpectedBreak = true
				break
			}
		}
		assert.True(t, foundExpectedBreak, "iteration %d (%s): expected break type %s not found", i, corruption.name, corruption.breakType)
	}
}

// TestPropertyHashChainComputationIsDeterministic tests deterministic hash computation
func TestPropertyHashChainComputationIsDeterministic(t *testing.T) {
	verifier := NewHashChainVerifier()
	
	for i := 0; i < 1000; i++ {
		// Create random chain
		chainLength := rand.Intn(10) + 1
		events1 := createValidEventChain(t, chainLength)
		events2 := make([]*Event, len(events1))
		
		// Create identical second chain
		for j := range events1 {
			events2[j] = events1[j].Clone()
			events2[j].ID = events1[j].ID
			events2[j].Timestamp = events1[j].Timestamp
			events2[j].TimestampNano = events1[j].TimestampNano
			events2[j].EventHash = events1[j].EventHash
			events2[j].PreviousHash = events1[j].PreviousHash
			events2[j].SequenceNum = events1[j].SequenceNum
			events2[j].immutable = true
		}
		
		// Compute aggregate hashes
		hash1, err := verifier.ComputeChainHash(events1)
		require.NoError(t, err, "iteration %d: first hash computation failed", i)
		
		hash2, err := verifier.ComputeChainHash(events2)
		require.NoError(t, err, "iteration %d: second hash computation failed", i)
		
		// Should be identical
		assert.Equal(t, hash1, hash2, "iteration %d: aggregate hashes differ", i)
	}
}

// TestPropertyHashChainVerificationIsConsistent tests consistency across verifier instances
func TestPropertyHashChainVerificationIsConsistent(t *testing.T) {
	for i := 0; i < 1000; i++ {
		// Create random chain
		chainLength := rand.Intn(10) + 1
		events := createValidEventChain(t, chainLength)
		
		// Create multiple verifiers with same config
		verifier1 := NewHashChainVerifier()
		verifier2 := NewHashChainVerifier()
		verifier3 := NewHashChainVerifierWithOptions(true, true, true) // Same as default
		
		// All should give same result
		result1, err1 := verifier1.VerifySequential(events)
		result2, err2 := verifier2.VerifySequential(events)
		result3, err3 := verifier3.VerifySequential(events)
		
		require.NoError(t, err1, "iteration %d: verifier1 failed", i)
		require.NoError(t, err2, "iteration %d: verifier2 failed", i)
		require.NoError(t, err3, "iteration %d: verifier3 failed", i)
		
		assert.Equal(t, result1.IsValid, result2.IsValid, "iteration %d: validity differs", i)
		assert.Equal(t, result1.IsValid, result3.IsValid, "iteration %d: validity differs", i)
		assert.Equal(t, result1.EventsVerified, result2.EventsVerified, "iteration %d: events verified differs", i)
		assert.Equal(t, result1.EventsVerified, result3.EventsVerified, "iteration %d: events verified differs", i)
		assert.Equal(t, len(result1.ChainBreaks), len(result2.ChainBreaks), "iteration %d: chain breaks count differs", i)
		assert.Equal(t, len(result1.ChainBreaks), len(result3.ChainBreaks), "iteration %d: chain breaks count differs", i)
	}
}

// TestPropertyChainStatisticsInvariants tests statistical invariants
func TestPropertyChainStatisticsInvariants(t *testing.T) {
	for i := 0; i < 1000; i++ {
		// Create random chain
		chainLength := rand.Intn(20) + 1
		events := createValidEventChain(t, chainLength)
		
		// Add random variety
		for j := range events {
			if rand.Float32() < 0.3 {
				eventTypes := []EventType{EventBidPlaced, EventConsentGranted, EventDataAccessed}
				events[j].Type = eventTypes[rand.Intn(len(eventTypes))]
			}
			if rand.Float32() < 0.2 {
				severities := []Severity{SeverityWarning, SeverityError, SeverityCritical}
				events[j].Severity = severities[rand.Intn(len(severities))]
			}
			// Randomly invalidate some events
			if rand.Float32() < 0.1 {
				events[j].ActorID = ""
			}
		}
		
		stats := ComputeChainStatistics(events)
		
		// Test invariants
		assert.Equal(t, len(events), stats.TotalEvents, "iteration %d: total events mismatch", i)
		assert.Equal(t, stats.TotalEvents, stats.ValidEvents+stats.InvalidEvents, "iteration %d: valid+invalid != total", i)
		assert.GreaterOrEqual(t, stats.ValidEvents, 0, "iteration %d: valid events cannot be negative", i)
		assert.GreaterOrEqual(t, stats.InvalidEvents, 0, "iteration %d: invalid events cannot be negative", i)
		
		// Time invariants
		if len(events) > 0 {
			assert.True(t, stats.EndTime.After(stats.StartTime) || stats.EndTime.Equal(stats.StartTime), "iteration %d: end time before start time", i)
			assert.GreaterOrEqual(t, stats.TimeSpan, time.Duration(0), "iteration %d: negative time span", i)
		}
		
		// Event type count should match total
		totalFromTypes := 0
		for _, count := range stats.EventTypes {
			totalFromTypes += count
		}
		assert.Equal(t, stats.TotalEvents, totalFromTypes, "iteration %d: event type counts don't sum to total", i)
		
		// Severity count should match total
		totalFromSeverities := 0
		for _, count := range stats.SeverityBreakdown {
			totalFromSeverities += count
		}
		assert.Equal(t, stats.TotalEvents, totalFromSeverities, "iteration %d: severity counts don't sum to total", i)
	}
}

// Benchmark tests

// BenchmarkHashChainVerification benchmarks chain verification performance
func BenchmarkHashChainVerification(b *testing.B) {
	chainSizes := []int{10, 50, 100, 500, 1000}
	
	for _, size := range chainSizes {
		b.Run(fmt.Sprintf("chain_size_%d", size), func(b *testing.B) {
			events := createValidEventChain(nil, size)
			verifier := NewHashChainVerifier()
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := verifier.VerifySequential(events)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkQuickVerifyChain benchmarks quick verification performance
func BenchmarkQuickVerifyChain(b *testing.B) {
	events := createValidEventChain(nil, 100)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := QuickVerifyChain(events)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkComputeChainHash benchmarks aggregate hash computation
func BenchmarkComputeChainHash(b *testing.B) {
	events := createValidEventChain(nil, 100)
	verifier := NewHashChainVerifier()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := verifier.ComputeChainHash(events)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkChainStatistics benchmarks statistics computation
func BenchmarkChainStatistics(b *testing.B) {
	events := createValidEventChain(nil, 1000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ComputeChainStatistics(events)
	}
}

// Helper functions

// createValidEventChain creates a valid hash chain of specified length
func createValidEventChain(t testing.TB, length int) []*Event {
	events := make([]*Event, length)
	previousHash := ""
	
	baseTime := time.Now().UTC()
	
	for i := 0; i < length; i++ {
		event, err := NewEvent(EventCallInitiated, fmt.Sprintf("actor-%d", i), fmt.Sprintf("target-%d", i), fmt.Sprintf("action-%d", i))
		if err != nil && t != nil {
			t.Fatalf("failed to create event %d: %v", i, err)
		}
		
		event.SequenceNum = int64(i + 1)
		event.Timestamp = baseTime.Add(time.Duration(i) * time.Millisecond)
		event.TimestampNano = event.Timestamp.UnixNano()
		
		hash, err := event.ComputeHash(previousHash)
		if err != nil && t != nil {
			t.Fatalf("failed to compute hash for event %d: %v", i, err)
		}
		
		events[i] = event
		previousHash = hash
	}
	
	return events
}

// createBreakType creates a specific type of chain break for testing
func createChainWithBreak(t testing.TB, breakType BreakType, breakIndex int) []*Event {
	events := createValidEventChain(t, 5)
	
	if breakIndex >= len(events) {
		return events
	}
	
	switch breakType {
	case BreakTypeHashMismatch:
		events[breakIndex].PreviousHash = "corrupted-hash"
	case BreakTypeSequenceGap:
		events[breakIndex].SequenceNum = events[breakIndex].SequenceNum + 10
	case BreakTypeTimestampReverse:
		if breakIndex > 0 {
			events[breakIndex].Timestamp = events[breakIndex-1].Timestamp.Add(-1 * time.Minute)
		}
	case BreakTypeCorruptedEvent:
		events[breakIndex].ActorID = "" // Make invalid
	}
	
	return events
}

// assertChainBreakType asserts that a specific break type exists in the results
func assertChainBreakType(t testing.TB, result *ChainVerificationResult, breakType BreakType) {
	found := false
	for _, chainBreak := range result.ChainBreaks {
		if chainBreak.BreakType == breakType {
			found = true
			break
		}
	}
	
	if t != nil {
		assert.True(t.(*testing.T), found, "expected break type %s not found", breakType)
	}
}

// assertValidChainResult asserts that a chain verification result indicates a valid chain
func assertValidChainResult(t testing.TB, result *ChainVerificationResult, expectedEvents int) {
	if t != nil {
		tt := t.(*testing.T)
		assert.True(tt, result.IsValid, "chain should be valid")
		assert.Equal(tt, expectedEvents, result.EventsVerified, "events verified count mismatch")
		assert.Len(tt, result.ChainBreaks, 0, "should have no chain breaks")
		assert.Len(tt, result.ErrorsEncountered, 0, "should have no errors")
		assert.NotEmpty(tt, result.AggregateHash, "should have aggregate hash")
		assert.Greater(tt, result.VerificationTime, time.Duration(0), "should have positive verification time")
	}
}

// generateRandomEventChain generates a random event chain for property testing
func generateRandomEventChain(length int, corruptionRate float32) []*Event {
	events := createValidEventChain(nil, length)
	
	// Randomly corrupt events
	for i := range events {
		if rand.Float32() < corruptionRate {
			corruptionType := rand.Intn(3)
			switch corruptionType {
			case 0:
				events[i].ActorID = "" // Invalid event
			case 1:
				if i > 0 {
					events[i].PreviousHash = "corrupted"
				}
			case 2:
				if i > 0 {
					events[i].SequenceNum = events[i-1].SequenceNum + 10
				}
			}
		}
	}
	
	return events
}