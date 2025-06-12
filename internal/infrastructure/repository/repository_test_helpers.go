package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

// Test data creation helpers

// createTestAccountAndCall creates a buyer account and call for testing bid scenarios
func createTestAccountAndCall(t *testing.T, testDB *testutil.TestDB) (*account.Account, *call.Call) {
	t.Helper()

	// Create buyer account
	buyerAccount := fixtures.NewAccountBuilder(testDB).
		WithType(account.TypeBuyer).
		WithEmail(fixtures.GenerateEmail(t, "bidtest-buyer")).
		WithBalance(1000.00).
		Build(t)

	err := createAccountInDBHelper(t, testDB, buyerAccount)
	require.NoError(t, err)

	// Create call
	testCall := fixtures.NewCallBuilder(t).
		WithBuyerID(buyerAccount.ID).
		Build()

	err = createCallInDB(t, testDB, testCall)
	require.NoError(t, err)

	return buyerAccount, testCall
}

// createAccountInDBHelper inserts an account directly into the database for testing
func createAccountInDBHelper(t *testing.T, testDB *testutil.TestDB, acc *account.Account) error {
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

// createCallInDB inserts a call directly into the database for testing
func createCallInDB(t *testing.T, testDB *testutil.TestDB, c *call.Call) error {
	t.Helper()

	// Handle optional seller ID
	var sellerID sql.NullString
	if c.SellerID != nil {
		sellerID = sql.NullString{String: c.SellerID.String(), Valid: true}
	}

	// Location is now handled in metadata, so we don't need this separate variable

	// Create metadata JSON including call_sid and location
	metadata := make(map[string]interface{})
	if c.CallSID != "" {
		metadata["call_sid"] = c.CallSID
	}
	if c.Location != nil {
		metadata["location"] = c.Location
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	// Convert cost to decimal if present
	var cost sql.NullFloat64
	if c.Cost != nil {
		cost = sql.NullFloat64{Float64: c.Cost.ToFloat64(), Valid: true}
	}

	_, err = testDB.DB().Exec(`
		INSERT INTO calls (
			id, from_number, to_number, status, direction,
			buyer_id, seller_id, started_at, ended_at,
			duration, cost, metadata,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14
		)
	`, c.ID, c.FromNumber.String(), c.ToNumber.String(), c.Status.String(), c.Direction.String(),
		c.BuyerID, sellerID, c.StartTime, c.EndTime,
		c.Duration, cost, metadataJSON,
		c.CreatedAt, c.UpdatedAt)

	return err
}

// Common assertion helpers

// assertBidEquals compares two bids for equality
func assertBidEquals(t *testing.T, expected, actual *bid.Bid) {
	t.Helper()

	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.CallID, actual.CallID)
	assert.Equal(t, expected.BuyerID, actual.BuyerID)
	assert.Equal(t, expected.SellerID, actual.SellerID)
	assert.Equal(t, expected.AuctionID, actual.AuctionID)
	assert.Equal(t, expected.Amount.ToFloat64(), actual.Amount.ToFloat64())
	assert.Equal(t, expected.Status, actual.Status)
	assert.Equal(t, expected.Rank, actual.Rank)
	assert.Equal(t, expected.Quality, actual.Quality)
	// Compare criteria with custom comparison
	assertBidCriteriaEquals(t, expected.Criteria, actual.Criteria)

	// Compare timestamps with tolerance
	assert.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, time.Second)
	assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
	assert.WithinDuration(t, expected.PlacedAt, actual.PlacedAt, time.Second)
	assert.WithinDuration(t, expected.ExpiresAt, actual.ExpiresAt, time.Second)

	// Compare optional timestamps
	if expected.AcceptedAt != nil && actual.AcceptedAt != nil {
		assert.WithinDuration(t, *expected.AcceptedAt, *actual.AcceptedAt, time.Second)
	} else {
		assert.Equal(t, expected.AcceptedAt, actual.AcceptedAt)
	}
}

// assertCallEquals compares two calls for equality
func assertCallEquals(t *testing.T, expected, actual *call.Call) {
	t.Helper()

	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.FromNumber.String(), actual.FromNumber.String())
	assert.Equal(t, expected.ToNumber.String(), actual.ToNumber.String())
	assert.Equal(t, expected.Status, actual.Status)
	assert.Equal(t, expected.Direction, actual.Direction)
	assert.Equal(t, expected.BuyerID, actual.BuyerID)
	assert.Equal(t, expected.SellerID, actual.SellerID)
	assert.Equal(t, expected.CallSID, actual.CallSID)
	assert.Equal(t, expected.Duration, actual.Duration)
	assert.Equal(t, expected.Location, actual.Location)

	// Compare timestamps with tolerance
	assert.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, time.Second)
	assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
	assert.WithinDuration(t, expected.StartTime, actual.StartTime, time.Second)

	// Compare optional timestamps
	if expected.EndTime != nil && actual.EndTime != nil {
		assert.WithinDuration(t, *expected.EndTime, *actual.EndTime, time.Second)
	} else {
		assert.Equal(t, expected.EndTime, actual.EndTime)
	}
}

// assertAccountEquals compares two accounts for equality
func assertAccountEquals(t *testing.T, expected, actual *account.Account) {
	t.Helper()

	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.Email.String(), actual.Email.String())
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Company, actual.Company)
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.Status, actual.Status)
	assert.Equal(t, expected.PhoneNumber.String(), actual.PhoneNumber.String())
	assert.Equal(t, expected.Balance.ToFloat64(), actual.Balance.ToFloat64())
	assert.Equal(t, expected.CreditLimit.ToFloat64(), actual.CreditLimit.ToFloat64())
	assert.Equal(t, expected.PaymentTerms, actual.PaymentTerms)
	assert.Equal(t, expected.TCPAConsent, actual.TCPAConsent)
	assert.Equal(t, expected.GDPRConsent, actual.GDPRConsent)
	assert.Equal(t, expected.ComplianceFlags, actual.ComplianceFlags)
	assert.Equal(t, expected.QualityMetrics, actual.QualityMetrics)
	assert.Equal(t, expected.Settings, actual.Settings)

	// Compare timestamps with tolerance
	assert.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, time.Second)
	assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
}

// assertBidCriteriaEquals compares two bid criteria for equality
func assertBidCriteriaEquals(t *testing.T, expected, actual bid.BidCriteria) {
	t.Helper()

	// Compare geography
	assert.Equal(t, expected.Geography.Countries, actual.Geography.Countries)
	assert.Equal(t, expected.Geography.States, actual.Geography.States)
	assert.Equal(t, expected.Geography.Cities, actual.Geography.Cities)
	assert.Equal(t, expected.Geography.ZipCodes, actual.Geography.ZipCodes)
	assert.Equal(t, expected.Geography.Radius, actual.Geography.Radius)

	// Compare time window
	assert.Equal(t, expected.TimeWindow.StartHour, actual.TimeWindow.StartHour)
	assert.Equal(t, expected.TimeWindow.EndHour, actual.TimeWindow.EndHour)
	assert.Equal(t, expected.TimeWindow.Days, actual.TimeWindow.Days)
	assert.Equal(t, expected.TimeWindow.Timezone, actual.TimeWindow.Timezone)

	// Compare arrays
	assert.Equal(t, expected.CallType, actual.CallType)
	assert.Equal(t, expected.Keywords, actual.Keywords)
	assert.Equal(t, expected.ExcludeList, actual.ExcludeList)

	// Compare Money values by their float representation
	// Use InDelta to handle precision differences in decimal storage
	assert.InDelta(t, expected.MaxBudget.ToFloat64(), actual.MaxBudget.ToFloat64(), 0.001)
	assert.Equal(t, expected.MaxBudget.Currency(), actual.MaxBudget.Currency())
}

// Test context helpers

// testContextWithTimeout creates a context with a reasonable timeout for tests
func testContextWithTimeout(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// compareMetadata compares two metadata maps for equality
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

	// Generate unique emails for test isolation
	timestamp := time.Now().UnixNano()
	buyerEmail := fmt.Sprintf("buyer%d@test.com", timestamp)
	sellerEmail := fmt.Sprintf("seller%d@test.com", timestamp+1)

	// Create accounts directly with minimal setup
	_, err := testDB.DB().Exec(`
		INSERT INTO accounts (
			id, email, name, company, type, status, phone_number,
			balance, credit_limit, payment_terms,
			tcpa_consent, gdpr_consent, compliance_flags,
			quality_score, fraud_score, settings,
			created_at, updated_at
		) VALUES 
		($1, $3, 'Buyer Test', 'Buyer Corp', 'buyer', 'active', '+15551234567',
		 1000.0, 5000.0, 30,
		 true, true, ARRAY[]::text[],
		 5.0, 0.0, '{}'::jsonb,
		 NOW(), NOW()),
		($2, $4, 'Seller Test', 'Seller Corp', 'seller', 'active', '+15559876543',
		 2000.0, 10000.0, 30,
		 true, true, ARRAY[]::text[],
		 5.0, 0.0, '{}'::jsonb,
		 NOW(), NOW())
	`, buyerID, sellerID, buyerEmail, sellerEmail)
	require.NoError(t, err)

	return buyerID, sellerID
}
