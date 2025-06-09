package repository

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestCallRepository_Create(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	repo := NewCallRepository(testDB.DB())
	ctx := testutil.TestContext(t)

	tests := []struct {
		name    string
		setup   func() *call.Call
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid call",
			setup: func() *call.Call {
				// Create test data with proper relationships
				testData := fixtures.CreateMinimalTestSet(t, testDB)
				return fixtures.NewCallBuilder(t).
					WithBuyerID(testData.BuyerAccount.ID).
					Build()
			},
			wantErr: false,
		},
		{
			name: "duplicate ID",
			setup: func() *call.Call {
				testData := fixtures.CreateMinimalTestSet(t, testDB)
				c := fixtures.NewCallBuilder(t).
					WithBuyerID(testData.BuyerAccount.ID).
					Build()
				// Insert first call
				err := repo.Create(ctx, c)
				require.NoError(t, err)
				// Return call with same ID
				return fixtures.NewCallBuilder(t).
					WithID(c.ID).
					WithBuyerID(testData.BuyerAccount.ID).
					Build()
			},
			wantErr: true,
			errMsg:  "duplicate key",
		},
		{
			name: "empty phone number",
			setup: func() *call.Call {
				testData := fixtures.CreateMinimalTestSet(t, testDB)
				return fixtures.NewCallBuilder(t).
					WithPhoneNumbers("", "+15559876543").
					WithBuyerID(testData.BuyerAccount.ID).
					Build()
			},
			wantErr: true,
			errMsg:  "from_number cannot be empty",
		},
		{
			name: "nil buyer ID",
			setup: func() *call.Call {
				c := fixtures.NewCallBuilder(t).Build()
				c.BuyerID = uuid.Nil
				return c
			},
			wantErr: true,
			errMsg:  "buyer_id cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name != "duplicate ID" {
				testDB.TruncateTables() // Clean state for each test except duplicate ID test
			}

			testCall := tt.setup()
			err := repo.Create(ctx, testCall)
			
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				
				// Verify the call was created in the database
				retrieved, err := repo.GetByID(ctx, testCall.ID)
				require.NoError(t, err)
				assert.Equal(t, testCall.ID, retrieved.ID)
				assert.Equal(t, testCall.FromNumber, retrieved.FromNumber)
				assert.Equal(t, testCall.ToNumber, retrieved.ToNumber)
				assert.Equal(t, testCall.Status, retrieved.Status)
				assert.Equal(t, testCall.BuyerID, retrieved.BuyerID)
			}
		})
	}
}

func TestCallRepository_GetByID(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	repo := NewCallRepository(testDB.DB())
	ctx := testutil.TestContext(t)

	// Create test data
	fixtures.WithMinimalData(t, testDB, func(testData *fixtures.TestDataSet) {
		// Create test call
		testCall := fixtures.NewCallBuilder(t).
			WithStatus(call.StatusInProgress).
			WithBuyerID(testData.BuyerAccount.ID).
			WithLocation(&call.Location{
				Country: "US",
				State:   "CA",
				City:    "Los Angeles",
			}).
			Build()

		// Insert the call
		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		tests := []struct {
			name    string
			id      uuid.UUID
			want    *call.Call
			wantErr bool
			errType error
		}{
			{
				name:    "existing call",
				id:      testCall.ID,
				want:    testCall,
				wantErr: false,
			},
			{
				name:    "non-existent call",
				id:      uuid.New(),
				wantErr: true,
				errType: sql.ErrNoRows,
			},
			{
				name:    "nil UUID",
				id:      uuid.Nil,
				wantErr: true,
				errType: sql.ErrNoRows,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := repo.GetByID(ctx, tt.id)

				if tt.wantErr {
					assert.Error(t, err)
					if tt.errType != nil {
						assert.ErrorIs(t, err, tt.errType)
					}
					assert.Nil(t, got)
				} else {
					assert.NoError(t, err)
					require.NotNil(t, got)
					assert.Equal(t, tt.want.ID, got.ID)
					assert.Equal(t, tt.want.FromNumber, got.FromNumber)
					assert.Equal(t, tt.want.ToNumber, got.ToNumber)
					assert.Equal(t, tt.want.Status, got.Status)
				}
			})
		}
	})
}

func TestCallRepository_Update(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	repo := NewCallRepository(testDB.DB())
	ctx := testutil.TestContext(t)

	// Create test data
	fixtures.WithMinimalData(t, testDB, func(testData *fixtures.TestDataSet) {
		// Create initial call
		initialCall := fixtures.NewCallBuilder(t).
			WithStatus(call.StatusPending).
			WithBuyerID(testData.BuyerAccount.ID).
			Build()

		err := repo.Create(ctx, initialCall)
		require.NoError(t, err)
		
		// Fetch the call to get the actual database timestamp
		initialCall, err = repo.GetByID(ctx, initialCall.ID)
		require.NoError(t, err)

		tests := []struct {
			name    string
			update  func(*call.Call)
			wantErr bool
			verify  func(t *testing.T, updated *call.Call)
		}{
			{
				name: "update status",
				update: func(c *call.Call) {
					// Add small delay to ensure UpdatedAt changes
					time.Sleep(10 * time.Millisecond)
					c.UpdateStatus(call.StatusInProgress)
				},
				wantErr: false,
				verify: func(t *testing.T, updated *call.Call) {
					assert.Equal(t, call.StatusInProgress, updated.Status)
					assert.True(t, updated.UpdatedAt.After(initialCall.UpdatedAt))
				},
			},
			{
				name: "complete call",
				update: func(c *call.Call) {
					c.Complete(300, 5.50) // 5 minutes, $5.50
				},
				wantErr: false,
				verify: func(t *testing.T, updated *call.Call) {
					assert.Equal(t, call.StatusCompleted, updated.Status)
					assert.NotNil(t, updated.EndTime)
					assert.NotNil(t, updated.Duration)
					assert.Equal(t, 300, *updated.Duration)
					assert.NotNil(t, updated.Cost)
					assert.Equal(t, 5.50, *updated.Cost)
				},
			},
			{
				name: "update non-existent call",
				update: func(c *call.Call) {
					c.ID = uuid.New() // Change to non-existent ID
					c.UpdateStatus(call.StatusFailed)
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Get fresh copy for each test
				callToUpdate, err := repo.GetByID(ctx, initialCall.ID)
				require.NoError(t, err)

				// Apply update
				tt.update(callToUpdate)

				// Update in repository
				err = repo.Update(ctx, callToUpdate)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)

					// Retrieve and verify
					updated, err := repo.GetByID(ctx, initialCall.ID)
					require.NoError(t, err)
					tt.verify(t, updated)
				}
			})
		}
	})
}

func TestCallRepository_List(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	repo := NewCallRepository(testDB.DB())
	ctx := testutil.TestContext(t)

	// Create complete test data set
	fixtures.WithTestData(t, testDB, func(testData *fixtures.TestDataSet) {
		// Create additional calls using the test scenarios
		scenarios := fixtures.NewCallScenarios(t)
		calls := []*call.Call{
			scenarios.InboundCall(),
			scenarios.OutboundCall(),
			scenarios.ActiveCall(),
			scenarios.CompletedCall(),
			scenarios.FailedCall(),
		}

		// Update calls to use valid buyer/seller IDs from test data
		for i, c := range calls {
			if i%2 == 0 {
				c.BuyerID = testData.BuyerAccount.ID
				c.SellerID = &testData.SellerAccount.ID
			} else {
				c.BuyerID = testData.SellerAccount.ID // Seller can also be a buyer
				c.SellerID = &testData.BuyerAccount.ID // Buyer can also be a seller
			}
		}

		// Insert all calls
		for _, c := range calls {
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		tests := []struct {
			name      string
			filter    CallFilter
			wantCount int
			verify    func(t *testing.T, results []*call.Call)
		}{
			{
				name:      "list all",
				filter:    CallFilter{},
				wantCount: 7, // 5 new + 2 from test data
			},
			{
				name: "filter by status",
				filter: CallFilter{
					Status: func() *call.Status { s := call.StatusCompleted; return &s }(),
				},
				wantCount: 1,
				verify: func(t *testing.T, results []*call.Call) {
					assert.Equal(t, call.StatusCompleted, results[0].Status)
				},
			},
			{
				name: "filter by buyer ID",
				filter: CallFilter{
					BuyerID: &testData.BuyerAccount.ID,
				},
				wantCount: 5, // 3 new + 2 from test data
			},
			{
				name: "filter by date range",
				filter: CallFilter{
					StartTimeFrom: func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
					StartTimeTo:   func() *time.Time { t := time.Now().Add(time.Hour); return &t }(),
				},
				wantCount: 7,
			},
			{
				name: "pagination",
				filter: CallFilter{
					Limit:  2,
					Offset: 1,
				},
				wantCount: 2,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				results, err := repo.List(ctx, tt.filter)
				require.NoError(t, err)
				assert.Len(t, results, tt.wantCount)

				if tt.verify != nil {
					tt.verify(t, results)
				}
			})
		}
	})
}

func TestCallRepository_Delete(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	repo := NewCallRepository(testDB.DB())
	ctx := testutil.TestContext(t)

	fixtures.WithMinimalData(t, testDB, func(testData *fixtures.TestDataSet) {
		// Create test call
		testCall := fixtures.NewCallBuilder(t).
			WithBuyerID(testData.BuyerAccount.ID).
			Build()
		err := repo.Create(ctx, testCall)
		require.NoError(t, err)

		// Verify it exists
		_, err = repo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)

		// Delete the call
		err = repo.Delete(ctx, testCall.ID)
		assert.NoError(t, err)

		// Verify it's deleted
		_, err = repo.GetByID(ctx, testCall.ID)
		assert.ErrorIs(t, err, sql.ErrNoRows)

		// Delete non-existent call should not error
		err = repo.Delete(ctx, uuid.New())
		assert.NoError(t, err)
	})
}

func TestCallRepository_CountByStatus(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	repo := NewCallRepository(testDB.DB())
	ctx := testutil.TestContext(t)

	fixtures.WithTestData(t, testDB, func(testData *fixtures.TestDataSet) {
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
				WithStatus(status).
				WithBuyerID(testData.BuyerAccount.ID).
				Build()
			err := repo.Create(ctx, c)
			require.NoError(t, err)
		}

		// Test counting (includes the 2 calls from test data)
		counts, err := repo.CountByStatus(ctx)
		require.NoError(t, err)

		expected := map[call.Status]int{
			call.StatusPending:    4, // 2 new + 2 from test data  
			call.StatusInProgress: 3,
			call.StatusCompleted:  1,
			call.StatusFailed:     1,
		}

		assert.Equal(t, expected, counts)
	})
}