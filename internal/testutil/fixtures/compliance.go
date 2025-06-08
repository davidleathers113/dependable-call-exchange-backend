package fixtures

import (
	"testing"
	"time"
	
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
)

// ComplianceRuleBuilder builds test ComplianceRule entities
type ComplianceRuleBuilder struct {
	t           *testing.T
	id          uuid.UUID
	name        string
	ruleType    compliance.RuleType
	status      compliance.RuleStatus
	priority    int
	conditions  []compliance.Condition
	actions     []compliance.Action
	geography   compliance.GeographicScope
	timeWindows []compliance.TimeWindow
	description string
	createdBy   uuid.UUID
	effectiveAt time.Time
	expiresAt   *time.Time
}

// NewComplianceRuleBuilder creates a new ComplianceRuleBuilder with defaults
func NewComplianceRuleBuilder(t *testing.T) *ComplianceRuleBuilder {
	t.Helper()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	createdBy, err := uuid.NewRandom()
	require.NoError(t, err)
	
	now := time.Now().UTC()
	return &ComplianceRuleBuilder{
		t:           t,
		id:          id,
		name:        "Default TCPA Rule",
		ruleType:    compliance.RuleTypeTCPA,
		status:      compliance.RuleStatusActive,
		priority:    100,
		description: "Default TCPA compliance rule",
		createdBy:   createdBy,
		effectiveAt: now,
		conditions: []compliance.Condition{
			{
				Field:    "time_of_day",
				Operator: "between",
				Value:    []int{9, 21}, // 9 AM to 9 PM
			},
		},
		actions: []compliance.Action{
			{
				Type:   compliance.ActionRequireConsent,
				Params: nil,
			},
			{
				Type:   compliance.ActionLog,
				Params: nil,
			},
		},
		geography: compliance.GeographicScope{
			Countries: []string{"US"},
		},
		timeWindows: []compliance.TimeWindow{
			{
				StartHour: 9,
				EndHour:   21,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
				Timezone:  "America/New_York",
			},
		},
	}
}

// WithID sets the rule ID
func (b *ComplianceRuleBuilder) WithID(id uuid.UUID) *ComplianceRuleBuilder {
	b.id = id
	return b
}

// WithType sets the rule type
func (b *ComplianceRuleBuilder) WithType(ruleType compliance.RuleType) *ComplianceRuleBuilder {
	b.ruleType = ruleType
	return b
}

// WithName sets the rule name
func (b *ComplianceRuleBuilder) WithName(name string) *ComplianceRuleBuilder {
	b.name = name
	return b
}

// WithStatus sets the rule status
func (b *ComplianceRuleBuilder) WithStatus(status compliance.RuleStatus) *ComplianceRuleBuilder {
	b.status = status
	return b
}

// WithConditions sets the rule conditions
func (b *ComplianceRuleBuilder) WithConditions(conditions []compliance.Condition) *ComplianceRuleBuilder {
	b.conditions = conditions
	return b
}

// WithActions sets the rule actions
func (b *ComplianceRuleBuilder) WithActions(actions []compliance.Action) *ComplianceRuleBuilder {
	b.actions = actions
	return b
}

// WithGeography sets the geographic scope
func (b *ComplianceRuleBuilder) WithGeography(geography compliance.GeographicScope) *ComplianceRuleBuilder {
	b.geography = geography
	return b
}

// WithTimeWindows sets the time windows
func (b *ComplianceRuleBuilder) WithTimeWindows(windows []compliance.TimeWindow) *ComplianceRuleBuilder {
	b.timeWindows = windows
	return b
}

// WithExpiration sets the expiration time
func (b *ComplianceRuleBuilder) WithExpiration(expiresAt time.Time) *ComplianceRuleBuilder {
	b.expiresAt = &expiresAt
	return b
}

// WithPriority sets the rule priority
func (b *ComplianceRuleBuilder) WithPriority(priority int) *ComplianceRuleBuilder {
	b.priority = priority
	return b
}

// Build creates the ComplianceRule entity
func (b *ComplianceRuleBuilder) Build() *compliance.ComplianceRule {
	now := time.Now().UTC()
	return &compliance.ComplianceRule{
		ID:          b.id,
		Name:        b.name,
		Type:        b.ruleType,
		Status:      b.status,
		Priority:    b.priority,
		Conditions:  b.conditions,
		Actions:     b.actions,
		Geography:   b.geography,
		TimeWindows: b.timeWindows,
		Description: b.description,
		CreatedBy:   b.createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		EffectiveAt: b.effectiveAt,
		ExpiresAt:   b.expiresAt,
	}
}

// ConsentRecordBuilder builds test ConsentRecord entities
type ConsentRecordBuilder struct {
	t               *testing.T
	id              uuid.UUID
	phoneNumber     string
	consentType     compliance.ConsentType
	status          compliance.ConsentStatus
	source          string
	ipAddress       string
	userAgent       string
	optInTimestamp  time.Time
	optOutTimestamp *time.Time
	expiresAt       *time.Time
}

// NewConsentRecordBuilder creates a new ConsentRecordBuilder
func NewConsentRecordBuilder(t *testing.T) *ConsentRecordBuilder {
	t.Helper()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	
	now := time.Now().UTC()
	return &ConsentRecordBuilder{
		t:              t,
		id:             id,
		phoneNumber:    "+15551234567",
		consentType:    compliance.ConsentTypeTCPA,
		status:         compliance.ConsentStatusActive,
		source:         "web_form",
		ipAddress:      "192.168.1.100",
		userAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		optInTimestamp: now,
		expiresAt:      nil, // No expiration by default
	}
}

// WithPhoneNumber sets the phone number
func (b *ConsentRecordBuilder) WithPhoneNumber(phone string) *ConsentRecordBuilder {
	b.phoneNumber = phone
	return b
}

// WithConsentType sets the consent type
func (b *ConsentRecordBuilder) WithConsentType(consentType compliance.ConsentType) *ConsentRecordBuilder {
	b.consentType = consentType
	return b
}

// WithStatus sets the consent status
func (b *ConsentRecordBuilder) WithStatus(status compliance.ConsentStatus) *ConsentRecordBuilder {
	b.status = status
	return b
}

// WithExpiration sets the expiration time
func (b *ConsentRecordBuilder) WithExpiration(duration time.Duration) *ConsentRecordBuilder {
	expires := time.Now().UTC().Add(duration)
	b.expiresAt = &expires
	return b
}

// Build creates the ConsentRecord entity
func (b *ConsentRecordBuilder) Build() *compliance.ConsentRecord {
	now := time.Now().UTC()
	return &compliance.ConsentRecord{
		ID:              b.id,
		PhoneNumber:     b.phoneNumber,
		ConsentType:     b.consentType,
		Status:          b.status,
		Source:          b.source,
		IPAddress:       b.ipAddress,
		UserAgent:       b.userAgent,
		OptInTimestamp:  b.optInTimestamp,
		OptOutTimestamp: b.optOutTimestamp,
		ExpiresAt:       b.expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// ComplianceScenarios provides common compliance test scenarios
type ComplianceScenarios struct {
	t *testing.T
}

// NewComplianceScenarios creates a new ComplianceScenarios helper
func NewComplianceScenarios(t *testing.T) *ComplianceScenarios {
	t.Helper()
	return &ComplianceScenarios{t: t}
}

// TCPATimeRule creates a TCPA time restriction rule
func (cs *ComplianceScenarios) TCPATimeRule() *compliance.ComplianceRule {
	return NewComplianceRuleBuilder(cs.t).
		WithName("TCPA Time Restrictions").
		WithType(compliance.RuleTypeTCPA).
		WithTimeWindows([]compliance.TimeWindow{
			{
				StartHour: 8,
				EndHour:   21,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
				Timezone:  "America/New_York",
			},
		}).
		WithConditions([]compliance.Condition{
			{
				Field:    "time_of_day",
				Operator: "between",
				Value:    []int{8, 21},
			},
			{
				Field:    "consent_status",
				Operator: "equals",
				Value:    "active",
			},
		}).
		WithActions([]compliance.Action{
			{Type: compliance.ActionBlock, Params: nil},
			{Type: compliance.ActionRequireConsent, Params: nil},
			{Type: compliance.ActionLog, Params: nil},
		}).
		WithGeography(compliance.GeographicScope{
			Countries: []string{"US"},
		}).
		WithPriority(1000). // High priority
		Build()
}

// DNCRule creates a Do Not Call list rule
func (cs *ComplianceScenarios) DNCRule() *compliance.ComplianceRule {
	return NewComplianceRuleBuilder(cs.t).
		WithName("National DNC Registry").
		WithType(compliance.RuleTypeDNC).
		WithConditions([]compliance.Condition{
			{
				Field:    "dnc_list_check",
				Operator: "in",
				Value:    []string{"national", "state", "internal"},
			},
		}).
		WithActions([]compliance.Action{
			{Type: compliance.ActionBlock, Params: nil},
			{Type: compliance.ActionLog, Params: map[string]interface{}{"severity": "high"}},
		}).
		WithPriority(2000). // Highest priority
		Build()
}

// StateSpecificRule creates a state-specific compliance rule
func (cs *ComplianceScenarios) StateSpecificRule(state string) *compliance.ComplianceRule {
	return NewComplianceRuleBuilder(cs.t).
		WithName(state + " State Regulations").
		WithType(compliance.RuleTypeCustom).
		WithGeography(compliance.GeographicScope{
			States: []string{state},
		}).
		WithTimeWindows([]compliance.TimeWindow{
			{
				StartHour: 9,
				EndHour:   20,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
				Timezone:  "America/Los_Angeles",
			},
		}).
		WithConditions([]compliance.Condition{
			{
				Field:    "call_count_daily",
				Operator: "less_than",
				Value:    3,
			},
			{
				Field:    "last_call_hours",
				Operator: "greater_than",
				Value:    24,
			},
		}).
		WithActions([]compliance.Action{
			{Type: compliance.ActionBlock, Params: nil},
			{Type: compliance.ActionRequireConsent, Params: nil},
			{Type: compliance.ActionLog, Params: nil},
		}).
		Build()
}

// GDPRRule creates a GDPR compliance rule
func (cs *ComplianceScenarios) GDPRRule() *compliance.ComplianceRule {
	return NewComplianceRuleBuilder(cs.t).
		WithName("GDPR Data Protection").
		WithType(compliance.RuleTypeGDPR).
		WithGeography(compliance.GeographicScope{
			Countries: []string{"GB", "FR", "DE", "IT", "ES"},
		}).
		WithConditions([]compliance.Condition{
			{
				Field:    "explicit_consent",
				Operator: "equals",
				Value:    true,
			},
			{
				Field:    "data_retention_days",
				Operator: "less_than",
				Value:    90,
			},
		}).
		WithActions([]compliance.Action{
			{Type: compliance.ActionBlock, Params: nil},
			{Type: compliance.ActionRequireConsent, Params: nil},
			{Type: compliance.ActionLog, Params: map[string]interface{}{"gdpr": true}},
		}).
		Build()
}

// ExpressConsent creates an express consent record
func (cs *ComplianceScenarios) ExpressConsent(phoneNumber string) *compliance.ConsentRecord {
	return NewConsentRecordBuilder(cs.t).
		WithPhoneNumber(phoneNumber).
		WithConsentType(compliance.ConsentTypeExpress).
		WithExpiration(365 * 24 * time.Hour). // 1 year
		Build()
}

// RevokedConsent creates a revoked consent record
func (cs *ComplianceScenarios) RevokedConsent(phoneNumber string) *compliance.ConsentRecord {
	return NewConsentRecordBuilder(cs.t).
		WithPhoneNumber(phoneNumber).
		WithStatus(compliance.ConsentStatusRevoked).
		Build()
}

// ViolationRecord creates a compliance violation record
func (cs *ComplianceScenarios) ViolationRecord(callID, accountID, ruleID uuid.UUID) *compliance.ComplianceViolation {
	return &compliance.ComplianceViolation{
		ID:            uuid.New(),
		CallID:        callID,
		AccountID:     accountID,
		RuleID:        ruleID,
		ViolationType: compliance.ViolationTCPA,
		Severity:      compliance.SeverityHigh,
		Description:   "Call attempted outside allowed hours",
		Resolved:      false,
		CreatedAt:     time.Now().UTC(),
	}
}