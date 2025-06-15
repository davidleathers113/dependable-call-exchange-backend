package services

import (
	"context"
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/types"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/errors"
)

// DNCComplianceService encapsulates complex business logic for DNC compliance checking
type DNCComplianceService struct {
	repository dnc.Repository
	timeZones  TimeZoneService
}

// NewDNCComplianceService creates a new instance of the compliance service
func NewDNCComplianceService(repository dnc.Repository, timeZones TimeZoneService) (*DNCComplianceService, error) {
	if repository == nil {
		return nil, errors.NewValidationError("INVALID_REPOSITORY", "repository cannot be nil")
	}
	if timeZones == nil {
		return nil, errors.NewValidationError("INVALID_TIMEZONE_SERVICE", "time zone service cannot be nil")
	}

	return &DNCComplianceService{
		repository: repository,
		timeZones:  timeZones,
	}, nil
}

// CheckCompliance performs a comprehensive DNC compliance check for a phone number
func (s *DNCComplianceService) CheckCompliance(ctx context.Context, phoneNumber *values.PhoneNumber, callTime time.Time) (*types.ComplianceResult, error) {
	if phoneNumber == nil {
		return nil, errors.NewValidationError("INVALID_PHONE", "phone number cannot be nil")
	}

	// 1. Check DNC lists
	entries, err := s.repository.FindByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return nil, errors.NewInternalError("failed to check DNC lists").WithCause(err)
	}

	// 2. Apply business rules
	violationReasons := make([]string, 0)
	isBlocked := false
	applicableLists := make([]types.DNCListType, 0)

	for _, entry := range entries {
		if entry.IsActive() {
			applicableLists = append(applicableLists, entry.ListType)
			
			// Federal DNC always blocks
			if entry.ListType == types.ListTypeFederal {
				isBlocked = true
				violationReasons = append(violationReasons, "Federal DNC list")
			}
			
			// State DNC blocks based on state rules
			if entry.ListType == types.ListTypeState && s.isStateBlocking(entry, phoneNumber) {
				isBlocked = true
				violationReasons = append(violationReasons, fmt.Sprintf("State DNC list (%s)", entry.StateCode))
			}
			
			// Internal and litigation always block
			if entry.ListType == types.ListTypeInternal || entry.ListType == types.ListTypeLitigation {
				isBlocked = true
				violationReasons = append(violationReasons, fmt.Sprintf("%s list", entry.ListType))
			}
		}
	}

	// 3. Check TCPA time restrictions
	tcpaCompliant := s.checkTCPATimeCompliance(phoneNumber, callTime)
	if !tcpaCompliant.IsCompliant {
		isBlocked = true
		violationReasons = append(violationReasons, tcpaCompliant.Reason)
	}

	// 4. Check wireless restrictions
	if phoneNumber.IsWireless() {
		wirelessCompliant := s.checkWirelessCompliance(phoneNumber, entries)
		if !wirelessCompliant.IsCompliant {
			isBlocked = true
			violationReasons = append(violationReasons, wirelessCompliant.Reason)
		}
	}

	result := &types.ComplianceResult{
		PhoneNumber:      phoneNumber,
		IsCompliant:      !isBlocked,
		CheckedAt:        time.Now(),
		ApplicableLists:  applicableLists,
		ViolationReasons: violationReasons,
		TCPACompliant:    tcpaCompliant.IsCompliant,
		StateCompliant:   s.checkStateCompliance(phoneNumber, entries),
	}

	return result, nil
}

// ValidateCall validates whether a call can be made based on DNC compliance
func (s *DNCComplianceService) ValidateCall(ctx context.Context, fromNumber, toNumber *values.PhoneNumber, callTime time.Time) (*types.CallValidation, error) {
	if fromNumber == nil || toNumber == nil {
		return nil, errors.NewValidationError("INVALID_NUMBERS", "phone numbers cannot be nil")
	}

	// Check compliance for the destination number
	compliance, err := s.CheckCompliance(ctx, toNumber, callTime)
	if err != nil {
		return nil, err
	}

	validation := &types.CallValidation{
		FromNumber:   fromNumber,
		ToNumber:     toNumber,
		CanCall:      compliance.IsCompliant,
		Reasons:      compliance.ViolationReasons,
		CheckedAt:    time.Now(),
		ValidUntil:   time.Now().Add(24 * time.Hour), // Cache validity
		Restrictions: make([]string, 0),
	}

	// Add specific restrictions even if call is allowed
	if compliance.IsCompliant {
		// Check for time-based restrictions
		if !s.isOptimalCallTime(toNumber, callTime) {
			validation.Restrictions = append(validation.Restrictions, "Non-optimal calling time")
		}
		
		// Check for frequency restrictions
		if s.hasRecentContact(ctx, toNumber) {
			validation.Restrictions = append(validation.Restrictions, "Recent contact - consider spacing calls")
		}
	}

	return validation, nil
}

// GetComplianceReport generates a detailed compliance report for a phone number
func (s *DNCComplianceService) GetComplianceReport(ctx context.Context, phoneNumber *values.PhoneNumber) (*types.ComplianceReport, error) {
	if phoneNumber == nil {
		return nil, errors.NewValidationError("INVALID_PHONE", "phone number cannot be nil")
	}

	entries, err := s.repository.FindByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return nil, errors.NewInternalError("failed to retrieve DNC entries").WithCause(err)
	}

	report := &types.ComplianceReport{
		PhoneNumber: phoneNumber,
		GeneratedAt: time.Now(),
		DNCListings: make([]types.DNCListingDetail, 0, len(entries)),
		TCPAStatus:  s.getTCPAStatus(phoneNumber),
		StateStatus: make(map[string]types.StateComplianceStatus),
	}

	// Compile DNC listing details
	for _, entry := range entries {
		detail := types.DNCListingDetail{
			ListType:    entry.ListType,
			AddedDate:   entry.CreatedAt,
			ExpiresDate: entry.ExpiresAt,
			Source:      entry.Source.Provider,
			IsActive:    entry.IsActive(),
			Metadata:    entry.Metadata,
		}
		
		if entry.ListType == types.ListTypeState {
			detail.StateCode = entry.StateCode
		}
		
		report.DNCListings = append(report.DNCListings, detail)
	}

	// Add state-specific compliance status
	states := s.getRelevantStates(phoneNumber)
	for _, state := range states {
		report.StateStatus[state] = s.getStateComplianceStatus(state, entries)
	}

	// Calculate overall risk score
	report.RiskScore = s.calculateComplianceRiskScore(report)
	report.Recommendations = s.generateRecommendations(report)

	return report, nil
}

// Helper methods

func (s *DNCComplianceService) checkTCPATimeCompliance(phoneNumber *values.PhoneNumber, callTime time.Time) types.TimeCompliance {
	// Get time zone for the phone number
	tz, err := s.timeZones.GetTimeZone(phoneNumber)
	if err != nil {
		// Default to most restrictive interpretation
		return types.TimeCompliance{
			IsCompliant: false,
			Reason:      "Unable to determine time zone",
		}
	}

	// Convert call time to local time
	localTime := callTime.In(tz)
	hour := localTime.Hour()

	// TCPA allows calls between 8 AM and 9 PM local time
	if hour < 8 || hour >= 21 {
		return types.TimeCompliance{
			IsCompliant: false,
			Reason:      fmt.Sprintf("Outside TCPA calling hours (8 AM - 9 PM) in %s", tz.String()),
		}
	}

	return types.TimeCompliance{
		IsCompliant: true,
	}
}

func (s *DNCComplianceService) checkWirelessCompliance(phoneNumber *values.PhoneNumber, entries []*dnc.Entry) types.TimeCompliance {
	// Wireless numbers require prior express written consent for marketing
	hasConsent := false
	
	// Check if there's an exemption in internal lists
	for _, entry := range entries {
		if entry.ListType == types.ListTypeInternal && entry.Metadata["consent_type"] == "express_written" {
			hasConsent = true
			break
		}
	}

	if !hasConsent {
		return types.TimeCompliance{
			IsCompliant: false,
			Reason:      "Wireless number requires prior express written consent",
		}
	}

	return types.TimeCompliance{
		IsCompliant: true,
	}
}

func (s *DNCComplianceService) isStateBlocking(entry *dnc.Entry, phoneNumber *values.PhoneNumber) bool {
	// State-specific blocking rules
	switch entry.StateCode {
	case "TX":
		// Texas has additional restrictions
		return true
	case "CA":
		// California requires specific consent
		return !s.hasCaliforniaConsent(phoneNumber)
	case "FL":
		// Florida follows federal rules
		return true
	default:
		return true
	}
}

func (s *DNCComplianceService) checkStateCompliance(phoneNumber *values.PhoneNumber, entries []*dnc.Entry) bool {
	// Check if all applicable state requirements are met
	for _, entry := range entries {
		if entry.ListType == types.ListTypeState && entry.IsActive() {
			if s.isStateBlocking(entry, phoneNumber) {
				return false
			}
		}
	}
	return true
}

func (s *DNCComplianceService) isOptimalCallTime(phoneNumber *values.PhoneNumber, callTime time.Time) bool {
	tz, err := s.timeZones.GetTimeZone(phoneNumber)
	if err != nil {
		return false
	}

	localTime := callTime.In(tz)
	hour := localTime.Hour()

	// Optimal calling hours are typically 10 AM - 7 PM
	return hour >= 10 && hour < 19
}

func (s *DNCComplianceService) hasRecentContact(ctx context.Context, phoneNumber *values.PhoneNumber) bool {
	// This would check call history - simplified for this implementation
	return false
}

func (s *DNCComplianceService) hasCaliforniaConsent(phoneNumber *values.PhoneNumber) bool {
	// California-specific consent check - simplified
	return false
}

func (s *DNCComplianceService) getTCPAStatus(phoneNumber *values.PhoneNumber) types.TCPAStatus {
	return types.TCPAStatus{
		RequiresConsent: phoneNumber.IsWireless(),
		ConsentType:     "prior express written consent",
		LastVerified:    time.Now(),
	}
}

func (s *DNCComplianceService) getRelevantStates(phoneNumber *values.PhoneNumber) []string {
	// Get states based on area code - simplified
	areaCode := phoneNumber.String()[2:5]
	stateMap := map[string][]string{
		"212": {"NY"},
		"213": {"CA"},
		"214": {"TX"},
		"305": {"FL"},
		// Add more mappings as needed
	}

	if states, ok := stateMap[areaCode]; ok {
		return states
	}
	return []string{}
}

func (s *DNCComplianceService) getStateComplianceStatus(state string, entries []*dnc.Entry) types.StateComplianceStatus {
	status := types.StateComplianceStatus{
		State:       state,
		IsCompliant: true,
		LastChecked: time.Now(),
	}

	for _, entry := range entries {
		if entry.ListType == types.ListTypeState && entry.StateCode == state && entry.IsActive() {
			status.IsCompliant = false
			status.Restrictions = append(status.Restrictions, "Listed on state DNC")
		}
	}

	return status
}

func (s *DNCComplianceService) calculateComplianceRiskScore(report *types.ComplianceReport) float64 {
	score := 0.0

	// Federal DNC listing is highest risk
	for _, listing := range report.DNCListings {
		if listing.IsActive {
			switch listing.ListType {
			case types.ListTypeFederal:
				score += 40.0
			case types.ListTypeLitigation:
				score += 35.0
			case types.ListTypeState:
				score += 25.0
			case types.ListTypeInternal:
				score += 20.0
			case types.ListTypeWireless:
				score += 15.0
			}
		}
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

func (s *DNCComplianceService) generateRecommendations(report *types.ComplianceReport) []string {
	recommendations := make([]string, 0)

	if report.RiskScore > 80 {
		recommendations = append(recommendations, "DO NOT CALL - High compliance risk")
	} else if report.RiskScore > 50 {
		recommendations = append(recommendations, "Verify consent before calling")
		recommendations = append(recommendations, "Consider alternative contact methods")
	}

	if report.TCPAStatus.RequiresConsent {
		recommendations = append(recommendations, "Ensure prior express written consent is documented")
	}

	for state, status := range report.StateStatus {
		if !status.IsCompliant {
			recommendations = append(recommendations, fmt.Sprintf("Check %s state-specific requirements", state))
		}
	}

	return recommendations
}

// TimeZoneService interface for time zone lookups
type TimeZoneService interface {
	GetTimeZone(phoneNumber *values.PhoneNumber) (*time.Location, error)
}