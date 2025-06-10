package fixtures

import (
	"fmt"
	"testing"
	"time"
	
	"github.com/google/uuid"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

// AccountBuilder builds test Account entities
type AccountBuilder struct {
	t               *testing.T
	id              uuid.UUID
	email           values.Email
	name            string
	company         *string
	accountType     account.AccountType
	status          account.Status
	phoneNumber     values.PhoneNumber
	address         account.Address
	balance         values.Money
	creditLimit     values.Money
	paymentTerms    int
	tcpaConsent     bool
	gdprConsent     bool
	qualityMetrics  values.QualityMetrics
	settings        account.AccountSettings
}

// NewAccountBuilder creates a new AccountBuilder with defaults
func NewAccountBuilder(testDB *testutil.TestDB) *AccountBuilder {
	id := uuid.New()
	
	company := "Test Company Inc"
	
	// Create value objects with unique email
	timestamp := time.Now().UnixNano()
	email := values.MustNewEmail(fmt.Sprintf("test%d@example.com", timestamp))
	phoneNumber := values.MustNewPhoneNumber("+15551234567")
	balance := values.MustNewMoneyFromFloat(1000.00, values.USD)
	creditLimit := values.MustNewMoneyFromFloat(5000.00, values.USD)
	qualityMetrics := values.NewDefaultQualityMetrics()
	
	return &AccountBuilder{
		t:               nil, // Will be set when Build is called
		id:              id,
		email:           email,
		name:            "Test User",
		company:         &company,
		accountType:     account.TypeBuyer,
		status:          account.StatusActive,
		phoneNumber:     phoneNumber,
		address: account.Address{
			Street:  "123 Main St",
			City:    "Los Angeles",
			State:   "CA",
			ZipCode: "90001",
			Country: "US",
		},
		balance:         balance,
		creditLimit:     creditLimit,
		paymentTerms:    30,
		tcpaConsent:     true,
		gdprConsent:     true,
		qualityMetrics:  qualityMetrics,
		settings: account.AccountSettings{
			Timezone:            "America/Los_Angeles",
			CallNotifications:   true,
			EmailNotifications:  true,
			SMSNotifications:    false,
			AllowedCallingHours: []int{9, 10, 11, 12, 13, 14, 15, 16, 17},
			BlockedAreaCodes:    []string{},
			MaxConcurrentCalls:  100,
			AutoBidding:         true,
			MaxBidAmount:        values.MustNewMoneyFromFloat(25.00, values.USD),
		},
	}
}

// WithID sets the account ID
func (b *AccountBuilder) WithID(id uuid.UUID) *AccountBuilder {
	b.id = id
	return b
}

// WithEmail sets the email
func (b *AccountBuilder) WithEmail(email string) *AccountBuilder {
	b.email = values.MustNewEmail(email)
	return b
}

// WithName sets the name
func (b *AccountBuilder) WithName(name string) *AccountBuilder {
	b.name = name
	return b
}

// WithCompany sets the company name
func (b *AccountBuilder) WithCompany(company string) *AccountBuilder {
	b.company = &company
	return b
}

// WithNoCompany removes the company
func (b *AccountBuilder) WithNoCompany() *AccountBuilder {
	b.company = nil
	return b
}

// WithType sets the account type
func (b *AccountBuilder) WithType(accountType account.AccountType) *AccountBuilder {
	b.accountType = accountType
	return b
}

// WithPhoneNumber sets the phone number
func (b *AccountBuilder) WithPhoneNumber(phone string) *AccountBuilder {
	b.phoneNumber = values.MustNewPhoneNumber(phone)
	return b
}

// WithAddress sets the address
func (b *AccountBuilder) WithAddress(address account.Address) *AccountBuilder {
	b.address = address
	return b
}

// WithStatus sets the account status
func (b *AccountBuilder) WithStatus(status account.Status) *AccountBuilder {
	b.status = status
	return b
}

// WithBalance sets the account balance
func (b *AccountBuilder) WithBalance(balance float64) *AccountBuilder {
	b.balance = values.MustNewMoneyFromFloat(balance, values.USD)
	return b
}

// WithCreditLimit sets the credit limit
func (b *AccountBuilder) WithCreditLimit(limit float64) *AccountBuilder {
	b.creditLimit = values.MustNewMoneyFromFloat(limit, values.USD)
	return b
}

// WithPaymentTerms sets the payment terms
func (b *AccountBuilder) WithPaymentTerms(days int) *AccountBuilder {
	b.paymentTerms = days
	return b
}

// WithTCPAConsent sets the TCPA consent
func (b *AccountBuilder) WithTCPAConsent(consent bool) *AccountBuilder {
	b.tcpaConsent = consent
	return b
}

// WithGDPRConsent sets the GDPR consent
func (b *AccountBuilder) WithGDPRConsent(consent bool) *AccountBuilder {
	b.gdprConsent = consent
	return b
}

// WithQualityScore sets the quality score
func (b *AccountBuilder) WithQualityScore(score float64) *AccountBuilder {
	// Create new quality metrics with updated quality score
	b.qualityMetrics = values.MustNewQualityMetrics(
		score,                               // qualityScore (updated)
		b.qualityMetrics.FraudScore,        // fraudScore (preserve)
		b.qualityMetrics.HistoricalRating,  // historicalRating (preserve)
		b.qualityMetrics.ConversionRate,    // conversionRate (preserve)
		b.qualityMetrics.AverageCallTime,   // averageCallTime (preserve)
		b.qualityMetrics.TrustScore,        // trustScore (preserve)
		b.qualityMetrics.ReliabilityScore,  // reliabilityScore (preserve)
	)
	return b
}

// WithFraudScore sets the fraud score
func (b *AccountBuilder) WithFraudScore(score float64) *AccountBuilder {
	// Create new quality metrics with updated fraud score
	b.qualityMetrics = values.MustNewQualityMetrics(
		b.qualityMetrics.QualityScore,      // qualityScore (preserve)
		score,                              // fraudScore (updated)
		b.qualityMetrics.HistoricalRating, // historicalRating (preserve)
		b.qualityMetrics.ConversionRate,   // conversionRate (preserve)
		b.qualityMetrics.AverageCallTime,  // averageCallTime (preserve)
		b.qualityMetrics.TrustScore,       // trustScore (preserve)
		b.qualityMetrics.ReliabilityScore, // reliabilityScore (preserve)
	)
	return b
}

// WithSettings sets custom settings
func (b *AccountBuilder) WithSettings(settings account.AccountSettings) *AccountBuilder {
	b.settings = settings
	return b
}

// Build creates the Account entity
func (b *AccountBuilder) Build(t *testing.T) *account.Account {
	t.Helper()
	b.t = t // Set the testing.T
	
	now := time.Now().UTC()
	acc := &account.Account{
		ID:              b.id,
		Email:           b.email,
		Name:            b.name,
		Type:            b.accountType,
		Status:          b.status,
		Company:         b.company,
		PhoneNumber:     b.phoneNumber,
		Address:         b.address,
		Balance:         b.balance,
		CreditLimit:     b.creditLimit,
		PaymentTerms:    b.paymentTerms,
		TCPAConsent:     b.tcpaConsent,
		GDPRConsent:     b.gdprConsent,
		ComplianceFlags: []string{},
		QualityMetrics:  b.qualityMetrics,
		Settings:        b.settings,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	
	// Set last login for active accounts
	if b.status == account.StatusActive {
		lastLogin := now.Add(-1 * time.Hour)
		acc.LastLoginAt = &lastLogin
	}
	
	return acc
}

// AccountScenarios provides common account test scenarios
type AccountScenarios struct {
	t      *testing.T
	testDB *testutil.TestDB
}

// NewAccountScenarios creates a new AccountScenarios helper
func NewAccountScenarios(t *testing.T, testDB *testutil.TestDB) *AccountScenarios {
	t.Helper()
	return &AccountScenarios{t: t, testDB: testDB}
}

// BuyerAccount creates a typical buyer account
func (as *AccountScenarios) BuyerAccount() *account.Account {
	return NewAccountBuilder(as.testDB).
		WithType(account.TypeBuyer).
		WithEmail(GenerateEmail(as.t, "buyer")).
		WithName("John Smith").
		WithCompany("Premium Leads LLC").
		WithBalance(2500.00).
		WithCreditLimit(10000.00).
		WithSettings(account.AccountSettings{
			Timezone:            "America/New_York",
			CallNotifications:   true,
			EmailNotifications:  true,
			SMSNotifications:    true,
			AllowedCallingHours: []int{8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			BlockedAreaCodes:    []string{"900", "976"},
			MaxConcurrentCalls:  500,
			AutoBidding:         true,
			MaxBidAmount:        values.MustNewMoneyFromFloat(50.00, values.USD),
		}).
		Build(as.t)
}

// SellerAccount creates a typical seller account
func (as *AccountScenarios) SellerAccount() *account.Account {
	return NewAccountBuilder(as.testDB).
		WithType(account.TypeSeller).
		WithEmail(GenerateEmail(as.t, "seller")).
		WithName("Sarah Johnson").
		WithCompany("Call Center Pro").
		WithBalance(5000.00).
		WithQualityScore(8.5).
		WithFraudScore(0.02).
		WithSettings(account.AccountSettings{
			Timezone:            "America/Chicago",
			CallNotifications:   true,
			EmailNotifications:  true,
			SMSNotifications:    false,
			AllowedCallingHours: []int{7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21},
			BlockedAreaCodes:    []string{},
			MaxConcurrentCalls:  1000,
			AutoBidding:         false,
			MaxBidAmount:        values.MustNewMoneyFromFloat(0.00, values.USD),
		}).
		Build(as.t)
}

// SuspendedAccount creates a suspended account
func (as *AccountScenarios) SuspendedAccount() *account.Account {
	return NewAccountBuilder(as.testDB).
		WithEmail(GenerateEmail(as.t, "suspended")).
		WithStatus(account.StatusSuspended).
		WithBalance(-500.00). // Negative balance
		WithQualityScore(2.0). // Low quality
		WithFraudScore(0.75). // High fraud score
		WithTCPAConsent(false).
		Build(as.t)
}

// PremiumAccount creates a high-tier account
func (as *AccountScenarios) PremiumAccount() *account.Account {
	return NewAccountBuilder(as.testDB).
		WithEmail(GenerateEmail(as.t, "premium")).
		WithName("Enterprise Admin").
		WithCompany("Enterprise Solutions Inc").
		WithBalance(50000.00).
		WithCreditLimit(100000.00).
		WithPaymentTerms(60). // Extended payment terms
		WithQualityScore(9.5).
		WithFraudScore(0.001).
		WithSettings(account.AccountSettings{
			Timezone:            "America/New_York",
			CallNotifications:   true,
			EmailNotifications:  true,
			SMSNotifications:    true,
			AllowedCallingHours: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}, // 24/7
			BlockedAreaCodes:    []string{},
			MaxConcurrentCalls:  5000,
			AutoBidding:         true,
			MaxBidAmount:        values.MustNewMoneyFromFloat(100.00, values.USD),
		}).
		Build(as.t)
}

// NewAccount creates a newly registered account
func (as *AccountScenarios) NewAccount() *account.Account {
	return NewAccountBuilder(as.testDB).
		WithEmail(GenerateEmail(as.t, "newuser")).
		WithStatus(account.StatusPending).
		WithBalance(0.00).
		WithCreditLimit(100.00). // Low initial limit
		WithPaymentTerms(7). // Shorter payment terms for new accounts
		WithQualityScore(5.0). // Neutral starting score
		WithFraudScore(0.0).
		WithSettings(account.AccountSettings{
			Timezone:            "UTC",
			CallNotifications:   true,
			EmailNotifications:  true,
			SMSNotifications:    false,
			AllowedCallingHours: []int{9, 10, 11, 12, 13, 14, 15, 16, 17},
			BlockedAreaCodes:    []string{},
			MaxConcurrentCalls:  10,
			AutoBidding:         false, // Manual mode for new users
			MaxBidAmount:        values.MustNewMoneyFromFloat(10.00, values.USD),
		}).
		Build(as.t)
}

// AccountSet creates a set of diverse accounts
func (as *AccountScenarios) AccountSet(buyers, sellers int) []*account.Account {
	accounts := make([]*account.Account, 0, buyers+sellers)
	
	// Create buyers
	for i := 0; i < buyers; i++ {
		account := NewAccountBuilder(as.testDB).
			WithType(account.TypeBuyer).
			WithEmail(GenerateEmail(as.t, "buyer")).
			WithCompany(GenerateCompanyName(as.t, "Buyer")).
			WithBalance(float64(1000 + i*500)).
			Build(as.t)
		accounts = append(accounts, account)
	}
	
	// Create sellers
	for i := 0; i < sellers; i++ {
		account := NewAccountBuilder(as.testDB).
			WithType(account.TypeSeller).
			WithEmail(GenerateEmail(as.t, "seller")).
			WithCompany(GenerateCompanyName(as.t, "Seller")).
			WithBalance(float64(2000 + i*1000)).
			Build(as.t)
		accounts = append(accounts, account)
	}
	
	return accounts
}

// GenerateEmail generates a unique test email
func GenerateEmail(t *testing.T, prefix string) string {
	t.Helper()
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s%d@test.example.com", prefix, timestamp)
}

// GenerateCompanyName generates a test company name
func GenerateCompanyName(t *testing.T, prefix string) string {
	t.Helper()
	suffixes := []string{"LLC", "Inc", "Corp", "Solutions", "Services", "Group"}
	idx := time.Now().UnixNano() % int64(len(suffixes))
	return prefix + " Test " + suffixes[idx]
}

// MultipleSellerAccounts creates multiple seller accounts for testing
func (as *AccountScenarios) MultipleSellerAccounts(count int) []*account.Account {
	sellers := make([]*account.Account, count)
	for i := 0; i < count; i++ {
		seller := NewAccountBuilder(as.testDB).
			WithType(account.TypeSeller).
			WithEmail(fmt.Sprintf("seller%d@test.com", i)).
			WithCompany(fmt.Sprintf("Seller %d Inc", i)).
			WithBalance(500.00 + float64(i*50)).
			WithQualityScore(0.80 + float64(i)*0.01).
			Build(as.t)
		sellers[i] = seller
	}
	return sellers
}

// SuspendedSellerAccount creates a suspended seller account
func (as *AccountScenarios) SuspendedSellerAccount() *account.Account {
	return NewAccountBuilder(as.testDB).
		WithType(account.TypeSeller).
		WithStatus(account.StatusSuspended).
		WithEmail("suspended-seller@test.com").
		WithCompany("Suspended Seller Inc").
		WithBalance(-100.00).
		WithQualityScore(1.0).
		WithFraudScore(0.95).
		Build(as.t)
}