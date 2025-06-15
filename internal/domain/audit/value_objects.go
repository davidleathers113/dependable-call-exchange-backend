package audit

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ActorType represents the type of entity performing an action
// Following DCE patterns: immutable value object with validation
type ActorType string

const (
	ActorTypeUser      ActorType = "user"
	ActorTypeSystem    ActorType = "system"
	ActorTypeAPI       ActorType = "api"
	ActorTypeService   ActorType = "service"
	ActorTypeAdmin     ActorType = "admin"
	ActorTypeGuest     ActorType = "guest"
	ActorTypeBot       ActorType = "bot"
	ActorTypeScheduler ActorType = "scheduler"
)

// NewActorType creates a new ActorType value object with validation
func NewActorType(actorType string) (ActorType, error) {
	if actorType == "" {
		return "", errors.NewValidationError("EMPTY_ACTOR_TYPE",
			"actor type cannot be empty")
	}

	normalized := ActorType(strings.ToLower(strings.TrimSpace(actorType)))

	if !normalized.IsValid() {
		return "", errors.NewValidationError("INVALID_ACTOR_TYPE",
			fmt.Sprintf("invalid actor type: %s", actorType))
	}

	return normalized, nil
}

// String returns the string representation of the actor type
func (at ActorType) String() string {
	return string(at)
}

// IsValid checks if the actor type is valid
func (at ActorType) IsValid() bool {
	switch at {
	case ActorTypeUser, ActorTypeSystem, ActorTypeAPI, ActorTypeService,
		ActorTypeAdmin, ActorTypeGuest, ActorTypeBot, ActorTypeScheduler:
		return true
	default:
		return false
	}
}

// Equal checks if two ActorType values are equal
func (at ActorType) Equal(other ActorType) bool {
	return at == other
}

// IsHuman returns true if the actor type represents a human user
func (at ActorType) IsHuman() bool {
	return at == ActorTypeUser || at == ActorTypeAdmin || at == ActorTypeGuest
}

// IsAutomated returns true if the actor type represents an automated system
func (at ActorType) IsAutomated() bool {
	return at == ActorTypeSystem || at == ActorTypeBot || at == ActorTypeScheduler
}

// GetDefaultTrustLevel returns the default trust level for this actor type
func (at ActorType) GetDefaultTrustLevel() string {
	switch at {
	case ActorTypeAdmin:
		return "high"
	case ActorTypeUser:
		return "medium"
	case ActorTypeAPI, ActorTypeService:
		return "medium"
	case ActorTypeSystem:
		return "high"
	case ActorTypeGuest:
		return "low"
	case ActorTypeBot, ActorTypeScheduler:
		return "medium"
	default:
		return "low"
	}
}

// TargetType represents the type of entity being acted upon
type TargetType string

const (
	TargetTypeUser          TargetType = "user"
	TargetTypeCall          TargetType = "call"
	TargetTypeBid           TargetType = "bid"
	TargetTypeAccount       TargetType = "account"
	TargetTypePhoneNumber   TargetType = "phone_number"
	TargetTypeUserProfile   TargetType = "user_profile"
	TargetTypeConfiguration TargetType = "configuration"
	TargetTypeRule          TargetType = "rule"
	TargetTypePermission    TargetType = "permission"
	TargetTypeSession       TargetType = "session"
	TargetTypeTransaction   TargetType = "transaction"
	TargetTypePayment       TargetType = "payment"
	TargetTypeAuditLog      TargetType = "audit_log"
	TargetTypeDatabase      TargetType = "database"
	TargetTypeFile          TargetType = "file"
	TargetTypeSystem        TargetType = "system"
)

// NewTargetType creates a new TargetType value object with validation
func NewTargetType(targetType string) (TargetType, error) {
	if targetType == "" {
		return "", errors.NewValidationError("EMPTY_TARGET_TYPE",
			"target type cannot be empty")
	}

	normalized := TargetType(strings.ToLower(strings.TrimSpace(targetType)))

	if !normalized.IsValid() {
		return "", errors.NewValidationError("INVALID_TARGET_TYPE",
			fmt.Sprintf("invalid target type: %s", targetType))
	}

	return normalized, nil
}

// String returns the string representation of the target type
func (tt TargetType) String() string {
	return string(tt)
}

// IsValid checks if the target type is valid
func (tt TargetType) IsValid() bool {
	switch tt {
	case TargetTypeUser, TargetTypeCall, TargetTypeBid, TargetTypeAccount,
		TargetTypePhoneNumber, TargetTypeUserProfile, TargetTypeConfiguration,
		TargetTypeRule, TargetTypePermission, TargetTypeSession, TargetTypeTransaction,
		TargetTypePayment, TargetTypeAuditLog, TargetTypeDatabase, TargetTypeFile,
		TargetTypeSystem:
		return true
	default:
		return false
	}
}

// Equal checks if two TargetType values are equal
func (tt TargetType) Equal(other TargetType) bool {
	return tt == other
}

// IsPII returns true if the target type contains personally identifiable information
func (tt TargetType) IsPII() bool {
	return tt == TargetTypeUser || tt == TargetTypePhoneNumber || tt == TargetTypeUserProfile
}

// IsFinancial returns true if the target type is financial-related
func (tt TargetType) IsFinancial() bool {
	return tt == TargetTypeTransaction || tt == TargetTypePayment
}

// GetDefaultDataClasses returns the default data classes for this target type
func (tt TargetType) GetDefaultDataClasses() []DataClass {
	switch tt {
	case TargetTypeUser:
		return []DataClass{DataClassPersonalData}
	case TargetTypePhoneNumber:
		return []DataClass{DataClassPhoneNumber, DataClassPersonalData}
	case TargetTypeUserProfile:
		return []DataClass{DataClassPersonalData, DataClassContactInfo}
	case TargetTypeTransaction, TargetTypePayment:
		return []DataClass{DataClassFinancialData}
	case TargetTypeCall:
		return []DataClass{DataClassCommunicationData}
	default:
		return []DataClass{}
	}
}

// EventCategory represents the high-level category of an audit event
type EventCategory string

const (
	EventCategoryConsent       EventCategory = "consent"
	EventCategoryDataAccess    EventCategory = "data_access"
	EventCategoryCall          EventCategory = "call"
	EventCategoryConfiguration EventCategory = "configuration"
	EventCategorySecurity      EventCategory = "security"
	EventCategoryMarketplace   EventCategory = "marketplace"
	EventCategoryFinancial     EventCategory = "financial"
	EventCategorySystem        EventCategory = "system"
	EventCategoryCompliance    EventCategory = "compliance"
	EventCategoryAudit         EventCategory = "audit"
	EventCategoryOther         EventCategory = "other"
)

// NewEventCategory creates a new EventCategory value object with validation
func NewEventCategory(category string) (EventCategory, error) {
	if category == "" {
		return "", errors.NewValidationError("EMPTY_EVENT_CATEGORY",
			"event category cannot be empty")
	}

	normalized := EventCategory(strings.ToLower(strings.TrimSpace(category)))

	if !normalized.IsValid() {
		return "", errors.NewValidationError("INVALID_EVENT_CATEGORY",
			fmt.Sprintf("invalid event category: %s", category))
	}

	return normalized, nil
}

// String returns the string representation of the event category
func (ec EventCategory) String() string {
	return string(ec)
}

// IsValid checks if the event category is valid
func (ec EventCategory) IsValid() bool {
	switch ec {
	case EventCategoryConsent, EventCategoryDataAccess, EventCategoryCall,
		EventCategoryConfiguration, EventCategorySecurity, EventCategoryMarketplace,
		EventCategoryFinancial, EventCategorySystem, EventCategoryCompliance,
		EventCategoryAudit, EventCategoryOther:
		return true
	default:
		return false
	}
}

// Equal checks if two EventCategory values are equal
func (ec EventCategory) Equal(other EventCategory) bool {
	return ec == other
}

// IsComplianceRelevant returns true if the category is compliance-relevant
func (ec EventCategory) IsComplianceRelevant() bool {
	return ec == EventCategoryConsent || ec == EventCategoryDataAccess ||
		ec == EventCategoryCompliance || ec == EventCategoryAudit
}

// GetDefaultRetentionYears returns the default retention period for this category
func (ec EventCategory) GetDefaultRetentionYears() int {
	switch ec {
	case EventCategoryConsent, EventCategoryCompliance, EventCategoryAudit:
		return 7 // 7 years for compliance-critical events
	case EventCategoryFinancial:
		return 10 // 10 years for financial records
	case EventCategorySecurity:
		return 3 // 3 years for security events
	default:
		return 7 // 7 years default
	}
}

// GetIcon returns an icon representation for the category
func (ec EventCategory) GetIcon() string {
	switch ec {
	case EventCategoryConsent:
		return "✓"
	case EventCategoryDataAccess:
		return "🔍"
	case EventCategoryCall:
		return "📞"
	case EventCategoryConfiguration:
		return "⚙️"
	case EventCategorySecurity:
		return "🔒"
	case EventCategoryMarketplace:
		return "💰"
	case EventCategoryFinancial:
		return "💳"
	case EventCategorySystem:
		return "🖥️"
	case EventCategoryCompliance:
		return "📋"
	case EventCategoryAudit:
		return "📊"
	default:
		return "📄"
	}
}

// ComplianceFlag represents a compliance-related flag with validation
type ComplianceFlag struct {
	name  string
	value bool
}

// NewComplianceFlag creates a new ComplianceFlag value object with validation
func NewComplianceFlag(name string, value bool) (ComplianceFlag, error) {
	if name == "" {
		return ComplianceFlag{}, errors.NewValidationError("EMPTY_COMPLIANCE_FLAG",
			"compliance flag name cannot be empty")
	}

	// Normalize flag name
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, " ", "_")

	if !isValidComplianceFlagName(normalized) {
		return ComplianceFlag{}, errors.NewValidationError("INVALID_COMPLIANCE_FLAG",
			fmt.Sprintf("invalid compliance flag name: %s", name))
	}

	return ComplianceFlag{
		name:  normalized,
		value: value,
	}, nil
}

// Name returns the flag name
func (cf ComplianceFlag) Name() string {
	return cf.name
}

// Value returns the flag value
func (cf ComplianceFlag) Value() bool {
	return cf.value
}

// String returns a string representation of the compliance flag
func (cf ComplianceFlag) String() string {
	return fmt.Sprintf("%s:%v", cf.name, cf.value)
}

// Equal checks if two ComplianceFlag values are equal
func (cf ComplianceFlag) Equal(other ComplianceFlag) bool {
	return cf.name == other.name && cf.value == other.value
}

// IsTrue returns true if the flag value is true
func (cf ComplianceFlag) IsTrue() bool {
	return cf.value
}

// IsFalse returns true if the flag value is false
func (cf ComplianceFlag) IsFalse() bool {
	return !cf.value
}

// DataClass represents a classification of data for GDPR/CCPA compliance
type DataClass string

const (
	DataClassPersonalData      DataClass = "personal_data"
	DataClassSensitiveData     DataClass = "sensitive_data"
	DataClassBiometricData     DataClass = "biometric_data"
	DataClassHealthData        DataClass = "health_data"
	DataClassFinancialData     DataClass = "financial_data"
	DataClassLocationData      DataClass = "location_data"
	DataClassCommunicationData DataClass = "communication_data"
	DataClassBehavioralData    DataClass = "behavioral_data"
	DataClassPhoneNumber       DataClass = "phone_number"
	DataClassEmail             DataClass = "email"
	DataClassIPAddress         DataClass = "ip_address"
	DataClassContactInfo       DataClass = "contact_info"
	DataClassDemographicData   DataClass = "demographic_data"
	DataClassPreferences       DataClass = "preferences"
	DataClassUsageData         DataClass = "usage_data"
	DataClassDeviceData        DataClass = "device_data"
	DataClassPublicData        DataClass = "public_data"
)

// NewDataClass creates a new DataClass value object with validation
func NewDataClass(dataClass string) (DataClass, error) {
	if dataClass == "" {
		return "", errors.NewValidationError("EMPTY_DATA_CLASS",
			"data class cannot be empty")
	}

	normalized := DataClass(strings.ToLower(strings.TrimSpace(dataClass)))

	if !normalized.IsValid() {
		return "", errors.NewValidationError("INVALID_DATA_CLASS",
			fmt.Sprintf("invalid data class: %s", dataClass))
	}

	return normalized, nil
}

// String returns the string representation of the data class
func (dc DataClass) String() string {
	return string(dc)
}

// IsValid checks if the data class is valid
func (dc DataClass) IsValid() bool {
	switch dc {
	case DataClassPersonalData, DataClassSensitiveData, DataClassBiometricData,
		DataClassHealthData, DataClassFinancialData, DataClassLocationData,
		DataClassCommunicationData, DataClassBehavioralData, DataClassPhoneNumber,
		DataClassEmail, DataClassIPAddress, DataClassContactInfo, DataClassDemographicData,
		DataClassPreferences, DataClassUsageData, DataClassDeviceData, DataClassPublicData:
		return true
	default:
		return false
	}
}

// Equal checks if two DataClass values are equal
func (dc DataClass) Equal(other DataClass) bool {
	return dc == other
}

// IsGDPRRelevant returns true if the data class is relevant for GDPR
func (dc DataClass) IsGDPRRelevant() bool {
	return dc != DataClassPublicData // All except public data is GDPR-relevant
}

// IsCCPARelevant returns true if the data class is relevant for CCPA
func (dc DataClass) IsCCPARelevant() bool {
	return dc == DataClassPersonalData || dc == DataClassSensitiveData ||
		dc == DataClassBiometricData || dc == DataClassLocationData ||
		dc == DataClassFinancialData
}

// IsSensitive returns true if the data class contains sensitive data
func (dc DataClass) IsSensitive() bool {
	return dc == DataClassSensitiveData || dc == DataClassBiometricData ||
		dc == DataClassHealthData || dc == DataClassFinancialData
}

// GetMinimumRetentionYears returns the minimum retention period for this data class
func (dc DataClass) GetMinimumRetentionYears() int {
	switch dc {
	case DataClassFinancialData:
		return 10 // Financial data requires longer retention
	case DataClassHealthData:
		return 7 // Health data requires extended retention
	case DataClassSensitiveData, DataClassBiometricData:
		return 7 // Sensitive data requires careful retention
	default:
		return 7 // Default 7 years for compliance
	}
}

// LegalBasis represents the legal basis for data processing under GDPR
type LegalBasis string

const (
	LegalBasisConsent            LegalBasis = "consent"
	LegalBasisContract           LegalBasis = "contract"
	LegalBasisLegalObligation    LegalBasis = "legal_obligation"
	LegalBasisVitalInterests     LegalBasis = "vital_interests"
	LegalBasisPublicTask         LegalBasis = "public_task"
	LegalBasisLegitimateInterest LegalBasis = "legitimate_interest"
)

// NewLegalBasis creates a new LegalBasis value object with validation
func NewLegalBasis(legalBasis string) (LegalBasis, error) {
	if legalBasis == "" {
		return "", errors.NewValidationError("EMPTY_LEGAL_BASIS",
			"legal basis cannot be empty")
	}

	normalized := LegalBasis(strings.ToLower(strings.TrimSpace(legalBasis)))

	if !normalized.IsValid() {
		return "", errors.NewValidationError("INVALID_LEGAL_BASIS",
			fmt.Sprintf("invalid legal basis: %s", legalBasis))
	}

	return normalized, nil
}

// String returns the string representation of the legal basis
func (lb LegalBasis) String() string {
	return string(lb)
}

// IsValid checks if the legal basis is valid under GDPR
func (lb LegalBasis) IsValid() bool {
	switch lb {
	case LegalBasisConsent, LegalBasisContract, LegalBasisLegalObligation,
		LegalBasisVitalInterests, LegalBasisPublicTask, LegalBasisLegitimateInterest:
		return true
	default:
		return false
	}
}

// Equal checks if two LegalBasis values are equal
func (lb LegalBasis) Equal(other LegalBasis) bool {
	return lb == other
}

// RequiresExplicitConsent returns true if this legal basis requires explicit consent
func (lb LegalBasis) RequiresExplicitConsent() bool {
	return lb == LegalBasisConsent
}

// AllowsWithdrawal returns true if the legal basis allows withdrawal of consent
func (lb LegalBasis) AllowsWithdrawal() bool {
	return lb == LegalBasisConsent || lb == LegalBasisLegitimateInterest
}

// GetDescription returns a human-readable description of the legal basis
func (lb LegalBasis) GetDescription() string {
	switch lb {
	case LegalBasisConsent:
		return "The data subject has given consent for processing"
	case LegalBasisContract:
		return "Processing is necessary for the performance of a contract"
	case LegalBasisLegalObligation:
		return "Processing is necessary for compliance with a legal obligation"
	case LegalBasisVitalInterests:
		return "Processing is necessary to protect vital interests"
	case LegalBasisPublicTask:
		return "Processing is necessary for the performance of a public task"
	case LegalBasisLegitimateInterest:
		return "Processing is necessary for legitimate interests"
	default:
		return "Unknown legal basis"
	}
}

// EventResult represents the outcome of an audited action with enhanced validation
type EventResult string

const (
	EventResultSuccess   EventResult = "success"
	EventResultFailure   EventResult = "failure"
	EventResultPartial   EventResult = "partial"
	EventResultPending   EventResult = "pending"
	EventResultTimeout   EventResult = "timeout"
	EventResultCancelled EventResult = "cancelled"
)

// NewEventResult creates a new EventResult value object with validation
func NewEventResult(result string) (EventResult, error) {
	if result == "" {
		return "", errors.NewValidationError("EMPTY_EVENT_RESULT",
			"event result cannot be empty")
	}

	normalized := EventResult(strings.ToLower(strings.TrimSpace(result)))

	if !normalized.IsValid() {
		return "", errors.NewValidationError("INVALID_EVENT_RESULT",
			fmt.Sprintf("invalid event result: %s", result))
	}

	return normalized, nil
}

// String returns the string representation of the event result
func (er EventResult) String() string {
	return string(er)
}

// IsValid checks if the event result is valid
func (er EventResult) IsValid() bool {
	switch er {
	case EventResultSuccess, EventResultFailure, EventResultPartial,
		EventResultPending, EventResultTimeout, EventResultCancelled:
		return true
	default:
		return false
	}
}

// Equal checks if two EventResult values are equal
func (er EventResult) Equal(other EventResult) bool {
	return er == other
}

// IsSuccess returns true if the result indicates success
func (er EventResult) IsSuccess() bool {
	return er == EventResultSuccess
}

// IsFailure returns true if the result indicates failure
func (er EventResult) IsFailure() bool {
	return er == EventResultFailure
}

// IsPartial returns true if the result indicates partial success
func (er EventResult) IsPartial() bool {
	return er == EventResultPartial
}

// IsCompleted returns true if the result indicates completion (success, failure, or partial)
func (er EventResult) IsCompleted() bool {
	return er == EventResultSuccess || er == EventResultFailure || er == EventResultPartial
}

// IsPending returns true if the result indicates the operation is still in progress
func (er EventResult) IsPending() bool {
	return er == EventResultPending
}

// GetIcon returns an icon representation for the result
func (er EventResult) GetIcon() string {
	switch er {
	case EventResultSuccess:
		return "✅"
	case EventResultFailure:
		return "❌"
	case EventResultPartial:
		return "⚠️"
	case EventResultPending:
		return "⏳"
	case EventResultTimeout:
		return "⏰"
	case EventResultCancelled:
		return "🚫"
	default:
		return "❓"
	}
}

// GetDefaultSeverity returns the default severity for this result type
func (er EventResult) GetDefaultSeverity() Severity {
	switch er {
	case EventResultSuccess:
		return SeverityInfo
	case EventResultFailure:
		return SeverityError
	case EventResultPartial:
		return SeverityWarning
	case EventResultTimeout:
		return SeverityWarning
	case EventResultCancelled:
		return SeverityInfo
	case EventResultPending:
		return SeverityInfo
	default:
		return SeverityInfo
	}
}

// JSON marshaling implementations for database storage

// MarshalJSON implements JSON marshaling for ActorType
func (at ActorType) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(at))
}

// UnmarshalJSON implements JSON unmarshaling for ActorType
func (at *ActorType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	actorType, err := NewActorType(s)
	if err != nil {
		return err
	}

	*at = actorType
	return nil
}

// Value implements driver.Valuer for database storage
func (at ActorType) Value() (driver.Value, error) {
	if at == "" {
		return nil, nil
	}
	return string(at), nil
}

// Scan implements sql.Scanner for database retrieval
func (at *ActorType) Scan(value interface{}) error {
	if value == nil {
		*at = ""
		return nil
	}

	var str string
	switch v := value.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("cannot scan %T into ActorType", value)
	}

	if str == "" {
		*at = ""
		return nil
	}

	actorType, err := NewActorType(str)
	if err != nil {
		return err
	}

	*at = actorType
	return nil
}

// Equivalent implementations for other value objects...
// (Similar JSON/Database methods for TargetType, EventCategory, DataClass, LegalBasis, EventResult)

// Helper functions

// isValidComplianceFlagName validates compliance flag names
func isValidComplianceFlagName(name string) bool {
	// List of known compliance flags
	validFlags := []string{
		"gdpr_compliant", "tcpa_compliant", "ccpa_compliant", "hipaa_compliant",
		"contains_pii", "contains_phi", "gdpr_relevant", "tcpa_relevant",
		"ccpa_relevant", "requires_consent", "explicit_consent", "opt_in_required",
		"data_minimization", "purpose_limitation", "retention_limited",
		"pseudonymized", "encrypted", "anonymized", "cross_border_transfer",
		"third_party_sharing", "marketing_allowed", "profiling_allowed",
	}

	for _, valid := range validFlags {
		if name == valid {
			return true
		}
	}

	// Allow custom flags that follow naming convention
	return len(name) >= 3 && len(name) <= 50
}

// Helper functions for timestamp handling

// NewRetentionPeriodFromDataClasses calculates retention period based on data classes
func NewRetentionPeriodFromDataClasses(dataClasses []DataClass) (time.Duration, error) {
	if len(dataClasses) == 0 {
		return 7 * 365 * 24 * time.Hour, nil // 7 years default
	}

	maxYears := 7
	for _, dataClass := range dataClasses {
		if years := dataClass.GetMinimumRetentionYears(); years > maxYears {
			maxYears = years
		}
	}

	return time.Duration(maxYears) * 365 * 24 * time.Hour, nil
}

// ValidateComplianceFlags validates a map of compliance flags
func ValidateComplianceFlags(flags map[string]bool) error {
	for name := range flags {
		if _, err := NewComplianceFlag(name, true); err != nil {
			return err
		}
	}
	return nil
}

// ValidateDataClasses validates a slice of data classes
func ValidateDataClasses(dataClasses []string) error {
	for _, dataClass := range dataClasses {
		if _, err := NewDataClass(dataClass); err != nil {
			return err
		}
	}
	return nil
}
