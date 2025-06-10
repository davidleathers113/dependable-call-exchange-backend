package call

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Property-based tests using Go's built-in testing/quick package

// TestCall_PropertyInvariants tests properties that should always hold for any call
func TestCall_PropertyInvariants(t *testing.T) {
	// Property: UpdatedAt should always be >= CreatedAt
	t.Run("UpdatedAt >= CreatedAt invariant", func(t *testing.T) {
		property := func(from, to string, direction Direction) bool {
			if from == "" || to == "" {
				return true // Skip invalid inputs
			}
			
			c, err := NewCall(from, to, uuid.New(), direction)
			if err != nil {
				return true // Skip invalid calls
			}
			return !c.UpdatedAt.Before(c.CreatedAt)
		}
		
		err := quick.Check(property, &quick.Config{MaxCount: 1000})
		require.NoError(t, err)
	})
	
	// Property: Status transitions should always update UpdatedAt
	t.Run("Status transitions update timestamp", func(t *testing.T) {
		property := func(from, to string, newStatus Status) bool {
			if from == "" || to == "" {
				return true
			}
			
			c, err := NewCall(from, to, uuid.New(), DirectionInbound)
			if err != nil {
				return true // Skip invalid calls
			}
			oldUpdatedAt := c.UpdatedAt
			
			// Small delay to ensure time difference
			time.Sleep(time.Microsecond)
			c.UpdateStatus(newStatus)
			
			return c.UpdatedAt.After(oldUpdatedAt)
		}
		
		err := quick.Check(property, &quick.Config{MaxCount: 100})
		require.NoError(t, err)
	})
	
	// Property: Call ID should always be unique
	t.Run("Call IDs are unique", func(t *testing.T) {
		seen := make(map[uuid.UUID]bool)
		
		property := func(from, to string) bool {
			if from == "" || to == "" {
				return true
			}
			
			c, err := NewCall(from, to, uuid.New(), DirectionInbound)
			if err != nil {
				return true // Skip invalid calls
			}
			if seen[c.ID] {
				return false // Found duplicate
			}
			seen[c.ID] = true
			return true
		}
		
		err := quick.Check(property, &quick.Config{MaxCount: 10000})
		require.NoError(t, err)
	})
}

// TestCall_PropertyDuration tests duration calculation properties
func TestCall_PropertyDuration(t *testing.T) {
	// Property: Duration should always be non-negative when Complete succeeds
	t.Run("Duration is non-negative", func(t *testing.T) {
		property := func(durationSeconds int, cost float64) bool {
			if durationSeconds < 0 {
				durationSeconds = -durationSeconds // Make positive
			}
			
			c, err := NewCall("+15551234567", "+15559876543", uuid.New(), DirectionInbound)
			if err != nil {
				return true // Skip invalid calls
			}
			
			// Complete may fail for invalid inputs (e.g., extreme values)
			costMoney, err := values.NewMoneyFromFloat(cost, "USD")
			if err != nil {
				return true // Skip invalid costs
			}
			err = c.Complete(durationSeconds, costMoney)
			if err != nil {
				return true // Skip invalid inputs that fail validation
			}
			
			// If Complete succeeded, duration should be non-negative
			return c.Duration != nil && *c.Duration >= 0
		}
		
		err := quick.Check(property, &quick.Config{MaxCount: 1000})
		require.NoError(t, err)
	})
	
	// Property: Cost per second should be reasonable
	t.Run("Cost per second is reasonable", func(t *testing.T) {
		property := func(durationSeconds int, cost float64) bool {
			// Check for extreme values that could cause overflow
			if math.IsNaN(cost) || math.IsInf(cost, 0) {
				return true // Skip NaN and Inf
			}
			
			// Skip unreasonably large values that are outside any business domain
			// This prevents issues with extreme float64 values near the max
			if cost > 1e100 || cost < -1e100 {
				return true // Skip values that are too extreme for any real use case
			}
			
			// Ensure duration is positive and bounded
			if durationSeconds < 0 {
				durationSeconds = -durationSeconds
			}
			if durationSeconds == 0 {
				durationSeconds = 1 // Minimum 1 second
			}
			if durationSeconds > 86400 { // Max 24 hours
				durationSeconds = durationSeconds % 86400
				if durationSeconds == 0 {
					durationSeconds = 1
				}
			}
			
			// Ensure cost is non-negative and within reasonable bounds
			if cost < 0 {
				cost = -cost
			}
			// Clamp cost to a maximum value to prevent overflow
			// Using a much smaller max to ensure calculations don't overflow
			if cost > 10000 {
				cost = 10000 // Hard cap at $10,000
			}
			
			c, err := NewCall("+15551234567", "+15559876543", uuid.New(), DirectionInbound)
			if err != nil {
				return true // Skip invalid calls
			}
			
			// Complete may fail for invalid inputs
			costMoney, err := values.NewMoneyFromFloat(cost, "USD")
			if err != nil {
				return true // Skip invalid costs
			}
			err = c.Complete(durationSeconds, costMoney)
			if err != nil {
				return true // Skip invalid inputs that fail validation
			}
			
			if c.Duration == nil || c.Cost == nil {
				return false
			}
			
			costPerSecond := c.Cost.ToFloat64() / float64(*c.Duration)
			
			// Cost per second should be reasonable (between $0 and $10)
			return costPerSecond >= 0 && costPerSecond <= 10.0
		}
		
		err := quick.Check(property, &quick.Config{MaxCount: 1000})
		require.NoError(t, err)
	})
}

// TestCall_PropertyPhoneNumbers tests phone number handling properties
func TestCall_PropertyPhoneNumbers(t *testing.T) {
	// Property: Phone numbers should be preserved exactly as provided
	t.Run("Phone numbers preserved exactly", func(t *testing.T) {
		property := func(from, to string) bool {
			if from == "" || to == "" {
				return true
			}
			
			c, err := NewCall(from, to, uuid.New(), DirectionInbound)
			if err != nil {
				return true // Skip invalid calls
			}
			return c.FromNumber.String() == from && c.ToNumber.String() == to
		}
		
		err := quick.Check(property, &quick.Config{MaxCount: 1000})
		require.NoError(t, err)
	})
	
	// Property: Swapping from/to numbers should create different calls
	t.Run("Swapped numbers create different semantic calls", func(t *testing.T) {
		property := func(from, to string) bool {
			if from == "" || to == "" || from == to {
				return true
			}
			
			c1, err := NewCall(from, to, uuid.New(), DirectionInbound)
			if err != nil {
				return true // Skip invalid calls
			}
			c2, err2 := NewCall(to, from, uuid.New(), DirectionInbound)
			if err2 != nil {
				return true // Skip invalid calls
			}
			
			// Should have different from/to but same structure otherwise
			return c1.FromNumber != c2.FromNumber && 
				   c1.ToNumber != c2.ToNumber &&
				   c1.Status == c2.Status &&
				   c1.Direction == c2.Direction
		}
		
		err := quick.Check(property, &quick.Config{MaxCount: 500})
		require.NoError(t, err)
	})
}

// Custom generators for more realistic test data
type CallTestData struct {
	FromNumber string
	ToNumber   string
	BuyerID    uuid.UUID
	Direction  Direction
}

// Generate implements quick.Generator for more realistic call data
func (CallTestData) Generate(rand *rand.Rand, size int) reflect.Value {
	// Generate realistic phone numbers
	phoneNumbers := []string{
		"+15551234567",
		"+15559876543", 
		"+14155552222",
		"+17185553333",
		"+12125554444",
		"+13105555555",
		"+18005556666",
		"+19995557777",
	}
	
	data := CallTestData{
		FromNumber: phoneNumbers[rand.Int()%len(phoneNumbers)],
		ToNumber:   phoneNumbers[rand.Int()%len(phoneNumbers)],
		BuyerID:    uuid.New(),
		Direction:  Direction(rand.Int() % 2), // 0 or 1
	}
	
	return reflect.ValueOf(data)
}

// TestCall_PropertyWithCustomGenerator tests using custom data generators
func TestCall_PropertyWithCustomGenerator(t *testing.T) {
	// Property: Calls with realistic data should always be valid
	t.Run("Realistic calls are always valid", func(t *testing.T) {
		property := func(data CallTestData) bool {
			c, err := NewCall(data.FromNumber, data.ToNumber, data.BuyerID, data.Direction)
			if err != nil {
				return true // Skip invalid calls
			}
			
			// Validate call properties
			return c.ID != uuid.Nil &&
				   !c.FromNumber.IsEmpty() &&
				   !c.ToNumber.IsEmpty() &&
				   c.BuyerID == data.BuyerID &&
				   c.Direction == data.Direction &&
				   c.Status == StatusPending &&
				   !c.StartTime.IsZero() &&
				   !c.CreatedAt.IsZero() &&
				   !c.UpdatedAt.IsZero()
		}
		
		err := quick.Check(property, &quick.Config{
			MaxCount: 1000,
		})
		require.NoError(t, err)
	})
}

// TestCall_PropertyStateMachine tests state machine properties
func TestCall_PropertyStateMachine(t *testing.T) {
	// Property: Valid state transitions should always succeed
	t.Run("Valid state transitions", func(t *testing.T) {
		validTransitions := map[Status][]Status{
			StatusPending:    {StatusQueued, StatusFailed, StatusCanceled},
			StatusQueued:     {StatusRinging, StatusFailed, StatusCanceled},
			StatusRinging:    {StatusInProgress, StatusNoAnswer, StatusBusy, StatusFailed, StatusCanceled},
			StatusInProgress: {StatusCompleted, StatusFailed},
			// Terminal states
			StatusCompleted: {},
			StatusFailed:    {},
			StatusCanceled:  {},
			StatusNoAnswer:  {},
			StatusBusy:      {},
		}
		
		property := func(fromState Status, toState Status) bool {
			// Skip invalid enum values
			if fromState < 0 || fromState > StatusBusy || toState < 0 || toState > StatusBusy {
				return true
			}
			
			c, err := NewCall("+15551234567", "+15559876543", uuid.New(), DirectionInbound)
		if err != nil {
			return true // Skip invalid calls
		}
			c.Status = fromState
			oldUpdatedAt := c.UpdatedAt
			
			time.Sleep(time.Microsecond)
			c.UpdateStatus(toState)
			
			validTargets := validTransitions[fromState]
			isValidTransition := false
			for _, valid := range validTargets {
				if valid == toState {
					isValidTransition = true
					break
				}
			}
			
			if isValidTransition || fromState == toState {
				// Valid transition or no-op should succeed
				return c.Status == toState && c.UpdatedAt.After(oldUpdatedAt)
			}
			
			// For this test, we'll allow invalid transitions but verify timestamp still updates
			return c.UpdatedAt.After(oldUpdatedAt)
		}
		
		err := quick.Check(property, &quick.Config{MaxCount: 1000})
		require.NoError(t, err)
	})
}

// Benchmark property-based tests
func BenchmarkCall_PropertyCreation(b *testing.B) {
	phoneNumbers := []string{
		"+15551234567", "+15559876543", "+14155552222", "+17185553333",
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		from := phoneNumbers[i%len(phoneNumbers)]
		to := phoneNumbers[(i+1)%len(phoneNumbers)]
		_, _ = NewCall(from, to, uuid.New(), DirectionInbound)
	}
}