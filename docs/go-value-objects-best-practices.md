# Go Value Objects: Best Practices & Implementation Patterns

This document provides comprehensive guidance for implementing value objects in Go, focusing on money/currency types, email/phone validation, JSON marshaling, database integration, comparison methods, immutability patterns, and error handling.

## Table of Contents

1. [Overview](#overview)
2. [Money/Currency Types with Decimal Precision](#moneycurrency-types-with-decimal-precision)
3. [Email Value Objects with Validation](#email-value-objects-with-validation)
4. [Phone Number Value Objects with Formatting](#phone-number-value-objects-with-formatting)
5. [JSON Marshaling/Unmarshaling Patterns](#json-marshalingunmarshaling-patterns)
6. [Database Scanning Patterns](#database-scanning-patterns)
7. [Comparison and Equality Methods](#comparison-and-equality-methods)
8. [Immutability Patterns in Go](#immutability-patterns-in-go)
9. [Error Handling in Value Object Constructors](#error-handling-in-value-object-constructors)
10. [Complete Examples](#complete-examples)

## Overview

Value objects are immutable objects that represent descriptive aspects of a domain with no conceptual identity. In Go, they help:

- Eliminate "stringly typed" code
- Provide type safety
- Encapsulate domain logic
- Enable proper validation
- Reduce memory overhead through immutability

**Key Characteristics:**
1. **Immutable** - State cannot change after creation
2. **Value Equality** - Two value objects are equal if their values are equal
3. **No Identity** - Only the values matter, not object identity
4. **Self-Validating** - Always in a valid state

## Money/Currency Types with Decimal Precision

### Why Avoid Floats for Money

```go
// ❌ DON'T: Floats cause precision errors
price := 0.1 + 0.2  // Results in 0.30000000000000004
```

### Pattern 1: Integer-Based Storage (Simple)

```go
package money

import (
    "fmt"
    "strconv"
    "strings"
)

// USD represents monetary value in USD cents to avoid floating-point errors
type USD int64

// NewUSD creates a new USD value from dollars and cents
func NewUSD(dollars, cents int64) USD {
    if cents < 0 || cents >= 100 {
        panic("cents must be between 0 and 99")
    }
    return USD(dollars*100 + cents)
}

// NewUSDFromFloat creates USD from a float64 dollar amount
func NewUSDFromFloat(amount float64) USD {
    return USD(amount * 100 + 0.5) // Add 0.5 for proper rounding
}

// NewUSDFromString parses a string like "12.34" into USD
func NewUSDFromString(s string) (USD, error) {
    parts := strings.Split(s, ".")
    if len(parts) > 2 {
        return 0, fmt.Errorf("invalid money format: %s", s)
    }
    
    dollars, err := strconv.ParseInt(parts[0], 10, 64)
    if err != nil {
        return 0, fmt.Errorf("invalid dollars: %w", err)
    }
    
    cents := int64(0)
    if len(parts) == 2 {
        centStr := parts[1]
        if len(centStr) > 2 {
            return 0, fmt.Errorf("too many decimal places: %s", s)
        }
        if len(centStr) == 1 {
            centStr += "0" // "1.5" -> "1.50"
        }
        cents, err = strconv.ParseInt(centStr, 10, 64)
        if err != nil {
            return 0, fmt.Errorf("invalid cents: %w", err)
        }
    }
    
    return NewUSD(dollars, cents), nil
}

// Dollars returns the dollar portion
func (u USD) Dollars() int64 {
    return int64(u) / 100
}

// Cents returns the cents portion
func (u USD) Cents() int64 {
    return int64(u) % 100
}

// Float64 returns the value as a float64 (use with caution)
func (u USD) Float64() float64 {
    return float64(u) / 100.0
}

// String implements fmt.Stringer
func (u USD) String() string {
    return fmt.Sprintf("$%d.%02d", u.Dollars(), u.Cents())
}

// Add returns a new USD with the sum
func (u USD) Add(other USD) USD {
    return USD(int64(u) + int64(other))
}

// Subtract returns a new USD with the difference
func (u USD) Subtract(other USD) USD {
    return USD(int64(u) - int64(other))
}

// Multiply returns a new USD multiplied by a factor
func (u USD) Multiply(factor float64) USD {
    return USD(float64(u)*factor + 0.5) // Add 0.5 for proper rounding
}

// IsZero returns true if the value is zero
func (u USD) IsZero() bool {
    return u == 0
}

// IsPositive returns true if the value is positive
func (u USD) IsPositive() bool {
    return u > 0
}

// IsNegative returns true if the value is negative
func (u USD) IsNegative() bool {
    return u < 0
}

// Equal implements equality comparison
func (u USD) Equal(other USD) bool {
    return u == other
}
```

### Pattern 2: Decimal-Based Storage (Complex)

```go
package money

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"
    
    "github.com/govalues/decimal"
)

// Money represents a monetary value with currency and precise decimal arithmetic
type Money struct {
    amount   decimal.Decimal
    currency string
}

// NewMoney creates a new Money value
func NewMoney(amount string, currency string) (Money, error) {
    if currency == "" {
        return Money{}, fmt.Errorf("currency cannot be empty")
    }
    
    dec, err := decimal.Parse(amount)
    if err != nil {
        return Money{}, fmt.Errorf("invalid amount %s: %w", amount, err)
    }
    
    return Money{
        amount:   dec,
        currency: currency,
    }, nil
}

// NewMoneyFromInt creates Money from integer cents
func NewMoneyFromInt(cents int64, currency string) (Money, error) {
    if currency == "" {
        return Money{}, fmt.Errorf("currency cannot be empty")
    }
    
    dec := decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
    return Money{
        amount:   dec,
        currency: currency,
    }, nil
}

// Amount returns the decimal amount
func (m Money) Amount() decimal.Decimal {
    return m.amount
}

// Currency returns the currency code
func (m Money) Currency() string {
    return m.currency
}

// String implements fmt.Stringer
func (m Money) String() string {
    return fmt.Sprintf("%s %s", m.amount.String(), m.currency)
}

// Add returns a new Money with the sum (same currency required)
func (m Money) Add(other Money) (Money, error) {
    if m.currency != other.currency {
        return Money{}, fmt.Errorf("cannot add different currencies: %s and %s", 
            m.currency, other.currency)
    }
    
    return Money{
        amount:   m.amount.Add(other.amount),
        currency: m.currency,
    }, nil
}

// Subtract returns a new Money with the difference
func (m Money) Subtract(other Money) (Money, error) {
    if m.currency != other.currency {
        return Money{}, fmt.Errorf("cannot subtract different currencies: %s and %s", 
            m.currency, other.currency)
    }
    
    return Money{
        amount:   m.amount.Sub(other.amount),
        currency: m.currency,
    }, nil
}

// Multiply returns a new Money multiplied by a decimal factor
func (m Money) Multiply(factor decimal.Decimal) Money {
    return Money{
        amount:   m.amount.Mul(factor),
        currency: m.currency,
    }
}

// IsZero returns true if the amount is zero
func (m Money) IsZero() bool {
    return m.amount.IsZero()
}

// IsPositive returns true if the amount is positive
func (m Money) IsPositive() bool {
    return m.amount.IsPos()
}

// IsNegative returns true if the amount is negative
func (m Money) IsNegative() bool {
    return m.amount.IsNeg()
}

// Equal implements equality comparison
func (m Money) Equal(other Money) bool {
    return m.amount.Equal(other.amount) && m.currency == other.currency
}
```

## Email Value Objects with Validation

```go
package email

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"
    "net/mail"
    "regexp"
    "strings"
)

// Email represents a validated email address
type Email struct {
    address string
}

// RFC 5322 compliant email regex (simplified)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// NewEmail creates a new Email value object with validation
func NewEmail(address string) (Email, error) {
    address = strings.TrimSpace(strings.ToLower(address))
    
    if address == "" {
        return Email{}, fmt.Errorf("email address cannot be empty")
    }
    
    // Use net/mail for RFC 5322 compliance
    parsed, err := mail.ParseAddress(address)
    if err != nil {
        return Email{}, fmt.Errorf("invalid email format: %w", err)
    }
    
    // Additional regex validation for stricter rules
    if !emailRegex.MatchString(parsed.Address) {
        return Email{}, fmt.Errorf("email format not allowed: %s", address)
    }
    
    return Email{address: parsed.Address}, nil
}

// MustNewEmail creates an Email or panics if invalid (use in tests/constants)
func MustNewEmail(address string) Email {
    email, err := NewEmail(address)
    if err != nil {
        panic(fmt.Sprintf("invalid email: %v", err))
    }
    return email
}

// String returns the email address
func (e Email) String() string {
    return e.address
}

// Value returns the underlying string value
func (e Email) Value() string {
    return e.address
}

// Domain returns the domain portion of the email
func (e Email) Domain() string {
    parts := strings.Split(e.address, "@")
    if len(parts) != 2 {
        return "" // Should never happen with validated email
    }
    return parts[1]
}

// LocalPart returns the local portion (before @) of the email
func (e Email) LocalPart() string {
    parts := strings.Split(e.address, "@")
    if len(parts) != 2 {
        return "" // Should never happen with validated email
    }
    return parts[0]
}

// Equal implements equality comparison
func (e Email) Equal(other Email) bool {
    return e.address == other.address
}

// IsEmpty returns true if the email is empty
func (e Email) IsEmpty() bool {
    return e.address == ""
}

// JSON Marshaling
func (e Email) MarshalJSON() ([]byte, error) {
    return json.Marshal(e.address)
}

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

// Database Scanning
func (e *Email) Scan(value interface{}) error {
    if value == nil {
        *e = Email{}
        return nil
    }
    
    var address string
    switch v := value.(type) {
    case string:
        address = v
    case []byte:
        address = string(v)
    default:
        return fmt.Errorf("cannot scan %T into Email", value)
    }
    
    email, err := NewEmail(address)
    if err != nil {
        return err
    }
    
    *e = email
    return nil
}

func (e Email) Value() (driver.Value, error) {
    if e.address == "" {
        return nil, nil
    }
    return e.address, nil
}
```

## Phone Number Value Objects with Formatting

```go
package phone

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"
    "regexp"
    "strings"
)

// Phone represents a validated phone number
type Phone struct {
    number      string // E.164 format: +1234567890
    countryCode string
    formatted   string // Human-readable format
}

// US phone number regex
var usPhoneRegex = regexp.MustCompile(`^\+?1?[-.\s]?\(?([0-9]{3})\)?[-.\s]?([0-9]{3})[-.\s]?([0-9]{4})$`)

// NewUSPhone creates a new US Phone number
func NewUSPhone(number string) (Phone, error) {
    // Remove all non-digit characters except +
    cleaned := regexp.MustCompile(`[^\d+]`).ReplaceAllString(number, "")
    
    // Remove leading +1 or 1 for US numbers
    if strings.HasPrefix(cleaned, "+1") {
        cleaned = cleaned[2:]
    } else if strings.HasPrefix(cleaned, "1") && len(cleaned) == 11 {
        cleaned = cleaned[1:]
    }
    
    if len(cleaned) != 10 {
        return Phone{}, fmt.Errorf("US phone number must be 10 digits, got %d", len(cleaned))
    }
    
    // Validate using regex
    e164 := "+1" + cleaned
    if !usPhoneRegex.MatchString(number) {
        return Phone{}, fmt.Errorf("invalid US phone number format: %s", number)
    }
    
    // Format as (XXX) XXX-XXXX
    formatted := fmt.Sprintf("(%s) %s-%s", 
        cleaned[0:3], cleaned[3:6], cleaned[6:10])
    
    return Phone{
        number:      e164,
        countryCode: "US",
        formatted:   formatted,
    }, nil
}

// NewPhone creates a phone number with country code validation
func NewPhone(number, countryCode string) (Phone, error) {
    switch strings.ToUpper(countryCode) {
    case "US", "USA":
        return NewUSPhone(number)
    default:
        return Phone{}, fmt.Errorf("unsupported country code: %s", countryCode)
    }
}

// E164 returns the phone number in E.164 format
func (p Phone) E164() string {
    return p.number
}

// Formatted returns the human-readable formatted number
func (p Phone) Formatted() string {
    return p.formatted
}

// CountryCode returns the country code
func (p Phone) CountryCode() string {
    return p.countryCode
}

// String implements fmt.Stringer
func (p Phone) String() string {
    return p.formatted
}

// Equal implements equality comparison
func (p Phone) Equal(other Phone) bool {
    return p.number == other.number
}

// IsEmpty returns true if the phone number is empty
func (p Phone) IsEmpty() bool {
    return p.number == ""
}

// AreaCode returns the area code for US numbers
func (p Phone) AreaCode() string {
    if p.countryCode != "US" || len(p.number) != 12 { // +1 + 10 digits
        return ""
    }
    return p.number[2:5] // Skip +1, take next 3 digits
}

// JSON Marshaling
func (p Phone) MarshalJSON() ([]byte, error) {
    return json.Marshal(p.number) // Store as E.164
}

func (p *Phone) UnmarshalJSON(data []byte) error {
    var number string
    if err := json.Unmarshal(data, &number); err != nil {
        return err
    }
    
    if number == "" {
        *p = Phone{}
        return nil
    }
    
    // Assume US for now, could be extended
    phone, err := NewUSPhone(number)
    if err != nil {
        return err
    }
    
    *p = phone
    return nil
}

// Database Scanning
func (p *Phone) Scan(value interface{}) error {
    if value == nil {
        *p = Phone{}
        return nil
    }
    
    var number string
    switch v := value.(type) {
    case string:
        number = v
    case []byte:
        number = string(v)
    default:
        return fmt.Errorf("cannot scan %T into Phone", value)
    }
    
    if number == "" {
        *p = Phone{}
        return nil
    }
    
    phone, err := NewUSPhone(number)
    if err != nil {
        return err
    }
    
    *p = phone
    return nil
}

func (p Phone) Value() (driver.Value, error) {
    if p.number == "" {
        return nil, nil
    }
    return p.number, nil // Store E.164 format in database
}
```

## JSON Marshaling/Unmarshaling Patterns

### Basic Pattern for Value Objects

```go
// MarshalJSON must be a value receiver for correct behavior
func (v ValueObject) MarshalJSON() ([]byte, error) {
    return json.Marshal(v.internalValue)
}

// UnmarshalJSON must be a pointer receiver to modify the object
func (v *ValueObject) UnmarshalJSON(data []byte) error {
    var raw string
    if err := json.Unmarshal(data, &raw); err != nil {
        return err
    }
    
    parsed, err := NewValueObject(raw)
    if err != nil {
        return err
    }
    
    *v = parsed
    return nil
}
```

### Custom JSON Tags and Formatting

```go
type UserAccount struct {
    ID       string `json:"id"`
    Email    Email  `json:"email"`
    Phone    Phone  `json:"phone,omitempty"`
    Balance  Money  `json:"balance"`
    Created  time.Time `json:"created_at"`
}

// Example with custom JSON handling
func (ua UserAccount) MarshalJSON() ([]byte, error) {
    type Alias UserAccount
    return json.Marshal(&struct {
        *Alias
        BalanceFormatted string `json:"balance_formatted"`
    }{
        Alias:            (*Alias)(&ua),
        BalanceFormatted: ua.Balance.String(),
    })
}
```

## Database Scanning Patterns

### Scanner and Valuer Interface Implementation

```go
import (
    "database/sql"
    "database/sql/driver"
)

// Valuer interface for writing to database
func (v ValueObject) Value() (driver.Value, error) {
    if v.IsEmpty() {
        return nil, nil
    }
    return v.internalValue, nil
}

// Scanner interface for reading from database
func (v *ValueObject) Scan(value interface{}) error {
    if value == nil {
        *v = ValueObject{} // Zero value
        return nil
    }
    
    var rawValue string
    switch val := value.(type) {
    case string:
        rawValue = val
    case []byte:
        rawValue = string(val)
    default:
        return fmt.Errorf("cannot scan %T into ValueObject", value)
    }
    
    parsed, err := NewValueObject(rawValue)
    if err != nil {
        return fmt.Errorf("failed to scan ValueObject: %w", err)
    }
    
    *v = parsed
    return nil
}
```

### Nullable Value Objects

```go
// NullableEmail wraps Email to handle NULL database values
type NullableEmail struct {
    Email Email
    Valid bool
}

func (ne *NullableEmail) Scan(value interface{}) error {
    if value == nil {
        ne.Email = Email{}
        ne.Valid = false
        return nil
    }
    
    if err := ne.Email.Scan(value); err != nil {
        return err
    }
    
    ne.Valid = !ne.Email.IsEmpty()
    return nil
}

func (ne NullableEmail) Value() (driver.Value, error) {
    if !ne.Valid {
        return nil, nil
    }
    return ne.Email.Value()
}
```

## Comparison and Equality Methods

### Basic Equality Pattern

```go
// Equal method for value comparison
func (v ValueObject) Equal(other ValueObject) bool {
    return v.internalValue == other.internalValue
}

// Equals method for interface compatibility
func (v ValueObject) Equals(other interface{}) bool {
    if o, ok := other.(ValueObject); ok {
        return v.Equal(o)
    }
    return false
}
```

### Comparable Interface Implementation

```go
// Comparable interface for sortable value objects
type Comparable interface {
    Compare(other Comparable) int
}

func (m Money) Compare(other Money) int {
    if m.currency != other.currency {
        panic("cannot compare different currencies")
    }
    
    return m.amount.Cmp(other.amount)
}

// Less returns true if this money is less than other
func (m Money) Less(other Money) bool {
    return m.Compare(other) < 0
}

// Greater returns true if this money is greater than other
func (m Money) Greater(other Money) bool {
    return m.Compare(other) > 0
}
```

### Hash Code for Maps

```go
import "hash/fnv"

// Hash returns a hash code for use in maps
func (v ValueObject) Hash() uint64 {
    h := fnv.New64a()
    h.Write([]byte(v.internalValue))
    return h.Sum64()
}

// Example usage in maps
type ValueObjectMap map[ValueObject]string

func (vom ValueObjectMap) Get(key ValueObject) (string, bool) {
    val, exists := vom[key]
    return val, exists
}
```

## Immutability Patterns in Go

### Constructor Pattern

```go
// Factory function ensures valid state and immutability
func NewValueObject(value string) (ValueObject, error) {
    if err := validate(value); err != nil {
        return ValueObject{}, err
    }
    
    return ValueObject{
        internalValue: value,
        computedField: computeField(value),
    }, nil
}

// MustNewValueObject for cases where you're certain the value is valid
func MustNewValueObject(value string) ValueObject {
    vo, err := NewValueObject(value)
    if err != nil {
        panic(fmt.Sprintf("invalid value object: %v", err))
    }
    return vo
}
```

### Defensive Copying for Slices and Maps

```go
type ValueObjectList struct {
    items []string // Internal slice - don't expose directly
}

func NewValueObjectList(items []string) ValueObjectList {
    // Defensive copy to prevent external modification
    copied := make([]string, len(items))
    copy(copied, items)
    
    return ValueObjectList{
        items: copied,
    }
}

// Items returns a copy of the internal slice
func (vol ValueObjectList) Items() []string {
    result := make([]string, len(vol.items))
    copy(result, vol.items)
    return result
}

// Add returns a new ValueObjectList with the item added
func (vol ValueObjectList) Add(item string) ValueObjectList {
    newItems := make([]string, len(vol.items)+1)
    copy(newItems, vol.items)
    newItems[len(vol.items)] = item
    
    return ValueObjectList{
        items: newItems,
    }
}
```

### Functional Style Operations

```go
// Map applies a function to create a new value object
func (vol ValueObjectList) Map(fn func(string) string) ValueObjectList {
    newItems := make([]string, len(vol.items))
    for i, item := range vol.items {
        newItems[i] = fn(item)
    }
    
    return ValueObjectList{
        items: newItems,
    }
}

// Filter returns a new list with items matching the predicate
func (vol ValueObjectList) Filter(predicate func(string) bool) ValueObjectList {
    var newItems []string
    for _, item := range vol.items {
        if predicate(item) {
            newItems = append(newItems, item)
        }
    }
    
    return ValueObjectList{
        items: newItems,
    }
}
```

## Error Handling in Value Object Constructors

### Error Types and Patterns

```go
// Custom error types for better error handling
type ValidationError struct {
    Field   string
    Value   string
    Message string
}

func (ve ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s=%q: %s", ve.Field, ve.Value, ve.Message)
}

// Constructor with detailed error information
func NewEmail(address string) (Email, error) {
    address = strings.TrimSpace(strings.ToLower(address))
    
    if address == "" {
        return Email{}, ValidationError{
            Field:   "email",
            Value:   address,
            Message: "cannot be empty",
        }
    }
    
    if len(address) > 254 {
        return Email{}, ValidationError{
            Field:   "email",
            Value:   address,
            Message: "exceeds maximum length of 254 characters",
        }
    }
    
    if !emailRegex.MatchString(address) {
        return Email{}, ValidationError{
            Field:   "email",
            Value:   address,
            Message: "invalid format",
        }
    }
    
    return Email{address: address}, nil
}
```

### Multiple Validation Errors

```go
import "errors"

// MultiError collects multiple validation errors
type MultiError struct {
    Errors []error
}

func (me MultiError) Error() string {
    if len(me.Errors) == 0 {
        return ""
    }
    
    if len(me.Errors) == 1 {
        return me.Errors[0].Error()
    }
    
    var messages []string
    for _, err := range me.Errors {
        messages = append(messages, err.Error())
    }
    
    return fmt.Sprintf("multiple errors: [%s]", strings.Join(messages, ", "))
}

func (me *MultiError) Add(err error) {
    if err != nil {
        me.Errors = append(me.Errors, err)
    }
}

func (me MultiError) HasErrors() bool {
    return len(me.Errors) > 0
}

// Constructor with multiple validations
func NewUserProfile(email, phone string) (UserProfile, error) {
    var errs MultiError
    
    emailObj, err := NewEmail(email)
    if err != nil {
        errs.Add(fmt.Errorf("email: %w", err))
    }
    
    phoneObj, err := NewPhone(phone, "US")
    if err != nil {
        errs.Add(fmt.Errorf("phone: %w", err))
    }
    
    if errs.HasErrors() {
        return UserProfile{}, errs
    }
    
    return UserProfile{
        Email: emailObj,
        Phone: phoneObj,
    }, nil
}
```

### Panic vs Error Guidelines

```go
// Use errors for expected validation failures
func NewEmail(address string) (Email, error) {
    if address == "" {
        return Email{}, errors.New("email cannot be empty")
    }
    // ... validation logic
    return Email{address: address}, nil
}

// Use panic only for programming errors or impossible states
func MustNewEmail(address string) Email {
    email, err := NewEmail(address)
    if err != nil {
        panic(fmt.Sprintf("MustNewEmail: %v", err))
    }
    return email
}

// Use Must* functions sparingly, mainly for:
// 1. Constants/literals known to be valid
// 2. Test data
// 3. Configuration where failure should stop the program
var AdminEmail = MustNewEmail("admin@example.com")
```

## Complete Examples

### Complete Money Implementation

```go
package money

import (
    "database/sql/driver"
    "encoding/json"
    "fmt"
    "strconv"
    "strings"
    
    "github.com/govalues/decimal"
)

type Currency string

const (
    USD Currency = "USD"
    EUR Currency = "EUR"
    GBP Currency = "GBP"
)

type Money struct {
    amount   decimal.Decimal
    currency Currency
}

func NewMoney(amount string, currency Currency) (Money, error) {
    if currency == "" {
        return Money{}, fmt.Errorf("currency cannot be empty")
    }
    
    dec, err := decimal.Parse(amount)
    if err != nil {
        return Money{}, fmt.Errorf("invalid amount %q: %w", amount, err)
    }
    
    return Money{
        amount:   dec,
        currency: currency,
    }, nil
}

func NewMoneyFromCents(cents int64, currency Currency) (Money, error) {
    if currency == "" {
        return Money{}, fmt.Errorf("currency cannot be empty")
    }
    
    dec := decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
    return Money{
        amount:   dec,
        currency: currency,
    }, nil
}

func (m Money) Amount() decimal.Decimal { return m.amount }
func (m Money) Currency() Currency     { return m.currency }
func (m Money) IsZero() bool           { return m.amount.IsZero() }
func (m Money) IsPositive() bool       { return m.amount.IsPos() }
func (m Money) IsNegative() bool       { return m.amount.IsNeg() }

func (m Money) String() string {
    return fmt.Sprintf("%s %s", m.amount.String(), m.currency)
}

func (m Money) Equal(other Money) bool {
    return m.amount.Equal(other.amount) && m.currency == other.currency
}

func (m Money) Add(other Money) (Money, error) {
    if m.currency != other.currency {
        return Money{}, fmt.Errorf("cannot add %s and %s", m.currency, other.currency)
    }
    return Money{amount: m.amount.Add(other.amount), currency: m.currency}, nil
}

func (m Money) Subtract(other Money) (Money, error) {
    if m.currency != other.currency {
        return Money{}, fmt.Errorf("cannot subtract %s and %s", m.currency, other.currency)
    }
    return Money{amount: m.amount.Sub(other.amount), currency: m.currency}, nil
}

func (m Money) Multiply(factor decimal.Decimal) Money {
    return Money{amount: m.amount.Mul(factor), currency: m.currency}
}

func (m Money) Divide(divisor decimal.Decimal) (Money, error) {
    if divisor.IsZero() {
        return Money{}, fmt.Errorf("cannot divide by zero")
    }
    return Money{amount: m.amount.Div(divisor), currency: m.currency}, nil
}

// JSON marshaling
func (m Money) MarshalJSON() ([]byte, error) {
    data := struct {
        Amount   string   `json:"amount"`
        Currency Currency `json:"currency"`
    }{
        Amount:   m.amount.String(),
        Currency: m.currency,
    }
    return json.Marshal(data)
}

func (m *Money) UnmarshalJSON(data []byte) error {
    var temp struct {
        Amount   string   `json:"amount"`
        Currency Currency `json:"currency"`
    }
    
    if err := json.Unmarshal(data, &temp); err != nil {
        return err
    }
    
    money, err := NewMoney(temp.Amount, temp.Currency)
    if err != nil {
        return err
    }
    
    *m = money
    return nil
}

// Database scanning
func (m *Money) Scan(value interface{}) error {
    if value == nil {
        *m = Money{}
        return nil
    }
    
    switch v := value.(type) {
    case string:
        return m.scanFromString(v)
    case []byte:
        return m.scanFromString(string(v))
    default:
        return fmt.Errorf("cannot scan %T into Money", value)
    }
}

func (m *Money) scanFromString(s string) error {
    parts := strings.Fields(s)
    if len(parts) != 2 {
        return fmt.Errorf("invalid money format: %q", s)
    }
    
    money, err := NewMoney(parts[0], Currency(parts[1]))
    if err != nil {
        return err
    }
    
    *m = money
    return nil
}

func (m Money) Value() (driver.Value, error) {
    if m.amount.IsZero() && m.currency == "" {
        return nil, nil
    }
    return fmt.Sprintf("%s %s", m.amount.String(), m.currency), nil
}

// Cents returns the amount in cents (for integer storage)
func (m Money) Cents() int64 {
    return m.amount.Mul(decimal.NewFromInt(100)).IntPart()
}

// FormatForDisplay formats the money for user display
func (m Money) FormatForDisplay() string {
    switch m.currency {
    case USD:
        return fmt.Sprintf("$%s", m.amount.StringFixed(2))
    case EUR:
        return fmt.Sprintf("€%s", m.amount.StringFixed(2))
    case GBP:
        return fmt.Sprintf("£%s", m.amount.StringFixed(2))
    default:
        return m.String()
    }
}
```

### Complete User Profile Example

```go
package user

import (
    "encoding/json"
    "time"
)

type UserProfile struct {
    ID        string    `json:"id"`
    Email     Email     `json:"email"`
    Phone     Phone     `json:"phone,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func NewUserProfile(id string, email Email, phone Phone) UserProfile {
    now := time.Now()
    return UserProfile{
        ID:        id,
        Email:     email,
        Phone:     phone,
        CreatedAt: now,
        UpdatedAt: now,
    }
}

func (up UserProfile) WithEmail(email Email) UserProfile {
    up.Email = email
    up.UpdatedAt = time.Now()
    return up
}

func (up UserProfile) WithPhone(phone Phone) UserProfile {
    up.Phone = phone
    up.UpdatedAt = time.Now()
    return up
}

func (up UserProfile) Equal(other UserProfile) bool {
    return up.ID == other.ID &&
        up.Email.Equal(other.Email) &&
        up.Phone.Equal(other.Phone)
}

// Example usage
func ExampleUsage() {
    // Create value objects
    email, err := NewEmail("user@example.com")
    if err != nil {
        panic(err)
    }
    
    phone, err := NewUSPhone("(555) 123-4567")
    if err != nil {
        panic(err)
    }
    
    money, err := NewMoney("100.50", USD)
    if err != nil {
        panic(err)
    }
    
    // Create user profile
    profile := NewUserProfile("user-123", email, phone)
    
    // JSON marshaling
    jsonData, err := json.Marshal(profile)
    if err != nil {
        panic(err)
    }
    
    // JSON unmarshaling
    var newProfile UserProfile
    if err := json.Unmarshal(jsonData, &newProfile); err != nil {
        panic(err)
    }
    
    // Database operations would use Scan/Value methods automatically
    fmt.Printf("Profile: %+v\n", profile)
    fmt.Printf("Money: %s\n", money.FormatForDisplay())
}
```

## Summary

Value objects in Go provide type safety, encapsulation, and immutability when implemented correctly. Key takeaways:

1. **Use factory functions** with validation for construction
2. **Implement JSON marshaling** with proper error handling
3. **Implement database interfaces** (Scanner/Valuer) for persistence
4. **Provide equality methods** for comparison
5. **Maintain immutability** through defensive copying and no setters
6. **Handle errors gracefully** in constructors and operations
7. **Use appropriate patterns** for your specific domain needs

These patterns ensure robust, maintainable code that properly encapsulates domain logic while integrating seamlessly with Go's ecosystem.