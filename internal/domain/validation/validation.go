package validation

import (
	"fmt"
	"math"
	"net/mail"
	"regexp"
	"strings"
)

var (
	// Phone number validation - supports E.164 format and common formats
	phoneRegex = regexp.MustCompile(`^(\+?[1-9]\d{0,14}|\d{10,15})$`)
	
	// Email validation uses Go's mail.ParseAddress
	// Name validation - allows letters, spaces, hyphens, apostrophes
	nameRegex = regexp.MustCompile(`^[\p{L}\s\-'\.]{2,100}$`)
	
	// Company name - more permissive, allows numbers and common business chars
	companyRegex = regexp.MustCompile(`^[\p{L}\p{N}\s\-'\.&,()]{2,200}$`)
	
	// Address validation patterns
	zipCodeRegex = regexp.MustCompile(`^\d{5}(-\d{4})?$`) // US ZIP codes
	stateRegex   = regexp.MustCompile(`^[A-Z]{2}$`)       // US state codes
)

// ValidateEmail validates email format
func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))
	
	// Parse email address
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}
	
	// Additional length check
	if len(email) > 255 {
		return fmt.Errorf("email too long (max 255 characters)")
	}
	
	return nil
}

// ValidatePhoneNumber validates phone number format
func ValidatePhoneNumber(phone string) error {
	if phone == "" {
		return fmt.Errorf("phone number cannot be empty")
	}
	
	// Remove common formatting characters
	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == '+' {
			return r
		}
		return -1
	}, phone)
	
	if !phoneRegex.MatchString(cleaned) {
		return fmt.Errorf("invalid phone number format")
	}
	
	// Check minimum length
	if len(cleaned) < 10 {
		return fmt.Errorf("phone number too short")
	}
	
	return nil
}

// ValidateName validates person name
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	
	name = strings.TrimSpace(name)
	
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

// ValidateCompanyName validates company name
func ValidateCompanyName(company string) error {
	if company == "" {
		return nil // Company is optional
	}
	
	company = strings.TrimSpace(company)
	
	if !companyRegex.MatchString(company) {
		return fmt.Errorf("invalid company name format")
	}
	
	if len(company) > 200 {
		return fmt.Errorf("company name too long (max 200 characters)")
	}
	
	return nil
}

// ValidateAddress validates address components
func ValidateAddress(street, city, state, zipCode, country string) error {
	if street == "" || city == "" || state == "" || zipCode == "" || country == "" {
		return fmt.Errorf("all address fields are required")
	}
	
	// Validate US addresses (can be extended for other countries)
	if strings.ToUpper(country) == "US" || strings.ToUpper(country) == "USA" {
		if !stateRegex.MatchString(strings.ToUpper(state)) {
			return fmt.Errorf("invalid US state code")
		}
		
		if !zipCodeRegex.MatchString(zipCode) {
			return fmt.Errorf("invalid US ZIP code format")
		}
	}
	
	// General length checks
	if len(street) > 200 {
		return fmt.Errorf("street address too long (max 200 characters)")
	}
	
	if len(city) > 100 {
		return fmt.Errorf("city name too long (max 100 characters)")
	}
	
	return nil
}

// ValidateAmount validates monetary amounts
func ValidateAmount(amount float64, fieldName string) error {
	// Check for special floating point values
	if math.IsNaN(amount) {
		return fmt.Errorf("%s cannot be NaN", fieldName)
	}
	
	if math.IsInf(amount, 0) {
		return fmt.Errorf("%s cannot be infinite", fieldName)
	}
	
	if amount < 0 {
		return fmt.Errorf("%s cannot be negative", fieldName)
	}
	
	// Check for values that would overflow when used in calculations
	// This prevents issues with extreme float values in property tests
	if amount > 1e15 { // Much smaller than float64 max to prevent overflow in calculations
		return fmt.Errorf("%s too large (max 1 quadrillion)", fieldName)
	}
	
	// Check for reasonable maximum for actual business logic
	if amount > 1e9 { // 1 billion
		return fmt.Errorf("%s too large for business use (max 1 billion)", fieldName)
	}
	
	return nil
}

// ValidateDuration validates duration in seconds
func ValidateDuration(duration int) error {
	if duration < 0 {
		return fmt.Errorf("duration cannot be negative")
	}
	
	// Max call duration of 24 hours (86400 seconds)
	if duration > 86400 {
		return fmt.Errorf("duration too long (max 24 hours)")
	}
	
	// Check for integer overflow edge cases
	if duration == math.MaxInt || duration == math.MinInt {
		return fmt.Errorf("duration value is invalid")
	}
	
	return nil
}