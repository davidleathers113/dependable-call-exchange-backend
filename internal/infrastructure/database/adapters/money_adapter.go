package adapters

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// MoneyAdapter handles database conversion for Money value objects
type MoneyAdapter struct{}

// NewMoneyAdapter creates a new money adapter
func NewMoneyAdapter() *MoneyAdapter {
	return &MoneyAdapter{}
}

// Scan implements sql.Scanner for Money value objects
// This method is called when reading from the database
func (a *MoneyAdapter) Scan(dest *values.Money, value interface{}) error {
	if value == nil {
		*dest = values.Zero("USD") // Default to USD zero
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return a.scanFromString(dest, string(v))
	case string:
		return a.scanFromString(dest, v)
	default:
		return fmt.Errorf("cannot scan %T into Money", value)
	}
}

// Value implements driver.Valuer for Money value objects
// This method is called when writing to the database
func (a *MoneyAdapter) Value(src values.Money) (driver.Value, error) {
	if src.IsZero() && src.Currency() == "" {
		return nil, nil
	}
	// Store as JSON for PostgreSQL JSONB compatibility
	return src.MarshalJSON()
}

// ScanNullable handles nullable Money fields
func (a *MoneyAdapter) ScanNullable(dest **values.Money, value interface{}) error {
	if value == nil {
		*dest = nil
		return nil
	}

	money := &values.Money{}
	err := a.Scan(money, value)
	if err != nil {
		return err
	}

	*dest = money
	return nil
}

// ValueNullable handles nullable Money fields
func (a *MoneyAdapter) ValueNullable(src *values.Money) (driver.Value, error) {
	if src == nil {
		return nil, nil
	}
	return a.Value(*src)
}

// ScanFromFloat64 scans a float64 amount with currency into Money
// Useful for legacy database fields that store money as float64
func (a *MoneyAdapter) ScanFromFloat64(dest *values.Money, amount float64, currency string) error {
	money, err := values.NewMoneyFromFloat(amount, currency)
	if err != nil {
		return fmt.Errorf("failed to create money from float64: %w", err)
	}
	*dest = money
	return nil
}

// ValueAsFloat64 returns the Money amount as float64
// Useful for legacy database fields that store money as float64
func (a *MoneyAdapter) ValueAsFloat64(src values.Money) float64 {
	return src.ToFloat64()
}

// Helper function to scan from string representation
func (a *MoneyAdapter) scanFromString(dest *values.Money, s string) error {
	// Try to parse as JSON first
	if strings.HasPrefix(s, "{") {
		return dest.UnmarshalJSON([]byte(s))
	}

	// Fall back to simple decimal parsing (assume USD)
	money, err := values.NewMoneyFromString(s, "USD")
	if err != nil {
		return fmt.Errorf("invalid money format: %w", err)
	}

	*dest = money
	return nil
}
