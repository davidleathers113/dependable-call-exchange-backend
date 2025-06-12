package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
)

// Email represents a validated email address value object
type Email struct {
	address string
}

var (
	// RFC 5322 compliant regex for stricter validation
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// NewEmail creates a new Email value object with validation
func NewEmail(address string) (Email, error) {
	if address == "" {
		return Email{}, fmt.Errorf("email address cannot be empty")
	}

	// Normalize the email address
	normalized := normalizeEmail(address)

	// Validate using Go's mail package (RFC 5322 compliant)
	parsed, err := mail.ParseAddress(normalized)
	if err != nil {
		return Email{}, fmt.Errorf("invalid email format: %w", err)
	}

	// Additional regex validation for stricter rules
	if !emailRegex.MatchString(parsed.Address) {
		return Email{}, fmt.Errorf("email address does not meet format requirements")
	}

	// Length validation
	if len(parsed.Address) > 254 {
		return Email{}, fmt.Errorf("email address too long (max 254 characters)")
	}

	return Email{address: parsed.Address}, nil
}

// MustNewEmail creates Email and panics on error (for constants/tests)
func MustNewEmail(address string) Email {
	email, err := NewEmail(address)
	if err != nil {
		panic(err)
	}
	return email
}

// String returns the email address
func (e Email) String() string {
	return e.address
}

// Address returns the email address (alias for String)
func (e Email) Address() string {
	return e.address
}

// LocalPart returns the local part of the email (before @)
func (e Email) LocalPart() string {
	parts := strings.Split(e.address, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[0]
}

// Domain returns the domain part of the email (after @)
func (e Email) Domain() string {
	parts := strings.Split(e.address, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// IsEmpty checks if the email is empty
func (e Email) IsEmpty() bool {
	return e.address == ""
}

// Equal checks if two Email values are equal
func (e Email) Equal(other Email) bool {
	return e.address == other.address
}

// IsDomainAllowed checks if the email domain is in the allowed list
func (e Email) IsDomainAllowed(allowedDomains []string) bool {
	domain := e.Domain()
	for _, allowed := range allowedDomains {
		if strings.EqualFold(domain, allowed) {
			return true
		}
	}
	return false
}

// IsDisposable checks if the email is from a known disposable email provider
func (e Email) IsDisposable() bool {
	domain := strings.ToLower(e.Domain())

	// Common disposable email domains
	disposableDomains := map[string]bool{
		"10minutemail.com":  true,
		"guerrillamail.com": true,
		"mailinator.com":    true,
		"tempmail.org":      true,
		"yopmail.com":       true,
		"temp-mail.org":     true,
		"throwaway.email":   true,
	}

	return disposableDomains[domain]
}

// MarshalJSON implements JSON marshaling
func (e Email) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.address)
}

// UnmarshalJSON implements JSON unmarshaling
func (e *Email) UnmarshalJSON(data []byte) error {
	var address string
	if err := json.Unmarshal(data, &address); err != nil {
		return err
	}

	email, err := NewEmail(address)
	if err != nil {
		return err
	}

	*e = email
	return nil
}

// Value implements driver.Valuer for database storage
func (e Email) Value() (driver.Value, error) {
	return e.address, nil
}

// Scan implements sql.Scanner for database retrieval
func (e *Email) Scan(value interface{}) error {
	if value == nil {
		*e = Email{}
		return nil
	}

	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into Email", value)
	}

	email, err := NewEmail(str)
	if err != nil {
		return err
	}

	*e = email
	return nil
}

// Helper functions

func normalizeEmail(address string) string {
	// Trim whitespace and convert to lowercase
	normalized := strings.TrimSpace(strings.ToLower(address))
	return normalized
}

// EmailValidationError represents validation errors for email addresses
type EmailValidationError struct {
	Address string
	Reason  string
}

func (e EmailValidationError) Error() string {
	return fmt.Sprintf("invalid email '%s': %s", e.Address, e.Reason)
}

// ValidateEmailDomain validates if a domain is acceptable for emails
func ValidateEmailDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	// Basic domain validation
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain format")
	}

	// Check for blocked domains
	blockedDomains := []string{
		"example.com",
		"test.com",
		"localhost",
	}

	for _, blocked := range blockedDomains {
		if strings.EqualFold(domain, blocked) {
			return fmt.Errorf("domain '%s' is not allowed", domain)
		}
	}

	return nil
}
