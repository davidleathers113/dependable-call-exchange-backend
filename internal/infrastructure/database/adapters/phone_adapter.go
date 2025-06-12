package adapters

import (
	"database/sql/driver"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// PhoneAdapter handles database conversion for PhoneNumber value objects
type PhoneAdapter struct{}

// NewPhoneAdapter creates a new phone adapter
func NewPhoneAdapter() *PhoneAdapter {
	return &PhoneAdapter{}
}

// Scan implements sql.Scanner for PhoneNumber value objects
// This method is called when reading from the database
func (a *PhoneAdapter) Scan(dest *values.PhoneNumber, value interface{}) error {
	if value == nil {
		*dest = values.PhoneNumber{}
		return nil
	}

	var number string
	switch v := value.(type) {
	case []byte:
		number = string(v)
	case string:
		number = v
	default:
		return fmt.Errorf("cannot scan %T into PhoneNumber", value)
	}

	phone, err := values.NewPhoneNumber(number)
	if err != nil {
		return err
	}

	*dest = phone
	return nil
}

// Value implements driver.Valuer for PhoneNumber value objects
// This method is called when writing to the database
func (a *PhoneAdapter) Value(src values.PhoneNumber) (driver.Value, error) {
	if src.IsEmpty() {
		return nil, nil
	}
	return src.String(), nil
}

// ScanNullable handles nullable PhoneNumber fields
func (a *PhoneAdapter) ScanNullable(dest **values.PhoneNumber, value interface{}) error {
	if value == nil {
		*dest = nil
		return nil
	}

	phone := &values.PhoneNumber{}
	err := a.Scan(phone, value)
	if err != nil {
		return err
	}

	*dest = phone
	return nil
}

// ValueNullable handles nullable PhoneNumber fields
func (a *PhoneAdapter) ValueNullable(src *values.PhoneNumber) (driver.Value, error) {
	if src == nil {
		return nil, nil
	}
	return a.Value(*src)
}
