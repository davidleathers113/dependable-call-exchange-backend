package account

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/validation"
)

type Account struct {
	ID           uuid.UUID   `json:"id"`
	Email        string      `json:"email"`
	Name         string      `json:"name"`
	Type         AccountType `json:"type"`
	Status       Status      `json:"status"`
	
	// Business details
	Company      *string `json:"company,omitempty"`
	PhoneNumber  string  `json:"phone_number"`
	Address      Address `json:"address"`
	
	// Financial
	Balance      float64 `json:"balance"`
	CreditLimit  float64 `json:"credit_limit"`
	PaymentTerms int     `json:"payment_terms"` // days
	
	// Compliance
	TCPAConsent     bool      `json:"tcpa_consent"`
	GDPRConsent     bool      `json:"gdpr_consent"`
	ComplianceFlags []string  `json:"compliance_flags"`
	
	// Quality metrics
	QualityScore    float64 `json:"quality_score"`
	FraudScore      float64 `json:"fraud_score"`
	
	// Settings
	Settings     AccountSettings `json:"settings"`
	
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
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
	Street   string `json:"street"`
	City     string `json:"city"`
	State    string `json:"state"`
	ZipCode  string `json:"zip_code"`
	Country  string `json:"country"`
}

type AccountSettings struct {
	Timezone               string   `json:"timezone"`
	CallNotifications      bool     `json:"call_notifications"`
	EmailNotifications     bool     `json:"email_notifications"`
	SMSNotifications       bool     `json:"sms_notifications"`
	AllowedCallingHours    []int    `json:"allowed_calling_hours"`
	BlockedAreaCodes       []string `json:"blocked_area_codes"`
	MaxConcurrentCalls     int      `json:"max_concurrent_calls"`
	AutoBidding            bool     `json:"auto_bidding"`
	MaxBidAmount           float64  `json:"max_bid_amount"`
}

func NewAccount(email, name string, accountType AccountType) (*Account, error) {
	// Validate email
	if err := validation.ValidateEmail(email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}
	
	// Validate name
	if err := validation.ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid name: %w", err)
	}
	
	// Validate account type
	switch accountType {
	case TypeBuyer, TypeSeller, TypeAdmin:
		// Valid types
	default:
		return nil, ErrInvalidAccountType
	}
	
	now := time.Now()
	return &Account{
		ID:              uuid.New(),
		Email:           email,
		Name:            name,
		Type:            accountType,
		Status:          StatusPending,
		Balance:         0.0,
		CreditLimit:     1000.0,
		PaymentTerms:    30,
		QualityScore:    5.0,
		FraudScore:      0.0,
		Settings: AccountSettings{
			Timezone:               "UTC",
			CallNotifications:      true,
			EmailNotifications:     true,
			SMSNotifications:       false,
			AllowedCallingHours:    []int{9, 10, 11, 12, 13, 14, 15, 16, 17}, // 9 AM - 5 PM
			MaxConcurrentCalls:     10,
			AutoBidding:            false,
			MaxBidAmount:           10.0,
		},
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (a *Account) UpdateBalance(amount float64) error {
	newBalance := a.Balance + amount
	if newBalance < 0 && newBalance < -a.CreditLimit {
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

var (
	ErrInsufficientFunds = fmt.Errorf("insufficient funds")
	ErrAccountSuspended  = fmt.Errorf("account suspended")
	ErrInvalidAccountType = fmt.Errorf("invalid account type")
)