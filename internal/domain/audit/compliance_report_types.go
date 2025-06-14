package audit

import (
	"time"

	"github.com/google/uuid"
)

// GDPRReportCriteria defines criteria for GDPR-specific reports
type GDPRReportCriteria struct {
	// Data subject identification
	DataSubjectID      string            `json:"data_subject_id"`
	DataSubjectEmail   string            `json:"data_subject_email,omitempty"`
	DataSubjectPhone   string            `json:"data_subject_phone,omitempty"`
	
	// Time range for the report
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	
	// GDPR scope
	RequestType        string            `json:"request_type"` // access, rectification, erasure, portability, restriction
	DataCategories     []string          `json:"data_categories,omitempty"` // personal, sensitive, biometric, etc.
	LegalBasisFilter   []string          `json:"legal_basis_filter,omitempty"`
	
	// Report configuration
	IncludeDataSources bool              `json:"include_data_sources"`
	IncludeProcessing  bool              `json:"include_processing"`
	IncludeThirdParties bool             `json:"include_third_parties"`
	IncludeRetention   bool              `json:"include_retention"`
	IncludeConsent     bool              `json:"include_consent"`
	
	// Detail level
	DetailLevel        string            `json:"detail_level"` // summary, detailed, comprehensive
	
	// Jurisdictional scope
	Jurisdictions      []string          `json:"jurisdictions,omitempty"`
	
	// Event filtering
	EventFilter        EventFilter       `json:"event_filter,omitempty"`
	
	// Output format
	Format             string            `json:"format,omitempty"` // json, xml, pdf, csv
	Language           string            `json:"language,omitempty"` // for multi-language support
}

// GDPRReport represents a GDPR compliance report
type GDPRReport struct {
	// Report metadata
	ID                 string                `json:"id"`
	GeneratedAt        time.Time             `json:"generated_at"`
	Criteria           GDPRReportCriteria    `json:"criteria"`
	
	// Data subject information
	DataSubject        *DataSubjectInfo      `json:"data_subject"`
	
	// GDPR rights analysis
	RightsAnalysis     *GDPRRightsAnalysis   `json:"rights_analysis"`
	
	// Data processing activities
	ProcessingActivities []*ProcessingActivity `json:"processing_activities,omitempty"`
	
	// Consent management
	ConsentHistory     *ConsentHistory       `json:"consent_history,omitempty"`
	
	// Data access and modification
	DataEvents         []*GDPRDataEvent      `json:"data_events,omitempty"`
	
	// Third-party sharing
	ThirdPartySharing  []*ThirdPartySharing  `json:"third_party_sharing,omitempty"`
	
	// Retention compliance
	RetentionCompliance *RetentionCompliance `json:"retention_compliance,omitempty"`
	
	// Breach notifications
	BreachNotifications []*BreachNotification `json:"breach_notifications,omitempty"`
	
	// Data export (for portability requests)
	DataExport         *DataExportInfo       `json:"data_export,omitempty"`
	
	// Compliance assessment
	ComplianceStatus   *GDPRComplianceStatus `json:"compliance_status"`
	
	// Recommendations
	Recommendations    []*GDPRRecommendation `json:"recommendations,omitempty"`
	
	// Report metadata
	ReportTime         time.Duration         `json:"report_time"`
	DataSources        []string              `json:"data_sources"`
	Version            string                `json:"version"`
}

// DataSubjectInfo contains information about the data subject
type DataSubjectInfo struct {
	ID                 string            `json:"id"`
	Email              string            `json:"email,omitempty"`
	Phone              string            `json:"phone,omitempty"`
	
	// Classification
	SubjectType        string            `json:"subject_type"` // customer, employee, prospect, etc.
	DataCategories     []string          `json:"data_categories"`
	
	// Lifecycle
	FirstSeen          time.Time         `json:"first_seen"`
	LastSeen           time.Time         `json:"last_seen"`
	IsActive           bool              `json:"is_active"`
	
	// Geographic information
	Jurisdiction       string            `json:"jurisdiction,omitempty"`
	Country            string            `json:"country,omitempty"`
	
	// Data volume
	TotalRecords       int64             `json:"total_records"`
	DataSize           int64             `json:"data_size"` // in bytes
}

// GDPRRightsAnalysis analyzes GDPR rights compliance
type GDPRRightsAnalysis struct {
	// Right to access
	AccessRights       *AccessRightsStatus   `json:"access_rights"`
	
	// Right to rectification
	RectificationRights *RectificationStatus `json:"rectification_rights"`
	
	// Right to erasure
	ErasureRights      *ErasureStatus        `json:"erasure_rights"`
	
	// Right to data portability
	PortabilityRights  *PortabilityStatus    `json:"portability_rights"`
	
	// Right to restriction
	RestrictionRights  *RestrictionStatus    `json:"restriction_rights"`
	
	// Right to object
	ObjectionRights    *ObjectionStatus      `json:"objection_rights"`
	
	// Rights related to automated decision-making
	AutomatedDecisionRights *AutomatedDecisionStatus `json:"automated_decision_rights"`
	
	// Overall rights compliance
	OverallCompliance  float64               `json:"overall_compliance"` // 0-100
	RightsViolations   []*RightsViolation    `json:"rights_violations,omitempty"`
}

// AccessRightsStatus represents compliance with the right to access
type AccessRightsStatus struct {
	IsCompliant        bool              `json:"is_compliant"`
	LastAccessRequest  *time.Time        `json:"last_access_request,omitempty"`
	RequestCount       int               `json:"request_count"`
	AverageResponseTime time.Duration    `json:"average_response_time"`
	
	// Data accessibility
	AccessibleData     []string          `json:"accessible_data"`
	InaccessibleData   []string          `json:"inaccessible_data,omitempty"`
	
	// Compliance issues
	Issues             []string          `json:"issues,omitempty"`
}

// RectificationStatus represents compliance with the right to rectification
type RectificationStatus struct {
	IsCompliant        bool              `json:"is_compliant"`
	LastRectification  *time.Time        `json:"last_rectification,omitempty"`
	RequestCount       int               `json:"request_count"`
	SuccessRate        float64           `json:"success_rate"`
	
	// Rectification capabilities
	RectifiableFields  []string          `json:"rectifiable_fields"`
	NonRectifiableFields []string        `json:"non_rectifiable_fields,omitempty"`
	
	// Compliance issues
	Issues             []string          `json:"issues,omitempty"`
}

// ErasureStatus represents compliance with the right to erasure (right to be forgotten)
type ErasureStatus struct {
	IsCompliant        bool              `json:"is_compliant"`
	LastErasure        *time.Time        `json:"last_erasure,omitempty"`
	RequestCount       int               `json:"request_count"`
	CompletionRate     float64           `json:"completion_rate"`
	
	// Erasure scope
	ErasableData       []string          `json:"erasable_data"`
	NonErasableData    []string          `json:"non_erasable_data,omitempty"`
	ErasureReasons     []string          `json:"erasure_reasons,omitempty"`
	
	// Legal holds
	LegalHolds         []*ComplianceLegalHold      `json:"legal_holds,omitempty"`
	
	// Compliance issues
	Issues             []string          `json:"issues,omitempty"`
}

// PortabilityStatus represents compliance with the right to data portability
type PortabilityStatus struct {
	IsCompliant        bool              `json:"is_compliant"`
	LastPortability    *time.Time        `json:"last_portability,omitempty"`
	RequestCount       int               `json:"request_count"`
	
	// Portability capabilities
	PortableFormats    []string          `json:"portable_formats"`
	PortableData       []string          `json:"portable_data"`
	NonPortableData    []string          `json:"non_portable_data,omitempty"`
	
	// Export information
	StandardFormats    []string          `json:"standard_formats"`
	CustomFormats      []string          `json:"custom_formats,omitempty"`
	
	// Compliance issues
	Issues             []string          `json:"issues,omitempty"`
}

// RestrictionStatus represents compliance with the right to restriction of processing
type RestrictionStatus struct {
	IsCompliant        bool              `json:"is_compliant"`
	ActiveRestrictions []*ProcessingRestriction `json:"active_restrictions,omitempty"`
	RequestCount       int               `json:"request_count"`
	
	// Restriction capabilities
	RestrictableProcessing []string      `json:"restrictable_processing"`
	
	// Compliance issues
	Issues             []string          `json:"issues,omitempty"`
}

// ObjectionStatus represents compliance with the right to object
type ObjectionStatus struct {
	IsCompliant        bool              `json:"is_compliant"`
	ObjectionCount     int               `json:"objection_count"`
	HonoredObjections  int               `json:"honored_objections"`
	
	// Objection types
	MarketingObjections    int           `json:"marketing_objections"`
	ProfilingObjections    int           `json:"profiling_objections"`
	LegitimateInterestObjections int     `json:"legitimate_interest_objections"`
	
	// Compliance issues
	Issues             []string          `json:"issues,omitempty"`
}

// AutomatedDecisionStatus represents compliance with rights related to automated decision-making
type AutomatedDecisionStatus struct {
	IsCompliant            bool          `json:"is_compliant"`
	HasAutomatedDecisions  bool          `json:"has_automated_decisions"`
	
	// Automated decision systems
	DecisionSystems        []*AutomatedDecisionSystem `json:"decision_systems,omitempty"`
	
	// Human review rights
	HumanReviewAvailable   bool          `json:"human_review_available"`
	ReviewRequestCount     int           `json:"review_request_count"`
	
	// Compliance issues
	Issues                 []string      `json:"issues,omitempty"`
}

// AutomatedDecisionSystem represents an automated decision-making system
type AutomatedDecisionSystem struct {
	SystemID               string        `json:"system_id"`
	SystemName             string        `json:"system_name"`
	DecisionType           string        `json:"decision_type"`
	
	// Transparency
	LogicExplanation       string        `json:"logic_explanation,omitempty"`
	SignificanceConsequences string      `json:"significance_consequences,omitempty"`
	
	// Subject involvement
	SubjectInvolved        bool          `json:"subject_involved"`
	LastDecision           *time.Time    `json:"last_decision,omitempty"`
	DecisionCount          int           `json:"decision_count"`
}

// RightsViolation represents a GDPR rights violation
type RightsViolation struct {
	ViolationType          string        `json:"violation_type"`
	RightViolated          string        `json:"right_violated"`
	Description            string        `json:"description"`
	Severity               string        `json:"severity"`
	DetectedAt             time.Time     `json:"detected_at"`
	Status                 string        `json:"status"`
	RemediationActions     []string      `json:"remediation_actions,omitempty"`
}

// ProcessingActivity represents a data processing activity
type ProcessingActivity struct {
	ID                     string        `json:"id"`
	Name                   string        `json:"name"`
	Description            string        `json:"description"`
	Purpose                string        `json:"purpose"`
	
	// Legal basis
	LegalBasis             string        `json:"legal_basis"`
	ConsentRequired        bool          `json:"consent_required"`
	ConsentObtained        bool          `json:"consent_obtained"`
	
	// Data categories
	DataCategories         []string      `json:"data_categories"`
	SpecialCategories      []string      `json:"special_categories,omitempty"`
	
	// Processing details
	ProcessingMethods      []string      `json:"processing_methods"`
	AutomatedProcessing    bool          `json:"automated_processing"`
	Profiling              bool          `json:"profiling"`
	
	// Temporal information
	StartDate              time.Time     `json:"start_date"`
	EndDate                *time.Time    `json:"end_date,omitempty"`
	LastProcessed          time.Time     `json:"last_processed"`
	ProcessingFrequency    string        `json:"processing_frequency"`
	
	// Data subjects
	DataSubjectCategories  []string      `json:"data_subject_categories"`
	ApproximateSubjects    int64         `json:"approximate_subjects,omitempty"`
	
	// Storage and retention
	StorageLocations       []string      `json:"storage_locations"`
	RetentionPeriod        string        `json:"retention_period"`
	RetentionJustification string        `json:"retention_justification,omitempty"`
	
	// Third parties
	ThirdParties           []string      `json:"third_parties,omitempty"`
	TransferMechanisms     []string      `json:"transfer_mechanisms,omitempty"`
	
	// Security measures
	SecurityMeasures       []string      `json:"security_measures"`
	
	// Compliance status
	IsCompliant            bool          `json:"is_compliant"`
	ComplianceIssues       []string      `json:"compliance_issues,omitempty"`
}

// ConsentHistory represents the history of consent for a data subject
type ConsentHistory struct {
	// Current consent status
	CurrentStatus          string        `json:"current_status"` // granted, withdrawn, expired, pending
	LastUpdated            time.Time     `json:"last_updated"`
	
	// Consent events
	ConsentEvents          []*ConsentEvent `json:"consent_events"`
	
	// Consent validity
	IsValid                bool          `json:"is_valid"`
	ExpiryDate             *time.Time    `json:"expiry_date,omitempty"`
	
	// Consent scope
	ConsentPurposes        []string      `json:"consent_purposes"`
	ConsentCategories      []string      `json:"consent_categories"`
	
	// Consent mechanism
	ConsentMethod          string        `json:"consent_method"` // explicit, implicit, opt_in, opt_out
	ConsentEvidence        []string      `json:"consent_evidence,omitempty"`
	
	// Withdrawal information
	WithdrawalMechanism    string        `json:"withdrawal_mechanism,omitempty"`
	IsWithdrawalEasy       bool          `json:"is_withdrawal_easy"`
	
	// Compliance assessment
	ConsentCompliance      float64       `json:"consent_compliance"` // 0-100
	ComplianceIssues       []string      `json:"compliance_issues,omitempty"`
}

// ConsentEvent represents a single consent-related event
type ConsentEvent struct {
	EventID                uuid.UUID     `json:"event_id"`
	Timestamp              time.Time     `json:"timestamp"`
	EventType              string        `json:"event_type"` // granted, withdrawn, updated, expired
	
	// Consent details
	Purposes               []string      `json:"purposes"`
	DataCategories         []string      `json:"data_categories"`
	ConsentMethod          string        `json:"consent_method"`
	
	// Context
	Channel                string        `json:"channel,omitempty"`
	UserAgent              string        `json:"user_agent,omitempty"`
	IPAddress              string        `json:"ip_address,omitempty"`
	
	// Evidence
	ConsentEvidence        interface{}   `json:"consent_evidence,omitempty"`
	ConsentText            string        `json:"consent_text,omitempty"`
	
	// Validity
	IsValid                bool          `json:"is_valid"`
	ValidityReason         string        `json:"validity_reason,omitempty"`
}

// GDPRDataEvent represents a data-related event for GDPR reporting
type GDPRDataEvent struct {
	EventID                uuid.UUID     `json:"event_id"`
	Timestamp              time.Time     `json:"timestamp"`
	EventType              EventType     `json:"event_type"`
	
	// Event details
	Action                 string        `json:"action"`
	Result                 string        `json:"result"`
	ActorID                string        `json:"actor_id"`
	
	// Data details
	DataCategories         []string      `json:"data_categories"`
	DataFields             []string      `json:"data_fields,omitempty"`
	DataSize               int64         `json:"data_size,omitempty"`
	
	// Legal basis
	LegalBasis             string        `json:"legal_basis"`
	Purpose                string        `json:"purpose"`
	
	// Location and context
	Location               string        `json:"location,omitempty"`
	SystemID               string        `json:"system_id,omitempty"`
	RequestID              string        `json:"request_id,omitempty"`
	
	// Compliance relevance
	IsCompliant            bool          `json:"is_compliant"`
	ComplianceFlags        []string      `json:"compliance_flags,omitempty"`
}

// ThirdPartySharing represents data sharing with third parties
type ThirdPartySharing struct {
	ThirdPartyID           string        `json:"third_party_id"`
	ThirdPartyName         string        `json:"third_party_name"`
	ThirdPartyType         string        `json:"third_party_type"` // processor, controller, joint_controller
	
	// Sharing details
	DataCategories         []string      `json:"data_categories"`
	SharingPurpose         string        `json:"sharing_purpose"`
	LegalBasis             string        `json:"legal_basis"`
	
	// Transfer details
	TransferMechanism      string        `json:"transfer_mechanism"` // adequacy_decision, sccs, bcrs, derogation
	Destination            string        `json:"destination"` // country/region
	IsThirdCountry         bool          `json:"is_third_country"`
	
	// Temporal information
	SharingStarted         time.Time     `json:"sharing_started"`
	SharingEnded           *time.Time    `json:"sharing_ended,omitempty"`
	LastShared             time.Time     `json:"last_shared"`
	SharingFrequency       string        `json:"sharing_frequency"`
	
	// Volume information
	RecordsShared          int64         `json:"records_shared"`
	DataVolumeShared       int64         `json:"data_volume_shared"`
	
	// Safeguards and agreements
	DataProcessingAgreement bool         `json:"data_processing_agreement"`
	SafeguardMeasures      []string      `json:"safeguard_measures"`
	
	// Compliance status
	IsCompliant            bool          `json:"is_compliant"`
	ComplianceIssues       []string      `json:"compliance_issues,omitempty"`
}

// RetentionCompliance represents data retention compliance analysis
type RetentionCompliance struct {
	// Overall retention status
	OverallCompliance      float64       `json:"overall_compliance"` // 0-100
	
	// Retention policies
	ApplicablePolicies     []*RetentionPolicy `json:"applicable_policies"`
	
	// Retention analysis
	DataCategories         []*CategoryRetention `json:"data_categories"`
	
	// Expiry information
	ExpiredData            []*ExpiredDataInfo    `json:"expired_data,omitempty"`
	SoonToExpire           []*SoonToExpireInfo   `json:"soon_to_expire,omitempty"`
	
	// Deletion tracking
	ScheduledDeletions     []*ScheduledDeletion  `json:"scheduled_deletions,omitempty"`
	CompletedDeletions     []*CompletedDeletion  `json:"completed_deletions,omitempty"`
	
	// Compliance issues
	RetentionViolations    []*RetentionViolation `json:"retention_violations,omitempty"`
	
	// Recommendations
	Recommendations        []string              `json:"recommendations,omitempty"`
}

// CategoryRetention represents retention information for a data category
type CategoryRetention struct {
	Category               string        `json:"category"`
	RetentionPeriod        string        `json:"retention_period"`
	LegalBasis             string        `json:"legal_basis"`
	
	// Current status
	RecordCount            int64         `json:"record_count"`
	OldestRecord           time.Time     `json:"oldest_record"`
	NewestRecord           time.Time     `json:"newest_record"`
	
	// Compliance
	IsCompliant            bool          `json:"is_compliant"`
	IssuesCount            int           `json:"issues_count"`
}

// ExpiredDataInfo represents information about expired data
type ExpiredDataInfo struct {
	Category               string        `json:"category"`
	RecordCount            int64         `json:"record_count"`
	ExpiredSince           time.Duration `json:"expired_since"`
	RetentionPeriod        string        `json:"retention_period"`
	DeletionStatus         string        `json:"deletion_status"`
}

// SoonToExpireInfo represents information about data that will expire soon
type SoonToExpireInfo struct {
	Category               string        `json:"category"`
	RecordCount            int64         `json:"record_count"`
	ExpiresIn              time.Duration `json:"expires_in"`
	RetentionPeriod        string        `json:"retention_period"`
	DeletionScheduled      bool          `json:"deletion_scheduled"`
}

// ScheduledDeletion represents a scheduled data deletion
type ScheduledDeletion struct {
	ID                     string        `json:"id"`
	Category               string        `json:"category"`
	ScheduledDate          time.Time     `json:"scheduled_date"`
	RecordCount            int64         `json:"record_count"`
	Status                 string        `json:"status"` // pending, in_progress, completed, failed
}

// CompletedDeletion represents a completed data deletion
type CompletedDeletion struct {
	ID                     string        `json:"id"`
	Category               string        `json:"category"`
	CompletedDate          time.Time     `json:"completed_date"`
	RecordsDeleted         int64         `json:"records_deleted"`
	DeletionMethod         string        `json:"deletion_method"`
	VerificationStatus     string        `json:"verification_status"`
}

// RetentionViolation represents a data retention violation
type RetentionViolation struct {
	ViolationType          string        `json:"violation_type"`
	Category               string        `json:"category"`
	Description            string        `json:"description"`
	Severity               string        `json:"severity"`
	RecordsAffected        int64         `json:"records_affected"`
	DetectedAt             time.Time     `json:"detected_at"`
	Status                 string        `json:"status"`
	RemediationPlan        string        `json:"remediation_plan,omitempty"`
}

// BreachNotification represents a data breach notification
type BreachNotification struct {
	ID                     string        `json:"id"`
	BreachID               string        `json:"breach_id"`
	NotificationDate       time.Time     `json:"notification_date"`
	NotificationType       string        `json:"notification_type"` // supervisory_authority, data_subjects, both
	
	// Breach details
	BreachDate             time.Time     `json:"breach_date"`
	BreachDescription      string        `json:"breach_description"`
	DataCategoriesAffected []string      `json:"data_categories_affected"`
	DataSubjectsAffected   int64         `json:"data_subjects_affected"`
	
	// Risk assessment
	RiskLevel              string        `json:"risk_level"`
	LikelyConsequences     string        `json:"likely_consequences"`
	
	// Notification details
	NotificationMethod     string        `json:"notification_method"`
	AuthorityNotified      bool          `json:"authority_notified"`
	SubjectsNotified       bool          `json:"subjects_notified"`
	
	// Follow-up
	FollowUpRequired       bool          `json:"follow_up_required"`
	FollowUpDate           *time.Time    `json:"follow_up_date,omitempty"`
	
	// Status
	Status                 string        `json:"status"` // reported, investigating, resolved
	Resolution             string        `json:"resolution,omitempty"`
}

// DataExportInfo represents data export information for portability requests
type DataExportInfo struct {
	ExportID               string        `json:"export_id"`
	ExportDate             time.Time     `json:"export_date"`
	RequestDate            time.Time     `json:"request_date"`
	
	// Export scope
	DataCategories         []string      `json:"data_categories"`
	DataSources            []string      `json:"data_sources"`
	
	// Export details
	Format                 string        `json:"format"`
	FileSize               int64         `json:"file_size"`
	RecordCount            int64         `json:"record_count"`
	
	// Delivery information
	DeliveryMethod         string        `json:"delivery_method"`
	ExpiryDate             time.Time     `json:"expiry_date"`
	
	// Status
	Status                 string        `json:"status"` // generating, ready, delivered, expired
	DownloadCount          int           `json:"download_count"`
}

// GDPRComplianceStatus represents overall GDPR compliance status
type GDPRComplianceStatus struct {
	OverallStatus          string        `json:"overall_status"` // compliant, non_compliant, partial
	ComplianceScore        float64       `json:"compliance_score"` // 0-100
	LastAssessment         time.Time     `json:"last_assessment"`
	
	// Compliance by area
	DataProtectionScore    float64       `json:"data_protection_score"`
	ConsentManagementScore float64       `json:"consent_management_score"`
	DataSubjectRightsScore float64       `json:"data_subject_rights_score"`
	RetentionComplianceScore float64     `json:"retention_compliance_score"`
	BreachManagementScore  float64       `json:"breach_management_score"`
	
	// Issues summary
	CriticalIssues         int           `json:"critical_issues"`
	HighPriorityIssues     int           `json:"high_priority_issues"`
	MediumPriorityIssues   int           `json:"medium_priority_issues"`
	LowPriorityIssues      int           `json:"low_priority_issues"`
	
	// Recent changes
	RecentImprovements     []string      `json:"recent_improvements,omitempty"`
	NewIssues              []string      `json:"new_issues,omitempty"`
	
	// Next actions
	ImmediateActions       []string      `json:"immediate_actions,omitempty"`
	UpcomingDeadlines      []ComplianceDeadline `json:"upcoming_deadlines,omitempty"`
}

// ComplianceDeadline represents an upcoming compliance deadline
type ComplianceDeadline struct {
	Description            string        `json:"description"`
	DueDate                time.Time     `json:"due_date"`
	Priority               string        `json:"priority"`
	RequiredActions        []string      `json:"required_actions"`
}

// GDPRRecommendation represents a GDPR compliance recommendation
type GDPRRecommendation struct {
	ID                     string        `json:"id"`
	Type                   string        `json:"type"` // immediate, short_term, long_term, strategic
	Priority               string        `json:"priority"`
	
	// Recommendation details
	Title                  string        `json:"title"`
	Description            string        `json:"description"`
	Rationale              string        `json:"rationale"`
	
	// Impact assessment
	ComplianceImpact       string        `json:"compliance_impact"`
	RiskReduction          float64       `json:"risk_reduction,omitempty"`
	
	// Implementation
	EstimatedEffort        string        `json:"estimated_effort,omitempty"`
	RequiredResources      []string      `json:"required_resources,omitempty"`
	Timeline               string        `json:"timeline,omitempty"`
	
	// Tracking
	Status                 string        `json:"status"` // pending, in_progress, completed, rejected
	AssignedTo             string        `json:"assigned_to,omitempty"`
	DueDate                *time.Time    `json:"due_date,omitempty"`
	
	// Related information
	RelatedArticles        []string      `json:"related_articles,omitempty"` // GDPR articles
	RelatedViolations      []string      `json:"related_violations,omitempty"`
}

// ComplianceLegalHold represents a legal hold preventing data deletion
type ComplianceLegalHold struct {
	ID                     string        `json:"id"`
	CaseID                 string        `json:"case_id,omitempty"`
	Description            string        `json:"description"`
	
	// Hold details
	IssuedBy               string        `json:"issued_by"`
	IssuedDate             time.Time     `json:"issued_date"`
	ExpiryDate             *time.Time    `json:"expiry_date,omitempty"`
	
	// Scope
	DataCategories         []string      `json:"data_categories"`
	DataSubjects           []string      `json:"data_subjects,omitempty"`
	
	// Status
	Status                 string        `json:"status"` // active, expired, lifted
	LiftedDate             *time.Time    `json:"lifted_date,omitempty"`
	LiftedBy               string        `json:"lifted_by,omitempty"`
	
	// Legal basis
	LegalAuthority         string        `json:"legal_authority"`
	CourtOrder             bool          `json:"court_order"`
	RegulatoryRequest      bool          `json:"regulatory_request"`
}

// ProcessingRestriction represents a restriction on data processing
type ProcessingRestriction struct {
	ID                     string        `json:"id"`
	DataSubjectID          string        `json:"data_subject_id"`
	RequestDate            time.Time     `json:"request_date"`
	EffectiveDate          time.Time     `json:"effective_date"`
	
	// Restriction scope
	DataCategories         []string      `json:"data_categories"`
	ProcessingActivities   []string      `json:"processing_activities"`
	RestrictedActions      []string      `json:"restricted_actions"`
	
	// Restriction reason
	RestrictionReason      string        `json:"restriction_reason"`
	LegalBasis             string        `json:"legal_basis"`
	
	// Exceptions
	AllowedProcessing      []string      `json:"allowed_processing,omitempty"`
	EmergencyOverride      bool          `json:"emergency_override"`
	
	// Status
	Status                 string        `json:"status"` // active, lifted, expired
	LiftedDate             *time.Time    `json:"lifted_date,omitempty"`
	ExpiryDate             *time.Time    `json:"expiry_date,omitempty"`
	
	// Implementation
	TechnicalMeasures      []string      `json:"technical_measures"`
	OrganizationalMeasures []string      `json:"organizational_measures"`
	VerificationMethod     string        `json:"verification_method"`
}