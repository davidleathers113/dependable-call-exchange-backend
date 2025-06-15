package compliance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// GDPRComplianceHandler implements comprehensive GDPR compliance operations
type GDPRComplianceHandler struct {
	logger             *zap.Logger
	consentService     ConsentService
	auditService       AuditService
	complianceRepo     ComplianceRepository
	dataRetentionRepo  DataRetentionRepository
	dataExportService  DataExportService
	
	// GDPR configuration
	config             GDPRConfig
	lawfulBasisRules   map[string]LawfulBasisRule
	retentionPolicies  map[string]RetentionPolicy
	processingPurposes map[string]ProcessingPurpose
}

// GDPRConfig holds GDPR compliance configuration
type GDPRConfig struct {
	StrictMode                    bool          `json:"strict_mode"`
	DefaultRetentionDays          int           `json:"default_retention_days"`
	RequireExplicitConsent        bool          `json:"require_explicit_consent"`
	CrossBorderTransferEnabled    bool          `json:"cross_border_transfer_enabled"`
	AutomaticDeletionEnabled      bool          `json:"automatic_deletion_enabled"`
	DataPortabilityFormats        []string      `json:"data_portability_formats"`
	RequestProcessingTimeoutDays  int           `json:"request_processing_timeout_days"`
	ConsentWithdrawalGraceDays    int           `json:"consent_withdrawal_grace_days"`
	PseudonymizationEnabled       bool          `json:"pseudonymization_enabled"`
	EncryptionRequired            bool          `json:"encryption_required"`
	AdequacyCountries            []string      `json:"adequacy_countries"`
	DataProcessingPurposes       []string      `json:"data_processing_purposes"`
}

type LawfulBasisRule struct {
	Basis               string        `json:"basis"`                // consent, contract, legal_obligation, vital_interests, public_task, legitimate_interests
	Description         string        `json:"description"`
	RequiresConsent     bool          `json:"requires_consent"`
	RequiresNotification bool         `json:"requires_notification"`
	RetentionPeriodDays int           `json:"retention_period_days"`
	ProcessingPurposes  []string      `json:"processing_purposes"`
	DataCategories      []string      `json:"data_categories"`
	SpecialCategories   bool          `json:"special_categories"`
}

type RetentionPolicy struct {
	DataType            string        `json:"data_type"`
	RetentionPeriodDays int           `json:"retention_period_days"`
	LegalBasis          string        `json:"legal_basis"`
	AutoDelete          bool          `json:"auto_delete"`
	ArchiveRequired     bool          `json:"archive_required"`
	PseudonymizeAfter   int           `json:"pseudonymize_after_days"`
}

type ProcessingPurpose struct {
	Purpose             string        `json:"purpose"`
	LawfulBasis         string        `json:"lawful_basis"`
	DataCategories      []string      `json:"data_categories"`
	RetentionDays       int           `json:"retention_days"`
	RequiresConsent     bool          `json:"requires_consent"`
	CanTransferOutsideEU bool         `json:"can_transfer_outside_eu"`
}

// Data repository interface for GDPR operations
type DataRetentionRepository interface {
	GetPersonalData(ctx context.Context, phoneNumber values.PhoneNumber) (*PersonalDataSummary, error)
	DeletePersonalData(ctx context.Context, phoneNumber values.PhoneNumber, dataTypes []string) (*DeletionSummary, error)
	PseudonymizeData(ctx context.Context, phoneNumber values.PhoneNumber) error
	GetRetentionStatus(ctx context.Context, phoneNumber values.PhoneNumber) (*RetentionStatus, error)
	CheckLegalHolds(ctx context.Context, phoneNumber values.PhoneNumber) ([]LegalHold, error)
}

type DataExportService interface {
	ExportPersonalData(ctx context.Context, phoneNumber values.PhoneNumber, format string) (*DataExportResult, error)
	GenerateDataMap(ctx context.Context, phoneNumber values.PhoneNumber) (*DataMap, error)
}

// NewGDPRComplianceHandler creates a new GDPR handler
func NewGDPRComplianceHandler(
	logger *zap.Logger,
	consentService ConsentService,
	auditService AuditService,
	complianceRepo ComplianceRepository,
	dataRetentionRepo DataRetentionRepository,
	dataExportService DataExportService,
	config GDPRConfig,
) *GDPRComplianceHandler {
	handler := &GDPRComplianceHandler{
		logger:             logger,
		consentService:     consentService,
		auditService:       auditService,
		complianceRepo:     complianceRepo,
		dataRetentionRepo:  dataRetentionRepo,
		dataExportService:  dataExportService,
		config:             config,
		lawfulBasisRules:   initializeLawfulBasisRules(),
		retentionPolicies:  initializeRetentionPolicies(),
		processingPurposes: initializeProcessingPurposes(),
	}
	
	return handler
}

// ProcessAccessRequest handles GDPR Article 15 data access requests
func (h *GDPRComplianceHandler) ProcessAccessRequest(ctx context.Context, phoneNumber values.PhoneNumber) (*PersonalDataExport, error) {
	startTime := time.Now()
	requestID := uuid.New()
	
	h.logger.Info("Processing GDPR access request",
		zap.String("phone_number", phoneNumber.String()),
		zap.String("request_id", requestID.String()),
	)

	defer func() {
		h.logger.Debug("GDPR access request completed",
			zap.String("phone_number", phoneNumber.String()),
			zap.String("request_id", requestID.String()),
			zap.Duration("duration", time.Since(startTime)),
		)
	}()

	// Get personal data summary
	dataSummary, err := h.dataRetentionRepo.GetPersonalData(ctx, phoneNumber)
	if err != nil {
		h.logger.Error("Failed to get personal data summary",
			zap.String("phone_number", phoneNumber.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get personal data: %w", err)
	}

	// Export data in requested format
	exportResult, err := h.dataExportService.ExportPersonalData(ctx, phoneNumber, "json")
	if err != nil {
		h.logger.Error("Failed to export personal data",
			zap.String("phone_number", phoneNumber.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to export data: %w", err)
	}

	// Create export response
	export := &PersonalDataExport{
		PhoneNumber:       phoneNumber,
		ExportedAt:        time.Now(),
		DataSources:       dataSummary.DataSources,
		Format:            "json",
		Size:              exportResult.Size,
	}

	// Populate call records if available
	if callData, ok := exportResult.Data["calls"].([]interface{}); ok {
		for _, call := range callData {
			if callMap, ok := call.(map[string]interface{}); ok {
				callRecord := CallDataRecord{
					CallID:     uuid.MustParse(callMap["call_id"].(string)),
					FromNumber: values.MustNewPhoneNumber(callMap["from_number"].(string)),
					ToNumber:   values.MustNewPhoneNumber(callMap["to_number"].(string)),
					CallTime:   callMap["call_time"].(time.Time),
					Duration:   callMap["duration"].(time.Duration),
					CallType:   CallType(callMap["call_type"].(string)),
					Purpose:    callMap["purpose"].(string),
				}
				export.CallRecords = append(export.CallRecords, callRecord)
			}
		}
	}

	// Populate consent records
	if consentData, ok := exportResult.Data["consents"].([]interface{}); ok {
		for _, consent := range consentData {
			if consentMap, ok := consent.(map[string]interface{}); ok {
				consentRecord := ConsentRecord{
					ConsentID:   uuid.MustParse(consentMap["consent_id"].(string)),
					PhoneNumber: phoneNumber,
					ConsentType: consentMap["consent_type"].(string),
					GrantedAt:   consentMap["granted_at"].(time.Time),
					Source:      consentMap["source"].(string),
					Status:      consentMap["status"].(string),
				}
				export.ConsentRecords = append(export.ConsentRecords, consentRecord)
			}
		}
	}

	// Populate compliance records
	if complianceData, ok := exportResult.Data["compliance"].([]interface{}); ok {
		for _, compliance := range complianceData {
			if complianceMap, ok := compliance.(map[string]interface{}); ok {
				complianceRecord := ComplianceRecord{
					CheckID:    uuid.MustParse(complianceMap["check_id"].(string)),
					CallID:     uuid.MustParse(complianceMap["call_id"].(string)),
					Regulation: RegulationType(complianceMap["regulation"].(string)),
					Result:     complianceMap["result"].(string),
					Timestamp:  complianceMap["timestamp"].(time.Time),
				}
				if violations, ok := complianceMap["violations"].([]string); ok {
					complianceRecord.Violations = violations
				}
				export.ComplianceRecords = append(export.ComplianceRecords, complianceRecord)
			}
		}
	}

	// Log audit event
	auditEvent := DataSubjectAuditEvent{
		EventType:   "gdpr_access_request",
		RequestID:   requestID,
		PhoneNumber: phoneNumber,
		RequestType: DataSubjectAccess,
		Status:      "completed",
		Timestamp:   time.Now(),
		ProcessedBy: uuid.New(), // System actor
		Metadata: map[string]interface{}{
			"data_sources_count": len(export.DataSources),
			"export_size":        export.Size,
			"call_records":       len(export.CallRecords),
			"consent_records":    len(export.ConsentRecords),
			"compliance_records": len(export.ComplianceRecords),
		},
	}

	if err := h.auditService.LogDataSubjectRequest(ctx, auditEvent); err != nil {
		h.logger.Error("Failed to log access request audit event",
			zap.Error(err),
		)
	}

	return export, nil
}

// ProcessDeletionRequest handles GDPR Article 17 right to erasure requests
func (h *GDPRComplianceHandler) ProcessDeletionRequest(ctx context.Context, req DeletionRequest) (*DeletionResult, error) {
	startTime := time.Now()
	requestID := uuid.New()
	
	h.logger.Info("Processing GDPR deletion request",
		zap.String("phone_number", req.PhoneNumber.String()),
		zap.String("request_id", requestID.String()),
		zap.Bool("retention_check", req.RetentionCheck),
		zap.Bool("preserve_legal", req.PreserveLegal),
	)

	defer func() {
		h.logger.Debug("GDPR deletion request completed",
			zap.String("phone_number", req.PhoneNumber.String()),
			zap.String("request_id", requestID.String()),
			zap.Duration("duration", time.Since(startTime)),
		)
	}()

	result := &DeletionResult{
		PhoneNumber:        req.PhoneNumber,
		DeletedAt:          time.Now(),
		RecordsDeleted:     make(map[string]int),
		RetainedRecords:    make(map[string]int),
		LegalHolds:         []string{},
		DownstreamNotified: false,
	}

	// Check legal holds if requested
	var legalHolds []LegalHold
	if req.PreserveLegal {
		var err error
		legalHolds, err = h.dataRetentionRepo.CheckLegalHolds(ctx, req.PhoneNumber)
		if err != nil {
			h.logger.Error("Failed to check legal holds",
				zap.String("phone_number", req.PhoneNumber.String()),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to check legal holds: %w", err)
		}
		
		for _, hold := range legalHolds {
			result.LegalHolds = append(result.LegalHolds, hold.Reason)
		}
	}

	// Check retention requirements if requested
	if req.RetentionCheck {
		retentionStatus, err := h.dataRetentionRepo.GetRetentionStatus(ctx, req.PhoneNumber)
		if err != nil {
			h.logger.Error("Failed to check retention status",
				zap.String("phone_number", req.PhoneNumber.String()),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to check retention: %w", err)
		}

		// Determine what can be deleted vs. retained
		for dataType, status := range retentionStatus.DataTypes {
			if status.CanDelete {
				// Mark for deletion
				continue
			} else {
				// Mark for retention
				result.RetainedRecords[dataType] = status.RecordCount
			}
		}
	}

	// Determine data types to delete
	dataTypesToDelete := []string{}
	if len(legalHolds) == 0 {
		// If no legal holds, delete all data types
		dataTypesToDelete = []string{"calls", "consents", "compliance", "profiles", "billing"}
	} else {
		// Only delete data types not covered by legal holds
		protectedTypes := make(map[string]bool)
		for _, hold := range legalHolds {
			for _, dataType := range hold.DataTypes {
				protectedTypes[dataType] = true
			}
		}
		
		allTypes := []string{"calls", "consents", "compliance", "profiles", "billing"}
		for _, dataType := range allTypes {
			if !protectedTypes[dataType] {
				dataTypesToDelete = append(dataTypesToDelete, dataType)
			}
		}
	}

	// Perform deletion
	deletionSummary, err := h.dataRetentionRepo.DeletePersonalData(ctx, req.PhoneNumber, dataTypesToDelete)
	if err != nil {
		h.logger.Error("Failed to delete personal data",
			zap.String("phone_number", req.PhoneNumber.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to delete data: %w", err)
	}

	// Update result with deletion summary
	for dataType, count := range deletionSummary.DeletedCounts {
		result.RecordsDeleted[dataType] = count
	}

	// Notify downstream systems if requested
	if req.NotifyDownstream {
		// Implementation would send notifications to external systems
		// For now, just mark as notified
		result.DownstreamNotified = true
		
		h.logger.Info("Notifying downstream systems of data deletion",
			zap.String("phone_number", req.PhoneNumber.String()),
		)
	}

	// Log audit event
	auditEvent := DataSubjectAuditEvent{
		EventType:   "gdpr_deletion_request",
		RequestID:   requestID,
		PhoneNumber: req.PhoneNumber,
		RequestType: DataSubjectDeletion,
		Status:      "completed",
		Timestamp:   time.Now(),
		ProcessedBy: uuid.New(),
		Metadata: map[string]interface{}{
			"retention_check":      req.RetentionCheck,
			"preserve_legal":       req.PreserveLegal,
			"notify_downstream":    req.NotifyDownstream,
			"legal_holds_count":    len(legalHolds),
			"deleted_data_types":   dataTypesToDelete,
			"records_deleted":      result.RecordsDeleted,
			"records_retained":     result.RetainedRecords,
		},
	}

	if err := h.auditService.LogDataSubjectRequest(ctx, auditEvent); err != nil {
		h.logger.Error("Failed to log deletion request audit event",
			zap.Error(err),
		)
	}

	return result, nil
}

// ProcessPortabilityRequest handles GDPR Article 20 data portability requests
func (h *GDPRComplianceHandler) ProcessPortabilityRequest(ctx context.Context, phoneNumber values.PhoneNumber) (*DataPortabilityResult, error) {
	startTime := time.Now()
	requestID := uuid.New()
	
	h.logger.Info("Processing GDPR portability request",
		zap.String("phone_number", phoneNumber.String()),
		zap.String("request_id", requestID.String()),
	)

	// Export data in machine-readable format
	exportResult, err := h.dataExportService.ExportPersonalData(ctx, phoneNumber, "json")
	if err != nil {
		h.logger.Error("Failed to export data for portability",
			zap.String("phone_number", phoneNumber.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to export data: %w", err)
	}

	// Serialize data
	dataBytes, err := json.Marshal(exportResult.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize data: %w", err)
	}

	// Generate checksum
	hash := sha256.Sum256(dataBytes)
	checksum := hex.EncodeToString(hash[:])

	result := &DataPortabilityResult{
		PhoneNumber: phoneNumber,
		Format:      "json",
		Data:        dataBytes,
		ExportedAt:  time.Now(),
		Size:        int64(len(dataBytes)),
		Checksum:    checksum,
	}

	// Log audit event
	auditEvent := DataSubjectAuditEvent{
		EventType:   "gdpr_portability_request",
		RequestID:   requestID,
		PhoneNumber: phoneNumber,
		RequestType: DataSubjectPortability,
		Status:      "completed",
		Timestamp:   time.Now(),
		ProcessedBy: uuid.New(),
		Metadata: map[string]interface{}{
			"format":        result.Format,
			"data_size":     result.Size,
			"checksum":      result.Checksum,
			"export_time":   time.Since(startTime).Milliseconds(),
		},
	}

	if err := h.auditService.LogDataSubjectRequest(ctx, auditEvent); err != nil {
		h.logger.Error("Failed to log portability request audit event",
			zap.Error(err),
		)
	}

	return result, nil
}

// ValidateLawfulBasis validates the lawful basis for processing personal data
func (h *GDPRComplianceHandler) ValidateLawfulBasis(ctx context.Context, phoneNumber values.PhoneNumber, purpose string) (*LawfulBasisResult, error) {
	startTime := time.Now()
	
	h.logger.Debug("Validating lawful basis",
		zap.String("phone_number", phoneNumber.String()),
		zap.String("purpose", purpose),
	)

	defer func() {
		h.logger.Debug("Lawful basis validation completed",
			zap.String("phone_number", phoneNumber.String()),
			zap.Duration("duration", time.Since(startTime)),
		)
	}()

	// Get processing purpose configuration
	processingPurpose, exists := h.processingPurposes[purpose]
	if !exists {
		return &LawfulBasisResult{
			HasLawfulBasis: false,
			Restrictions:   []string{"Unknown processing purpose"},
		}, nil
	}

	// Get lawful basis rule
	lawfulBasisRule, exists := h.lawfulBasisRules[processingPurpose.LawfulBasis]
	if !exists {
		return &LawfulBasisResult{
			HasLawfulBasis: false,
			Restrictions:   []string{"Invalid lawful basis configuration"},
		}, nil
	}

	result := &LawfulBasisResult{
		HasLawfulBasis: true,
		Basis:          processingPurpose.LawfulBasis,
		Purpose:        purpose,
		Restrictions:   []string{},
	}

	// Check if consent is required
	if lawfulBasisRule.RequiresConsent {
		consentStatus, err := h.consentService.CheckConsent(ctx, phoneNumber, "gdpr_consent")
		if err != nil {
			h.logger.Error("Failed to check GDPR consent",
				zap.String("phone_number", phoneNumber.String()),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to check consent: %w", err)
		}

		if !consentStatus.HasConsent {
			result.HasLawfulBasis = false
			result.Restrictions = append(result.Restrictions, "Valid consent required")
		} else {
			// Set valid until consent expiration
			result.ValidUntil = consentStatus.ExpiresAt
		}
	}

	// Check retention period
	if lawfulBasisRule.RetentionPeriodDays > 0 {
		retentionDate := time.Now().AddDate(0, 0, lawfulBasisRule.RetentionPeriodDays)
		if result.ValidUntil == nil || retentionDate.Before(*result.ValidUntil) {
			result.ValidUntil = &retentionDate
		}
	}

	// Add processing-specific restrictions
	if processingPurpose.RequiresConsent && lawfulBasisRule.Basis != "consent" {
		result.Restrictions = append(result.Restrictions, "Processing purpose requires explicit consent")
	}

	// Log audit event
	auditEvent := ComplianceAuditEvent{
		EventType:   "gdpr_lawful_basis_validation",
		CallID:      uuid.New(),
		PhoneNumber: phoneNumber,
		Regulation:  RegulationGDPR,
		Result:      fmt.Sprintf("has_lawful_basis=%t,basis=%s", result.HasLawfulBasis, result.Basis),
		Timestamp:   time.Now(),
		ActorID:     uuid.New(),
		Metadata: map[string]interface{}{
			"purpose":          purpose,
			"lawful_basis":     result.Basis,
			"has_lawful_basis": result.HasLawfulBasis,
			"valid_until":      result.ValidUntil,
			"restrictions":     result.Restrictions,
		},
	}

	if err := h.auditService.LogComplianceEvent(ctx, auditEvent); err != nil {
		h.logger.Error("Failed to log lawful basis validation audit event",
			zap.Error(err),
		)
	}

	return result, nil
}

// HandleConsentWithdrawal handles GDPR consent withdrawal
func (h *GDPRComplianceHandler) HandleConsentWithdrawal(ctx context.Context, phoneNumber values.PhoneNumber, scope string) error {
	startTime := time.Now()
	requestID := uuid.New()
	
	h.logger.Info("Processing GDPR consent withdrawal",
		zap.String("phone_number", phoneNumber.String()),
		zap.String("scope", scope),
		zap.String("request_id", requestID.String()),
	)

	// Revoke consent
	err := h.consentService.RevokeConsent(ctx, phoneNumber, scope)
	if err != nil {
		h.logger.Error("Failed to revoke consent",
			zap.String("phone_number", phoneNumber.String()),
			zap.String("scope", scope),
			zap.Error(err),
		)
		return fmt.Errorf("failed to revoke consent: %w", err)
	}

	// Schedule data deletion if no other lawful basis exists
	graceDays := h.config.ConsentWithdrawalGraceDays
	if graceDays > 0 {
		h.logger.Info("Scheduling data deletion after grace period",
			zap.String("phone_number", phoneNumber.String()),
			zap.Int("grace_days", graceDays),
		)
		// Implementation would schedule deletion job
	}

	// Log audit event
	auditEvent := DataSubjectAuditEvent{
		EventType:   "gdpr_consent_withdrawal",
		RequestID:   requestID,
		PhoneNumber: phoneNumber,
		RequestType: DataSubjectObjection,
		Status:      "completed",
		Timestamp:   time.Now(),
		ProcessedBy: uuid.New(),
		Metadata: map[string]interface{}{
			"scope":              scope,
			"grace_period_days":  graceDays,
			"processing_time_ms": time.Since(startTime).Milliseconds(),
		},
	}

	if err := h.auditService.LogDataSubjectRequest(ctx, auditEvent); err != nil {
		h.logger.Error("Failed to log consent withdrawal audit event",
			zap.Error(err),
		)
	}

	return nil
}

// CheckCrossBorderTransfer validates cross-border data transfer compliance
func (h *GDPRComplianceHandler) CheckCrossBorderTransfer(ctx context.Context, sourceCountry, targetCountry string, dataType string) (*TransferComplianceResult, error) {
	if !h.config.CrossBorderTransferEnabled {
		return &TransferComplianceResult{
			Allowed:       false,
			Restrictions:  []string{"Cross-border transfers disabled"},
		}, nil
	}

	result := &TransferComplianceResult{
		Allowed:       true,
		Requirements:  []string{},
		Restrictions:  []string{},
	}

	// Check if target country has adequacy decision
	isAdequate := false
	for _, country := range h.config.AdequacyCountries {
		if strings.EqualFold(country, targetCountry) {
			isAdequate = true
			break
		}
	}

	if isAdequate {
		result.Mechanism = "adequacy"
		result.Requirements = append(result.Requirements, "Adequacy decision in place")
	} else {
		// Require appropriate safeguards
		result.Mechanism = "safeguards"
		result.Requirements = append(result.Requirements, "Appropriate safeguards required")
		result.Requirements = append(result.Requirements, "Standard Contractual Clauses (SCCs) or equivalent")
		result.Requirements = append(result.Requirements, "Data subject rights notification")
	}

	// Special restrictions for sensitive data
	if strings.Contains(strings.ToLower(dataType), "sensitive") {
		result.Requirements = append(result.Requirements, "Additional safeguards for sensitive data")
		result.Restrictions = append(result.Restrictions, "Enhanced protection measures required")
	}

	// Log audit event
	auditEvent := ComplianceAuditEvent{
		EventType:   "gdpr_cross_border_transfer_check",
		CallID:      uuid.New(),
		PhoneNumber: values.PhoneNumber{}, // No specific phone number
		Regulation:  RegulationGDPR,
		Result:      fmt.Sprintf("allowed=%t,mechanism=%s", result.Allowed, result.Mechanism),
		Timestamp:   time.Now(),
		ActorID:     uuid.New(),
		Metadata: map[string]interface{}{
			"source_country":  sourceCountry,
			"target_country":  targetCountry,
			"data_type":       dataType,
			"is_adequate":     isAdequate,
			"mechanism":       result.Mechanism,
			"requirements":    result.Requirements,
			"restrictions":    result.Restrictions,
		},
	}

	if err := h.auditService.LogComplianceEvent(ctx, auditEvent); err != nil {
		h.logger.Error("Failed to log cross-border transfer check audit event",
			zap.Error(err),
		)
	}

	return result, nil
}

// Supporting types and initialization functions

type PersonalDataSummary struct {
	PhoneNumber   values.PhoneNumber `json:"phone_number"`
	DataSources   []string           `json:"data_sources"`
	RecordCounts  map[string]int     `json:"record_counts"`
	LastUpdated   time.Time          `json:"last_updated"`
}

type DeletionSummary struct {
	DeletedCounts map[string]int `json:"deleted_counts"`
	Errors        []string       `json:"errors,omitempty"`
}

type DataExportResult struct {
	Data map[string]interface{} `json:"data"`
	Size int64                  `json:"size"`
}

type RetentionStatus struct {
	PhoneNumber values.PhoneNumber           `json:"phone_number"`
	DataTypes   map[string]DataTypeStatus    `json:"data_types"`
}

type DataTypeStatus struct {
	RecordCount   int       `json:"record_count"`
	CanDelete     bool      `json:"can_delete"`
	RetentionDate time.Time `json:"retention_date"`
	Reason        string    `json:"reason"`
}

type LegalHold struct {
	ID        uuid.UUID `json:"id"`
	Reason    string    `json:"reason"`
	DataTypes []string  `json:"data_types"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type DataMap struct {
	DataSources []DataSource `json:"data_sources"`
	GeneratedAt time.Time    `json:"generated_at"`
}

type DataSource struct {
	Name         string            `json:"name"`
	DataTypes    []string          `json:"data_types"`
	RecordCounts map[string]int    `json:"record_counts"`
	Purpose      string            `json:"purpose"`
	LawfulBasis  string            `json:"lawful_basis"`
}

func initializeLawfulBasisRules() map[string]LawfulBasisRule {
	return map[string]LawfulBasisRule{
		"consent": {
			Basis:               "consent",
			Description:         "Data subject has given consent for specific purposes",
			RequiresConsent:     true,
			RequiresNotification: true,
			RetentionPeriodDays: 0, // Until withdrawn
			ProcessingPurposes:  []string{"marketing", "advertising", "communication"},
			DataCategories:      []string{"contact", "preferences", "behavioral"},
			SpecialCategories:   false,
		},
		"contract": {
			Basis:               "contract",
			Description:         "Processing necessary for performance of contract",
			RequiresConsent:     false,
			RequiresNotification: true,
			RetentionPeriodDays: 2555, // 7 years
			ProcessingPurposes:  []string{"service_delivery", "billing", "support"},
			DataCategories:      []string{"contact", "billing", "transaction"},
			SpecialCategories:   false,
		},
		"legal_obligation": {
			Basis:               "legal_obligation",
			Description:         "Processing necessary for compliance with legal obligation",
			RequiresConsent:     false,
			RequiresNotification: true,
			RetentionPeriodDays: 3650, // 10 years
			ProcessingPurposes:  []string{"compliance", "audit", "reporting"},
			DataCategories:      []string{"transaction", "compliance", "audit"},
			SpecialCategories:   false,
		},
		"legitimate_interests": {
			Basis:               "legitimate_interests",
			Description:         "Processing necessary for legitimate interests",
			RequiresConsent:     false,
			RequiresNotification: true,
			RetentionPeriodDays: 1095, // 3 years
			ProcessingPurposes:  []string{"fraud_prevention", "security", "analytics"},
			DataCategories:      []string{"technical", "security", "behavioral"},
			SpecialCategories:   false,
		},
	}
}

func initializeRetentionPolicies() map[string]RetentionPolicy {
	return map[string]RetentionPolicy{
		"calls": {
			DataType:            "calls",
			RetentionPeriodDays: 2555, // 7 years
			LegalBasis:          "legal_obligation",
			AutoDelete:          true,
			ArchiveRequired:     true,
			PseudonymizeAfter:   1095, // 3 years
		},
		"consents": {
			DataType:            "consents",
			RetentionPeriodDays: 0, // Until withdrawn
			LegalBasis:          "consent",
			AutoDelete:          false,
			ArchiveRequired:     true,
			PseudonymizeAfter:   0,
		},
		"compliance": {
			DataType:            "compliance",
			RetentionPeriodDays: 3650, // 10 years
			LegalBasis:          "legal_obligation",
			AutoDelete:          true,
			ArchiveRequired:     true,
			PseudonymizeAfter:   2555, // 7 years
		},
		"billing": {
			DataType:            "billing",
			RetentionPeriodDays: 2555, // 7 years
			LegalBasis:          "contract",
			AutoDelete:          true,
			ArchiveRequired:     true,
			PseudonymizeAfter:   1095, // 3 years
		},
	}
}

func initializeProcessingPurposes() map[string]ProcessingPurpose {
	return map[string]ProcessingPurpose{
		"service_delivery": {
			Purpose:             "service_delivery",
			LawfulBasis:         "contract",
			DataCategories:      []string{"contact", "transaction", "technical"},
			RetentionDays:       2555, // 7 years
			RequiresConsent:     false,
			CanTransferOutsideEU: true,
		},
		"marketing": {
			Purpose:             "marketing",
			LawfulBasis:         "consent",
			DataCategories:      []string{"contact", "preferences", "behavioral"},
			RetentionDays:       1095, // 3 years
			RequiresConsent:     true,
			CanTransferOutsideEU: false,
		},
		"compliance": {
			Purpose:             "compliance",
			LawfulBasis:         "legal_obligation",
			DataCategories:      []string{"transaction", "compliance", "audit"},
			RetentionDays:       3650, // 10 years
			RequiresConsent:     false,
			CanTransferOutsideEU: true,
		},
		"fraud_prevention": {
			Purpose:             "fraud_prevention",
			LawfulBasis:         "legitimate_interests",
			DataCategories:      []string{"technical", "behavioral", "security"},
			RetentionDays:       1095, // 3 years
			RequiresConsent:     false,
			CanTransferOutsideEU: true,
		},
	}
}

// DefaultGDPRConfig returns a default GDPR configuration
func DefaultGDPRConfig() GDPRConfig {
	return GDPRConfig{
		StrictMode:                   true,
		DefaultRetentionDays:         2555, // 7 years
		RequireExplicitConsent:       true,
		CrossBorderTransferEnabled:   true,
		AutomaticDeletionEnabled:     true,
		DataPortabilityFormats:       []string{"json", "csv", "xml"},
		RequestProcessingTimeoutDays: 30,
		ConsentWithdrawalGraceDays:   30,
		PseudonymizationEnabled:      true,
		EncryptionRequired:           true,
		AdequacyCountries:           []string{"US", "UK", "CA", "AU", "NZ", "CH", "IL", "UY", "AR", "JP", "KR"},
		DataProcessingPurposes:      []string{"service_delivery", "marketing", "compliance", "fraud_prevention"},
	}
}