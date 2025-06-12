package repository

import (
	"context"
	"testing"
	"testing/quick"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestCallRepository_PropertyBased(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewCallRepository(testDB.DB())

	buyerID, _ := setupTestAccounts(t, testDB)

	t.Run("phone_number_preservation", func(t *testing.T) {
		// Property: Phone numbers should be preserved exactly
		property := func(fromDigits, toDigits string) bool {
			// Normalize to valid phone numbers
			if len(fromDigits) < 10 || len(toDigits) < 10 {
				return true // Skip invalid inputs
			}

			// Take first 10 digits for US phone numbers
			fromPhone := "+1" + fromDigits[:10]
			toPhone := "+1" + toDigits[:10]

			// Validate phone numbers
			fromNumber, err := values.NewPhoneNumber(fromPhone)
			if err != nil {
				return true // Skip invalid phone numbers
			}
			toNumber, err := values.NewPhoneNumber(toPhone)
			if err != nil {
				return true // Skip invalid phone numbers
			}

			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				Build()
			testCall.FromNumber = fromNumber
			testCall.ToNumber = toNumber

			err = repo.Create(ctx, testCall)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}

			// Property: Phone numbers should be preserved exactly
			return retrieved.FromNumber.String() == fromNumber.String() &&
				retrieved.ToNumber.String() == toNumber.String()
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
			t.Error(err)
		}
	})

	t.Run("status_transitions", func(t *testing.T) {
		// Property: Status updates should always be persisted correctly
		property := func(initialStatus, finalStatus uint8) bool {
			// Map to valid status values
			statuses := []call.Status{
				call.StatusPending, call.StatusQueued, call.StatusRinging,
				call.StatusInProgress, call.StatusCompleted, call.StatusFailed,
				call.StatusCanceled, call.StatusNoAnswer, call.StatusBusy,
			}

			initial := statuses[int(initialStatus)%len(statuses)]
			final := statuses[int(finalStatus)%len(statuses)]

			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithStatus(initial).
				Build()

			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}

			// Update status
			testCall.Status = final
			testCall.UpdatedAt = time.Now().UTC()

			err = repo.Update(ctx, testCall)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}

			// Property: Status should be updated correctly
			// Note: Some statuses map to the same DB value:
			// StatusCanceled -> "failed" -> StatusFailed
			// StatusBusy -> "no_answer" -> StatusNoAnswer
			expectedStatus := final
			if final == call.StatusCanceled {
				expectedStatus = call.StatusFailed
			} else if final == call.StatusBusy {
				expectedStatus = call.StatusNoAnswer
			}
			return retrieved.Status == expectedStatus
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 30}); err != nil {
			t.Error(err)
		}
	})

	t.Run("duration_consistency", func(t *testing.T) {
		// Property: Call duration should be preserved accurately
		property := func(duration int) bool {
			// Normalize to valid duration range (0 to 24 hours in seconds)
			if duration < 0 || duration > 86400 {
				return true // Skip invalid durations
			}

			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithStatus(call.StatusCompleted).
				Build()

			// Set duration
			testCall.Duration = &duration
			endTime := time.Now().UTC()
			testCall.EndTime = &endTime

			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}

			// Property: Duration should be preserved exactly
			return retrieved.Duration != nil && *retrieved.Duration == duration
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 25}); err != nil {
			t.Error(err)
		}
	})

	t.Run("cost_precision", func(t *testing.T) {
		// Property: Cost values should maintain precision
		property := func(costCents int) bool {
			// Normalize to valid cost range (0 to $1000)
			if costCents < 0 || costCents > 100000 {
				return true // Skip invalid costs
			}

			costFloat := float64(costCents) / 100.0
			cost := values.MustNewMoneyFromFloat(costFloat, "USD")

			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithStatus(call.StatusCompleted).
				Build()

			// Set cost
			testCall.Cost = &cost

			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}

			// Property: Cost should be preserved with precision
			return retrieved.Cost != nil &&
				retrieved.Cost.ToFloat64() == cost.ToFloat64()
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
			t.Error(err)
		}
	})

	t.Run("location_json_roundtrip", func(t *testing.T) {
		// Property: Location data should survive JSON roundtrip
		property := func(country, state, city string, lat, lon float64) bool {
			// Skip empty or very long strings
			if len(country) == 0 || len(country) > 100 ||
				len(state) == 0 || len(state) > 100 ||
				len(city) == 0 || len(city) > 100 {
				return true
			}

			// Normalize coordinates to valid ranges
			if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
				return true
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
			return retrieved.Location != nil &&
				retrieved.Location.Country == country &&
				retrieved.Location.State == state &&
				retrieved.Location.City == city &&
				retrieved.Location.Latitude == lat &&
				retrieved.Location.Longitude == lon
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 15}); err != nil {
			t.Error(err)
		}
	})

	t.Run("timestamp_ordering", func(t *testing.T) {
		// Property: Timestamps should maintain logical ordering
		property := func(minutesAgo uint) bool {
			// Limit to reasonable time range
			if minutesAgo > 10000 {
				return true
			}

			now := time.Now().UTC()
			startTime := now.Add(-time.Duration(minutesAgo) * time.Minute)

			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				WithStatus(call.StatusCompleted).
				Build()

			// Set timestamps
			testCall.StartTime = startTime
			duration := 120 // 2 minutes
			testCall.Duration = &duration
			endTime := startTime.Add(time.Duration(duration) * time.Second)
			testCall.EndTime = &endTime

			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}

			// Properties to verify:
			// 1. StartTime <= EndTime (if EndTime exists)
			// 2. StartTime <= CreatedAt (call started before or at creation)
			// 3. CreatedAt <= UpdatedAt
			if retrieved.EndTime != nil {
				if retrieved.StartTime.After(*retrieved.EndTime) {
					return false
				}
			}

			return !retrieved.StartTime.After(retrieved.CreatedAt) &&
				!retrieved.CreatedAt.After(retrieved.UpdatedAt)
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
			t.Error(err)
		}
	})

	t.Run("metadata_preservation", func(t *testing.T) {
		// Property: Optional metadata fields should be preserved
		property := func(sessionID, userAgent, ipAddress string) bool {
			// Skip empty or very long strings
			if len(sessionID) == 0 || len(sessionID) > 200 ||
				len(userAgent) == 0 || len(userAgent) > 500 ||
				len(ipAddress) == 0 || len(ipAddress) > 50 {
				return true
			}

			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				Build()

			// Set metadata
			testCall.SessionID = &sessionID
			testCall.UserAgent = &userAgent
			testCall.IPAddress = &ipAddress

			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}

			// Property: Metadata should be preserved exactly
			return retrieved.SessionID != nil && *retrieved.SessionID == sessionID &&
				retrieved.UserAgent != nil && *retrieved.UserAgent == userAgent &&
				retrieved.IPAddress != nil && *retrieved.IPAddress == ipAddress
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 15}); err != nil {
			t.Error(err)
		}
	})

	t.Run("seller_id_optional", func(t *testing.T) {
		// Property: SellerID should be optional and preserved when set
		property := func(hasSeller bool) bool {
			testCall := fixtures.NewCallBuilder(t).
				WithBuyerID(buyerID).
				Build()

			if hasSeller {
				_, sellerID := setupTestAccounts(t, testDB)
				testCall.SellerID = &sellerID
			} else {
				testCall.SellerID = nil
			}

			err := repo.Create(ctx, testCall)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testCall.ID)
			if err != nil {
				return false
			}

			// Property: SellerID presence should match
			if hasSeller {
				return retrieved.SellerID != nil &&
					testCall.SellerID != nil &&
					*retrieved.SellerID == *testCall.SellerID
			}
			return retrieved.SellerID == nil
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 10}); err != nil {
			t.Error(err)
		}
	})
}
