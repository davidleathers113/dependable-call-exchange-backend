package values

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// SuppressReason represents a reason for suppressing a phone number from calling
type SuppressReason struct {
	reason string
}

// Supported suppress reasons
const (
	SuppressReasonFederal        = "federal_dnc"
	SuppressReasonState          = "state_dnc"
	SuppressReasonCompanyPolicy  = "company_policy"
	SuppressReasonUserRequest    = "user_request"
	SuppressReasonLitigation     = "litigation"
	SuppressReasonFraud          = "fraud"
	SuppressReasonInvalidNumber  = "invalid_number"
	SuppressReasonBusinessHours  = "business_hours"
	SuppressReasonExcessiveCalls = "excessive_calls"
	SuppressReasonOptOut         = "opt_out"
)

var (
	// Map of reason to display names
	reasonDisplayNames = map[string]string{
		SuppressReasonFederal:        "Federal Do Not Call Registry",
		SuppressReasonState:          "State Do Not Call Registry",
		SuppressReasonCompanyPolicy:  "Company Policy Restriction",
		SuppressReasonUserRequest:    "User Request",
		SuppressReasonLitigation:     "Litigation Hold",
		SuppressReasonFraud:          "Fraud Prevention",
		SuppressReasonInvalidNumber:  "Invalid Phone Number",
		SuppressReasonBusinessHours:  "Outside Business Hours",
		SuppressReasonExcessiveCalls: "Excessive Call Frequency",
		SuppressReasonOptOut:         "Opt-Out Request",
	}

	// Supported reasons for validation
	supportedReasons = map[string]bool{
		SuppressReasonFederal:        true,
		SuppressReasonState:          true,
		SuppressReasonCompanyPolicy:  true,
		SuppressReasonUserRequest:    true,
		SuppressReasonLitigation:     true,
		SuppressReasonFraud:          true,
		SuppressReasonInvalidNumber:  true,
		SuppressReasonBusinessHours:  true,
		SuppressReasonExcessiveCalls: true,
		SuppressReasonOptOut:         true,
	}

	// Severity levels (1-10, higher = more severe)
	reasonSeverityLevels = map[string]int{
		SuppressReasonFederal:        10, // Highest - federal law
		SuppressReasonState:          9,  // State law
		SuppressReasonLitigation:     9,  // Legal risk
		SuppressReasonFraud:          8,  // Security risk
		SuppressReasonOptOut:         7,  // Consumer protection
		SuppressReasonUserRequest:    6,  // User preference
		SuppressReasonCompanyPolicy:  5,  // Company rules
		SuppressReasonInvalidNumber:  4,  // Technical issue
		SuppressReasonExcessiveCalls: 3,  // Rate limiting
		SuppressReasonBusinessHours:  2,  // Temporary restriction
	}

	// Compliance implications
	complianceReasons = map[string]struct {
		RequiresDocumentation bool
		PenaltyAmount         int    // Potential penalty in USD
		ComplianceCode        string // Regulatory code
		RetentionDays         int    // How long to retain records
	}{
		SuppressReasonFederal: {
			RequiresDocumentation: true,
			PenaltyAmount:         43792, // FTC penalty per violation (2024)
			ComplianceCode:        "TCPA-FEDERAL",
			RetentionDays:         1825, // 5 years
		},
		SuppressReasonState: {
			RequiresDocumentation: true,
			PenaltyAmount:         25000, // Average state penalty
			ComplianceCode:        "TCPA-STATE",
			RetentionDays:         1095, // 3 years
		},
		SuppressReasonLitigation: {
			RequiresDocumentation: true,
			PenaltyAmount:         0, // Varies by case
			ComplianceCode:        "LEGAL-HOLD",
			RetentionDays:         2555, // 7 years
		},
		SuppressReasonOptOut: {
			RequiresDocumentation: true,
			PenaltyAmount:         1500, // Per TCPA violation
			ComplianceCode:        "TCPA-OPTOUT",
			RetentionDays:         1825, // 5 years
		},
	}

	// Reasons that can be overridden with proper authorization
	overridableReasons = map[string]bool{
		SuppressReasonCompanyPolicy:  true,
		SuppressReasonBusinessHours:  true,
		SuppressReasonExcessiveCalls: true,
		SuppressReasonInvalidNumber:  false, // Cannot override technical issues
	}

	// Reasons that expire automatically
	temporaryReasons = map[string]bool{
		SuppressReasonBusinessHours:  true,
		SuppressReasonExcessiveCalls: true,
	}
)

// NewSuppressReason creates a new SuppressReason value object with validation
func NewSuppressReason(reason string) (SuppressReason, error) {
	if reason == "" {
		return SuppressReason{}, errors.NewValidationError("EMPTY_SUPPRESS_REASON",
			"suppress reason cannot be empty")
	}

	// Normalize reason
	normalized := strings.ToLower(strings.TrimSpace(reason))
	// Handle common variations
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	if !supportedReasons[normalized] {
		return SuppressReason{}, errors.NewValidationError("UNSUPPORTED_SUPPRESS_REASON",
			fmt.Sprintf("suppress reason '%s' is not supported", reason))
	}

	return SuppressReason{reason: normalized}, nil
}

// MustNewSuppressReason creates SuppressReason and panics on error (for constants/tests)
func MustNewSuppressReason(reason string) SuppressReason {
	sr, err := NewSuppressReason(reason)
	if err != nil {
		panic(err)
	}
	return sr
}

// Standard suppress reasons
func FederalDNCSuppressReason() SuppressReason {
	return MustNewSuppressReason(SuppressReasonFederal)
}

func StateDNCSuppressReason() SuppressReason {
	return MustNewSuppressReason(SuppressReasonState)
}

func CompanyPolicySuppressReason() SuppressReason {
	return MustNewSuppressReason(SuppressReasonCompanyPolicy)
}

func UserRequestSuppressReason() SuppressReason {
	return MustNewSuppressReason(SuppressReasonUserRequest)
}

func OptOutSuppressReason() SuppressReason {
	return MustNewSuppressReason(SuppressReasonOptOut)
}

// String returns the reason string
func (sr SuppressReason) String() string {
	return sr.reason
}

// Value returns the underlying reason value
func (sr SuppressReason) Value() string {
	return sr.reason
}

// IsValid checks if the suppress reason is valid
func (sr SuppressReason) IsValid() bool {
	return sr.reason != "" && supportedReasons[sr.reason]
}

// IsEmpty checks if the reason is empty
func (sr SuppressReason) IsEmpty() bool {
	return sr.reason == ""
}

// Equal checks if two SuppressReason values are equal
func (sr SuppressReason) Equal(other SuppressReason) bool {
	return sr.reason == other.reason
}

// DisplayName returns the human-readable name for the reason
func (sr SuppressReason) DisplayName() string {
	if name, ok := reasonDisplayNames[sr.reason]; ok {
		return name
	}
	return strings.Title(strings.ReplaceAll(sr.reason, "_", " "))
}

// SeverityLevel returns the severity level (1-10, higher = more severe)
func (sr SuppressReason) SeverityLevel() int {
	if level, ok := reasonSeverityLevels[sr.reason]; ok {
		return level
	}
	return 1
}

// IsRegulatory checks if the reason has regulatory/legal implications
func (sr SuppressReason) IsRegulatory() bool {
	return sr.reason == SuppressReasonFederal || 
		   sr.reason == SuppressReasonState || 
		   sr.reason == SuppressReasonLitigation ||
		   sr.reason == SuppressReasonOptOut
}

// RequiresDocumentation checks if the reason requires compliance documentation
func (sr SuppressReason) RequiresDocumentation() bool {
	if compliance, ok := complianceReasons[sr.reason]; ok {
		return compliance.RequiresDocumentation
	}
	return false
}

// GetPenaltyAmount returns the potential penalty amount in USD
func (sr SuppressReason) GetPenaltyAmount() int {
	if compliance, ok := complianceReasons[sr.reason]; ok {
		return compliance.PenaltyAmount
	}
	return 0
}

// GetComplianceCode returns the regulatory compliance code
func (sr SuppressReason) GetComplianceCode() string {
	if compliance, ok := complianceReasons[sr.reason]; ok {
		return compliance.ComplianceCode
	}
	return ""
}

// GetRetentionDays returns how long records must be retained
func (sr SuppressReason) GetRetentionDays() int {
	if compliance, ok := complianceReasons[sr.reason]; ok {
		return compliance.RetentionDays
	}
	return 365 // Default 1 year retention
}

// IsOverridable checks if the reason can be overridden with authorization
func (sr SuppressReason) IsOverridable() bool {
	return overridableReasons[sr.reason]
}

// IsTemporary checks if the reason expires automatically
func (sr SuppressReason) IsTemporary() bool {
	return temporaryReasons[sr.reason]
}

// IsPermanent checks if the reason is permanent
func (sr SuppressReason) IsPermanent() bool {
	return !sr.IsTemporary()
}

// HasHigherSeverity compares severity with another reason
func (sr SuppressReason) HasHigherSeverity(other SuppressReason) bool {
	return sr.SeverityLevel() > other.SeverityLevel()
}

// GetRiskLevel returns the risk level as a string
func (sr SuppressReason) GetRiskLevel() string {
	severity := sr.SeverityLevel()
	switch {
	case severity >= 9:
		return "critical"
	case severity >= 7:
		return "high"
	case severity >= 5:
		return "medium"
	case severity >= 3:
		return "low"
	default:
		return "minimal"
	}
}

// ValidateForAction validates if the reason is appropriate for a specific action
func (sr SuppressReason) ValidateForAction(action string) error {
	switch strings.ToLower(action) {
	case "override":
		if !sr.IsOverridable() {
			return errors.NewValidationError("NON_OVERRIDABLE_REASON",
				fmt.Sprintf("%s cannot be overridden", sr.DisplayName()))
		}
	case "expire":
		if !sr.IsTemporary() {
			return errors.NewValidationError("NON_EXPIRABLE_REASON",
				fmt.Sprintf("%s does not expire automatically", sr.DisplayName()))
		}
	case "document":
		if !sr.RequiresDocumentation() {
			return nil // No error, documentation is optional
		}
	default:
		// No specific validation for other actions
		return nil
	}
	return nil
}

// GetNotificationRequirement returns who should be notified
func (sr SuppressReason) GetNotificationRequirement() string {
	severity := sr.SeverityLevel()
	switch {
	case severity >= 9:
		return "legal_compliance_team"
	case severity >= 7:
		return "compliance_manager"
	case severity >= 5:
		return "team_lead"
	default:
		return "none"
	}
}

// GetAuditPriority returns the audit priority
func (sr SuppressReason) GetAuditPriority() string {
	if sr.IsRegulatory() {
		return "high"
	}
	if sr.SeverityLevel() >= 7 {
		return "medium"
	}
	return "low"
}

// MarshalJSON implements JSON marshaling
func (sr SuppressReason) MarshalJSON() ([]byte, error) {
	return json.Marshal(sr.reason)
}

// UnmarshalJSON implements JSON unmarshaling
func (sr *SuppressReason) UnmarshalJSON(data []byte) error {
	var reason string
	if err := json.Unmarshal(data, &reason); err != nil {
		return err
	}

	suppressReason, err := NewSuppressReason(reason)
	if err != nil {
		return err
	}

	*sr = suppressReason
	return nil
}

// Value implements driver.Valuer for database storage
func (sr SuppressReason) Value() (driver.Value, error) {
	if sr.reason == "" {
		return nil, nil
	}
	return sr.reason, nil
}

// Scan implements sql.Scanner for database retrieval
func (sr *SuppressReason) Scan(value interface{}) error {
	if value == nil {
		*sr = SuppressReason{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into SuppressReason", value)
	}

	if str == "" {
		*sr = SuppressReason{}
		return nil
	}

	suppressReason, err := NewSuppressReason(str)
	if err != nil {
		return err
	}

	*sr = suppressReason
	return nil
}

// GetSupportedReasons returns all supported suppress reasons
func GetSupportedReasons() []string {
	reasons := make([]string, 0, len(supportedReasons))
	for reason := range supportedReasons {
		reasons = append(reasons, reason)
	}
	return reasons
}

// GetRegulatoryReasons returns all regulatory suppress reasons
func GetRegulatoryReasons() []string {
	reasons := make([]string, 0)
	for reason := range supportedReasons {
		sr := MustNewSuppressReason(reason)
		if sr.IsRegulatory() {
			reasons = append(reasons, reason)
		}
	}
	return reasons
}

// ValidateSuppressReason validates that a string could be a valid suppress reason
func ValidateSuppressReason(reason string) error {
	if reason == "" {
		return errors.NewValidationError("EMPTY_SUPPRESS_REASON", "suppress reason cannot be empty")
	}

	normalized := strings.ToLower(strings.TrimSpace(reason))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	if !supportedReasons[normalized] {
		return errors.NewValidationError("UNSUPPORTED_SUPPRESS_REASON",
			fmt.Sprintf("suppress reason '%s' is not supported", reason))
	}

	return nil
}

// GetMostSevereReason returns the most severe reason from a list
func GetMostSevereReason(reasons []SuppressReason) SuppressReason {
	if len(reasons) == 0 {
		return SuppressReason{}
	}

	mostSevere := reasons[0]
	for _, reason := range reasons[1:] {
		if reason.HasHigherSeverity(mostSevere) {
			mostSevere = reason
		}
	}
	return mostSevere
}