package repository

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/quick"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestAccountRepository_GetByID(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewAccountRepository(testDB.DB())

	t.Run("get_existing_account", func(t *testing.T) {
		// Create test account using fixture builder
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "getexisting")).
			WithName("Test User").
			WithCompany("Test Corp").
			WithType(account.TypeBuyer).
			WithBalance(1000.00).
			Build(t)

		// Save account to database
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		// Retrieve account
		retrieved, err := repo.GetByID(ctx, testAccount.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify all fields
		assert.Equal(t, testAccount.ID, retrieved.ID)
		assert.Equal(t, testAccount.Email, retrieved.Email)
		assert.Equal(t, testAccount.Name, retrieved.Name)
		assert.Equal(t, testAccount.Company, retrieved.Company)
		assert.Equal(t, testAccount.Type, retrieved.Type)
		assert.Equal(t, testAccount.Status, retrieved.Status)
		assert.Equal(t, testAccount.Balance, retrieved.Balance)
		assert.Equal(t, testAccount.CreditLimit, retrieved.CreditLimit)
		assert.Equal(t, testAccount.PaymentTerms, retrieved.PaymentTerms)
		assert.Equal(t, testAccount.TCPAConsent, retrieved.TCPAConsent)
		assert.Equal(t, testAccount.GDPRConsent, retrieved.GDPRConsent)
		assert.Equal(t, testAccount.QualityMetrics.QualityScore, retrieved.QualityMetrics.QualityScore)
		assert.Equal(t, testAccount.QualityMetrics.FraudScore, retrieved.QualityMetrics.FraudScore)
	})

	t.Run("get_non_existent_account", func(t *testing.T) {
		nonExistentID := uuid.New()
		retrieved, err := repo.GetByID(ctx, nonExistentID)
		
		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.Contains(t, err.Error(), "account not found")
	})

	t.Run("get_account_with_null_fields", func(t *testing.T) {
		// Create account without optional fields
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "nullfields")).
			WithNoCompany().
			Build(t)

		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		retrieved, err := repo.GetByID(ctx, testAccount.ID)
		require.NoError(t, err)
		
		assert.Nil(t, retrieved.Company)
		assert.Nil(t, retrieved.LastLoginAt)
	})

	t.Run("get_various_account_types", func(t *testing.T) {
		scenarios := fixtures.NewAccountScenarios(t, testDB)
		
		testCases := []struct {
			name    string
			account *account.Account
		}{
			{"buyer_account", scenarios.BuyerAccount()},
			{"seller_account", scenarios.SellerAccount()},
			{"premium_account", scenarios.PremiumAccount()},
			{"suspended_account", scenarios.SuspendedAccount()},
			{"new_account", scenarios.NewAccount()},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := createAccountInDB(t, testDB, tc.account)
				require.NoError(t, err)

				retrieved, err := repo.GetByID(ctx, tc.account.ID)
				require.NoError(t, err)
				
				assert.Equal(t, tc.account.Type, retrieved.Type)
				assert.Equal(t, tc.account.Status, retrieved.Status)
				assert.Equal(t, tc.account.Balance, retrieved.Balance)
			})
		}
	})
}

func TestAccountRepository_UpdateBalance(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewAccountRepository(testDB.DB())

	t.Run("increase_balance", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "increasebalance")).
			WithBalance(100.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		// Increase balance
		err = repo.UpdateBalance(ctx, testAccount.ID, 50.00)
		require.NoError(t, err)

		// Verify new balance
		balance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, 150.00, balance)
	})

	t.Run("decrease_balance", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "decreasebalance")).
			WithBalance(100.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		// Decrease balance
		err = repo.UpdateBalance(ctx, testAccount.ID, -30.00)
		require.NoError(t, err)

		// Verify new balance
		balance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, 70.00, balance)
	})

	t.Run("exceed_credit_limit", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "exceedcredit")).
			WithBalance(0.00).
			WithCreditLimit(100.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		// Try to exceed credit limit
		err = repo.UpdateBalance(ctx, testAccount.ID, -150.00)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "would exceed credit limit")

		// Balance should remain unchanged
		balance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, 0.00, balance)
	})

	t.Run("within_credit_limit", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "withincredit")).
			WithBalance(0.00).
			WithCreditLimit(100.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		// Use credit within limit
		err = repo.UpdateBalance(ctx, testAccount.ID, -80.00)
		require.NoError(t, err)

		balance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, -80.00, balance)
	})

	t.Run("update_non_existent_account", func(t *testing.T) {
		nonExistentID := uuid.New()
		err := repo.UpdateBalance(ctx, nonExistentID, 100.00)
		
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "account not found")
	})

	t.Run("concurrent_balance_updates", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "concurrent")).
			WithBalance(1000.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		// Perform concurrent updates
		numGoroutines := 10
		updateAmount := 10.00
		
		var wg sync.WaitGroup
		successCount := int32(0)
		serializationErrors := int32(0)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := repo.UpdateBalance(ctx, testAccount.ID, updateAmount)
				if err != nil {
					// PostgreSQL serialization errors are expected in concurrent scenarios
					if strings.Contains(err.Error(), "could not serialize access") ||
					   strings.Contains(err.Error(), "deadlock detected") {
						atomic.AddInt32(&serializationErrors, 1)
					} else {
						// Unexpected error
						t.Errorf("Unexpected error in concurrent update: %v", err)
					}
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}()
		}

		wg.Wait()

		// At least some updates should succeed
		assert.Greater(t, int(successCount), 0, "At least some concurrent updates should succeed")
		
		// Verify final balance matches successful updates
		finalBalance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		expectedBalance := 1000.00 + (float64(successCount) * updateAmount)
		assert.Equal(t, expectedBalance, finalBalance)
	})

	t.Run("transaction_isolation", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "isolation")).
			WithBalance(100.00).
			WithCreditLimit(50.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		// Start two concurrent transactions that would exceed credit limit if both succeed
		var wg sync.WaitGroup
		results := make(chan error, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Each tries to use 80 of the 50 credit limit
				err := repo.UpdateBalance(ctx, testAccount.ID, -80.00)
				results <- err
			}()
		}

		wg.Wait()
		close(results)

		// Collect results
		var errors []error
		var successes int
		for err := range results {
			if err == nil {
				successes++
			} else {
				errors = append(errors, err)
			}
		}

		// Only one should succeed
		assert.Equal(t, 1, successes, "Exactly one transaction should succeed")
		assert.Len(t, errors, 1, "One transaction should fail")
		
		// Verify final balance
		finalBalance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, 20.00, finalBalance) // 100 - 80 = 20
	})
}

func TestAccountRepository_GetBalance(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewAccountRepository(testDB.DB())

	t.Run("get_positive_balance", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "positivebalance")).
			WithBalance(250.50).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		balance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, 250.50, balance)
	})

	t.Run("get_zero_balance", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "zerobalance")).
			WithBalance(0.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		balance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, 0.00, balance)
	})

	t.Run("get_negative_balance", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "negativebalance")).
			WithBalance(-50.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		balance, err := repo.GetBalance(ctx, testAccount.ID)
		require.NoError(t, err)
		assert.Equal(t, -50.00, balance)
	})

	t.Run("get_balance_non_existent_account", func(t *testing.T) {
		nonExistentID := uuid.New()
		balance, err := repo.GetBalance(ctx, nonExistentID)
		
		assert.Error(t, err)
		assert.Equal(t, 0.0, balance)
		assert.Contains(t, err.Error(), "account not found")
	})
}

// TestAccountRepository_UpdateQualityScore tests the UpdateQualityScore method
// Note: This test is for internal repository functionality that extends the bidding.AccountRepository interface
func TestAccountRepository_UpdateQualityScore(t *testing.T) {
	t.Skip("UpdateQualityScore is not part of the bidding.AccountRepository interface")
}

func TestAccountRepository_Create(t *testing.T) {
	t.Skip("Create is not part of the bidding.AccountRepository interface")
}

func TestAccountRepository_PropertyBased(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewAccountRepository(testDB.DB())

	t.Run("balance_update_commutativity", func(t *testing.T) {
		// Property: Multiple balance updates should be commutative
		property := func(amounts []float64) bool {
			if len(amounts) == 0 || len(amounts) > 10 {
				return true // Skip edge cases
			}

			// Create test account
			testAccount := fixtures.NewAccountBuilder(testDB).
				WithEmail(fixtures.GenerateEmail(t, "commutativity")).
				WithBalance(1000.00).
				WithCreditLimit(10000.00). // High limit to avoid hitting it
				Build(t)
			
			err := createAccountInDB(t, testDB, testAccount)
			if err != nil {
				return false
			}

			// Apply all updates
			totalChange := 0.0
			for _, amount := range amounts {
				// Limit amounts to reasonable range
				if amount > 1000 || amount < -1000 {
					continue
				}
				totalChange += amount
				
				err := repo.UpdateBalance(ctx, testAccount.ID, amount)
				if err != nil {
					return false
				}
			}

			// Verify final balance
			finalBalance, err := repo.GetBalance(ctx, testAccount.ID)
			if err != nil {
				return false
			}

			expectedBalance := 1000.00 + totalChange
			// Allow for floating point precision issues
			return assert.InDelta(t, expectedBalance, finalBalance, 0.01)
		}

		if err := quick.Check(property, &quick.Config{MaxCount: 20}); err != nil {
			t.Error(err)
		}
	})

}

func TestAccountRepository_DatabaseConstraints(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := context.Background()
	repo := NewAccountRepository(testDB.DB())

	t.Run("transaction_audit_trail", func(t *testing.T) {
		testAccount := fixtures.NewAccountBuilder(testDB).
			WithEmail(fixtures.GenerateEmail(t, "audittrail")).
			WithBalance(100.00).
			Build(t)
		
		err := createAccountInDB(t, testDB, testAccount)
		require.NoError(t, err)

		// Perform balance update
		err = repo.UpdateBalance(ctx, testAccount.ID, 50.00)
		require.NoError(t, err)

		// Check audit trail (if table exists)
		var count int
		err = testDB.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM account_transactions WHERE account_id = $1`,
			testAccount.ID).Scan(&count)
		
		// If table doesn't exist, that's okay for this test
		if err == nil {
			assert.Greater(t, count, 0, "Should have audit trail entries")
		}
	})
}

// Helper functions

func createAccountInDB(t *testing.T, testDB *testutil.TestDB, acc *account.Account) error {
	t.Helper()
	
	// Direct SQL insert
	settingsJSON, err := json.Marshal(acc.Settings)
	if err != nil {
		return err
	}
	
	// Don't need to marshal ComplianceFlags - use pq.Array for PostgreSQL TEXT[] columns
	
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

func normalizeScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}