package audit

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/google/uuid"
)

// TCPA Types

// TCPAValidationRequest represents a TCPA compliance validation request
type TCPAValidationRequest struct {
	PhoneNumber  string    `json:"phone_number"`
	CallerID     string    `json:"caller_id,omitempty"`
	CallTime     time.Time `json:"call_time,omitempty"`
	Timezone     string    `json:"timezone,omitempty"`
	CallType     string    `json:"call_type,omitempty"` // marketing, transactional, informational
	BusinessType string    `json:"business_type,omitempty"`
	ActorID      string    `json:"actor_id"`
}

// TCPAValidationResult represents the result of TCPA validation
type TCPAValidationResult struct {
	RequestID     string                  `json:"request_id"`
	PhoneNumber   string                  `json:"phone_number"`
	ValidatedAt   time.Time               `json:"validated_at"`
	IsCompliant   bool                    `json:"is_compliant"`
	ConsentStatus string                  `json:"consent_status,omitempty"`
	ConsentExpiry *time.Time              `json:"consent_expiry,omitempty"`
	IsDNC         bool                    `json:"is_dnc"`
	Violations    []ComplianceViolation   `json:"violations,omitempty"`
	Requirements  []ComplianceRequirement `json:"requirements"`
	RiskScore     float64                 `json:"risk_score,omitempty"`
}

// TCPAConsent represents TCPA consent data
type TCPAConsent struct {
	PhoneNumber string    `json:"phone_number"`
	ConsentType string    `json:"consent_type"` // EXPRESS, IMPLIED, PRIOR_BUSINESS
	Source      string    `json:"source"`       // web_form, phone_call, sms, etc.
	IPAddress   string    `json:"ip_address,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	ActorID     string    `json:"actor_id"`
	ExpiryDays  int       `json:"expiry_days,omitempty"`
	ConsentText string    `json:"consent_text,omitempty"`
	Timestamp   time.Time `json:"timestamp,omitempty"`
}

// TCPARevocation represents TCPA consent revocation
type TCPARevocation struct {
	PhoneNumber string    `json:"phone_number"`
	Reason      string    `json:"reason,omitempty"`
	Source      string    `json:"source"` // opt_out_request, complaint, dnc_registration
	ActorID     string    `json:"actor_id"`
	Timestamp   time.Time `json:"timestamp,omitempty"`
}

// GDPR Types

// GDPRRequest represents a GDPR data subject request
type GDPRRequest struct {
	Type                 string                 `json:"type"` // ACCESS, ERASURE, RECTIFICATION, PORTABILITY, RESTRICTION
	DataSubjectID        string                 `json:"data_subject_id"`
	DataSubjectEmail     string                 `json:"data_subject_email,omitempty"`
	DataSubjectPhone     string                 `json:"data_subject_phone,omitempty"`
	VerificationMethod   string                 `json:"verification_method,omitempty"`
	IdentityVerified     bool                   `json:"identity_verified"`
	DataCategories       []string               `json:"data_categories,omitempty"`
	ExportFormat         string                 `json:"export_format,omitempty"` // JSON, XML, CSV
	RectificationData    map[string]interface{} `json:"rectification_data,omitempty"`
	RestrictedActivities []string               `json:"restricted_activities,omitempty"`
	Reason               string                 `json:"reason,omitempty"`
	RequestDate          time.Time              `json:"request_date,omitempty"`
	Deadline             time.Time              `json:"deadline,omitempty"`
}

// GDPRRequestResult represents the result of processing a GDPR request
type GDPRRequestResult struct {
	RequestID             string                 `json:"request_id"`
	RequestType           string                 `json:"request_type"`
	Status                string                 `json:"status"` // PROCESSING, COMPLETED, FAILED, REJECTED, PARTIALLY_COMPLETED
	ReceivedAt            time.Time              `json:"received_at"`
	CompletedAt           *time.Time             `json:"completed_at,omitempty"`
	Deadline              time.Time              `json:"deadline"`
	DataSubject           string                 `json:"data_subject"`
	DataExport            *DataExportInfo        `json:"data_export,omitempty"`
	DataAffected          *DataAffectedInfo      `json:"data_affected,omitempty"`
	ProcessingRestriction *ProcessingRestriction `json:"processing_restriction,omitempty"`
	RejectionReason       string                 `json:"rejection_reason,omitempty"`
	Notes                 string                 `json:"notes,omitempty"`
	Error                 string                 `json:"error,omitempty"`
	ProcessingTime        time.Duration          `json:"processing_time,omitempty"`
}

// DataExportInfo represents data export information
type DataExportInfo struct {
	ExportID       string    `json:"export_id"`
	Format         string    `json:"format"`
	FileSize       int64     `json:"file_size"`
	RecordCount    int64     `json:"record_count"`
	DataCategories []string  `json:"data_categories,omitempty"`
	ExpiryDate     time.Time `json:"expiry_date"`
	DownloadURL    string    `json:"download_url,omitempty"`
	AccessKey      string    `json:"access_key,omitempty"`
}

// DataAffectedInfo represents information about affected data
type DataAffectedInfo struct {
	RecordsDeleted    int64    `json:"records_deleted"`
	RecordsAnonymized int64    `json:"records_anonymized"`
	RecordsModified   int64    `json:"records_modified"`
	RecordsArchived   int64    `json:"records_archived"`
	DataCategories    []string `json:"data_categories"`
	FieldsUpdated     []string `json:"fields_updated,omitempty"`
}

// ProcessingRestriction represents a GDPR processing restriction
type ProcessingRestriction struct {
	ID                     string     `json:"id"`
	DataSubjectID          string     `json:"data_subject_id"`
	RequestDate            time.Time  `json:"request_date"`
	EffectiveDate          time.Time  `json:"effective_date"`
	DataCategories         []string   `json:"data_categories"`
	ProcessingActivities   []string   `json:"processing_activities"`
	RestrictedActions      []string   `json:"restricted_actions"`
	RestrictionReason      string     `json:"restriction_reason"`
	LegalBasis             string     `json:"legal_basis"`
	Status                 string     `json:"status"` // active, lifted, expired
	LiftedDate             *time.Time `json:"lifted_date,omitempty"`
	ExpiryDate             *time.Time `json:"expiry_date,omitempty"`
	TechnicalMeasures      []string   `json:"technical_measures,omitempty"`
	OrganizationalMeasures []string   `json:"organizational_measures,omitempty"`
	VerificationMethod     string     `json:"verification_method,omitempty"`
}

// CCPA Types

// CCPARequest represents a CCPA consumer privacy request
type CCPARequest struct {
	Type        string    `json:"type"` // OPT_OUT, DELETE, KNOW
	ConsumerID  string    `json:"consumer_id"`
	Email       string    `json:"email,omitempty"`
	Phone       string    `json:"phone,omitempty"`
	Categories  []string  `json:"categories,omitempty"`
	RequestDate time.Time `json:"request_date,omitempty"`
	Verified    bool      `json:"verified"`
}

// CCPARequestResult represents the result of processing a CCPA request
type CCPARequestResult struct {
	RequestID       string     `json:"request_id"`
	RequestType     string     `json:"request_type"`
	Status          string     `json:"status"`
	ReceivedAt      time.Time  `json:"received_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Consumer        string     `json:"consumer"`
	OptOutApplied   bool       `json:"opt_out_applied,omitempty"`
	DataDeleted     bool       `json:"data_deleted,omitempty"`
	DataProvided    bool       `json:"data_provided,omitempty"`
	RejectionReason string     `json:"rejection_reason,omitempty"`
	Error           string     `json:"error,omitempty"`
}

// PrivacyPreference represents a consumer privacy preference
type PrivacyPreference struct {
	ID          string     `json:"id"`
	ConsumerID  string     `json:"consumer_id"`
	Type        string     `json:"type"` // CCPA_OPT_OUT, EMAIL_OPT_OUT, etc.
	Value       string     `json:"value"`
	EffectiveAt time.Time  `json:"effective_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Categories  []string   `json:"categories,omitempty"`
	Source      string     `json:"source,omitempty"`
}

// SOX Types

// SOXReportCriteria represents criteria for SOX compliance reporting
type SOXReportCriteria struct {
	Period      string    `json:"period"` // Q1, Q2, Q3, Q4, ANNUAL
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Scope       []string  `json:"scope,omitempty"`        // financial_reporting, internal_controls, etc.
	DetailLevel string    `json:"detail_level,omitempty"` // summary, detailed
}

// SOXComplianceReport represents a SOX compliance report
type SOXComplianceReport struct {
	ReportID         string                `json:"report_id"`
	GeneratedAt      time.Time             `json:"generated_at"`
	Period           string                `json:"period"`
	IsCompliant      bool                  `json:"is_compliant"`
	DataIntegrity    *DataIntegrityResult  `json:"data_integrity"`
	AccessControls   *AccessControlsResult `json:"access_controls"`
	AuditTrailStatus *AuditTrailResult     `json:"audit_trail_status"`
	Controls         []SOXControl          `json:"controls"`
	Findings         []SOXFinding          `json:"findings"`
	Recommendations  []string              `json:"recommendations,omitempty"`
}

// DataIntegrityResult represents data integrity assessment results
type DataIntegrityResult struct {
	OverallScore     float64 `json:"overall_score"`
	HashChainValid   bool    `json:"hash_chain_valid"`
	SequenceComplete bool    `json:"sequence_complete"`
	DataCorruption   bool    `json:"data_corruption"`
	IssuesFound      int     `json:"issues_found"`
	TotalRecords     int64   `json:"total_records"`
}

// AccessControlsResult represents access controls assessment results
type AccessControlsResult struct {
	OverallScore           float64 `json:"overall_score"`
	SegregationOfDuties    bool    `json:"segregation_of_duties"`
	AuthenticationControls bool    `json:"authentication_controls"`
	AuthorizationControls  bool    `json:"authorization_controls"`
	AccessReviewCompleted  bool    `json:"access_review_completed"`
	ViolationsFound        int     `json:"violations_found"`
}

// AuditTrailResult represents audit trail assessment results
type AuditTrailResult struct {
	OverallScore     float64 `json:"overall_score"`
	TrailComplete    bool    `json:"trail_complete"`
	TrailAccurate    bool    `json:"trail_accurate"`
	TrailTamperProof bool    `json:"trail_tamper_proof"`
	GapsFound        int     `json:"gaps_found"`
	TotalEvents      int64   `json:"total_events"`
}

// SOXControl represents a SOX internal control
type SOXControl struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Type          string    `json:"type"`   // PREVENTIVE, DETECTIVE, CORRECTIVE
	Status        string    `json:"status"` // PASSED, FAILED, NOT_TESTED
	TestedAt      time.Time `json:"tested_at"`
	TestedBy      string    `json:"tested_by"`
	FailureReason string    `json:"failure_reason,omitempty"`
	Evidence      []string  `json:"evidence,omitempty"`
}

// SOXFinding represents a SOX compliance finding
type SOXFinding struct {
	ID          string    `json:"id"`
	ControlID   string    `json:"control_id,omitempty"`
	Type        string    `json:"type"`     // CONTROL_FAILURE, DATA_INTEGRITY, ACCESS_VIOLATION
	Severity    string    `json:"severity"` // LOW, MEDIUM, HIGH, CRITICAL
	Description string    `json:"description"`
	Impact      string    `json:"impact,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
	Status      string    `json:"status"` // OPEN, REMEDIATED, ACCEPTED
}

// Retention Types

// RetentionPolicy represents a data retention policy
type RetentionPolicy struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	DataTypes       []string          `json:"data_types"`
	RetentionPeriod RetentionPeriod   `json:"retention_period"`
	Actions         []RetentionAction `json:"actions"`
	LegalBasis      string            `json:"legal_basis,omitempty"`
	Exceptions      []string          `json:"exceptions,omitempty"`
	IsActive        bool              `json:"is_active"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// RetentionPeriod represents a retention time period
type RetentionPeriod struct {
	Duration int    `json:"duration"`
	Unit     string `json:"unit"` // days, months, years, hours
}

// RetentionAction represents an action to take on retained data
type RetentionAction struct {
	Type       string   `json:"type"` // DELETE, ARCHIVE, ANONYMIZE
	AfterDays  int      `json:"after_days"`
	Conditions []string `json:"conditions,omitempty"`
}

// RetentionResult represents the result of applying a retention policy
type RetentionResult struct {
	PolicyID          string     `json:"policy_id"`
	ExecutionID       string     `json:"execution_id"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	Status            string     `json:"status"` // RUNNING, COMPLETED, FAILED, COMPLETED_WITH_ERRORS
	RecordsEvaluated  int64      `json:"records_evaluated"`
	RecordsDeleted    int64      `json:"records_deleted"`
	RecordsArchived   int64      `json:"records_archived"`
	RecordsAnonymized int64      `json:"records_anonymized"`
	Errors            []string   `json:"errors,omitempty"`
	Error             string     `json:"error,omitempty"`
}

// LegalHold represents a legal hold on data
type LegalHold struct {
	ID                string     `json:"id"`
	CaseID            string     `json:"case_id,omitempty"`
	Description       string     `json:"description"`
	IssuedBy          string     `json:"issued_by"`
	IssuedDate        time.Time  `json:"issued_date"`
	ExpiryDate        *time.Time `json:"expiry_date,omitempty"`
	DataCategories    []string   `json:"data_categories"`
	DataSubjects      []string   `json:"data_subjects,omitempty"`
	Status            string     `json:"status"` // active, expired, lifted
	LiftedDate        *time.Time `json:"lifted_date,omitempty"`
	LiftedBy          string     `json:"lifted_by,omitempty"`
	LegalAuthority    string     `json:"legal_authority"`
	CourtOrder        bool       `json:"court_order"`
	RegulatoryRequest bool       `json:"regulatory_request"`
}

// Anonymization Types

// AnonymizationResult represents the result of data anonymization
type AnonymizationResult struct {
	DataSubjectID     string     `json:"data_subject_id"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	Success           bool       `json:"success"`
	RecordsAnonymized int64      `json:"records_anonymized"`
	RecordsDeleted    int64      `json:"records_deleted"`
	DataCategories    []string   `json:"data_categories"`
	Method            string     `json:"method,omitempty"`
	Errors            []string   `json:"errors,omitempty"`
}

// Common Types

// ComplianceViolation represents a compliance violation
type ComplianceViolation struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"` // LOW, MEDIUM, HIGH, CRITICAL
	Description string `json:"description"`
	Regulation  string `json:"regulation"` // TCPA, GDPR, CCPA, SOX
	Impact      string `json:"impact,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// ComplianceRequirement represents a compliance requirement
type ComplianceRequirement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // MANDATORY, RECOMMENDED, BEST_PRACTICE
	Regulation  string `json:"regulation"`
	Status      string `json:"status,omitempty"` // MET, NOT_MET, PARTIAL
}

// RetentionEligibleData represents data eligible for retention actions
type RetentionEligibleData struct {
	TotalRecords  int64                   `json:"total_records"`
	DataByType    map[string]int64        `json:"data_by_type"`
	EligibleItems []RetentionEligibleItem `json:"eligible_items"`
}

// RetentionEligibleItem represents a single item eligible for retention action
type RetentionEligibleItem struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
	EligibleAt  time.Time `json:"eligible_at"`
	DataSubject string    `json:"data_subject,omitempty"`
	DataSize    int64     `json:"data_size,omitempty"`
	LegalHolds  []string  `json:"legal_holds,omitempty"`
	Categories  []string  `json:"categories,omitempty"`
}

// ComplianceEngine interface for different compliance frameworks
type ComplianceEngine interface {
	ValidateCompliance(ctx context.Context, data interface{}) (*ComplianceValidationResult, error)
	GetRequirements() []ComplianceRequirement
	CheckViolations(ctx context.Context, events []audit.Event) ([]ComplianceViolation, error)
	GenerateReport(ctx context.Context, criteria interface{}) (interface{}, error)
}

// ComplianceValidationResult represents generic compliance validation result
type ComplianceValidationResult struct {
	Framework   string                `json:"framework"`
	IsCompliant bool                  `json:"is_compliant"`
	Score       float64               `json:"score"`
	Violations  []ComplianceViolation `json:"violations"`
	Timestamp   time.Time             `json:"timestamp"`
}

// ExportDataResult represents the result of data export
type ExportDataResult struct {
	ExportID       string    `json:"export_id"`
	Format         string    `json:"format"`
	FileSize       int64     `json:"file_size"`
	RecordCount    int64     `json:"record_count"`
	DataCategories []string  `json:"data_categories"`
	ExpiryDate     time.Time `json:"expiry_date"`
	FilePath       string    `json:"file_path,omitempty"`
	DownloadURL    string    `json:"download_url,omitempty"`
}
