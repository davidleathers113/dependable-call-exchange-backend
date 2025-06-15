package compliance

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// Service implements the comprehensive compliance service
type Service struct {
	logger           *zap.Logger
	tcpaValidator    TCPAValidator
	gdprHandler      GDPRHandler
	reporter         ComplianceReporter
	consentService   ConsentService
	auditService     AuditService
	geoService       GeolocationService
	complianceRepo   ComplianceRepository
	
	// Service configuration
	config           ServiceConfig
}

// ServiceConfig holds the main compliance service configuration
type ServiceConfig struct {
	TCPAConfig       TCPAConfig       `json:"tcpa_config"`
	GDPRConfig       GDPRConfig       `json:"gdpr_config"`
	ReporterConfig   ReporterConfig   `json:"reporter_config"`
	FailClosedMode   bool             `json:"fail_closed_mode"`
	DefaultTimeout   time.Duration    `json:"default_timeout"`
	EnableParallelChecks bool         `json:"enable_parallel_checks"`
	CacheEnabled     bool             `json:"cache_enabled"`
	CacheTTL         time.Duration    `json:"cache_ttl"`
	MetricsEnabled   bool             `json:"metrics_enabled"`
}

// NewService creates a new compliance service with all validators and handlers
func NewService(
	logger *zap.Logger,
	consentService ConsentService,
	auditService AuditService,
	geoService GeolocationService,
	complianceRepo ComplianceRepository,
	dataRetentionRepo DataRetentionRepository,
	dataExportService DataExportService,
	alertService AlertService,
	certificateService CertificateService,
	config ServiceConfig,
) *Service {
	// Create TCPA validator
	tcpaValidator := NewTCPAComplianceValidator(
		logger.Named("tcpa"),
		consentService,
		auditService,
		geoService,
		complianceRepo,
		config.TCPAConfig,
	)

	// Create GDPR handler
	gdprHandler := NewGDPRComplianceHandler(
		logger.Named("gdpr"),
		consentService,
		auditService,
		complianceRepo,
		dataRetentionRepo,
		dataExportService,
		config.GDPRConfig,
	)

	// Create compliance reporter
	reporter := NewComplianceReporter(
		logger.Named("reporter"),
		complianceRepo,
		auditService,
		alertService,
		certificateService,
		config.ReporterConfig,
	)

	return &Service{
		logger:         logger,
		tcpaValidator:  tcpaValidator,
		gdprHandler:    gdprHandler,
		reporter:       reporter,
		consentService: consentService,
		auditService:   auditService,
		geoService:     geoService,
		complianceRepo: complianceRepo,
		config:         config,
	}
}

// ValidateTCPA validates TCPA compliance for a call
func (s *Service) ValidateTCPA(ctx context.Context, req TCPAValidationRequest) (*ComplianceResult, error) {
	startTime := time.Now()
	
	s.logger.Info("Validating TCPA compliance",
		zap.String("call_id", req.CallID.String()),
		zap.String("from_number", req.FromNumber.String()),
		zap.String("to_number", req.ToNumber.String()),
		zap.String("call_type", string(req.CallType)),
	)

	result := &ComplianceResult{
		Approved:    true,
		Violations:  []*compliance.ComplianceViolation{},
		Warnings:    []ComplianceWarning{},
		CheckID:     uuid.New(),
		Timestamp:   time.Now(),
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, s.config.DefaultTimeout)
	defer cancel()

	// Perform TCPA validations
	var validationErrors []error

	// 1. Time restrictions validation
	timeReq := TimeValidationRequest{
		PhoneNumber: req.ToNumber,
		CallTime:    req.CallTime,
		Location:    req.Location,
		CallType:    req.CallType,
	}

	timeResult, err := s.tcpaValidator.ValidateTimeRestrictions(timeoutCtx, timeReq)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("time validation failed: %w", err))
		if s.config.FailClosedMode {
			result.Approved = false
		}
	} else if !timeResult.Allowed {
		violation := &compliance.ComplianceViolation{
			ID:            uuid.New(),
			CallID:        req.CallID,
			ViolationType: compliance.ViolationTimeRestriction,
			Severity:      compliance.SeverityHigh,
			Description:   timeResult.Reason,
			Resolved:      false,
			CreatedAt:     time.Now(),
		}
		result.Violations = append(result.Violations, violation)
		result.Approved = false
	}

	// 2. Wireless consent validation
	consentResult, err := s.tcpaValidator.ValidateWirelessConsent(timeoutCtx, req.ToNumber)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("consent validation failed: %w", err))
		if s.config.FailClosedMode {
			result.Approved = false
		}
	} else if consentResult.IsWireless && !consentResult.HasConsent {
		violation := &compliance.ComplianceViolation{
			ID:            uuid.New(),
			CallID:        req.CallID,
			ViolationType: compliance.ViolationConsent,
			Severity:      compliance.SeverityCritical,
			Description:   fmt.Sprintf("Wireless number requires %s consent", consentResult.RequiredType),
			Resolved:      false,
			CreatedAt:     time.Now(),
		}
		result.Violations = append(result.Violations, violation)
		result.Approved = false
	}

	// 3. State-specific rules validation
	if req.Location != nil {
		stateResult, err := s.tcpaValidator.CheckStateSpecificRules(timeoutCtx, *req.Location, req.CallType)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("state rules validation failed: %w", err))
			if s.config.FailClosedMode {
				result.Approved = false
			}
		} else if !stateResult.Compliant {
			violation := &compliance.ComplianceViolation{
				ID:            uuid.New(),
				CallID:        req.CallID,
				ViolationType: compliance.ViolationTCPA,
				Severity:      compliance.SeverityMedium,
				Description:   "State-specific TCPA rules violation",
				Resolved:      false,
				CreatedAt:     time.Now(),
			}
			result.Violations = append(result.Violations, violation)
			result.Approved = false
		}
	}

	// 4. Caller ID validation if provided
	if req.CallerID != nil {
		err := s.tcpaValidator.ValidateCallerID(timeoutCtx, *req.CallerID, req.FromNumber)
		if err != nil {
			violation := &compliance.ComplianceViolation{
				ID:            uuid.New(),
				CallID:        req.CallID,
				ViolationType: compliance.ViolationTCPA,
				Severity:      compliance.SeverityHigh,
				Description:   "Caller ID validation failed: " + err.Error(),
				Resolved:      false,
				CreatedAt:     time.Now(),
			}
			result.Violations = append(result.Violations, violation)
			result.Approved = false
		}
	}

	// Save violations to repository
	for _, violation := range result.Violations {
		if err := s.complianceRepo.SaveViolation(timeoutCtx, violation); err != nil {
			s.logger.Error("Failed to save violation",
				zap.String("violation_id", violation.ID.String()),
				zap.Error(err),
			)
		}
	}

	// Create compliance check record
	complianceCheck := &compliance.ComplianceCheck{
		CallID:        req.CallID,
		PhoneNumber:   req.ToNumber.String(),
		CallerID:      req.FromNumber.String(),
		TimeOfCall:    req.CallTime,
		ConsentStatus: compliance.ConsentStatusActive, // Would be determined from consent validation
		Violations:    result.Violations,
		Approved:      result.Approved,
	}

	if req.Location != nil {
		complianceCheck.Geography = *req.Location
	}

	if !result.Approved {
		complianceCheck.Reason = "TCPA compliance violations detected"
	}

	// Save compliance check
	if err := s.complianceRepo.SaveComplianceCheck(timeoutCtx, complianceCheck); err != nil {
		s.logger.Error("Failed to save compliance check",
			zap.String("check_id", result.CheckID.String()),
			zap.Error(err),
		)
	}

	result.ProcessTime = time.Since(startTime)

	s.logger.Info("TCPA compliance validation completed",
		zap.String("call_id", req.CallID.String()),
		zap.Bool("approved", result.Approved),
		zap.Int("violations", len(result.Violations)),
		zap.Duration("process_time", result.ProcessTime),
	)

	return result, nil
}

// CheckCallingHours validates TCPA calling hours
func (s *Service) CheckCallingHours(ctx context.Context, phoneNumber values.PhoneNumber, callTime time.Time) (*TimeValidationResult, error) {
	req := TimeValidationRequest{
		PhoneNumber: phoneNumber,
		CallTime:    callTime,
		CallType:    CallTypeMarketing, // Default to marketing for strictest rules
	}

	return s.tcpaValidator.ValidateTimeRestrictions(ctx, req)
}

// ValidateWirelessConsent validates consent for wireless numbers
func (s *Service) ValidateWirelessConsent(ctx context.Context, phoneNumber values.PhoneNumber) (*ConsentValidationResult, error) {
	return s.tcpaValidator.ValidateWirelessConsent(ctx, phoneNumber)
}

// ProcessDataSubjectRequest handles GDPR data subject requests
func (s *Service) ProcessDataSubjectRequest(ctx context.Context, req DataSubjectRequest) (*DataSubjectResponse, error) {
	startTime := time.Now()
	requestID := uuid.New()
	
	s.logger.Info("Processing data subject request",
		zap.String("request_id", requestID.String()),
		zap.String("phone_number", req.PhoneNumber.String()),
		zap.String("request_type", string(req.RequestType)),
	)

	response := &DataSubjectResponse{
		RequestID:   requestID,
		Status:      "processing",
		ProcessTime: 0,
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, s.config.DefaultTimeout)
	defer cancel()

	switch req.RequestType {
	case DataSubjectAccess:
		exportData, err := s.gdprHandler.ProcessAccessRequest(timeoutCtx, req.PhoneNumber)
		if err != nil {
			response.Status = "failed"
			response.Message = err.Error()
			return response, err
		}
		response.Status = "completed"
		response.Data = exportData
		response.CompletedAt = &exportData.ExportedAt

	case DataSubjectDeletion:
		deletionReq := DeletionRequest{
			PhoneNumber:      req.PhoneNumber,
			RetentionCheck:   true,
			PreserveLegal:    true,
			NotifyDownstream: true,
		}
		deletionResult, err := s.gdprHandler.ProcessDeletionRequest(timeoutCtx, deletionReq)
		if err != nil {
			response.Status = "failed"
			response.Message = err.Error()
			return response, err
		}
		response.Status = "completed"
		response.Data = deletionResult
		response.CompletedAt = &deletionResult.DeletedAt

	case DataSubjectPortability:
		portabilityResult, err := s.gdprHandler.ProcessPortabilityRequest(timeoutCtx, req.PhoneNumber)
		if err != nil {
			response.Status = "failed"
			response.Message = err.Error()
			return response, err
		}
		response.Status = "completed"
		response.Data = portabilityResult
		response.CompletedAt = &portabilityResult.ExportedAt

	default:
		response.Status = "failed"
		response.Message = "Unsupported request type"
		return response, fmt.Errorf("unsupported request type: %s", req.RequestType)
	}

	response.ProcessTime = time.Since(startTime)

	s.logger.Info("Data subject request completed",
		zap.String("request_id", requestID.String()),
		zap.String("status", response.Status),
		zap.Duration("process_time", response.ProcessTime),
	)

	return response, nil
}

// ValidateGDPRConsent validates GDPR consent for a specific purpose
func (s *Service) ValidateGDPRConsent(ctx context.Context, phoneNumber values.PhoneNumber, purpose string) (*GDPRConsentResult, error) {
	lawfulBasisResult, err := s.gdprHandler.ValidateLawfulBasis(ctx, phoneNumber, purpose)
	if err != nil {
		return nil, err
	}

	result := &GDPRConsentResult{
		HasValidConsent: lawfulBasisResult.HasLawfulBasis,
		LawfulBasis:     lawfulBasisResult.Basis,
		Purpose:         lawfulBasisResult.Purpose,
		CanProcess:      lawfulBasisResult.HasLawfulBasis,
		Restrictions:    lawfulBasisResult.Restrictions,
	}

	// If consent is the lawful basis, get consent details
	if lawfulBasisResult.Basis == "consent" {
		consentStatus, err := s.consentService.CheckConsent(ctx, phoneNumber, "gdpr_consent")
		if err != nil {
			s.logger.Error("Failed to check GDPR consent details",
				zap.String("phone_number", phoneNumber.String()),
				zap.Error(err),
			)
		} else {
			result.ConsentDate = consentStatus.GrantedAt
		}
	}

	return result, nil
}

// HandleConsentWithdrawal handles GDPR consent withdrawal
func (s *Service) HandleConsentWithdrawal(ctx context.Context, req ConsentWithdrawalRequest) error {
	return s.gdprHandler.HandleConsentWithdrawal(ctx, req.PhoneNumber, req.Scope)
}

// ExportPersonalData exports personal data for GDPR compliance
func (s *Service) ExportPersonalData(ctx context.Context, phoneNumber values.PhoneNumber) (*PersonalDataExport, error) {
	return s.gdprHandler.ProcessAccessRequest(ctx, phoneNumber)
}

// DeletePersonalData deletes personal data for GDPR compliance
func (s *Service) DeletePersonalData(ctx context.Context, phoneNumber values.PhoneNumber, retentionCheck bool) (*DeletionResult, error) {
	req := DeletionRequest{
		PhoneNumber:      phoneNumber,
		RetentionCheck:   retentionCheck,
		PreserveLegal:    true,
		NotifyDownstream: true,
	}
	return s.gdprHandler.ProcessDeletionRequest(ctx, req)
}

// PerformComplianceCheck performs comprehensive compliance validation
func (s *Service) PerformComplianceCheck(ctx context.Context, req ComplianceCheckRequest) (*ComplianceCheckResult, error) {
	startTime := time.Now()
	checkID := uuid.New()
	
	s.logger.Info("Performing comprehensive compliance check",
		zap.String("check_id", checkID.String()),
		zap.String("call_id", req.CallID.String()),
		zap.Strings("regulations", []string(req.Regulations)),
	)

	result := &ComplianceCheckResult{
		CheckID:     checkID,
		CallID:      req.CallID,
		Approved:    true,
		Regulations: []RegulationResult{},
		Violations:  []*compliance.ComplianceViolation{},
		Warnings:    []ComplianceWarning{},
		Timestamp:   time.Now(),
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, s.config.DefaultTimeout)
	defer cancel()

	// Check each requested regulation
	for _, regulation := range req.Regulations {
		regResult := RegulationResult{
			Regulation: regulation,
			Compliant:  true,
			Violations: []string{},
			Warnings:   []string{},
		}

		switch regulation {
		case RegulationTCPA:
			tcpaReq := TCPAValidationRequest{
				CallID:     req.CallID,
				FromNumber: req.FromNumber,
				ToNumber:   req.ToNumber,
				CallType:   req.CallType,
				CallTime:   req.CallTime,
				Location:   req.Location,
				Purpose:    req.Purpose,
			}

			tcpaResult, err := s.ValidateTCPA(timeoutCtx, tcpaReq)
			if err != nil {
				regResult.Compliant = false
				regResult.Violations = append(regResult.Violations, err.Error())
				if s.config.FailClosedMode {
					result.Approved = false
				}
			} else if !tcpaResult.Approved {
				regResult.Compliant = false
				for _, violation := range tcpaResult.Violations {
					regResult.Violations = append(regResult.Violations, violation.Description)
					result.Violations = append(result.Violations, violation)
				}
				result.Approved = false
			}

		case RegulationGDPR:
			gdprResult, err := s.ValidateGDPRConsent(timeoutCtx, req.ToNumber, req.Purpose)
			if err != nil {
				regResult.Compliant = false
				regResult.Violations = append(regResult.Violations, err.Error())
				if s.config.FailClosedMode {
					result.Approved = false
				}
			} else if !gdprResult.CanProcess {
				regResult.Compliant = false
				regResult.Violations = append(regResult.Violations, "GDPR consent validation failed")
				for _, restriction := range gdprResult.Restrictions {
					regResult.Violations = append(regResult.Violations, restriction)
				}
				
				violation := &compliance.ComplianceViolation{
					ID:            uuid.New(),
					CallID:        req.CallID,
					ViolationType: compliance.ViolationGDPR,
					Severity:      compliance.SeverityHigh,
					Description:   "GDPR compliance violation: " + strings.Join(gdprResult.Restrictions, "; "),
					Resolved:      false,
					CreatedAt:     time.Now(),
				}
				result.Violations = append(result.Violations, violation)
				result.Approved = false
			}

		// Add other regulations as needed
		default:
			regResult.Warnings = append(regResult.Warnings, "Regulation not implemented: "+string(regulation))
		}

		result.Regulations = append(result.Regulations, regResult)
	}

	// Save all violations
	for _, violation := range result.Violations {
		if err := s.complianceRepo.SaveViolation(timeoutCtx, violation); err != nil {
			s.logger.Error("Failed to save violation",
				zap.String("violation_id", violation.ID.String()),
				zap.Error(err),
			)
		}
	}

	result.ProcessTime = time.Since(startTime)

	s.logger.Info("Comprehensive compliance check completed",
		zap.String("check_id", checkID.String()),
		zap.Bool("approved", result.Approved),
		zap.Int("violations", len(result.Violations)),
		zap.Duration("process_time", result.ProcessTime),
	)

	return result, nil
}

// ValidateCallPermission validates if a call is permitted under all applicable regulations
func (s *Service) ValidateCallPermission(ctx context.Context, req CallPermissionRequest) (*CallPermissionResult, error) {
	checkReq := ComplianceCheckRequest{
		CallID:      req.CallID,
		FromNumber:  req.FromNumber,
		ToNumber:    req.ToNumber,
		CallType:    req.CallType,
		CallTime:    req.CallTime,
		Purpose:     req.Purpose,
		Regulations: req.Regulations,
	}

	checkResult, err := s.PerformComplianceCheck(ctx, checkReq)
	if err != nil {
		return nil, err
	}

	result := &CallPermissionResult{
		Permitted:  checkResult.Approved,
		Violations: checkResult.Violations,
		Conditions: []PermissionCondition{},
	}

	if !checkResult.Approved {
		result.Reason = "Compliance violations detected"
	}

	// Add conditions based on regulation results
	for _, regResult := range checkResult.Regulations {
		if !regResult.Compliant {
			for _, violation := range regResult.Violations {
				condition := PermissionCondition{
					Type:        "violation",
					Description: violation,
				}
				result.Conditions = append(result.Conditions, condition)
			}
		}
	}

	return result, nil
}

// GenerateComplianceReport generates compliance reports
func (s *Service) GenerateComplianceReport(ctx context.Context, req ComplianceReportRequest) (*ComplianceReport, error) {
	return s.reporter.GenerateComplianceReport(ctx, req)
}

// GetViolations retrieves compliance violations
func (s *Service) GetViolations(ctx context.Context, filters ViolationFilters) (*ViolationSummary, error) {
	violations, err := s.reporter.DetectViolations(ctx, filters)
	if err != nil {
		return nil, err
	}

	summary := &ViolationSummary{
		Total:      len(violations),
		ByType:     make(map[string]int),
		BySeverity: make(map[string]int),
		Resolved:   0,
		Pending:    0,
	}

	for _, violation := range violations {
		summary.ByType[violation.ViolationType.String()]++
		summary.BySeverity[violation.Severity.String()]++
		
		if violation.Resolved {
			summary.Resolved++
		} else {
			summary.Pending++
		}
	}

	return summary, nil
}

// MonitorCompliance starts real-time compliance monitoring
func (s *Service) MonitorCompliance(ctx context.Context, config MonitoringConfig) error {
	if !s.config.ReporterConfig.EnableRealTimeMonitoring {
		return fmt.Errorf("real-time monitoring is disabled")
	}

	alertChan, err := s.reporter.MonitorRealTimeCompliance(ctx, config)
	if err != nil {
		return err
	}

	// Process alerts in a separate goroutine
	go func() {
		for alert := range alertChan {
			s.logger.Warn("Compliance alert received",
				zap.String("alert_id", alert.AlertID.String()),
				zap.String("type", alert.Type),
				zap.String("severity", string(alert.Severity)),
				zap.String("message", alert.Message),
			)

			// Here you would typically:
			// 1. Send notifications
			// 2. Create incidents
			// 3. Take automated actions
			// 4. Update monitoring dashboards
		}
	}()

	return nil
}

// DefaultServiceConfig returns a default service configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		TCPAConfig:           DefaultTCPAConfig(),
		GDPRConfig:           DefaultGDPRConfig(),
		ReporterConfig:       DefaultReporterConfig(),
		FailClosedMode:       true, // Fail closed for compliance safety
		DefaultTimeout:       30 * time.Second,
		EnableParallelChecks: true,
		CacheEnabled:         true,
		CacheTTL:             5 * time.Minute,
		MetricsEnabled:       true,
	}
}