package compliance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// TCPAComplianceValidator implements comprehensive TCPA compliance validation
type TCPAComplianceValidator struct {
	logger            *zap.Logger
	consentService    ConsentService
	auditService      AuditService
	geoService        GeolocationService
	complianceRepo    ComplianceRepository
	
	// TCPA configuration
	config            TCPAConfig
	stateRules        map[string]StateSpecificRules
	callingHours      CallingHoursConfig
	wirelessCarriers  map[string]CarrierInfo
}

// TCPAConfig holds TCPA compliance configuration
type TCPAConfig struct {
	StrictMode                bool          `json:"strict_mode"`
	RequireWrittenConsent     bool          `json:"require_written_consent"`
	RequireCallerIDMatch      bool          `json:"require_caller_id_match"`
	DefaultCallingHours       TimeWindow    `json:"default_calling_hours"`
	EmergencyOverride         bool          `json:"emergency_override"`
	GracePeriodMinutes        int           `json:"grace_period_minutes"`
	AutoDetectWireless        bool          `json:"auto_detect_wireless"`
	StateSpecificRulesEnabled bool          `json:"state_specific_rules_enabled"`
	ViolationCooldownMinutes  int           `json:"violation_cooldown_minutes"`
}

type CallingHoursConfig struct {
	DefaultStart    int                     `json:"default_start"`    // 8 AM
	DefaultEnd      int                     `json:"default_end"`      // 9 PM
	StateOverrides  map[string]TimeWindow   `json:"state_overrides"`
	HolidayRules    map[string]TimeWindow   `json:"holiday_rules"`
	TimezoneRules   map[string]TimeWindow   `json:"timezone_rules"`
}

type TimeWindow struct {
	StartHour int    `json:"start_hour"`
	EndHour   int    `json:"end_hour"`
	Timezone  string `json:"timezone"`
}

type StateSpecificRules struct {
	StateCode               string        `json:"state_code"`
	CallingHours           TimeWindow    `json:"calling_hours"`
	RequireWrittenConsent  bool          `json:"require_written_consent"`
	NoCallRegistry         bool          `json:"no_call_registry"`
	AdditionalRestrictions []string      `json:"additional_restrictions"`
	CooldownPeriodDays     int           `json:"cooldown_period_days"`
	MaxCallsPerDay         int           `json:"max_calls_per_day"`
	MaxCallsPerWeek        int           `json:"max_calls_per_week"`
}

type CarrierInfo struct {
	CarrierName string   `json:"carrier_name"`
	IsWireless  bool     `json:"is_wireless"`
	Prefixes    []string `json:"prefixes"`
	Region      string   `json:"region"`
}

// NewTCPAComplianceValidator creates a new TCPA validator
func NewTCPAComplianceValidator(
	logger *zap.Logger,
	consentService ConsentService,
	auditService AuditService,
	geoService GeolocationService,
	complianceRepo ComplianceRepository,
	config TCPAConfig,
) *TCPAComplianceValidator {
	validator := &TCPAComplianceValidator{
		logger:         logger,
		consentService: consentService,
		auditService:   auditService,
		geoService:     geoService,
		complianceRepo: complianceRepo,
		config:         config,
		stateRules:     initializeStateRules(),
		callingHours:   initializeCallingHours(),
		wirelessCarriers: initializeWirelessCarriers(),
	}
	
	return validator
}

// ValidateTimeRestrictions validates TCPA time restrictions (8 AM - 9 PM local time)
func (v *TCPAComplianceValidator) ValidateTimeRestrictions(ctx context.Context, req TimeValidationRequest) (*TimeValidationResult, error) {
	startTime := time.Now()
	defer func() {
		v.logger.Debug("TCPA time validation completed",
			zap.String("phone_number", req.PhoneNumber.String()),
			zap.Duration("duration", time.Since(startTime)),
		)
	}()

	// Get location if not provided
	location := req.Location
	if location == nil {
		var err error
		location, err = v.geoService.GetLocation(ctx, req.PhoneNumber)
		if err != nil {
			v.logger.Warn("Failed to get location for phone number",
				zap.String("phone_number", req.PhoneNumber.String()),
				zap.Error(err),
			)
			// Use default location/timezone
			location = &compliance.Location{
				Country:  "US",
				State:    "",
				Timezone: "America/New_York",
			}
		}
	}

	// Get timezone
	timezone := location.Timezone
	if timezone == "" {
		var err error
		timezone, err = v.geoService.GetTimezone(ctx, *location)
		if err != nil {
			v.logger.Warn("Failed to get timezone",
				zap.Any("location", location),
				zap.Error(err),
			)
			timezone = "America/New_York" // Default to Eastern
		}
	}

	// Convert to local time
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		v.logger.Error("Invalid timezone",
			zap.String("timezone", timezone),
			zap.Error(err),
		)
		loc = time.UTC
	}

	localTime := req.CallTime.In(loc)
	hour := localTime.Hour()

	// Get applicable calling hours
	callingHours := v.getCallingHours(location.State, req.CallType)
	
	// Check if within allowed hours
	allowed := v.isWithinCallingHours(hour, callingHours)
	
	result := &TimeValidationResult{
		Allowed:   allowed,
		LocalTime: localTime,
		Timezone:  timezone,
	}

	if !allowed {
		result.Reason = fmt.Sprintf("Call time %02d:%02d is outside allowed hours (%02d:00-%02d:00 %s)",
			localTime.Hour(), localTime.Minute(),
			callingHours.StartHour, callingHours.EndHour, timezone)
		
		// Calculate next allowed time
		nextAllowed := v.calculateNextAllowedTime(localTime, callingHours)
		result.NextAllowed = &nextAllowed
	}

	// Log audit event
	auditEvent := ComplianceAuditEvent{
		EventType:   "tcpa_time_validation",
		CallID:      uuid.New(), // Generate for tracking
		PhoneNumber: req.PhoneNumber,
		Regulation:  RegulationTCPA,
		Result:      map[string]interface{}{"allowed": allowed, "reason": result.Reason},
		Timestamp:   time.Now(),
		ActorID:     uuid.New(), // System actor
		Metadata: map[string]interface{}{
			"local_time":     localTime,
			"timezone":       timezone,
			"calling_hours":  callingHours,
			"call_type":      req.CallType,
		},
	}

	if err := v.auditService.LogComplianceEvent(ctx, auditEvent); err != nil {
		v.logger.Error("Failed to log TCPA time validation audit event",
			zap.Error(err),
		)
	}

	return result, nil
}

// ValidateWirelessConsent validates consent for wireless numbers (requires express written consent)
func (v *TCPAComplianceValidator) ValidateWirelessConsent(ctx context.Context, phoneNumber values.PhoneNumber) (*ConsentValidationResult, error) {
	startTime := time.Now()
	defer func() {
		v.logger.Debug("TCPA wireless consent validation completed",
			zap.String("phone_number", phoneNumber.String()),
			zap.Duration("duration", time.Since(startTime)),
		)
	}()

	// Check if number is wireless
	isWireless := v.isWirelessNumber(phoneNumber)
	
	// Get consent status
	consentStatus, err := v.consentService.CheckConsent(ctx, phoneNumber, "tcpa_express_written")
	if err != nil {
		v.logger.Error("Failed to check consent status",
			zap.String("phone_number", phoneNumber.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to check consent: %w", err)
	}

	result := &ConsentValidationResult{
		IsWireless:     isWireless,
		HasConsent:     consentStatus.HasConsent,
		ConsentType:    consentStatus.ConsentType,
		ConsentDate:    consentStatus.GrantedAt,
		ExpirationDate: consentStatus.ExpiresAt,
		Source:         consentStatus.Source,
	}

	if isWireless {
		result.RequiredType = "express_written"
		
		// For wireless numbers, require express written consent
		if !consentStatus.HasConsent {
			result.HasConsent = false
		} else if consentStatus.ConsentType != "express_written" && consentStatus.ConsentType != "tcpa_express_written" {
			result.HasConsent = false
		}
	} else {
		result.RequiredType = "express_or_implied"
		// For landlines, implied consent may be sufficient
	}

	// Log audit event
	auditEvent := ComplianceAuditEvent{
		EventType:   "tcpa_wireless_consent_validation",
		CallID:      uuid.New(),
		PhoneNumber: phoneNumber,
		Regulation:  RegulationTCPA,
		Result:      fmt.Sprintf("has_consent=%t,is_wireless=%t", result.HasConsent, result.IsWireless),
		Timestamp:   time.Now(),
		ActorID:     uuid.New(),
		Metadata: map[string]interface{}{
			"is_wireless":      isWireless,
			"has_consent":      result.HasConsent,
			"consent_type":     result.ConsentType,
			"required_type":    result.RequiredType,
			"consent_source":   result.Source,
		},
	}

	if err := v.auditService.LogComplianceEvent(ctx, auditEvent); err != nil {
		v.logger.Error("Failed to log wireless consent validation audit event",
			zap.Error(err),
		)
	}

	return result, nil
}

// CheckStateSpecificRules validates state-specific TCPA requirements
func (v *TCPAComplianceValidator) CheckStateSpecificRules(ctx context.Context, location compliance.Location, callType CallType) (*StateComplianceResult, error) {
	if !v.config.StateSpecificRulesEnabled {
		return &StateComplianceResult{Compliant: true}, nil
	}

	stateCode := strings.ToUpper(location.State)
	rules, exists := v.stateRules[stateCode]
	if !exists {
		// No specific rules for this state
		return &StateComplianceResult{Compliant: true}, nil
	}

	result := &StateComplianceResult{
		Compliant:       true,
		ApplicableRules: []string{},
		Restrictions:    []string{},
		Requirements:    []string{},
	}

	// Check state-specific requirements
	result.ApplicableRules = append(result.ApplicableRules, fmt.Sprintf("State rules for %s", stateCode))

	// Check calling hours
	if rules.CallingHours.StartHour != 0 || rules.CallingHours.EndHour != 0 {
		result.Requirements = append(result.Requirements,
			fmt.Sprintf("Calling hours: %02d:00-%02d:00 %s",
				rules.CallingHours.StartHour,
				rules.CallingHours.EndHour,
				rules.CallingHours.Timezone))
	}

	// Check consent requirements
	if rules.RequireWrittenConsent {
		result.Requirements = append(result.Requirements, "Written consent required")
	}

	// Check no-call registry
	if rules.NoCallRegistry {
		result.Requirements = append(result.Requirements, "Check state no-call registry")
	}

	// Add additional restrictions
	for _, restriction := range rules.AdditionalRestrictions {
		result.Restrictions = append(result.Restrictions, restriction)
	}

	// Check call limits
	if rules.MaxCallsPerDay > 0 {
		result.Restrictions = append(result.Restrictions,
			fmt.Sprintf("Maximum %d calls per day", rules.MaxCallsPerDay))
	}

	if rules.MaxCallsPerWeek > 0 {
		result.Restrictions = append(result.Restrictions,
			fmt.Sprintf("Maximum %d calls per week", rules.MaxCallsPerWeek))
	}

	// Log audit event
	auditEvent := ComplianceAuditEvent{
		EventType:   "tcpa_state_rules_check",
		CallID:      uuid.New(),
		PhoneNumber: values.PhoneNumber{}, // No specific phone number for state rules
		Regulation:  RegulationTCPA,
		Result:      fmt.Sprintf("compliant=%t,state=%s", result.Compliant, stateCode),
		Timestamp:   time.Now(),
		ActorID:     uuid.New(),
		Metadata: map[string]interface{}{
			"state_code":        stateCode,
			"applicable_rules":  result.ApplicableRules,
			"restrictions":      result.Restrictions,
			"requirements":      result.Requirements,
			"call_type":         callType,
		},
	}

	if err := v.auditService.LogComplianceEvent(ctx, auditEvent); err != nil {
		v.logger.Error("Failed to log state rules check audit event",
			zap.Error(err),
		)
	}

	return result, nil
}

// ValidateCallerID validates that Caller ID matches the actual calling number
func (v *TCPAComplianceValidator) ValidateCallerID(ctx context.Context, callerID, actualNumber values.PhoneNumber) error {
	if !v.config.RequireCallerIDMatch {
		return nil
	}

	if callerID.String() != actualNumber.String() {
		violation := &compliance.ComplianceViolation{
			ID:            uuid.New(),
			CallID:        uuid.New(),
			ViolationType: compliance.ViolationTCPA,
			Severity:      compliance.SeverityHigh,
			Description:   fmt.Sprintf("Caller ID spoofing detected: displayed %s, actual %s", callerID.String(), actualNumber.String()),
			Resolved:      false,
			CreatedAt:     time.Now(),
		}

		if err := v.complianceRepo.SaveViolation(ctx, violation); err != nil {
			v.logger.Error("Failed to save caller ID violation",
				zap.Error(err),
			)
		}

		// Log violation audit event
		violationEvent := ViolationAuditEvent{
			EventType:     "tcpa_caller_id_violation",
			ViolationID:   violation.ID,
			CallID:        violation.CallID,
			ViolationType: violation.ViolationType,
			Severity:      violation.Severity,
			Description:   violation.Description,
			Timestamp:     time.Now(),
			DetectedBy:    "tcpa_validator",
			Metadata: map[string]interface{}{
				"caller_id":      callerID.String(),
				"actual_number":  actualNumber.String(),
				"violation_type": "caller_id_spoofing",
			},
		}

		if err := v.auditService.LogViolation(ctx, violationEvent); err != nil {
			v.logger.Error("Failed to log caller ID violation audit event",
				zap.Error(err),
			)
		}

		return fmt.Errorf("caller ID validation failed: spoofing detected")
	}

	return nil
}

// Helper methods

func (v *TCPAComplianceValidator) getCallingHours(state string, callType CallType) TimeWindow {
	// Check for state-specific override
	if stateRules, exists := v.stateRules[strings.ToUpper(state)]; exists {
		if stateRules.CallingHours.StartHour != 0 || stateRules.CallingHours.EndHour != 0 {
			return stateRules.CallingHours
		}
	}

	// Check for calling hours override in config
	if override, exists := v.callingHours.StateOverrides[strings.ToUpper(state)]; exists {
		return override
	}

	// Return default calling hours
	return v.config.DefaultCallingHours
}

func (v *TCPAComplianceValidator) isWithinCallingHours(hour int, callingHours TimeWindow) bool {
	start := callingHours.StartHour
	end := callingHours.EndHour

	if start <= end {
		// Same day (e.g., 8 AM to 9 PM)
		return hour >= start && hour <= end
	} else {
		// Overnight (e.g., 10 PM to 6 AM)
		return hour >= start || hour <= end
	}
}

func (v *TCPAComplianceValidator) calculateNextAllowedTime(localTime time.Time, callingHours TimeWindow) time.Time {
	year, month, day := localTime.Date()
	
	// Try today first
	todayStart := time.Date(year, month, day, callingHours.StartHour, 0, 0, 0, localTime.Location())
	if todayStart.After(localTime) {
		return todayStart
	}
	
	// Try tomorrow
	tomorrow := time.Date(year, month, day+1, callingHours.StartHour, 0, 0, 0, localTime.Location())
	return tomorrow
}

func (v *TCPAComplianceValidator) isWirelessNumber(phoneNumber values.PhoneNumber) bool {
	if !v.config.AutoDetectWireless {
		return false
	}

	// For US numbers, check area code and exchange patterns
	if phoneNumber.IsUS() {
		areaCode := phoneNumber.AreaCode()
		exchange := phoneNumber.Exchange()
		
		// Check against known wireless patterns
		for _, carrier := range v.wirelessCarriers {
			if carrier.IsWireless {
				for _, prefix := range carrier.Prefixes {
					if strings.HasPrefix(areaCode+exchange, prefix) {
						return true
					}
				}
			}
		}
	}

	// For international numbers, we would need a more sophisticated lookup
	// For now, assume they could be wireless
	return !phoneNumber.IsUS()
}

// Initialization functions

func initializeStateRules() map[string]StateSpecificRules {
	return map[string]StateSpecificRules{
		"CA": {
			StateCode:              "CA",
			CallingHours:           TimeWindow{StartHour: 8, EndHour: 20, Timezone: "America/Los_Angeles"},
			RequireWrittenConsent:  true,
			NoCallRegistry:         true,
			AdditionalRestrictions: []string{"CCPA compliance required", "Do not call registry check mandatory"},
			CooldownPeriodDays:     30,
			MaxCallsPerDay:         3,
			MaxCallsPerWeek:        10,
		},
		"NY": {
			StateCode:              "NY",
			CallingHours:           TimeWindow{StartHour: 8, EndHour: 21, Timezone: "America/New_York"},
			RequireWrittenConsent:  false,
			NoCallRegistry:         true,
			AdditionalRestrictions: []string{"State no-call registry check required"},
			CooldownPeriodDays:     14,
			MaxCallsPerDay:         5,
			MaxCallsPerWeek:        15,
		},
		"FL": {
			StateCode:              "FL",
			CallingHours:           TimeWindow{StartHour: 8, EndHour: 21, Timezone: "America/New_York"},
			RequireWrittenConsent:  false,
			NoCallRegistry:         true,
			AdditionalRestrictions: []string{"Florida Telemarketing Act compliance"},
			CooldownPeriodDays:     7,
			MaxCallsPerDay:         4,
			MaxCallsPerWeek:        12,
		},
		"TX": {
			StateCode:              "TX",
			CallingHours:           TimeWindow{StartHour: 8, EndHour: 21, Timezone: "America/Chicago"},
			RequireWrittenConsent:  false,
			NoCallRegistry:         true,
			AdditionalRestrictions: []string{"Texas no-call list compliance"},
			CooldownPeriodDays:     21,
			MaxCallsPerDay:         6,
			MaxCallsPerWeek:        18,
		},
	}
}

func initializeCallingHours() CallingHoursConfig {
	return CallingHoursConfig{
		DefaultStart: 8,  // 8 AM
		DefaultEnd:   21, // 9 PM
		StateOverrides: map[string]TimeWindow{
			"HI": {StartHour: 8, EndHour: 20, Timezone: "Pacific/Honolulu"},
			"AK": {StartHour: 8, EndHour: 20, Timezone: "America/Anchorage"},
		},
		HolidayRules: map[string]TimeWindow{
			"christmas": {StartHour: 12, EndHour: 18, Timezone: ""},
			"thanksgiving": {StartHour: 12, EndHour: 18, Timezone: ""},
		},
		TimezoneRules: map[string]TimeWindow{
			"Pacific/Honolulu": {StartHour: 8, EndHour: 20, Timezone: "Pacific/Honolulu"},
			"America/Anchorage": {StartHour: 8, EndHour: 20, Timezone: "America/Anchorage"},
		},
	}
}

func initializeWirelessCarriers() map[string]CarrierInfo {
	return map[string]CarrierInfo{
		"verizon": {
			CarrierName: "Verizon Wireless",
			IsWireless:  true,
			Prefixes:    []string{"201", "202", "203", "551", "732", "848", "862", "908", "973"},
			Region:      "US",
		},
		"att": {
			CarrierName: "AT&T Mobility",
			IsWireless:  true,
			Prefixes:    []string{"214", "469", "817", "940", "972", "430", "903", "945", "979"},
			Region:      "US",
		},
		"tmobile": {
			CarrierName: "T-Mobile US",
			IsWireless:  true,
			Prefixes:    []string{"206", "253", "360", "425", "509", "564", "235", "639", "672"},
			Region:      "US",
		},
		"sprint": {
			CarrierName: "Sprint Corporation",
			IsWireless:  true,
			Prefixes:    []string{"316", "620", "785", "913", "816", "417", "636", "660", "573"},
			Region:      "US",
		},
	}
}

// DefaultTCPAConfig returns a default TCPA configuration
func DefaultTCPAConfig() TCPAConfig {
	return TCPAConfig{
		StrictMode:                true,
		RequireWrittenConsent:     true,
		RequireCallerIDMatch:      true,
		DefaultCallingHours:       TimeWindow{StartHour: 8, EndHour: 21, Timezone: "America/New_York"},
		EmergencyOverride:         true,
		GracePeriodMinutes:        5,
		AutoDetectWireless:        true,
		StateSpecificRulesEnabled: true,
		ViolationCooldownMinutes:  15,
	}
}