// Package adapters provides infrastructure adapters for domain value objects.
//
// This package isolates database concerns from domain value objects by providing
// dedicated adapters that handle conversion between pure domain objects and
// database representations. This maintains clean architecture by ensuring
// domain objects don't import database/sql or other infrastructure dependencies.
//
// Each adapter implements the Scan and Value methods for sql.Scanner and
// driver.Valuer interfaces, allowing seamless integration with SQL databases
// while keeping the domain layer pure.
package adapters

import (
	"database/sql/driver"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Adapters provides a centralized interface to all value object adapters
type Adapters struct {
	Email          *EmailAdapter
	Money          *MoneyAdapter
	Phone          *PhoneAdapter
	QualityMetrics *QualityMetricsAdapter
}

// NewAdapters creates a new instance of all adapters
func NewAdapters() *Adapters {
	return &Adapters{
		Email:          NewEmailAdapter(),
		Money:          NewMoneyAdapter(),
		Phone:          NewPhoneAdapter(),
		QualityMetrics: NewQualityMetricsAdapter(),
	}
}

// Scanner interface for value objects that can be scanned from database
type Scanner[T any] interface {
	Scan(dest *T, value interface{}) error
}

// Valuer interface for value objects that can be stored in database
type Valuer[T any] interface {
	Value(src T) (driver.Value, error)
}

// Adapter combines Scanner and Valuer for bidirectional conversion
type Adapter[T any] interface {
	Scanner[T]
	Valuer[T]
}

// Convenience functions for direct usage without adapter instances

// ScanEmail scans a database value into an Email value object
func ScanEmail(dest *values.Email, value interface{}) error {
	adapter := NewEmailAdapter()
	return adapter.Scan(dest, value)
}

// ValueEmail converts an Email value object to a database value
func ValueEmail(src values.Email) (driver.Value, error) {
	adapter := NewEmailAdapter()
	return adapter.Value(src)
}

// ScanMoney scans a database value into a Money value object
func ScanMoney(dest *values.Money, value interface{}) error {
	adapter := NewMoneyAdapter()
	return adapter.Scan(dest, value)
}

// ValueMoney converts a Money value object to a database value
func ValueMoney(src values.Money) (driver.Value, error) {
	adapter := NewMoneyAdapter()
	return adapter.Value(src)
}

// ScanPhone scans a database value into a PhoneNumber value object
func ScanPhone(dest *values.PhoneNumber, value interface{}) error {
	adapter := NewPhoneAdapter()
	return adapter.Scan(dest, value)
}

// ValuePhone converts a PhoneNumber value object to a database value
func ValuePhone(src values.PhoneNumber) (driver.Value, error) {
	adapter := NewPhoneAdapter()
	return adapter.Value(src)
}

// ScanQualityMetrics scans a database value into a QualityMetrics value object
func ScanQualityMetrics(dest *values.QualityMetrics, value interface{}) error {
	adapter := NewQualityMetricsAdapter()
	return adapter.Scan(dest, value)
}

// ValueQualityMetrics converts a QualityMetrics value object to a database value
func ValueQualityMetrics(src values.QualityMetrics) (driver.Value, error) {
	adapter := NewQualityMetricsAdapter()
	return adapter.Value(src)
}
