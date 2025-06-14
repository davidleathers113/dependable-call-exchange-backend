package consent

import (
	"context"
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"go.uber.org/zap"
)

// ComplianceAdapter implements the ComplianceChecker interface
// It integrates with the existing compliance domain to validate consent requirements
type ComplianceAdapter struct {
	logger         *zap.Logger
	ruleRepository compliance.RuleRepository
}

// NewComplianceAdapter creates a new compliance adapter
func NewComplianceAdapter(logger *zap.Logger, ruleRepo compliance.RuleRepository) ComplianceChecker {
	return &ComplianceAdapter{
		logger:         logger,
		ruleRepository: ruleRepo,
	}
}

// CheckConsentRequirements checks if consent is required for a phone number
func (ca *ComplianceAdapter) CheckConsentRequirements(ctx context.Context, phoneNumber string, consentType consent.Type) (*compliance.ComplianceRule, error) {
	// Validate phone number
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE", "invalid phone number format").WithCause(err)
	}

	// Get applicable compliance rules based on phone number geography
	geography := ca.getPhoneGeography(phone)
	
	// Find rules that apply to this consent type and geography
	rules, err := ca.ruleRepository.FindByTypeAndGeography(ctx, ca.mapConsentTypeToRuleType(consentType), geography)
	if err != nil {
		return nil, errors.NewInternalError("failed to find compliance rules").WithCause(err)
	}

	// Find the highest priority rule that requires consent
	var applicableRule *compliance.ComplianceRule
	highestPriority := -1

	for _, rule := range rules {
		if rule.Status != compliance.RuleStatusActive {
			continue
		}

		// Check if rule is within effective dates
		if !ca.isRuleEffective(rule) {
			continue
		}

		// Check if rule has consent requirement action
		if ca.hasConsentRequirement(rule) && rule.Priority > highestPriority {
			applicableRule = rule
			highestPriority = rule.Priority
		}
	}

	return applicableRule, nil
}

// ValidateConsentGrant validates a consent grant request against compliance rules
func (ca *ComplianceAdapter) ValidateConsentGrant(ctx context.Context, req GrantConsentRequest) error {
	logger := ca.logger.With(
		zap.String("consent_type", req.ConsentType.String()),
		zap.String("channel", req.Channel.String()),
	)

	// Validate phone number if provided
	if req.PhoneNumber != "" {
		phone, err := values.NewPhoneNumber(req.PhoneNumber)
		if err != nil {
			return errors.NewValidationError("INVALID_PHONE", "invalid phone number format").WithCause(err)
		}

		// Check for DNC rules
		if err := ca.checkDNCCompliance(ctx, phone); err != nil {
			logger.Error("DNC compliance check failed", zap.Error(err))
			return err
		}
	}

	// Validate consent type against channel
	if !ca.isValidChannelForType(req.ConsentType, req.Channel) {
		return errors.NewValidationError("INVALID_CHANNEL", 
			fmt.Sprintf("channel %s is not valid for consent type %s", req.Channel, req.ConsentType))
	}

	// Check time-based restrictions
	if err := ca.checkTimeRestrictions(ctx, req); err != nil {
		logger.Error("time restriction check failed", zap.Error(err))
		return err
	}

	// Validate based on consent type
	switch req.ConsentType {
	case consent.TypeTCPA:
		return ca.validateTCPAConsent(ctx, req)
	case consent.TypeGDPR:
		return ca.validateGDPRConsent(ctx, req)
	case consent.TypeCCPA:
		return ca.validateCCPAConsent(ctx, req)
	case consent.TypeMarketing:
		return ca.validateMarketingConsent(ctx, req)
	default:
		return nil
	}
}

// Helper methods

func (ca *ComplianceAdapter) getPhoneGeography(phone values.PhoneNumber) compliance.GeographicScope {
	// Extract country code and area code from phone number
	// This is a simplified implementation - in production, use a proper phone number library
	phoneStr := phone.String()
	
	// US numbers start with +1
	if len(phoneStr) >= 2 && phoneStr[:2] == "+1" {
		// Extract state from area code (simplified)
		if len(phoneStr) >= 5 {
			areaCode := phoneStr[2:5]
			state := ca.getStateFromAreaCode(areaCode)
			return compliance.GeographicScope{
				Countries: []string{"US"},
				States:    []string{state},
			}
		}
	}

	// Default to US for now
	return compliance.GeographicScope{
		Countries: []string{"US"},
	}
}

func (ca *ComplianceAdapter) getStateFromAreaCode(areaCode string) string {
	// Simplified area code to state mapping
	// In production, use a comprehensive mapping
	areaCodeMap := map[string]string{
		"212": "NY", "213": "CA", "214": "TX", "215": "PA",
		"312": "IL", "313": "MI", "314": "MO", "404": "GA",
		"415": "CA", "512": "TX", "617": "MA", "702": "NV",
		// Add more mappings as needed
	}

	if state, ok := areaCodeMap[areaCode]; ok {
		return state
	}
	return "Unknown"
}

func (ca *ComplianceAdapter) mapConsentTypeToRuleType(consentType consent.Type) compliance.RuleType {
	switch consentType {
	case consent.TypeTCPA:
		return compliance.RuleTypeTCPA
	case consent.TypeGDPR:
		return compliance.RuleTypeGDPR
	case consent.TypeCCPA:
		return compliance.RuleTypeCCPA
	case consent.TypeDNC:
		return compliance.RuleTypeDNC
	default:
		return compliance.RuleTypeCustom
	}
}

func (ca *ComplianceAdapter) isRuleEffective(rule *compliance.ComplianceRule) bool {
	now := time.Now()
	
	// Check if rule is effective yet
	if now.Before(rule.EffectiveAt) {
		return false
	}

	// Check if rule has expired
	if rule.ExpiresAt != nil && now.After(*rule.ExpiresAt) {
		return false
	}

	return true
}

func (ca *ComplianceAdapter) hasConsentRequirement(rule *compliance.ComplianceRule) bool {
	for _, action := range rule.Actions {
		if action.Type == compliance.ActionRequireConsent {
			return true
		}
	}
	return false
}

func (ca *ComplianceAdapter) checkDNCCompliance(ctx context.Context, phone values.PhoneNumber) error {
	// Check if phone is on DNC list
	rules, err := ca.ruleRepository.FindByTypeAndGeography(ctx, compliance.RuleTypeDNC, compliance.GeographicScope{})
	if err != nil {
		return errors.NewInternalError("failed to check DNC compliance").WithCause(err)
	}

	for _, rule := range rules {
		if rule.Status != compliance.RuleStatusActive {
			continue
		}

		// Check if phone matches DNC conditions
		for _, condition := range rule.Conditions {
			if condition.Field == "phone_number" && condition.Operator == "in_list" {
				// In production, this would check against actual DNC registry
				// For now, we'll assume compliance
				continue
			}
		}
	}

	return nil
}

func (ca *ComplianceAdapter) isValidChannelForType(consentType consent.Type, channel consent.Channel) bool {
	// Define valid channels for each consent type
	validChannels := map[consent.Type][]consent.Channel{
		consent.TypeTCPA: {
			consent.ChannelWeb,
			consent.ChannelSMS,
			consent.ChannelVoice,
			consent.ChannelAPI,
		},
		consent.TypeGDPR: {
			consent.ChannelWeb,
			consent.ChannelEmail,
			consent.ChannelAPI,
		},
		consent.TypeCCPA: {
			consent.ChannelWeb,
			consent.ChannelEmail,
			consent.ChannelAPI,
		},
		consent.TypeMarketing: {
			consent.ChannelWeb,
			consent.ChannelSMS,
			consent.ChannelEmail,
			consent.ChannelAPI,
		},
		consent.TypeDNC: {
			consent.ChannelWeb,
			consent.ChannelVoice,
			consent.ChannelAPI,
		},
	}

	channels, ok := validChannels[consentType]
	if !ok {
		return true // Allow all channels for unknown types
	}

	for _, validChannel := range channels {
		if channel == validChannel {
			return true
		}
	}

	return false
}

func (ca *ComplianceAdapter) checkTimeRestrictions(ctx context.Context, req GrantConsentRequest) error {
	// For TCPA, check calling time restrictions
	if req.ConsentType == consent.TypeTCPA {
		now := time.Now()
		hour := now.Hour()

		// TCPA restricts calls between 9 PM and 8 AM local time
		if hour >= 21 || hour < 8 {
			ca.logger.Warn("consent grant attempted outside TCPA hours",
				zap.Int("hour", hour),
				zap.String("consent_type", req.ConsentType.String()),
			)
			// Note: We allow consent to be granted outside hours, 
			// but log it for compliance reporting
		}
	}

	return nil
}

func (ca *ComplianceAdapter) validateTCPAConsent(ctx context.Context, req GrantConsentRequest) error {
	// TCPA-specific validation
	if req.Channel == consent.ChannelVoice || req.Channel == consent.ChannelSMS {
		// Require explicit opt-in for voice and SMS
		if req.Preferences == nil || req.Preferences["explicit_opt_in"] != "true" {
			return errors.NewValidationError("TCPA_EXPLICIT_CONSENT_REQUIRED",
				"TCPA requires explicit opt-in for voice and SMS communications")
		}
	}

	return nil
}

func (ca *ComplianceAdapter) validateGDPRConsent(ctx context.Context, req GrantConsentRequest) error {
	// GDPR-specific validation
	
	// Require purpose specification
	if req.Preferences == nil || req.Preferences["purpose"] == "" {
		return errors.NewValidationError("GDPR_PURPOSE_REQUIRED",
			"GDPR requires explicit purpose for data processing")
	}

	// Require data controller identification
	if req.Preferences["data_controller"] == "" {
		return errors.NewValidationError("GDPR_CONTROLLER_REQUIRED",
			"GDPR requires identification of data controller")
	}

	// Validate withdrawal mechanism is provided
	if req.Preferences["withdrawal_mechanism"] == "" {
		return errors.NewValidationError("GDPR_WITHDRAWAL_REQUIRED",
			"GDPR requires clear withdrawal mechanism")
	}

	return nil
}

func (ca *ComplianceAdapter) validateCCPAConsent(ctx context.Context, req GrantConsentRequest) error {
	// CCPA-specific validation
	
	// Check for sale opt-out preference
	if req.Preferences != nil && req.Preferences["do_not_sell"] == "true" {
		// Log CCPA opt-out request
		ca.logger.Info("CCPA do-not-sell preference recorded",
			zap.String("phone_number", req.PhoneNumber),
		)
	}

	return nil
}

func (ca *ComplianceAdapter) validateMarketingConsent(ctx context.Context, req GrantConsentRequest) error {
	// Marketing-specific validation
	
	// Ensure frequency preferences are set
	if req.Preferences == nil || req.Preferences["frequency"] == "" {
		// Set default frequency if not specified
		if req.Preferences == nil {
			req.Preferences = make(map[string]string)
		}
		req.Preferences["frequency"] = "weekly"
	}

	return nil
}