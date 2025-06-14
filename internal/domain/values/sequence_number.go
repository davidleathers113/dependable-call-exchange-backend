package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// SequenceNumber represents a monotonic sequence number for audit events
type SequenceNumber struct {
	value uint64
}

const (
	// Maximum sequence number value (2^63 - 1 for safe database storage)
	MaxSequenceNumber = uint64(9223372036854775807)
	// Minimum sequence number value
	MinSequenceNumber = uint64(1)
)

// NewSequenceNumber creates a new SequenceNumber value object with validation
func NewSequenceNumber(value uint64) (SequenceNumber, error) {
	if value == 0 {
		return SequenceNumber{}, errors.NewValidationError("ZERO_SEQUENCE", 
			"sequence number cannot be zero")
	}

	if value > MaxSequenceNumber {
		return SequenceNumber{}, errors.NewValidationError("SEQUENCE_TOO_LARGE", 
			fmt.Sprintf("sequence number %d exceeds maximum %d", value, MaxSequenceNumber))
	}

	return SequenceNumber{value: value}, nil
}

// NewSequenceNumberFromString creates SequenceNumber from string representation
func NewSequenceNumberFromString(value string) (SequenceNumber, error) {
	if value == "" {
		return SequenceNumber{}, errors.NewValidationError("EMPTY_SEQUENCE", 
			"sequence number string cannot be empty")
	}

	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return SequenceNumber{}, errors.NewValidationError("INVALID_SEQUENCE_FORMAT", 
			"sequence number must be a valid positive integer").WithCause(err)
	}

	return NewSequenceNumber(parsed)
}

// MustNewSequenceNumber creates SequenceNumber and panics on error (for constants/tests)
func MustNewSequenceNumber(value uint64) SequenceNumber {
	seq, err := NewSequenceNumber(value)
	if err != nil {
		panic(err)
	}
	return seq
}

// First returns the first sequence number (1)
func FirstSequenceNumber() SequenceNumber {
	return MustNewSequenceNumber(MinSequenceNumber)
}

// Value returns the sequence number value
func (s SequenceNumber) Value() uint64 {
	return s.value
}

// String returns the string representation of the sequence number
func (s SequenceNumber) String() string {
	return strconv.FormatUint(s.value, 10)
}

// IsZero checks if the sequence number is zero (invalid state)
func (s SequenceNumber) IsZero() bool {
	return s.value == 0
}

// IsFirst checks if this is the first sequence number
func (s SequenceNumber) IsFirst() bool {
	return s.value == MinSequenceNumber
}

// Equal checks if two SequenceNumber values are equal
func (s SequenceNumber) Equal(other SequenceNumber) bool {
	return s.value == other.value
}

// Compare returns -1, 0, or 1 based on comparison with other SequenceNumber
func (s SequenceNumber) Compare(other SequenceNumber) int {
	if s.value < other.value {
		return -1
	}
	if s.value > other.value {
		return 1
	}
	return 0
}

// LessThan checks if this sequence number is less than other
func (s SequenceNumber) LessThan(other SequenceNumber) bool {
	return s.value < other.value
}

// LessThanOrEqual checks if this sequence number is less than or equal to other
func (s SequenceNumber) LessThanOrEqual(other SequenceNumber) bool {
	return s.value <= other.value
}

// GreaterThan checks if this sequence number is greater than other
func (s SequenceNumber) GreaterThan(other SequenceNumber) bool {
	return s.value > other.value
}

// GreaterThanOrEqual checks if this sequence number is greater than or equal to other
func (s SequenceNumber) GreaterThanOrEqual(other SequenceNumber) bool {
	return s.value >= other.value
}

// Next returns the next sequence number
func (s SequenceNumber) Next() (SequenceNumber, error) {
	if s.value >= MaxSequenceNumber {
		return SequenceNumber{}, errors.NewValidationError("SEQUENCE_OVERFLOW", 
			"sequence number would overflow maximum value")
	}

	return SequenceNumber{value: s.value + 1}, nil
}

// Previous returns the previous sequence number
func (s SequenceNumber) Previous() (SequenceNumber, error) {
	if s.value <= MinSequenceNumber {
		return SequenceNumber{}, errors.NewValidationError("SEQUENCE_UNDERFLOW", 
			"sequence number would underflow minimum value")
	}

	return SequenceNumber{value: s.value - 1}, nil
}

// Add adds a delta to the sequence number
func (s SequenceNumber) Add(delta uint64) (SequenceNumber, error) {
	if delta == 0 {
		return s, nil
	}

	if s.value > MaxSequenceNumber-delta {
		return SequenceNumber{}, errors.NewValidationError("SEQUENCE_OVERFLOW", 
			fmt.Sprintf("adding %d to %d would overflow maximum", delta, s.value))
	}

	return SequenceNumber{value: s.value + delta}, nil
}

// Subtract subtracts a delta from the sequence number
func (s SequenceNumber) Subtract(delta uint64) (SequenceNumber, error) {
	if delta == 0 {
		return s, nil
	}

	if s.value < delta || s.value-delta < MinSequenceNumber {
		return SequenceNumber{}, errors.NewValidationError("SEQUENCE_UNDERFLOW", 
			fmt.Sprintf("subtracting %d from %d would underflow minimum", delta, s.value))
	}

	return SequenceNumber{value: s.value - delta}, nil
}

// Distance calculates the distance between two sequence numbers
func (s SequenceNumber) Distance(other SequenceNumber) uint64 {
	if s.value >= other.value {
		return s.value - other.value
	}
	return other.value - s.value
}

// InRange checks if the sequence number is within the given range (inclusive)
func (s SequenceNumber) InRange(min, max SequenceNumber) bool {
	return s.value >= min.value && s.value <= max.value
}

// Format returns a formatted string for display
func (s SequenceNumber) Format() string {
	if s.IsZero() {
		return "<invalid>"
	}
	return fmt.Sprintf("seq:%d", s.value)
}

// FormatWithPrefix returns a formatted string with custom prefix
func (s SequenceNumber) FormatWithPrefix(prefix string) string {
	if s.IsZero() {
		return fmt.Sprintf("%s:<invalid>", prefix)
	}
	return fmt.Sprintf("%s:%d", prefix, s.value)
}

// MarshalJSON implements JSON marshaling
func (s SequenceNumber) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.value)
}

// UnmarshalJSON implements JSON unmarshaling
func (s *SequenceNumber) UnmarshalJSON(data []byte) error {
	var value uint64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	seq, err := NewSequenceNumber(value)
	if err != nil {
		return err
	}

	*s = seq
	return nil
}

// DatabaseValue implements driver.Valuer for database storage
func (s SequenceNumber) DatabaseValue() (driver.Value, error) {
	if s.value == 0 {
		return nil, nil
	}
	return int64(s.value), nil
}

// Scan implements sql.Scanner for database retrieval
func (s *SequenceNumber) Scan(value interface{}) error {
	if value == nil {
		*s = SequenceNumber{}
		return nil
	}

	var val uint64
	switch v := value.(type) {
	case int64:
		if v < 0 {
			return fmt.Errorf("sequence number cannot be negative: %d", v)
		}
		val = uint64(v)
	case uint64:
		val = v
	case int:
		if v < 0 {
			return fmt.Errorf("sequence number cannot be negative: %d", v)
		}
		val = uint64(v)
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse sequence number string '%s': %w", v, err)
		}
		val = parsed
	default:
		return fmt.Errorf("cannot scan %T into SequenceNumber", value)
	}

	if val == 0 {
		*s = SequenceNumber{}
		return nil
	}

	seq, err := NewSequenceNumber(val)
	if err != nil {
		return err
	}

	*s = seq
	return nil
}

// SequenceGenerator provides thread-safe sequence number generation
type SequenceGenerator struct {
	current uint64
	mutex   sync.Mutex
}

// NewSequenceGenerator creates a new sequence generator starting from the given value
func NewSequenceGenerator(start uint64) (*SequenceGenerator, error) {
	if start == 0 {
		start = MinSequenceNumber
	}

	if start > MaxSequenceNumber {
		return nil, errors.NewValidationError("INVALID_START_SEQUENCE", 
			"start sequence number exceeds maximum")
	}

	return &SequenceGenerator{
		current: start - 1, // Subtract 1 so first Next() call returns start
	}, nil
}

// Next generates the next sequence number (thread-safe)
func (sg *SequenceGenerator) Next() (SequenceNumber, error) {
	sg.mutex.Lock()
	defer sg.mutex.Unlock()

	if sg.current >= MaxSequenceNumber {
		return SequenceNumber{}, errors.NewValidationError("SEQUENCE_EXHAUSTED", 
			"sequence generator has reached maximum value")
	}

	sg.current++
	return SequenceNumber{value: sg.current}, nil
}

// Current returns the current sequence number (thread-safe)
func (sg *SequenceGenerator) Current() SequenceNumber {
	sg.mutex.Lock()
	defer sg.mutex.Unlock()

	if sg.current == 0 {
		return SequenceNumber{} // Invalid/uninitialized state
	}
	return SequenceNumber{value: sg.current}
}

// Reset resets the generator to the given start value (thread-safe)
func (sg *SequenceGenerator) Reset(start uint64) error {
	if start == 0 {
		start = MinSequenceNumber
	}

	if start > MaxSequenceNumber {
		return errors.NewValidationError("INVALID_START_SEQUENCE", 
			"start sequence number exceeds maximum")
	}

	sg.mutex.Lock()
	defer sg.mutex.Unlock()

	sg.current = start - 1
	return nil
}

// Range represents a range of sequence numbers
type SequenceRange struct {
	Start SequenceNumber
	End   SequenceNumber
}

// NewSequenceRange creates a new sequence range
func NewSequenceRange(start, end SequenceNumber) (*SequenceRange, error) {
	if start.IsZero() || end.IsZero() {
		return nil, errors.NewValidationError("INVALID_RANGE", 
			"range boundaries cannot be zero")
	}

	if start.GreaterThan(end) {
		return nil, errors.NewValidationError("INVALID_RANGE", 
			"start sequence must be less than or equal to end sequence")
	}

	return &SequenceRange{
		Start: start,
		End:   end,
	}, nil
}

// Contains checks if the range contains the given sequence number
func (sr *SequenceRange) Contains(seq SequenceNumber) bool {
	return seq.GreaterThanOrEqual(sr.Start) && seq.LessThanOrEqual(sr.End)
}

// Size returns the number of sequence numbers in the range
func (sr *SequenceRange) Size() uint64 {
	return sr.End.value - sr.Start.value + 1
}

// IsEmpty checks if the range is empty
func (sr *SequenceRange) IsEmpty() bool {
	return sr.Start.GreaterThan(sr.End)
}

// String returns a string representation of the range
func (sr *SequenceRange) String() string {
	return fmt.Sprintf("[%s-%s]", sr.Start.String(), sr.End.String())
}

// ValidationError represents validation errors for sequence numbers
type SequenceValidationError struct {
	Value  uint64
	Reason string
}

func (e SequenceValidationError) Error() string {
	return fmt.Sprintf("invalid sequence number %d: %s", e.Value, e.Reason)
}

// ValidateSequenceNumber validates that a uint64 could be a valid sequence number
func ValidateSequenceNumber(value uint64) error {
	if value == 0 {
		return errors.NewValidationError("ZERO_SEQUENCE", "sequence number cannot be zero")
	}

	if value > MaxSequenceNumber {
		return errors.NewValidationError("SEQUENCE_TOO_LARGE", 
			fmt.Sprintf("sequence number %d exceeds maximum %d", value, MaxSequenceNumber))
	}

	return nil
}