package account_test

import (
	"testing"
	"time"
	
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestNewAccount(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		userName    string
		accountType account.AccountType
		validate    func(t *testing.T, a *account.Account)
	}{
		{
			name:        "creates buyer account with defaults",
			email:       "buyer@example.com",
			userName:    "John Buyer",
			accountType: account.TypeBuyer,
			validate: func(t *testing.T, a *account.Account) {
				assert.NotEqual(t, uuid.Nil, a.ID)
				assert.Equal(t, "buyer@example.com", a.Email)
				assert.Equal(t, "John Buyer", a.Name)
				assert.Equal(t, account.TypeBuyer, a.Type)
				assert.Equal(t, account.StatusPending, a.Status)
				assert.Equal(t, 0.0, a.Balance)
				assert.Equal(t, 1000.0, a.CreditLimit)
				assert.Equal(t, 30, a.PaymentTerms)
				assert.Equal(t, 5.0, a.QualityScore)
				assert.Equal(t, 0.0, a.FraudScore)
				assert.NotZero(t, a.CreatedAt)
				assert.NotZero(t, a.UpdatedAt)
				assert.Nil(t, a.LastLoginAt)
				assert.Nil(t, a.Company)
			},
		},
		{
			name:        "creates seller account",
			email:       "seller@example.com",
			userName:    "Jane Seller",
			accountType: account.TypeSeller,
			validate: func(t *testing.T, a *account.Account) {
				assert.Equal(t, account.TypeSeller, a.Type)
				assert.Equal(t, account.StatusPending, a.Status)
				assert.Equal(t, "seller@example.com", a.Email)
			},
		},
		{
			name:        "creates admin account",
			email:       "admin@example.com",
			userName:    "Admin User",
			accountType: account.TypeAdmin,
			validate: func(t *testing.T, a *account.Account) {
				assert.Equal(t, account.TypeAdmin, a.Type)
			},
		},
		{
			name:        "default settings are correct",
			email:       "test@example.com",
			userName:    "Test User",
			accountType: account.TypeBuyer,
			validate: func(t *testing.T, a *account.Account) {
				assert.Equal(t, "UTC", a.Settings.Timezone)
				assert.True(t, a.Settings.CallNotifications)
				assert.True(t, a.Settings.EmailNotifications)
				assert.False(t, a.Settings.SMSNotifications)
				assert.Equal(t, []int{9, 10, 11, 12, 13, 14, 15, 16, 17}, a.Settings.AllowedCallingHours)
				assert.Equal(t, 10, a.Settings.MaxConcurrentCalls)
				assert.False(t, a.Settings.AutoBidding)
				assert.Equal(t, 10.0, a.Settings.MaxBidAmount)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := account.NewAccount(tt.email, tt.userName, tt.accountType)
			require.NotNil(t, a)
			tt.validate(t, a)
		})
	}
}

func TestAccount_UpdateBalance(t *testing.T) {
	tests := []struct {
		name           string
		setup          func() *account.Account
		amount         float64
		expectedError  error
		expectedBalance float64
	}{
		{
			name: "adds positive amount",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithBalance(100.00).
					Build()
			},
			amount:          50.00,
			expectedError:   nil,
			expectedBalance: 150.00,
		},
		{
			name: "subtracts negative amount",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithBalance(100.00).
					Build()
			},
			amount:          -30.00,
			expectedError:   nil,
			expectedBalance: 70.00,
		},
		{
			name: "allows negative balance within credit limit",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithBalance(100.00).
					WithCreditLimit(500.00).
					Build()
			},
			amount:          -300.00,
			expectedError:   nil,
			expectedBalance: -200.00,
		},
		{
			name: "rejects amount exceeding credit limit",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithBalance(100.00).
					WithCreditLimit(500.00).
					Build()
			},
			amount:          -700.00,
			expectedError:   account.ErrInsufficientFunds,
			expectedBalance: 100.00, // Balance unchanged
		},
		{
			name: "handles zero amount",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithBalance(100.00).
					Build()
			},
			amount:          0.00,
			expectedError:   nil,
			expectedBalance: 100.00,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.setup()
			oldUpdatedAt := a.UpdatedAt
			
			time.Sleep(10 * time.Millisecond)
			err := a.UpdateBalance(tt.amount)
			
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.True(t, a.UpdatedAt.After(oldUpdatedAt))
			}
			assert.Equal(t, tt.expectedBalance, a.Balance)
		})
	}
}

func TestAccount_IsSuspended(t *testing.T) {
	tests := []struct {
		name     string
		status   account.Status
		expected bool
	}{
		{"active account", account.StatusActive, false},
		{"pending account", account.StatusPending, false},
		{"suspended account", account.StatusSuspended, true},
		{"banned account", account.StatusBanned, true},
		{"closed account", account.StatusClosed, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := fixtures.NewAccountBuilder(t).
				WithStatus(tt.status).
				Build()
			
			assert.Equal(t, tt.expected, a.IsSuspended())
		})
	}
}

func TestAccount_CanMakeCalls(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *account.Account
		expected    bool
	}{
		{
			name: "active account with TCPA consent",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithStatus(account.StatusActive).
					WithTCPAConsent(true).
					Build()
			},
			expected: true,
		},
		{
			name: "active account without TCPA consent",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithStatus(account.StatusActive).
					WithTCPAConsent(false).
					Build()
			},
			expected: false,
		},
		{
			name: "suspended account with TCPA consent",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithStatus(account.StatusSuspended).
					WithTCPAConsent(true).
					Build()
			},
			expected: false,
		},
		{
			name: "pending account with TCPA consent",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithStatus(account.StatusPending).
					WithTCPAConsent(true).
					Build()
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.setup()
			assert.Equal(t, tt.expected, a.CanMakeCalls())
		})
	}
}

func TestAccountType_String(t *testing.T) {
	tests := []struct {
		accountType account.AccountType
		expected    string
	}{
		{account.TypeBuyer, "buyer"},
		{account.TypeSeller, "seller"},
		{account.TypeAdmin, "admin"},
		{account.AccountType(999), "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.accountType.String())
		})
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   account.Status
		expected string
	}{
		{account.StatusPending, "pending"},
		{account.StatusActive, "active"},
		{account.StatusSuspended, "suspended"},
		{account.StatusBanned, "banned"},
		{account.StatusClosed, "closed"},
		{account.Status(999), "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestAccount_Scenarios(t *testing.T) {
	scenarios := fixtures.NewAccountScenarios(t)
	
	t.Run("buyer account", func(t *testing.T) {
		a := scenarios.BuyerAccount()
		assert.Equal(t, account.TypeBuyer, a.Type)
		assert.NotNil(t, a.Company)
		assert.True(t, a.Settings.AutoBidding)
		assert.Greater(t, a.Balance, 0.0)
		assert.Greater(t, a.CreditLimit, a.Balance)
	})
	
	t.Run("seller account", func(t *testing.T) {
		a := scenarios.SellerAccount()
		assert.Equal(t, account.TypeSeller, a.Type)
		assert.False(t, a.Settings.AutoBidding)
		assert.Greater(t, a.Settings.MaxConcurrentCalls, 100)
		assert.Greater(t, a.QualityScore, 5.0)
	})
	
	t.Run("suspended account", func(t *testing.T) {
		a := scenarios.SuspendedAccount()
		assert.Equal(t, account.StatusSuspended, a.Status)
		assert.Less(t, a.Balance, 0.0)
		assert.Less(t, a.QualityScore, 5.0)
		assert.Greater(t, a.FraudScore, 0.5)
		assert.False(t, a.TCPAConsent)
		assert.True(t, a.IsSuspended())
		assert.False(t, a.CanMakeCalls())
	})
	
	t.Run("premium account", func(t *testing.T) {
		a := scenarios.PremiumAccount()
		assert.Greater(t, a.Balance, 10000.0)
		assert.Greater(t, a.CreditLimit, 50000.0)
		assert.Equal(t, 60, a.PaymentTerms)
		assert.Greater(t, a.QualityScore, 9.0)
		assert.Less(t, a.FraudScore, 0.01)
		assert.Equal(t, 24, len(a.Settings.AllowedCallingHours)) // 24/7
		assert.Equal(t, 5000, a.Settings.MaxConcurrentCalls)
	})
	
	t.Run("new account", func(t *testing.T) {
		a := scenarios.NewAccount()
		assert.Equal(t, account.StatusPending, a.Status)
		assert.Equal(t, 0.0, a.Balance)
		assert.Equal(t, 100.0, a.CreditLimit)
		assert.Equal(t, 7, a.PaymentTerms)
		assert.Equal(t, 5.0, a.QualityScore)
		assert.False(t, a.Settings.AutoBidding)
		assert.Nil(t, a.LastLoginAt)
	})
}

func TestAccount_Address(t *testing.T) {
	t.Run("complete address", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithAddress(account.Address{
				Street:  "123 Main St",
				City:    "Los Angeles",
				State:   "CA",
				ZipCode: "90001",
				Country: "US",
			}).
			Build()
		
		assert.Equal(t, "123 Main St", a.Address.Street)
		assert.Equal(t, "Los Angeles", a.Address.City)
		assert.Equal(t, "CA", a.Address.State)
		assert.Equal(t, "90001", a.Address.ZipCode)
		assert.Equal(t, "US", a.Address.Country)
	})
}

func TestAccount_ComplianceFlags(t *testing.T) {
	t.Run("TCPA and GDPR consent", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithTCPAConsent(true).
			WithGDPRConsent(true).
			Build()
		
		assert.True(t, a.TCPAConsent)
		assert.True(t, a.GDPRConsent)
		assert.Empty(t, a.ComplianceFlags)
	})
	
	t.Run("no consent given", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithTCPAConsent(false).
			WithGDPRConsent(false).
			Build()
		
		assert.False(t, a.TCPAConsent)
		assert.False(t, a.GDPRConsent)
		assert.False(t, a.CanMakeCalls())
	})
}

func TestAccount_Settings(t *testing.T) {
	t.Run("business hours only", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithSettings(account.AccountSettings{
				Timezone:            "America/New_York",
				AllowedCallingHours: []int{9, 10, 11, 12, 13, 14, 15, 16, 17},
				MaxConcurrentCalls:  50,
			}).
			Build()
		
		assert.Equal(t, "America/New_York", a.Settings.Timezone)
		assert.Equal(t, 9, len(a.Settings.AllowedCallingHours))
		assert.Equal(t, 50, a.Settings.MaxConcurrentCalls)
	})
	
	t.Run("auto bidding settings", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithSettings(account.AccountSettings{
				AutoBidding:  true,
				MaxBidAmount: 75.00,
			}).
			Build()
		
		assert.True(t, a.Settings.AutoBidding)
		assert.Equal(t, 75.00, a.Settings.MaxBidAmount)
	})
	
	t.Run("blocked area codes", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithSettings(account.AccountSettings{
				BlockedAreaCodes: []string{"900", "976", "555"},
			}).
			Build()
		
		assert.Len(t, a.Settings.BlockedAreaCodes, 3)
		assert.Contains(t, a.Settings.BlockedAreaCodes, "900")
		assert.Contains(t, a.Settings.BlockedAreaCodes, "976")
		assert.Contains(t, a.Settings.BlockedAreaCodes, "555")
	})
}

func TestAccount_EdgeCases(t *testing.T) {
	t.Run("very large balance", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithBalance(999999999.99).
			Build()
		
		assert.Equal(t, 999999999.99, a.Balance)
	})
	
	t.Run("deeply negative balance", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithBalance(-10000.00).
			WithCreditLimit(15000.00).
			Build()
		
		assert.Equal(t, -10000.00, a.Balance)
		
		// Can still spend within credit limit
		err := a.UpdateBalance(-4999.00)
		assert.NoError(t, err)
		assert.Equal(t, -14999.00, a.Balance)
		
		// But not beyond credit limit
		err = a.UpdateBalance(-2.00)
		assert.ErrorIs(t, err, account.ErrInsufficientFunds)
	})
	
	t.Run("concurrent balance updates", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).
			WithBalance(1000.00).
			Build()
		
		done := make(chan bool, 2)
		
		go func() {
			a.UpdateBalance(100.00)
			done <- true
		}()
		
		go func() {
			a.UpdateBalance(-50.00)
			done <- true
		}()
		
		<-done
		<-done
		
		// One of these should be true
		validBalances := []float64{1050.00, 1100.00, 950.00}
		assert.Contains(t, validBalances, a.Balance)
	})
}

func TestAccount_Performance(t *testing.T) {
	t.Run("account creation performance", func(t *testing.T) {
		start := time.Now()
		count := 10000
		
		for i := 0; i < count; i++ {
			_ = account.NewAccount("test@example.com", "Test User", account.TypeBuyer)
		}
		
		elapsed := time.Since(start)
		perAccount := elapsed / time.Duration(count)
		
		assert.Less(t, perAccount, 10*time.Microsecond,
			"Account creation took %v per account, expected < 10µs", perAccount)
	})
	
	t.Run("balance update performance", func(t *testing.T) {
		a := fixtures.NewAccountBuilder(t).Build()
		
		start := time.Now()
		count := 10000
		
		for i := 0; i < count; i++ {
			_ = a.UpdateBalance(1.00)
		}
		
		elapsed := time.Since(start)
		perUpdate := elapsed / time.Duration(count)
		
		assert.Less(t, perUpdate, 1*time.Microsecond,
			"Balance update took %v per update, expected < 1µs", perUpdate)
	})
}

func TestAccount_TableDriven(t *testing.T) {
	type testCase struct {
		name     string
		setup    func() *account.Account
		action   func(*account.Account) error
		validate func(*testing.T, *account.Account, error)
	}
	
	tests := []testCase{
		{
			name: "successful payment",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithBalance(100.00).
					Build()
			},
			action: func(a *account.Account) error {
				return a.UpdateBalance(50.00)
			},
			validate: func(t *testing.T, a *account.Account, err error) {
				assert.NoError(t, err)
				assert.Equal(t, 150.00, a.Balance)
			},
		},
		{
			name: "overdraft protection",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithBalance(100.00).
					WithCreditLimit(200.00).
					Build()
			},
			action: func(a *account.Account) error {
				return a.UpdateBalance(-350.00)
			},
			validate: func(t *testing.T, a *account.Account, err error) {
				assert.ErrorIs(t, err, account.ErrInsufficientFunds)
				assert.Equal(t, 100.00, a.Balance) // Unchanged
			},
		},
		{
			name: "status change affects calling",
			setup: func() *account.Account {
				return fixtures.NewAccountBuilder(t).
					WithStatus(account.StatusActive).
					WithTCPAConsent(true).
					Build()
			},
			action: func(a *account.Account) error {
				a.Status = account.StatusSuspended
				return nil
			},
			validate: func(t *testing.T, a *account.Account, err error) {
				assert.NoError(t, err)
				assert.False(t, a.CanMakeCalls())
				assert.True(t, a.IsSuspended())
			},
		},
	}
	
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := tc.setup()
			err := tc.action(a)
			tc.validate(t, a, err)
		})
	}
}