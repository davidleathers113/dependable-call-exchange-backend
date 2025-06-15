package compliance

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// ComplianceService provides comprehensive compliance validation and management
type ComplianceService interface {
	// TCPA Compliance
	ValidateTCPA(ctx context.Context, req TCPAValidationRequest) (*ComplianceResult, error)
	CheckCallingHours(ctx context.Context, phoneNumber values.PhoneNumber, callTime time.Time) (*TimeValidationResult, error)
	ValidateWirelessConsent(ctx context.Context, phoneNumber values.PhoneNumber) (*ConsentValidationResult, error)
	
	// GDPR Compliance
	ProcessDataSubjectRequest(ctx context.Context, req DataSubjectRequest) (*DataSubjectResponse, error)
	ValidateGDPRConsent(ctx context.Context, phoneNumber values.PhoneNumber, purpose string) (*GDPRConsentResult, error)
	HandleConsentWithdrawal(ctx context.Context, req ConsentWithdrawalRequest) error
	ExportPersonalData(ctx context.Context, phoneNumber values.PhoneNumber) (*PersonalDataExport, error)
	DeletePersonalData(ctx context.Context, phoneNumber values.PhoneNumber, retentionCheck bool) (*DeletionResult, error)
	
	// General Compliance
	PerformComplianceCheck(ctx context.Context, req ComplianceCheckRequest) (*ComplianceCheckResult, error)
	ValidateCallPermission(ctx context.Context, req CallPermissionRequest) (*CallPermissionResult, error)
	
	// Reporting and Monitoring
	GenerateComplianceReport(ctx context.Context, req ComplianceReportRequest) (*ComplianceReport, error)
	GetViolations(ctx context.Context, filters ViolationFilters) (*ViolationSummary, error)
	MonitorCompliance(ctx context.Context, config MonitoringConfig) error
}

// TCPAValidator handles TCPA-specific compliance validation
type TCPAValidator interface {
	ValidateTimeRestrictions(ctx context.Context, req TimeValidationRequest) (*TimeValidationResult, error)
	ValidateWirelessConsent(ctx context.Context, phoneNumber values.PhoneNumber) (*ConsentValidationResult, error)
	CheckStateSpecificRules(ctx context.Context, location compliance.Location, callType CallType) (*StateComplianceResult, error)
	ValidateCallerID(ctx context.Context, callerID, actualNumber values.PhoneNumber) error
}

// GDPRHandler handles GDPR-specific compliance operations
type GDPRHandler interface {
	ProcessAccessRequest(ctx context.Context, phoneNumber values.PhoneNumber) (*PersonalDataExport, error)
	ProcessDeletionRequest(ctx context.Context, req DeletionRequest) (*DeletionResult, error)
	ProcessPortabilityRequest(ctx context.Context, phoneNumber values.PhoneNumber) (*DataPortabilityResult, error)
	ValidateLawfulBasis(ctx context.Context, phoneNumber values.PhoneNumber, purpose string) (*LawfulBasisResult, error)
	HandleConsentWithdrawal(ctx context.Context, phoneNumber values.PhoneNumber, scope string) error
	CheckCrossBorderTransfer(ctx context.Context, sourceCountry, targetCountry string, dataType string) (*TransferComplianceResult, error)
}

// ComplianceReporter handles compliance monitoring and reporting
type ComplianceReporter interface {
	GenerateComplianceReport(ctx context.Context, req ComplianceReportRequest) (*ComplianceReport, error)
	DetectViolations(ctx context.Context, filters ViolationFilters) ([]*compliance.ComplianceViolation, error)
	GenerateComplianceCertificate(ctx context.Context, req CertificateRequest) (*ComplianceCertificate, error)
	MonitorRealTimeCompliance(ctx context.Context, config MonitoringConfig) (<-chan ComplianceAlert, error)
	AnalyzeComplianceTrends(ctx context.Context, req TrendAnalysisRequest) (*ComplianceTrends, error)
}

// Repository interfaces for data access
type ComplianceRepository interface {
	SaveComplianceCheck(ctx context.Context, check *compliance.ComplianceCheck) error
	GetComplianceCheck(ctx context.Context, callID uuid.UUID) (*compliance.ComplianceCheck, error)
	SaveViolation(ctx context.Context, violation *compliance.ComplianceViolation) error
	GetViolations(ctx context.Context, filters ViolationFilters) ([]*compliance.ComplianceViolation, error)
	GetComplianceMetrics(ctx context.Context, req MetricsRequest) (*ComplianceMetrics, error)
}

// External service interfaces
type ConsentService interface {
	CheckConsent(ctx context.Context, phoneNumber values.PhoneNumber, consentType string) (*ConsentStatus, error)
	RevokeConsent(ctx context.Context, phoneNumber values.PhoneNumber, scope string) error
	GetConsentHistory(ctx context.Context, phoneNumber values.PhoneNumber) ([]*ConsentRecord, error)
}

type AuditService interface {
	LogComplianceEvent(ctx context.Context, event ComplianceAuditEvent) error
	LogViolation(ctx context.Context, violation ViolationAuditEvent) error
	LogDataSubjectRequest(ctx context.Context, event DataSubjectAuditEvent) error
}

type GeolocationService interface {
	GetLocation(ctx context.Context, phoneNumber values.PhoneNumber) (*compliance.Location, error)
	GetTimezone(ctx context.Context, location compliance.Location) (string, error)
}

// Request/Response DTOs

type TCPAValidationRequest struct {
	CallID       uuid.UUID            `json:"call_id"`
	FromNumber   values.PhoneNumber   `json:"from_number"`
	ToNumber     values.PhoneNumber   `json:"to_number"`
	CallerID     *values.PhoneNumber  `json:"caller_id,omitempty"`
	CallType     CallType             `json:"call_type"`
	CallTime     time.Time            `json:"call_time"`
	Location     *compliance.Location `json:"location,omitempty"`
	Purpose      string               `json:"purpose"`
}

type CallPermissionRequest struct {
	CallID       uuid.UUID            `json:"call_id"`
	FromNumber   values.PhoneNumber   `json:"from_number"`
	ToNumber     values.PhoneNumber   `json:"to_number"`
	CallType     CallType             `json:"call_type"`
	CallTime     time.Time            `json:"call_time"`
	Purpose      string               `json:"purpose"`
	RequestorID  uuid.UUID            `json:"requestor_id"`
	Regulations  []RegulationType     `json:"regulations"`
}

type ComplianceCheckRequest struct {
	CallID       uuid.UUID            `json:"call_id"`
	FromNumber   values.PhoneNumber   `json:"from_number"`
	ToNumber     values.PhoneNumber   `json:"to_number"`
	CallType     CallType             `json:"call_type"`
	CallTime     time.Time            `json:"call_time"`
	Purpose      string               `json:"purpose"`
	Regulations  []RegulationType     `json:"regulations"`
	Location     *compliance.Location `json:"location,omitempty"`
}

type DataSubjectRequest struct {
	PhoneNumber values.PhoneNumber `json:"phone_number"`
	Email       string             `json:"email,omitempty"`
	RequestType DataSubjectRequestType `json:"request_type"`
	Purpose     string             `json:"purpose,omitempty"`
	Scope       string             `json:"scope,omitempty"`
	Format      string             `json:"format,omitempty"` // for portability requests
}

type ConsentWithdrawalRequest struct {
	PhoneNumber values.PhoneNumber `json:"phone_number"`
	ConsentType string             `json:"consent_type"`
	Scope       string             `json:"scope"`
	Reason      string             `json:"reason,omitempty"`
}

type TimeValidationRequest struct {
	PhoneNumber values.PhoneNumber   `json:"phone_number"`
	CallTime    time.Time            `json:"call_time"`
	Location    *compliance.Location `json:"location,omitempty"`
	CallType    CallType             `json:"call_type"`
}

type ComplianceReportRequest struct {
	ReportType   ComplianceReportType `json:"report_type"`
	StartDate    time.Time            `json:"start_date"`
	EndDate      time.Time            `json:"end_date"`
	Regulations  []RegulationType     `json:"regulations,omitempty"`
	Scope        string               `json:"scope,omitempty"`
	Format       string               `json:"format"`
	IncludeRaw   bool                 `json:"include_raw_data"`
}

type ViolationFilters struct {
	StartDate     *time.Time          `json:"start_date,omitempty"`
	EndDate       *time.Time          `json:"end_date,omitempty"`
	ViolationType []compliance.ViolationType `json:"violation_types,omitempty"`
	Severity      []compliance.Severity      `json:"severities,omitempty"`
	Resolved      *bool               `json:"resolved,omitempty"`
	AccountID     *uuid.UUID          `json:"account_id,omitempty"`
	Limit         int                 `json:"limit,omitempty"`
	Offset        int                 `json:"offset,omitempty"`
}

type DeletionRequest struct {
	PhoneNumber      values.PhoneNumber `json:"phone_number"`
	RetentionCheck   bool               `json:"retention_check"`
	PreserveLegal    bool               `json:"preserve_legal_holds"`
	NotifyDownstream bool               `json:"notify_downstream"`
}

type CertificateRequest struct {
	Regulations  []RegulationType `json:"regulations"`
	StartDate    time.Time        `json:"start_date"`
	EndDate      time.Time        `json:"end_date"`
	Scope        string           `json:"scope"`
	Requestor    string           `json:"requestor"`
}

type MonitoringConfig struct {
	Enabled         bool                        `json:"enabled"`
	CheckInterval   time.Duration               `json:"check_interval"`
	Regulations     []RegulationType            `json:"regulations"`
	AlertThresholds map[string]float64          `json:"alert_thresholds"`
	Notifications   []NotificationChannel       `json:"notifications"`
}

type TrendAnalysisRequest struct {
	StartDate    time.Time            `json:"start_date"`
	EndDate      time.Time            `json:"end_date"`
	Granularity  string               `json:"granularity"` // hour, day, week, month
	Regulations  []RegulationType     `json:"regulations,omitempty"`
	Metrics      []string             `json:"metrics"` // violations, checks, consent_rates
}

type MetricsRequest struct {
	StartDate    time.Time            `json:"start_date"`
	EndDate      time.Time            `json:"end_date"`
	Regulations  []RegulationType     `json:"regulations,omitempty"`
	GroupBy      string               `json:"group_by"` // day, week, month
}

// Response DTOs

type ComplianceResult struct {
	Approved    bool                         `json:"approved"`
	Violations  []*compliance.ComplianceViolation `json:"violations,omitempty"`
	Warnings    []ComplianceWarning          `json:"warnings,omitempty"`
	CheckID     uuid.UUID                    `json:"check_id"`
	Timestamp   time.Time                    `json:"timestamp"`
	ProcessTime time.Duration                `json:"process_time"`
}

type TimeValidationResult struct {
	Allowed     bool      `json:"allowed"`
	Reason      string    `json:"reason,omitempty"`
	LocalTime   time.Time `json:"local_time"`
	Timezone    string    `json:"timezone"`
	NextAllowed *time.Time `json:"next_allowed,omitempty"`
}

type ConsentValidationResult struct {
	HasConsent     bool               `json:"has_consent"`
	ConsentType    string             `json:"consent_type,omitempty"`
	ConsentDate    *time.Time         `json:"consent_date,omitempty"`
	ExpirationDate *time.Time         `json:"expiration_date,omitempty"`
	Source         string             `json:"source,omitempty"`
	IsWireless     bool               `json:"is_wireless"`
	RequiredType   string             `json:"required_type,omitempty"`
}

type StateComplianceResult struct {
	Compliant      bool     `json:"compliant"`
	ApplicableRules []string `json:"applicable_rules,omitempty"`
	Restrictions   []string `json:"restrictions,omitempty"`
	Requirements   []string `json:"requirements,omitempty"`
}

type GDPRConsentResult struct {
	HasValidConsent bool       `json:"has_valid_consent"`
	LawfulBasis     string     `json:"lawful_basis,omitempty"`
	ConsentDate     *time.Time `json:"consent_date,omitempty"`
	Purpose         string     `json:"purpose,omitempty"`
	CanProcess      bool       `json:"can_process"`
	Restrictions    []string   `json:"restrictions,omitempty"`
}

type DataSubjectResponse struct {
	RequestID   uuid.UUID `json:"request_id"`
	Status      string    `json:"status"`
	Message     string    `json:"message,omitempty"`
	ProcessTime time.Duration `json:"process_time"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Data        interface{} `json:"data,omitempty"` // for access/portability requests
}

type PersonalDataExport struct {
	PhoneNumber   values.PhoneNumber `json:"phone_number"`
	ExportedAt    time.Time          `json:"exported_at"`
	DataSources   []string           `json:"data_sources"`
	CallRecords   []CallDataRecord   `json:"call_records,omitempty"`
	ConsentRecords []ConsentRecord   `json:"consent_records,omitempty"`
	ComplianceRecords []ComplianceRecord `json:"compliance_records,omitempty"`
	Format        string             `json:"format"`
	Size          int64              `json:"size"`
}

type DeletionResult struct {
	PhoneNumber    values.PhoneNumber `json:"phone_number"`
	DeletedAt      time.Time          `json:"deleted_at"`
	RecordsDeleted map[string]int     `json:"records_deleted"`
	RetainedRecords map[string]int    `json:"retained_records"`
	LegalHolds     []string           `json:"legal_holds,omitempty"`
	DownstreamNotified bool           `json:"downstream_notified"`
}

type DataPortabilityResult struct {
	PhoneNumber values.PhoneNumber `json:"phone_number"`
	Format      string             `json:"format"`
	Data        []byte             `json:"data"`
	ExportedAt  time.Time          `json:"exported_at"`
	Size        int64              `json:"size"`
	Checksum    string             `json:"checksum"`
}

type LawfulBasisResult struct {
	HasLawfulBasis bool     `json:"has_lawful_basis"`
	Basis          string   `json:"basis,omitempty"`
	Purpose        string   `json:"purpose,omitempty"`
	ValidUntil     *time.Time `json:"valid_until,omitempty"`
	Restrictions   []string `json:"restrictions,omitempty"`
}

type TransferComplianceResult struct {
	Allowed        bool     `json:"allowed"`
	Mechanism      string   `json:"mechanism,omitempty"` // adequacy, safeguards, derogation
	Requirements   []string `json:"requirements,omitempty"`
	Restrictions   []string `json:"restrictions,omitempty"`
}

type ComplianceCheckResult struct {
	CheckID      uuid.UUID                    `json:"check_id"`
	CallID       uuid.UUID                    `json:"call_id"`
	Approved     bool                         `json:"approved"`
	Regulations  []RegulationResult           `json:"regulations"`
	Violations   []*compliance.ComplianceViolation `json:"violations,omitempty"`
	Warnings     []ComplianceWarning          `json:"warnings,omitempty"`
	ProcessTime  time.Duration                `json:"process_time"`
	Timestamp    time.Time                    `json:"timestamp"`
}

type CallPermissionResult struct {
	Permitted    bool                         `json:"permitted"`
	Reason       string                       `json:"reason,omitempty"`
	Conditions   []PermissionCondition        `json:"conditions,omitempty"`
	Violations   []*compliance.ComplianceViolation `json:"violations,omitempty"`
	ValidUntil   *time.Time                   `json:"valid_until,omitempty"`
}

type ComplianceReport struct {
	ReportID     uuid.UUID            `json:"report_id"`
	ReportType   ComplianceReportType `json:"report_type"`
	GeneratedAt  time.Time            `json:"generated_at"`
	Period       ReportPeriod         `json:"period"`
	Summary      ComplianceSummary    `json:"summary"`
	Details      interface{}          `json:"details,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type ViolationSummary struct {
	Total        int                  `json:"total"`
	ByType       map[string]int       `json:"by_type"`
	BySeverity   map[string]int       `json:"by_severity"`
	Resolved     int                  `json:"resolved"`
	Pending      int                  `json:"pending"`
	TrendData    []ViolationTrend     `json:"trend_data,omitempty"`
}

type ComplianceCertificate struct {
	CertificateID uuid.UUID            `json:"certificate_id"`
	IssuedAt      time.Time            `json:"issued_at"`
	ValidFrom     time.Time            `json:"valid_from"`
	ValidTo       time.Time            `json:"valid_to"`
	Regulations   []RegulationType     `json:"regulations"`
	Scope         string               `json:"scope"`
	Status        string               `json:"status"`
	Findings      []CertificateFinding `json:"findings"`
	Signature     string               `json:"signature"`
}

type ComplianceTrends struct {
	Period       ReportPeriod         `json:"period"`
	Metrics      map[string][]TrendDataPoint `json:"metrics"`
	Insights     []TrendInsight       `json:"insights"`
	Predictions  []TrendPrediction    `json:"predictions,omitempty"`
}

type ComplianceMetrics struct {
	TotalChecks      int64               `json:"total_checks"`
	ApprovedChecks   int64               `json:"approved_checks"`
	ViolationCount   int64               `json:"violation_count"`
	ComplianceRate   float64             `json:"compliance_rate"`
	ByRegulation     map[string]int64    `json:"by_regulation"`
	TrendData        []MetricDataPoint   `json:"trend_data"`
}

// Supporting types

type CallType string
const (
	CallTypeMarketing    CallType = "marketing"
	CallTypeSales        CallType = "sales"
	CallTypeService      CallType = "service"
	CallTypeInformational CallType = "informational"
	CallTypeEmergency    CallType = "emergency"
)

type RegulationType string
const (
	RegulationTCPA   RegulationType = "tcpa"
	RegulationGDPR   RegulationType = "gdpr"
	RegulationCCPA   RegulationType = "ccpa"
	RegulationDNC    RegulationType = "dnc"
	RegulationCustom RegulationType = "custom"
)

type DataSubjectRequestType string
const (
	DataSubjectAccess       DataSubjectRequestType = "access"
	DataSubjectDeletion     DataSubjectRequestType = "deletion"
	DataSubjectPortability  DataSubjectRequestType = "portability"
	DataSubjectRectification DataSubjectRequestType = "rectification"
	DataSubjectRestriction  DataSubjectRequestType = "restriction"
	DataSubjectObjection    DataSubjectRequestType = "objection"
)

type ComplianceReportType string
const (
	ReportTypeViolations ComplianceReportType = "violations"
	ReportTypeConsent    ComplianceReportType = "consent"
	ReportTypeAudit      ComplianceReportType = "audit"
	ReportTypeTrends     ComplianceReportType = "trends"
	ReportTypeCertification ComplianceReportType = "certification"
)

type NotificationChannel string
const (
	NotificationEmail   NotificationChannel = "email"
	NotificationSlack   NotificationChannel = "slack"
	NotificationWebhook NotificationChannel = "webhook"
	NotificationSMS     NotificationChannel = "sms"
)

// Supporting data structures

type ComplianceWarning struct {
	Type        string `json:"type"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
	Recommendation string `json:"recommendation,omitempty"`
}

type RegulationResult struct {
	Regulation RegulationType `json:"regulation"`
	Compliant  bool           `json:"compliant"`
	Violations []string       `json:"violations,omitempty"`
	Warnings   []string       `json:"warnings,omitempty"`
}

type PermissionCondition struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	ValidUntil  *time.Time `json:"valid_until,omitempty"`
}

type ReportPeriod struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type ComplianceSummary struct {
	TotalChecks      int64   `json:"total_checks"`
	ApprovedChecks   int64   `json:"approved_checks"`
	ViolationCount   int64   `json:"violation_count"`
	ComplianceRate   float64 `json:"compliance_rate"`
	CriticalViolations int64 `json:"critical_violations"`
}

type ViolationTrend struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
	Type  string    `json:"type"`
}

type CertificateFinding struct {
	Category    string `json:"category"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Evidence    []string `json:"evidence,omitempty"`
}

type TrendDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type TrendInsight struct {
	Type        string  `json:"type"`
	Message     string  `json:"message"`
	Confidence  float64 `json:"confidence"`
	Impact      string  `json:"impact"`
}

type TrendPrediction struct {
	Metric      string    `json:"metric"`
	PredictedValue float64 `json:"predicted_value"`
	Confidence  float64   `json:"confidence"`
	Timestamp   time.Time `json:"timestamp"`
}

type MetricDataPoint struct {
	Date   time.Time `json:"date"`
	Value  int64     `json:"value"`
	Type   string    `json:"type"`
}

type ComplianceAlert struct {
	AlertID     uuid.UUID            `json:"alert_id"`
	Type        string               `json:"type"`
	Severity    compliance.Severity  `json:"severity"`
	Message     string               `json:"message"`
	Timestamp   time.Time            `json:"timestamp"`
	CallID      *uuid.UUID           `json:"call_id,omitempty"`
	Regulation  RegulationType       `json:"regulation"`
	ActionRequired bool              `json:"action_required"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type CallDataRecord struct {
	CallID      uuid.UUID            `json:"call_id"`
	FromNumber  values.PhoneNumber   `json:"from_number"`
	ToNumber    values.PhoneNumber   `json:"to_number"`
	CallTime    time.Time            `json:"call_time"`
	Duration    time.Duration        `json:"duration"`
	CallType    CallType             `json:"call_type"`
	Purpose     string               `json:"purpose"`
}

type ConsentRecord struct {
	ConsentID   uuid.UUID          `json:"consent_id"`
	PhoneNumber values.PhoneNumber `json:"phone_number"`
	ConsentType string             `json:"consent_type"`
	GrantedAt   time.Time          `json:"granted_at"`
	Source      string             `json:"source"`
	Status      string             `json:"status"`
}

type ComplianceRecord struct {
	CheckID     uuid.UUID            `json:"check_id"`
	CallID      uuid.UUID            `json:"call_id"`
	Regulation  RegulationType       `json:"regulation"`
	Result      string               `json:"result"`
	Timestamp   time.Time            `json:"timestamp"`
	Violations  []string             `json:"violations,omitempty"`
}

type ConsentStatus struct {
	HasConsent   bool       `json:"has_consent"`
	ConsentType  string     `json:"consent_type,omitempty"`
	GrantedAt    *time.Time `json:"granted_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Source       string     `json:"source,omitempty"`
}

// Audit event types for IMMUTABLE_AUDIT integration

type ComplianceAuditEvent struct {
	EventType   string               `json:"event_type"`
	CallID      uuid.UUID            `json:"call_id"`
	PhoneNumber values.PhoneNumber   `json:"phone_number"`
	Regulation  RegulationType       `json:"regulation"`
	Result      string               `json:"result"`
	Timestamp   time.Time            `json:"timestamp"`
	ActorID     uuid.UUID            `json:"actor_id"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type ViolationAuditEvent struct {
	EventType     string                        `json:"event_type"`
	ViolationID   uuid.UUID                     `json:"violation_id"`
	CallID        uuid.UUID                     `json:"call_id"`
	ViolationType compliance.ViolationType     `json:"violation_type"`
	Severity      compliance.Severity          `json:"severity"`
	Description   string                       `json:"description"`
	Timestamp     time.Time                    `json:"timestamp"`
	DetectedBy    string                       `json:"detected_by"`
	Metadata      map[string]interface{}       `json:"metadata"`
}

type DataSubjectAuditEvent struct {
	EventType     string                 `json:"event_type"`
	RequestID     uuid.UUID              `json:"request_id"`
	PhoneNumber   values.PhoneNumber     `json:"phone_number"`
	RequestType   DataSubjectRequestType `json:"request_type"`
	Status        string                 `json:"status"`
	Timestamp     time.Time              `json:"timestamp"`
	ProcessedBy   uuid.UUID              `json:"processed_by"`
	Metadata      map[string]interface{} `json:"metadata"`
}