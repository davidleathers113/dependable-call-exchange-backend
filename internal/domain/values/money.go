package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Money represents a monetary value with currency and precision handling
type Money struct {
	amount   decimal.Decimal
	currency string
}

// Common currency codes (ISO 4217)
const (
	USD = "USD"
	EUR = "EUR"
	GBP = "GBP"
	JPY = "JPY"
	CAD = "CAD"
)

// NewMoney creates a new Money value object
func NewMoney(amount decimal.Decimal, currency string) (Money, error) {
	if err := validateCurrency(currency); err != nil {
		return Money{}, err
	}
	
	return Money{
		amount:   amount,
		currency: currency,
	}, nil
}

// NewMoneyFromString creates Money from string amount and currency
func NewMoneyFromString(amount, currency string) (Money, error) {
	dec, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, fmt.Errorf("invalid amount: %w", err)
	}
	
	return NewMoney(dec, currency)
}

// NewMoneyFromFloat creates Money from float64 amount and currency
// Note: Use with caution due to floating point precision issues
func NewMoneyFromFloat(amount float64, currency string) (Money, error) {
	dec := decimal.NewFromFloat(amount)
	return NewMoney(dec, currency)
}

// NewMoneyFromCents creates Money from integer cents (smallest unit)
func NewMoneyFromCents(cents int64, currency string) (Money, error) {
	dec := decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
	return NewMoney(dec, currency)
}

// MustNewMoney creates Money and panics on error (for constants/tests)
func MustNewMoney(amount decimal.Decimal, currency string) Money {
	m, err := NewMoney(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// MustNewMoneyFromFloat creates Money from float and panics on error (for constants/tests)
func MustNewMoneyFromFloat(amount float64, currency string) Money {
	m, err := NewMoneyFromFloat(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// Zero returns a zero Money value in the given currency
func Zero(currency string) Money {
	return MustNewMoney(decimal.Zero, currency)
}

// Amount returns the decimal amount
func (m Money) Amount() decimal.Decimal {
	return m.amount
}

// Currency returns the currency code
func (m Money) Currency() string {
	return m.currency
}

// String returns formatted money string (e.g., "$123.45")
func (m Money) String() string {
	symbol := getCurrencySymbol(m.currency)
	return symbol + m.amount.StringFixed(2)
}

// StringWithCode returns money with currency code (e.g., "123.45 USD")
func (m Money) StringWithCode() string {
	return m.amount.StringFixed(2) + " " + m.currency
}

// IsZero checks if the amount is zero
func (m Money) IsZero() bool {
	return m.amount.IsZero()
}

// IsPositive checks if the amount is positive
func (m Money) IsPositive() bool {
	return m.amount.IsPositive()
}

// IsNegative checks if the amount is negative
func (m Money) IsNegative() bool {
	return m.amount.IsNegative()
}

// Equal checks if two Money values are equal (same amount and currency)
func (m Money) Equal(other Money) bool {
	return m.amount.Equal(other.amount) && m.currency == other.currency
}

// Compare returns -1, 0, or 1 based on comparison with other Money
// Panics if currencies don't match
func (m Money) Compare(other Money) int {
	if m.currency != other.currency {
		panic(fmt.Sprintf("cannot compare different currencies: %s vs %s", m.currency, other.currency))
	}
	return m.amount.Cmp(other.amount)
}

// Add adds two Money values (must have same currency)
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("cannot add different currencies: %s and %s", m.currency, other.currency)
	}
	
	return Money{
		amount:   m.amount.Add(other.amount),
		currency: m.currency,
	}, nil
}

// Sub subtracts other Money from this Money (must have same currency)
func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("cannot subtract different currencies: %s and %s", m.currency, other.currency)
	}
	
	return Money{
		amount:   m.amount.Sub(other.amount),
		currency: m.currency,
	}, nil
}

// Mul multiplies Money by a decimal factor
func (m Money) Mul(factor decimal.Decimal) Money {
	return Money{
		amount:   m.amount.Mul(factor),
		currency: m.currency,
	}
}

// Div divides Money by a decimal factor
func (m Money) Div(factor decimal.Decimal) (Money, error) {
	if factor.IsZero() {
		return Money{}, fmt.Errorf("division by zero")
	}
	
	return Money{
		amount:   m.amount.Div(factor),
		currency: m.currency,
	}, nil
}

// MulFloat multiplies Money by a float64 factor
func (m Money) MulFloat(factor float64) Money {
	return m.Mul(decimal.NewFromFloat(factor))
}

// Round rounds the amount to the specified decimal places
func (m Money) Round(places int32) Money {
	return Money{
		amount:   m.amount.Round(places),
		currency: m.currency,
	}
}

// RoundToNearestCent rounds to 2 decimal places
func (m Money) RoundToNearestCent() Money {
	return m.Round(2)
}

// ToCents converts to integer cents (smallest unit)
func (m Money) ToCents() int64 {
	return m.amount.Mul(decimal.NewFromInt(100)).IntPart()
}

// ToFloat64 converts to float64 (use with caution for precision)
func (m Money) ToFloat64() float64 {
	f, _ := m.amount.Float64()
	return f
}

// JSON marshaling
func (m Money) MarshalJSON() ([]byte, error) {
	data := struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}{
		Amount:   m.amount.String(),
		Currency: m.currency,
	}
	return json.Marshal(data)
}

// JSON unmarshaling
func (m *Money) UnmarshalJSON(data []byte) error {
	var temp struct {
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	
	amount, err := decimal.NewFromString(temp.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}
	
	money, err := NewMoney(amount, temp.Currency)
	if err != nil {
		return err
	}
	
	*m = money
	return nil
}

// Database scanning (implements sql.Scanner)
func (m *Money) Scan(value interface{}) error {
	if value == nil {
		*m = Money{}
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		return m.scanFromString(string(v))
	case string:
		return m.scanFromString(v)
	default:
		return fmt.Errorf("cannot scan %T into Money", value)
	}
}

// Database value (implements driver.Valuer)
func (m Money) Value() (driver.Value, error) {
	if m.amount.IsZero() && m.currency == "" {
		return nil, nil
	}
	// Store as JSON for PostgreSQL JSONB compatibility
	return m.MarshalJSON()
}

// Helper functions

func validateCurrency(currency string) error {
	if currency == "" {
		return fmt.Errorf("currency cannot be empty")
	}
	
	currency = strings.ToUpper(currency)
	
	// Basic ISO 4217 format validation
	if len(currency) != 3 {
		return fmt.Errorf("currency code must be 3 characters")
	}
	
	// Check against common currencies (can be extended)
	validCurrencies := map[string]bool{
		USD: true, EUR: true, GBP: true, JPY: true, CAD: true,
		"AUD": true, "CHF": true, "CNY": true, "SEK": true, "NZD": true,
	}
	
	if !validCurrencies[currency] {
		return fmt.Errorf("unsupported currency: %s", currency)
	}
	
	return nil
}

func getCurrencySymbol(currency string) string {
	symbols := map[string]string{
		USD: "$",
		EUR: "€",
		GBP: "£",
		JPY: "¥",
		CAD: "C$",
	}
	
	if symbol, ok := symbols[currency]; ok {
		return symbol
	}
	return currency + " "
}

func (m *Money) scanFromString(s string) error {
	// Try to parse as JSON first
	if strings.HasPrefix(s, "{") {
		return m.UnmarshalJSON([]byte(s))
	}
	
	// Fall back to simple decimal parsing (assume USD)
	amount, err := decimal.NewFromString(s)
	if err != nil {
		return fmt.Errorf("invalid money format: %w", err)
	}
	
	money, err := NewMoney(amount, USD)
	if err != nil {
		return err
	}
	
	*m = money
	return nil
}