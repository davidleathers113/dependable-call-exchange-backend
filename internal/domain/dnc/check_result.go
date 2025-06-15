package dnc

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// BlockReason represents a reason why a number is blocked
type BlockReason struct {
	Source         values.ListSource     `json:"source"`
	Reason         values.SuppressReason `json:"reason"`
	Description    string                `json:"description"`
	ProviderName   string                `json:"provider_name"`
	ProviderID     uuid.UUID             `json:"provider_id"`
	ExpiresAt      *time.Time            `json:"expires_at,omitempty"`
	Severity       string                `json:"severity"` // high, medium, low
	ComplianceCode string                `json:"compliance_code"`
}

// DNCCheckResult represents the aggregated result of checking a phone number against multiple DNC lists
// This entity aggregates results from multiple providers and determines the final compliance decision
type DNCCheckResult struct {
	ID          uuid.UUID          `json:"id"`
	PhoneNumber values.PhoneNumber `json:"phone_number"`
	IsBlocked   bool               `json:"is_blocked"`
	Reasons     []BlockReason      `json:"reasons"`
	CheckedAt   time.Time          `json:"checked_at"`
	Sources     []values.ListSource `json:"sources"` // All sources that were checked
	TTL         time.Duration      `json:"ttl"`     // How long this result can be cached
	
	// Performance metrics
	CheckDuration time.Duration      `json:"check_duration"`
	SourcesCount  int                `json:"sources_count"`
	
	// Compliance metadata
	ComplianceLevel string            `json:"compliance_level"` // strict, standard, relaxed
	RiskScore       float64           `json:"risk_score"`       // 0.0 to 1.0
	
	// Additional metadata
	Metadata    map[string]string     `json:"metadata,omitempty"`
}

// NewDNCCheckResult creates a new DNC check result
// All business rules and validation are enforced in the constructor
func NewDNCCheckResult(phoneNumber string) (*DNCCheckResult, error) {
	// Validate phone number
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", "invalid phone number format").WithCause(err)
	}

	return &DNCCheckResult{
		ID:              uuid.New(),
		PhoneNumber:     phone,
		IsBlocked:       false,
		Reasons:         []BlockReason{},
		CheckedAt:       time.Now().UTC(),
		Sources:         []values.ListSource{},
		TTL:             24 * time.Hour, // Default TTL
		ComplianceLevel: "standard",
		RiskScore:       0.0,
		Metadata:        make(map[string]string),
	}, nil
}

// AddBlockReason adds a blocking reason to the result
func (r *DNCCheckResult) AddBlockReason(reason BlockReason) error {
	// Validate the block reason
	if reason.ProviderID == uuid.Nil {
		return errors.NewValidationError("INVALID_PROVIDER", "provider ID cannot be empty")
	}

	if reason.ProviderName == "" {
		return errors.NewValidationError("INVALID_PROVIDER", "provider name cannot be empty")
	}

	// Set severity if not provided
	if reason.Severity == "" {
		reason.Severity = r.determineSeverity(reason.Reason)
	}

	r.Reasons = append(r.Reasons, reason)
	r.IsBlocked = true
	
	// Update risk score based on the new reason
	r.updateRiskScore()
	
	return nil
}

// AddCheckedSource records that a source was checked
func (r *DNCCheckResult) AddCheckedSource(source values.ListSource) {
	// Avoid duplicates
	for _, s := range r.Sources {
		if s == source {
			return
		}
	}
	r.Sources = append(r.Sources, source)
	r.SourcesCount = len(r.Sources)
}

// SetCheckDuration sets how long the check took
func (r *DNCCheckResult) SetCheckDuration(duration time.Duration) {
	r.CheckDuration = duration
}

// SetTTL sets the time-to-live for caching this result
func (r *DNCCheckResult) SetTTL(ttl time.Duration) error {
	if ttl < time.Minute {
		return errors.NewValidationError("INVALID_TTL", "TTL must be at least 1 minute")
	}

	if ttl > 7*24*time.Hour {
		return errors.NewValidationError("INVALID_TTL", "TTL cannot exceed 7 days")
	}

	r.TTL = ttl
	return nil
}

// SetComplianceLevel sets the compliance level for this check
func (r *DNCCheckResult) SetComplianceLevel(level string) error {
	switch level {
	case "strict", "standard", "relaxed":
		r.ComplianceLevel = level
		return nil
	default:
		return errors.NewValidationError("INVALID_COMPLIANCE_LEVEL", 
			fmt.Sprintf("invalid compliance level: %s", level))
	}
}

// GetBlockingReasons returns all blocking reasons sorted by severity
func (r *DNCCheckResult) GetBlockingReasons() []BlockReason {
	// Sort by severity (high > medium > low) and then by provider name
	reasons := make([]BlockReason, len(r.Reasons))
	copy(reasons, r.Reasons)
	
	sort.Slice(reasons, func(i, j int) bool {
		// Compare severity
		if reasons[i].Severity != reasons[j].Severity {
			return r.severityPriority(reasons[i].Severity) > r.severityPriority(reasons[j].Severity)
		}
		// If severity is the same, sort by provider name
		return reasons[i].ProviderName < reasons[j].ProviderName
	})
	
	return reasons
}

// GetComplianceInfo returns compliance-relevant information
func (r *DNCCheckResult) GetComplianceInfo() map[string]interface{} {
	info := map[string]interface{}{
		"phone_number":     r.PhoneNumber.String(),
		"is_blocked":       r.IsBlocked,
		"checked_at":       r.CheckedAt,
		"compliance_level": r.ComplianceLevel,
		"risk_score":       r.RiskScore,
		"sources_checked":  r.SourcesCount,
		"ttl_seconds":      r.TTL.Seconds(),
	}

	// Add blocking reasons summary
	if r.IsBlocked {
		reasonSummary := []map[string]interface{}{}
		for _, reason := range r.Reasons {
			reasonSummary = append(reasonSummary, map[string]interface{}{
				"source":          string(reason.Source),
				"reason":          string(reason.Reason),
				"provider":        reason.ProviderName,
				"severity":        reason.Severity,
				"compliance_code": reason.ComplianceCode,
			})
		}
		info["blocking_reasons"] = reasonSummary
		info["highest_severity"] = r.GetHighestSeverity()
	}

	// Add source breakdown
	sourceMap := make(map[string]bool)
	for _, source := range r.Sources {
		sourceMap[string(source)] = true
	}
	info["sources"] = sourceMap

	return info
}

// GetHighestSeverity returns the highest severity among all blocking reasons
func (r *DNCCheckResult) GetHighestSeverity() string {
	if len(r.Reasons) == 0 {
		return "none"
	}

	highestPriority := 0
	highestSeverity := "low"

	for _, reason := range r.Reasons {
		priority := r.severityPriority(reason.Severity)
		if priority > highestPriority {
			highestPriority = priority
			highestSeverity = reason.Severity
		}
	}

	return highestSeverity
}

// HasPermanentBlock checks if any blocking reason is permanent (no expiration)
func (r *DNCCheckResult) HasPermanentBlock() bool {
	for _, reason := range r.Reasons {
		if reason.ExpiresAt == nil {
			return true
		}
	}
	return false
}

// GetEarliestExpiration returns the earliest expiration time among all blocking reasons
func (r *DNCCheckResult) GetEarliestExpiration() *time.Time {
	var earliest *time.Time

	for _, reason := range r.Reasons {
		if reason.ExpiresAt != nil {
			if earliest == nil || reason.ExpiresAt.Before(*earliest) {
				earliest = reason.ExpiresAt
			}
		}
	}

	return earliest
}

// IsExpired checks if the result has expired based on TTL
func (r *DNCCheckResult) IsExpired() bool {
	return time.Since(r.CheckedAt) > r.TTL
}

// GetComplianceRecommendation provides a compliance recommendation based on the check result
func (r *DNCCheckResult) GetComplianceRecommendation() string {
	if !r.IsBlocked {
		return "OK_TO_CALL"
	}

	// Check for regulatory blocks
	hasRegulatory := false
	hasConsumerRequest := false
	hasFraud := false

	for _, reason := range r.Reasons {
		// Use the value object's compliance checking
		if reason.Reason.IsRegulatory() {
			hasRegulatory = true
		}
		
		// Check specific reason types using string comparison
		reasonStr := reason.Reason.String()
		switch reasonStr {
		case "user_request", "consumer_request":
			hasConsumerRequest = true
		case "fraud", "fraud_prevention":
			hasFraud = true
		}
	}

	if hasRegulatory {
		return "DO_NOT_CALL_REGULATORY"
	}
	if hasFraud {
		return "DO_NOT_CALL_FRAUD_RISK"
	}
	if hasConsumerRequest {
		return "DO_NOT_CALL_CONSUMER_REQUEST"
	}

	return "DO_NOT_CALL_POLICY"
}

// GetComplianceCodes returns all unique compliance codes
func (r *DNCCheckResult) GetComplianceCodes() []string {
	codeMap := make(map[string]bool)
	for _, reason := range r.Reasons {
		if reason.ComplianceCode != "" {
			codeMap[reason.ComplianceCode] = true
		}
	}

	codes := make([]string, 0, len(codeMap))
	for code := range codeMap {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	return codes
}

// SetMetadata sets a metadata key-value pair
func (r *DNCCheckResult) SetMetadata(key, value string) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata[key] = value
}

// GetSummary returns a human-readable summary of the check result
func (r *DNCCheckResult) GetSummary() string {
	if !r.IsBlocked {
		return fmt.Sprintf("Phone number %s is not on any DNC list (checked %d sources)",
			r.PhoneNumber.String(), r.SourcesCount)
	}

	reasons := []string{}
	for _, reason := range r.Reasons {
		reasons = append(reasons, fmt.Sprintf("%s (%s)", reason.ProviderName, reason.Reason))
	}

	return fmt.Sprintf("Phone number %s is blocked by %d source(s): %s",
		r.PhoneNumber.String(), len(r.Reasons), strings.Join(reasons, ", "))
}

// Helper methods

// determineSeverity determines the severity based on the suppress reason
func (r *DNCCheckResult) determineSeverity(reason values.SuppressReason) string {
	// Use the value object's built-in risk level
	riskLevel := reason.GetRiskLevel()
	switch riskLevel {
	case "critical":
		return "high"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

// severityPriority returns a numeric priority for severity levels
func (r *DNCCheckResult) severityPriority(severity string) int {
	switch severity {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// updateRiskScore updates the risk score based on current blocking reasons
func (r *DNCCheckResult) updateRiskScore() {
	if len(r.Reasons) == 0 {
		r.RiskScore = 0.0
		return
	}

	// Base score on number of blocks and their severity
	score := 0.0
	
	for _, reason := range r.Reasons {
		switch reason.Severity {
		case "high":
			score += 0.4
		case "medium":
			score += 0.2
		case "low":
			score += 0.1
		}
		
		// Additional weight for regulatory reasons
		if reason.Reason.IsRegulatory() {
			score += 0.3
		}
		
		// Use the severity level from the value object
		severityLevel := reason.Reason.SeverityLevel()
		score += float64(severityLevel) / 100.0 // Normalize severity to 0.0-0.1 range
	}
	
	// Normalize to 0.0-1.0 range
	if score > 1.0 {
		score = 1.0
	}
	
	r.RiskScore = score
}

// NewBlockReason creates a new block reason with proper value objects
func NewBlockReason(source, reason, providerName string, providerID uuid.UUID) (BlockReason, error) {
	// Validate and create value objects
	listSource, err := values.NewListSource(source)
	if err != nil {
		return BlockReason{}, errors.NewValidationError("INVALID_SOURCE", "invalid list source").WithCause(err)
	}

	suppressReason, err := values.NewSuppressReason(reason)
	if err != nil {
		return BlockReason{}, errors.NewValidationError("INVALID_REASON", "invalid suppress reason").WithCause(err)
	}

	if providerName == "" {
		return BlockReason{}, errors.NewValidationError("INVALID_PROVIDER", "provider name cannot be empty")
	}

	if providerID == uuid.Nil {
		return BlockReason{}, errors.NewValidationError("INVALID_PROVIDER", "provider ID cannot be empty")
	}

	return BlockReason{
		Source:         listSource,
		Reason:         suppressReason,
		ProviderName:   providerName,
		ProviderID:     providerID,
		Severity:       suppressReason.GetRiskLevel(),
		ComplianceCode: suppressReason.GetComplianceCode(),
	}, nil
}

// CanCall determines if a call can be made based on this check result
func (r *DNCCheckResult) CanCall() bool {
	return !r.IsBlocked
}

// GetHighestAuthoritySource returns the source with the highest authority level
func (r *DNCCheckResult) GetHighestAuthoritySource() *values.ListSource {
	if len(r.Reasons) == 0 {
		return nil
	}

	var highest *values.ListSource
	highestLevel := 0

	for _, reason := range r.Reasons {
		level := reason.Source.AuthorityLevel()
		if level > highestLevel {
			highestLevel = level
			highest = &reason.Source
		}
	}

	return highest
}

// GetViolationCount returns the number of violations by severity
func (r *DNCCheckResult) GetViolationCount() map[string]int {
	counts := map[string]int{
		"high":   0,
		"medium": 0,
		"low":    0,
	}

	for _, reason := range r.Reasons {
		counts[reason.Severity]++
	}

	return counts
}

// IsHighRisk checks if this result represents a high compliance risk
func (r *DNCCheckResult) IsHighRisk() bool {
	return r.RiskScore >= 0.7 || r.GetHighestSeverity() == "high"
}