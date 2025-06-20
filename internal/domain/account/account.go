package account

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

type Account struct {
	ID     uuid.UUID    `json:"id"`
	Email  values.Email `json:"email"`
	Name   string       `json:"name"`
	Type   AccountType  `json:"type"`
	Status Status       `json:"status"`

	// Business details
	Company     *string            `json:"company,omitempty"`
	PhoneNumber values.PhoneNumber `json:"phone_number"`
	Address     Address            `json:"address"`

	// Financial
	Balance      values.Money `json:"balance"`
	CreditLimit  values.Money `json:"credit_limit"`
	PaymentTerms int          `json:"payment_terms"` // days

	// Compliance
	TCPAConsent     bool     `json:"tcpa_consent"`
	GDPRConsent     bool     `json:"gdpr_consent"`
	ComplianceFlags []string `json:"compliance_flags"`

	// Quality metrics
	QualityMetrics values.QualityMetrics `json:"quality_metrics"`

	// Settings
	Settings AccountSettings `json:"settings"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type AccountType int

const (
	TypeBuyer AccountType = iota
	TypeSeller
	TypeAdmin
)

func (t AccountType) String() string {
	switch t {
	case TypeBuyer:
		return "buyer"
	case TypeSeller:
		return "seller"
	case TypeAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

type Status int

const (
	StatusPending Status = iota
	StatusActive
	StatusSuspended
	StatusBanned
	StatusClosed
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusActive:
		return "active"
	case StatusSuspended:
		return "suspended"
	case StatusBanned:
		return "banned"
	case StatusClosed:
		return "closed"
	default:
		return "unknown"
	}
}

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

type AccountSettings struct {
	Timezone            string       `json:"timezone"`
	CallNotifications   bool         `json:"call_notifications"`
	EmailNotifications  bool         `json:"email_notifications"`
	SMSNotifications    bool         `json:"sms_notifications"`
	AllowedCallingHours []int        `json:"allowed_calling_hours"`
	BlockedAreaCodes    []string     `json:"blocked_area_codes"`
	MaxConcurrentCalls  int          `json:"max_concurrent_calls"`
	AutoBidding         bool         `json:"auto_bidding"`
	MaxBidAmount        values.Money `json:"max_bid_amount"`
}

func NewAccount(emailStr, name string, accountType AccountType) (*Account, error) {
	// Create email value object
	email, err := values.NewEmail(emailStr)
	if err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	// Validate name (moved from validation package to domain)
	if err := validateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}

	// Validate account type
	switch accountType {
	case TypeBuyer, TypeSeller, TypeAdmin:
		// Valid types
	default:
		return nil, ErrInvalidAccountType
	}

	// Create money value objects
	balance, err := values.NewMoneyFromFloat(0.0, values.USD)
	if err != nil {
		return nil, fmt.Errorf("failed to create balance: %w", err)
	}

	creditLimit, err := values.NewMoneyFromFloat(1000.0, values.USD)
	if err != nil {
		return nil, fmt.Errorf("failed to create credit limit: %w", err)
	}

	maxBidAmount, err := values.NewMoneyFromFloat(10.0, values.USD)
	if err != nil {
		return nil, fmt.Errorf("failed to create max bid amount: %w", err)
	}

	now := time.Now()
	return &Account{
		ID:             uuid.New(),
		Email:          email,
		Name:           name,
		Type:           accountType,
		Status:         StatusPending,
		Balance:        balance,
		CreditLimit:    creditLimit,
		PaymentTerms:   30,
		QualityMetrics: values.NewDefaultQualityMetrics(),
		Settings: AccountSettings{
			Timezone:            "UTC",
			CallNotifications:   true,
			EmailNotifications:  true,
			SMSNotifications:    false,
			AllowedCallingHours: []int{9, 10, 11, 12, 13, 14, 15, 16, 17}, // 9 AM - 5 PM
			MaxConcurrentCalls:  10,
			AutoBidding:         false,
			MaxBidAmount:        maxBidAmount,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (a *Account) UpdateBalance(amount values.Money) error {
	// Ensure currencies match
	if a.Balance.Currency() != amount.Currency() {
		return fmt.Errorf("currency mismatch: account balance is %s but amount is %s",
			a.Balance.Currency(), amount.Currency())
	}

	newBalance, err := a.Balance.Add(amount)
	if err != nil {
		return err
	}

	// Check if new balance would exceed credit limit (for negative amounts)
	if newBalance.ToFloat64() < 0 && newBalance.ToFloat64() < -a.CreditLimit.ToFloat64() {
		return ErrInsufficientFunds
	}

	a.Balance = newBalance
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) IsSuspended() bool {
	return a.Status == StatusSuspended || a.Status == StatusBanned
}

func (a *Account) CanMakeCalls() bool {
	return a.Status == StatusActive && a.TCPAConsent
}

// SetPhoneNumber updates the account's phone number with validation
func (a *Account) SetPhoneNumber(phoneStr string) error {
	phoneNumber, err := values.NewPhoneNumber(phoneStr)
	if err != nil {
		return fmt.Errorf("invalid phone number: %w", err)
	}

	a.PhoneNumber = phoneNumber
	a.UpdatedAt = time.Now()
	return nil
}

// UpdateEmail updates the account's email with validation
func (a *Account) UpdateEmail(emailStr string) error {
	email, err := values.NewEmail(emailStr)
	if err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}

	a.Email = email
	a.UpdatedAt = time.Now()
	return nil
}

// HasSufficientBalance checks if account has enough balance for an amount
func (a *Account) HasSufficientBalance(amount values.Money) bool {
	if a.Balance.Currency() != amount.Currency() {
		return false // Currency mismatch
	}

	return a.Balance.ToFloat64() >= amount.ToFloat64()
}

// validateName validates person name within the account domain
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	name = strings.TrimSpace(name)

	// Name validation - allows letters, spaces, hyphens, apostrophes
	nameRegex := regexp.MustCompile(`^[\p{L}\s\-'\.]{2,100}$`)
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("invalid name format")
	}

	if len(name) < 2 {
		return fmt.Errorf("name too short (min 2 characters)")
	}

	if len(name) > 100 {
		return fmt.Errorf("name too long (max 100 characters)")
	}

	return nil
}

var (
	ErrInsufficientFunds  = fmt.Errorf("insufficient funds")
	ErrAccountSuspended   = fmt.Errorf("account suspended")
	ErrInvalidAccountType = fmt.Errorf("invalid account type")
)
