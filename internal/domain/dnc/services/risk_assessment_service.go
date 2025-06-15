package services

import (
	"context"
	"math"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/types"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/errors"
	"github.com/shopspring/decimal"
)

// DNCRiskAssessmentService encapsulates business logic for calculating violation risks and penalties
type DNCRiskAssessmentService struct {
	repository    dnc.Repository
	penaltyRates  *PenaltyRateConfiguration
	historyLookup CallHistoryService
}

// PenaltyRateConfiguration holds the penalty rates for different violation types
type PenaltyRateConfiguration struct {
	FederalBasePenalty        decimal.Decimal
	StatePenaltyMultiplier    map[string]decimal.Decimal
	LitigationPenaltyFactor   decimal.Decimal
	RepeatViolationMultiplier decimal.Decimal
	WirelessPenaltyMultiplier decimal.Decimal
	MaxPenaltyPerViolation    decimal.Decimal
}

// NewDNCRiskAssessmentService creates a new instance of the risk assessment service
func NewDNCRiskAssessmentService(repository dnc.Repository, penaltyRates *PenaltyRateConfiguration, historyLookup CallHistoryService) (*DNCRiskAssessmentService, error) {
	if repository == nil {
		return nil, errors.NewValidationError("INVALID_REPOSITORY", "repository cannot be nil")
	}
	if penaltyRates == nil {
		return nil, errors.NewValidationError("INVALID_PENALTY_RATES", "penalty rates configuration cannot be nil")
	}
	if historyLookup == nil {
		return nil, errors.NewValidationError("INVALID_HISTORY_SERVICE", "call history service cannot be nil")
	}

	return &DNCRiskAssessmentService{
		repository:    repository,
		penaltyRates:  penaltyRates,
		historyLookup: historyLookup,
	}, nil
}

// AssessRisk performs a comprehensive risk assessment for calling a phone number
func (s *DNCRiskAssessmentService) AssessRisk(ctx context.Context, phoneNumber *values.PhoneNumber, callContext *types.CallContext) (*types.RiskAssessment, error) {
	if phoneNumber == nil {
		return nil, errors.NewValidationError("INVALID_PHONE", "phone number cannot be nil")
	}
	if callContext == nil {
		return nil, errors.NewValidationError("INVALID_CONTEXT", "call context cannot be nil")
	}

	// 1. Get DNC entries for the phone number
	entries, err := s.repository.FindByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return nil, errors.NewInternalError("failed to retrieve DNC entries").WithCause(err)
	}

	// 2. Get call history for this number
	history, err := s.historyLookup.GetCallHistory(ctx, phoneNumber, 90) // Last 90 days
	if err != nil {
		return nil, errors.NewInternalError("failed to retrieve call history").WithCause(err)
	}

	// 3. Calculate base risk score
	baseRisk := s.calculateBaseRiskScore(entries, phoneNumber)

	// 4. Apply risk modifiers
	modifiedRisk := s.applyRiskModifiers(baseRisk, history, callContext)

	// 5. Calculate potential penalties
	penalties := s.calculatePotentialPenalties(entries, history, phoneNumber)

	// 6. Determine risk factors
	riskFactors := s.identifyRiskFactors(entries, history, phoneNumber, callContext)

	// 7. Generate mitigation strategies
	mitigations := s.generateMitigationStrategies(riskFactors, entries)

	assessment := &types.RiskAssessment{
		PhoneNumber:       phoneNumber,
		RiskScore:         modifiedRisk,
		RiskLevel:         s.determineRiskLevel(modifiedRisk),
		PotentialPenalty:  penalties.Total,
		PenaltyBreakdown:  penalties,
		RiskFactors:       riskFactors,
		AssessedAt:        time.Now(),
		ValidUntil:        time.Now().Add(24 * time.Hour),
		Recommendations:   s.generateRecommendations(modifiedRisk, riskFactors),
		MitigationStrategies: mitigations,
		ConfidenceLevel:   s.calculateConfidenceLevel(entries, history),
	}

	return assessment, nil
}

// CalculatePenalty calculates the potential penalty for a specific violation scenario
func (s *DNCRiskAssessmentService) CalculatePenalty(ctx context.Context, violation *types.ViolationScenario) (*types.PenaltyCalculation, error) {
	if violation == nil {
		return nil, errors.NewValidationError("INVALID_VIOLATION", "violation scenario cannot be nil")
	}

	entries, err := s.repository.FindByPhoneNumber(ctx, violation.PhoneNumber)
	if err != nil {
		return nil, errors.NewInternalError("failed to retrieve DNC entries").WithCause(err)
	}

	history, err := s.historyLookup.GetCallHistory(ctx, violation.PhoneNumber, 365) // Full year for repeat violations
	if err != nil {
		return nil, errors.NewInternalError("failed to retrieve call history").WithCause(err)
	}

	calculation := &types.PenaltyCalculation{
		Scenario:     violation,
		CalculatedAt: time.Now(),
		Components:   make([]types.PenaltyComponent, 0),
	}

	basePenalty := s.penaltyRates.FederalBasePenalty

	// Federal DNC violation base penalty
	if s.hasFederalDNCListing(entries) {
		component := types.PenaltyComponent{
			Type:        "federal_dnc",
			Description: "Federal DNC violation base penalty",
			Amount:      basePenalty,
			Multiplier:  decimal.NewFromFloat(1.0),
		}
		calculation.Components = append(calculation.Components, component)
		calculation.Total = calculation.Total.Add(basePenalty)
	}

	// State-specific penalties
	for _, entry := range entries {
		if entry.ListType == types.ListTypeState && entry.IsActive() {
			if multiplier, exists := s.penaltyRates.StatePenaltyMultiplier[entry.StateCode]; exists {
				stateAmount := basePenalty.Mul(multiplier)
				component := types.PenaltyComponent{
					Type:        "state_dnc",
					Description: "State DNC violation (" + entry.StateCode + ")",
					Amount:      stateAmount,
					Multiplier:  multiplier,
				}
				calculation.Components = append(calculation.Components, component)
				calculation.Total = calculation.Total.Add(stateAmount)
			}
		}
	}

	// Litigation list penalty
	if s.hasLitigationListing(entries) {
		litigationPenalty := basePenalty.Mul(s.penaltyRates.LitigationPenaltyFactor)
		component := types.PenaltyComponent{
			Type:        "litigation",
			Description: "Litigation list violation",
			Amount:      litigationPenalty,
			Multiplier:  s.penaltyRates.LitigationPenaltyFactor,
		}
		calculation.Components = append(calculation.Components, component)
		calculation.Total = calculation.Total.Add(litigationPenalty)
	}

	// Repeat violation multiplier
	repeatViolations := s.countRecentViolations(history)
	if repeatViolations > 0 {
		multiplier := s.penaltyRates.RepeatViolationMultiplier.Pow(decimal.NewFromInt(int64(repeatViolations)))
		additionalPenalty := calculation.Total.Mul(multiplier.Sub(decimal.NewFromFloat(1.0)))
		
		component := types.PenaltyComponent{
			Type:        "repeat_violation",
			Description: "Repeat violation multiplier",
			Amount:      additionalPenalty,
			Multiplier:  multiplier,
		}
		calculation.Components = append(calculation.Components, component)
		calculation.Total = calculation.Total.Add(additionalPenalty)
	}

	// Wireless penalty multiplier
	if violation.PhoneNumber.IsWireless() {
		wirelessAdditional := calculation.Total.Mul(s.penaltyRates.WirelessPenaltyMultiplier.Sub(decimal.NewFromFloat(1.0)))
		component := types.PenaltyComponent{
			Type:        "wireless",
			Description: "Wireless number additional penalty",
			Amount:      wirelessAdditional,
			Multiplier:  s.penaltyRates.WirelessPenaltyMultiplier,
		}
		calculation.Components = append(calculation.Components, component)
		calculation.Total = calculation.Total.Add(wirelessAdditional)
	}

	// Cap at maximum penalty
	if calculation.Total.GreaterThan(s.penaltyRates.MaxPenaltyPerViolation) {
		calculation.Total = s.penaltyRates.MaxPenaltyPerViolation
		calculation.IsCapped = true
	}

	// Calculate confidence level based on data quality
	calculation.ConfidenceLevel = s.calculatePenaltyConfidence(entries, history, violation)

	return calculation, nil
}

// GetRiskScore calculates a numerical risk score (0-100) for a phone number
func (s *DNCRiskAssessmentService) GetRiskScore(ctx context.Context, phoneNumber *values.PhoneNumber) (float64, error) {
	if phoneNumber == nil {
		return 0, errors.NewValidationError("INVALID_PHONE", "phone number cannot be nil")
	}

	entries, err := s.repository.FindByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return 0, errors.NewInternalError("failed to retrieve DNC entries").WithCause(err)
	}

	history, err := s.historyLookup.GetCallHistory(ctx, phoneNumber, 30) // Last 30 days
	if err != nil {
		return 0, errors.NewInternalError("failed to retrieve call history").WithCause(err)
	}

	baseScore := s.calculateBaseRiskScore(entries, phoneNumber)
	
	// Default call context for basic risk assessment
	defaultContext := &types.CallContext{
		Purpose:    types.CallPurposeMarketing,
		Industry:   "telecommunications",
		TimeOfDay:  time.Now().Hour(),
		IsRecorded: false,
	}

	finalScore := s.applyRiskModifiers(baseScore, history, defaultContext)

	return finalScore, nil
}

// Helper methods

func (s *DNCRiskAssessmentService) calculateBaseRiskScore(entries []*dnc.Entry, phoneNumber *values.PhoneNumber) float64 {
	score := 0.0

	for _, entry := range entries {
		if entry.IsActive() {
			switch entry.ListType {
			case types.ListTypeFederal:
				score += 40.0 // Federal DNC is highest risk
			case types.ListTypeLitigation:
				score += 35.0 // Litigation list is very high risk
			case types.ListTypeState:
				score += 25.0 // State DNC is moderate-high risk
			case types.ListTypeInternal:
				score += 20.0 // Internal suppression is moderate risk
			case types.ListTypeWireless:
				score += 15.0 // Wireless requires special handling
			}
		}
	}

	// Additional risk for wireless numbers
	if phoneNumber.IsWireless() {
		score += 10.0
	}

	// Cap at 100
	return math.Min(score, 100.0)
}

func (s *DNCRiskAssessmentService) applyRiskModifiers(baseScore float64, history *types.CallHistory, context *types.CallContext) float64 {
	modifiedScore := baseScore

	// Recent violation history increases risk
	if history.ViolationCount > 0 {
		modifiedScore += float64(history.ViolationCount) * 5.0
	}

	// Frequent calling increases risk
	if history.CallCount > 10 {
		modifiedScore += 5.0
	}

	// Call purpose affects risk
	switch context.Purpose {
	case types.CallPurposeMarketing:
		modifiedScore += 5.0
	case types.CallPurposeDebt:
		modifiedScore += 10.0
	case types.CallPurposeEmergency:
		modifiedScore -= 20.0 // Emergency calls have legal exemptions
	}

	// Time of day affects risk
	if context.TimeOfDay < 8 || context.TimeOfDay >= 21 {
		modifiedScore += 15.0 // Outside allowed hours significantly increases risk
	}

	return math.Min(modifiedScore, 100.0)
}

func (s *DNCRiskAssessmentService) calculatePotentialPenalties(entries []*dnc.Entry, history *types.CallHistory, phoneNumber *values.PhoneNumber) *types.PenaltyBreakdown {
	breakdown := &types.PenaltyBreakdown{
		Components: make(map[string]decimal.Decimal),
	}

	basePenalty := s.penaltyRates.FederalBasePenalty

	if s.hasFederalDNCListing(entries) {
		breakdown.Components["federal"] = basePenalty
		breakdown.Total = breakdown.Total.Add(basePenalty)
	}

	if s.hasLitigationListing(entries) {
		litigationPenalty := basePenalty.Mul(s.penaltyRates.LitigationPenaltyFactor)
		breakdown.Components["litigation"] = litigationPenalty
		breakdown.Total = breakdown.Total.Add(litigationPenalty)
	}

	// State penalties
	for _, entry := range entries {
		if entry.ListType == types.ListTypeState && entry.IsActive() {
			if multiplier, exists := s.penaltyRates.StatePenaltyMultiplier[entry.StateCode]; exists {
				statePenalty := basePenalty.Mul(multiplier)
				breakdown.Components["state_"+entry.StateCode] = statePenalty
				breakdown.Total = breakdown.Total.Add(statePenalty)
			}
		}
	}

	// Repeat violation multiplier
	if history.ViolationCount > 0 {
		multiplier := s.penaltyRates.RepeatViolationMultiplier.Pow(decimal.NewFromInt(int64(history.ViolationCount)))
		additionalPenalty := breakdown.Total.Mul(multiplier.Sub(decimal.NewFromFloat(1.0)))
		breakdown.Components["repeat"] = additionalPenalty
		breakdown.Total = breakdown.Total.Add(additionalPenalty)
	}

	return breakdown
}

func (s *DNCRiskAssessmentService) identifyRiskFactors(entries []*dnc.Entry, history *types.CallHistory, phoneNumber *values.PhoneNumber, context *types.CallContext) []types.RiskFactor {
	factors := make([]types.RiskFactor, 0)

	// DNC listing factors
	for _, entry := range entries {
		if entry.IsActive() {
			factor := types.RiskFactor{
				Type:        "dnc_listing",
				Severity:    s.getSeverityForListType(entry.ListType),
				Description: "Number is on " + string(entry.ListType) + " DNC list",
				Impact:      s.getImpactForListType(entry.ListType),
			}
			factors = append(factors, factor)
		}
	}

	// Wireless factor
	if phoneNumber.IsWireless() {
		factors = append(factors, types.RiskFactor{
			Type:        "wireless_number",
			Severity:    types.SeverityMedium,
			Description: "Wireless numbers require prior express written consent",
			Impact:      "Additional consent requirements and penalties",
		})
	}

	// Time factor
	if context.TimeOfDay < 8 || context.TimeOfDay >= 21 {
		factors = append(factors, types.RiskFactor{
			Type:        "time_violation",
			Severity:    types.SeverityHigh,
			Description: "Calling outside permitted hours (8 AM - 9 PM)",
			Impact:      "TCPA time restriction violation",
		})
	}

	// History factors
	if history.ViolationCount > 0 {
		factors = append(factors, types.RiskFactor{
			Type:        "violation_history",
			Severity:    types.SeverityHigh,
			Description: "Previous violations on record",
			Impact:      "Escalated penalties for repeat violations",
		})
	}

	return factors
}

func (s *DNCRiskAssessmentService) generateMitigationStrategies(factors []types.RiskFactor, entries []*dnc.Entry) []types.MitigationStrategy {
	strategies := make([]types.MitigationStrategy, 0)

	for _, factor := range factors {
		switch factor.Type {
		case "dnc_listing":
			strategies = append(strategies, types.MitigationStrategy{
				Type:         "avoid_contact",
				Description:  "Do not contact this number",
				Effectiveness: "100%",
				Cost:         "Low",
				Timeframe:    "Immediate",
			})
		case "wireless_number":
			strategies = append(strategies, types.MitigationStrategy{
				Type:         "verify_consent",
				Description:  "Verify prior express written consent",
				Effectiveness: "95%",
				Cost:         "Medium",
				Timeframe:    "1-2 hours",
			})
		case "time_violation":
			strategies = append(strategies, types.MitigationStrategy{
				Type:         "reschedule_call",
				Description:  "Schedule call during permitted hours",
				Effectiveness: "100%",
				Cost:         "Low",
				Timeframe:    "Next business day",
			})
		}
	}

	return strategies
}

func (s *DNCRiskAssessmentService) determineRiskLevel(score float64) types.RiskLevel {
	switch {
	case score >= 80:
		return types.RiskLevelCritical
	case score >= 60:
		return types.RiskLevelHigh
	case score >= 40:
		return types.RiskLevelMedium
	case score >= 20:
		return types.RiskLevelLow
	default:
		return types.RiskLevelMinimal
	}
}

func (s *DNCRiskAssessmentService) generateRecommendations(score float64, factors []types.RiskFactor) []string {
	recommendations := make([]string, 0)

	if score >= 80 {
		recommendations = append(recommendations, "DO NOT CALL - Critical compliance risk")
		recommendations = append(recommendations, "Seek legal counsel before any contact")
	} else if score >= 60 {
		recommendations = append(recommendations, "High risk - avoid contact unless legally justified")
		recommendations = append(recommendations, "Document all compliance checks")
	} else if score >= 40 {
		recommendations = append(recommendations, "Moderate risk - verify all exemptions apply")
		recommendations = append(recommendations, "Consider alternative contact methods")
	}

	// Specific recommendations based on risk factors
	for _, factor := range factors {
		switch factor.Type {
		case "wireless_number":
			recommendations = append(recommendations, "Verify express written consent for wireless number")
		case "time_violation":
			recommendations = append(recommendations, "Reschedule to permitted calling hours")
		}
	}

	return recommendations
}

func (s *DNCRiskAssessmentService) calculateConfidenceLevel(entries []*dnc.Entry, history *types.CallHistory) float64 {
	confidence := 100.0

	// Reduce confidence if data is stale
	for _, entry := range entries {
		age := time.Since(entry.CreatedAt).Hours() / 24
		if age > 30 {
			confidence -= 5.0
		}
	}

	// Reduce confidence if no recent history
	if history.LastCallDate.IsZero() || time.Since(history.LastCallDate).Hours() > 720 { // 30 days
		confidence -= 10.0
	}

	return math.Max(confidence, 60.0) // Minimum 60% confidence
}

// Helper methods for specific checks

func (s *DNCRiskAssessmentService) hasFederalDNCListing(entries []*dnc.Entry) bool {
	for _, entry := range entries {
		if entry.ListType == types.ListTypeFederal && entry.IsActive() {
			return true
		}
	}
	return false
}

func (s *DNCRiskAssessmentService) hasLitigationListing(entries []*dnc.Entry) bool {
	for _, entry := range entries {
		if entry.ListType == types.ListTypeLitigation && entry.IsActive() {
			return true
		}
	}
	return false
}

func (s *DNCRiskAssessmentService) countRecentViolations(history *types.CallHistory) int {
	// Count violations in the last 12 months
	return history.ViolationCount
}

func (s *DNCRiskAssessmentService) getSeverityForListType(listType types.DNCListType) types.Severity {
	switch listType {
	case types.ListTypeFederal, types.ListTypeLitigation:
		return types.SeverityCritical
	case types.ListTypeState:
		return types.SeverityHigh
	case types.ListTypeInternal:
		return types.SeverityMedium
	default:
		return types.SeverityLow
	}
}

func (s *DNCRiskAssessmentService) getImpactForListType(listType types.DNCListType) string {
	switch listType {
	case types.ListTypeFederal:
		return "Federal penalties up to $40,654 per violation"
	case types.ListTypeLitigation:
		return "High litigation risk and escalated penalties"
	case types.ListTypeState:
		return "State-specific penalties and compliance violations"
	case types.ListTypeInternal:
		return "Company policy violation"
	default:
		return "Compliance requirements apply"
	}
}

func (s *DNCRiskAssessmentService) calculatePenaltyConfidence(entries []*dnc.Entry, history *types.CallHistory, scenario *types.ViolationScenario) float64 {
	confidence := 95.0

	// Reduce confidence for stale data
	for _, entry := range entries {
		if time.Since(entry.CreatedAt).Hours() > 720 { // 30 days
			confidence -= 5.0
		}
	}

	// Reduce confidence if missing context
	if scenario.CallPurpose == "" {
		confidence -= 10.0
	}

	return math.Max(confidence, 70.0)
}

// CallHistoryService interface for retrieving call history
type CallHistoryService interface {
	GetCallHistory(ctx context.Context, phoneNumber *values.PhoneNumber, days int) (*types.CallHistory, error)
}