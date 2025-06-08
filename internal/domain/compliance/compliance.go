package compliance

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ComplianceRule struct {
	ID          uuid.UUID   `json:"id"`
	Name        string      `json:"name"`
	Type        RuleType    `json:"type"`
	Status      RuleStatus  `json:"status"`
	Priority    int         `json:"priority"`
	
	// Rule definition
	Conditions  []Condition `json:"conditions"`
	Actions     []Action    `json:"actions"`
	
	// Geographic scope
	Geography   GeographicScope `json:"geography"`
	
	// Time restrictions
	TimeWindows []TimeWindow `json:"time_windows"`
	
	// Metadata
	Description string    `json:"description"`
	CreatedBy   uuid.UUID `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	EffectiveAt time.Time `json:"effective_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type RuleType int

const (
	RuleTypeTCPA RuleType = iota
	RuleTypeGDPR
	RuleTypeCCPA
	RuleTypeDNC
	RuleTypeCustom
)

func (t RuleType) String() string {
	switch t {
	case RuleTypeTCPA:
		return "tcpa"
	case RuleTypeGDPR:
		return "gdpr"
	case RuleTypeCCPA:
		return "ccpa"
	case RuleTypeDNC:
		return "dnc"
	case RuleTypeCustom:
		return "custom"
	default:
		return "unknown"
	}
}

type RuleStatus int

const (
	RuleStatusDraft RuleStatus = iota
	RuleStatusActive
	RuleStatusInactive
	RuleStatusExpired
)

type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type Action struct {
	Type   ActionType  `json:"type"`
	Params interface{} `json:"params"`
}

type ActionType int

const (
	ActionBlock ActionType = iota
	ActionWarn
	ActionLog
	ActionRequireConsent
	ActionTimeRestrict
)

type GeographicScope struct {
	Countries []string `json:"countries"`
	States    []string `json:"states"`
	Cities    []string `json:"cities"`
	ZipCodes  []string `json:"zip_codes"`
}

type TimeWindow struct {
	StartHour int      `json:"start_hour"`
	EndHour   int      `json:"end_hour"`
	Days      []string `json:"days"`
	Timezone  string   `json:"timezone"`
}

type ComplianceViolation struct {
	ID          uuid.UUID    `json:"id"`
	CallID      uuid.UUID    `json:"call_id"`
	AccountID   uuid.UUID    `json:"account_id"`
	RuleID      uuid.UUID    `json:"rule_id"`
	ViolationType ViolationType `json:"violation_type"`
	Severity    Severity     `json:"severity"`
	Description string       `json:"description"`
	Resolved    bool         `json:"resolved"`
	ResolvedBy  *uuid.UUID   `json:"resolved_by,omitempty"`
	ResolvedAt  *time.Time   `json:"resolved_at,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

type ViolationType int

const (
	ViolationTCPA ViolationType = iota
	ViolationGDPR
	ViolationDNC
	ViolationTimeRestriction
	ViolationConsent
	ViolationFraud
)

type Severity int

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

type ConsentRecord struct {
	ID              uuid.UUID   `json:"id"`
	PhoneNumber     string      `json:"phone_number"`
	ConsentType     ConsentType `json:"consent_type"`
	Status          ConsentStatus `json:"status"`
	Source          string      `json:"source"`
	IPAddress       string      `json:"ip_address"`
	UserAgent       string      `json:"user_agent"`
	OptInTimestamp  time.Time   `json:"opt_in_timestamp"`
	OptOutTimestamp *time.Time  `json:"opt_out_timestamp,omitempty"`
	ExpiresAt       *time.Time  `json:"expires_at,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type ConsentType int

const (
	ConsentTypeTCPA ConsentType = iota
	ConsentTypeGDPR
	ConsentTypeCCPA
	ConsentTypeMarketing
)

type ConsentStatus int

const (
	ConsentStatusActive ConsentStatus = iota
	ConsentStatusExpired
	ConsentStatusRevoked
	ConsentStatusPending
)

type ComplianceCheck struct {
	CallID        uuid.UUID `json:"call_id"`
	PhoneNumber   string    `json:"phone_number"`
	CallerID      string    `json:"caller_id"`
	Geography     Location  `json:"geography"`
	TimeOfCall    time.Time `json:"time_of_call"`
	ConsentStatus ConsentStatus `json:"consent_status"`
	Violations    []ComplianceViolation `json:"violations"`
	Approved      bool      `json:"approved"`
	Reason        string    `json:"reason,omitempty"`
}

type Location struct {
	Country   string  `json:"country"`
	State     string  `json:"state"`
	City      string  `json:"city"`
	ZipCode   string  `json:"zip_code"`
	Timezone  string  `json:"timezone"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func NewComplianceRule(name string, ruleType RuleType, createdBy uuid.UUID) *ComplianceRule {
	now := time.Now()
	return &ComplianceRule{
		ID:          uuid.New(),
		Name:        name,
		Type:        ruleType,
		Status:      RuleStatusDraft,
		Priority:    1,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		EffectiveAt: now,
	}
}

func NewConsentRecord(phoneNumber string, consentType ConsentType, source, ipAddress, userAgent string) *ConsentRecord {
	now := time.Now()
	return &ConsentRecord{
		ID:             uuid.New(),
		PhoneNumber:    phoneNumber,
		ConsentType:    consentType,
		Status:         ConsentStatusActive,
		Source:         source,
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		OptInTimestamp: now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (c *ConsentRecord) Revoke() {
	now := time.Now()
	c.Status = ConsentStatusRevoked
	c.OptOutTimestamp = &now
	c.UpdatedAt = now
}

func (cc *ComplianceCheck) AddViolation(violation ComplianceViolation) {
	cc.Violations = append(cc.Violations, violation)
	cc.Approved = false
}

func (cc *ComplianceCheck) IsCallAllowed() bool {
	return cc.Approved && len(cc.Violations) == 0
}

var (
	ErrComplianceViolation = fmt.Errorf("compliance violation detected")
	ErrNoConsent          = fmt.Errorf("no valid consent found")
	ErrTimeRestriction    = fmt.Errorf("call not allowed at this time")
	ErrGeographicRestriction = fmt.Errorf("calls not allowed in this geography")
)