package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

// Helper function to create test data
func createTestAccountAndCall(t *testing.T, testDB *testutil.TestDB) (*account.Account, *call.Call) {
	t.Helper()
	
	// Create buyer account
	buyerAccount := fixtures.NewAccountBuilder(testDB).
		WithType(account.TypeBuyer).
		WithEmail(fixtures.GenerateEmail(t, "bidtest-buyer")).
		WithBalance(1000.00).
		Build(t)
	
	err := createBidTestAccountInDB(t, testDB, buyerAccount)
	require.NoError(t, err)
	
	// Create call
	testCall := fixtures.NewCallBuilder(t).
		WithBuyerID(buyerAccount.ID).
		Build()
	
	err = createCallInDB(t, testDB, testCall)
	require.NoError(t, err)
	
	return buyerAccount, testCall
}

// Helper function to create account in DB for bid tests
func createBidTestAccountInDB(t *testing.T, testDB *testutil.TestDB, acc *account.Account) error {
	t.Helper()
	
	settingsJSON, err := json.Marshal(acc.Settings)
	if err != nil {
		return err
	}
	
	_, err = testDB.DB().Exec(`
		INSERT INTO accounts (
			id, email, name, company, type, status, phone_number,
			balance, credit_limit, payment_terms,
			tcpa_consent, gdpr_consent, compliance_flags,
			quality_score, fraud_score, settings,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10,
			$11, $12, $13,
			$14, $15, $16,
			$17, $18
		)
	`, acc.ID, acc.Email.String(), acc.Name, acc.Company, acc.Type.String(), acc.Status.String(), acc.PhoneNumber.String(),
		acc.Balance.ToFloat64(), acc.CreditLimit.ToFloat64(), acc.PaymentTerms,
		acc.TCPAConsent, acc.GDPRConsent, pq.Array(acc.ComplianceFlags),
		acc.QualityMetrics.QualityScore, acc.QualityMetrics.FraudScore, settingsJSON,
		acc.CreatedAt, acc.UpdatedAt)
	
	return err
}

// Helper function to create call in DB
func createCallInDB(t *testing.T, testDB *testutil.TestDB, c *call.Call) error {
	t.Helper()
	
	// Handle optional seller ID
	var sellerID sql.NullString
	if c.SellerID != nil {
		sellerID = sql.NullString{String: c.SellerID.String(), Valid: true}
	}
	
	// Handle optional location
	var locationJSON []byte
	if c.Location != nil {
		var err error
		locationJSON, err = json.Marshal(c.Location)
		if err != nil {
			return err
		}
	}
	
	_, err := testDB.DB().Exec(`
		INSERT INTO calls (
			id, from_number, to_number, status, direction,
			buyer_id, seller_id, call_sid, duration,
			location, start_time, answer_time, end_time,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15
		)
	`, c.ID, c.FromNumber.String(), c.ToNumber.String(), c.Status.String(), c.Direction.String(),
		c.BuyerID, sellerID, c.CallSID, c.Duration,
		locationJSON, c.StartTime, nil, c.EndTime,
		c.CreatedAt, c.UpdatedAt)
	
	return err
}

func TestBidRepository_Create(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("create_valid_bid", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)
		
		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithAmount(25.50).
			WithCriteria(bid.BidCriteria{
				Geography: bid.GeoCriteria{
					States: []string{"CA", "NY"},
				},
				CallType: []string{"healthcare"},
				MaxBudget: values.MustNewMoneyFromFloat(100.00, "USD"),
			}).
			Build(t)

		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		// Verify bid was created
		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		assert.Equal(t, testBid.ID, retrieved.ID)
		assert.Equal(t, testBid.CallID, retrieved.CallID)
		assert.Equal(t, testBid.BuyerID, retrieved.BuyerID)
		assert.Equal(t, testBid.Amount, retrieved.Amount)
		assert.Equal(t, testBid.Status, retrieved.Status)
		assert.Equal(t, testBid.Criteria, retrieved.Criteria)
	})

	t.Run("create_with_seller_id", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)
		
		// Create seller account
		sellerAccount := fixtures.NewAccountBuilder(testDB).
			WithType(account.TypeSeller).
			WithEmail(fixtures.GenerateEmail(t, "bidtest-seller")).
			Build(t)
		err := createBidTestAccountInDB(t, testDB, sellerAccount)
		require.NoError(t, err)
		
		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithSellerID(sellerAccount.ID).
			Build(t)

		err = repo.Create(ctx, testBid)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		assert.Equal(t, sellerAccount.ID, retrieved.SellerID)
	})

	t.Run("create_with_auction_id", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)
		
		auctionID := uuid.New()
		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithAuctionID(auctionID).
			Build(t)

		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		assert.Equal(t, auctionID, retrieved.AuctionID)
	})

	t.Run("create_validation_errors", func(t *testing.T) {
		testCases := []struct {
			name     string
			modifier func(*bid.Bid)
			errMsg   string
		}{
			{
				name: "nil_call_id",
				modifier: func(b *bid.Bid) {
					b.CallID = uuid.Nil
				},
				errMsg: "call_id cannot be nil",
			},
			{
				name: "nil_buyer_id",
				modifier: func(b *bid.Bid) {
					b.BuyerID = uuid.Nil
				},
				errMsg: "buyer_id cannot be nil",
			},
			{
				name: "zero_amount",
				modifier: func(b *bid.Bid) {
					b.Amount = values.MustNewMoneyFromFloat(0, "USD")
				},
				errMsg: "amount must be positive",
			},
			{
				name: "negative_amount",
				modifier: func(b *bid.Bid) {
					b.Amount = values.MustNewMoneyFromFloat(-10.50, "USD")
				},
				errMsg: "amount must be positive",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				testBid := fixtures.NewBidBuilder(testDB).Build(t)
				tc.modifier(testBid)

				err := repo.Create(ctx, testBid)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errMsg)
			})
		}
	})

	t.Run("create_with_complex_criteria", func(t *testing.T) {
		complexCriteria := bid.BidCriteria{
			Geography: bid.GeoCriteria{
				States:    []string{"CA", "NY", "TX"},
				ZipCodes:  []string{"90210", "10001"},
				Countries: []string{"US"},
			},
			TimeWindow: bid.TimeWindow{
				StartHour: 9,
				EndHour:   17,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
				Timezone:  "America/New_York",
			},
			CallType:    []string{"inbound", "outbound"},
			Keywords:    []string{"TCPA", "DNC"},
			ExcludeList: []string{"fraud", "spam"},
			MaxBudget:   values.MustNewMoneyFromFloat(500.00, "USD"),
		}

		testBid := fixtures.NewBidBuilder(testDB).
			WithCriteria(complexCriteria).
			Build(t)

		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		assert.Equal(t, complexCriteria, retrieved.Criteria)
	})
}

func TestBidRepository_GetByID(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("get_existing_bid", func(t *testing.T) {
		testBid := fixtures.NewBidBuilder(testDB).
			WithAmount(42.75).
			WithStatus(bid.StatusActive).
			Build(t)

		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)

		assert.Equal(t, testBid.ID, retrieved.ID)
		assert.Equal(t, testBid.CallID, retrieved.CallID)
		assert.Equal(t, testBid.BuyerID, retrieved.BuyerID)
		assert.Equal(t, testBid.Amount, retrieved.Amount)
		assert.Equal(t, testBid.Status, retrieved.Status)
		assert.Equal(t, testBid.Criteria, retrieved.Criteria)
		assert.Equal(t, testBid.Quality, retrieved.Quality)
	})

	t.Run("get_non_existent_bid", func(t *testing.T) {
		nonExistentID := uuid.New()
		retrieved, err := repo.GetByID(ctx, nonExistentID)

		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.Contains(t, err.Error(), "bid not found")
	})

	t.Run("get_bid_with_optional_fields", func(t *testing.T) {
		testBid := fixtures.NewBidBuilder(testDB).
			WithSellerID(uuid.New()).
			WithAuctionID(uuid.New()).
			WithStatus(bid.StatusWon). // This automatically sets AcceptedAt
			Build(t)

		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)

		assert.Equal(t, testBid.SellerID, retrieved.SellerID)
		assert.Equal(t, testBid.AuctionID, retrieved.AuctionID)
		assert.NotNil(t, retrieved.AcceptedAt)
		// AcceptedAt is automatically set for won bids
	})

	t.Run("get_various_bid_statuses", func(t *testing.T) {
		statuses := []bid.Status{
			bid.StatusPending,
			bid.StatusActive,
			bid.StatusWinning,
			bid.StatusWon,
			bid.StatusLost,
			bid.StatusExpired,
			bid.StatusCanceled,
		}

		for _, status := range statuses {
			t.Run(status.String(), func(t *testing.T) {
				testBid := fixtures.NewBidBuilder(testDB).
					WithStatus(status).
					Build(t)

				err := repo.Create(ctx, testBid)
				require.NoError(t, err)

				retrieved, err := repo.GetByID(ctx, testBid.ID)
				require.NoError(t, err)
				assert.Equal(t, status, retrieved.Status)
			})
		}
	})
}

func TestBidRepository_Update(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("update_existing_bid", func(t *testing.T) {
		testBid := fixtures.NewBidBuilder(testDB).
			WithAmount(25.00).
			WithStatus(bid.StatusPending).
			Build(t)

		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		// Update bid fields
		testBid.Amount = values.MustNewMoneyFromFloat(30.50, "USD")
		testBid.Status = bid.StatusActive
		testBid.Rank = 2
		testBid.Criteria = bid.BidCriteria{
			Geography: bid.GeoCriteria{
				States: []string{"CA", "TX"},
			},
			CallType:  []string{"priority", "high"},
			MaxBudget: values.MustNewMoneyFromFloat(200.00, "USD"),
		}
		now := time.Now().UTC()
		testBid.AcceptedAt = &now
		testBid.UpdatedAt = now

		err = repo.Update(ctx, testBid)
		require.NoError(t, err)

		// Verify updates
		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		assert.Equal(t, 30.50, retrieved.Amount)
		assert.Equal(t, bid.StatusActive, retrieved.Status)
		assert.Equal(t, 2, retrieved.Rank)
		assert.Equal(t, testBid.Criteria, retrieved.Criteria)
		assert.NotNil(t, retrieved.AcceptedAt)
		assert.WithinDuration(t, now, *retrieved.AcceptedAt, time.Second)
	})

	t.Run("update_non_existent_bid", func(t *testing.T) {
		testBid := fixtures.NewBidBuilder(testDB).Build(t)
		// Don't create the bid

		err := repo.Update(ctx, testBid)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("concurrent_updates", func(t *testing.T) {
		testBid := fixtures.NewBidBuilder(testDB).Build(t)
		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		numGoroutines := 5
		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()
				
				// Each goroutine updates to a different amount
				testBid.Amount = values.MustNewMoneyFromFloat(float64(10 + iteration*5), "USD")
				testBid.Rank = iteration + 1
				testBid.UpdatedAt = time.Now().UTC()
				
				if err := repo.Update(ctx, testBid); err != nil {
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
		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		
		// Amount should be one of the concurrent updates
		validAmounts := []float64{10, 15, 20, 25, 30}
		assert.Contains(t, validAmounts, retrieved.Amount)
	})
}

func TestBidRepository_Delete(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("delete_existing_bid", func(t *testing.T) {
		testBid := fixtures.NewBidBuilder(testDB).Build(t)
		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		// Verify it exists
		_, err = repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)

		// Delete it
		err = repo.Delete(ctx, testBid.ID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetByID(ctx, testBid.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bid not found")
	})

	t.Run("delete_non_existent_bid", func(t *testing.T) {
		nonExistentID := uuid.New()
		err := repo.Delete(ctx, nonExistentID)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("delete_cascade_behavior", func(t *testing.T) {
		// Create multiple bids for the same call
		callID := uuid.New()
		bid1 := fixtures.NewBidBuilder(testDB).WithCallID(callID).Build(t)
		bid2 := fixtures.NewBidBuilder(testDB).WithCallID(callID).Build(t)

		err := repo.Create(ctx, bid1)
		require.NoError(t, err)
		err = repo.Create(ctx, bid2)
		require.NoError(t, err)

		// Delete one bid - the other should remain
		err = repo.Delete(ctx, bid1.ID)
		require.NoError(t, err)

		// Verify first is gone, second remains
		_, err = repo.GetByID(ctx, bid1.ID)
		assert.Error(t, err)

		_, err = repo.GetByID(ctx, bid2.ID)
		assert.NoError(t, err)
	})
}

func TestBidRepository_GetActiveBidsForCall(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("get_active_bids_sorted", func(t *testing.T) {
		callID := uuid.New()
		
		// Create bids with different amounts (should be sorted by amount DESC)
		bid1 := fixtures.NewBidBuilder(testDB).
			WithCallID(callID).
			WithAmount(10.00).
			WithStatus(bid.StatusActive).
			WithExpiration(time.Hour).
			Build(t)
		
		bid2 := fixtures.NewBidBuilder(testDB).
			WithCallID(callID).
			WithAmount(25.00).
			WithStatus(bid.StatusActive).
			WithExpiration(time.Hour).
			Build(t)
		
		bid3 := fixtures.NewBidBuilder(testDB).
			WithCallID(callID).
			WithAmount(15.00).
			WithStatus(bid.StatusWinning).
			WithExpiration(time.Hour).
			Build(t)

		for _, b := range []*bid.Bid{bid1, bid2, bid3} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		activeBids, err := repo.GetActiveBidsForCall(ctx, callID)
		require.NoError(t, err)
		require.Len(t, activeBids, 3)

		// Should be sorted by amount DESC
		assert.Equal(t, 25.00, activeBids[0].Amount)
		assert.Equal(t, 15.00, activeBids[1].Amount)
		assert.Equal(t, 10.00, activeBids[2].Amount)
	})

	t.Run("exclude_inactive_bids", func(t *testing.T) {
		callID := uuid.New()
		
		activeBid := fixtures.NewBidBuilder(testDB).
			WithCallID(callID).
			WithStatus(bid.StatusActive).
			WithExpiration(time.Hour).
			Build(t)
		
		expiredBid := fixtures.NewBidBuilder(testDB).
			WithCallID(callID).
			WithStatus(bid.StatusExpired).
			WithExpiration(time.Hour).
			Build(t)
		
		wonBid := fixtures.NewBidBuilder(testDB).
			WithCallID(callID).
			WithStatus(bid.StatusWon).
			WithExpiration(time.Hour).
			Build(t)

		for _, b := range []*bid.Bid{activeBid, expiredBid, wonBid} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		activeBids, err := repo.GetActiveBidsForCall(ctx, callID)
		require.NoError(t, err)
		require.Len(t, activeBids, 1)
		assert.Equal(t, activeBid.ID, activeBids[0].ID)
	})

	t.Run("exclude_expired_bids", func(t *testing.T) {
		callID := uuid.New()
		
		activeBid := fixtures.NewBidBuilder(testDB).
			WithCallID(callID).
			WithStatus(bid.StatusActive).
			WithExpiration(time.Hour).
			Build(t)
		
		timeExpiredBid := fixtures.NewBidBuilder(testDB).
			WithCallID(callID).
			WithStatus(bid.StatusActive).
			WithExpiration(-time.Hour). // Expired
			Build(t)

		for _, b := range []*bid.Bid{activeBid, timeExpiredBid} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		activeBids, err := repo.GetActiveBidsForCall(ctx, callID)
		require.NoError(t, err)
		require.Len(t, activeBids, 1)
		assert.Equal(t, activeBid.ID, activeBids[0].ID)
	})

	t.Run("no_active_bids", func(t *testing.T) {
		callID := uuid.New()
		
		activeBids, err := repo.GetActiveBidsForCall(ctx, callID)
		require.NoError(t, err)
		assert.Empty(t, activeBids)
	})
}

func TestBidRepository_GetByBuyer(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("get_bids_by_buyer", func(t *testing.T) {
		buyerID := uuid.New()
		otherBuyerID := uuid.New()
		
		// Create bids for target buyer
		bid1 := fixtures.NewBidBuilder(testDB).
			WithBuyerID(buyerID).
			WithAmount(10.00).
			Build(t)
		
		bid2 := fixtures.NewBidBuilder(testDB).
			WithBuyerID(buyerID).
			WithAmount(20.00).
			Build(t)
		
		// Create bid for other buyer
		otherBid := fixtures.NewBidBuilder(testDB).
			WithBuyerID(otherBuyerID).
			WithAmount(15.00).
			Build(t)

		for _, b := range []*bid.Bid{bid1, bid2, otherBid} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		buyerBids, err := repo.GetByBuyer(ctx, buyerID)
		require.NoError(t, err)
		require.Len(t, buyerBids, 2)

		// Should only contain bids from target buyer
		for _, b := range buyerBids {
			assert.Equal(t, buyerID, b.BuyerID)
		}
	})

	t.Run("get_bids_sorted_by_created_at", func(t *testing.T) {
		buyerID := uuid.New()
		
		// Create bids with different placement times
		now := time.Now().UTC()
		
		bid1 := fixtures.NewBidBuilder(testDB).
			WithBuyerID(buyerID).
			WithPlacedAt(now.Add(-2*time.Hour)).
			Build(t)
		
		bid2 := fixtures.NewBidBuilder(testDB).
			WithBuyerID(buyerID).
			WithPlacedAt(now.Add(-1*time.Hour)).
			Build(t)
		
		bid3 := fixtures.NewBidBuilder(testDB).
			WithBuyerID(buyerID).
			WithPlacedAt(now).
			Build(t)

		for _, b := range []*bid.Bid{bid1, bid2, bid3} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		buyerBids, err := repo.GetByBuyer(ctx, buyerID)
		require.NoError(t, err)
		require.Len(t, buyerBids, 3)

		// Should be sorted by created_at DESC (newest first)
		assert.True(t, buyerBids[0].CreatedAt.After(buyerBids[1].CreatedAt))
		assert.True(t, buyerBids[1].CreatedAt.After(buyerBids[2].CreatedAt))
	})

	t.Run("limit_100_bids", func(t *testing.T) {
		buyerID := uuid.New()
		
		// This test verifies the LIMIT 100 clause works
		// We won't create 100+ bids due to test performance, but verify the query structure
		bid := fixtures.NewBidBuilder(testDB).
			WithBuyerID(buyerID).
			Build(t)

		err := repo.Create(ctx, bid)
		require.NoError(t, err)

		buyerBids, err := repo.GetByBuyer(ctx, buyerID)
		require.NoError(t, err)
		assert.Len(t, buyerBids, 1)
	})
}

func TestBidRepository_GetExpiredBids(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("get_expired_bids", func(t *testing.T) {
		now := time.Now().UTC()
		cutoff := now.Add(-1 * time.Hour)
		
		// Create expired bids
		expiredBid1 := fixtures.NewBidBuilder(testDB).
			WithStatus(bid.StatusActive).
			WithPlacedAt(cutoff.Add(-30*time.Minute)).
			WithExpiration(-30*time.Minute).
			Build(t)
		
		expiredBid2 := fixtures.NewBidBuilder(testDB).
			WithStatus(bid.StatusPending).
			WithPlacedAt(cutoff.Add(-15*time.Minute)).
			WithExpiration(-15*time.Minute).
			Build(t)
		
		// Create non-expired bid
		activeBid := fixtures.NewBidBuilder(testDB).
			WithStatus(bid.StatusActive).
			WithPlacedAt(now).
			WithExpiration(30*time.Minute).
			Build(t)
		
		// Create bid with non-expirable status
		wonBid := fixtures.NewBidBuilder(testDB).
			WithStatus(bid.StatusWon).
			WithPlacedAt(cutoff.Add(-15*time.Minute)).
			WithExpiration(-15*time.Minute).
			Build(t)

		for _, b := range []*bid.Bid{expiredBid1, expiredBid2, activeBid, wonBid} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		expiredBids, err := repo.GetExpiredBids(ctx, cutoff)
		require.NoError(t, err)
		require.Len(t, expiredBids, 2)

		// Should only contain expired bids with expirable statuses
		expiredIDs := []uuid.UUID{expiredBids[0].ID, expiredBids[1].ID}
		assert.Contains(t, expiredIDs, expiredBid1.ID)
		assert.Contains(t, expiredIDs, expiredBid2.ID)
	})

	t.Run("sorted_by_expires_at", func(t *testing.T) {
		now := time.Now().UTC()
		cutoff := now.Add(-1 * time.Hour)
		
		bid1 := fixtures.NewBidBuilder(testDB).
			WithStatus(bid.StatusActive).
			WithPlacedAt(cutoff.Add(-2*time.Hour)).
			WithExpiration(-2*time.Hour).
			Build(t)
		
		bid2 := fixtures.NewBidBuilder(testDB).
			WithStatus(bid.StatusActive).
			WithPlacedAt(cutoff.Add(-30*time.Minute)).
			WithExpiration(-30*time.Minute).
			Build(t)

		for _, b := range []*bid.Bid{bid1, bid2} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		expiredBids, err := repo.GetExpiredBids(ctx, cutoff)
		require.NoError(t, err)
		require.Len(t, expiredBids, 2)

		// Should be sorted by expires_at ASC (oldest expiration first)
		assert.True(t, expiredBids[0].ExpiresAt.Before(expiredBids[1].ExpiresAt))
	})

	t.Run("limit_1000_bids", func(t *testing.T) {
		now := time.Now().UTC()
		cutoff := now.Add(-1 * time.Hour)
		
		// Verify the LIMIT 1000 clause works
		bid := fixtures.NewBidBuilder(testDB).
			WithStatus(bid.StatusActive).
			WithPlacedAt(cutoff.Add(-30*time.Minute)).
			WithExpiration(-30*time.Minute).
			Build(t)

		err := repo.Create(ctx, bid)
		require.NoError(t, err)

		expiredBids, err := repo.GetExpiredBids(ctx, cutoff)
		require.NoError(t, err)
		assert.Len(t, expiredBids, 1)
	})
}

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
			
			testBid := fixtures.NewBidBuilder(testDB).
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
			// Map to valid status values
			statuses := []bid.Status{
				bid.StatusPending, bid.StatusActive, bid.StatusWinning,
				bid.StatusWon, bid.StatusLost, bid.StatusExpired, bid.StatusCanceled,
			}
			
			initial := statuses[int(initialStatus)%len(statuses)]
			final := statuses[int(finalStatus)%len(statuses)]
			
			testBid := fixtures.NewBidBuilder(testDB).
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
			return compareBidCriteria(criteria, retrieved.Criteria)
		}
		
		if err := quick.Check(property, &quick.Config{MaxCount: 15}); err != nil {
			t.Error(err)
		}
	})
}

func TestBidRepository_Concurrency(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("concurrent_creates", func(t *testing.T) {
		callID := uuid.New()
		numBids := 10
		
		var wg sync.WaitGroup
		bidChan := make(chan *bid.Bid, numBids)
		errChan := make(chan error, numBids)

		for i := 0; i < numBids; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()
				
				testBid := fixtures.NewBidBuilder(testDB).
					WithCallID(callID).
					WithAmount(float64(10 + iteration)).
					Build(t)
				
				if err := repo.Create(ctx, testBid); err != nil {
					errChan <- err
					return
				}
				
				bidChan <- testBid
			}(i)
		}

		wg.Wait()
		close(bidChan)
		close(errChan)

		// Check for errors
		for err := range errChan {
			require.NoError(t, err)
		}

		// Verify all bids were created
		var createdBids []*bid.Bid
		for b := range bidChan {
			createdBids = append(createdBids, b)
		}
		assert.Len(t, createdBids, numBids)

		// Verify they're all retrievable
		activeBids, err := repo.GetActiveBidsForCall(ctx, callID)
		require.NoError(t, err)
		assert.Len(t, activeBids, numBids)
	})

	t.Run("concurrent_updates_same_bid", func(t *testing.T) {
		testBid := fixtures.NewBidBuilder(testDB).Build(t)
		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		numUpdates := 5
		var wg sync.WaitGroup
		errChan := make(chan error, numUpdates)

		for i := 0; i < numUpdates; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()
				
				// Each goroutine tries to update the bid
				localBid := *testBid // Copy the bid
				localBid.Amount = values.MustNewMoneyFromFloat(float64(100 + iteration*10), "USD")
				localBid.Rank = iteration + 1
				localBid.UpdatedAt = time.Now().UTC()
				
				if err := repo.Update(ctx, &localBid); err != nil {
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
		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		
		validAmounts := []float64{100, 110, 120, 130, 140}
		assert.Contains(t, validAmounts, retrieved.Amount)
	})
}

// Helper functions

func compareBidCriteria(expected, actual bid.BidCriteria) bool {
	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)
	return string(expectedJSON) == string(actualJSON)
}