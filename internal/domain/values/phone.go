package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// PhoneNumber represents a validated phone number value object
type PhoneNumber struct {
	number string // Stored in E.164 format (+1234567890)
}

var (
	// E.164 format regex: + followed by up to 15 digits
	e164Regex = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

	// US phone number regex for parsing various formats
	usPhoneRegex = regexp.MustCompile(`^(?:\+?1[-.\s]?)?\(?([0-9]{3})\)?[-.\s]?([0-9]{3})[-.\s]?([0-9]{4})$`)

	// International phone regex for basic validation
	intlPhoneRegex = regexp.MustCompile(`^(?:\+?[1-9]\d{0,3})?[-.\s]?\(?[0-9]{1,4}\)?[-.\s]?[0-9]{1,4}[-.\s]?[0-9]{1,9}$`)
)

// NewPhoneNumber creates a new PhoneNumber value object with validation
func NewPhoneNumber(number string) (PhoneNumber, error) {
	if number == "" {
		return PhoneNumber{}, fmt.Errorf("phone number cannot be empty")
	}

	// Clean and normalize the input
	cleaned := cleanPhoneNumber(number)

	// Try to parse as E.164 format first
	if e164Regex.MatchString(cleaned) {
		return PhoneNumber{number: cleaned}, nil
	}

	// Try to parse as US phone number
	if normalized, ok := parseUSPhoneNumber(number); ok {
		return PhoneNumber{number: normalized}, nil
	}

	// Try basic international format
	if normalized, ok := parseInternationalPhoneNumber(number); ok {
		return PhoneNumber{number: normalized}, nil
	}

	return PhoneNumber{}, fmt.Errorf("invalid phone number format: %s", number)
}

// NewPhoneNumberE164 creates a PhoneNumber from E.164 format with strict validation
func NewPhoneNumberE164(number string) (PhoneNumber, error) {
	if !e164Regex.MatchString(number) {
		return PhoneNumber{}, fmt.Errorf("invalid E.164 format: %s", number)
	}

	return PhoneNumber{number: number}, nil
}

// MustNewPhoneNumber creates PhoneNumber and panics on error (for constants/tests)
func MustNewPhoneNumber(number string) PhoneNumber {
	phone, err := NewPhoneNumber(number)
	if err != nil {
		panic(err)
	}
	return phone
}

// String returns the phone number in E.164 format
func (p PhoneNumber) String() string {
	return p.number
}

// E164 returns the phone number in E.164 format (alias for String)
func (p PhoneNumber) E164() string {
	return p.number
}

// IsEmpty checks if the phone number is empty
func (p PhoneNumber) IsEmpty() bool {
	return p.number == ""
}

// Equal checks if two PhoneNumber values are equal
func (p PhoneNumber) Equal(other PhoneNumber) bool {
	return p.number == other.number
}

// CountryCode returns the country code (e.g., "+1" for US/Canada)
func (p PhoneNumber) CountryCode() string {
	if len(p.number) < 2 {
		return ""
	}

	// Extract country code - this is a simplified implementation
	if strings.HasPrefix(p.number, "+1") {
		return "+1"
	}
	if strings.HasPrefix(p.number, "+44") {
		return "+44"
	}
	if strings.HasPrefix(p.number, "+33") {
		return "+33"
	}
	if strings.HasPrefix(p.number, "+49") {
		return "+49"
	}

	// Generic extraction for other countries (up to 4 digits)
	for i := 2; i <= 5 && i < len(p.number); i++ {
		return p.number[:i]
	}

	return ""
}

// NationalNumber returns the national number (without country code)
func (p PhoneNumber) NationalNumber() string {
	countryCode := p.CountryCode()
	if countryCode == "" {
		return p.number
	}
	return p.number[len(countryCode):]
}

// FormatUS returns US-formatted phone number (XXX) XXX-XXXX
func (p PhoneNumber) FormatUS() string {
	if !p.IsUS() {
		return p.number
	}

	national := p.NationalNumber()
	if len(national) != 10 {
		return p.number
	}

	return fmt.Sprintf("(%s) %s-%s",
		national[:3],
		national[3:6],
		national[6:])
}

// FormatInternational returns international format with spaces
func (p PhoneNumber) FormatInternational() string {
	countryCode := p.CountryCode()
	national := p.NationalNumber()

	if countryCode == "" {
		return p.number
	}

	// Add spaces for readability
	return countryCode + " " + formatNationalWithSpaces(national)
}

// IsUS checks if the phone number is from US/Canada (+1)
func (p PhoneNumber) IsUS() bool {
	return strings.HasPrefix(p.number, "+1")
}

// IsMobile checks if the phone number is likely a mobile number (US only)
func (p PhoneNumber) IsMobile() bool {
	if !p.IsUS() {
		return false // Cannot determine for international numbers
	}

	national := p.NationalNumber()
	if len(national) != 10 {
		return false
	}

	// US mobile number area codes (simplified)
	areaCode := national[:3]
	mobileAreaCodes := map[string]bool{
		"201": true, "202": true, "203": true, "204": true, "205": true,
		// Add more mobile area codes as needed
	}

	return mobileAreaCodes[areaCode]
}

// AreaCode returns the area code for US numbers
func (p PhoneNumber) AreaCode() string {
	if !p.IsUS() {
		return ""
	}

	national := p.NationalNumber()
	if len(national) != 10 {
		return ""
	}

	return national[:3]
}

// Exchange returns the exchange code for US numbers
func (p PhoneNumber) Exchange() string {
	if !p.IsUS() {
		return ""
	}

	national := p.NationalNumber()
	if len(national) != 10 {
		return ""
	}

	return national[3:6]
}

// Subscriber returns the subscriber number for US numbers
func (p PhoneNumber) Subscriber() string {
	if !p.IsUS() {
		return ""
	}

	national := p.NationalNumber()
	if len(national) != 10 {
		return ""
	}

	return national[6:]
}

// MarshalJSON implements JSON marshaling
func (p PhoneNumber) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.number)
}

// UnmarshalJSON implements JSON unmarshaling
func (p *PhoneNumber) UnmarshalJSON(data []byte) error {
	var number string
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}

	phone, err := NewPhoneNumber(number)
	if err != nil {
		return err
	}

	*p = phone
	return nil
}

// Value implements driver.Valuer for database storage
func (p PhoneNumber) Value() (driver.Value, error) {
	if p.number == "" {
		return nil, nil
	}
	return p.number, nil
}

// Scan implements sql.Scanner for database retrieval
func (p *PhoneNumber) Scan(value interface{}) error {
	if value == nil {
		*p = PhoneNumber{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into PhoneNumber", value)
	}

	if str == "" {
		*p = PhoneNumber{}
		return nil
	}

	phone, err := NewPhoneNumber(str)
	if err != nil {
		return err
	}

	*p = phone
	return nil
}

// Helper functions

func cleanPhoneNumber(number string) string {
	// Remove all non-digit characters except +
	cleaned := ""
	for _, char := range number {
		if char >= '0' && char <= '9' || char == '+' {
			cleaned += string(char)
		}
	}
	return cleaned
}

func parseUSPhoneNumber(number string) (string, bool) {
	matches := usPhoneRegex.FindStringSubmatch(number)
	if len(matches) != 4 {
		return "", false
	}

	// Format as E.164 (+1AAANNNNNNN)
	return "+1" + matches[1] + matches[2] + matches[3], true
}

func parseInternationalPhoneNumber(number string) (string, bool) {
	cleaned := cleanPhoneNumber(number)

	// Must start with + for international
	if !strings.HasPrefix(cleaned, "+") {
		return "", false
	}

	// Must be valid E.164 format
	if !e164Regex.MatchString(cleaned) {
		return "", false
	}

	return cleaned, true
}

func formatNationalWithSpaces(national string) string {
	if len(national) <= 4 {
		return national
	}

	// For US numbers (10 digits), format as XXX XXX XXXX
	if len(national) == 10 {
		return national[:3] + " " + national[3:6] + " " + national[6:]
	}

	// For other numbers, add space every 3 digits for readability
	result := ""
	for i, char := range national {
		if i > 0 && i%3 == 0 {
			result += " "
		}
		result += string(char)
	}

	return result
}

// PhoneValidationError represents validation errors for phone numbers
type PhoneValidationError struct {
	Number string
	Reason string
}

func (e PhoneValidationError) Error() string {
	return fmt.Sprintf("invalid phone number '%s': %s", e.Number, e.Reason)
}
