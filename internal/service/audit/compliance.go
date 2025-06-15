package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// ComplianceService implements comprehensive compliance management
// Following DCE patterns: orchestrates domain services, handles multiple compliance frameworks
type ComplianceService struct {
	// Repositories
	auditRepo      audit.EventRepository
	complianceRepo compliance.Repository
	queryRepo      audit.QueryRepository
	integrityRepo  audit.IntegrityRepository

	// Domain services
	hashChainService        *audit.HashChainService
	integrityService        *audit.IntegrityCheckService
	complianceVerifyService *audit.ComplianceVerificationService

	// Configuration
	retentionPolicies map[string]RetentionPolicy
	legalHolds        map[string]LegalHold

	// Compliance engines
	tcpaEngine ComplianceEngine
	gdprEngine ComplianceEngine
	ccpaEngine ComplianceEngine
	soxEngine  ComplianceEngine
}

// NewComplianceService creates a new compliance management service
func NewComplianceService(
	auditRepo audit.EventRepository,
	complianceRepo compliance.Repository,
	queryRepo audit.QueryRepository,
	integrityRepo audit.IntegrityRepository,
	hashChainService *audit.HashChainService,
	integrityService *audit.IntegrityCheckService,
	complianceVerifyService *audit.ComplianceVerificationService,
) *ComplianceService {
	service := &ComplianceService{
		auditRepo:               auditRepo,
		complianceRepo:          complianceRepo,
		queryRepo:               queryRepo,
		integrityRepo:           integrityRepo,
		hashChainService:        hashChainService,
		integrityService:        integrityService,
		complianceVerifyService: complianceVerifyService,
		retentionPolicies:       make(map[string]RetentionPolicy),
		legalHolds:              make(map[string]LegalHold),
	}

	// Initialize compliance engines
	service.tcpaEngine = NewTCPAEngine(complianceRepo, auditRepo)
	service.gdprEngine = NewGDPREngine(complianceRepo, auditRepo)
	service.ccpaEngine = NewCCPAEngine(complianceRepo, auditRepo)
	service.soxEngine = NewSOXEngine(auditRepo, integrityRepo)

	// Load default retention policies
	service.loadDefaultRetentionPolicies()

	return service
}

// TCPA Compliance Methods

// ValidateTCPACompliance validates TCPA compliance for a call
func (s *ComplianceService) ValidateTCPACompliance(ctx context.Context, req TCPAValidationRequest) (*TCPAValidationResult, error) {
	result := &TCPAValidationResult{
		RequestID:    uuid.New().String(),
		PhoneNumber:  req.PhoneNumber,
		ValidatedAt:  time.Now().UTC(),
		IsCompliant:  true,
		Violations:   make([]ComplianceViolation, 0),
		Requirements: make([]ComplianceRequirement, 0),
	}

	// Check consent status
	consentRecord, err := s.complianceRepo.GetConsentByPhone(ctx, req.PhoneNumber)
	if err != nil && !errors.IsNotFound(err) {
		return nil, errors.NewInternalError("failed to check consent").WithCause(err)
	}

	if consentRecord == nil || !consentRecord.IsActive() {
		result.IsCompliant = false
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:        "NO_CONSENT",
			Severity:    "CRITICAL",
			Description: "No active consent found for phone number",
			Regulation:  "TCPA",
			Impact:      "Call cannot proceed without explicit consent",
		})
	} else {
		result.ConsentStatus = "ACTIVE"
		result.ConsentExpiry = consentRecord.ExpiresAt
	}

	// Check DNC registry
	isDNC, err := s.checkDNCRegistry(ctx, req.PhoneNumber)
	if err != nil {
		return nil, errors.NewInternalError("failed to check DNC").WithCause(err)
	}

	if isDNC {
		result.IsCompliant = false
		result.IsDNC = true
		result.Violations = append(result.Violations, ComplianceViolation{
			Type:        "DNC_REGISTERED",
			Severity:    "CRITICAL",
			Description: "Phone number is on Do Not Call registry",
			Regulation:  "TCPA",
			Impact:      "Calls prohibited unless exemption applies",
		})
	}

	// Check time restrictions
	if req.CallTime.IsZero() {
		req.CallTime = time.Now()
	}

	timeCompliant, timeViolation := s.checkTCPATimeRestrictions(req.CallTime, req.Timezone)
	if !timeCompliant {
		result.IsCompliant = false
		result.Violations = append(result.Violations, timeViolation)
	}

	// Check call frequency limits
	frequencyCompliant, frequencyViolation := s.checkCallFrequency(ctx, req.PhoneNumber)
	if !frequencyCompliant {
		result.IsCompliant = false
		result.Violations = append(result.Violations, frequencyViolation)
	}

	// Add requirements
	result.Requirements = s.getTCPARequirements()

	// Record compliance check
	s.recordComplianceCheck(ctx, "TCPA", req.PhoneNumber, result.IsCompliant, result.Violations)

	return result, nil
}

// RecordTCPAConsent records TCPA consent
func (s *ComplianceService) RecordTCPAConsent(ctx context.Context, consent TCPAConsent) error {
	// Validate consent
	if err := s.validateTCPAConsent(consent); err != nil {
		return errors.NewValidationError("INVALID_CONSENT", err.Error())
	}

	// Create consent record
	consentRecord := compliance.NewConsentRecord(
		consent.PhoneNumber,
		s.mapConsentType(consent.ConsentType),
		consent.Source,
		consent.IPAddress,
		consent.UserAgent,
	)

	// Set expiry if provided
	if consent.ExpiryDays > 0 {
		expiry := time.Now().AddDate(0, 0, consent.ExpiryDays)
		consentRecord.ExpiresAt = &expiry
	}

	// Store consent
	if err := s.complianceRepo.SaveConsent(ctx, consentRecord); err != nil {
		return errors.NewInternalError("failed to save consent").WithCause(err)
	}

	// Create audit event
	event := &audit.Event{
		ID:            uuid.New(),
		Type:          audit.EventConsentGranted,
		ActorID:       consent.ActorID,
		TargetID:      consent.PhoneNumber,
		Action:        "grant_tcpa_consent",
		Result:        "success",
		Timestamp:     time.Now().UTC(),
		TimestampNano: time.Now().UnixNano(),
		DataClasses:   []string{"phone_number", "consent"},
		LegalBasis:    "explicit_consent",
		ComplianceFlags: map[string]interface{}{
			"tcpa_consent_type":   consent.ConsentType,
			"tcpa_consent_source": consent.Source,
			"tcpa_consent_expiry": consent.ExpiryDays,
		},
	}

	if err := s.auditRepo.CreateEvent(ctx, event); err != nil {
		return errors.NewInternalError("failed to create audit event").WithCause(err)
	}

	return nil
}

// RevokeTCPAConsent revokes TCPA consent
func (s *ComplianceService) RevokeTCPAConsent(ctx context.Context, req TCPARevocation) error {
	// Get existing consent
	consentRecord, err := s.complianceRepo.GetConsentByPhone(ctx, req.PhoneNumber)
	if err != nil {
		return errors.NewNotFoundError("consent").WithCause(err)
	}

	// Revoke consent
	consentRecord.Revoke()

	// Update consent record
	if err := s.complianceRepo.UpdateConsent(ctx, consentRecord); err != nil {
		return errors.NewInternalError("failed to update consent").WithCause(err)
	}

	// Create audit event
	event := &audit.Event{
		ID:            uuid.New(),
		Type:          audit.EventConsentRevoked,
		ActorID:       req.ActorID,
		TargetID:      req.PhoneNumber,
		Action:        "revoke_tcpa_consent",
		Result:        "success",
		Timestamp:     time.Now().UTC(),
		TimestampNano: time.Now().UnixNano(),
		DataClasses:   []string{"phone_number", "consent"},
		LegalBasis:    "consent_withdrawal",
		ComplianceFlags: map[string]interface{}{
			"tcpa_revocation_reason": req.Reason,
			"tcpa_revocation_source": req.Source,
		},
	}

	if err := s.auditRepo.CreateEvent(ctx, event); err != nil {
		return errors.NewInternalError("failed to create audit event").WithCause(err)
	}

	return nil
}

// GDPR Compliance Methods

// ProcessGDPRRequest processes a GDPR data subject request
func (s *ComplianceService) ProcessGDPRRequest(ctx context.Context, req GDPRRequest) (*GDPRRequestResult, error) {
	result := &GDPRRequestResult{
		RequestID:   uuid.New().String(),
		RequestType: req.Type,
		Status:      "PROCESSING",
		ReceivedAt:  time.Now().UTC(),
		DataSubject: req.DataSubjectID,
	}

	switch req.Type {
	case "ACCESS":
		return s.processGDPRAccessRequest(ctx, req, result)
	case "ERASURE":
		return s.processGDPRErasureRequest(ctx, req, result)
	case "RECTIFICATION":
		return s.processGDPRRectificationRequest(ctx, req, result)
	case "PORTABILITY":
		return s.processGDPRPortabilityRequest(ctx, req, result)
	case "RESTRICTION":
		return s.processGDPRRestrictionRequest(ctx, req, result)
	default:
		return nil, errors.NewValidationError("INVALID_REQUEST_TYPE", "Unknown GDPR request type")
	}
}

// processGDPRAccessRequest handles GDPR access requests
func (s *ComplianceService) processGDPRAccessRequest(ctx context.Context, req GDPRRequest, result *GDPRRequestResult) (*GDPRRequestResult, error) {
	// Generate GDPR report
	criteria := audit.GDPRReportCriteria{
		DataSubjectID:      req.DataSubjectID,
		DataSubjectEmail:   req.DataSubjectEmail,
		DataSubjectPhone:   req.DataSubjectPhone,
		StartTime:          time.Now().AddDate(-2, 0, 0), // Last 2 years
		EndTime:            time.Now(),
		RequestType:        "access",
		IncludeDataSources: true,
		IncludeProcessing:  true,
		IncludeConsent:     true,
		DetailLevel:        "comprehensive",
	}

	report, err := s.generateGDPRReport(ctx, criteria)
	if err != nil {
		result.Status = "FAILED"
		result.Error = err.Error()
		return result, err
	}

	// Store report for retrieval
	result.Status = "COMPLETED"
	result.CompletedAt = &[]time.Time{time.Now().UTC()}[0]
	result.DataExport = &DataExportInfo{
		ExportID:    report.ID,
		Format:      "JSON",
		RecordCount: int64(len(report.DataEvents)),
		ExpiryDate:  time.Now().AddDate(0, 0, 30), // 30 days to download
	}

	// Create audit event
	s.createGDPRAuditEvent(ctx, "access_request", req.DataSubjectID, "completed")

	return result, nil
}

// processGDPRErasureRequest handles GDPR erasure requests (right to be forgotten)
func (s *ComplianceService) processGDPRErasureRequest(ctx context.Context, req GDPRRequest, result *GDPRRequestResult) (*GDPRRequestResult, error) {
	// Check for legal holds
	hasLegalHold, holdReason := s.checkLegalHolds(ctx, req.DataSubjectID)
	if hasLegalHold {
		result.Status = "REJECTED"
		result.RejectionReason = fmt.Sprintf("Legal hold in place: %s", holdReason)
		return result, nil
	}

	// Check for legitimate reasons to retain
	retentionRequired, retentionReason := s.checkRetentionRequirements(ctx, req.DataSubjectID)
	if retentionRequired {
		result.Status = "PARTIALLY_COMPLETED"
		result.Notes = fmt.Sprintf("Some data retained due to: %s", retentionReason)
	}

	// Perform anonymization instead of deletion where required
	anonymized, err := s.anonymizeDataSubject(ctx, req.DataSubjectID, req.DataCategories)
	if err != nil {
		result.Status = "FAILED"
		result.Error = err.Error()
		return result, err
	}

	result.Status = "COMPLETED"
	result.CompletedAt = &[]time.Time{time.Now().UTC()}[0]
	result.DataAffected = &DataAffectedInfo{
		RecordsAnonymized: anonymized.RecordsAnonymized,
		RecordsDeleted:    anonymized.RecordsDeleted,
		DataCategories:    anonymized.DataCategories,
	}

	// Create audit event
	s.createGDPRAuditEvent(ctx, "erasure_request", req.DataSubjectID, "completed")

	return result, nil
}

// processGDPRRectificationRequest handles GDPR rectification requests
func (s *ComplianceService) processGDPRRectificationRequest(ctx context.Context, req GDPRRequest, result *GDPRRequestResult) (*GDPRRequestResult, error) {
	// Validate rectification data
	if req.RectificationData == nil {
		return nil, errors.NewValidationError("MISSING_DATA", "Rectification data required")
	}

	// Apply rectifications
	rectified := 0
	failed := 0

	for field, newValue := range req.RectificationData {
		err := s.rectifyDataField(ctx, req.DataSubjectID, field, newValue)
		if err != nil {
			failed++
			result.Notes += fmt.Sprintf("Failed to rectify %s: %v; ", field, err)
		} else {
			rectified++
		}
	}

	if failed > 0 && rectified == 0 {
		result.Status = "FAILED"
	} else if failed > 0 {
		result.Status = "PARTIALLY_COMPLETED"
	} else {
		result.Status = "COMPLETED"
	}

	result.CompletedAt = &[]time.Time{time.Now().UTC()}[0]
	result.DataAffected = &DataAffectedInfo{
		RecordsModified: int64(rectified),
		FieldsUpdated:   getMapKeys(req.RectificationData),
	}

	// Create audit event
	s.createGDPRAuditEvent(ctx, "rectification_request", req.DataSubjectID, result.Status)

	return result, nil
}

// processGDPRPortabilityRequest handles GDPR data portability requests
func (s *ComplianceService) processGDPRPortabilityRequest(ctx context.Context, req GDPRRequest, result *GDPRRequestResult) (*GDPRRequestResult, error) {
	// Export data in machine-readable format
	exportData, err := s.exportDataSubjectData(ctx, req.DataSubjectID, req.ExportFormat)
	if err != nil {
		result.Status = "FAILED"
		result.Error = err.Error()
		return result, err
	}

	result.Status = "COMPLETED"
	result.CompletedAt = &[]time.Time{time.Now().UTC()}[0]
	result.DataExport = &DataExportInfo{
		ExportID:    exportData.ExportID,
		Format:      exportData.Format,
		FileSize:    exportData.FileSize,
		RecordCount: exportData.RecordCount,
		ExpiryDate:  exportData.ExpiryDate,
	}

	// Create audit event
	s.createGDPRAuditEvent(ctx, "portability_request", req.DataSubjectID, "completed")

	return result, nil
}

// processGDPRRestrictionRequest handles GDPR processing restriction requests
func (s *ComplianceService) processGDPRRestrictionRequest(ctx context.Context, req GDPRRequest, result *GDPRRequestResult) (*GDPRRequestResult, error) {
	// Apply processing restrictions
	restriction := &ProcessingRestriction{
		ID:                   uuid.New().String(),
		DataSubjectID:        req.DataSubjectID,
		RequestDate:          time.Now().UTC(),
		EffectiveDate:        time.Now().UTC(),
		DataCategories:       req.DataCategories,
		ProcessingActivities: req.RestrictedActivities,
		RestrictedActions:    []string{"marketing", "profiling", "automated_decisions"},
		RestrictionReason:    req.Reason,
		LegalBasis:           "data_subject_request",
		Status:               "active",
	}

	if err := s.applyProcessingRestriction(ctx, restriction); err != nil {
		result.Status = "FAILED"
		result.Error = err.Error()
		return result, err
	}

	result.Status = "COMPLETED"
	result.CompletedAt = &[]time.Time{time.Now().UTC()}[0]
	result.ProcessingRestriction = restriction

	// Create audit event
	s.createGDPRAuditEvent(ctx, "restriction_request", req.DataSubjectID, "completed")

	return result, nil
}

// CCPA Compliance Methods

// ProcessCCPARequest processes CCPA privacy requests
func (s *ComplianceService) ProcessCCPARequest(ctx context.Context, req CCPARequest) (*CCPARequestResult, error) {
	result := &CCPARequestResult{
		RequestID:   uuid.New().String(),
		RequestType: req.Type,
		Status:      "PROCESSING",
		ReceivedAt:  time.Now().UTC(),
		Consumer:    req.ConsumerID,
	}

	// Verify California resident
	isResident, err := s.verifyCaliforniaResident(ctx, req.ConsumerID)
	if err != nil {
		return nil, errors.NewInternalError("failed to verify residency").WithCause(err)
	}

	if !isResident {
		result.Status = "REJECTED"
		result.RejectionReason = "Not a California resident"
		return result, nil
	}

	switch req.Type {
	case "OPT_OUT":
		return s.processCCPAOptOut(ctx, req, result)
	case "DELETE":
		return s.processCCPADeletion(ctx, req, result)
	case "KNOW":
		return s.processCCPAKnow(ctx, req, result)
	default:
		return nil, errors.NewValidationError("INVALID_REQUEST_TYPE", "Unknown CCPA request type")
	}
}

// processCCPAOptOut handles CCPA opt-out requests
func (s *ComplianceService) processCCPAOptOut(ctx context.Context, req CCPARequest, result *CCPARequestResult) (*CCPARequestResult, error) {
	// Record opt-out preference
	optOut := &PrivacyPreference{
		ID:          uuid.New().String(),
		ConsumerID:  req.ConsumerID,
		Type:        "CCPA_OPT_OUT",
		Value:       "true",
		EffectiveAt: time.Now().UTC(),
		Categories:  []string{"sale_of_data", "sharing_for_behavioral_advertising"},
	}

	if err := s.recordPrivacyPreference(ctx, optOut); err != nil {
		result.Status = "FAILED"
		result.Error = err.Error()
		return result, err
	}

	result.Status = "COMPLETED"
	result.CompletedAt = &[]time.Time{time.Now().UTC()}[0]
	result.OptOutApplied = true

	// Create audit event
	s.createCCPAAuditEvent(ctx, "opt_out", req.ConsumerID, "completed")

	return result, nil
}

// SOX Compliance Methods

// GenerateSOXComplianceReport generates SOX compliance report for financial data
func (s *ComplianceService) GenerateSOXComplianceReport(ctx context.Context, criteria SOXReportCriteria) (*SOXComplianceReport, error) {
	report := &SOXComplianceReport{
		ReportID:    uuid.New().String(),
		GeneratedAt: time.Now().UTC(),
		Period:      criteria.Period,
		IsCompliant: true,
		Controls:    make([]SOXControl, 0),
		Findings:    make([]SOXFinding, 0),
	}

	// Check financial data integrity
	integrityResult, err := s.checkFinancialDataIntegrity(ctx, criteria.StartDate, criteria.EndDate)
	if err != nil {
		return nil, errors.NewInternalError("failed to check data integrity").WithCause(err)
	}

	report.DataIntegrity = integrityResult

	// Check access controls
	accessResult, err := s.checkFinancialAccessControls(ctx)
	if err != nil {
		return nil, errors.NewInternalError("failed to check access controls").WithCause(err)
	}

	report.AccessControls = accessResult

	// Check audit trail completeness
	auditResult, err := s.checkAuditTrailCompleteness(ctx, criteria.StartDate, criteria.EndDate)
	if err != nil {
		return nil, errors.NewInternalError("failed to check audit trail").WithCause(err)
	}

	report.AuditTrailStatus = auditResult

	// Evaluate controls
	controls := s.evaluateSOXControls(ctx)
	report.Controls = controls

	// Determine compliance status
	for _, control := range controls {
		if control.Status == "FAILED" {
			report.IsCompliant = false
			report.Findings = append(report.Findings, SOXFinding{
				ID:          uuid.New().String(),
				ControlID:   control.ID,
				Type:        "CONTROL_FAILURE",
				Severity:    "HIGH",
				Description: fmt.Sprintf("Control %s failed: %s", control.Name, control.FailureReason),
			})
		}
	}

	return report, nil
}

// Retention Policy Methods

// ApplyRetentionPolicy applies retention policy to data
func (s *ComplianceService) ApplyRetentionPolicy(ctx context.Context, policyID string) (*RetentionResult, error) {
	policy, exists := s.retentionPolicies[policyID]
	if !exists {
		return nil, errors.NewNotFoundError("retention policy")
	}

	result := &RetentionResult{
		PolicyID:    policyID,
		ExecutionID: uuid.New().String(),
		StartedAt:   time.Now().UTC(),
		Status:      "RUNNING",
	}

	// Find data eligible for retention action
	eligibleData, err := s.findRetentionEligibleData(ctx, policy)
	if err != nil {
		result.Status = "FAILED"
		result.Error = err.Error()
		return result, err
	}

	result.RecordsEvaluated = eligibleData.TotalRecords

	// Apply retention actions
	for _, action := range policy.Actions {
		switch action.Type {
		case "DELETE":
			deleted, err := s.deleteExpiredData(ctx, eligibleData, action)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.RecordsDeleted += deleted
			}
		case "ARCHIVE":
			archived, err := s.archiveExpiredData(ctx, eligibleData, action)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.RecordsArchived += archived
			}
		case "ANONYMIZE":
			anonymized, err := s.anonymizeExpiredData(ctx, eligibleData, action)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.RecordsAnonymized += anonymized
			}
		}
	}

	result.CompletedAt = &[]time.Time{time.Now().UTC()}[0]
	result.Status = "COMPLETED"
	if len(result.Errors) > 0 {
		result.Status = "COMPLETED_WITH_ERRORS"
	}

	// Create audit event
	s.createRetentionAuditEvent(ctx, policyID, result)

	return result, nil
}

// ApplyLegalHold applies a legal hold to prevent data deletion
func (s *ComplianceService) ApplyLegalHold(ctx context.Context, hold LegalHold) error {
	// Validate legal hold
	if err := s.validateLegalHold(hold); err != nil {
		return errors.NewValidationError("INVALID_LEGAL_HOLD", err.Error())
	}

	// Store legal hold
	s.legalHolds[hold.ID] = hold

	// Create audit event
	event := &audit.Event{
		ID:          uuid.New(),
		Type:        audit.EventLegalHoldApplied,
		ActorID:     hold.IssuedBy,
		TargetID:    hold.ID,
		Action:      "apply_legal_hold",
		Result:      "success",
		Timestamp:   time.Now().UTC(),
		DataClasses: hold.DataCategories,
		LegalBasis:  "legal_obligation",
		ComplianceFlags: map[string]interface{}{
			"legal_hold_id":     hold.ID,
			"legal_hold_reason": hold.Description,
			"court_order":       hold.CourtOrder,
		},
	}

	if err := s.auditRepo.CreateEvent(ctx, event); err != nil {
		return errors.NewInternalError("failed to create audit event").WithCause(err)
	}

	return nil
}

// RemoveLegalHold removes a legal hold
func (s *ComplianceService) RemoveLegalHold(ctx context.Context, holdID string, removalReason string) error {
	hold, exists := s.legalHolds[holdID]
	if !exists {
		return errors.NewNotFoundError("legal hold")
	}

	// Update hold status
	hold.Status = "lifted"
	now := time.Now().UTC()
	hold.LiftedDate = &now
	hold.LiftedBy = "system" // Would get from context

	// Remove from active holds
	delete(s.legalHolds, holdID)

	// Create audit event
	event := &audit.Event{
		ID:          uuid.New(),
		Type:        audit.EventLegalHoldRemoved,
		ActorID:     hold.LiftedBy,
		TargetID:    holdID,
		Action:      "remove_legal_hold",
		Result:      "success",
		Timestamp:   time.Now().UTC(),
		DataClasses: hold.DataCategories,
		LegalBasis:  "legal_hold_lifted",
		ComplianceFlags: map[string]interface{}{
			"legal_hold_id":  holdID,
			"removal_reason": removalReason,
		},
	}

	if err := s.auditRepo.CreateEvent(ctx, event); err != nil {
		return errors.NewInternalError("failed to create audit event").WithCause(err)
	}

	return nil
}

// Anonymization Methods

// AnonymizeDataSubject anonymizes all data for a data subject
func (s *ComplianceService) anonymizeDataSubject(ctx context.Context, dataSubjectID string, categories []string) (*AnonymizationResult, error) {
	result := &AnonymizationResult{
		DataSubjectID:     dataSubjectID,
		StartedAt:         time.Now().UTC(),
		DataCategories:    categories,
		RecordsAnonymized: 0,
		RecordsDeleted:    0,
	}

	// Get all data for subject
	filter := audit.EventFilter{
		ActorIDs:  []string{dataSubjectID},
		TargetIDs: []string{dataSubjectID},
	}

	events, err := s.auditRepo.GetEvents(ctx, filter)
	if err != nil {
		return nil, errors.NewInternalError("failed to get events").WithCause(err)
	}

	// Anonymize each event
	for _, event := range events.Events {
		if s.shouldAnonymize(event, categories) {
			if err := s.anonymizeEvent(ctx, event); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to anonymize event %s: %v", event.ID, err))
			} else {
				result.RecordsAnonymized++
			}
		}
	}

	result.CompletedAt = &[]time.Time{time.Now().UTC()}[0]
	result.Success = len(result.Errors) == 0

	return result, nil
}

// Helper Methods

// checkDNCRegistry checks if phone number is on DNC registry
func (s *ComplianceService) checkDNCRegistry(ctx context.Context, phoneNumber string) (bool, error) {
	// This would integrate with actual DNC registry APIs
	// For now, return mock implementation
	return false, nil
}

// checkTCPATimeRestrictions checks TCPA time restrictions
func (s *ComplianceService) checkTCPATimeRestrictions(callTime time.Time, timezone string) (bool, ComplianceViolation) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	localTime := callTime.In(loc)
	hour := localTime.Hour()

	// TCPA restricts calls before 8 AM or after 9 PM local time
	if hour < 8 || hour >= 21 {
		return false, ComplianceViolation{
			Type:        "TIME_RESTRICTION",
			Severity:    "HIGH",
			Description: fmt.Sprintf("Call at %d:00 violates TCPA time restrictions (8 AM - 9 PM)", hour),
			Regulation:  "TCPA",
			Impact:      "Call prohibited during restricted hours",
		}
	}

	return true, ComplianceViolation{}
}

// checkCallFrequency checks call frequency limits
func (s *ComplianceService) checkCallFrequency(ctx context.Context, phoneNumber string) (bool, ComplianceViolation) {
	// Check recent calls to this number
	filter := audit.EventFilter{
		TargetIDs: []string{phoneNumber},
		Types:     []audit.EventType{audit.EventCallInitiated},
		StartTime: &[]time.Time{time.Now().AddDate(0, 0, -7)}[0], // Last 7 days
	}

	events, err := s.auditRepo.GetEvents(ctx, filter)
	if err != nil {
		return true, ComplianceViolation{} // Fail open
	}

	// Check frequency limits
	if len(events.Events) >= 3 {
		return false, ComplianceViolation{
			Type:        "FREQUENCY_LIMIT",
			Severity:    "MEDIUM",
			Description: fmt.Sprintf("Exceeded call frequency limit (%d calls in 7 days)", len(events.Events)),
			Regulation:  "TCPA",
			Impact:      "Additional calls may constitute harassment",
		}
	}

	return true, ComplianceViolation{}
}

// getTCPARequirements returns TCPA compliance requirements
func (s *ComplianceService) getTCPARequirements() []ComplianceRequirement {
	return []ComplianceRequirement{
		{
			ID:          "TCPA-001",
			Name:        "Explicit Consent",
			Description: "Obtain explicit written consent before making calls",
			Type:        "MANDATORY",
			Regulation:  "TCPA",
		},
		{
			ID:          "TCPA-002",
			Name:        "Time Restrictions",
			Description: "Calls only between 8 AM and 9 PM recipient's local time",
			Type:        "MANDATORY",
			Regulation:  "TCPA",
		},
		{
			ID:          "TCPA-003",
			Name:        "DNC Compliance",
			Description: "Honor Do Not Call registry and internal opt-outs",
			Type:        "MANDATORY",
			Regulation:  "TCPA",
		},
		{
			ID:          "TCPA-004",
			Name:        "Caller ID",
			Description: "Display accurate caller ID information",
			Type:        "MANDATORY",
			Regulation:  "TCPA",
		},
	}
}

// loadDefaultRetentionPolicies loads default retention policies
func (s *ComplianceService) loadDefaultRetentionPolicies() {
	// Call data retention
	s.retentionPolicies["CALL_DATA"] = RetentionPolicy{
		ID:          "CALL_DATA",
		Name:        "Call Data Retention",
		Description: "Standard retention for call records",
		DataTypes:   []string{"call_records", "call_metadata"},
		RetentionPeriod: RetentionPeriod{
			Duration: 24 * 30 * 6, // 6 months
			Unit:     "hours",
		},
		Actions: []RetentionAction{
			{Type: "ARCHIVE", AfterDays: 90},
			{Type: "DELETE", AfterDays: 180},
		},
	}

	// Consent data retention
	s.retentionPolicies["CONSENT_DATA"] = RetentionPolicy{
		ID:          "CONSENT_DATA",
		Name:        "Consent Data Retention",
		Description: "Extended retention for consent records",
		DataTypes:   []string{"consent_records", "opt_out_records"},
		RetentionPeriod: RetentionPeriod{
			Duration: 24 * 365 * 3, // 3 years
			Unit:     "hours",
		},
		Actions: []RetentionAction{
			{Type: "ARCHIVE", AfterDays: 365},
		},
	}

	// Financial data retention (SOX)
	s.retentionPolicies["FINANCIAL_DATA"] = RetentionPolicy{
		ID:          "FINANCIAL_DATA",
		Name:        "Financial Data Retention",
		Description: "SOX-compliant retention for financial records",
		DataTypes:   []string{"transactions", "billing_records", "financial_reports"},
		RetentionPeriod: RetentionPeriod{
			Duration: 24 * 365 * 7, // 7 years
			Unit:     "hours",
		},
		Actions: []RetentionAction{
			{Type: "ARCHIVE", AfterDays: 365 * 2},
		},
		LegalBasis: "SOX compliance requirement",
	}
}

// validateTCPAConsent validates TCPA consent data
func (s *ComplianceService) validateTCPAConsent(consent TCPAConsent) error {
	if consent.PhoneNumber == "" {
		return fmt.Errorf("phone number required")
	}

	if consent.ConsentType == "" {
		return fmt.Errorf("consent type required")
	}

	if consent.Source == "" {
		return fmt.Errorf("consent source required")
	}

	// Validate phone number format
	phoneNumber, err := values.NewPhoneNumber(consent.PhoneNumber)
	if err != nil {
		return fmt.Errorf("invalid phone number: %w", err)
	}
	consent.PhoneNumber = phoneNumber.String()

	return nil
}

// mapConsentType maps string consent type to domain enum
func (s *ComplianceService) mapConsentType(consentType string) compliance.ConsentType {
	switch consentType {
	case "EXPRESS":
		return compliance.ConsentTypeExpress
	case "IMPLIED":
		return compliance.ConsentTypeImplied
	case "PRIOR_BUSINESS":
		return compliance.ConsentTypePriorBusiness
	default:
		return compliance.ConsentTypeExpress
	}
}

// recordComplianceCheck records a compliance check in audit log
func (s *ComplianceService) recordComplianceCheck(ctx context.Context, framework string, target string, compliant bool, violations []ComplianceViolation) {
	status := "compliant"
	if !compliant {
		status = "non_compliant"
	}

	event := &audit.Event{
		ID:            uuid.New(),
		Type:          audit.EventComplianceChecked,
		ActorID:       "system",
		TargetID:      target,
		Action:        fmt.Sprintf("check_%s_compliance", framework),
		Result:        status,
		Timestamp:     time.Now().UTC(),
		TimestampNano: time.Now().UnixNano(),
		ComplianceFlags: map[string]interface{}{
			"framework":       framework,
			"violations":      violations,
			"violation_count": len(violations),
		},
	}

	// Best effort - don't fail the check if audit fails
	_ = s.auditRepo.CreateEvent(ctx, event)
}

// createGDPRAuditEvent creates a GDPR-specific audit event
func (s *ComplianceService) createGDPRAuditEvent(ctx context.Context, action string, dataSubjectID string, result string) {
	event := &audit.Event{
		ID:            uuid.New(),
		Type:          audit.EventGDPRRequest,
		ActorID:       dataSubjectID,
		TargetID:      dataSubjectID,
		Action:        action,
		Result:        result,
		Timestamp:     time.Now().UTC(),
		TimestampNano: time.Now().UnixNano(),
		DataClasses:   []string{"personal_data"},
		LegalBasis:    "gdpr_data_subject_rights",
		ComplianceFlags: map[string]interface{}{
			"gdpr_request_type": action,
			"gdpr_article":      s.getGDPRArticle(action),
		},
	}

	_ = s.auditRepo.CreateEvent(ctx, event)
}

// getGDPRArticle returns the GDPR article for a given request type
func (s *ComplianceService) getGDPRArticle(requestType string) string {
	articles := map[string]string{
		"access_request":        "Article 15",
		"erasure_request":       "Article 17",
		"rectification_request": "Article 16",
		"portability_request":   "Article 20",
		"restriction_request":   "Article 18",
	}
	return articles[requestType]
}

// Helper functions

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
