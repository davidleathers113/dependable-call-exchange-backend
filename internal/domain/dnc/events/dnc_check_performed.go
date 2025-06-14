package events

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// DNCCheckResult represents the result of a DNC check
type DNCCheckResult string

const (
	DNCCheckResultBlocked    DNCCheckResult = "blocked"
	DNCCheckResultAllowed    DNCCheckResult = "allowed"
	DNCCheckResultError      DNCCheckResult = "error"
	DNCCheckResultTimeout    DNCCheckResult = "timeout"
	DNCCheckResultPartial    DNCCheckResult = "partial"
	DNCCheckResultUnknown    DNCCheckResult = "unknown"
)

// DNCCheckPerformedEvent represents when a DNC check is performed
type DNCCheckPerformedEvent struct {
	// Base event information
	EventID       uuid.UUID                    `json:"event_id"`
	EventType     audit.EventType              `json:"event_type"`
	EventVersion  string                       `json:"event_version"`
	Timestamp     time.Time                    `json:"timestamp"`
	
	// DNC check specific fields
	PhoneNumber   values.PhoneNumber           `json:"phone_number"`
	Result        DNCCheckResult               `json:"result"`
	CheckedAt     time.Time                    `json:"checked_at"`
	Sources       []string                     `json:"sources"`
	Latency       time.Duration                `json:"latency"`
	
	// Performance metrics
	TotalSources     int                       `json:"total_sources"`
	SuccessfulSources int                      `json:"successful_sources"`
	FailedSources    int                       `json:"failed_sources"`
	CacheHits        int                       `json:"cache_hits"`
	CacheMisses      int                       `json:"cache_misses"`
	
	// Result details
	IsBlocked        bool                      `json:"is_blocked"`
	BlockingReasons  []string                  `json:"blocking_reasons,omitempty"`
	HighestSeverity  string                    `json:"highest_severity,omitempty"`
	RiskScore        float64                   `json:"risk_score"`
	ConfidenceScore  float64                   `json:"confidence_score"`
	
	// Check context
	CheckReason      string                    `json:"check_reason"`      // routing, compliance, verification
	InitiatedBy      uuid.UUID                 `json:"initiated_by"`      // User or system ID
	CallID          *uuid.UUID                 `json:"call_id,omitempty"` // Associated call if applicable
	CheckType       string                     `json:"check_type"`        // real_time, batch, manual
	
	// Compliance metadata
	ComplianceLevel  string                    `json:"compliance_level"`  // strict, standard, relaxed
	TCPARelevant     bool                      `json:"tcpa_relevant"`
	GDPRRelevant     bool                      `json:"gdpr_relevant"`
	
	// Error information
	ErrorCode       *string                    `json:"error_code,omitempty"`
	ErrorMessage    *string                    `json:"error_message,omitempty"`
	WarningMessages []string                   `json:"warning_messages,omitempty"`
	
	// Audit trail metadata
	RequestID       string                     `json:"request_id"`
	SessionID       string                     `json:"session_id,omitempty"`
	UserAgent       string                     `json:"user_agent,omitempty"`
	IPAddress       string                     `json:"ip_address,omitempty"`
	
	// System metadata
	ServerID        string                     `json:"server_id,omitempty"`
	Environment     string                     `json:"environment,omitempty"`
	ProcessingTime  time.Duration              `json:"processing_time,omitempty"`
	BatchID        *string                     `json:"batch_id,omitempty"`
	
	// Compliance metadata
	ComplianceFlags map[string]bool            `json:"compliance_flags,omitempty"`
	DataClasses     []string                   `json:"data_classes,omitempty"`
	LegalBasis      string                     `json:"legal_basis,omitempty"`
}

// NewDNCCheckPerformedEvent creates a new DNC check performed event
func NewDNCCheckPerformedEvent(
	phoneNumber string,
	result DNCCheckResult,
	sources []string,
	latency time.Duration,
	initiatedBy uuid.UUID,
	checkReason string,
) (*DNCCheckPerformedEvent, error) {
	// Validate phone number
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", 
			"phone number must be valid E.164 format").WithCause(err)
	}
	
	// Validate initiated by user ID
	if initiatedBy == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_USER_ID", 
			"initiated by user ID cannot be empty")
	}
	
	// Validate result
	if err := validateCheckResult(result); err != nil {
		return nil, err
	}
	
	// Validate sources
	if len(sources) == 0 {
		return nil, errors.NewValidationError("INVALID_SOURCES", 
			"at least one source must be specified")
	}
	
	// Validate check reason
	if checkReason == "" {
		return nil, errors.NewValidationError("INVALID_CHECK_REASON", 
			"check reason cannot be empty")
	}
	
	now := time.Now().UTC()
	
	event := &DNCCheckPerformedEvent{
		EventID:           uuid.New(),
		EventType:         audit.EventDNCCheckPerformed,
		EventVersion:      "1.0",
		Timestamp:         now,
		PhoneNumber:       phone,
		Result:            result,
		CheckedAt:         now,
		Sources:           sources,
		Latency:           latency,
		TotalSources:      len(sources),
		InitiatedBy:       initiatedBy,
		CheckReason:       checkReason,
		CheckType:         "real_time",
		ComplianceLevel:   "standard",
		TCPARelevant:      true,
		GDPRRelevant:      false,
		ComplianceFlags:   make(map[string]bool),
		DataClasses:       []string{"phone_number", "dnc_status"},
		LegalBasis:        "legitimate_interest",
		WarningMessages:   make([]string, 0),
	}
	
	// Set compliance flags based on result
	event.setComplianceFlags()
	
	// Initialize performance metrics
	event.updatePerformanceMetrics()
	
	return event, nil
}

// GetEventType returns the event type
func (e *DNCCheckPerformedEvent) GetEventType() audit.EventType {
	return audit.EventDNCCheckPerformed
}

// GetEventVersion returns the event version
func (e *DNCCheckPerformedEvent) GetEventVersion() string {
	return e.EventVersion
}

// GetEventID returns the event ID
func (e *DNCCheckPerformedEvent) GetEventID() uuid.UUID {
	return e.EventID
}

// GetTimestamp returns the event timestamp
func (e *DNCCheckPerformedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetAggregateID returns the phone number as the aggregate ID
func (e *DNCCheckPerformedEvent) GetAggregateID() string {
	return e.PhoneNumber.String()
}

// GetAggregateType returns the aggregate type
func (e *DNCCheckPerformedEvent) GetAggregateType() string {
	return "phone_number"
}

// SetPerformanceMetrics sets detailed performance metrics
func (e *DNCCheckPerformedEvent) SetPerformanceMetrics(successful, failed, cacheHits, cacheMisses int) {
	e.SuccessfulSources = successful
	e.FailedSources = failed
	e.CacheHits = cacheHits
	e.CacheMisses = cacheMisses
	e.updatePerformanceMetrics()
}

// SetResultDetails sets detailed result information
func (e *DNCCheckPerformedEvent) SetResultDetails(isBlocked bool, reasons []string, severity string, riskScore, confidenceScore float64) {
	e.IsBlocked = isBlocked
	e.BlockingReasons = reasons
	e.HighestSeverity = severity
	e.RiskScore = riskScore
	e.ConfidenceScore = confidenceScore
}

// SetCheckContext sets additional check context
func (e *DNCCheckPerformedEvent) SetCheckContext(checkType string, callID *uuid.UUID) {
	e.CheckType = checkType
	e.CallID = callID
}

// SetComplianceContext sets compliance-specific context
func (e *DNCCheckPerformedEvent) SetComplianceContext(level string, tcpaRelevant, gdprRelevant bool) {
	e.ComplianceLevel = level
	e.TCPARelevant = tcpaRelevant
	e.GDPRRelevant = gdprRelevant
	e.setComplianceFlags()
}

// SetError sets error information for failed checks
func (e *DNCCheckPerformedEvent) SetError(code, message string) {
	e.ErrorCode = &code
	e.ErrorMessage = &message
	e.Result = DNCCheckResultError
}

// AddWarning adds a warning message
func (e *DNCCheckPerformedEvent) AddWarning(message string) {
	e.WarningMessages = append(e.WarningMessages, message)
}

// SetRequestContext sets the request context information
func (e *DNCCheckPerformedEvent) SetRequestContext(requestID, sessionID, userAgent, ipAddress string) {
	e.RequestID = requestID
	if sessionID != "" {
		e.SessionID = sessionID
	}
	if userAgent != "" {
		e.UserAgent = userAgent
	}
	if ipAddress != "" {
		e.IPAddress = ipAddress
	}
}

// SetSystemContext sets system-specific context
func (e *DNCCheckPerformedEvent) SetSystemContext(serverID, environment string) {
	e.ServerID = serverID
	e.Environment = environment
}

// SetBatchID sets the batch ID for bulk operations
func (e *DNCCheckPerformedEvent) SetBatchID(batchID string) {
	e.BatchID = &batchID
}

// SetProcessingTime sets the processing time for performance tracking
func (e *DNCCheckPerformedEvent) SetProcessingTime(duration time.Duration) {
	e.ProcessingTime = duration
}

// IsSuccessful returns true if the check completed successfully
func (e *DNCCheckPerformedEvent) IsSuccessful() bool {
	return e.Result != DNCCheckResultError && e.Result != DNCCheckResultTimeout
}

// IsBlocked returns true if the number was blocked
func (e *DNCCheckPerformedEvent) IsNumberBlocked() bool {
	return e.IsBlocked && e.Result == DNCCheckResultBlocked
}

// IsError returns true if the check resulted in an error
func (e *DNCCheckPerformedEvent) IsError() bool {
	return e.Result == DNCCheckResultError
}

// IsTimeout returns true if the check timed out
func (e *DNCCheckPerformedEvent) IsTimeout() bool {
	return e.Result == DNCCheckResultTimeout
}

// IsPartialResult returns true if only some sources were checked
func (e *DNCCheckPerformedEvent) IsPartialResult() bool {
	return e.Result == DNCCheckResultPartial
}

// IsRealTimeCheck returns true if this was a real-time check
func (e *DNCCheckPerformedEvent) IsRealTimeCheck() bool {
	return e.CheckType == "real_time"
}

// IsBatchCheck returns true if this was a batch check
func (e *DNCCheckPerformedEvent) IsBatchCheck() bool {
	return e.CheckType == "batch"
}

// IsManualCheck returns true if this was a manual check
func (e *DNCCheckPerformedEvent) IsManualCheck() bool {
	return e.CheckType == "manual"
}

// IsHighRisk returns true if the check indicates high risk
func (e *DNCCheckPerformedEvent) IsHighRisk() bool {
	return e.RiskScore >= 0.7 || e.HighestSeverity == "high"
}

// GetSuccessRate returns the success rate of source checks
func (e *DNCCheckPerformedEvent) GetSuccessRate() float64 {
	if e.TotalSources == 0 {
		return 0.0
	}
	return float64(e.SuccessfulSources) / float64(e.TotalSources)
}

// GetCacheHitRate returns the cache hit rate
func (e *DNCCheckPerformedEvent) GetCacheHitRate() float64 {
	total := e.CacheHits + e.CacheMisses
	if total == 0 {
		return 0.0
	}
	return float64(e.CacheHits) / float64(total)
}

// HasWarnings returns true if there are warning messages
func (e *DNCCheckPerformedEvent) HasWarnings() bool {
	return len(e.WarningMessages) > 0
}

// GetPerformanceMetrics returns performance-related metadata
func (e *DNCCheckPerformedEvent) GetPerformanceMetrics() map[string]interface{} {
	return map[string]interface{}{
		"latency_ms":        e.Latency.Milliseconds(),
		"total_sources":     e.TotalSources,
		"successful_sources": e.SuccessfulSources,
		"failed_sources":    e.FailedSources,
		"success_rate":      e.GetSuccessRate(),
		"cache_hits":        e.CacheHits,
		"cache_misses":      e.CacheMisses,
		"cache_hit_rate":    e.GetCacheHitRate(),
		"processing_time":   e.ProcessingTime.String(),
		"risk_score":        e.RiskScore,
		"confidence_score":  e.ConfidenceScore,
	}
}

// GetComplianceMetadata returns compliance-specific metadata
func (e *DNCCheckPerformedEvent) GetComplianceMetadata() map[string]interface{} {
	metadata := map[string]interface{}{
		"phone_number":      e.PhoneNumber.String(),
		"check_result":      string(e.Result),
		"is_blocked":        e.IsBlocked,
		"check_reason":      e.CheckReason,
		"check_type":        e.CheckType,
		"compliance_level":  e.ComplianceLevel,
		"tcpa_relevant":     e.TCPARelevant,
		"gdpr_relevant":     e.GDPRRelevant,
		"legal_basis":       e.LegalBasis,
		"data_classes":      e.DataClasses,
		"is_high_risk":      e.IsHighRisk(),
		"sources_checked":   e.Sources,
		"latency_ms":        e.Latency.Milliseconds(),
	}
	
	if e.CallID != nil {
		metadata["call_id"] = e.CallID.String()
	}
	
	if len(e.BlockingReasons) > 0 {
		metadata["blocking_reasons"] = e.BlockingReasons
	}
	
	if e.HighestSeverity != "" {
		metadata["highest_severity"] = e.HighestSeverity
	}
	
	if e.ErrorCode != nil {
		metadata["error_code"] = *e.ErrorCode
	}
	
	if e.ErrorMessage != nil {
		metadata["error_message"] = *e.ErrorMessage
	}
	
	if len(e.WarningMessages) > 0 {
		metadata["warnings"] = e.WarningMessages
	}
	
	if e.BatchID != nil {
		metadata["batch_id"] = *e.BatchID
	}
	
	return metadata
}

// ToAuditEvent converts the domain event to an audit event
func (e *DNCCheckPerformedEvent) ToAuditEvent() (*audit.Event, error) {
	auditEvent, err := audit.NewEvent(
		e.EventType,
		e.InitiatedBy.String(),
		e.PhoneNumber.String(),
		"check_dnc_status",
	)
	if err != nil {
		return nil, err
	}
	
	// Set result based on check outcome
	result := "success"
	if e.IsError() {
		result = "error"
	} else if e.IsTimeout() {
		result = "timeout"
	}
	
	// Set additional audit event fields
	auditEvent.TargetType = "phone_number"
	auditEvent.Result = result
	auditEvent.RequestID = e.RequestID
	auditEvent.SessionID = e.SessionID
	auditEvent.ActorAgent = e.UserAgent
	auditEvent.ActorIP = e.IPAddress
	auditEvent.LegalBasis = e.LegalBasis
	auditEvent.DataClasses = e.DataClasses
	auditEvent.ComplianceFlags = e.ComplianceFlags
	
	// Add DNC-specific metadata
	auditEvent.Metadata = map[string]interface{}{
		"check_result":       string(e.Result),
		"is_blocked":         e.IsBlocked,
		"sources_checked":    e.Sources,
		"latency_ms":         e.Latency.Milliseconds(),
		"check_reason":       e.CheckReason,
		"check_type":         e.CheckType,
		"compliance_level":   e.ComplianceLevel,
		"total_sources":      e.TotalSources,
		"successful_sources": e.SuccessfulSources,
		"failed_sources":     e.FailedSources,
		"success_rate":       e.GetSuccessRate(),
		"cache_hits":         e.CacheHits,
		"cache_misses":       e.CacheMisses,
		"cache_hit_rate":     e.GetCacheHitRate(),
		"risk_score":         e.RiskScore,
		"confidence_score":   e.ConfidenceScore,
		"processing_time":    e.ProcessingTime.String(),
	}
	
	if e.CallID != nil {
		auditEvent.Metadata["call_id"] = e.CallID.String()
	}
	
	if len(e.BlockingReasons) > 0 {
		auditEvent.Metadata["blocking_reasons"] = e.BlockingReasons
	}
	
	if e.HighestSeverity != "" {
		auditEvent.Metadata["highest_severity"] = e.HighestSeverity
	}
	
	if e.ErrorCode != nil {
		auditEvent.Metadata["error_code"] = *e.ErrorCode
	}
	
	if e.ErrorMessage != nil {
		auditEvent.Metadata["error_message"] = *e.ErrorMessage
	}
	
	if len(e.WarningMessages) > 0 {
		auditEvent.Metadata["warnings"] = e.WarningMessages
	}
	
	if e.BatchID != nil {
		auditEvent.Metadata["batch_id"] = *e.BatchID
	}
	
	return auditEvent, nil
}

// setComplianceFlags sets compliance flags based on check context
func (e *DNCCheckPerformedEvent) setComplianceFlags() {
	e.ComplianceFlags["dnc_check"] = true
	e.ComplianceFlags["contains_pii"] = true
	
	// Set TCPA relevance
	if e.TCPARelevant {
		e.ComplianceFlags["tcpa_relevant"] = true
	}
	
	// Set GDPR relevance
	if e.GDPRRelevant {
		e.ComplianceFlags["gdpr_relevant"] = true
	}
	
	// Set flags based on check type
	if e.IsRealTimeCheck() {
		e.ComplianceFlags["real_time_check"] = true
	}
	
	if e.IsBatchCheck() {
		e.ComplianceFlags["batch_check"] = true
	}
	
	if e.IsManualCheck() {
		e.ComplianceFlags["manual_check"] = true
	}
	
	// Set flags based on result
	if e.IsBlocked {
		e.ComplianceFlags["number_blocked"] = true
	}
	
	if e.IsError() {
		e.ComplianceFlags["check_failed"] = true
	}
	
	if e.IsTimeout() {
		e.ComplianceFlags["check_timeout"] = true
	}
	
	if e.IsHighRisk() {
		e.ComplianceFlags["high_risk"] = true
	}
	
	// Set performance flags
	if e.GetSuccessRate() < 0.5 {
		e.ComplianceFlags["low_success_rate"] = true
	}
	
	if e.Latency > 5*time.Second {
		e.ComplianceFlags["slow_check"] = true
	}
}

// updatePerformanceMetrics updates calculated performance metrics
func (e *DNCCheckPerformedEvent) updatePerformanceMetrics() {
	// Update failed sources count
	if e.SuccessfulSources > 0 && e.TotalSources > 0 {
		e.FailedSources = e.TotalSources - e.SuccessfulSources
	}
	
	// Ensure consistency
	if e.SuccessfulSources+e.FailedSources > e.TotalSources {
		e.FailedSources = e.TotalSources - e.SuccessfulSources
	}
}

// validateCheckResult validates the check result
func validateCheckResult(result DNCCheckResult) error {
	switch result {
	case DNCCheckResultBlocked, DNCCheckResultAllowed, DNCCheckResultError,
		DNCCheckResultTimeout, DNCCheckResultPartial, DNCCheckResultUnknown:
		return nil
	default:
		return errors.NewValidationError("INVALID_CHECK_RESULT", 
			"invalid check result: "+string(result))
	}
}

// Validate performs validation of the event
func (e *DNCCheckPerformedEvent) Validate() error {
	if e.EventID == uuid.Nil {
		return errors.NewValidationError("MISSING_EVENT_ID", "event ID is required")
	}
	
	if e.EventType != audit.EventDNCCheckPerformed {
		return errors.NewValidationError("INVALID_EVENT_TYPE", "invalid event type")
	}
	
	if e.EventVersion == "" {
		return errors.NewValidationError("MISSING_EVENT_VERSION", "event version is required")
	}
	
	if e.PhoneNumber.String() == "" {
		return errors.NewValidationError("MISSING_PHONE_NUMBER", "phone number is required")
	}
	
	if e.InitiatedBy == uuid.Nil {
		return errors.NewValidationError("MISSING_INITIATED_BY", "initiated by user ID is required")
	}
	
	if e.CheckReason == "" {
		return errors.NewValidationError("MISSING_CHECK_REASON", "check reason is required")
	}
	
	if len(e.Sources) == 0 {
		return errors.NewValidationError("MISSING_SOURCES", "at least one source is required")
	}
	
	if e.RequestID == "" {
		return errors.NewValidationError("MISSING_REQUEST_ID", "request ID is required")
	}
	
	// Validate check result
	if err := validateCheckResult(e.Result); err != nil {
		return err
	}
	
	// Validate performance metrics consistency
	if e.SuccessfulSources+e.FailedSources > e.TotalSources {
		return errors.NewValidationError("INVALID_METRICS", 
			"successful + failed sources cannot exceed total sources")
	}
	
	// Validate risk and confidence scores
	if e.RiskScore < 0.0 || e.RiskScore > 1.0 {
		return errors.NewValidationError("INVALID_RISK_SCORE", 
			"risk score must be between 0.0 and 1.0")
	}
	
	if e.ConfidenceScore < 0.0 || e.ConfidenceScore > 1.0 {
		return errors.NewValidationError("INVALID_CONFIDENCE_SCORE", 
			"confidence score must be between 0.0 and 1.0")
	}
	
	return nil
}