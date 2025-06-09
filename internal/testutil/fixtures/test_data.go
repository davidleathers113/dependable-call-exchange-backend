package fixtures

import (
	"encoding/json"
	"testing"
	
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
	buyerAccount := NewAccountBuilder(t).
		WithType(account.TypeBuyer).
		WithEmail(GenerateEmail(t, "buyer")).
		WithName("Test Buyer").
		WithCompany("Buyer Corp").
		Build()
	
	sellerAccount := NewAccountBuilder(t).
		WithType(account.TypeSeller).
		WithEmail(GenerateEmail(t, "seller")).
		WithName("Test Seller").
		WithCompany("Seller Inc").
		Build()
		
	sellerAccount1 := NewAccountBuilder(t).
		WithType(account.TypeSeller).
		WithEmail(GenerateEmail(t, "seller1")).
		WithName("Test Seller 1").
		WithCompany("Seller 1 Inc").
		WithQualityScore(0.90).
		Build()
		
	sellerAccount2 := NewAccountBuilder(t).
		WithType(account.TypeSeller).
		WithEmail(GenerateEmail(t, "seller2")).
		WithName("Test Seller 2").
		WithCompany("Seller 2 Inc").
		WithQualityScore(0.95).
		Build()
	
	// Insert accounts into database
	insertAccount(t, db, buyerAccount)
	insertAccount(t, db, sellerAccount)
	insertAccount(t, db, sellerAccount1)
	insertAccount(t, db, sellerAccount2)
	
	// Create calls that reference the accounts
	inboundCall := NewCallBuilder(t).
		WithDirection(call.DirectionInbound).
		WithBuyerID(buyerAccount.ID).
		WithSellerID(sellerAccount.ID).
		Build()
	
	outboundCall := NewCallBuilder(t).
		WithDirection(call.DirectionOutbound).
		WithBuyerID(buyerAccount.ID).
		WithSellerID(sellerAccount.ID).
		Build()
	
	// Insert calls into database
	insertCall(t, db, inboundCall)
	insertCall(t, db, outboundCall)
	
	// Create bids that reference calls and buyers
	activeBid := NewBidBuilder(db).
		WithCallID(inboundCall.ID).
		WithBuyerID(buyerAccount.ID).
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
	buyerAccount := NewAccountBuilder(t).
		WithType(account.TypeBuyer).
		WithEmail(GenerateEmail(t, "buyer")).
		Build()
	
	insertAccount(t, db, buyerAccount)
	
	// Create call with buyer
	call := NewCallBuilder(t).
		WithBuyerID(buyerAccount.ID).
		Build()
	
	insertCall(t, db, call)
	
	return &TestDataSet{
		BuyerAccount: buyerAccount,
		InboundCall:  call,
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
		INSERT INTO accounts (id, email, company, type, status, balance, credit_limit, quality_score, settings)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, acc.ID, acc.Email, acc.Company, acc.Type.String(), acc.Status.String(),
		acc.Balance, acc.CreditLimit, acc.QualityScore, settingsJSON)
	
	if err != nil {
		t.Fatalf("failed to insert test account: %v", err)
	}
}

func insertCall(t *testing.T, db *testutil.TestDB, c *call.Call) {
	t.Helper()
	
	_, err := db.DB().Exec(`
		INSERT INTO calls (id, from_number, to_number, status, type, buyer_id, seller_id, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, c.ID, c.FromNumber, c.ToNumber, c.Status.String(), c.Direction.String(),
		c.BuyerID, c.SellerID, c.StartTime)
	
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
	
	_, err = db.DB().Exec(`
		INSERT INTO bids (id, call_id, buyer_id, amount, status, criteria, placed_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, b.ID, b.CallID, b.BuyerID, b.Amount, b.Status.String(),
		criteriaJSON, b.PlacedAt, b.ExpiresAt)
	
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