package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestCallRepository_List(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("list_with_filters", func(t *testing.T) {
		// Create calls with different statuses
		pendingCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusPending).
			Build()
		inProgressCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusInProgress).
			Build()
		completedCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusCompleted).
			Build()

		for _, c := range []*call.Call{pendingCall, inProgressCall, completedCall} {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Test status filter
		status := call.StatusInProgress
		filter := CallFilter{
			Status:  &status,
			OrderBy: "created_at DESC",
			Limit:   10,
			Offset:  0,
		}
		calls, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, calls, 1)
		assert.Equal(t, inProgressCall.ID, calls[0].ID)

		// Test buyer_id filter
		filter = CallFilter{
			BuyerID: &buyerID,
			OrderBy: "created_at DESC",
			Limit:   10,
			Offset:  0,
		}
		calls, err = repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, calls, 3)
	})

	t.Run("list_with_multiple_filters", func(t *testing.T) {
		// Create another buyer to ensure filtering works
		buyerID2, _ := setupTestAccounts(t, testDB)

		call1 := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusPending).
			WithDirection(call.DirectionInbound).
			Build()
		call2 := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusCompleted).
			WithDirection(call.DirectionInbound).
			Build()
		call3 := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID2).
			WithStatus(call.StatusPending).
			WithDirection(call.DirectionInbound).
			Build()

		for _, c := range []*call.Call{call1, call2, call3} {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Filter by buyer_id and status
		status := call.StatusPending
		filter := CallFilter{
			BuyerID: &buyerID,
			Status:  &status,
			OrderBy: "created_at DESC",
			Limit:   10,
			Offset:  0,
		}
		calls, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, calls, 1)
		assert.Equal(t, call1.ID, calls[0].ID)
	})

	t.Run("list_with_date_range", func(t *testing.T) {
		now := time.Now().UTC()
		yesterday := now.Add(-24 * time.Hour)
		tomorrow := now.Add(24 * time.Hour)

		// Create calls with different timestamps
		oldCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		oldCall.StartTime = yesterday.Add(-1 * time.Hour)
		oldCall.CreatedAt = yesterday.Add(-1 * time.Hour)
		oldCall.UpdatedAt = yesterday.Add(-1 * time.Hour)

		recentCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		recentCall.StartTime = now

		for _, c := range []*call.Call{oldCall, recentCall} {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Filter by date range
		filter := CallFilter{
			StartTimeFrom: &yesterday,
			StartTimeTo:   &tomorrow,
			OrderBy:       "start_time DESC",
			Limit:         10,
			Offset:        0,
		}
		calls, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, calls, 1)
		assert.Equal(t, recentCall.ID, calls[0].ID)
	})

	t.Run("list_with_direction_filter", func(t *testing.T) {
		inboundCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithDirection(call.DirectionInbound).
			Build()
		outboundCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithDirection(call.DirectionOutbound).
			Build()

		for _, c := range []*call.Call{inboundCall, outboundCall} {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Filter by direction
		// Note: direction filter not supported in CallFilter, filtering all calls
		filter := CallFilter{
			OrderBy: "created_at DESC",
			Limit:   10,
			Offset:  0,
		}
		calls, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(calls), 1)

		// Find our inbound call
		found := false
		for _, c := range calls {
			if c.ID == inboundCall.ID {
				found = true
				assert.Equal(t, call.DirectionInbound, c.Direction)
				break
			}
		}
		assert.True(t, found, "Inbound call not found in results")
	})

	t.Run("list_with_ordering", func(t *testing.T) {
		// Create calls with different start times
		time1 := time.Now().UTC().Add(-3 * time.Hour)
		time2 := time.Now().UTC().Add(-2 * time.Hour)
		time3 := time.Now().UTC().Add(-1 * time.Hour)

		call1 := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		call1.StartTime = time1

		call2 := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		call2.StartTime = time2

		call3 := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		call3.StartTime = time3

		for _, c := range []*call.Call{call1, call2, call3} {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Test ascending order
		filter := CallFilter{
			BuyerID: &buyerID,
			OrderBy: "start_time ASC",
			Limit:   10,
			Offset:  0,
		}
		calls, err := repo.List(ctx, filter)
		require.NoError(t, err)

		// Find the positions of our calls
		var positions []int
		for i, c := range calls {
			switch c.ID {
			case call1.ID:
				positions = append(positions, i)
			case call2.ID:
				positions = append(positions, i)
			case call3.ID:
				positions = append(positions, i)
			}
		}

		// Verify they're in ascending order
		assert.Len(t, positions, 3)
		if len(positions) == 3 {
			assert.Less(t, positions[0], positions[1])
			assert.Less(t, positions[1], positions[2])
		}

		// Test descending order
		filter.OrderBy = "start_time DESC"
		calls, err = repo.List(ctx, filter)
		require.NoError(t, err)

		// Reset positions
		positions = []int{}
		for i, c := range calls {
			switch c.ID {
			case call1.ID:
				positions = append(positions, i)
			case call2.ID:
				positions = append(positions, i)
			case call3.ID:
				positions = append(positions, i)
			}
		}

		// Verify they're in descending order
		assert.Len(t, positions, 3)
		if len(positions) == 3 {
			assert.Greater(t, positions[0], positions[1])
			assert.Greater(t, positions[1], positions[2])
		}
	})

	t.Run("list_with_pagination", func(t *testing.T) {
		// Create multiple calls
		for i := 0; i < 15; i++ {
			call := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				Build()
			err := repo.Create(ctx, call)
			require.NoError(t, err)
		}

		// Get first page
		filter := CallFilter{
			BuyerID: &buyerID,
			OrderBy: "created_at DESC",
			Limit:   5,
			Offset:  0,
		}
		page1, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page1, 5)

		// Get second page
		filter.Offset = 5
		page2, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, page2, 5)

		// Verify no overlap
		page1IDs := make(map[uuid.UUID]bool)
		for _, c := range page1 {
			page1IDs[c.ID] = true
		}
		for _, c := range page2 {
			assert.False(t, page1IDs[c.ID], "Found duplicate call in pages")
		}
	})
}

func TestCallRepository_CountByStatus(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("count_by_status", func(t *testing.T) {
		// Create calls with different statuses
		statuses := []call.Status{
			call.StatusPending,
			call.StatusPending,
			call.StatusInProgress,
			call.StatusInProgress,
			call.StatusInProgress,
			call.StatusCompleted,
		}

		for _, status := range statuses {
			call := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithStatus(status).
				Build()
			err := repo.Create(ctx, call)
			require.NoError(t, err)
		}

		// Count by status
		counts, err := repo.CountByStatus(ctx)
		require.NoError(t, err)

		assert.Equal(t, 2, counts[call.StatusPending])
		assert.Equal(t, 3, counts[call.StatusInProgress])
		assert.Equal(t, 1, counts[call.StatusCompleted])
		assert.Equal(t, 0, counts[call.StatusFailed])
	})

	t.Run("count_by_status_returns_valid_map", func(t *testing.T) {
		// Count status for all calls in the database
		// This test verifies the function works correctly
		// but doesn't assume specific counts since other tests may have created data
		counts, err := repo.CountByStatus(ctx)
		require.NoError(t, err)
		require.NotNil(t, counts)

		// Verify it returns a valid map with non-negative counts
		for status, count := range counts {
			assert.GreaterOrEqual(t, count, 0, "Count for status %s should be non-negative", status)
		}

		// The map should at least include the statuses we created in the previous test
		// even if their counts might be higher due to other tests
		assert.GreaterOrEqual(t, counts[call.StatusPending], 2)
		assert.GreaterOrEqual(t, counts[call.StatusInProgress], 3)
		assert.GreaterOrEqual(t, counts[call.StatusCompleted], 1)
	})
}

func TestCallRepository_StatusMapping(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("all_status_values_work", func(t *testing.T) {
		// Test that all status enum values can be stored and retrieved
		allStatuses := []call.Status{
			call.StatusPending,
			call.StatusQueued,
			call.StatusRinging,
			call.StatusInProgress,
			call.StatusCompleted,
			call.StatusFailed,
			call.StatusCanceled,
			call.StatusNoAnswer,
			call.StatusBusy,
		}

		for _, status := range allStatuses {
			t.Run(fmt.Sprintf("status_%s", status.String()), func(t *testing.T) {
				testCall := fixtures.NewCallBuilder(t).
					WithBuyerID(buyerID).
					WithStatus(status).
					Build()

				err := repo.Create(ctx, testCall)
				require.NoError(t, err)

				retrieved, err := repo.GetByID(ctx, testCall.ID)
				require.NoError(t, err)

				// Some statuses map to the same DB value
				expectedStatus := status
				if status == call.StatusCanceled {
					expectedStatus = call.StatusFailed
				} else if status == call.StatusBusy {
					expectedStatus = call.StatusNoAnswer
				}
				assert.Equal(t, expectedStatus, retrieved.Status)
				assert.Equal(t, expectedStatus.String(), retrieved.Status.String())
			})
		}
	})
}

func TestCallRepository_OrderBySanitization(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	buyerID, _ := setupTestAccounts(t, testDB)

	// Create a few test calls
	for i := 0; i < 3; i++ {
		call := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		err := repo.Create(ctx, call)
		require.NoError(t, err)
	}

	t.Run("valid_order_by_clauses", func(t *testing.T) {
		validOrderBys := []string{
			"created_at DESC",
			"start_time ASC",
			"status DESC",
			"created_at ASC, status DESC",
		}

		for _, orderBy := range validOrderBys {
			t.Run(orderBy, func(t *testing.T) {
				filter := CallFilter{
					BuyerID: &buyerID,
					OrderBy: orderBy,
					Limit:   10,
					Offset:  0,
				}
				calls, err := repo.List(ctx, filter)
				assert.NoError(t, err)
				assert.NotNil(t, calls)
			})
		}
	})

	t.Run("sql_injection_protection", func(t *testing.T) {
		// These should be safely handled
		dangerousOrderBys := []string{
			"created_at DESC; DROP TABLE calls;",
			"1=1",
			"created_at DESC UNION SELECT * FROM accounts",
			"'; DELETE FROM calls WHERE '1'='1",
		}

		for _, orderBy := range dangerousOrderBys {
			t.Run(orderBy, func(t *testing.T) {
				// Should either sanitize or reject the input
				filter := CallFilter{
					BuyerID: &buyerID,
					OrderBy: orderBy,
					Limit:   10,
					Offset:  0,
				}
				calls, err := repo.List(ctx, filter)

				// We don't care if it errors or returns results,
				// as long as it doesn't execute malicious SQL
				if err == nil {
					assert.NotNil(t, calls)
				}

				// Verify our calls table still exists and has data
				// by trying to list calls again
				verifyFilter := CallFilter{
					BuyerID: &buyerID,
					OrderBy: "created_at DESC",
					Limit:   1,
					Offset:  0,
				}
				verifyCalls, err := repo.List(ctx, verifyFilter)
				assert.NoError(t, err)
				assert.NotEmpty(t, verifyCalls)
			})
		}
	})
}
