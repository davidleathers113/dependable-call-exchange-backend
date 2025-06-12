package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

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
				CallType:  []string{"healthcare"},
				MaxBudget: values.MustNewMoneyFromFloat(100.00, "USD"),
			}).
			Build(t)

		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		// Verify bid was created
		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		assertBidEquals(t, testBid, retrieved)
	})

	t.Run("create_with_seller_id", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		// Create seller account
		sellerAccount := fixtures.NewAccountBuilder(testDB).
			WithType(account.TypeSeller).
			WithEmail(fixtures.GenerateEmail(t, "bidtest-seller")).
			Build(t)
		err := createAccountInDBHelper(t, testDB, sellerAccount)
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
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

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
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithCriteria(complexCriteria).
			Build(t)

		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)
		assertBidCriteriaEquals(t, complexCriteria, retrieved.Criteria)
	})
}

func TestBidRepository_GetByID(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("get_existing_bid", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
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
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		// Create seller account
		sellerAccount := fixtures.NewAccountBuilder(testDB).
			WithType(account.TypeSeller).
			WithEmail(fixtures.GenerateEmail(t, "bidtest-seller-optional")).
			Build(t)
		err := createAccountInDBHelper(t, testDB, sellerAccount)
		require.NoError(t, err)

		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithSellerID(sellerAccount.ID).
			WithAuctionID(uuid.New()).
			WithStatus(bid.StatusWon). // This automatically sets AcceptedAt
			Build(t)

		err = repo.Create(ctx, testBid)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testBid.ID)
		require.NoError(t, err)

		assert.Equal(t, testBid.SellerID, retrieved.SellerID)
		assert.Equal(t, testBid.AuctionID, retrieved.AuctionID)
		assert.NotNil(t, retrieved.AcceptedAt)
		// AcceptedAt is automatically set for won bids
	})

	t.Run("get_various_bid_statuses", func(t *testing.T) {
		// Only test statuses that exist in the production database
		statuses := []bid.Status{
			bid.StatusActive,
			bid.StatusWon,
			bid.StatusLost,
			bid.StatusExpired,
		}

		for _, status := range statuses {
			t.Run(status.String(), func(t *testing.T) {
				// Create parent entities first
				buyerAccount, testCall := createTestAccountAndCall(t, testDB)

				testBid := fixtures.NewBidBuilder(testDB).
					WithCallID(testCall.ID).
					WithBuyerID(buyerAccount.ID).
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
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithAmount(25.00).
			WithStatus(bid.StatusActive).
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
		assert.Equal(t, 30.50, retrieved.Amount.ToFloat64())
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
}

func TestBidRepository_Delete(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("delete_existing_bid", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			Build(t)
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
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		// Create multiple bids for the same call
		bid1 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			Build(t)
		bid2 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			Build(t)

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
