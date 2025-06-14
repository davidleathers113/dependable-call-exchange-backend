package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ChainVerifier provides hash chain integrity verification
// Following DCE patterns: interface for testability, concrete implementation
type ChainVerifier interface {
	// VerifySequential verifies a sequence of events maintains hash chain integrity
	VerifySequential(events []*Event) (*ChainVerificationResult, error)
	
	// VerifyEvent verifies a single event's hash is correct
	VerifyEvent(event *Event, expectedPreviousHash string) (bool, error)
	
	// ComputeChainHash computes the aggregate hash for a chain of events
	ComputeChainHash(events []*Event) (string, error)
	
	// DetectBreaks finds all hash chain breaks in a sequence
	DetectBreaks(events []*Event) ([]*ChainBreak, error)
}

// HashChainVerifier implements ChainVerifier
type HashChainVerifier struct {
	// Configuration for verification behavior
	allowEmptyChain    bool
	requireSequential  bool
	validateTimestamps bool
}

// NewHashChainVerifier creates a new hash chain verifier with default settings
func NewHashChainVerifier() *HashChainVerifier {
	return &HashChainVerifier{
		allowEmptyChain:    true,
		requireSequential:  true,
		validateTimestamps: true,
	}
}

// NewHashChainVerifierWithOptions creates a verifier with custom options
func NewHashChainVerifierWithOptions(allowEmpty, requireSeq, validateTime bool) *HashChainVerifier {
	return &HashChainVerifier{
		allowEmptyChain:    allowEmpty,
		requireSequential:  requireSeq,
		validateTimestamps: validateTime,
	}
}

// ChainVerificationResult contains the results of hash chain verification
type ChainVerificationResult struct {
	IsValid           bool          `json:"is_valid"`
	EventsVerified    int           `json:"events_verified"`
	ChainBreaks       []*ChainBreak `json:"chain_breaks,omitempty"`
	AggregateHash     string        `json:"aggregate_hash"`
	VerificationTime  time.Duration `json:"verification_time"`
	StartSequence     int64         `json:"start_sequence,omitempty"`
	EndSequence       int64         `json:"end_sequence,omitempty"`
	ErrorsEncountered []string      `json:"errors_encountered,omitempty"`
}

// ChainBreak represents a detected break in the hash chain
type ChainBreak struct {
	EventID          string    `json:"event_id"`
	SequenceNum      int64     `json:"sequence_num"`
	ExpectedHash     string    `json:"expected_hash"`
	ActualHash       string    `json:"actual_hash"`
	BreakType        BreakType `json:"break_type"`
	Description      string    `json:"description"`
	PreviousEventID  string    `json:"previous_event_id,omitempty"`
	TimestampIssue   bool      `json:"timestamp_issue"`
}

// BreakType categorizes the type of chain break
type BreakType string

const (
	BreakTypeHashMismatch     BreakType = "hash_mismatch"
	BreakTypeMissingPrevious  BreakType = "missing_previous"
	BreakTypeSequenceGap      BreakType = "sequence_gap"
	BreakTypeTimestampReverse BreakType = "timestamp_reverse"
	BreakTypeCorruptedEvent   BreakType = "corrupted_event"
)

// String returns the string representation of the break type
func (bt BreakType) String() string {
	return string(bt)
}

// VerifySequential verifies hash chain integrity for a sequence of events
func (v *HashChainVerifier) VerifySequential(events []*Event) (*ChainVerificationResult, error) {
	startTime := time.Now()
	
	result := &ChainVerificationResult{
		IsValid:           true,
		EventsVerified:    0,
		ChainBreaks:       make([]*ChainBreak, 0),
		ErrorsEncountered: make([]string, 0),
		VerificationTime:  0,
	}

	// Handle empty chain
	if len(events) == 0 {
		if !v.allowEmptyChain {
			return nil, errors.NewValidationError("EMPTY_CHAIN", 
				"empty event chain not allowed")
		}
		result.AggregateHash = ""
		result.VerificationTime = time.Since(startTime)
		return result, nil
	}

	// Sort events by sequence number if required
	if v.requireSequential {
		sortedEvents := make([]*Event, len(events))
		copy(sortedEvents, events)
		sort.Slice(sortedEvents, func(i, j int) bool {
			return sortedEvents[i].SequenceNum < sortedEvents[j].SequenceNum
		})
		events = sortedEvents
	}

	// Set sequence range
	if len(events) > 0 {
		result.StartSequence = events[0].SequenceNum
		result.EndSequence = events[len(events)-1].SequenceNum
	}

	// Verify each event in sequence
	var previousHash string
	var previousTimestamp time.Time
	
	for i, event := range events {
		result.EventsVerified++

		// Validate event structure
		if err := event.Validate(); err != nil {
			result.IsValid = false
			result.ErrorsEncountered = append(result.ErrorsEncountered, 
				fmt.Sprintf("Event %s validation failed: %v", event.ID, err))
			continue
		}

		// Check sequence continuity
		if v.requireSequential && i > 0 {
			expectedSeq := events[i-1].SequenceNum + 1
			if event.SequenceNum != expectedSeq {
				result.IsValid = false
				result.ChainBreaks = append(result.ChainBreaks, &ChainBreak{
					EventID:     event.ID.String(),
					SequenceNum: event.SequenceNum,
					BreakType:   BreakTypeSequenceGap,
					Description: fmt.Sprintf("Expected sequence %d, got %d", 
						expectedSeq, event.SequenceNum),
				})
			}
		}

		// Check timestamp ordering
		if v.validateTimestamps && i > 0 {
			if event.Timestamp.Before(previousTimestamp) {
				result.IsValid = false
				result.ChainBreaks = append(result.ChainBreaks, &ChainBreak{
					EventID:        event.ID.String(),
					SequenceNum:    event.SequenceNum,
					BreakType:      BreakTypeTimestampReverse,
					Description:    "Event timestamp is before previous event",
					TimestampIssue: true,
				})
			}
		}

		// Verify hash chain
		isValid, err := v.VerifyEvent(event, previousHash)
		if err != nil {
			result.IsValid = false
			result.ErrorsEncountered = append(result.ErrorsEncountered,
				fmt.Sprintf("Hash verification error for event %s: %v", event.ID, err))
			continue
		}

		if !isValid {
			result.IsValid = false
			result.ChainBreaks = append(result.ChainBreaks, &ChainBreak{
				EventID:         event.ID.String(),
				SequenceNum:     event.SequenceNum,
				ExpectedHash:    previousHash,
				ActualHash:      event.PreviousHash,
				BreakType:       BreakTypeHashMismatch,
				Description:     "Hash chain break detected",
				PreviousEventID: getPreviousEventID(events, i),
			})
		}

		// Update for next iteration
		previousHash = event.EventHash
		previousTimestamp = event.Timestamp
	}

	// Compute aggregate hash
	aggregateHash, err := v.ComputeChainHash(events)
	if err != nil {
		result.ErrorsEncountered = append(result.ErrorsEncountered,
			fmt.Sprintf("Failed to compute aggregate hash: %v", err))
	} else {
		result.AggregateHash = aggregateHash
	}

	result.VerificationTime = time.Since(startTime)
	return result, nil
}

// VerifyEvent verifies a single event's hash chain integrity
func (v *HashChainVerifier) VerifyEvent(event *Event, expectedPreviousHash string) (bool, error) {
	if event == nil {
		return false, errors.NewValidationError("NIL_EVENT", "event cannot be nil")
	}

	// Check if event is immutable and has a hash
	if !event.IsImmutable() || event.EventHash == "" {
		return false, errors.NewValidationError("EVENT_NOT_HASHED", 
			"event must be immutable with computed hash")
	}

	// Verify previous hash matches
	if event.PreviousHash != expectedPreviousHash {
		return false, nil
	}

	// Recompute hash to verify integrity
	eventCopy := event.Clone()
	eventCopy.PreviousHash = expectedPreviousHash
	eventCopy.EventHash = "" // Clear to recompute
	
	computedHash, err := eventCopy.ComputeHash(expectedPreviousHash)
	if err != nil {
		return false, errors.NewInternalError("failed to recompute hash").WithCause(err)
	}

	return computedHash == event.EventHash, nil
}

// ComputeChainHash computes an aggregate hash for the entire chain
func (v *HashChainVerifier) ComputeChainHash(events []*Event) (string, error) {
	if len(events) == 0 {
		return "", nil
	}

	// Create deterministic representation of the chain
	chainData := make([]string, len(events))
	for i, event := range events {
		chainData[i] = fmt.Sprintf("%d:%s:%s", 
			event.SequenceNum, event.ID.String(), event.EventHash)
	}

	// Sort to ensure deterministic order
	sort.Strings(chainData)

	// Concatenate and hash
	chainString := ""
	for _, data := range chainData {
		chainString += data + "|"
	}

	hash := sha256.Sum256([]byte(chainString))
	return hex.EncodeToString(hash[:]), nil
}

// DetectBreaks finds all hash chain breaks in a sequence
func (v *HashChainVerifier) DetectBreaks(events []*Event) ([]*ChainBreak, error) {
	result, err := v.VerifySequential(events)
	if err != nil {
		return nil, err
	}

	return result.ChainBreaks, nil
}

// Helper functions

// getPreviousEventID gets the ID of the previous event in the sequence
func getPreviousEventID(events []*Event, currentIndex int) string {
	if currentIndex == 0 || currentIndex >= len(events) {
		return ""
	}
	return events[currentIndex-1].ID.String()
}

// ChainStatistics provides statistical information about a hash chain
type ChainStatistics struct {
	TotalEvents      int           `json:"total_events"`
	ValidEvents      int           `json:"valid_events"`
	InvalidEvents    int           `json:"invalid_events"`
	ChainBreaks      int           `json:"chain_breaks"`
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	TimeSpan         time.Duration `json:"time_span"`
	EventTypes       map[EventType]int `json:"event_types"`
	SeverityBreakdown map[Severity]int `json:"severity_breakdown"`
	CategoryBreakdown map[Category]int `json:"category_breakdown"`
}

// ComputeChainStatistics computes statistics for a chain of events
func ComputeChainStatistics(events []*Event) *ChainStatistics {
	stats := &ChainStatistics{
		TotalEvents:       len(events),
		EventTypes:        make(map[EventType]int),
		SeverityBreakdown: make(map[Severity]int),
		CategoryBreakdown: make(map[Category]int),
	}

	if len(events) == 0 {
		return stats
	}

	// Initialize times
	stats.StartTime = events[0].Timestamp
	stats.EndTime = events[0].Timestamp

	// Analyze events
	for _, event := range events {
		// Update time range
		if event.Timestamp.Before(stats.StartTime) {
			stats.StartTime = event.Timestamp
		}
		if event.Timestamp.After(stats.EndTime) {
			stats.EndTime = event.Timestamp
		}

		// Count by type
		stats.EventTypes[event.Type]++

		// Count by severity
		stats.SeverityBreakdown[event.Severity]++

		// Count by category
		category := Category(event.Category)
		stats.CategoryBreakdown[category]++

		// Check validity (basic validation)
		if err := event.Validate(); err == nil {
			stats.ValidEvents++
		} else {
			stats.InvalidEvents++
		}
	}

	stats.TimeSpan = stats.EndTime.Sub(stats.StartTime)
	return stats
}

// VerifyChainIntegrity is a convenience function for full chain verification
func VerifyChainIntegrity(events []*Event) (*ChainVerificationResult, error) {
	verifier := NewHashChainVerifier()
	return verifier.VerifySequential(events)
}

// QuickVerifyChain performs a fast integrity check (less comprehensive)
func QuickVerifyChain(events []*Event) (bool, error) {
	if len(events) == 0 {
		return true, nil
	}

	var previousHash string
	for _, event := range events {
		if event.PreviousHash != previousHash {
			return false, nil
		}
		if !event.IsImmutable() || event.EventHash == "" {
			return false, errors.NewValidationError("INVALID_EVENT", 
				"event must be immutable with hash")
		}
		previousHash = event.EventHash
	}

	return true, nil
}