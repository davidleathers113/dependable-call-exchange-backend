package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// RetentionPeriod represents a time-based retention period for audit compliance
type RetentionPeriod struct {
	duration time.Duration
	years    int // Cached years for compliance reporting
}

const (
	// Minimum retention period (7 years for most compliance requirements)
	MinRetentionYears = 7
	MaxRetentionYears = 100

	// Standard retention periods
	RetentionYear  = 365 * 24 * time.Hour
	Retention7Year = 7 * RetentionYear
	Retention10Year = 10 * RetentionYear
)

// NewRetentionPeriod creates a new RetentionPeriod value object with validation
func NewRetentionPeriod(duration time.Duration) (RetentionPeriod, error) {
	if duration <= 0 {
		return RetentionPeriod{}, errors.NewValidationError("INVALID_RETENTION_DURATION", 
			"retention period must be positive")
	}

	// Convert to years for validation
	years := int(duration.Hours() / (24 * 365))
	
	if years < MinRetentionYears {
		return RetentionPeriod{}, errors.NewValidationError("RETENTION_TOO_SHORT", 
			fmt.Sprintf("retention period must be at least %d years for compliance", MinRetentionYears))
	}

	if years > MaxRetentionYears {
		return RetentionPeriod{}, errors.NewValidationError("RETENTION_TOO_LONG", 
			fmt.Sprintf("retention period cannot exceed %d years", MaxRetentionYears))
	}

	return RetentionPeriod{
		duration: duration,
		years:    years,
	}, nil
}

// NewRetentionPeriodFromYears creates RetentionPeriod from number of years
func NewRetentionPeriodFromYears(years int) (RetentionPeriod, error) {
	if years < MinRetentionYears {
		return RetentionPeriod{}, errors.NewValidationError("RETENTION_TOO_SHORT", 
			fmt.Sprintf("retention period must be at least %d years", MinRetentionYears))
	}

	if years > MaxRetentionYears {
		return RetentionPeriod{}, errors.NewValidationError("RETENTION_TOO_LONG", 
			fmt.Sprintf("retention period cannot exceed %d years", MaxRetentionYears))
	}

	duration := time.Duration(years) * RetentionYear
	return RetentionPeriod{
		duration: duration,
		years:    years,
	}, nil
}

// NewRetentionPeriodFromString creates RetentionPeriod from string representation
func NewRetentionPeriodFromString(value string) (RetentionPeriod, error) {
	if value == "" {
		return RetentionPeriod{}, errors.NewValidationError("EMPTY_RETENTION", 
			"retention period string cannot be empty")
	}

	value = strings.TrimSpace(strings.ToLower(value))

	// Handle special cases
	switch value {
	case "minimum", "min":
		return NewRetentionPeriodFromYears(MinRetentionYears)
	case "standard":
		return NewRetentionPeriodFromYears(7)
	case "extended":
		return NewRetentionPeriodFromYears(10)
	case "permanent", "forever":
		return NewRetentionPeriodFromYears(MaxRetentionYears)
	}

	// Try parsing as duration
	if duration, err := time.ParseDuration(value); err == nil {
		return NewRetentionPeriod(duration)
	}

	// Try parsing as years
	if strings.HasSuffix(value, "y") || strings.HasSuffix(value, "year") || strings.HasSuffix(value, "years") {
		yearStr := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(value, "years"), "year"), "y")
		if years, err := strconv.Atoi(strings.TrimSpace(yearStr)); err == nil {
			return NewRetentionPeriodFromYears(years)
		}
	}

	return RetentionPeriod{}, errors.NewValidationError("INVALID_RETENTION_FORMAT", 
		"retention period must be a valid duration or number of years")
}

// MustNewRetentionPeriod creates RetentionPeriod and panics on error (for constants/tests)
func MustNewRetentionPeriod(duration time.Duration) RetentionPeriod {
	rp, err := NewRetentionPeriod(duration)
	if err != nil {
		panic(err)
	}
	return rp
}

// MustNewRetentionPeriodFromYears creates RetentionPeriod from years and panics on error
func MustNewRetentionPeriodFromYears(years int) RetentionPeriod {
	rp, err := NewRetentionPeriodFromYears(years)
	if err != nil {
		panic(err)
	}
	return rp
}

// Standard retention periods
func StandardRetention() RetentionPeriod {
	return MustNewRetentionPeriodFromYears(7)
}

func ExtendedRetention() RetentionPeriod {
	return MustNewRetentionPeriodFromYears(10)
}

func MinimumRetention() RetentionPeriod {
	return MustNewRetentionPeriodFromYears(MinRetentionYears)
}

// Duration returns the retention duration
func (rp RetentionPeriod) Duration() time.Duration {
	return rp.duration
}

// Years returns the retention period in years
func (rp RetentionPeriod) Years() int {
	return rp.years
}

// String returns a human-readable string representation
func (rp RetentionPeriod) String() string {
	if rp.years == 1 {
		return "1 year"
	}
	return fmt.Sprintf("%d years", rp.years)
}

// IsZero checks if the retention period is zero (invalid state)
func (rp RetentionPeriod) IsZero() bool {
	return rp.duration == 0
}

// Equal checks if two RetentionPeriod values are equal
func (rp RetentionPeriod) Equal(other RetentionPeriod) bool {
	return rp.duration == other.duration
}

// Compare returns -1, 0, or 1 based on comparison with other RetentionPeriod
func (rp RetentionPeriod) Compare(other RetentionPeriod) int {
	if rp.duration < other.duration {
		return -1
	}
	if rp.duration > other.duration {
		return 1
	}
	return 0
}

// LessThan checks if this retention period is less than other
func (rp RetentionPeriod) LessThan(other RetentionPeriod) bool {
	return rp.duration < other.duration
}

// GreaterThan checks if this retention period is greater than other
func (rp RetentionPeriod) GreaterThan(other RetentionPeriod) bool {
	return rp.duration > other.duration
}

// IsMinimum checks if this is the minimum retention period
func (rp RetentionPeriod) IsMinimum() bool {
	return rp.years == MinRetentionYears
}

// IsStandard checks if this is the standard retention period (7 years)
func (rp RetentionPeriod) IsStandard() bool {
	return rp.years == 7
}

// IsExtended checks if this is the extended retention period (10 years)
func (rp RetentionPeriod) IsExtended() bool {
	return rp.years == 10
}

// CalculateExpirationDate calculates when data created at the given time should expire
func (rp RetentionPeriod) CalculateExpirationDate(createdAt time.Time) time.Time {
	return createdAt.Add(rp.duration)
}

// IsExpired checks if data created at the given time has expired
func (rp RetentionPeriod) IsExpired(createdAt time.Time) bool {
	return time.Now().After(rp.CalculateExpirationDate(createdAt))
}

// TimeUntilExpiration returns how much time is left until expiration
func (rp RetentionPeriod) TimeUntilExpiration(createdAt time.Time) time.Duration {
	expiration := rp.CalculateExpirationDate(createdAt)
	remaining := time.Until(expiration)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// TimeSinceExpiration returns how much time has passed since expiration
func (rp RetentionPeriod) TimeSinceExpiration(createdAt time.Time) time.Duration {
	expiration := rp.CalculateExpirationDate(createdAt)
	elapsed := time.Since(expiration)
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// IsCompliant checks if the retention period meets compliance requirements for the given jurisdiction
func (rp RetentionPeriod) IsCompliant(jurisdiction string) bool {
	switch strings.ToUpper(jurisdiction) {
	case "US", "USA", "UNITED STATES":
		// US federal requirements (Sarbanes-Oxley, etc.)
		return rp.years >= 7
	case "EU", "EUROPE", "GDPR":
		// GDPR and EU requirements
		return rp.years >= 7
	case "UK", "UNITED KINGDOM":
		// UK requirements
		return rp.years >= 7
	case "CA", "CANADA":
		// Canadian requirements
		return rp.years >= 7
	default:
		// Default to minimum requirement
		return rp.years >= MinRetentionYears
	}
}

// GetComplianceNote returns a note about compliance for the retention period
func (rp RetentionPeriod) GetComplianceNote() string {
	switch {
	case rp.years < MinRetentionYears:
		return "Below minimum compliance requirements"
	case rp.years == MinRetentionYears:
		return "Meets minimum compliance requirements"
	case rp.years >= 10:
		return "Exceeds standard compliance requirements"
	default:
		return "Meets standard compliance requirements"
	}
}

// Format returns a formatted string for display
func (rp RetentionPeriod) Format() string {
	if rp.IsZero() {
		return "<invalid>"
	}
	return fmt.Sprintf("retention:%s", rp.String())
}

// FormatWithCompliance returns formatted string with compliance info
func (rp RetentionPeriod) FormatWithCompliance() string {
	if rp.IsZero() {
		return "<invalid>"
	}
	return fmt.Sprintf("retention:%s (%s)", rp.String(), rp.GetComplianceNote())
}

// MarshalJSON implements JSON marshaling
func (rp RetentionPeriod) MarshalJSON() ([]byte, error) {
	data := struct {
		Years    int    `json:"years"`
		Duration string `json:"duration"`
	}{
		Years:    rp.years,
		Duration: rp.duration.String(),
	}
	return json.Marshal(data)
}

// UnmarshalJSON implements JSON unmarshaling
func (rp *RetentionPeriod) UnmarshalJSON(data []byte) error {
	var temp struct {
		Years    int    `json:"years"`
		Duration string `json:"duration"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Prefer years if provided, otherwise use duration
	if temp.Years > 0 {
		retention, err := NewRetentionPeriodFromYears(temp.Years)
		if err != nil {
			return err
		}
		*rp = retention
		return nil
	}

	if temp.Duration != "" {
		duration, err := time.ParseDuration(temp.Duration)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}

		retention, err := NewRetentionPeriod(duration)
		if err != nil {
			return err
		}
		*rp = retention
		return nil
	}

	return errors.NewValidationError("MISSING_RETENTION_DATA", 
		"either years or duration must be specified")
}

// Value implements driver.Valuer for database storage
func (rp RetentionPeriod) Value() (driver.Value, error) {
	if rp.duration == 0 {
		return nil, nil
	}
	return rp.years, nil
}

// Scan implements sql.Scanner for database retrieval
func (rp *RetentionPeriod) Scan(value interface{}) error {
	if value == nil {
		*rp = RetentionPeriod{}
		return nil
	}

	var years int
	switch v := value.(type) {
	case int64:
		years = int(v)
	case int:
		years = v
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("cannot parse retention period string '%s': %w", v, err)
		}
		years = parsed
	default:
		return fmt.Errorf("cannot scan %T into RetentionPeriod", value)
	}

	if years == 0 {
		*rp = RetentionPeriod{}
		return nil
	}

	retention, err := NewRetentionPeriodFromYears(years)
	if err != nil {
		return err
	}

	*rp = retention
	return nil
}

// RetentionPolicy represents a retention policy with different periods for different data types
type RetentionPolicy struct {
	AuditEvents     RetentionPeriod
	CallRecords     RetentionPeriod
	BidData         RetentionPeriod
	FinancialData   RetentionPeriod
	PersonalData    RetentionPeriod
}

// NewRetentionPolicy creates a new retention policy with default periods
func NewRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		AuditEvents:   StandardRetention(),   // 7 years
		CallRecords:   StandardRetention(),   // 7 years
		BidData:       StandardRetention(),   // 7 years
		FinancialData: ExtendedRetention(),   // 10 years
		PersonalData:  StandardRetention(),   // 7 years
	}
}

// IsCompliantWith checks if the policy meets compliance requirements
func (rp *RetentionPolicy) IsCompliantWith(jurisdiction string) bool {
	return rp.AuditEvents.IsCompliant(jurisdiction) &&
		rp.CallRecords.IsCompliant(jurisdiction) &&
		rp.BidData.IsCompliant(jurisdiction) &&
		rp.FinancialData.IsCompliant(jurisdiction) &&
		rp.PersonalData.IsCompliant(jurisdiction)
}

// ValidationError represents validation errors for retention periods
type RetentionValidationError struct {
	Value  string
	Reason string
}

func (e RetentionValidationError) Error() string {
	return fmt.Sprintf("invalid retention period '%s': %s", e.Value, e.Reason)
}

// ValidateRetentionPeriod validates that a duration could be a valid retention period
func ValidateRetentionPeriod(duration time.Duration) error {
	if duration <= 0 {
		return errors.NewValidationError("INVALID_RETENTION_DURATION", 
			"retention period must be positive")
	}

	years := int(duration.Hours() / (24 * 365))
	
	if years < MinRetentionYears {
		return errors.NewValidationError("RETENTION_TOO_SHORT", 
			fmt.Sprintf("retention period must be at least %d years", MinRetentionYears))
	}

	if years > MaxRetentionYears {
		return errors.NewValidationError("RETENTION_TOO_LONG", 
			fmt.Sprintf("retention period cannot exceed %d years", MaxRetentionYears))
	}

	return nil
}