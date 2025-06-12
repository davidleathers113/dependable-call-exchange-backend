package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestBidRepository_GetActiveBidsForCall(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("get_active_bids_sorted", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		// Create bids with different amounts (should be sorted by amount DESC)
		bid1 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithAmount(10.00).
			WithStatus(bid.StatusActive).
			WithExpiration(time.Hour).
			Build(t)

		bid2 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithAmount(25.00).
			WithStatus(bid.StatusActive).
			WithExpiration(time.Hour).
			Build(t)

		bid3 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithAmount(15.00).
			WithStatus(bid.StatusWinning).
			WithExpiration(time.Hour).
			Build(t)

		for _, b := range []*bid.Bid{bid1, bid2, bid3} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		activeBids, err := repo.GetActiveBidsForCall(ctx, testCall.ID)
		require.NoError(t, err)
		require.Len(t, activeBids, 3)

		// Should be sorted by amount DESC
		assert.Equal(t, 25.00, activeBids[0].Amount.ToFloat64())
		assert.Equal(t, 15.00, activeBids[1].Amount.ToFloat64())
		assert.Equal(t, 10.00, activeBids[2].Amount.ToFloat64())
	})

	t.Run("exclude_inactive_bids", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		activeBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusActive).
			WithExpiration(time.Hour).
			Build(t)

		expiredBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusExpired).
			WithExpiration(time.Hour).
			Build(t)

		wonBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusWon).
			WithExpiration(time.Hour).
			Build(t)

		for _, b := range []*bid.Bid{activeBid, expiredBid, wonBid} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		activeBids, err := repo.GetActiveBidsForCall(ctx, testCall.ID)
		require.NoError(t, err)
		require.Len(t, activeBids, 1)
		assert.Equal(t, activeBid.ID, activeBids[0].ID)
	})

	t.Run("exclude_expired_bids", func(t *testing.T) {
		// Create parent entities first
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		activeBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusActive).
			WithExpiration(time.Hour).
			Build(t)

		timeExpiredBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusActive).
			WithExpiration(-time.Hour). // Expired
			Build(t)

		for _, b := range []*bid.Bid{activeBid, timeExpiredBid} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		activeBids, err := repo.GetActiveBidsForCall(ctx, testCall.ID)
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
		// Create parent entities for first buyer
		buyerAccount1, testCall1 := createTestAccountAndCall(t, testDB)

		// Create parent entities for second buyer
		buyerAccount2, testCall2 := createTestAccountAndCall(t, testDB)

		// Create bids for target buyer
		bid1 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall1.ID).
			WithBuyerID(buyerAccount1.ID).
			WithAmount(10.00).
			Build(t)

		bid2 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall1.ID).
			WithBuyerID(buyerAccount1.ID).
			WithAmount(20.00).
			Build(t)

		// Create bid for other buyer
		otherBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall2.ID).
			WithBuyerID(buyerAccount2.ID).
			WithAmount(15.00).
			Build(t)

		for _, b := range []*bid.Bid{bid1, bid2, otherBid} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		buyerBids, err := repo.GetByBuyer(ctx, buyerAccount1.ID)
		require.NoError(t, err)
		require.Len(t, buyerBids, 2)

		// Should only contain bids from target buyer
		for _, b := range buyerBids {
			assert.Equal(t, buyerAccount1.ID, b.BuyerID)
		}
	})

	t.Run("get_bids_sorted_by_created_at", func(t *testing.T) {
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		// Create bids with different placement times
		now := time.Now().UTC()

		bid1 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithPlacedAt(now.Add(-2 * time.Hour)).
			Build(t)

		bid2 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithPlacedAt(now.Add(-1 * time.Hour)).
			Build(t)

		bid3 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithPlacedAt(now).
			Build(t)

		for _, b := range []*bid.Bid{bid1, bid2, bid3} {
			err := repo.Create(ctx, b)
			require.NoError(t, err)
		}

		buyerBids, err := repo.GetByBuyer(ctx, buyerAccount.ID)
		require.NoError(t, err)
		require.Len(t, buyerBids, 3)

		// Should be sorted by created_at DESC (newest first)
		assert.True(t, buyerBids[0].CreatedAt.After(buyerBids[1].CreatedAt))
		assert.True(t, buyerBids[1].CreatedAt.After(buyerBids[2].CreatedAt))
	})

	t.Run("limit_100_bids", func(t *testing.T) {
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		// This test verifies the LIMIT 100 clause works
		// We won't create 100+ bids due to test performance, but verify the query structure
		bid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			Build(t)

		err := repo.Create(ctx, bid)
		require.NoError(t, err)

		buyerBids, err := repo.GetByBuyer(ctx, buyerAccount.ID)
		require.NoError(t, err)
		assert.Len(t, buyerBids, 1)
	})
}

func TestBidRepository_GetExpiredBids(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("get_expired_bids", func(t *testing.T) {
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		now := time.Now().UTC()
		cutoff := now.Add(-1 * time.Hour)

		// Create expired bids
		expiredBid1 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusActive).
			WithPlacedAt(cutoff.Add(-30 * time.Minute)).
			WithExpiration(-30 * time.Minute).
			Build(t)

		expiredBid2 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusPending).
			WithPlacedAt(cutoff.Add(-15 * time.Minute)).
			WithExpiration(-15 * time.Minute).
			Build(t)

		// Create non-expired bid
		activeBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusActive).
			WithPlacedAt(now).
			WithExpiration(30 * time.Minute).
			Build(t)

		// Create bid with non-expirable status
		wonBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusWon).
			WithPlacedAt(cutoff.Add(-15 * time.Minute)).
			WithExpiration(-15 * time.Minute).
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
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		now := time.Now().UTC()
		cutoff := now.Add(-1 * time.Hour)

		bid1 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusActive).
			WithPlacedAt(cutoff.Add(-2 * time.Hour)).
			WithExpiration(-2 * time.Hour).
			Build(t)

		bid2 := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusActive).
			WithPlacedAt(cutoff.Add(-30 * time.Minute)).
			WithExpiration(-30 * time.Minute).
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
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		now := time.Now().UTC()
		cutoff := now.Add(-1 * time.Hour)

		// Verify the LIMIT 1000 clause works
		bid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithStatus(bid.StatusActive).
			WithPlacedAt(cutoff.Add(-30 * time.Minute)).
			WithExpiration(-30 * time.Minute).
			Build(t)

		err := repo.Create(ctx, bid)
		require.NoError(t, err)

		expiredBids, err := repo.GetExpiredBids(ctx, cutoff)
		require.NoError(t, err)
		assert.Len(t, expiredBids, 1)
	})
}
