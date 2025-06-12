package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestBidRepository_Concurrency(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewBidRepository(testDB.DB())

	t.Run("concurrent_creates", func(t *testing.T) {
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		numBids := 10

		var wg sync.WaitGroup
		bidChan := make(chan *bid.Bid, numBids)
		errChan := make(chan error, numBids)

		for i := 0; i < numBids; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()

				testBid := fixtures.NewBidBuilder(testDB).
					WithCallID(testCall.ID).
					WithBuyerID(buyerAccount.ID).
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
		activeBids, err := repo.GetActiveBidsForCall(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Len(t, activeBids, numBids)
	})

	t.Run("concurrent_updates_same_bid", func(t *testing.T) {
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			Build(t)
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
				localBid.Amount = values.MustNewMoneyFromFloat(float64(100+iteration*10), "USD")
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
		assert.Contains(t, validAmounts, retrieved.Amount.ToFloat64())
	})

	t.Run("concurrent_creates_different_calls", func(t *testing.T) {
		numCalls := 5
		numBidsPerCall := 3

		var wg sync.WaitGroup
		errChan := make(chan error, numCalls*numBidsPerCall)

		for i := 0; i < numCalls; i++ {
			// Create parent entities for each call
			buyerAccount, testCall := createTestAccountAndCall(t, testDB)

			for j := 0; j < numBidsPerCall; j++ {
				wg.Add(1)
				go func(callIdx, bidIdx int) {
					defer wg.Done()

					testBid := fixtures.NewBidBuilder(testDB).
						WithCallID(testCall.ID).
						WithBuyerID(buyerAccount.ID).
						WithAmount(float64(10 + callIdx*10 + bidIdx)).
						Build(t)

					if err := repo.Create(ctx, testBid); err != nil {
						errChan <- err
					}
				}(i, j)
			}
		}

		wg.Wait()
		close(errChan)

		// Check for errors
		for err := range errChan {
			require.NoError(t, err)
		}
	})

	t.Run("concurrent_reads_and_writes", func(t *testing.T) {
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		// Create initial bid
		testBid := fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(buyerAccount.ID).
			WithAmount(50.00).
			Build(t)
		err := repo.Create(ctx, testBid)
		require.NoError(t, err)

		numOperations := 10
		var wg sync.WaitGroup
		errChan := make(chan error, numOperations*2)

		// Start readers
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Read operations
				_, err := repo.GetByID(ctx, testBid.ID)
				if err != nil {
					errChan <- err
				}

				_, err = repo.GetActiveBidsForCall(ctx, testCall.ID)
				if err != nil {
					errChan <- err
				}
			}()
		}

		// Start writers
		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(iteration int) {
				defer wg.Done()

				// Update operation
				localBid := *testBid
				localBid.Amount = values.MustNewMoneyFromFloat(float64(50+iteration), "USD")
				localBid.UpdatedAt = time.Now().UTC()

				if err := repo.Update(ctx, &localBid); err != nil {
					errChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// All operations should succeed
		for err := range errChan {
			assert.NoError(t, err)
		}
	})

	t.Run("concurrent_deletes", func(t *testing.T) {
		// Create parent entities
		buyerAccount, testCall := createTestAccountAndCall(t, testDB)

		// Create multiple bids
		numBids := 5
		bids := make([]*bid.Bid, numBids)

		for i := 0; i < numBids; i++ {
			testBid := fixtures.NewBidBuilder(testDB).
				WithCallID(testCall.ID).
				WithBuyerID(buyerAccount.ID).
				WithAmount(float64(10 + i*5)).
				Build(t)

			err := repo.Create(ctx, testBid)
			require.NoError(t, err)
			bids[i] = testBid
		}

		var wg sync.WaitGroup
		successCount := int32(0)
		var successMutex sync.Mutex

		// Try to delete all bids concurrently
		for i := 0; i < numBids; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				err := repo.Delete(ctx, bids[idx].ID)
				if err == nil {
					successMutex.Lock()
					successCount++
					successMutex.Unlock()
				}
			}(i)
		}

		wg.Wait()

		// All deletes should succeed
		assert.Equal(t, int32(numBids), successCount)

		// Verify all bids are gone
		for _, b := range bids {
			_, err := repo.GetByID(ctx, b.ID)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "bid not found")
		}
	})
}
