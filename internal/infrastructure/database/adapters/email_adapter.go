package adapters

import (
	"database/sql/driver"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// EmailAdapter handles database conversion for Email value objects
type EmailAdapter struct{}

// NewEmailAdapter creates a new email adapter
func NewEmailAdapter() *EmailAdapter {
	return &EmailAdapter{}
}

// Scan implements sql.Scanner for Email value objects
// This method is called when reading from the database
func (a *EmailAdapter) Scan(dest *values.Email, value interface{}) error {
	if value == nil {
		*dest = values.Email{}
		return nil
	}

	var address string
	switch v := value.(type) {
	case []byte:
		address = string(v)
	case string:
		address = v
	default:
		return fmt.Errorf("cannot scan %T into Email", value)
	}

	email, err := values.NewEmail(address)
	if err != nil {
		return err
	}

	*dest = email
	return nil
}

// Value implements driver.Valuer for Email value objects
// This method is called when writing to the database
func (a *EmailAdapter) Value(src values.Email) (driver.Value, error) {
	if src.IsEmpty() {
		return nil, nil
	}
	return src.String(), nil
}

// ScanNullable handles nullable Email fields
func (a *EmailAdapter) ScanNullable(dest **values.Email, value interface{}) error {
	if value == nil {
		*dest = nil
		return nil
	}

	email := &values.Email{}
	err := a.Scan(email, value)
	if err != nil {
		return err
	}

	*dest = email
	return nil
}

// ValueNullable handles nullable Email fields
func (a *EmailAdapter) ValueNullable(src *values.Email) (driver.Value, error) {
	if src == nil {
		return nil, nil
	}
	return a.Value(*src)
}
