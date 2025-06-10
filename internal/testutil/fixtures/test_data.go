package fixtures

import (
	"encoding/json"
	"testing"
	
	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

// TestDataSet provides a complete set of test data with proper relationships
type TestDataSet struct {
	// Accounts that can be referenced by calls and bids
	BuyerAccount   *account.Account
	SellerAccount  *account.Account
	SellerAccount1 *account.Account // Additional seller for testing
	SellerAccount2 *account.Account // Additional seller for testing
	
	// Calls that reference the accounts above
	InboundCall  *call.Call
	OutboundCall *call.Call
	
	// Bids that reference calls and buyers above
	ActiveBid *bid.Bid
}

// AccountTestData wraps account data for test insertion
type AccountTestData struct {
	*account.Account
}

// CallTestData wraps call data for test insertion
type CallTestData struct {
	*call.Call
}

// BidTestData wraps bid data for test insertion
type BidTestData struct {
	*bid.Bid
}

// CreateCompleteTestSet creates a full test data set with all relationships properly configured
func CreateCompleteTestSet(t *testing.T, db *testutil.TestDB) *TestDataSet {
	t.Helper()
	
	// Create accounts first (no dependencies)
	buyerAccount := NewAccountBuilder(db).
		WithType(account.TypeBuyer).
		WithEmail(GenerateEmail(t, "buyer")).
		WithName("Test Buyer").
		WithCompany("Buyer Corp").
		Build(t)
	
	sellerAccount := NewAccountBuilder(db).
		WithType(account.TypeSeller).
		WithEmail(GenerateEmail(t, "seller")).
		WithName("Test Seller").
		WithCompany("Seller Inc").
		Build(t)
		
	sellerAccount1 := NewAccountBuilder(db).
		WithType(account.TypeSeller).
		WithEmail(GenerateEmail(t, "seller1")).
		WithName("Test Seller 1").
		WithCompany("Seller 1 Inc").
		WithQualityScore(0.90).
		Build(t)
		
	sellerAccount2 := NewAccountBuilder(db).
		WithType(account.TypeSeller).
		WithEmail(GenerateEmail(t, "seller2")).
		WithName("Test Seller 2").
		WithCompany("Seller 2 Inc").
		WithQualityScore(0.95).
		Build(t)
	
	// Insert accounts into database
	insertAccount(t, db, buyerAccount)
	insertAccount(t, db, sellerAccount)
	insertAccount(t, db, sellerAccount1)
	insertAccount(t, db, sellerAccount2)
	
	// Create calls from sellers (marketplace calls awaiting buyer assignment)
	// IMPORTANT: For marketplace calls:
	// - SellerID is set (the seller who generated/owns the call)
	// - BuyerID will be set later when a buyer wins the bid
	inboundCall := NewCallBuilder(t).
		WithDirection(call.DirectionInbound).
		WithSellerID(sellerAccount.ID).
		WithoutBuyer(). // Explicitly indicate no buyer yet
		Build()
	
	outboundCall := NewCallBuilder(t).
		WithDirection(call.DirectionOutbound).
		WithSellerID(sellerAccount.ID).
		WithoutBuyer(). // Explicitly indicate no buyer yet
		Build()
	
	// Insert calls into database
	insertCall(t, db, inboundCall)
	insertCall(t, db, outboundCall)
	
	// Create bids from buyers on seller calls
	// IMPORTANT: Only buyers place bids on seller calls
	activeBid := NewBidBuilder(db).
		WithCallID(inboundCall.ID).
		WithBuyerID(buyerAccount.ID).
		WithSellerID(sellerAccount.ID). // The seller who owns the call
		WithAmount(25.50).
		Build(t)
	
	// Insert bids into database
	insertBid(t, db, activeBid)
	
	return &TestDataSet{
		BuyerAccount:   buyerAccount,
		SellerAccount:  sellerAccount,
		SellerAccount1: sellerAccount1,
		SellerAccount2: sellerAccount2,
		InboundCall:    inboundCall,
		OutboundCall:   outboundCall,
		ActiveBid:      activeBid,
	}
}

// CreateMinimalTestSet creates a minimal test data set (buyer account + call)
func CreateMinimalTestSet(t *testing.T, db *testutil.TestDB) *TestDataSet {
	t.Helper()
	
	// Create buyer account
	buyerAccount := NewAccountBuilder(db).
		WithType(account.TypeBuyer).
		WithEmail(GenerateEmail(t, "buyer")).
		Build(t)
	
	insertAccount(t, db, buyerAccount)
	
	// Create seller's call awaiting buyer
	sellerAccount := NewAccountBuilder(db).
		WithType(account.TypeSeller).
		WithEmail(GenerateEmail(t, "seller")).
		Build(t)
	
	insertAccount(t, db, sellerAccount)
	
	call := NewCallBuilder(t).
		WithSellerID(sellerAccount.ID).
		WithoutBuyer().
		Build()
	
	insertCall(t, db, call)
	
	return &TestDataSet{
		BuyerAccount:  buyerAccount,
		SellerAccount: sellerAccount,
		InboundCall:   call,
	}
}

// Helper functions to insert data

func insertAccount(t *testing.T, db *testutil.TestDB, acc *account.Account) {
	t.Helper()
	
	// Convert settings to JSON
	settingsJSON, err := json.Marshal(acc.Settings)
	if err != nil {
		t.Fatalf("failed to marshal account settings: %v", err)
	}
	
	_, err = db.DB().Exec(`
		INSERT INTO accounts (id, email, name, company, type, status, phone_number, balance, credit_limit, payment_terms, tcpa_consent, gdpr_consent, quality_score, fraud_score, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, acc.ID, acc.Email.String(), acc.Name, acc.Company, acc.Type.String(), acc.Status.String(),
		acc.PhoneNumber.String(), acc.Balance.ToFloat64(), acc.CreditLimit.ToFloat64(), acc.PaymentTerms, acc.TCPAConsent, acc.GDPRConsent,
		acc.QualityMetrics.QualityScore, acc.QualityMetrics.FraudScore, settingsJSON, acc.CreatedAt, acc.UpdatedAt)
	
	if err != nil {
		t.Fatalf("failed to insert test account: %v", err)
	}
}

func insertCall(t *testing.T, db *testutil.TestDB, c *call.Call) {
	t.Helper()
	
	// Handle nil buyer ID for marketplace calls
	var buyerID interface{} = c.BuyerID
	if c.BuyerID == uuid.Nil {
		buyerID = nil
	}
	
	_, err := db.DB().Exec(`
		INSERT INTO calls (id, from_number, to_number, status, type, buyer_id, seller_id, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, c.ID, c.FromNumber, c.ToNumber, c.Status.String(), c.Direction.String(),
		buyerID, c.SellerID, c.StartTime)
	
	if err != nil {
		t.Fatalf("failed to insert test call: %v", err)
	}
}

func insertBid(t *testing.T, db *testutil.TestDB, b *bid.Bid) {
	t.Helper()
	
	// Convert criteria to JSON
	criteriaJSON, err := json.Marshal(b.Criteria)
	if err != nil {
		t.Fatalf("failed to marshal bid criteria: %v", err)
	}
	
	// Convert quality metrics to JSON
	qualityJSON, err := json.Marshal(b.Quality)
	if err != nil {
		t.Fatalf("failed to marshal bid quality metrics: %v", err)
	}
	
	_, err = db.DB().Exec(`
		INSERT INTO bids (id, call_id, buyer_id, seller_id, amount, status, criteria, quality_metrics, placed_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, b.ID, b.CallID, b.BuyerID, b.SellerID, b.Amount.ToFloat64(), b.Status.String(),
		criteriaJSON, qualityJSON, b.PlacedAt, b.ExpiresAt)
	
	if err != nil {
		t.Fatalf("failed to insert test bid: %v", err)
	}
}

// WithTestData runs a test function with a complete test data set
func WithTestData(t *testing.T, db *testutil.TestDB, fn func(*TestDataSet)) {
	t.Helper()
	
	// Clean database
	db.TruncateTables()
	
	// Create test data
	testData := CreateCompleteTestSet(t, db)
	
	// Run test
	fn(testData)
}

// WithMinimalData runs a test function with minimal test data
func WithMinimalData(t *testing.T, db *testutil.TestDB, fn func(*TestDataSet)) {
	t.Helper()
	
	// Clean database
	db.TruncateTables()
	
	// Create minimal test data
	testData := CreateMinimalTestSet(t, db)
	
	// Run test
	fn(testData)
}