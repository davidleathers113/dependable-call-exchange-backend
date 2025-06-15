package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// CheckType represents the type of DNC check performed
type CheckType struct {
	checkType string
}

// Supported check types
const (
	CheckTypeManual    = "manual"
	CheckTypeAutomated = "automated"
	CheckTypePeriodic  = "periodic"
	CheckTypeRealTime  = "real_time"
)

var (
	// Map of check type to display names
	checkTypeDisplayNames = map[string]string{
		CheckTypeManual:    "Manual Check",
		CheckTypeAutomated: "Automated Check",
		CheckTypePeriodic:  "Periodic Check",
		CheckTypeRealTime:  "Real-Time Check",
	}

	// Supported check types for validation
	supportedCheckTypes = map[string]bool{
		CheckTypeManual:    true,
		CheckTypeAutomated: true,
		CheckTypePeriodic:  true,
		CheckTypeRealTime:  true,
	}

	// Performance characteristics (relative cost/latency)
	checkTypePerformance = map[string]struct {
		LatencyMs   int  // Expected latency in milliseconds
		Cost        int  // Relative cost (1-10 scale)
		RequiresAPI bool // Whether it requires external API
	}{
		CheckTypeManual:    {LatencyMs: 0, Cost: 1, RequiresAPI: false},      // Human initiated
		CheckTypeAutomated: {LatencyMs: 100, Cost: 3, RequiresAPI: true},     // Batch processing
		CheckTypePeriodic:  {LatencyMs: 0, Cost: 2, RequiresAPI: false},      // Pre-cached results
		CheckTypeRealTime:  {LatencyMs: 50, Cost: 5, RequiresAPI: true},      // Live API check
	}

	// Check types that should be logged for compliance
	auditableCheckTypes = map[string]bool{
		CheckTypeManual:    true,
		CheckTypeAutomated: true,
		CheckTypePeriodic:  true,
		CheckTypeRealTime:  true,
	}

	// Check types that can be cached
	cacheableCheckTypes = map[string]bool{
		CheckTypeAutomated: true,
		CheckTypePeriodic:  true,
		CheckTypeRealTime:  true,
	}
)

// NewCheckType creates a new CheckType value object with validation
func NewCheckType(checkType string) (CheckType, error) {
	if checkType == "" {
		return CheckType{}, errors.NewValidationError("EMPTY_CHECK_TYPE",
			"check type cannot be empty")
	}

	// Normalize check type
	normalized := strings.ToLower(strings.TrimSpace(checkType))
	// Handle common variations
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	if !supportedCheckTypes[normalized] {
		return CheckType{}, errors.NewValidationError("UNSUPPORTED_CHECK_TYPE",
			fmt.Sprintf("check type '%s' is not supported", checkType))
	}

	return CheckType{checkType: normalized}, nil
}

// MustNewCheckType creates CheckType and panics on error (for constants/tests)
func MustNewCheckType(checkType string) CheckType {
	ct, err := NewCheckType(checkType)
	if err != nil {
		panic(err)
	}
	return ct
}

// Standard check types
func ManualCheckType() CheckType {
	return MustNewCheckType(CheckTypeManual)
}

func AutomatedCheckType() CheckType {
	return MustNewCheckType(CheckTypeAutomated)
}

func PeriodicCheckType() CheckType {
	return MustNewCheckType(CheckTypePeriodic)
}

func RealTimeCheckType() CheckType {
	return MustNewCheckType(CheckTypeRealTime)
}

// String returns the check type string
func (ct CheckType) String() string {
	return ct.checkType
}

// Value returns the underlying check type value
func (ct CheckType) Value() string {
	return ct.checkType
}

// IsValid checks if the check type is valid
func (ct CheckType) IsValid() bool {
	return ct.checkType != "" && supportedCheckTypes[ct.checkType]
}

// IsEmpty checks if the check type is empty
func (ct CheckType) IsEmpty() bool {
	return ct.checkType == ""
}

// Equal checks if two CheckType values are equal
func (ct CheckType) Equal(other CheckType) bool {
	return ct.checkType == other.checkType
}

// DisplayName returns the human-readable name for the check type
func (ct CheckType) DisplayName() string {
	if name, ok := checkTypeDisplayNames[ct.checkType]; ok {
		return name
	}
	return strings.Title(strings.ReplaceAll(ct.checkType, "_", " "))
}

// IsManual checks if the check type is manual
func (ct CheckType) IsManual() bool {
	return ct.checkType == CheckTypeManual
}

// IsAutomated checks if the check type is automated
func (ct CheckType) IsAutomated() bool {
	return ct.checkType == CheckTypeAutomated
}

// IsPeriodic checks if the check type is periodic
func (ct CheckType) IsPeriodic() bool {
	return ct.checkType == CheckTypePeriodic
}

// IsRealTime checks if the check type is real-time
func (ct CheckType) IsRealTime() bool {
	return ct.checkType == CheckTypeRealTime
}

// ExpectedLatencyMs returns the expected latency in milliseconds
func (ct CheckType) ExpectedLatencyMs() int {
	if perf, ok := checkTypePerformance[ct.checkType]; ok {
		return perf.LatencyMs
	}
	return 0
}

// RelativeCost returns the relative cost on a 1-10 scale
func (ct CheckType) RelativeCost() int {
	if perf, ok := checkTypePerformance[ct.checkType]; ok {
		return perf.Cost
	}
	return 1
}

// RequiresAPI checks if the check type requires external API calls
func (ct CheckType) RequiresAPI() bool {
	if perf, ok := checkTypePerformance[ct.checkType]; ok {
		return perf.RequiresAPI
	}
	return false
}

// IsAuditable checks if the check type should be logged for compliance
func (ct CheckType) IsAuditable() bool {
	return auditableCheckTypes[ct.checkType]
}

// IsCacheable checks if the check type results can be cached
func (ct CheckType) IsCacheable() bool {
	return cacheableCheckTypes[ct.checkType]
}

// RequiresUserInteraction checks if the check type requires user interaction
func (ct CheckType) RequiresUserInteraction() bool {
	return ct.checkType == CheckTypeManual
}

// IsAsynchronous checks if the check type should be processed asynchronously
func (ct CheckType) IsAsynchronous() bool {
	return ct.checkType == CheckTypePeriodic || ct.checkType == CheckTypeAutomated
}

// GetCacheTTL returns the recommended cache TTL in seconds
func (ct CheckType) GetCacheTTL() int {
	switch ct.checkType {
	case CheckTypeRealTime:
		return 300 // 5 minutes for real-time checks
	case CheckTypeAutomated:
		return 3600 // 1 hour for automated checks
	case CheckTypePeriodic:
		return 86400 // 24 hours for periodic checks
	default:
		return 0 // No caching for manual checks
	}
}

// GetAuditLevel returns the audit detail level required
func (ct CheckType) GetAuditLevel() string {
	switch ct.checkType {
	case CheckTypeManual:
		return "detailed" // Manual checks need full audit trail
	case CheckTypeRealTime:
		return "standard" // Real-time checks need standard logging
	case CheckTypeAutomated, CheckTypePeriodic:
		return "summary" // Batch checks can use summary logging
	default:
		return "minimal"
	}
}

// ValidateForContext validates if the check type is appropriate for a specific context
func (ct CheckType) ValidateForContext(context string) error {
	switch strings.ToLower(context) {
	case "call_initiation":
		if ct.checkType != CheckTypeRealTime {
			return errors.NewValidationError("INAPPROPRIATE_CHECK_TYPE",
				fmt.Sprintf("%s is not suitable for call initiation", ct.DisplayName()))
		}
	case "batch_import":
		if ct.checkType == CheckTypeRealTime || ct.checkType == CheckTypeManual {
			return errors.NewValidationError("INAPPROPRIATE_CHECK_TYPE",
				fmt.Sprintf("%s is not suitable for batch import", ct.DisplayName()))
		}
	case "compliance_audit":
		// All types are suitable for compliance audit
		return nil
	default:
		// No specific validation for other contexts
		return nil
	}
	return nil
}

// GetProcessingMode returns the recommended processing mode
func (ct CheckType) GetProcessingMode() string {
	switch ct.checkType {
	case CheckTypeManual:
		return "interactive"
	case CheckTypeRealTime:
		return "synchronous"
	case CheckTypeAutomated, CheckTypePeriodic:
		return "asynchronous"
	default:
		return "unknown"
	}
}

// MarshalJSON implements JSON marshaling
func (ct CheckType) MarshalJSON() ([]byte, error) {
	return json.Marshal(ct.checkType)
}

// UnmarshalJSON implements JSON unmarshaling
func (ct *CheckType) UnmarshalJSON(data []byte) error {
	var checkType string
	if err := json.Unmarshal(data, &checkType); err != nil {
		return err
	}

	checkTypeObj, err := NewCheckType(checkType)
	if err != nil {
		return err
	}

	*ct = checkTypeObj
	return nil
}

// Value implements driver.Valuer for database storage
func (ct CheckType) Value() (driver.Value, error) {
	if ct.checkType == "" {
		return nil, nil
	}
	return ct.checkType, nil
}

// Scan implements sql.Scanner for database retrieval
func (ct *CheckType) Scan(value interface{}) error {
	if value == nil {
		*ct = CheckType{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into CheckType", value)
	}

	if str == "" {
		*ct = CheckType{}
		return nil
	}

	checkType, err := NewCheckType(str)
	if err != nil {
		return err
	}

	*ct = checkType
	return nil
}

// GetSupportedCheckTypes returns all supported check types
func GetSupportedCheckTypes() []string {
	types := make([]string, 0, len(supportedCheckTypes))
	for checkType := range supportedCheckTypes {
		types = append(types, checkType)
	}
	return types
}

// GetCheckTypeDisplayNames returns all check type display names
func GetCheckTypeDisplayNames() []string {
	names := make([]string, 0, len(checkTypeDisplayNames))
	for _, name := range checkTypeDisplayNames {
		names = append(names, name)
	}
	return names
}

// ValidateCheckType validates that a string could be a valid check type
func ValidateCheckType(checkType string) error {
	if checkType == "" {
		return errors.NewValidationError("EMPTY_CHECK_TYPE", "check type cannot be empty")
	}

	normalized := strings.ToLower(strings.TrimSpace(checkType))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	if !supportedCheckTypes[normalized] {
		return errors.NewValidationError("UNSUPPORTED_CHECK_TYPE",
			fmt.Sprintf("check type '%s' is not supported", checkType))
	}

	return nil
}

// GetOptimalCheckType returns the optimal check type for a given use case
func GetOptimalCheckType(useCase string, latencyRequirementMs int) (CheckType, error) {
	switch strings.ToLower(useCase) {
	case "call_routing":
		if latencyRequirementMs < 100 {
			return RealTimeCheckType(), nil
		}
		return PeriodicCheckType(), nil
	case "bulk_validation":
		return AutomatedCheckType(), nil
	case "user_request":
		return ManualCheckType(), nil
	case "scheduled_compliance":
		return PeriodicCheckType(), nil
	default:
		// Default to automated for unknown use cases
		return AutomatedCheckType(), nil
	}
}