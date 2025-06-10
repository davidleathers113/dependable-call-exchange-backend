package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestCallRepository_Create(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts that can be referenced by calls
	buyerID, sellerID := setupTestAccounts(t, testDB)

	t.Run("create_valid_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithPhoneNumbers("+15551234567", "+15559876543").
			WithStatus(call.StatusPending).
			WithDirection(call.DirectionInbound).
			WithBuyerID(buyerID).
			WithLocation(&call.Location{
				Country:   "US",
				State:     "CA",
				City:      "Los Angeles",
				Latitude:  34.0522,
				Longitude: -118.2437,
				Timezone:  "America/Los_Angeles",
			}).
			Build()

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Verify call was created
		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Equal(t, testCall.ID, retrieved.ID)
		assert.Equal(t, testCall.FromNumber, retrieved.FromNumber)
		assert.Equal(t, testCall.ToNumber, retrieved.ToNumber)
		assert.Equal(t, testCall.Status, retrieved.Status)
		assert.Equal(t, testCall.Direction, retrieved.Direction)
		assert.Equal(t, testCall.BuyerID, retrieved.BuyerID)
	})

	t.Run("create_with_seller_id", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithSellerID(sellerID).
			Build()

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved.SellerID)
		assert.Equal(t, sellerID, *retrieved.SellerID)
	})

	t.Run("create_with_complex_metadata", func(t *testing.T) {
		location := &call.Location{
			Country:   "United States",
			State:     "California", 
			City:      "San Francisco",
			Latitude:  37.7749,
			Longitude: -122.4194,
			Timezone:  "America/Los_Angeles",
		}

		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithLocation(location).
			Build()

		// Add additional metadata
		testCall.CallSID = "CALL123456789"
		sessionID := "session-abc-123"
		userAgent := "SIP/2.0 (TestAgent)"
		ipAddress := "192.168.1.100"
		testCall.SessionID = &sessionID
		testCall.UserAgent = &userAgent
		testCall.IPAddress = &ipAddress

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Equal(t, testCall.CallSID, retrieved.CallSID)
		assert.Equal(t, testCall.SessionID, retrieved.SessionID)
		assert.Equal(t, testCall.UserAgent, retrieved.UserAgent)
		assert.Equal(t, testCall.IPAddress, retrieved.IPAddress)
		require.NotNil(t, retrieved.Location)
		assert.Equal(t, location.Country, retrieved.Location.Country)
		assert.Equal(t, location.State, retrieved.Location.State)
		assert.Equal(t, location.City, retrieved.Location.City)
		assert.Equal(t, location.Latitude, retrieved.Location.Latitude)
		assert.Equal(t, location.Longitude, retrieved.Location.Longitude)
		assert.Equal(t, location.Timezone, retrieved.Location.Timezone)
	})

	t.Run("create_validation_errors", func(t *testing.T) {
		testCases := []struct {
			name     string
			modifier func(*call.Call)
			errMsg   string
		}{
			{
				name: "empty_from_number",
				modifier: func(c *call.Call) {
					c.FromNumber = values.PhoneNumber{} // Empty phone number
				},
				errMsg: "from_number cannot be empty",
			},
			{
				name: "empty_to_number",
				modifier: func(c *call.Call) {
					c.ToNumber = values.PhoneNumber{} // Empty phone number
				},
				errMsg: "from_number cannot be empty",
			},
			{
				name: "nil_buyer_id",
				modifier: func(c *call.Call) {
					c.BuyerID = uuid.Nil
				},
				errMsg: "buyer_id cannot be nil",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				testCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
				tc.modifier(testCall)

				err := repo.Create(ctx, testCall)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			})
		}
	})

	t.Run("create_duplicate_id", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		
		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Try to create with same ID
		duplicateCall := fixtures.NewCallBuilder(t).
			WithID(testCall.ID).
			WithBuyerID(buyerID).
			Build()

		err = repo.Create(ctx, duplicateCall)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate key")
	})

	t.Run("create_with_different_directions", func(t *testing.T) {
		directions := []call.Direction{call.DirectionInbound, call.DirectionOutbound}
		
		for _, direction := range directions {
			t.Run(direction.String(), func(t *testing.T) {
				testCall := fixtures.NewCallBuilder(t).
					WithBuyerID(buyerID).
					WithDirection(direction).
					Build()

				err := repo.Create(ctx, testCall)
				require.NoError(t, err)

				retrieved, err := repo.GetByID(ctx, testCall.ID)
				require.NoError(t, err)
				assert.Equal(t, direction, retrieved.Direction)
			})
		}
	})
}

func TestCallRepository_GetByID(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts for this test
	buyerID, sellerID := setupTestAccounts(t, testDB)

	t.Run("get_existing_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusInProgress).
			WithLocation(&call.Location{
				Country: "US",
				State:   "NY",
				City:    "New York",
			}).
			Build()

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)

		assert.Equal(t, testCall.ID, retrieved.ID)
		assert.Equal(t, testCall.FromNumber, retrieved.FromNumber)
		assert.Equal(t, testCall.ToNumber, retrieved.ToNumber)
		assert.Equal(t, testCall.Status, retrieved.Status)
		assert.Equal(t, testCall.Direction, retrieved.Direction)
		assert.Equal(t, testCall.BuyerID, retrieved.BuyerID)
		assert.Equal(t, testCall.CallSID, retrieved.CallSID)
	})

	t.Run("get_non_existent_call", func(t *testing.T) {
		nonExistentID := uuid.New()
		retrieved, err := repo.GetByID(ctx, nonExistentID)

		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("get_call_with_optional_fields", func(t *testing.T) {
		duration := 300
		costMoney := values.MustNewMoneyFromFloat(12.50, "USD")
		endTime := time.Now().UTC()

		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithSellerID(sellerID).
			WithStatus(call.StatusCompleted).
			Build()

		// Set optional fields after creation
		testCall.EndTime = &endTime
		testCall.Duration = &duration
		testCall.Cost = &costMoney

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Update with optional fields
		err = repo.Update(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)

		require.NotNil(t, retrieved.SellerID)
		assert.Equal(t, sellerID, *retrieved.SellerID)
		require.NotNil(t, retrieved.EndTime)
		assert.WithinDuration(t, endTime, *retrieved.EndTime, time.Second)
		require.NotNil(t, retrieved.Duration)
		assert.Equal(t, duration, *retrieved.Duration)
		require.NotNil(t, retrieved.Cost)
		assert.Equal(t, costMoney, *retrieved.Cost)
	})

	t.Run("get_various_call_statuses", func(t *testing.T) {
		statuses := []call.Status{
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

		for _, status := range statuses {
			t.Run(status.String(), func(t *testing.T) {
				testCall := fixtures.NewCallBuilder(t).
					WithBuyerID(buyerID).
					WithStatus(status).
					Build()

				err := repo.Create(ctx, testCall)
				require.NoError(t, err)

				retrieved, err := repo.GetByID(ctx, testCall.ID)
				require.NoError(t, err)
				
				// Verify status - note that some statuses don't have exact roundtrip
				expectedStatus := status
				// Canceled maps to failed in DB and back
				if status == call.StatusCanceled {
					expectedStatus = call.StatusFailed
				}
				// Busy maps to no_answer in DB and back
				if status == call.StatusBusy {
					expectedStatus = call.StatusNoAnswer
				}
				assert.Equal(t, expectedStatus, retrieved.Status)
			})
		}
	})
}

func TestCallRepository_Update(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts
	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("update_existing_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusPending).
			Build()

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Update call fields
		testCall.Status = call.StatusInProgress
		now := time.Now().UTC()
		testCall.UpdatedAt = now
		duration := 180
		costMoney := values.MustNewMoneyFromFloat(9.99, "USD")
		testCall.Duration = &duration
		testCall.Cost = &costMoney

		err = repo.Update(ctx, testCall)
		require.NoError(t, err)

		// Verify updates
		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Equal(t, call.StatusInProgress, retrieved.Status)
		require.NotNil(t, retrieved.Duration)
		assert.Equal(t, duration, *retrieved.Duration)
		require.NotNil(t, retrieved.Cost)
		assert.Equal(t, costMoney, *retrieved.Cost)
	})

	t.Run("update_non_existent_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		// Don't create the call

		err := repo.Update(ctx, testCall)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("complete_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusInProgress).
			Build()

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Complete the call
		testCall.Status = call.StatusCompleted
		endTime := time.Now().UTC()
		duration := 420
		costMoney := values.MustNewMoneyFromFloat(15.75, "USD")
		testCall.EndTime = &endTime
		testCall.Duration = &duration
		testCall.Cost = &costMoney
		testCall.UpdatedAt = endTime

		err = repo.Update(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Equal(t, call.StatusCompleted, retrieved.Status)
		require.NotNil(t, retrieved.EndTime)
		assert.WithinDuration(t, endTime, *retrieved.EndTime, time.Second)
		require.NotNil(t, retrieved.Duration)
		assert.Equal(t, duration, *retrieved.Duration)
		require.NotNil(t, retrieved.Cost)
		assert.Equal(t, costMoney, *retrieved.Cost)
	})

	t.Run("concurrent_updates", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		numGoroutines := 5
		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()
				
				// Each goroutine updates to a different status and cost
				localCall := *testCall // Copy the call
				localCall.Status = call.StatusInProgress
				costMoney := values.MustNewMoneyFromFloat(float64(10 + iteration*5), "USD")
				localCall.Cost = &costMoney
				localCall.UpdatedAt = time.Now().UTC()
				
				if err := repo.Update(ctx, &localCall); err != nil {
					errChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// All updates should succeed
		for err := range errChan {
			require.NoError(t, err)
		}

		// Verify final state
		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		
		// Cost should be one of the concurrent updates
		validCosts := []float64{10, 15, 20, 25, 30}
		require.NotNil(t, retrieved.Cost)
		assert.Contains(t, validCosts, *retrieved.Cost)
	})
}

func TestCallRepository_Delete(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts
	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("delete_existing_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Verify it exists
		_, err = repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)

		// Delete it
		err = repo.Delete(ctx, testCall.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetByID(ctx, testCall.ID)
		assert.Error(t, err)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("delete_non_existent_call", func(t *testing.T) {
		nonExistentID := uuid.New()
		err := repo.Delete(ctx, nonExistentID)
		
		// Delete of non-existent should not error (idempotent)
		assert.NoError(t, err)
	})

	t.Run("delete_cascade_behavior", func(t *testing.T) {
		// Create multiple calls
		call1 := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		call2 := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()

		err := repo.Create(ctx, call1)
		require.NoError(t, err)
		err = repo.Create(ctx, call2)
		require.NoError(t, err)

		// Delete one call - the other should remain
		err = repo.Delete(ctx, call1.ID)
		require.NoError(t, err)

		// Verify first is gone, second remains
		_, err = repo.GetByID(ctx, call1.ID)
		assert.Error(t, err)

		_, err = repo.GetByID(ctx, call2.ID)
		assert.NoError(t, err)
	})
}

func TestCallRepository_List(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts
	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("list_all_calls", func(t *testing.T) {
		// Create test calls
		calls := []*call.Call{
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID).WithStatus(call.StatusPending).Build(),
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID).WithStatus(call.StatusInProgress).Build(),
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID).WithStatus(call.StatusCompleted).Build(),
		}

		for _, c := range calls {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		results, err := repo.List(ctx, CallFilter{})
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("filter_by_status", func(t *testing.T) {
		testDB.TruncateTables()
		
		// Recreate test accounts after truncate
		buyerID, _ := setupTestAccounts(t, testDB)

		calls := []*call.Call{
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID).WithStatus(call.StatusPending).Build(),
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID).WithStatus(call.StatusPending).Build(),
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID).WithStatus(call.StatusInProgress).Build(),
		}

		for _, c := range calls {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Filter by pending status
		status := call.StatusPending
		results, err := repo.List(ctx, CallFilter{Status: &status})
		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, result := range results {
			assert.Equal(t, call.StatusPending, result.Status)
		}
	})

	t.Run("filter_by_buyer_id", func(t *testing.T) {
		testDB.TruncateTables()
		
		// Create two different buyer accounts
		buyerID1, _ := setupTestAccounts(t, testDB)
		buyerID2 := uuid.New()
		
		// Create second buyer account
		_, err := testDB.DB().Exec(`
			INSERT INTO accounts (
				id, email, name, company, type, status, phone_number,
				balance, credit_limit, payment_terms,
				tcpa_consent, gdpr_consent, compliance_flags,
				quality_score, fraud_score, settings,
				created_at, updated_at
			) VALUES 
			($1, 'buyer2@test.com', 'Buyer Test 2', 'Buyer Corp 2', 'buyer', 'active', '+15551234568',
			 1000.0, 5000.0, 30,
			 true, true, ARRAY[]::text[],
			 5.0, 0.0, '{}'::jsonb,
			 NOW(), NOW())
		`, buyerID2)
		require.NoError(t, err)

		calls := []*call.Call{
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID1).Build(),
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID1).Build(),
			fixtures.NewCallBuilder(t).WithBuyerID(buyerID2).Build(),
		}

		for _, c := range calls {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		results, err := repo.List(ctx, CallFilter{BuyerID: &buyerID1})
		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, result := range results {
			assert.Equal(t, buyerID1, result.BuyerID)
		}
	})

	t.Run("filter_by_time_range", func(t *testing.T) {
		testDB.TruncateTables()
		
		// Recreate test accounts after truncate
		buyerID, _ := setupTestAccounts(t, testDB)

		now := time.Now().UTC()
		past := now.Add(-2 * time.Hour)
		future := now.Add(2 * time.Hour)

		// Create calls with different start times
		oldCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		oldCall.StartTime = past.Add(-1 * time.Hour)
		
		recentCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		recentCall.StartTime = now

		futureCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		futureCall.StartTime = future.Add(1 * time.Hour)

		for _, c := range []*call.Call{oldCall, recentCall, futureCall} {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Filter by time range
		startFrom := past
		startTo := future
		results, err := repo.List(ctx, CallFilter{
			StartTimeFrom: &startFrom,
			StartTimeTo:   &startTo,
		})
		require.NoError(t, err)
		assert.Len(t, results, 1) // Only recentCall should match
		assert.Equal(t, recentCall.ID, results[0].ID)
	})

	t.Run("pagination", func(t *testing.T) {
		testDB.TruncateTables()
		
		// Recreate test accounts after truncate
		buyerID, _ := setupTestAccounts(t, testDB)

		// Create 5 calls
		for i := 0; i < 5; i++ {
			c := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Test pagination
		results, err := repo.List(ctx, CallFilter{Limit: 2, Offset: 1})
		require.NoError(t, err)
		assert.Len(t, results, 2)

		// Test limit only
		results, err = repo.List(ctx, CallFilter{Limit: 3})
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("complex_filter_combination", func(t *testing.T) {
		testDB.TruncateTables()
		
		// Create test accounts
		buyerID1, _ := setupTestAccounts(t, testDB)
		buyerID2 := uuid.New()
		
		// Create second buyer account
		_, err := testDB.DB().Exec(`
			INSERT INTO accounts (
				id, email, name, company, type, status, phone_number,
				balance, credit_limit, payment_terms,
				tcpa_consent, gdpr_consent, compliance_flags,
				quality_score, fraud_score, settings,
				created_at, updated_at
			) VALUES 
			($1, 'buyer3@test.com', 'Buyer Test 3', 'Buyer Corp 3', 'buyer', 'active', '+15551234569',
			 1000.0, 5000.0, 30,
			 true, true, ARRAY[]::text[],
			 5.0, 0.0, '{}'::jsonb,
			 NOW(), NOW())
		`, buyerID2)
		require.NoError(t, err)

		status := call.StatusInProgress
		now := time.Now().UTC()

		// Create matching call
		matchingCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID1).
			WithStatus(status).
			Build()
		matchingCall.StartTime = now

		// Create non-matching calls
		wrongBuyer := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID2).
			WithStatus(status).
			Build()
		wrongBuyer.StartTime = now

		wrongStatus := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID1).
			WithStatus(call.StatusCompleted).
			Build()
		wrongStatus.StartTime = now

		for _, c := range []*call.Call{matchingCall, wrongBuyer, wrongStatus} {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		startFrom := now.Add(-1 * time.Hour)
		startTo := now.Add(1 * time.Hour)

		results, err := repo.List(ctx, CallFilter{
			Status:        &status,
			BuyerID:       &buyerID1,
			StartTimeFrom: &startFrom,
			StartTimeTo:   &startTo,
		})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, matchingCall.ID, results[0].ID)
	})
}

func TestCallRepository_CountByStatus(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts
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
			call.StatusFailed,
		}

		for _, status := range statuses {
			c := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithStatus(status).
				Build()
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		counts, err := repo.CountByStatus(ctx)
		require.NoError(t, err)

		expected := map[call.Status]int{
			call.StatusPending:    2,
			call.StatusInProgress: 3,
			call.StatusCompleted:  1,
			call.StatusFailed:     1,
		}

		assert.Equal(t, expected, counts)
	})

	t.Run("count_empty_database", func(t *testing.T) {
		testDB.TruncateTables()

		counts, err := repo.CountByStatus(ctx)
		require.NoError(t, err)
		assert.Empty(t, counts)
	})
}

func TestCallRepository_StatusMapping(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts
	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("status_mapping_roundtrip", func(t *testing.T) {
		statusMappings := map[call.Status]string{
			call.StatusPending:    "pending",
			call.StatusQueued:     "queued",
			call.StatusRinging:    "ringing",
			call.StatusInProgress: "in_progress",
			call.StatusCompleted:  "completed",
			call.StatusFailed:     "failed",
			call.StatusCanceled:   "failed", // Maps to failed in DB
			call.StatusNoAnswer:   "no_answer",
			call.StatusBusy:       "no_answer", // Maps to no_answer in DB
		}

		for domainStatus, dbEnum := range statusMappings {
			t.Run(domainStatus.String(), func(t *testing.T) {
				// Test mapStatusToEnum
				assert.Equal(t, dbEnum, mapStatusToEnum(domainStatus))

				// Test creating and retrieving call with this status
				testCall := fixtures.NewCallBuilder(t).
					WithBuyerID(buyerID).
					WithStatus(domainStatus).
					Build()

				err := repo.Create(ctx, testCall)
				require.NoError(t, err)

				retrieved, err := repo.GetByID(ctx, testCall.ID)
				require.NoError(t, err)
				
				// Status should be preserved for most statuses
				expectedStatus := domainStatus
				// Canceled maps to failed in DB and back
				if domainStatus == call.StatusCanceled {
					expectedStatus = call.StatusFailed
				}
				// Busy maps to no_answer in DB and back
				if domainStatus == call.StatusBusy {
					expectedStatus = call.StatusNoAnswer
				}
				assert.Equal(t, expectedStatus, retrieved.Status)
			})
		}
	})

	t.Run("enum_to_status_mapping", func(t *testing.T) {
		enumMappings := map[string]call.Status{
			"pending":     call.StatusPending,
			"queued":      call.StatusQueued,
			"ringing":     call.StatusRinging,
			"in_progress": call.StatusInProgress,
			"completed":   call.StatusCompleted,
			"failed":      call.StatusFailed,
			"no_answer":   call.StatusNoAnswer,
			"unknown":     call.StatusPending, // Default case
		}

		for enum, expectedStatus := range enumMappings {
			t.Run(enum, func(t *testing.T) {
				assert.Equal(t, expectedStatus, mapEnumToStatus(enum))
			})
		}
	})
}

func TestCallRepository_OrderBySanitization(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty_string", "", "created_at DESC"},
		{"valid_column_asc", "status ASC", "status ASC"},
		{"valid_column_desc", "duration DESC", "duration DESC"},
		{"valid_column_no_direction", "cost", "cost DESC"},
		{"invalid_column", "malicious_column", "created_at DESC"},
		{"sql_injection_attempt", "id; DROP TABLE calls;", "created_at DESC"},
		{"valid_column_invalid_direction", "status MALICIOUS", "status DESC"},
		{"extra_spaces", "  duration   DESC  ", "duration DESC"},
		{"mixed_case", "Cost desc", "created_at DESC"}, // Invalid column name (case-sensitive)
		{"all_valid_columns", "id", "id DESC"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeOrderBy(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}

	t.Run("sql_injection_protection_integration", func(t *testing.T) {
		testDB := testutil.NewTestDB(t)
		ctx := context.Background()
		repo := NewCallRepository(testDB.DB())

		// Create test accounts for this sub-test
		buyerID, _ := setupTestAccounts(t, testDB)

		// Create test call
		testCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Try malicious ORDER BY - should not cause error or injection
		maliciousOrderBy := "id; DROP TABLE calls; --"
		results, err := repo.List(ctx, CallFilter{OrderBy: maliciousOrderBy})
		require.NoError(t, err)
		assert.Len(t, results, 1) // Table should still exist and contain data
	})
}

func TestCallRepository_PropertyBased(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts
	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("phone_number_consistency", func(t *testing.T) {
		// Property: Phone numbers should always be preserved exactly
		property := func(seed1, seed2 uint64) bool {
			// Generate valid phone numbers from seeds
			fromNum := fmt.Sprintf("+1555%07d", seed1 % 10000000)
			toNum := fmt.Sprintf("+1555%07d", seed2 % 10000000)
			
			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithPhoneNumbers(fromNum, toNum).
				Build()
			
			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}
			
			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}
			
			// Property: Phone numbers should be preserved exactly
			return retrieved.FromNumber.String() == fromNum && retrieved.ToNumber.String() == toNum
		}
		
		if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
			t.Error(err)
		}
	})

	t.Run("metadata_json_roundtrip", func(t *testing.T) {
		// Property: Complex metadata should survive JSON roundtrip
		property := func(country, state, city string, lat, lon float64) bool {
			if country == "" || len(country) > 50 {
				return true // Skip invalid countries
			}
			if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
				return true // Skip invalid coordinates
			}
			
			location := &call.Location{
				Country:   country,
				State:     state,
				City:      city,
				Latitude:  lat,
				Longitude: lon,
				Timezone:  "UTC",
			}
			
			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithLocation(location).
				Build()
			
			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}
			
			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}
			
			// Property: Location should be preserved exactly
			if retrieved.Location == nil {
				return false
			}
			return retrieved.Location.Country == country &&
				retrieved.Location.State == state &&
				retrieved.Location.City == city &&
				retrieved.Location.Latitude == lat &&
				retrieved.Location.Longitude == lon
		}
		
		if err := quick.Check(property, &quick.Config{MaxCount: 15}); err != nil {
			t.Error(err)
		}
	})

	t.Run("cost_duration_consistency", func(t *testing.T) {
		// Property: Cost and duration should be preserved accurately
		property := func(duration int, cost float64) bool {
			// Normalize to valid ranges
			if duration < 0 || duration > 36000 { // 0 to 10 hours
				return true
			}
			if cost < 0 || cost > 1000 { // $0 to $1000
				return true
			}
			
			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithStatus(call.StatusCompleted).
				Build()
			
			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}
			
			// Update with cost and duration
			costMoney := values.MustNewMoneyFromFloat(cost, "USD")
			testCall.Duration = &duration
			testCall.Cost = &costMoney
			testCall.UpdatedAt = time.Now().UTC()
			
			err = repo.Update(ctx, testCall)
			if err != nil {
				return false
			}
			
			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}
			
			// Property: Cost and duration should be preserved exactly
			return retrieved.Duration != nil && *retrieved.Duration == duration &&
				retrieved.Cost != nil && retrieved.Cost.ToFloat64() == cost
		}
		
		if err := quick.Check(property, &quick.Config{MaxCount: 25}); err != nil {
			t.Error(err)
		}
	})
}

func TestCallRepository_Concurrency(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	// Create test accounts
	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("concurrent_creates", func(t *testing.T) {
		numCalls := 10
		
		var wg sync.WaitGroup
		callChan := make(chan *call.Call, numCalls)
		errChan := make(chan error, numCalls)

		for i := 0; i < numCalls; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()
				
				testCall := fixtures.NewCallBuilder(t).
					WithBuyerID(buyerID).
					WithPhoneNumbers(
						"+1555100"+fmt.Sprintf("%04d", iteration),
						"+1555200"+fmt.Sprintf("%04d", iteration),
					).
					Build()
				
				if err := repo.Create(ctx, testCall); err != nil {
					errChan <- err
					return
				}
				
				callChan <- testCall
			}(i)
		}

		wg.Wait()
		close(callChan)
		close(errChan)

		// Check for errors
		for err := range errChan {
			require.NoError(t, err)
		}

		// Verify all calls were created
		var createdCalls []*call.Call
		for c := range callChan {
			createdCalls = append(createdCalls, c)
		}
		assert.Len(t, createdCalls, numCalls)

		// Verify they're all retrievable
		for _, c := range createdCalls {
			retrieved, err := repo.GetByID(ctx, c.ID)
			require.NoError(t, err)
			assert.Equal(t, c.ID, retrieved.ID)
		}
	})

	t.Run("concurrent_status_updates", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		numUpdates := 5
		statuses := []call.Status{
			call.StatusQueued,
			call.StatusRinging,
			call.StatusInProgress,
			call.StatusCompleted,
			call.StatusFailed,
		}

		var wg sync.WaitGroup
		errChan := make(chan error, numUpdates)

		for i := 0; i < numUpdates; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()
				
				// Each goroutine tries to update to a different status
				localCall := *testCall // Copy the call
				localCall.Status = statuses[iteration]
				localCall.UpdatedAt = time.Now().UTC()
				
				if err := repo.Update(ctx, &localCall); err != nil {
					errChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// All updates should succeed (last writer wins)
		for err := range errChan {
			assert.NoError(t, err)
		}

		// Verify final state is one of the updates
		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Contains(t, statuses, retrieved.Status)
	})
}

func TestCallRepository_TransactionSupport(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()

	// Create test accounts
	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("transaction_rollback", func(t *testing.T) {
		err := testDB.RunInTransaction(func(tx *sql.Tx) error {
			repo := NewCallRepositoryWithTx(tx)
			
			testCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
			err := repo.Create(ctx, testCall)
			require.NoError(t, err)
			
			// Verify call exists within transaction
			_, err = repo.GetByID(ctx, testCall.ID)
			require.NoError(t, err)
			
			// Return error to trigger rollback
			return assert.AnError
		})
		assert.Error(t, err)

		// Verify call was rolled back
		regularRepo := NewCallRepository(testDB.DB())
		calls, err := regularRepo.List(ctx, CallFilter{})
		require.NoError(t, err)
		assert.Empty(t, calls)
	})

	t.Run("transaction_commit", func(t *testing.T) {
		var testCallID uuid.UUID
		
		err := testDB.WithTx(ctx, func(tx *sql.Tx) error {
			repo := NewCallRepositoryWithTx(tx)
			
			testCall := fixtures.NewCallBuilder(t).WithBuyerID(buyerID).Build()
			testCallID = testCall.ID
			
			return repo.Create(ctx, testCall)
		})
		require.NoError(t, err)

		// Verify call was committed
		regularRepo := NewCallRepository(testDB.DB())
		retrieved, err := regularRepo.GetByID(ctx, testCallID)
		require.NoError(t, err)
		assert.Equal(t, testCallID, retrieved.ID)
	})
}

// Helper functions

func compareMetadata(expected, actual map[string]interface{}) bool {
	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)
	return string(expectedJSON) == string(actualJSON)
}

// setupTestAccounts creates test buyer and seller accounts for call repository tests
func setupTestAccounts(t *testing.T, testDB *testutil.TestDB) (buyerID, sellerID uuid.UUID) {
	t.Helper()
	
	buyerID = uuid.New()
	sellerID = uuid.New()
	
	// Create accounts directly with minimal setup
	_, err := testDB.DB().Exec(`
		INSERT INTO accounts (
			id, email, name, company, type, status, phone_number,
			balance, credit_limit, payment_terms,
			tcpa_consent, gdpr_consent, compliance_flags,
			quality_score, fraud_score, settings,
			created_at, updated_at
		) VALUES 
		($1, 'buyer@test.com', 'Buyer Test', 'Buyer Corp', 'buyer', 'active', '+15551234567',
		 1000.0, 5000.0, 30,
		 true, true, ARRAY[]::text[],
		 5.0, 0.0, '{}'::jsonb,
		 NOW(), NOW()),
		($2, 'seller@test.com', 'Seller Test', 'Seller Corp', 'seller', 'active', '+15559876543',
		 2000.0, 10000.0, 30,
		 true, true, ARRAY[]::text[],
		 5.0, 0.0, '{}'::jsonb,
		 NOW(), NOW())
	`, buyerID, sellerID)
	require.NoError(t, err)
	
	return buyerID, sellerID
}

