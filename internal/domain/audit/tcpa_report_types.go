package audit

import (
	"time"

	"github.com/google/uuid"
)

// TCPAReportCriteria defines criteria for TCPA-specific reports
type TCPAReportCriteria struct {
	// Phone number identification
	PhoneNumber        string            `json:"phone_number"`
	PhoneNumbers       []string          `json:"phone_numbers,omitempty"` // For bulk reports
	
	// Time range for the report
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	
	// TCPA scope
	CallTypes          []string          `json:"call_types,omitempty"` // marketing, informational, transactional
	ConsentTypes       []string          `json:"consent_types,omitempty"` // express_written, express_oral, implied
	
	// Report configuration
	IncludeCallHistory bool              `json:"include_call_history"`
	IncludeConsentHistory bool           `json:"include_consent_history"`
	IncludeDNCStatus   bool              `json:"include_dnc_status"`
	IncludeViolations  bool              `json:"include_violations"`
	IncludeWirelessInfo bool             `json:"include_wireless_info"`
	
	// Detail level
	DetailLevel        string            `json:"detail_level"` // summary, detailed, comprehensive
	
	// Jurisdictional scope
	States             []string          `json:"states,omitempty"`
	FederalOnly        bool              `json:"federal_only"`
	
	// Event filtering
	EventFilter        EventFilter       `json:"event_filter,omitempty"`
	
	// Output format
	Format             string            `json:"format,omitempty"` // json, pdf, csv, excel
}

// TCPAReport represents a TCPA compliance report
type TCPAReport struct {
	// Report metadata
	ID                 string                `json:"id"`
	GeneratedAt        time.Time             `json:"generated_at"`
	Criteria           TCPAReportCriteria    `json:"criteria"`
	
	// Phone number information
	PhoneNumberInfo    *PhoneNumberInfo      `json:"phone_number_info"`
	
	// TCPA compliance analysis
	ComplianceAnalysis *TCPAComplianceAnalysis `json:"compliance_analysis"`
	
	// Call history
	CallHistory        []*TCPACallRecord     `json:"call_history,omitempty"`
	
	// Consent management
	ConsentHistory     *TCPAConsentHistory   `json:"consent_history,omitempty"`
	
	// Do Not Call status
	DNCStatus          *DNCStatus            `json:"dnc_status,omitempty"`
	
	// Violations and compliance issues
	Violations         []*TCPAViolation      `json:"violations,omitempty"`
	
	// Wireless carrier information
	WirelessInfo       *WirelessCarrierInfo  `json:"wireless_info,omitempty"`
	
	// Time-based analysis
	CallingPatterns    *CallingPatterns      `json:"calling_patterns,omitempty"`
	
	// Compliance scoring
	ComplianceScore    *TCPAComplianceScore  `json:"compliance_score"`
	
	// Recommendations
	Recommendations    []*TCPARecommendation `json:"recommendations,omitempty"`
	
	// Risk assessment
	RiskAssessment     *TCPARiskAssessment   `json:"risk_assessment,omitempty"`
	
	// Report metadata
	ReportTime         time.Duration         `json:"report_time"`
	DataSources        []string              `json:"data_sources"`
	Version            string                `json:"version"`
}

// PhoneNumberInfo contains information about the phone number
type PhoneNumberInfo struct {
	PhoneNumber        string            `json:"phone_number"`
	FormattedNumber    string            `json:"formatted_number"`
	
	// Number classification
	NumberType         string            `json:"number_type"` // mobile, landline, voip, toll_free, unknown
	IsWireless         bool              `json:"is_wireless"`
	IsPortedNumber     bool              `json:"is_ported_number"`
	
	// Geographic information
	OriginalCarrier    string            `json:"original_carrier,omitempty"`
	CurrentCarrier     string            `json:"current_carrier,omitempty"`
	State              string            `json:"state,omitempty"`
	City               string            `json:"city,omitempty"`
	TimeZone           string            `json:"time_zone,omitempty"`
	
	// Regulatory status
	IsRegistered       bool              `json:"is_registered"` // In our system
	RegistrationDate   *time.Time        `json:"registration_date,omitempty"`
	
	// Call history summary
	TotalCalls         int64             `json:"total_calls"`
	FirstCall          *time.Time        `json:"first_call,omitempty"`
	LastCall           *time.Time        `json:"last_call,omitempty"`
	
	// Data quality
	ValidationStatus   string            `json:"validation_status"` // valid, invalid, unknown
	LastValidated      *time.Time        `json:"last_validated,omitempty"`
}

// TCPAComplianceAnalysis analyzes TCPA compliance for the phone number
type TCPAComplianceAnalysis struct {
	// Overall compliance
	OverallCompliance  float64           `json:"overall_compliance"` // 0-100
	ComplianceStatus   string            `json:"compliance_status"` // compliant, non_compliant, partial
	
	// Consent compliance
	ConsentCompliance  *ConsentComplianceStatus `json:"consent_compliance"`
	
	// Calling time compliance
	TimeCompliance     *CallingTimeCompliance   `json:"time_compliance"`
	
	// Frequency compliance
	FrequencyCompliance *FrequencyCompliance    `json:"frequency_compliance"`
	
	// Content compliance
	ContentCompliance  *ContentCompliance       `json:"content_compliance"`
	
	// Do Not Call compliance
	DNCCompliance      *DNCComplianceStatus     `json:"dnc_compliance"`
	
	// Record keeping compliance
	RecordKeeping      *RecordKeepingCompliance `json:"record_keeping"`
	
	// Caller ID compliance
	CallerIDCompliance *CallerIDCompliance      `json:"caller_id_compliance"`
	
	// Automated dialing compliance
	AutoDialingCompliance *AutoDialingCompliance `json:"auto_dialing_compliance"`
	
	// Violations summary
	ViolationCount     int               `json:"violation_count"`
	CriticalViolations int               `json:"critical_violations"`
	RecentViolations   int               `json:"recent_violations"`
	
	// Compliance trends
	ComplianceTrend    string            `json:"compliance_trend"` // improving, declining, stable
	LastAssessment     time.Time         `json:"last_assessment"`
}

// ConsentComplianceStatus represents consent compliance status
type ConsentComplianceStatus struct {
	IsCompliant        bool              `json:"is_compliant"`
	ConsentStatus      string            `json:"consent_status"` // granted, revoked, expired, none
	ConsentType        string            `json:"consent_type"` // express_written, express_oral, implied
	
	// Consent details
	ConsentDate        *time.Time        `json:"consent_date,omitempty"`
	ConsentExpiry      *time.Time        `json:"consent_expiry,omitempty"`
	ConsentMethod      string            `json:"consent_method,omitempty"`
	ConsentScope       []string          `json:"consent_scope,omitempty"`
	
	// Revocation details
	RevocationDate     *time.Time        `json:"revocation_date,omitempty"`
	RevocationMethod   string            `json:"revocation_method,omitempty"`
	
	// Compliance issues
	Issues             []string          `json:"issues,omitempty"`
	RequiredActions    []string          `json:"required_actions,omitempty"`
}

// CallingTimeCompliance represents compliance with calling time restrictions
type CallingTimeCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// Time zone compliance
	CorrectTimeZone    bool              `json:"correct_time_zone"`
	TimeZoneUsed       string            `json:"time_zone_used"`
	
	// Calling window compliance
	CallsInWindow      int64             `json:"calls_in_window"`
	CallsOutsideWindow int64             `json:"calls_outside_window"`
	ComplianceRate     float64           `json:"compliance_rate"`
	
	// Violation details
	EarlyCallViolations int64            `json:"early_call_violations"`
	LateCallViolations  int64            `json:"late_call_violations"`
	WeekendViolations   int64            `json:"weekend_violations"`
	HolidayViolations   int64            `json:"holiday_violations"`
	
	// Recent violations
	RecentViolations   []*TimeViolation  `json:"recent_violations,omitempty"`
	
	// Allowed calling window
	AllowedStart       string            `json:"allowed_start"` // e.g., "08:00"
	AllowedEnd         string            `json:"allowed_end"`   // e.g., "21:00"
	LocalTimeZone      string            `json:"local_time_zone"`
}

// TimeViolation represents a calling time violation
type TimeViolation struct {
	CallID             uuid.UUID         `json:"call_id"`
	CallTime           time.Time         `json:"call_time"`
	LocalTime          string            `json:"local_time"`
	ViolationType      string            `json:"violation_type"` // early, late, weekend, holiday
	TimeZoneUsed       string            `json:"time_zone_used"`
	Severity           string            `json:"severity"`
}

// FrequencyCompliance represents compliance with call frequency limits
type FrequencyCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// Daily frequency
	DailyLimit         int               `json:"daily_limit,omitempty"`
	DailyActual        map[string]int    `json:"daily_actual"` // date -> count
	DailyViolations    int               `json:"daily_violations"`
	
	// Weekly frequency
	WeeklyLimit        int               `json:"weekly_limit,omitempty"`
	WeeklyActual       map[string]int    `json:"weekly_actual"` // week -> count
	WeeklyViolations   int               `json:"weekly_violations"`
	
	// Monthly frequency
	MonthlyLimit       int               `json:"monthly_limit,omitempty"`
	MonthlyActual      map[string]int    `json:"monthly_actual"` // month -> count
	MonthlyViolations  int               `json:"monthly_violations"`
	
	// Campaign frequency (for marketing calls)
	CampaignLimit      int               `json:"campaign_limit,omitempty"`
	CampaignActual     int               `json:"campaign_actual"`
	CampaignViolations int               `json:"campaign_violations"`
	
	// Frequency violations
	RecentViolations   []*FrequencyViolation `json:"recent_violations,omitempty"`
}

// FrequencyViolation represents a call frequency violation
type FrequencyViolation struct {
	ViolationType      string            `json:"violation_type"` // daily, weekly, monthly, campaign
	Period             string            `json:"period"` // e.g., "2024-01-15" for daily
	Limit              int               `json:"limit"`
	ActualCount        int               `json:"actual_count"`
	ExcessCalls        int               `json:"excess_calls"`
	FirstViolation     time.Time         `json:"first_violation"`
	LastViolation      time.Time         `json:"last_violation"`
}

// ContentCompliance represents compliance with call content requirements
type ContentCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// Disclosure compliance
	HasProperDisclosure bool             `json:"has_proper_disclosure"`
	DiscloseCompanyName bool             `json:"disclose_company_name"`
	DiscloseCallPurpose bool             `json:"disclose_call_purpose"`
	DiscloseOptOut      bool             `json:"disclose_opt_out"`
	
	// Message content compliance
	MessageCompliance  *MessageContentCompliance `json:"message_compliance,omitempty"`
	
	// Opt-out compliance
	OptOutCompliance   *OptOutCompliance         `json:"opt_out_compliance"`
	
	// Recording compliance
	RecordingCompliance *RecordingCompliance     `json:"recording_compliance,omitempty"`
	
	// Content violations
	ContentViolations  []*ContentViolation       `json:"content_violations,omitempty"`
}

// MessageContentCompliance represents compliance with message content rules
type MessageContentCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// Required elements
	HasIdentification  bool              `json:"has_identification"`
	HasCallPurpose     bool              `json:"has_call_purpose"`
	HasOptOutInstructions bool           `json:"has_opt_out_instructions"`
	HasContactInfo     bool              `json:"has_contact_info"`
	
	// Prohibited content
	HasMisleadingContent bool            `json:"has_misleading_content"`
	HasDeceptiveContent  bool            `json:"has_deceptive_content"`
	HasIllegalContent    bool            `json:"has_illegal_content"`
	
	// Content analysis results
	ContentAnalysisScore float64          `json:"content_analysis_score"` // 0-100
	ComplianceFlags     []string          `json:"compliance_flags,omitempty"`
}

// OptOutCompliance represents compliance with opt-out requirements
type OptOutCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// Opt-out mechanism availability
	HasOptOutMechanism bool              `json:"has_opt_out_mechanism"`
	OptOutMethods      []string          `json:"opt_out_methods"` // voice, keypress, text, email
	
	// Opt-out processing
	OptOutRequests     int               `json:"opt_out_requests"`
	ProcessedOptOuts   int               `json:"processed_opt_outs"`
	ProcessingRate     float64           `json:"processing_rate"`
	AverageProcessingTime time.Duration  `json:"average_processing_time"`
	
	// Opt-out honoring
	HonoredOptOuts     int               `json:"honored_opt_outs"`
	ViolatedOptOuts    int               `json:"violated_opt_outs"`
	OptOutViolations   []*OptOutViolation `json:"opt_out_violations,omitempty"`
	
	// Timing compliance
	ImmediateHonoring  bool              `json:"immediate_honoring"`
	MaxProcessingTime  time.Duration     `json:"max_processing_time"`
	DelayedProcessing  int               `json:"delayed_processing"`
}

// OptOutViolation represents an opt-out violation
type OptOutViolation struct {
	ViolationType      string            `json:"violation_type"` // ignored_request, delayed_processing, continued_calling
	OptOutDate         time.Time         `json:"opt_out_date"`
	ViolationDate      time.Time         `json:"violation_date"`
	CallCount          int               `json:"call_count"`
	Description        string            `json:"description"`
	Severity           string            `json:"severity"`
}

// RecordingCompliance represents compliance with call recording requirements
type RecordingCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	RecordingRequired  bool              `json:"recording_required"`
	
	// Recording disclosure
	DisclosureProvided bool              `json:"disclosure_provided"`
	DisclosureMethod   string            `json:"disclosure_method,omitempty"`
	
	// Consent for recording
	ConsentObtained    bool              `json:"consent_obtained"`
	ConsentMethod      string            `json:"consent_method,omitempty"`
	
	// Recording retention
	RetentionCompliant bool              `json:"retention_compliant"`
	RetentionPeriod    string            `json:"retention_period,omitempty"`
	
	// Recording violations
	RecordingViolations []*RecordingViolation `json:"recording_violations,omitempty"`
}

// RecordingViolation represents a recording compliance violation
type RecordingViolation struct {
	ViolationType      string            `json:"violation_type"` // no_disclosure, no_consent, retention_violation
	CallID             uuid.UUID         `json:"call_id"`
	ViolationDate      time.Time         `json:"violation_date"`
	Description        string            `json:"description"`
	Severity           string            `json:"severity"`
}

// ContentViolation represents a content compliance violation
type ContentViolation struct {
	ViolationType      string            `json:"violation_type"`
	CallID             uuid.UUID         `json:"call_id"`
	ViolationDate      time.Time         `json:"violation_date"`
	Description        string            `json:"description"`
	Content            string            `json:"content,omitempty"`
	Severity           string            `json:"severity"`
	RegulatoryReference string           `json:"regulatory_reference,omitempty"`
}

// DNCComplianceStatus represents Do Not Call compliance status
type DNCComplianceStatus struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// DNC registry status
	FederalDNCStatus   string            `json:"federal_dnc_status"` // registered, not_registered, unknown
	StateDNCStatus     map[string]string `json:"state_dnc_status,omitempty"`
	InternalDNCStatus  string            `json:"internal_dnc_status"`
	
	// DNC check history
	LastDNCCheck       *time.Time        `json:"last_dnc_check,omitempty"`
	DNCCheckFrequency  string            `json:"dnc_check_frequency"`
	DNCChecksPassed    int64             `json:"dnc_checks_passed"`
	DNCChecksFailed    int64             `json:"dnc_checks_failed"`
	
	// DNC violations
	DNCViolations      []*DNCViolation   `json:"dnc_violations,omitempty"`
	ViolationCount     int               `json:"violation_count"`
	
	// Exemptions
	ApplicableExemptions []string         `json:"applicable_exemptions,omitempty"`
	ExemptionJustification string         `json:"exemption_justification,omitempty"`
}

// DNCViolation represents a Do Not Call violation
type DNCViolation struct {
	ViolationType      string            `json:"violation_type"` // federal_dnc, state_dnc, internal_dnc
	CallID             uuid.UUID         `json:"call_id"`
	ViolationDate      time.Time         `json:"violation_date"`
	RegistryType       string            `json:"registry_type"`
	RegistrationDate   *time.Time        `json:"registration_date,omitempty"`
	CallPurpose        string            `json:"call_purpose"`
	ExemptionClaimed   string            `json:"exemption_claimed,omitempty"`
	ExemptionValid     bool              `json:"exemption_valid"`
	Severity           string            `json:"severity"`
	PotentialFine      float64           `json:"potential_fine,omitempty"`
}

// RecordKeepingCompliance represents compliance with record keeping requirements
type RecordKeepingCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// Required records
	RequiredRecords    []string          `json:"required_records"`
	AvailableRecords   []string          `json:"available_records"`
	MissingRecords     []string          `json:"missing_records,omitempty"`
	
	// Record retention
	RetentionPeriod    string            `json:"retention_period"`
	RetentionCompliant bool              `json:"retention_compliant"`
	
	// Record completeness
	CompletenessScore  float64           `json:"completeness_score"` // 0-100
	IncompleteRecords  int               `json:"incomplete_records"`
	
	// Record quality
	DataQualityScore   float64           `json:"data_quality_score"` // 0-100
	QualityIssues      []string          `json:"quality_issues,omitempty"`
	
	// Access and retrieval
	RecordsAccessible  bool              `json:"records_accessible"`
	AverageRetrievalTime time.Duration   `json:"average_retrieval_time"`
	
	// Compliance violations
	RecordKeepingViolations []*RecordKeepingViolation `json:"record_keeping_violations,omitempty"`
}

// RecordKeepingViolation represents a record keeping violation
type RecordKeepingViolation struct {
	ViolationType      string            `json:"violation_type"` // missing_record, incomplete_record, retention_violation
	RecordType         string            `json:"record_type"`
	Description        string            `json:"description"`
	DetectedDate       time.Time         `json:"detected_date"`
	Severity           string            `json:"severity"`
	Impact             string            `json:"impact"`
}

// CallerIDCompliance represents compliance with Caller ID requirements
type CallerIDCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// Caller ID transmission
	CallerIDTransmitted bool             `json:"caller_id_transmitted"`
	CallerIDAccurate   bool              `json:"caller_id_accurate"`
	CallerIDComplete   bool              `json:"caller_id_complete"`
	
	// Caller ID information
	DisplayedNumber    string            `json:"displayed_number,omitempty"`
	DisplayedName      string            `json:"displayed_name,omitempty"`
	ActualNumber       string            `json:"actual_number,omitempty"`
	ActualName         string            `json:"actual_name,omitempty"`
	
	// Spoofing detection
	PossibleSpoofing   bool              `json:"possible_spoofing"`
	SpoofingEvidence   []string          `json:"spoofing_evidence,omitempty"`
	
	// Compliance violations
	CallerIDViolations []*CallerIDViolation `json:"caller_id_violations,omitempty"`
}

// CallerIDViolation represents a Caller ID violation
type CallerIDViolation struct {
	ViolationType      string            `json:"violation_type"` // blocked, spoofed, inaccurate, incomplete
	CallID             uuid.UUID         `json:"call_id"`
	ViolationDate      time.Time         `json:"violation_date"`
	ExpectedCallerID   string            `json:"expected_caller_id"`
	ActualCallerID     string            `json:"actual_caller_id"`
	Description        string            `json:"description"`
	Severity           string            `json:"severity"`
}

// AutoDialingCompliance represents compliance with automated dialing restrictions
type AutoDialingCompliance struct {
	IsCompliant        bool              `json:"is_compliant"`
	
	// Auto-dialing detection
	UsesAutoDialer     bool              `json:"uses_auto_dialer"`
	AutoDialerType     string            `json:"auto_dialer_type,omitempty"`
	
	// Consent requirements for auto-dialing
	ConsentRequired    bool              `json:"consent_required"`
	ConsentObtained    bool              `json:"consent_obtained"`
	ConsentType        string            `json:"consent_type,omitempty"`
	
	// Artificial voice detection
	UsesArtificialVoice bool             `json:"uses_artificial_voice"`
	VoiceType          string            `json:"voice_type,omitempty"`
	
	// Prerecorded message compliance
	UsesPrerecorded    bool              `json:"uses_prerecorded"`
	PrerecordedConsent bool              `json:"prerecorded_consent"`
	
	// Technology compliance
	TechnologyCompliance *TechnologyCompliance `json:"technology_compliance,omitempty"`
	
	// Violations
	AutoDialingViolations []*AutoDialingViolation `json:"auto_dialing_violations,omitempty"`
}

// TechnologyCompliance represents compliance with technology-specific rules
type TechnologyCompliance struct {
	// Predictive dialing
	PredictiveDialing  bool              `json:"predictive_dialing"`
	AbandonRate        float64           `json:"abandon_rate,omitempty"`
	MaxAbandonRate     float64           `json:"max_abandon_rate"`
	AbandonRateCompliant bool            `json:"abandon_rate_compliant"`
	
	// Live agent transfer
	LiveAgentRequired  bool              `json:"live_agent_required"`
	LiveAgentAvailable bool              `json:"live_agent_available"`
	TransferTime       time.Duration     `json:"transfer_time,omitempty"`
	MaxTransferTime    time.Duration     `json:"max_transfer_time"`
	
	// Silent calls
	SilentCallDetection bool             `json:"silent_call_detection"`
	SilentCallCount    int               `json:"silent_call_count"`
	MaxSilentCalls     int               `json:"max_silent_calls"`
}

// AutoDialingViolation represents an automated dialing violation
type AutoDialingViolation struct {
	ViolationType      string            `json:"violation_type"` // no_consent, prerecorded_without_consent, excessive_abandon_rate
	CallID             uuid.UUID         `json:"call_id"`
	ViolationDate      time.Time         `json:"violation_date"`
	TechnologyUsed     string            `json:"technology_used"`
	Description        string            `json:"description"`
	Severity           string            `json:"severity"`
	RegulatoryReference string           `json:"regulatory_reference,omitempty"`
}

// TCPACallRecord represents a single call record for TCPA analysis
type TCPACallRecord struct {
	CallID             uuid.UUID         `json:"call_id"`
	Timestamp          time.Time         `json:"timestamp"`
	LocalTime          string            `json:"local_time"`
	
	// Call details
	Direction          string            `json:"direction"` // inbound, outbound
	CallType           string            `json:"call_type"` // marketing, informational, transactional
	CallPurpose        string            `json:"call_purpose"`
	Duration           time.Duration     `json:"duration"`
	CallResult         string            `json:"call_result"` // answered, no_answer, busy, failed
	
	// Caller information
	CallerID           string            `json:"caller_id"`
	CallerName         string            `json:"caller_name,omitempty"`
	CallingNumber      string            `json:"calling_number"`
	
	// Campaign information
	CampaignID         string            `json:"campaign_id,omitempty"`
	CampaignName       string            `json:"campaign_name,omitempty"`
	
	// Technology used
	AutoDialer         bool              `json:"auto_dialer"`
	PrerecordedMessage bool              `json:"prerecorded_message"`
	ArtificialVoice    bool              `json:"artificial_voice"`
	
	// Consent status at time of call
	ConsentStatus      string            `json:"consent_status"`
	ConsentDate        *time.Time        `json:"consent_date,omitempty"`
	ConsentType        string            `json:"consent_type,omitempty"`
	
	// DNC status at time of call
	FederalDNC         bool              `json:"federal_dnc"`
	StateDNC           bool              `json:"state_dnc"`
	InternalDNC        bool              `json:"internal_dnc"`
	
	// Compliance assessment
	IsCompliant        bool              `json:"is_compliant"`
	ViolationTypes     []string          `json:"violation_types,omitempty"`
	ComplianceScore    float64           `json:"compliance_score"` // 0-100
	
	// Agent information
	AgentID            string            `json:"agent_id,omitempty"`
	AgentName          string            `json:"agent_name,omitempty"`
	
	// Recording information
	IsRecorded         bool              `json:"is_recorded"`
	RecordingConsent   bool              `json:"recording_consent"`
	RecordingID        string            `json:"recording_id,omitempty"`
	
	// Opt-out requests
	OptOutRequested    bool              `json:"opt_out_requested"`
	OptOutMethod       string            `json:"opt_out_method,omitempty"`
	OptOutProcessed    bool              `json:"opt_out_processed"`
	
	// Quality and monitoring
	QualityScore       float64           `json:"quality_score,omitempty"` // 0-100
	MonitoringFlags    []string          `json:"monitoring_flags,omitempty"`
	
	// Metadata
	RequestID          string            `json:"request_id,omitempty"`
	SessionID          string            `json:"session_id,omitempty"`
	Environment        string            `json:"environment"`
}

// TCPAConsentHistory represents consent history for TCPA compliance
type TCPAConsentHistory struct {
	// Current consent status
	CurrentStatus      string            `json:"current_status"` // granted, revoked, expired, none
	LastUpdated        time.Time         `json:"last_updated"`
	
	// Consent events
	ConsentEvents      []*TCPAConsentEvent `json:"consent_events"`
	
	// Consent scope
	ConsentedPurposes  []string          `json:"consented_purposes"`
	ConsentedChannels  []string          `json:"consented_channels"`
	ConsentedCallTypes []string          `json:"consented_call_types"`
	
	// Consent quality
	IsExpressWritten   bool              `json:"is_express_written"`
	IsExpressOral      bool              `json:"is_express_oral"`
	ConsentEvidence    []string          `json:"consent_evidence,omitempty"`
	
	// Consent lifecycle
	FirstConsent       *time.Time        `json:"first_consent,omitempty"`
	LastConsent        *time.Time        `json:"last_consent,omitempty"`
	ConsentCount       int               `json:"consent_count"`
	RevocationCount    int               `json:"revocation_count"`
	
	// Compliance assessment
	ConsentCompliance  float64           `json:"consent_compliance"` // 0-100
	ComplianceIssues   []string          `json:"compliance_issues,omitempty"`
}

// TCPAConsentEvent represents a single consent event
type TCPAConsentEvent struct {
	EventID            uuid.UUID         `json:"event_id"`
	Timestamp          time.Time         `json:"timestamp"`
	EventType          string            `json:"event_type"` // granted, revoked, updated, expired
	
	// Consent details
	ConsentType        string            `json:"consent_type"` // express_written, express_oral, implied
	ConsentMethod      string            `json:"consent_method"` // web_form, phone_call, sms, email
	ConsentScope       []string          `json:"consent_scope"`
	
	// Context information
	Channel            string            `json:"channel,omitempty"`
	Source             string            `json:"source,omitempty"`
	UserAgent          string            `json:"user_agent,omitempty"`
	IPAddress          string            `json:"ip_address,omitempty"`
	
	// Evidence and verification
	ConsentEvidence    interface{}       `json:"consent_evidence,omitempty"`
	VerificationMethod string            `json:"verification_method,omitempty"`
	IsVerified         bool              `json:"is_verified"`
	
	// Legal compliance
	IsLegallyValid     bool              `json:"is_legally_valid"`
	ValidationReason   string            `json:"validation_reason,omitempty"`
	
	// Expiry information
	ExpiryDate         *time.Time        `json:"expiry_date,omitempty"`
	AutoExpiry         bool              `json:"auto_expiry"`
}

// DNCStatus represents Do Not Call registry status
type DNCStatus struct {
	PhoneNumber        string            `json:"phone_number"`
	LastChecked        time.Time         `json:"last_checked"`
	
	// Federal DNC status
	FederalDNC         *DNCRegistryStatus `json:"federal_dnc"`
	
	// State DNC status
	StateDNC           map[string]*DNCRegistryStatus `json:"state_dnc,omitempty"`
	
	// Internal DNC status
	InternalDNC        *InternalDNCStatus `json:"internal_dnc"`
	
	// Wireless registry status
	WirelessDNC        *DNCRegistryStatus `json:"wireless_dnc,omitempty"`
	
	// Overall status
	OverallStatus      string            `json:"overall_status"` // safe, caution, do_not_call
	CanCall            bool              `json:"can_call"`
	RestrictedCallTypes []string         `json:"restricted_call_types,omitempty"`
	
	// Exemptions
	ApplicableExemptions []DNCExemption   `json:"applicable_exemptions,omitempty"`
	
	// Check history
	CheckHistory       []*DNCCheckRecord `json:"check_history,omitempty"`
}

// DNCRegistryStatus represents status in a specific DNC registry
type DNCRegistryStatus struct {
	IsRegistered       bool              `json:"is_registered"`
	RegistrationDate   *time.Time        `json:"registration_date,omitempty"`
	RegistryName       string            `json:"registry_name"`
	RegistryType       string            `json:"registry_type"` // federal, state, wireless
	
	// Registration details
	RegistrationMethod string            `json:"registration_method,omitempty"`
	RegistrationSource string            `json:"registration_source,omitempty"`
	
	// Status validation
	StatusConfidence   float64           `json:"status_confidence"` // 0-100
	LastVerified       time.Time         `json:"last_verified"`
	VerificationMethod string            `json:"verification_method"`
	
	// Expiry information
	ExpiryDate         *time.Time        `json:"expiry_date,omitempty"`
	AutoRenewal        bool              `json:"auto_renewal"`
}

// InternalDNCStatus represents internal Do Not Call status
type InternalDNCStatus struct {
	IsRegistered       bool              `json:"is_registered"`
	RegistrationDate   *time.Time        `json:"registration_date,omitempty"`
	RegistrationReason string            `json:"registration_reason,omitempty"`
	
	// Internal categorization
	DNCCategory        string            `json:"dnc_category,omitempty"` // customer_request, compliance, quality
	Priority           string            `json:"priority"` // high, medium, low
	
	// Removal tracking
	RemovalDate        *time.Time        `json:"removal_date,omitempty"`
	RemovalReason      string            `json:"removal_reason,omitempty"`
	
	// Notes and comments
	Notes              string            `json:"notes,omitempty"`
	LastUpdatedBy      string            `json:"last_updated_by,omitempty"`
}

// DNCExemption represents an exemption from DNC restrictions
type DNCExemption struct {
	ExemptionType      string            `json:"exemption_type"` // established_business_relationship, prior_consent, inquiry_response
	Description        string            `json:"description"`
	IsValid            bool              `json:"is_valid"`
	ExpiryDate         *time.Time        `json:"expiry_date,omitempty"`
	Evidence           []string          `json:"evidence,omitempty"`
	ApplicableCallTypes []string         `json:"applicable_call_types"`
}

// DNCCheckRecord represents a historical DNC check
type DNCCheckRecord struct {
	CheckDate          time.Time         `json:"check_date"`
	CheckType          string            `json:"check_type"` // manual, automated, bulk
	Result             string            `json:"result"` // safe, registered, error
	DataSource         string            `json:"data_source"`
	Confidence         float64           `json:"confidence"` // 0-100
	ProcessingTime     time.Duration     `json:"processing_time"`
}

// WirelessCarrierInfo represents wireless carrier information
type WirelessCarrierInfo struct {
	PhoneNumber        string            `json:"phone_number"`
	IsWireless         bool              `json:"is_wireless"`
	
	// Current carrier
	CurrentCarrier     *CarrierInfo      `json:"current_carrier,omitempty"`
	
	// Original carrier (before porting)
	OriginalCarrier    *CarrierInfo      `json:"original_carrier,omitempty"`
	
	// Porting information
	IsPortedNumber     bool              `json:"is_ported_number"`
	PortDate           *time.Time        `json:"port_date,omitempty"`
	PortingHistory     []*PortingRecord  `json:"porting_history,omitempty"`
	
	// Network information
	NetworkType        string            `json:"network_type,omitempty"` // GSM, CDMA, LTE, 5G
	TechnologySupport  []string          `json:"technology_support,omitempty"`
	
	// Regulatory information
	IsRegulated        bool              `json:"is_regulated"`
	RegulatoryStatus   string            `json:"regulatory_status,omitempty"`
	
	// Data quality
	DataConfidence     float64           `json:"data_confidence"` // 0-100
	LastUpdated        time.Time         `json:"last_updated"`
	DataSource         string            `json:"data_source"`
}

// CarrierInfo represents information about a wireless carrier
type CarrierInfo struct {
	CarrierName        string            `json:"carrier_name"`
	CarrierCode        string            `json:"carrier_code,omitempty"`
	CarrierType        string            `json:"carrier_type"` // major, regional, mvno
	
	// Network information
	NetworkCoverage    []string          `json:"network_coverage,omitempty"`
	ServiceTypes       []string          `json:"service_types,omitempty"`
	
	// TCPA considerations
	TCPACompliance     *CarrierTCPAInfo  `json:"tcpa_compliance,omitempty"`
}

// CarrierTCPAInfo represents TCPA-specific carrier information
type CarrierTCPAInfo struct {
	RequiresConsent    bool              `json:"requires_consent"`
	ConsentType        string            `json:"consent_type,omitempty"`
	HasSpecialRules    bool              `json:"has_special_rules"`
	SpecialRules       []string          `json:"special_rules,omitempty"`
	ContactPreferences []string          `json:"contact_preferences,omitempty"`
}

// PortingRecord represents a number porting event
type PortingRecord struct {
	PortDate           time.Time         `json:"port_date"`
	FromCarrier        string            `json:"from_carrier"`
	ToCarrier          string            `json:"to_carrier"`
	PortType           string            `json:"port_type"` // in, out, through
	PortReason         string            `json:"port_reason,omitempty"`
}

// CallingPatterns represents analysis of calling patterns
type CallingPatterns struct {
	// Temporal patterns
	HourlyDistribution map[int]int64     `json:"hourly_distribution"`
	DailyDistribution  map[string]int64  `json:"daily_distribution"`
	MonthlyDistribution map[string]int64 `json:"monthly_distribution"`
	
	// Call frequency
	CallsPerDay        float64           `json:"calls_per_day"`
	CallsPerWeek       float64           `json:"calls_per_week"`
	CallsPerMonth      float64           `json:"calls_per_month"`
	
	// Call timing analysis
	PeakHours          []int             `json:"peak_hours"`
	QuietHours         []int             `json:"quiet_hours"`
	WeekendCalls       int64             `json:"weekend_calls"`
	HolidayCalls       int64             `json:"holiday_calls"`
	
	// Pattern analysis
	PatternType        string            `json:"pattern_type"` // regular, irregular, burst, sporadic
	Regularity         float64           `json:"regularity"` // 0-100
	Predictability     float64           `json:"predictability"` // 0-100
	
	// Seasonal patterns
	SeasonalTrends     map[string]float64 `json:"seasonal_trends,omitempty"`
	
	// Anomaly detection
	AnomalousPatterns  []*CallingAnomaly `json:"anomalous_patterns,omitempty"`
	
	// Business implications
	OptimalCallTimes   []string          `json:"optimal_call_times,omitempty"`
	AvoidTimes         []string          `json:"avoid_times,omitempty"`
}

// CallingAnomaly represents an anomalous calling pattern
type CallingAnomaly struct {
	AnomalyType        string            `json:"anomaly_type"` // volume_spike, unusual_timing, frequency_change
	DetectedDate       time.Time         `json:"detected_date"`
	Description        string            `json:"description"`
	Severity           string            `json:"severity"`
	Impact             string            `json:"impact,omitempty"`
	Recommendation     string            `json:"recommendation,omitempty"`
}

// TCPAComplianceScore represents overall TCPA compliance scoring
type TCPAComplianceScore struct {
	OverallScore       float64           `json:"overall_score"` // 0-100
	ScoreBreakdown     *ScoreBreakdown   `json:"score_breakdown"`
	
	// Risk assessment
	RiskLevel          string            `json:"risk_level"` // low, medium, high, critical
	RiskScore          float64           `json:"risk_score"` // 0-100
	
	// Compliance trends
	ScoreTrend         string            `json:"score_trend"` // improving, declining, stable
	PreviousScore      float64           `json:"previous_score,omitempty"`
	ScoreChange        float64           `json:"score_change,omitempty"`
	
	// Benchmarking
	IndustryAverage    float64           `json:"industry_average,omitempty"`
	PeerRanking        string            `json:"peer_ranking,omitempty"`
	
	// Scoring metadata
	ScoredAt           time.Time         `json:"scored_at"`
	ScoringMethod      string            `json:"scoring_method"`
	ConfidenceLevel    float64           `json:"confidence_level"` // 0-100
}

// ScoreBreakdown provides detailed scoring breakdown
type ScoreBreakdown struct {
	ConsentScore       float64           `json:"consent_score"`
	TimeComplianceScore float64          `json:"time_compliance_score"`
	FrequencyScore     float64           `json:"frequency_score"`
	ContentScore       float64           `json:"content_score"`
	DNCScore           float64           `json:"dnc_score"`
	RecordKeepingScore float64           `json:"record_keeping_score"`
	CallerIDScore      float64           `json:"caller_id_score"`
	AutoDialingScore   float64           `json:"auto_dialing_score"`
	
	// Weight information
	ScoreWeights       map[string]float64 `json:"score_weights"`
}

// TCPAViolation represents a TCPA violation
type TCPAViolation struct {
	ID                 uuid.UUID         `json:"id"`
	ViolationType      string            `json:"violation_type"`
	ViolationCategory  string            `json:"violation_category"` // consent, timing, frequency, content, dnc, technology
	Description        string            `json:"description"`
	Severity           string            `json:"severity"` // low, medium, high, critical
	
	// Violation details
	CallID             uuid.UUID         `json:"call_id,omitempty"`
	ViolationDate      time.Time         `json:"violation_date"`
	DetectedDate       time.Time         `json:"detected_date"`
	
	// Context
	PhoneNumber        string            `json:"phone_number"`
	CallPurpose        string            `json:"call_purpose,omitempty"`
	CampaignID         string            `json:"campaign_id,omitempty"`
	
	// Regulatory reference
	RegulatorySection  string            `json:"regulatory_section,omitempty"`
	LegalCitation      string            `json:"legal_citation,omitempty"`
	
	// Financial impact
	PotentialFine      float64           `json:"potential_fine,omitempty"`
	FineRange          string            `json:"fine_range,omitempty"`
	LiabilityRisk      string            `json:"liability_risk,omitempty"`
	
	// Resolution tracking
	Status             string            `json:"status"` // open, investigating, resolved, disputed
	ResolutionPlan     string            `json:"resolution_plan,omitempty"`
	ResolvedDate       *time.Time        `json:"resolved_date,omitempty"`
	ResolvedBy         string            `json:"resolved_by,omitempty"`
	
	// Recurrence tracking
	IsRecurring        bool              `json:"is_recurring"`
	RecurrenceCount    int               `json:"recurrence_count,omitempty"`
	LastOccurrence     *time.Time        `json:"last_occurrence,omitempty"`
	
	// Impact assessment
	CustomerImpact     string            `json:"customer_impact,omitempty"`
	BusinessImpact     string            `json:"business_impact,omitempty"`
	ReputationalRisk   string            `json:"reputational_risk,omitempty"`
}

// TCPARecommendation represents a TCPA compliance recommendation
type TCPARecommendation struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"` // immediate, short_term, long_term, strategic
	Priority           string            `json:"priority"` // low, medium, high, critical
	Category           string            `json:"category"` // consent, timing, technology, process
	
	// Recommendation details
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Rationale          string            `json:"rationale"`
	
	// Benefits
	ComplianceBenefit  string            `json:"compliance_benefit"`
	RiskReduction      float64           `json:"risk_reduction,omitempty"` // 0-100
	CostSavings        float64           `json:"cost_savings,omitempty"`
	
	// Implementation
	Implementation     *ImplementationPlan `json:"implementation,omitempty"`
	
	// Impact assessment
	BusinessImpact     string            `json:"business_impact,omitempty"`
	TechnicalImpact    string            `json:"technical_impact,omitempty"`
	OperationalImpact  string            `json:"operational_impact,omitempty"`
	
	// Tracking
	Status             string            `json:"status"` // pending, approved, in_progress, completed, rejected
	AssignedTo         string            `json:"assigned_to,omitempty"`
	DueDate            *time.Time        `json:"due_date,omitempty"`
	
	// Related information
	RelatedViolations  []uuid.UUID       `json:"related_violations,omitempty"`
	RelatedRegulations []string          `json:"related_regulations,omitempty"`
}

// ImplementationPlan provides implementation details for recommendations
type ImplementationPlan struct {
	EstimatedEffort    string            `json:"estimated_effort"` // hours, days, weeks
	RequiredResources  []string          `json:"required_resources"`
	Timeline           string            `json:"timeline"`
	Milestones         []string          `json:"milestones,omitempty"`
	Dependencies       []string          `json:"dependencies,omitempty"`
	SuccessMetrics     []string          `json:"success_metrics,omitempty"`
}

// TCPARiskAssessment provides TCPA-specific risk assessment
type TCPARiskAssessment struct {
	OverallRiskLevel   string            `json:"overall_risk_level"` // low, medium, high, critical
	RiskScore          float64           `json:"risk_score"` // 0-100
	
	// Risk categories
	ConsentRisk        *RiskCategoryAssessment `json:"consent_risk"`
	TimingRisk         *RiskCategoryAssessment `json:"timing_risk"`
	FrequencyRisk      *RiskCategoryAssessment `json:"frequency_risk"`
	ContentRisk        *RiskCategoryAssessment `json:"content_risk"`
	DNCRisk            *RiskCategoryAssessment `json:"dnc_risk"`
	TechnologyRisk     *RiskCategoryAssessment `json:"technology_risk"`
	
	// Financial risk
	FinancialRisk      *FinancialRiskAssessment `json:"financial_risk"`
	
	// Operational risk
	OperationalRisk    *OperationalRiskAssessment `json:"operational_risk"`
	
	// Legal risk
	LegalRisk          *LegalRiskAssessment    `json:"legal_risk"`
	
	// Risk trends
	RiskTrend          string            `json:"risk_trend"` // increasing, decreasing, stable
	RiskVelocity       float64           `json:"risk_velocity"` // Rate of change
	
	// Mitigation effectiveness
	MitigationCoverage float64           `json:"mitigation_coverage"` // % of risks mitigated
	
	// Assessment metadata
	AssessedAt         time.Time         `json:"assessed_at"`
	AssessmentMethod   string            `json:"assessment_method"`
	ConfidenceLevel    float64           `json:"confidence_level"` // 0-100
	NextReviewDue      time.Time         `json:"next_review_due"`
}

// RiskCategoryAssessment provides risk assessment for a specific category
type RiskCategoryAssessment struct {
	RiskLevel          string            `json:"risk_level"`
	RiskScore          float64           `json:"risk_score"` // 0-100
	KeyRisks           []string          `json:"key_risks"`
	Likelihood         float64           `json:"likelihood"` // 0-100
	Impact             float64           `json:"impact"` // 0-100
	MitigationStatus   string            `json:"mitigation_status"`
	Recommendations    []string          `json:"recommendations,omitempty"`
}

// FinancialRiskAssessment provides financial risk details
type FinancialRiskAssessment struct {
	EstimatedExposure  float64           `json:"estimated_exposure"`
	MaxPotentialLoss   float64           `json:"max_potential_loss"`
	MinPotentialLoss   float64           `json:"min_potential_loss"`
	ExpectedLoss       float64           `json:"expected_loss"`
	
	// Fine risk breakdown
	FineRiskByViolationType map[string]float64 `json:"fine_risk_by_violation_type"`
	
	// Insurance and coverage
	InsuranceCoverage  float64           `json:"insurance_coverage,omitempty"`
	UncoveredExposure  float64           `json:"uncovered_exposure,omitempty"`
	
	// Cost factors
	LegalCosts         float64           `json:"legal_costs,omitempty"`
	RemediationCosts   float64           `json:"remediation_costs,omitempty"`
	BusinessDisruption float64           `json:"business_disruption,omitempty"`
}

// OperationalRiskAssessment provides operational risk details
type OperationalRiskAssessment struct {
	ProcessRisk        float64           `json:"process_risk"` // 0-100
	TechnologyRisk     float64           `json:"technology_risk"` // 0-100
	PersonnelRisk      float64           `json:"personnel_risk"` // 0-100
	TrainingRisk       float64           `json:"training_risk"` // 0-100
	
	// Key operational risks
	KeyRisks           []string          `json:"key_risks"`
	ProcessGaps        []string          `json:"process_gaps,omitempty"`
	TechnologyGaps     []string          `json:"technology_gaps,omitempty"`
	SkillGaps          []string          `json:"skill_gaps,omitempty"`
	
	// Mitigation measures
	ExistingControls   []string          `json:"existing_controls"`
	RequiredControls   []string          `json:"required_controls,omitempty"`
}

// LegalRiskAssessment provides legal risk details
type LegalRiskAssessment struct {
	LitigationRisk     float64           `json:"litigation_risk"` // 0-100
	RegulatoryRisk     float64           `json:"regulatory_risk"` // 0-100
	ComplianceRisk     float64           `json:"compliance_risk"` // 0-100
	
	// Legal exposure
	ClassActionRisk    bool              `json:"class_action_risk"`
	StatutoryDamages   float64           `json:"statutory_damages,omitempty"`
	PunitiveDamages    float64           `json:"punitive_damages,omitempty"`
	
	// Regulatory attention
	RegulatoryScrutiny string            `json:"regulatory_scrutiny"` // low, medium, high
	EnforcementHistory []string          `json:"enforcement_history,omitempty"`
	
	// Legal precedents
	RelevantCases      []string          `json:"relevant_cases,omitempty"`
	IndustryEnforcement []string         `json:"industry_enforcement,omitempty"`
}