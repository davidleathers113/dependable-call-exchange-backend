package repository

import (
	"context"
	"encoding/json"
	"testing"
	"testing/quick"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestBidRepository_PropertyBased(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("amount_consistency", func(t *testing.T) {
		// Property: Amount should always be preserved accurately
		property := func(amount float64) bool {
			// Normalize to positive amount in reasonable range
			if amount <= 0 || amount > 10000 {
				return true // Skip invalid amounts
			}

			// Create parent entities
			buyerAccount, testCall := createTestAccountAndCall(t, testDB)

			testBid := fixtures.NewBidBuilder(testDB).
				WithCallID(testCall.ID).
				WithBuyerID(buyerAccount.ID).
				WithAmount(amount).
				Build(t)

			err := repo.Create(ctx, testBid)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testBid.ID)
			if err != nil {
				return false
			}

			// Property: Amount should be preserved exactly
			return retrieved.Amount.ToFloat64() == amount
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
			t.Error(err)
		}
	})

	t.Run("status_transitions", func(t *testing.T) {
		// Property: Status updates should always be persisted correctly
		property := func(initialStatus, finalStatus uint8) bool {
			// Map to valid status values (only those supported by production schema)
			statuses := []bid.Status{
				bid.StatusActive, bid.StatusWon, bid.StatusLost, bid.StatusExpired,
			}

			initial := statuses[int(initialStatus)%len(statuses)]
			final := statuses[int(finalStatus)%len(statuses)]

			// Create parent entities
			buyerAccount, testCall := createTestAccountAndCall(t, testDB)

			testBid := fixtures.NewBidBuilder(testDB).
				WithCallID(testCall.ID).
				WithBuyerID(buyerAccount.ID).
				WithStatus(initial).
				Build(t)

			err := repo.Create(ctx, testBid)
			if err != nil {
				return false
			}

			// Update status
			testBid.Status = final
			testBid.UpdatedAt = time.Now().UTC()

			err = repo.Update(ctx, testBid)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testBid.ID)
			if err != nil {
				return false
			}

			// Property: Status should be updated correctly
			return retrieved.Status == final
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 30}); err != nil {
			t.Error(err)
		}
	})

	t.Run("criteria_json_roundtrip", func(t *testing.T) {
		// Property: Complex criteria should survive JSON roundtrip
		property := func(states []string, callTypes []string, maxBudget float64) bool {
			if len(states) == 0 || len(states) > 10 {
				return true // Skip edge cases
			}
			if len(callTypes) == 0 || len(callTypes) > 5 {
				return true // Skip edge cases
			}
			if maxBudget < 0 || maxBudget > 10000 {
				return true // Skip invalid ranges
			}

			// Create parent entities
			buyerAccount, testCall := createTestAccountAndCall(t, testDB)

			criteria := bid.BidCriteria{
				Geography: bid.GeoCriteria{
					States:    states,
					Countries: []string{"US"},
				},
				TimeWindow: bid.TimeWindow{
					StartHour: 9,
					EndHour:   17,
					Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
					Timezone:  "America/New_York",
				},
				CallType:  callTypes,
				MaxBudget: values.MustNewMoneyFromFloat(maxBudget, "USD"),
			}

			testBid := fixtures.NewBidBuilder(testDB).
				WithCallID(testCall.ID).
				WithBuyerID(buyerAccount.ID).
				WithCriteria(criteria).
				Build(t)

			err := repo.Create(ctx, testBid)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testBid.ID)
			if err != nil {
				return false
			}

			// Property: Criteria should be preserved exactly
			// Compare MaxBudget separately due to JSON marshaling precision
			if criteria.MaxBudget.ToFloat64() != retrieved.Criteria.MaxBudget.ToFloat64() {
				return false
			}
			if criteria.MaxBudget.Currency() != retrieved.Criteria.MaxBudget.Currency() {
				return false
			}

			// Compare other fields
			criteriaWithoutMoney := criteria
			retrievedWithoutMoney := retrieved.Criteria
			criteriaWithoutMoney.MaxBudget = values.Zero("USD")
			retrievedWithoutMoney.MaxBudget = values.Zero("USD")

			expectedJSON, _ := json.Marshal(criteriaWithoutMoney)
			actualJSON, _ := json.Marshal(retrievedWithoutMoney)
			return string(expectedJSON) == string(actualJSON)
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 15}); err != nil {
			t.Error(err)
		}
	})

	t.Run("rank_persistence", func(t *testing.T) {
		// Property: Bid rank should be persisted and retrieved correctly
		property := func(rank int) bool {
			// Normalize rank to reasonable range
			if rank < 0 || rank > 100 {
				return true // Skip invalid ranks
			}

			// Create parent entities
			buyerAccount, testCall := createTestAccountAndCall(t, testDB)

			testBid := fixtures.NewBidBuilder(testDB).
				WithCallID(testCall.ID).
				WithBuyerID(buyerAccount.ID).
				WithRank(rank).
				Build(t)

			err := repo.Create(ctx, testBid)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testBid.ID)
			if err != nil {
				return false
			}

			// Property: Rank should be preserved exactly
			return retrieved.Rank == rank
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
			t.Error(err)
		}
	})

	t.Run("quality_score_range", func(t *testing.T) {
		// Property: Quality scores should be within valid range
		property := func(qualityScore, fraudScore float64) bool {
			// Normalize to valid range (0-10 for quality metrics)
			if qualityScore < 0 || qualityScore > 10 {
				return true // Skip invalid scores
			}
			if fraudScore < 0 || fraudScore > 10 {
				return true // Skip invalid scores
			}

			// Create parent entities
			buyerAccount, testCall := createTestAccountAndCall(t, testDB)

			quality := values.MustNewQualityMetrics(
				qualityScore, // quality_score
				fraudScore,   // fraud_score
				5.0,          // historical_rating
				0.5,          // conversion_rate
				180,          // average_call_time
				7.0,          // trust_score
				6.5,          // reliability_score
			)

			testBid := fixtures.NewBidBuilder(testDB).
				WithCallID(testCall.ID).
				WithBuyerID(buyerAccount.ID).
				WithQuality(quality).
				Build(t)

			err := repo.Create(ctx, testBid)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testBid.ID)
			if err != nil {
				return false
			}

			// Property: Quality metrics should be preserved exactly
			return retrieved.Quality.QualityScore == qualityScore &&
				retrieved.Quality.FraudScore == fraudScore &&
				retrieved.Quality.HistoricalRating == 5.0 &&
				retrieved.Quality.ConversionRate == 0.5 &&
				retrieved.Quality.AverageCallTime == 180 &&
				retrieved.Quality.TrustScore == 7.0 &&
				retrieved.Quality.ReliabilityScore == 6.5
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 25}); err != nil {
			t.Error(err)
		}
	})

	t.Run("timestamp_ordering_invariants", func(t *testing.T) {
		// Property: Timestamps should maintain logical ordering
		property := func(hoursBefore uint) bool {
			// Limit to reasonable time range
			if hoursBefore > 1000 {
				return true
			}

			// Create parent entities
			buyerAccount, testCall := createTestAccountAndCall(t, testDB)

			now := time.Now().UTC()
			placedAt := now.Add(-time.Duration(hoursBefore) * time.Hour)

			testBid := fixtures.NewBidBuilder(testDB).
				WithCallID(testCall.ID).
				WithBuyerID(buyerAccount.ID).
				WithPlacedAt(placedAt).
				WithExpiration(time.Hour * 24). // Expires 24 hours after placement
				Build(t)

			err := repo.Create(ctx, testBid)
			if err != nil {
				return false
			}

			retrieved, err := repo.GetByID(ctx, testBid.ID)
			if err != nil {
				return false
			}

			// Properties to verify:
			// 1. PlacedAt <= CreatedAt (bid placed before or at creation)
			// 2. CreatedAt <= UpdatedAt (created before or at update)
			// 3. PlacedAt < ExpiresAt (placed before expiration)
			return !retrieved.PlacedAt.After(retrieved.CreatedAt) &&
				!retrieved.CreatedAt.After(retrieved.UpdatedAt) &&
				retrieved.PlacedAt.Before(retrieved.ExpiresAt)
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
			t.Error(err)
		}
	})
}
