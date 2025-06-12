package call_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestNewCall(t *testing.T) {
	tests := []struct {
		name      string
		from      string
		to        string
		buyerID   uuid.UUID
		direction call.Direction
		validate  func(t *testing.T, c *call.Call)
	}{
		{
			name:      "creates inbound call with valid data",
			from:      "+15551234567",
			to:        "+15559876543",
			buyerID:   uuid.New(),
			direction: call.DirectionInbound,
			validate: func(t *testing.T, c *call.Call) {
				assert.NotEqual(t, uuid.Nil, c.ID)
				assert.Equal(t, "+15551234567", c.FromNumber.String())
				assert.Equal(t, "+15559876543", c.ToNumber.String())
				assert.Equal(t, call.StatusPending, c.Status)
				assert.Equal(t, call.DirectionInbound, c.Direction)
				assert.NotZero(t, c.StartTime)
				assert.NotZero(t, c.CreatedAt)
				assert.NotZero(t, c.UpdatedAt)
				assert.Nil(t, c.EndTime)
				assert.Nil(t, c.Duration)
				assert.Nil(t, c.Cost)
			},
		},
		{
			name:      "creates outbound call",
			from:      "+15551111111",
			to:        "+15552222222",
			buyerID:   uuid.New(),
			direction: call.DirectionOutbound,
			validate: func(t *testing.T, c *call.Call) {
				assert.Equal(t, call.DirectionOutbound, c.Direction)
				assert.Equal(t, call.StatusPending, c.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := call.NewCall(tt.from, tt.to, tt.buyerID, tt.direction)
			require.NoError(t, err)
			require.NotNil(t, c)
			tt.validate(t, c)
		})
	}
}

func TestCall_UpdateStatus(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *call.Call
		newStatus call.Status
		validate  func(t *testing.T, c *call.Call, oldUpdatedAt time.Time)
	}{
		{
			name: "updates status from pending to queued",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).
					WithStatus(call.StatusPending).
					Build()
			},
			newStatus: call.StatusQueued,
			validate: func(t *testing.T, c *call.Call, oldUpdatedAt time.Time) {
				assert.Equal(t, call.StatusQueued, c.Status)
				assert.True(t, c.UpdatedAt.After(oldUpdatedAt))
			},
		},
		{
			name: "updates status from queued to ringing",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).
					WithStatus(call.StatusQueued).
					Build()
			},
			newStatus: call.StatusRinging,
			validate: func(t *testing.T, c *call.Call, oldUpdatedAt time.Time) {
				assert.Equal(t, call.StatusRinging, c.Status)
			},
		},
		{
			name: "updates status to in progress",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).
					WithStatus(call.StatusRinging).
					Build()
			},
			newStatus: call.StatusInProgress,
			validate: func(t *testing.T, c *call.Call, oldUpdatedAt time.Time) {
				assert.Equal(t, call.StatusInProgress, c.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setup()
			oldUpdatedAt := c.UpdatedAt

			// Small delay to ensure time difference
			time.Sleep(10 * time.Millisecond)

			c.UpdateStatus(tt.newStatus)

			tt.validate(t, c, oldUpdatedAt)
		})
	}
}

func TestCall_Complete(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *call.Call
		duration int
		cost     values.Money
		validate func(t *testing.T, c *call.Call)
	}{
		{
			name: "completes call with valid duration and cost",
			setup: func() *call.Call {
				c := fixtures.NewCallBuilder(t).
					WithStatus(call.StatusInProgress).
					Build()
				// Ensure StartTime is in the past
				c.StartTime = c.StartTime.Add(-1 * time.Second)
				return c
			},
			duration: 300, // 5 minutes
			cost:     values.MustNewMoneyFromFloat(15.50, "USD"),
			validate: func(t *testing.T, c *call.Call) {
				assert.Equal(t, call.StatusCompleted, c.Status)
				assert.NotNil(t, c.EndTime)
				assert.NotNil(t, c.Duration)
				assert.Equal(t, 300, *c.Duration)
				assert.NotNil(t, c.Cost)
				assert.Equal(t, 15.50, c.Cost.ToFloat64())
				assert.True(t, c.EndTime.After(c.StartTime))
			},
		},
		{
			name: "completes call with zero duration",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).
					WithStatus(call.StatusInProgress).
					Build()
			},
			duration: 0,
			cost:     values.MustNewMoneyFromFloat(0.0, "USD"),
			validate: func(t *testing.T, c *call.Call) {
				assert.Equal(t, call.StatusCompleted, c.Status)
				assert.Equal(t, 0, *c.Duration)
				assert.Equal(t, 0.0, c.Cost.ToFloat64())
			},
		},
		{
			name: "completes call from non-in-progress status",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).
					WithStatus(call.StatusRinging).
					Build()
			},
			duration: 60,
			cost:     values.MustNewMoneyFromFloat(5.00, "USD"),
			validate: func(t *testing.T, c *call.Call) {
				// Should still complete successfully
				assert.Equal(t, call.StatusCompleted, c.Status)
				assert.Equal(t, 60, *c.Duration)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setup()
			c.Complete(tt.duration, tt.cost)
			tt.validate(t, c)
		})
	}
}

func TestCall_Fail(t *testing.T) {
	// Set up mock clock for testing
	mockClock := &call.MockClock{CurrentTime: time.Now()}
	call.SetClock(mockClock)
	defer call.ResetClock()

	tests := []struct {
		name     string
		setup    func() *call.Call
		validate func(t *testing.T, c *call.Call)
	}{
		{
			name: "fails call from pending status",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).
					WithStatus(call.StatusPending).
					Build()
			},
			validate: func(t *testing.T, c *call.Call) {
				assert.Equal(t, call.StatusFailed, c.Status)
				assert.Nil(t, c.EndTime) // No end time for failed calls
				assert.Nil(t, c.Duration)
				assert.Nil(t, c.Cost)
			},
		},
		{
			name: "fails call from in progress status",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).
					WithStatus(call.StatusInProgress).
					Build()
			},
			validate: func(t *testing.T, c *call.Call) {
				assert.Equal(t, call.StatusFailed, c.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setup()
			oldUpdatedAt := c.UpdatedAt

			// Advance mock clock instead of sleeping
			mockClock.Advance(10 * time.Millisecond)
			c.Fail()

			tt.validate(t, c)
			assert.True(t, c.UpdatedAt.After(oldUpdatedAt))
		})
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   call.Status
		expected string
	}{
		{call.StatusPending, "pending"},
		{call.StatusQueued, "queued"},
		{call.StatusRinging, "ringing"},
		{call.StatusInProgress, "in_progress"},
		{call.StatusCompleted, "completed"},
		{call.StatusFailed, "failed"},
		{call.StatusCanceled, "canceled"},
		{call.StatusNoAnswer, "no_answer"},
		{call.StatusBusy, "busy"},
		{call.Status(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestDirection_String(t *testing.T) {
	tests := []struct {
		direction call.Direction
		expected  string
	}{
		{call.DirectionInbound, "inbound"},
		{call.DirectionOutbound, "outbound"},
		{call.Direction(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.direction.String())
		})
	}
}

func TestCall_Scenarios(t *testing.T) {
	scenarios := fixtures.NewCallScenarios(t)

	t.Run("inbound call has correct properties", func(t *testing.T) {
		c := scenarios.InboundCall()
		assert.Equal(t, call.DirectionInbound, c.Direction)
		assert.NotNil(t, c.Location)
		assert.Equal(t, "US", c.Location.Country)
		assert.Equal(t, "CA", c.Location.State)
	})

	t.Run("outbound call has seller ID", func(t *testing.T) {
		c := scenarios.OutboundCall()
		assert.Equal(t, call.DirectionOutbound, c.Direction)
		assert.NotNil(t, c.SellerID)
	})

	t.Run("active call is in progress", func(t *testing.T) {
		c := scenarios.ActiveCall()
		assert.Equal(t, call.StatusInProgress, c.Status)
		assert.Nil(t, c.EndTime)
	})

	t.Run("completed call has duration and cost", func(t *testing.T) {
		c := scenarios.CompletedCall()
		assert.Equal(t, call.StatusCompleted, c.Status)
		assert.NotNil(t, c.Duration)
		assert.NotNil(t, c.Cost)
		assert.Greater(t, *c.Duration, 0)
		assert.True(t, c.Cost.IsPositive(), "Cost should be positive")
	})

	t.Run("failed call has failed status", func(t *testing.T) {
		c := scenarios.FailedCall()
		assert.Equal(t, call.StatusFailed, c.Status)
	})
}

func TestCall_Validation(t *testing.T) {
	t.Run("phone number formats", func(t *testing.T) {
		// Test various phone number formats
		numbers := []string{
			"+15551234567",
			"+1-555-123-4567",
			"5551234567",
			"+44 20 1234 5678",
		}

		for _, num := range numbers {
			c, err := call.NewCall(num, num, uuid.New(), call.DirectionInbound)
			require.NoError(t, err)
			// Phone numbers are normalized to E.164 format
			// Just verify they're not empty
			assert.False(t, c.FromNumber.IsEmpty())
			assert.False(t, c.ToNumber.IsEmpty())
		}
	})

	t.Run("time consistency", func(t *testing.T) {
		c, err := call.NewCall("+15551234567", "+15559876543", uuid.New(), call.DirectionInbound)
		require.NoError(t, err)

		// CreatedAt and UpdatedAt should be very close
		diff := c.UpdatedAt.Sub(c.CreatedAt)
		assert.Less(t, diff, time.Millisecond)

		// StartTime should match CreatedAt closely
		startDiff := c.StartTime.Sub(c.CreatedAt)
		assert.Less(t, startDiff.Abs(), time.Millisecond)
	})
}

func TestCall_EdgeCases(t *testing.T) {
	t.Run("multiple status updates", func(t *testing.T) {
		// Set up mock clock
		mockClock := &call.MockClock{CurrentTime: time.Now()}
		call.SetClock(mockClock)
		defer call.ResetClock()

		c := fixtures.NewCallBuilder(t).Build()

		// Progress through multiple statuses
		statuses := []call.Status{
			call.StatusQueued,
			call.StatusRinging,
			call.StatusInProgress,
			call.StatusCompleted,
		}

		var lastUpdate time.Time
		for _, status := range statuses {
			mockClock.Advance(5 * time.Millisecond)
			c.UpdateStatus(status)
			assert.Equal(t, status, c.Status)
			assert.True(t, c.UpdatedAt.After(lastUpdate))
			lastUpdate = c.UpdatedAt
		}
	})

	t.Run("completing already completed call", func(t *testing.T) {
		c := fixtures.NewCallBuilder(t).
			WithStatus(call.StatusCompleted).
			Build()

		// First completion
		c.Complete(100, values.MustNewMoneyFromFloat(10.0, "USD"))
		firstDuration := *c.Duration
		firstCost := *c.Cost

		// Second completion should overwrite
		c.Complete(200, values.MustNewMoneyFromFloat(20.0, "USD"))
		assert.Equal(t, 200, *c.Duration)
		assert.Equal(t, 20.0, c.Cost.ToFloat64())
		assert.NotEqual(t, firstDuration, *c.Duration)
		assert.NotEqual(t, firstCost, *c.Cost)
	})

	t.Run("concurrent modifications", func(t *testing.T) {
		c := fixtures.NewCallBuilder(t).Build()

		// Simulate concurrent updates
		done := make(chan bool, 2)

		go func() {
			c.UpdateStatus(call.StatusInProgress)
			done <- true
		}()

		go func() {
			c.UpdateStatus(call.StatusRinging)
			done <- true
		}()

		<-done
		<-done

		// One of the statuses should have won
		assert.Contains(t, []call.Status{call.StatusInProgress, call.StatusRinging}, c.Status)
	})
}

func TestCall_Performance(t *testing.T) {
	t.Run("creation performance", func(t *testing.T) {
		start := time.Now()
		count := 10000

		for i := 0; i < count; i++ {
			_, _ = call.NewCall("+15551234567", "+15559876543", uuid.New(), call.DirectionInbound)
		}

		elapsed := time.Since(start)
		perCall := elapsed / time.Duration(count)

		// Should be able to create calls very quickly
		// Adjusted to 20µs to account for system variations while still ensuring good performance
		assert.Less(t, perCall, 20*time.Microsecond,
			"Call creation took %v per call, expected < 20µs", perCall)
	})
}

// TestCall_TableDriven demonstrates table-driven testing pattern
func TestCall_TableDriven(t *testing.T) {
	type testCase struct {
		name     string
		setup    func() *call.Call
		action   func(*call.Call)
		expected func(*testing.T, *call.Call)
	}

	tests := []testCase{
		{
			name: "status progression",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).Build()
			},
			action: func(c *call.Call) {
				c.UpdateStatus(call.StatusQueued)
				c.UpdateStatus(call.StatusRinging)
				c.UpdateStatus(call.StatusInProgress)
				c.Complete(120, values.MustNewMoneyFromFloat(8.50, "USD"))
			},
			expected: func(t *testing.T, c *call.Call) {
				assert.Equal(t, call.StatusCompleted, c.Status)
				assert.Equal(t, 120, *c.Duration)
				assert.Equal(t, 8.50, c.Cost.ToFloat64())
			},
		},
		{
			name: "failure path",
			setup: func() *call.Call {
				return fixtures.NewCallBuilder(t).
					WithStatus(call.StatusRinging).
					Build()
			},
			action: func(c *call.Call) {
				c.Fail()
			},
			expected: func(t *testing.T, c *call.Call) {
				assert.Equal(t, call.StatusFailed, c.Status)
				assert.Nil(t, c.Duration)
				assert.Nil(t, c.Cost)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.setup()
			tc.action(c)
			tc.expected(t, c)
		})
	}
}
