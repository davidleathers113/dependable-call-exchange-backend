package compliance

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ComplianceRule struct {
	ID       uuid.UUID  `json:"id"`
	Name     string     `json:"name"`
	Type     RuleType   `json:"type"`
	Status   RuleStatus `json:"status"`
	Priority int        `json:"priority"`

	// Rule definition
	Conditions []Condition `json:"conditions"`
	Actions    []Action    `json:"actions"`

	// Geographic scope
	Geography GeographicScope `json:"geography"`

	// Time restrictions
	TimeWindows []TimeWindow `json:"time_windows"`

	// Metadata
	Description string     `json:"description"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	EffectiveAt time.Time  `json:"effective_at"`
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
	ID            uuid.UUID     `json:"id"`
	CallID        uuid.UUID     `json:"call_id"`
	AccountID     uuid.UUID     `json:"account_id"`
	RuleID        uuid.UUID     `json:"rule_id"`
	ViolationType ViolationType `json:"violation_type"`
	Severity      Severity      `json:"severity"`
	Description   string        `json:"description"`
	Resolved      bool          `json:"resolved"`
	ResolvedBy    *uuid.UUID    `json:"resolved_by,omitempty"`
	ResolvedAt    *time.Time    `json:"resolved_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
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
	ID              uuid.UUID     `json:"id"`
	PhoneNumber     string        `json:"phone_number"`
	ConsentType     ConsentType   `json:"consent_type"`
	Status          ConsentStatus `json:"status"`
	Source          string        `json:"source"`
	IPAddress       string        `json:"ip_address"`
	UserAgent       string        `json:"user_agent"`
	OptInTimestamp  time.Time     `json:"opt_in_timestamp"`
	OptOutTimestamp *time.Time    `json:"opt_out_timestamp,omitempty"`
	ExpiresAt       *time.Time    `json:"expires_at,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

type ConsentType int

const (
	ConsentTypeExpress ConsentType = iota
	ConsentTypeImplied
	ConsentTypePriorBusiness
)

func (t ConsentType) String() string {
	switch t {
	case ConsentTypeExpress:
		return "express"
	case ConsentTypeImplied:
		return "implied"
	case ConsentTypePriorBusiness:
		return "prior_business"
	default:
		return "unknown"
	}
}

type ConsentStatus int

const (
	ConsentStatusActive ConsentStatus = iota
	ConsentStatusExpired
	ConsentStatusRevoked
	ConsentStatusPending
)

type ComplianceCheck struct {
	CallID        uuid.UUID             `json:"call_id"`
	PhoneNumber   string                `json:"phone_number"`
	CallerID      string                `json:"caller_id"`
	Geography     Location              `json:"geography"`
	TimeOfCall    time.Time             `json:"time_of_call"`
	ConsentStatus ConsentStatus         `json:"consent_status"`
	Violations    []ComplianceViolation `json:"violations"`
	Approved      bool                  `json:"approved"`
	Reason        string                `json:"reason,omitempty"`
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

// Activate activates a compliance rule
func (cr *ComplianceRule) Activate() error {
	if cr.Status == RuleStatusActive {
		return fmt.Errorf("rule is already active")
	}

	// Validate rule before activation
	if err := cr.Validate(); err != nil {
		return fmt.Errorf("cannot activate invalid rule: %w", err)
	}

	cr.Status = RuleStatusActive
	cr.UpdatedAt = time.Now()
	return nil
}

// Deactivate deactivates a compliance rule
func (cr *ComplianceRule) Deactivate() error {
	if cr.Status != RuleStatusActive {
		return fmt.Errorf("can only deactivate active rules")
	}

	cr.Status = RuleStatusInactive
	cr.UpdatedAt = time.Now()
	return nil
}

// IsActive returns true if the rule is active and within its effective period
func (cr *ComplianceRule) IsActive() bool {
	if cr.Status != RuleStatusActive {
		return false
	}

	now := time.Now()

	// Check if rule is effective yet
	if now.Before(cr.EffectiveAt) {
		return false
	}

	// Check if rule has expired
	if cr.ExpiresAt != nil && now.After(*cr.ExpiresAt) {
		return false
	}

	return true
}

// Validate validates the rule configuration
func (cr *ComplianceRule) Validate() error {
	if cr.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}

	if len(cr.Conditions) == 0 {
		return fmt.Errorf("rule must have at least one condition")
	}

	if len(cr.Actions) == 0 {
		return fmt.Errorf("rule must have at least one action")
	}

	// Validate conditions
	for i, condition := range cr.Conditions {
		if err := cr.validateCondition(condition); err != nil {
			return fmt.Errorf("invalid condition %d: %w", i, err)
		}
	}

	// Validate time windows
	for i, tw := range cr.TimeWindows {
		if err := cr.validateTimeWindow(tw); err != nil {
			return fmt.Errorf("invalid time window %d: %w", i, err)
		}
	}

	return nil
}

func (cr *ComplianceRule) validateCondition(condition Condition) error {
	if condition.Field == "" {
		return fmt.Errorf("condition field cannot be empty")
	}

	if condition.Operator == "" {
		return fmt.Errorf("condition operator cannot be empty")
	}

	// Validate operator
	validOps := []string{"equals", "not_equals", "contains", "not_contains", "greater_than", "less_than", "in", "not_in"}
	valid := false
	for _, op := range validOps {
		if condition.Operator == op {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid operator: %s", condition.Operator)
	}

	return nil
}

func (cr *ComplianceRule) validateTimeWindow(tw TimeWindow) error {
	if tw.StartHour < 0 || tw.StartHour > 23 {
		return fmt.Errorf("start hour must be between 0 and 23")
	}

	if tw.EndHour < 0 || tw.EndHour > 23 {
		return fmt.Errorf("end hour must be between 0 and 23")
	}

	if tw.Timezone == "" {
		return fmt.Errorf("timezone cannot be empty")
	}

	return nil
}

// EvaluateConditions evaluates all conditions against the provided data
func (cr *ComplianceRule) EvaluateConditions(data map[string]interface{}) (bool, error) {
	if !cr.IsActive() {
		return true, nil // Inactive rules always pass
	}

	for _, condition := range cr.Conditions {
		matches, err := cr.evaluateCondition(condition, data)
		if err != nil {
			return false, err
		}

		if !matches {
			return false, nil // Any condition failure means overall failure
		}
	}

	return true, nil
}

func (cr *ComplianceRule) evaluateCondition(condition Condition, data map[string]interface{}) (bool, error) {
	fieldValue, exists := data[condition.Field]
	if !exists {
		return false, fmt.Errorf("field %s not found in data", condition.Field)
	}

	switch condition.Operator {
	case "equals":
		return fieldValue == condition.Value, nil

	case "not_equals":
		return fieldValue != condition.Value, nil

	case "contains":
		fieldStr, ok := fieldValue.(string)
		valueStr, ok2 := condition.Value.(string)
		if !ok || !ok2 {
			return false, fmt.Errorf("contains operator requires string values")
		}
		return fmt.Sprintf("%v", fieldStr) == fmt.Sprintf("%v", valueStr), nil

	case "greater_than":
		fieldFloat, ok := fieldValue.(float64)
		valueFloat, ok2 := condition.Value.(float64)
		if !ok || !ok2 {
			return false, fmt.Errorf("greater_than operator requires numeric values")
		}
		return fieldFloat > valueFloat, nil

	case "less_than":
		fieldFloat, ok := fieldValue.(float64)
		valueFloat, ok2 := condition.Value.(float64)
		if !ok || !ok2 {
			return false, fmt.Errorf("less_than operator requires numeric values")
		}
		return fieldFloat < valueFloat, nil

	default:
		return false, fmt.Errorf("unsupported operator: %s", condition.Operator)
	}
}

// CheckTimeWindow validates if the current time is within allowed windows
func (cr *ComplianceRule) CheckTimeWindow(checkTime time.Time, timezone string) bool {
	if len(cr.TimeWindows) == 0 {
		return true // No time restrictions
	}

	for _, tw := range cr.TimeWindows {
		if cr.timeInWindow(checkTime, tw, timezone) {
			return true
		}
	}

	return false
}

func (cr *ComplianceRule) timeInWindow(checkTime time.Time, tw TimeWindow, timezone string) bool {
	// Convert to target timezone if specified
	if tw.Timezone != "" {
		timezone = tw.Timezone
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Fallback to UTC if timezone is invalid
		loc = time.UTC
	}

	localTime := checkTime.In(loc)
	hour := localTime.Hour()
	weekday := localTime.Weekday().String()

	// Check if current day is allowed
	if len(tw.Days) > 0 {
		dayAllowed := false
		for _, day := range tw.Days {
			if day == weekday {
				dayAllowed = true
				break
			}
		}
		if !dayAllowed {
			return false
		}
	}

	// Check hour range
	if tw.StartHour <= tw.EndHour {
		// Same day range (e.g., 9 AM to 5 PM)
		return hour >= tw.StartHour && hour <= tw.EndHour
	} else {
		// Overnight range (e.g., 10 PM to 6 AM)
		return hour >= tw.StartHour || hour <= tw.EndHour
	}
}

// CheckGeographicScope validates if the location is within the rule's scope
func (cr *ComplianceRule) CheckGeographicScope(location Location) bool {
	scope := cr.Geography

	// Check countries
	if len(scope.Countries) > 0 {
		found := false
		for _, country := range scope.Countries {
			if country == location.Country {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check states
	if len(scope.States) > 0 {
		found := false
		for _, state := range scope.States {
			if state == location.State {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check cities
	if len(scope.Cities) > 0 {
		found := false
		for _, city := range scope.Cities {
			if city == location.City {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check zip codes
	if len(scope.ZipCodes) > 0 {
		found := false
		for _, zip := range scope.ZipCodes {
			if zip == location.ZipCode {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// IsExpired checks if the consent record has expired
func (c *ConsentRecord) IsExpired() bool {
	if c.ExpiresAt == nil {
		return false // No expiration set
	}
	return time.Now().After(*c.ExpiresAt)
}

// IsActive returns true if consent is active and not expired
func (c *ConsentRecord) IsActive() bool {
	return c.Status == ConsentStatusActive && !c.IsExpired()
}

// Extend extends the consent expiration by the specified duration
func (c *ConsentRecord) Extend(duration time.Duration) error {
	if c.Status != ConsentStatusActive {
		return fmt.Errorf("can only extend active consent")
	}

	now := time.Now()
	newExpiry := now.Add(duration)
	c.ExpiresAt = &newExpiry
	c.UpdatedAt = now
	return nil
}

var (
	ErrComplianceViolation   = fmt.Errorf("compliance violation detected")
	ErrNoConsent             = fmt.Errorf("no valid consent found")
	ErrTimeRestriction       = fmt.Errorf("call not allowed at this time")
	ErrGeographicRestriction = fmt.Errorf("calls not allowed in this geography")
)
