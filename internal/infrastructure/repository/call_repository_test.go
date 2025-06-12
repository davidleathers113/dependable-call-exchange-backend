package repository

import (
	"context"
	"testing"
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
		assert.NotNil(t, retrieved.SessionID)
		assert.Equal(t, sessionID, *retrieved.SessionID)
		assert.NotNil(t, retrieved.UserAgent)
		assert.Equal(t, userAgent, *retrieved.UserAgent)
		assert.NotNil(t, retrieved.IPAddress)
		assert.Equal(t, ipAddress, *retrieved.IPAddress)
		assert.NotNil(t, retrieved.Location)
		assert.Equal(t, location.Country, retrieved.Location.Country)
		assert.Equal(t, location.State, retrieved.Location.State)
		assert.Equal(t, location.City, retrieved.Location.City)
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
					c.FromNumber = values.PhoneNumber{}
				},
				errMsg: "from_number cannot be empty",
			},
			{
				name: "empty_to_number",
				modifier: func(c *call.Call) {
					c.ToNumber = values.PhoneNumber{}
				},
				errMsg: "from_number cannot be empty",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				testCall := fixtures.NewCallBuilder(t).
					WithBuyerID(buyerID).
					Build()
				tc.modifier(testCall)

				err := repo.Create(ctx, testCall)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			})
		}
	})

	t.Run("create_with_various_statuses", func(t *testing.T) {
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

				// Some statuses map to the same DB value
				expectedStatus := status
				if status == call.StatusCanceled {
					expectedStatus = call.StatusFailed
				} else if status == call.StatusBusy {
					expectedStatus = call.StatusNoAnswer
				}
				assert.Equal(t, expectedStatus, retrieved.Status)
			})
		}
	})

	t.Run("create_with_optional_timestamps", func(t *testing.T) {
		now := time.Now().UTC()
		duration := 180 // 3 minutes
		cost := values.MustNewMoneyFromFloat(5.50, "USD")

		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusCompleted).
			Build()

		// Set optional fields
		testCall.EndTime = &now
		testCall.Duration = &duration
		testCall.Cost = &cost

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved.EndTime)
		assert.WithinDuration(t, now, *retrieved.EndTime, time.Second)
		assert.NotNil(t, retrieved.Duration)
		assert.Equal(t, duration, *retrieved.Duration)
		assert.NotNil(t, retrieved.Cost)
		assert.Equal(t, cost.ToFloat64(), retrieved.Cost.ToFloat64())
	})
}

func TestCallRepository_GetByID(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("get_existing_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusInProgress).
			Build()

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)

		assertCallEquals(t, testCall, retrieved)
	})

	t.Run("get_non_existent_call", func(t *testing.T) {
		nonExistentID := uuid.New()
		retrieved, err := repo.GetByID(ctx, nonExistentID)

		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.Contains(t, err.Error(), "call not found")
	})

	t.Run("get_call_with_all_fields", func(t *testing.T) {
		location := &call.Location{
			Country:   "US",
			State:     "NY",
			City:      "New York",
			Latitude:  40.7128,
			Longitude: -74.0060,
			Timezone:  "America/New_York",
		}

		sessionID := "session-xyz-789"
		userAgent := "Mozilla/5.0 (Test)"
		ipAddress := "10.0.0.1"
		endTime := time.Now().UTC()
		duration := 240
		cost := values.MustNewMoneyFromFloat(8.75, "USD")

		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithLocation(location).
			Build()

		// Set all optional fields (excluding RouteID which isn't persisted)
		testCall.SessionID = &sessionID
		testCall.UserAgent = &userAgent
		testCall.IPAddress = &ipAddress
		testCall.EndTime = &endTime
		testCall.Duration = &duration
		testCall.Cost = &cost

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)

		// Verify all fields (excluding RouteID which isn't persisted)
		assert.NotNil(t, retrieved.SessionID)
		assert.Equal(t, sessionID, *retrieved.SessionID)
		assert.NotNil(t, retrieved.UserAgent)
		assert.Equal(t, userAgent, *retrieved.UserAgent)
		assert.NotNil(t, retrieved.IPAddress)
		assert.Equal(t, ipAddress, *retrieved.IPAddress)
		assert.NotNil(t, retrieved.Location)
		assert.Equal(t, location.Country, retrieved.Location.Country)
		assert.NotNil(t, retrieved.EndTime)
		assert.NotNil(t, retrieved.Duration)
		assert.NotNil(t, retrieved.Cost)
	})
}

func TestCallRepository_Update(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	buyerID, sellerID := setupTestAccounts(t, testDB)

	t.Run("update_existing_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithStatus(call.StatusPending).
			Build()

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Update call fields
		testCall.Status = call.StatusInProgress
		testCall.SellerID = &sellerID

		now := time.Now().UTC()
		duration := 120
		cost := values.MustNewMoneyFromFloat(3.50, "USD")

		testCall.EndTime = &now
		testCall.Duration = &duration
		testCall.Cost = &cost
		testCall.UpdatedAt = now

		err = repo.Update(ctx, testCall)
		require.NoError(t, err)

		// Verify updates
		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Equal(t, call.StatusInProgress, retrieved.Status)
		assert.NotNil(t, retrieved.SellerID)
		assert.Equal(t, sellerID, *retrieved.SellerID)
		assert.NotNil(t, retrieved.EndTime)
		assert.NotNil(t, retrieved.Duration)
		assert.Equal(t, duration, *retrieved.Duration)
		assert.NotNil(t, retrieved.Cost)
		assert.Equal(t, cost.ToFloat64(), retrieved.Cost.ToFloat64())
	})

	t.Run("update_non_existent_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		// Don't create the call

		err := repo.Update(ctx, testCall)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("update_location", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Update location
		newLocation := &call.Location{
			Country:   "Canada",
			State:     "Ontario",
			City:      "Toronto",
			Latitude:  43.6532,
			Longitude: -79.3832,
			Timezone:  "America/Toronto",
		}
		testCall.Location = newLocation
		testCall.UpdatedAt = time.Now().UTC()

		err = repo.Update(ctx, testCall)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved.Location)
		assert.Equal(t, newLocation.Country, retrieved.Location.Country)
		assert.Equal(t, newLocation.State, retrieved.Location.State)
		assert.Equal(t, newLocation.City, retrieved.Location.City)
	})
}

func TestCallRepository_Delete(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("delete_existing_call", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
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
		assert.Contains(t, err.Error(), "call not found")
	})

	t.Run("delete_non_existent_call", func(t *testing.T) {
		nonExistentID := uuid.New()
		err := repo.Delete(ctx, nonExistentID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("delete_cascade_behavior", func(t *testing.T) {
		// Create multiple calls with same buyer
		call1 := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()
		call2 := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			Build()

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

	t.Run("delete_call_with_metadata", func(t *testing.T) {
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(buyerID).
			WithLocation(&call.Location{
				Country: "US",
				State:   "CA",
				City:    "San Diego",
			}).
			Build()

		// Add optional metadata
		sessionID := "delete-test-session"
		testCall.SessionID = &sessionID

		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Delete it
		err = repo.Delete(ctx, testCall.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetByID(ctx, testCall.ID)
		assert.Error(t, err)
	})
}
