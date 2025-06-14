package audit

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// AccessAuditCriteria defines criteria for access audit reports
type AccessAuditCriteria struct {
	// Time range for the audit
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	
	// Scope of access audit
	UserIDs            []string          `json:"user_ids,omitempty"`
	ResourceTypes      []string          `json:"resource_types,omitempty"`
	ResourceIDs        []string          `json:"resource_ids,omitempty"`
	AccessTypes        []string          `json:"access_types,omitempty"` // read, write, delete, admin
	
	// Access patterns to analyze
	AnalyzeSuccessful  bool              `json:"analyze_successful"`
	AnalyzeFailed      bool              `json:"analyze_failed"`
	AnalyzePrivileged  bool              `json:"analyze_privileged"`
	AnalyzeUnusual     bool              `json:"analyze_unusual"`
	
	// Risk assessment
	IncludeRiskAnalysis bool             `json:"include_risk_analysis"`
	RiskThreshold      float64           `json:"risk_threshold,omitempty"` // 0-100
	
	// Compliance focus
	ComplianceFrameworks []string        `json:"compliance_frameworks,omitempty"` // SOX, HIPAA, PCI-DSS, etc.
	DataClassifications []string         `json:"data_classifications,omitempty"`
	
	// Aggregation and grouping
	GroupBy            []string          `json:"group_by,omitempty"` // user, resource, time, department
	TimeGranularity    string            `json:"time_granularity,omitempty"` // hour, day, week
	
	// Detail level
	DetailLevel        string            `json:"detail_level"` // summary, detailed, comprehensive
	IncludeMetadata    bool              `json:"include_metadata"`
	
	// Event filtering
	EventFilter        EventFilter       `json:"event_filter,omitempty"`
	
	// Performance options
	MaxResults         int               `json:"max_results,omitempty"`
	UseCache           bool              `json:"use_cache"`
}

// AccessAuditReport represents a comprehensive access audit report
type AccessAuditReport struct {
	// Report metadata
	ID                 string                    `json:"id"`
	GeneratedAt        time.Time                 `json:"generated_at"`
	Criteria           AccessAuditCriteria       `json:"criteria"`
	
	// Executive summary
	Summary            *AccessAuditSummary       `json:"summary"`
	
	// Access analysis
	AccessPatterns     *AccessPatternAnalysis    `json:"access_patterns"`
	UserActivity       []*UserAccessActivity     `json:"user_activity,omitempty"`
	ResourceAccess     []*ResourceAccessAnalysis `json:"resource_access,omitempty"`
	
	// Security analysis
	SecurityFindings   *AccessSecurityFindings   `json:"security_findings"`
	RiskAssessment     *AccessRiskAssessment     `json:"risk_assessment,omitempty"`
	
	// Compliance analysis
	ComplianceStatus   *AccessComplianceStatus   `json:"compliance_status,omitempty"`
	PolicyViolations   []*AccessPolicyViolation  `json:"policy_violations,omitempty"`
	
	// Temporal analysis
	TimeBasedAnalysis  *TimeBasedAccessAnalysis  `json:"time_based_analysis,omitempty"`
	
	// Anomaly detection
	Anomalies          []*AccessAnomaly          `json:"anomalies,omitempty"`
	
	// Recommendations
	Recommendations    []*AccessRecommendation   `json:"recommendations,omitempty"`
	
	// Report metrics
	ReportTime         time.Duration             `json:"report_time"`
	EventsAnalyzed     int64                     `json:"events_analyzed"`
	DataSources        []string                  `json:"data_sources"`
}

// AccessAuditSummary provides high-level access audit insights
type AccessAuditSummary struct {
	// Overall metrics
	TotalAccessEvents  int64             `json:"total_access_events"`
	UniqueUsers        int64             `json:"unique_users"`
	UniqueResources    int64             `json:"unique_resources"`
	
	// Access outcomes
	SuccessfulAccess   int64             `json:"successful_access"`
	FailedAccess       int64             `json:"failed_access"`
	SuccessRate        float64           `json:"success_rate"`
	
	// Risk indicators
	HighRiskEvents     int64             `json:"high_risk_events"`
	PolicyViolations   int64             `json:"policy_violations"`
	AnomalyCount       int64             `json:"anomaly_count"`
	
	// User patterns
	TopUsers           []UserAccessSummary `json:"top_users,omitempty"`
	NewUsers           int64             `json:"new_users"`
	InactiveUsers      int64             `json:"inactive_users"`
	
	// Resource patterns
	TopResources       []ResourceAccessSummary `json:"top_resources,omitempty"`
	SensitiveAccess    int64             `json:"sensitive_access"`
	
	// Temporal patterns
	PeakAccessHours    []int             `json:"peak_access_hours"`
	WeekendAccess      int64             `json:"weekend_access"`
	
	// Compliance indicators
	ComplianceScore    float64           `json:"compliance_score,omitempty"` // 0-100
	ComplianceGaps     int               `json:"compliance_gaps"`
}

// UserAccessSummary provides summary of user access patterns
type UserAccessSummary struct {
	UserID             string            `json:"user_id"`
	UserName           string            `json:"user_name,omitempty"`
	Department         string            `json:"department,omitempty"`
	Role               string            `json:"role,omitempty"`
	AccessCount        int64             `json:"access_count"`
	ResourcesAccessed  int64             `json:"resources_accessed"`
	FailedAttempts     int64             `json:"failed_attempts"`
	RiskScore          float64           `json:"risk_score,omitempty"`
	LastAccess         time.Time         `json:"last_access"`
}

// ResourceAccessSummary provides summary of resource access patterns
type ResourceAccessSummary struct {
	ResourceID         string            `json:"resource_id"`
	ResourceType       string            `json:"resource_type"`
	ResourceName       string            `json:"resource_name,omitempty"`
	Classification     string            `json:"classification,omitempty"`
	AccessCount        int64             `json:"access_count"`
	UniqueUsers        int64             `json:"unique_users"`
	FailedAttempts     int64             `json:"failed_attempts"`
	RiskScore          float64           `json:"risk_score,omitempty"`
	LastAccess         time.Time         `json:"last_access"`
}

// AccessPatternAnalysis provides detailed access pattern analysis
type AccessPatternAnalysis struct {
	// Overall patterns
	AccessTrends       *TrendAnalysis    `json:"access_trends"`
	
	// User behavior patterns
	UserPatterns       []*UserAccessPattern   `json:"user_patterns,omitempty"`
	
	// Resource access patterns  
	ResourcePatterns   []*ResourceAccessPattern `json:"resource_patterns,omitempty"`
	
	// Temporal patterns
	TemporalPatterns   []*TemporalAccessPattern `json:"temporal_patterns,omitempty"`
	
	// Geographic patterns
	GeographicPatterns []*GeographicAccessPattern `json:"geographic_patterns,omitempty"`
	
	// Technology patterns
	TechnologyPatterns []*TechnologyAccessPattern `json:"technology_patterns,omitempty"`
}

// UserAccessActivity represents detailed user access activity
type UserAccessActivity struct {
	UserID             string            `json:"user_id"`
	UserName           string            `json:"user_name,omitempty"`
	Department         string            `json:"department,omitempty"`
	Role               string            `json:"role,omitempty"`
	
	// Activity metrics
	TotalAccess        int64             `json:"total_access"`
	SuccessfulAccess   int64             `json:"successful_access"`
	FailedAccess       int64             `json:"failed_access"`
	
	// Resource interaction
	ResourceTypes      map[string]int64  `json:"resource_types"`
	SensitiveAccess    int64             `json:"sensitive_access"`
	PrivilegedAccess   int64             `json:"privileged_access"`
	
	// Temporal patterns
	ActiveHours        []int             `json:"active_hours"`
	ActiveDays         []string          `json:"active_days"`
	FirstAccess        time.Time         `json:"first_access"`
	LastAccess         time.Time         `json:"last_access"`
	
	// Risk assessment
	RiskScore          float64           `json:"risk_score"`
	RiskFactors        []string          `json:"risk_factors,omitempty"`
	
	// Compliance
	PolicyViolations   int               `json:"policy_violations"`
	ComplianceIssues   []string          `json:"compliance_issues,omitempty"`
	
	// Recent activity
	RecentEvents       []*AccessEvent    `json:"recent_events,omitempty"`
}

// AccessEvent represents a single access event for reporting
type AccessEvent struct {
	EventID            uuid.UUID         `json:"event_id"`
	Timestamp          time.Time         `json:"timestamp"`
	UserID             string            `json:"user_id"`
	ResourceID         string            `json:"resource_id"`
	ResourceType       string            `json:"resource_type"`
	AccessType         string            `json:"access_type"`
	Result             string            `json:"result"`
	IPAddress          string            `json:"ip_address,omitempty"`
	UserAgent          string            `json:"user_agent,omitempty"`
	RiskScore          float64           `json:"risk_score,omitempty"`
	IsAnomaly          bool              `json:"is_anomaly"`
}

// ResourceAccessAnalysis represents detailed resource access analysis
type ResourceAccessAnalysis struct {
	ResourceID         string            `json:"resource_id"`
	ResourceType       string            `json:"resource_type"`
	ResourceName       string            `json:"resource_name,omitempty"`
	Classification     string            `json:"classification,omitempty"`
	Owner              string            `json:"owner,omitempty"`
	
	// Access metrics
	TotalAccess        int64             `json:"total_access"`
	UniqueUsers        int64             `json:"unique_users"`
	SuccessfulAccess   int64             `json:"successful_access"`
	FailedAccess       int64             `json:"failed_access"`
	
	// User patterns
	TopUsers           []UserAccessSummary `json:"top_users,omitempty"`
	AccessDistribution map[string]int64  `json:"access_distribution"` // department/role -> count
	
	// Temporal patterns
	AccessTrends       *TrendAnalysis    `json:"access_trends,omitempty"`
	PeakUsage          []string          `json:"peak_usage,omitempty"`
	
	// Security assessment
	SecurityEvents     int64             `json:"security_events"`
	RiskScore          float64           `json:"risk_score"`
	ThreatIndicators   []string          `json:"threat_indicators,omitempty"`
	
	// Compliance
	ComplianceEvents   int64             `json:"compliance_events"`
	DataClassification string            `json:"data_classification,omitempty"`
	RegulationsApplied []string          `json:"regulations_applied,omitempty"`
}

// AccessSecurityFindings represents security-related findings from access audit
type AccessSecurityFindings struct {
	// Overall security posture
	SecurityScore      float64           `json:"security_score"` // 0-100
	SecurityLevel      string            `json:"security_level"` // poor, fair, good, excellent
	
	// Threat indicators
	ThreatIndicators   []*ThreatIndicator     `json:"threat_indicators,omitempty"`
	SecurityIncidents  []*SecurityIncident    `json:"security_incidents,omitempty"`
	
	// Access anomalies
	AnomalousAccess    []*AccessAnomaly       `json:"anomalous_access,omitempty"`
	SuspiciousPatterns []*SuspiciousPattern   `json:"suspicious_patterns,omitempty"`
	
	// Privilege analysis
	PrivilegeAnalysis  *PrivilegeAnalysis     `json:"privilege_analysis,omitempty"`
	
	// Failed access analysis
	FailedAccessAnalysis *FailedAccessAnalysis `json:"failed_access_analysis,omitempty"`
	
	// Vulnerability indicators
	VulnerabilityIndicators []string         `json:"vulnerability_indicators,omitempty"`
	
	// Recommendations
	SecurityRecommendations []string         `json:"security_recommendations,omitempty"`
}

// ThreatIndicator represents a potential security threat
type ThreatIndicator struct {
	IndicatorType      string            `json:"indicator_type"` // unusual_access, privilege_escalation, data_exfiltration
	Severity           string            `json:"severity"`
	Description        string            `json:"description"`
	Evidence           []string          `json:"evidence"`
	FirstSeen          time.Time         `json:"first_seen"`
	LastSeen           time.Time         `json:"last_seen"`
	Confidence         float64           `json:"confidence"` // 0-100
	AffectedUsers      []string          `json:"affected_users,omitempty"`
	AffectedResources  []string          `json:"affected_resources,omitempty"`
	Recommendation     string            `json:"recommendation,omitempty"`
}

// SecurityIncident represents a security incident identified in access audit
type SecurityIncident struct {
	IncidentID         string            `json:"incident_id"`
	IncidentType       string            `json:"incident_type"`
	Severity           string            `json:"severity"`
	Description        string            `json:"description"`
	DetectedAt         time.Time         `json:"detected_at"`
	Status             string            `json:"status"` // open, investigating, resolved
	AffectedUsers      []string          `json:"affected_users"`
	AffectedResources  []string          `json:"affected_resources"`
	EventCount         int64             `json:"event_count"`
	TimeSpan           time.Duration     `json:"time_span"`
	RiskScore          float64           `json:"risk_score"`
	ImpactAssessment   string            `json:"impact_assessment,omitempty"`
}

// AccessAnomaly represents an anomalous access pattern
type AccessAnomaly struct {
	AnomalyID          string            `json:"anomaly_id"`
	AnomalyType        string            `json:"anomaly_type"` // time, location, resource, volume, pattern
	Severity           string            `json:"severity"`
	Description        string            `json:"description"`
	
	// Detection details
	DetectedAt         time.Time         `json:"detected_at"`
	AnomalyWindow      TimeRange         `json:"anomaly_window"`
	Confidence         float64           `json:"confidence"` // 0-100
	
	// Affected entities
	AffectedUser       string            `json:"affected_user,omitempty"`
	AffectedResource   string            `json:"affected_resource,omitempty"`
	AffectedEvents     []uuid.UUID       `json:"affected_events,omitempty"`
	
	// Analysis
	ExpectedBehavior   string            `json:"expected_behavior"`
	ObservedBehavior   string            `json:"observed_behavior"`
	DeviationScore     float64           `json:"deviation_score"`
	
	// Context
	BaselinePeriod     TimeRange         `json:"baseline_period,omitempty"`
	HistoricalContext  string            `json:"historical_context,omitempty"`
	
	// Response
	RequiresInvestigation bool           `json:"requires_investigation"`
	AutomatedResponse  string            `json:"automated_response,omitempty"`
	Recommendation     string            `json:"recommendation,omitempty"`
}

// SuspiciousPattern represents a suspicious access pattern
type SuspiciousPattern struct {
	PatternID          string            `json:"pattern_id"`
	PatternType        string            `json:"pattern_type"` // credential_stuffing, privilege_escalation, data_mining
	Description        string            `json:"description"`
	Severity           string            `json:"severity"`
	
	// Pattern characteristics
	Frequency          float64           `json:"frequency"`
	Duration           time.Duration     `json:"duration"`
	AffectedUsers      []string          `json:"affected_users"`
	AffectedResources  []string          `json:"affected_resources"`
	
	// Risk assessment
	RiskScore          float64           `json:"risk_score"`
	LikelihoodMalicious float64          `json:"likelihood_malicious"` // 0-100
	
	// Detection metadata
	FirstDetected      time.Time         `json:"first_detected"`
	LastDetected       time.Time         `json:"last_detected"`
	DetectionMethod    string            `json:"detection_method"`
	
	// Response recommendation
	RecommendedActions []string          `json:"recommended_actions"`
}

// PrivilegeAnalysis provides analysis of privilege usage
type PrivilegeAnalysis struct {
	// Overall privilege metrics
	TotalPrivilegedUsers int64           `json:"total_privileged_users"`
	PrivilegedAccess     int64           `json:"privileged_access"`
	PrivilegeUtilization float64         `json:"privilege_utilization"` // % of privileged accounts used
	
	// Privilege types
	PrivilegeDistribution map[string]int64 `json:"privilege_distribution"`
	
	// Risk assessment
	ExcessivePrivileges  []*ExcessivePrivilege `json:"excessive_privileges,omitempty"`
	UnusedPrivileges     []*UnusedPrivilege    `json:"unused_privileges,omitempty"`
	PrivilegeEscalation  []*PrivilegeEscalation `json:"privilege_escalation,omitempty"`
	
	// Compliance
	PrivilegeCompliance  float64         `json:"privilege_compliance"` // 0-100
	SODViolations        int             `json:"sod_violations"` // Segregation of Duties
	
	// Recommendations
	PrivilegeRecommendations []string    `json:"privilege_recommendations,omitempty"`
}

// ExcessivePrivilege represents a user with excessive privileges
type ExcessivePrivilege struct {
	UserID             string            `json:"user_id"`
	PrivilegeType      string            `json:"privilege_type"`
	GrantedDate        time.Time         `json:"granted_date"`
	LastUsed           *time.Time        `json:"last_used,omitempty"`
	UsageFrequency     float64           `json:"usage_frequency"`
	BusinessJustification string         `json:"business_justification,omitempty"`
	RiskScore          float64           `json:"risk_score"`
	Recommendation     string            `json:"recommendation"`
}

// UnusedPrivilege represents an unused privilege
type UnusedPrivilege struct {
	UserID             string            `json:"user_id"`
	PrivilegeType      string            `json:"privilege_type"`
	GrantedDate        time.Time         `json:"granted_date"`
	LastUsed           *time.Time        `json:"last_used,omitempty"`
	DaysSinceUsed      int               `json:"days_since_used"`
	Recommendation     string            `json:"recommendation"`
}

// PrivilegeEscalation represents a privilege escalation event
type PrivilegeEscalation struct {
	UserID             string            `json:"user_id"`
	FromPrivilege      string            `json:"from_privilege"`
	ToPrivilege        string            `json:"to_privilege"`
	EscalationDate     time.Time         `json:"escalation_date"`
	Method             string            `json:"method"`
	IsAuthorized       bool              `json:"is_authorized"`
	RiskScore          float64           `json:"risk_score"`
	Evidence           []string          `json:"evidence"`
}

// FailedAccessAnalysis provides analysis of failed access attempts
type FailedAccessAnalysis struct {
	// Overall metrics
	TotalFailedAttempts int64            `json:"total_failed_attempts"`
	FailureRate         float64          `json:"failure_rate"`
	
	// Failure patterns
	FailuresByType      map[string]int64 `json:"failures_by_type"`
	FailuresByUser      []UserFailureSummary `json:"failures_by_user,omitempty"`
	FailuresByResource  []ResourceFailureSummary `json:"failures_by_resource,omitempty"`
	
	// Temporal analysis
	FailureTrends       *TrendAnalysis   `json:"failure_trends,omitempty"`
	PeakFailureTimes    []string         `json:"peak_failure_times,omitempty"`
	
	// Potential attacks
	BruteForceAttempts  []*BruteForceAttempt `json:"brute_force_attempts,omitempty"`
	CredentialStuffing  []*CredentialStuffingAttempt `json:"credential_stuffing,omitempty"`
	
	// Geographic analysis
	FailuresByLocation  map[string]int64 `json:"failures_by_location,omitempty"`
	SuspiciousLocations []string         `json:"suspicious_locations,omitempty"`
}

// UserFailureSummary represents failure summary for a user
type UserFailureSummary struct {
	UserID             string            `json:"user_id"`
	FailureCount       int64             `json:"failure_count"`
	FailureRate        float64           `json:"failure_rate"`
	LastFailure        time.Time         `json:"last_failure"`
	FailureTypes       map[string]int64  `json:"failure_types"`
	IsLocked           bool              `json:"is_locked"`
	RiskScore          float64           `json:"risk_score"`
}

// ResourceFailureSummary represents failure summary for a resource
type ResourceFailureSummary struct {
	ResourceID         string            `json:"resource_id"`
	FailureCount       int64             `json:"failure_count"`
	AttemptingUsers    []string          `json:"attempting_users"`
	LastFailure        time.Time         `json:"last_failure"`
	FailureTypes       map[string]int64  `json:"failure_types"`
	RiskScore          float64           `json:"risk_score"`
}

// BruteForceAttempt represents a potential brute force attack
type BruteForceAttempt struct {
	TargetUser         string            `json:"target_user"`
	SourceIP           string            `json:"source_ip"`
	AttemptCount       int64             `json:"attempt_count"`
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	IsOngoing          bool              `json:"is_ongoing"`
	Confidence         float64           `json:"confidence"` // 0-100
	Blocked            bool              `json:"blocked"`
	BlockedAt          *time.Time        `json:"blocked_at,omitempty"`
}

// CredentialStuffingAttempt represents a potential credential stuffing attack
type CredentialStuffingAttempt struct {
	SourceIP           string            `json:"source_ip"`
	TargetUsers        []string          `json:"target_users"`
	AttemptCount       int64             `json:"attempt_count"`
	SuccessCount       int64             `json:"success_count"`
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	IsOngoing          bool              `json:"is_ongoing"`
	Confidence         float64           `json:"confidence"` // 0-100
	AttackPattern      string            `json:"attack_pattern"`
}

// AccessRiskAssessment provides risk assessment for access patterns
type AccessRiskAssessment struct {
	OverallRiskScore   float64           `json:"overall_risk_score"` // 0-100
	RiskLevel          string            `json:"risk_level"` // low, medium, high, critical
	
	// Risk categories
	UserRisks          []*UserRiskProfile     `json:"user_risks,omitempty"`
	ResourceRisks      []*ResourceRiskProfile `json:"resource_risks,omitempty"`
	TechnicalRisks     []*TechnicalRisk       `json:"technical_risks,omitempty"`
	
	// Risk trends
	RiskTrend          string            `json:"risk_trend"` // increasing, decreasing, stable
	RiskVelocity       float64           `json:"risk_velocity"` // rate of change
	
	// Top risks
	TopRisks           []string          `json:"top_risks"`
	EmergingRisks      []string          `json:"emerging_risks,omitempty"`
	
	// Mitigation status
	MitigatedRisks     []string          `json:"mitigated_risks,omitempty"`
	UnmitigatedRisks   []string          `json:"unmitigated_risks,omitempty"`
	MitigationCoverage float64           `json:"mitigation_coverage"` // % of risks mitigated
}

// UserRiskProfile represents risk profile for a user
type UserRiskProfile struct {
	UserID             string            `json:"user_id"`
	RiskScore          float64           `json:"risk_score"` // 0-100
	RiskLevel          string            `json:"risk_level"`
	RiskFactors        []string          `json:"risk_factors"`
	AnomalyCount       int               `json:"anomaly_count"`
	ViolationCount     int               `json:"violation_count"`
	LastRiskEvent      time.Time         `json:"last_risk_event"`
	TrendDirection     string            `json:"trend_direction"` // increasing, decreasing, stable
	Recommendations    []string          `json:"recommendations,omitempty"`
}

// ResourceRiskProfile represents risk profile for a resource
type ResourceRiskProfile struct {
	ResourceID         string            `json:"resource_id"`
	RiskScore          float64           `json:"risk_score"` // 0-100
	RiskLevel          string            `json:"risk_level"`
	RiskFactors        []string          `json:"risk_factors"`
	ThreatCount        int               `json:"threat_count"`
	VulnerabilityCount int               `json:"vulnerability_count"`
	LastThreatEvent    time.Time         `json:"last_threat_event"`
	ExposureLevel      string            `json:"exposure_level"`
	Recommendations    []string          `json:"recommendations,omitempty"`
}

// TechnicalRisk represents a technical risk factor
type TechnicalRisk struct {
	RiskType           string            `json:"risk_type"` // authentication, authorization, encryption, monitoring
	Description        string            `json:"description"`
	RiskScore          float64           `json:"risk_score"` // 0-100
	Impact             string            `json:"impact"`
	Likelihood         string            `json:"likelihood"`
	DetectionDate      time.Time         `json:"detection_date"`
	AffectedSystems    []string          `json:"affected_systems,omitempty"`
	MitigationStatus   string            `json:"mitigation_status"`
	Recommendations    []string          `json:"recommendations,omitempty"`
}

// AccessComplianceStatus represents compliance status for access controls
type AccessComplianceStatus struct {
	OverallCompliance  float64           `json:"overall_compliance"` // 0-100
	ComplianceLevel    string            `json:"compliance_level"` // poor, fair, good, excellent
	
	// Framework compliance
	FrameworkCompliance map[string]float64 `json:"framework_compliance"` // framework -> score
	
	// Specific compliance areas
	AuthenticationCompliance float64      `json:"authentication_compliance"`
	AuthorizationCompliance  float64      `json:"authorization_compliance"`
	AuditingCompliance       float64      `json:"auditing_compliance"`
	DataProtectionCompliance float64      `json:"data_protection_compliance"`
	
	// Violations
	ActiveViolations   int               `json:"active_violations"`
	ResolvedViolations int               `json:"resolved_violations"`
	
	// Compliance trends
	ComplianceTrend    string            `json:"compliance_trend"` // improving, declining, stable
	LastAssessment     time.Time         `json:"last_assessment"`
	NextAssessment     time.Time         `json:"next_assessment"`
	
	// Gaps and recommendations
	ComplianceGaps     []string          `json:"compliance_gaps,omitempty"`
	Recommendations    []string          `json:"recommendations,omitempty"`
}

// AccessPolicyViolation represents a policy violation
type AccessPolicyViolation struct {
	ViolationID        string            `json:"violation_id"`
	PolicyID           string            `json:"policy_id"`
	PolicyName         string            `json:"policy_name"`
	ViolationType      string            `json:"violation_type"`
	Severity           string            `json:"severity"`
	Description        string            `json:"description"`
	
	// Violation details
	UserID             string            `json:"user_id"`
	ResourceID         string            `json:"resource_id"`
	EventID            uuid.UUID         `json:"event_id"`
	ViolationDate      time.Time         `json:"violation_date"`
	DetectedDate       time.Time         `json:"detected_date"`
	
	// Context
	BusinessContext    string            `json:"business_context,omitempty"`
	TechnicalContext   string            `json:"technical_context,omitempty"`
	
	// Impact
	ImpactAssessment   string            `json:"impact_assessment,omitempty"`
	BusinessImpact     string            `json:"business_impact,omitempty"`
	RiskScore          float64           `json:"risk_score"`
	
	// Resolution
	Status             string            `json:"status"` // open, investigating, resolved, accepted_risk
	ResolutionPlan     string            `json:"resolution_plan,omitempty"`
	ResolvedDate       *time.Time        `json:"resolved_date,omitempty"`
	ResolutionNotes    string            `json:"resolution_notes,omitempty"`
	
	// Recurrence
	IsRecurring        bool              `json:"is_recurring"`
	RecurrenceCount    int               `json:"recurrence_count,omitempty"`
}

// TimeBasedAccessAnalysis provides temporal analysis of access patterns
type TimeBasedAccessAnalysis struct {
	// Time distribution
	HourlyDistribution map[int]int64     `json:"hourly_distribution"`
	DailyDistribution  map[string]int64  `json:"daily_distribution"`
	WeeklyDistribution map[string]int64  `json:"weekly_distribution"`
	MonthlyDistribution map[string]int64 `json:"monthly_distribution"`
	
	// Peak analysis
	PeakHours          []int             `json:"peak_hours"`
	PeakDays           []string          `json:"peak_days"`
	OffHoursAccess     int64             `json:"off_hours_access"`
	WeekendAccess      int64             `json:"weekend_access"`
	HolidayAccess      int64             `json:"holiday_access"`
	
	// Anomalous timing
	UnusualTimeAccess  []*UnusualTimeAccess `json:"unusual_time_access,omitempty"`
	
	// Business hours analysis
	BusinessHoursAccess   int64          `json:"business_hours_access"`
	NonBusinessHoursAccess int64         `json:"non_business_hours_access"`
	BusinessHoursDefinition string       `json:"business_hours_definition"`
	
	// Trends
	AccessTrends       *TrendAnalysis    `json:"access_trends,omitempty"`
	SeasonalPatterns   map[string]float64 `json:"seasonal_patterns,omitempty"`
}

// UnusualTimeAccess represents access at unusual times
type UnusualTimeAccess struct {
	UserID             string            `json:"user_id"`
	AccessTime         time.Time         `json:"access_time"`
	LocalTime          string            `json:"local_time"`
	IsBusinessHours    bool              `json:"is_business_hours"`
	IsWeekend          bool              `json:"is_weekend"`
	IsHoliday          bool              `json:"is_holiday"`
	ResourceID         string            `json:"resource_id"`
	AnomalyScore       float64           `json:"anomaly_score"`
	Context            string            `json:"context,omitempty"`
}

// AccessRecommendation represents a recommendation for improving access controls
type AccessRecommendation struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"` // security, compliance, efficiency, risk_reduction
	Priority           string            `json:"priority"` // low, medium, high, critical
	Category           string            `json:"category"` // authentication, authorization, monitoring, policy
	
	// Recommendation details
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Rationale          string            `json:"rationale"`
	
	// Benefits
	SecurityImprovement string           `json:"security_improvement,omitempty"`
	ComplianceImprovement string         `json:"compliance_improvement,omitempty"`
	RiskReduction      float64           `json:"risk_reduction,omitempty"` // 0-100
	CostSavings        float64           `json:"cost_savings,omitempty"`
	
	// Implementation
	Implementation     *ImplementationPlan `json:"implementation,omitempty"`
	
	// Impact assessment
	BusinessImpact     string            `json:"business_impact,omitempty"`
	TechnicalImpact    string            `json:"technical_impact,omitempty"`
	UserImpact         string            `json:"user_impact,omitempty"`
	
	// Related information
	RelatedViolations  []string          `json:"related_violations,omitempty"`
	RelatedRisks       []string          `json:"related_risks,omitempty"`
	RelatedPolicies    []string          `json:"related_policies,omitempty"`
	
	// Tracking
	Status             string            `json:"status"` // pending, approved, in_progress, completed, rejected
	AssignedTo         string            `json:"assigned_to,omitempty"`
	DueDate            *time.Time        `json:"due_date,omitempty"`
	CompletedDate      *time.Time        `json:"completed_date,omitempty"`
}

// UserAccessPattern represents a user's access pattern
type UserAccessPattern struct {
	UserID             string            `json:"user_id"`
	PatternType        string            `json:"pattern_type"` // regular, irregular, seasonal, evolving
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"` // 0-100
	
	// Pattern characteristics
	TypicalAccessTimes []string          `json:"typical_access_times"`
	TypicalResources   []string          `json:"typical_resources"`
	AccessFrequency    float64           `json:"access_frequency"`
	SessionDuration    time.Duration     `json:"session_duration"`
	
	// Variations
	PatternVariations  []string          `json:"pattern_variations,omitempty"`
	SeasonalChanges    []string          `json:"seasonal_changes,omitempty"`
	
	// Risk assessment
	PatternRisk        float64           `json:"pattern_risk"` // 0-100
	AnomalyIndicators  []string          `json:"anomaly_indicators,omitempty"`
}

// ResourceAccessPattern represents a resource's access pattern
type ResourceAccessPattern struct {
	ResourceID         string            `json:"resource_id"`
	PatternType        string            `json:"pattern_type"` // high_traffic, periodic, regulated, sensitive
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"` // 0-100
	
	// Access characteristics
	TypicalUsers       []string          `json:"typical_users"`
	AccessVolume       float64           `json:"access_volume"`
	PeakUsageTimes     []string          `json:"peak_usage_times"`
	
	// Usage patterns
	UsageType          string            `json:"usage_type"` // regular, batch, interactive, automated
	AccessMethods      []string          `json:"access_methods"`
	
	// Security considerations
	SecurityLevel      string            `json:"security_level"`
	ThreatExposure     float64           `json:"threat_exposure"` // 0-100
	ProtectionLevel    float64           `json:"protection_level"` // 0-100
}

// TemporalAccessPattern represents temporal access patterns
type TemporalAccessPattern struct {
	PatternType        string            `json:"pattern_type"` // daily, weekly, monthly, seasonal
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"` // 0-100
	
	// Pattern data
	TimeWindows        []TimeWindow      `json:"time_windows"`
	RecurrenceRule     string            `json:"recurrence_rule,omitempty"`
	
	// Statistical data
	AverageVolume      float64           `json:"average_volume"`
	PeakVolume         float64           `json:"peak_volume"`
	MinVolume          float64           `json:"min_volume"`
	StandardDeviation  float64           `json:"standard_deviation"`
	
	// Business relevance
	BusinessAlignment  string            `json:"business_alignment,omitempty"`
	OperationalImpact  string            `json:"operational_impact,omitempty"`
}

// TimeWindow represents a time window for access patterns
type TimeWindow struct {
	Start              string            `json:"start"` // e.g., "09:00" or "Monday"
	End                string            `json:"end"`   // e.g., "17:00" or "Friday"
	Volume             float64           `json:"volume"`
	UserCount          int64             `json:"user_count"`
	ResourceCount      int64             `json:"resource_count"`
}

// GeographicAccessPattern represents geographic access patterns
type GeographicAccessPattern struct {
	PatternType        string            `json:"pattern_type"` // local, distributed, international, suspicious
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"` // 0-100
	
	// Geographic data
	Locations          []LocationAccess  `json:"locations"`
	PrimaryRegions     []string          `json:"primary_regions"`
	
	// Anomaly detection
	UnusualLocations   []string          `json:"unusual_locations,omitempty"`
	LocationAnomalities []LocationAnomaly `json:"location_anomalies,omitempty"`
	
	// Risk assessment
	GeographicRisk     float64           `json:"geographic_risk"` // 0-100
	ComplianceRisk     float64           `json:"compliance_risk"` // 0-100
}

// LocationAccess represents access from a specific location
type LocationAccess struct {
	Country            string            `json:"country"`
	Region             string            `json:"region,omitempty"`
	City               string            `json:"city,omitempty"`
	AccessCount        int64             `json:"access_count"`
	UserCount          int64             `json:"user_count"`
	FirstSeen          time.Time         `json:"first_seen"`
	LastSeen           time.Time         `json:"last_seen"`
	IsKnownLocation    bool              `json:"is_known_location"`
	RiskScore          float64           `json:"risk_score,omitempty"`
}

// LocationAnomaly represents a geographic anomaly
type LocationAnomaly struct {
	Location           string            `json:"location"`
	AnomalyType        string            `json:"anomaly_type"` // new_location, impossible_travel, high_risk_location
	UserID             string            `json:"user_id,omitempty"`
	DetectedAt         time.Time         `json:"detected_at"`
	PreviousLocation   string            `json:"previous_location,omitempty"`
	TravelTime         time.Duration     `json:"travel_time,omitempty"`
	IsPhysicallyPossible bool            `json:"is_physically_possible"`
	RiskScore          float64           `json:"risk_score"`
	Recommendation     string            `json:"recommendation,omitempty"`
}

// TechnologyAccessPattern represents technology-based access patterns
type TechnologyAccessPattern struct {
	PatternType        string            `json:"pattern_type"` // device, browser, os, application
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"` // 0-100
	
	// Technology data
	Technologies       []TechnologyUsage `json:"technologies"`
	CommonCombinations []string          `json:"common_combinations"`
	
	// Security analysis
	SecurityRisk       float64           `json:"security_risk"` // 0-100
	VulnerableVersions []string          `json:"vulnerable_versions,omitempty"`
	
	// Recommendations
	SecurityRecommendations []string     `json:"security_recommendations,omitempty"`
}

// TechnologyUsage represents usage of a specific technology
type TechnologyUsage struct {
	TechnologyType     string            `json:"technology_type"` // browser, os, device, application
	TechnologyName     string            `json:"technology_name"`
	Version            string            `json:"version,omitempty"`
	AccessCount        int64             `json:"access_count"`
	UserCount          int64             `json:"user_count"`
	FirstSeen          time.Time         `json:"first_seen"`
	LastSeen           time.Time         `json:"last_seen"`
	IsSupported        bool              `json:"is_supported"`
	IsSecure           bool              `json:"is_secure"`
	RiskScore          float64           `json:"risk_score,omitempty"`
}

// TrailCriteria defines criteria for audit trail construction
type TrailCriteria struct {
	// Entity identification
	EntityID           string            `json:"entity_id"`
	EntityType         string            `json:"entity_type"`
	
	// Time range
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	
	// Trail scope
	IncludeRelated     bool              `json:"include_related"` // Include related entities
	RelationshipDepth  int               `json:"relationship_depth,omitempty"`
	IncludeSystemEvents bool             `json:"include_system_events"`
	IncludeUserEvents  bool              `json:"include_user_events"`
	
	// Event filtering
	EventTypes         []EventType       `json:"event_types,omitempty"`
	EventFilter        EventFilter       `json:"event_filter,omitempty"`
	
	// Detail level
	DetailLevel        string            `json:"detail_level"` // summary, detailed, comprehensive
	IncludeMetadata    bool              `json:"include_metadata"`
	IncludeTimeline    bool              `json:"include_timeline"`
	IncludeCorrelations bool             `json:"include_correlations"`
	
	// Compliance focus
	ComplianceRelevant bool              `json:"compliance_relevant"`
	IncludeEvidence    bool              `json:"include_evidence"`
	
	// Performance options
	MaxEvents          int               `json:"max_events,omitempty"`
	UseCache           bool              `json:"use_cache"`
}

// AuditTrail represents a complete audit trail for an entity
type AuditTrail struct {
	// Trail metadata
	EntityID           string            `json:"entity_id"`
	EntityType         string            `json:"entity_type"`
	GeneratedAt        time.Time         `json:"generated_at"`
	Criteria           TrailCriteria     `json:"criteria"`
	
	// Trail summary
	Summary            *TrailSummary     `json:"summary"`
	
	// Events in chronological order
	Events             []*TrailEvent     `json:"events"`
	
	// Timeline visualization data
	Timeline           *TrailTimeline    `json:"timeline,omitempty"`
	
	// Relationships and correlations
	RelatedEntities    []*RelatedEntity  `json:"related_entities,omitempty"`
	Correlations       []*EventCorrelation `json:"correlations,omitempty"`
	
	// Key milestones and lifecycle events
	Milestones         []*TrailMilestone `json:"milestones,omitempty"`
	
	// Compliance and evidence
	ComplianceEvents   []*ComplianceEvidence `json:"compliance_events,omitempty"`
	
	// Analysis and insights
	TrailAnalysis      *TrailAnalysis    `json:"trail_analysis,omitempty"`
	
	// Report metadata
	EventCount         int64             `json:"event_count"`
	TrailTime          time.Duration     `json:"trail_time"`
	DataSources        []string          `json:"data_sources"`
}

// TrailSummary provides summary of the audit trail
type TrailSummary struct {
	// Basic metrics
	TotalEvents        int64             `json:"total_events"`
	EventTypes         map[EventType]int64 `json:"event_types"`
	TimeSpan           time.Duration     `json:"time_span"`
	
	// Lifecycle summary
	LifecycleStage     string            `json:"lifecycle_stage"`
	CreatedAt          time.Time         `json:"created_at"`
	LastModified       time.Time         `json:"last_modified"`
	CurrentStatus      string            `json:"current_status"`
	
	// Activity summary
	ActivityPeriods    []ActivityPeriod  `json:"activity_periods,omitempty"`
	QuietPeriods       []QuietPeriod     `json:"quiet_periods,omitempty"`
	
	// Key statistics
	UniqueActors       int64             `json:"unique_actors"`
	SystemEvents       int64             `json:"system_events"`
	UserEvents         int64             `json:"user_events"`
	ErrorEvents        int64             `json:"error_events"`
	
	// Compliance summary
	ComplianceRelevant int64             `json:"compliance_relevant"`
	ViolationEvents    int64             `json:"violation_events"`
	
	// Risk indicators
	RiskEvents         int64             `json:"risk_events"`
	SecurityEvents     int64             `json:"security_events"`
	AnomalyEvents      int64             `json:"anomaly_events"`
}

// TrailEvent represents a single event in the audit trail
type TrailEvent struct {
	// Event identification
	EventID            uuid.UUID         `json:"event_id"`
	SequenceNumber     int64             `json:"sequence_number"`
	Timestamp          time.Time         `json:"timestamp"`
	
	// Event details
	EventType          EventType         `json:"event_type"`
	Action             string            `json:"action"`
	Result             string            `json:"result"`
	Description        string            `json:"description,omitempty"`
	
	// Actor information
	ActorID            string            `json:"actor_id"`
	ActorType          string            `json:"actor_type"`
	ActorName          string            `json:"actor_name,omitempty"`
	
	// Context
	Context            map[string]interface{} `json:"context,omitempty"`
	RequestID          string            `json:"request_id,omitempty"`
	SessionID          string            `json:"session_id,omitempty"`
	
	// Trail relevance
	Relevance          string            `json:"relevance"` // direct, indirect, related
	Importance         string            `json:"importance"` // low, medium, high, critical
	
	// Change details (for modification events)
	Changes            []*FieldChange    `json:"changes,omitempty"`
	
	// Compliance and security
	IsComplianceRelevant bool            `json:"is_compliance_relevant"`
	IsSecurityRelevant   bool            `json:"is_security_relevant"`
	ComplianceFlags      []string        `json:"compliance_flags,omitempty"`
	
	// Evidence and verification
	Evidence           []string          `json:"evidence,omitempty"`
	DigitalSignature   string            `json:"digital_signature,omitempty"`
	IsVerified         bool              `json:"is_verified"`
}

// FieldChange represents a change to a specific field
type FieldChange struct {
	FieldName          string            `json:"field_name"`
	OldValue           interface{}       `json:"old_value"`
	NewValue           interface{}       `json:"new_value"`
	ChangeType         string            `json:"change_type"` // create, update, delete
	ChangeReason       string            `json:"change_reason,omitempty"`
}

// ActivityPeriod represents a period of high activity
type ActivityPeriod struct {
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	EventCount         int64             `json:"event_count"`
	ActivityType       string            `json:"activity_type"`
	Description        string            `json:"description,omitempty"`
}

// QuietPeriod represents a period of low or no activity
type QuietPeriod struct {
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	Duration           time.Duration     `json:"duration"`
	Reason             string            `json:"reason,omitempty"`
}

// TrailTimeline provides timeline visualization data
type TrailTimeline struct {
	// Timeline configuration
	TimeGranularity    string            `json:"time_granularity"` // minute, hour, day
	
	// Timeline data points
	TimePoints         []*TimelinePoint  `json:"time_points"`
	
	// Key events on timeline
	KeyEvents          []*KeyTimelineEvent `json:"key_events"`
	
	// Timeline annotations
	Annotations        []*TimelineAnnotation `json:"annotations,omitempty"`
	
	// Visual configuration
	TimelineConfig     *TimelineConfig   `json:"timeline_config,omitempty"`
}

// TimelinePoint represents a point on the timeline
type TimelinePoint struct {
	Timestamp          time.Time         `json:"timestamp"`
	EventCount         int64             `json:"event_count"`
	EventTypes         map[EventType]int64 `json:"event_types"`
	ActivityLevel      string            `json:"activity_level"` // low, medium, high
	HasAnomalies       bool              `json:"has_anomalies"`
	HasViolations      bool              `json:"has_violations"`
}

// KeyTimelineEvent represents a significant event on the timeline
type KeyTimelineEvent struct {
	EventID            uuid.UUID         `json:"event_id"`
	Timestamp          time.Time         `json:"timestamp"`
	EventType          EventType         `json:"event_type"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Importance         string            `json:"importance"` // low, medium, high, critical
	Icon               string            `json:"icon,omitempty"`
	Color              string            `json:"color,omitempty"`
}

// TimelineAnnotation represents an annotation on the timeline
type TimelineAnnotation struct {
	StartTime          time.Time         `json:"start_time"`
	EndTime            *time.Time        `json:"end_time,omitempty"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	AnnotationType     string            `json:"annotation_type"` // milestone, period, alert, note
	Color              string            `json:"color,omitempty"`
}

// TimelineConfig provides configuration for timeline visualization
type TimelineConfig struct {
	ShowEventTypes     []EventType       `json:"show_event_types,omitempty"`
	ColorScheme        string            `json:"color_scheme,omitempty"`
	ShowAnnotations    bool              `json:"show_annotations"`
	ShowKeyEvents      bool              `json:"show_key_events"`
	InteractiveMode    bool              `json:"interactive_mode"`
}

// RelatedEntity represents an entity related to the main entity
type RelatedEntity struct {
	EntityID           string            `json:"entity_id"`
	EntityType         string            `json:"entity_type"`
	EntityName         string            `json:"entity_name,omitempty"`
	RelationshipType   string            `json:"relationship_type"`
	RelationshipStrength float64         `json:"relationship_strength"` // 0-100
	EventCount         int64             `json:"event_count"`
	FirstInteraction   time.Time         `json:"first_interaction"`
	LastInteraction    time.Time         `json:"last_interaction"`
	IsActive           bool              `json:"is_active"`
	Relevance          string            `json:"relevance"` // high, medium, low
}

// TrailMilestone represents a significant milestone in the entity's lifecycle
type TrailMilestone struct {
	MilestoneType      string            `json:"milestone_type"` // created, activated, modified, archived
	Timestamp          time.Time         `json:"timestamp"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	EventID            uuid.UUID         `json:"event_id,omitempty"`
	ActorID            string            `json:"actor_id,omitempty"`
	Impact             string            `json:"impact,omitempty"`
	BusinessSignificance string          `json:"business_significance,omitempty"`
}

// ComplianceEvidence represents compliance-relevant evidence in the trail
type ComplianceEvidence struct {
	EvidenceType       string            `json:"evidence_type"` // consent, disclosure, deletion, breach
	EventID            uuid.UUID         `json:"event_id"`
	Timestamp          time.Time         `json:"timestamp"`
	Description        string            `json:"description"`
	ComplianceFramework string           `json:"compliance_framework"`
	RequirementMet     string            `json:"requirement_met"`
	EvidenceQuality    string            `json:"evidence_quality"` // strong, moderate, weak
	DigitalSignature   string            `json:"digital_signature,omitempty"`
	ChainOfCustody     []string          `json:"chain_of_custody,omitempty"`
	VerificationStatus string            `json:"verification_status"` // verified, pending, failed
}

// TrailAnalysis provides analysis and insights about the audit trail
type TrailAnalysis struct {
	// Pattern analysis
	Patterns           []*TrailPattern   `json:"patterns,omitempty"`
	
	// Lifecycle analysis
	LifecycleAnalysis  *LifecycleAnalysis `json:"lifecycle_analysis,omitempty"`
	
	// Behavioral analysis
	BehaviorAnalysis   *BehaviorAnalysis  `json:"behavior_analysis,omitempty"`
	
	// Risk analysis
	RiskAnalysis       *TrailRiskAnalysis `json:"risk_analysis,omitempty"`
	
	// Compliance analysis
	ComplianceAnalysis *TrailComplianceAnalysis `json:"compliance_analysis,omitempty"`
	
	// Quality analysis
	QualityAnalysis    *TrailQualityAnalysis `json:"quality_analysis,omitempty"`
	
	// Insights and recommendations
	KeyInsights        []string          `json:"key_insights,omitempty"`
	Recommendations    []string          `json:"recommendations,omitempty"`
	RedFlags           []string          `json:"red_flags,omitempty"`
}

// TrailPattern represents a pattern detected in the audit trail
type TrailPattern struct {
	PatternType        string            `json:"pattern_type"` // cyclical, linear, clustered, random
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"` // 0-100
	Frequency          float64           `json:"frequency,omitempty"`
	Regularity         float64           `json:"regularity,omitempty"` // 0-100
	Predictability     float64           `json:"predictability,omitempty"` // 0-100
	BusinessRelevance  string            `json:"business_relevance,omitempty"`
	AnomalyIndicator   bool              `json:"anomaly_indicator"`
}

// LifecycleAnalysis provides analysis of the entity's lifecycle
type LifecycleAnalysis struct {
	CurrentStage       string            `json:"current_stage"`
	StageHistory       []LifecycleStage  `json:"stage_history"`
	AverageStageTime   map[string]time.Duration `json:"average_stage_time,omitempty"`
	StageTransitions   []StageTransition `json:"stage_transitions,omitempty"`
	LifecycleHealth    string            `json:"lifecycle_health"` // healthy, concerning, problematic
	ExpectedNextStage  string            `json:"expected_next_stage,omitempty"`
	LifecycleCompleteness float64        `json:"lifecycle_completeness"` // 0-100
}

// LifecycleStage represents a stage in the entity's lifecycle
type LifecycleStage struct {
	StageName          string            `json:"stage_name"`
	StartTime          time.Time         `json:"start_time"`
	EndTime            *time.Time        `json:"end_time,omitempty"`
	Duration           time.Duration     `json:"duration,omitempty"`
	TriggerEvent       uuid.UUID         `json:"trigger_event,omitempty"`
	IsComplete         bool              `json:"is_complete"`
	StageHealth        string            `json:"stage_health,omitempty"`
}

// StageTransition represents a transition between lifecycle stages
type StageTransition struct {
	FromStage          string            `json:"from_stage"`
	ToStage            string            `json:"to_stage"`
	TransitionTime     time.Time         `json:"transition_time"`
	TriggerEvent       uuid.UUID         `json:"trigger_event"`
	TransitionReason   string            `json:"transition_reason,omitempty"`
	IsNormalTransition bool              `json:"is_normal_transition"`
}

// BehaviorAnalysis provides behavioral analysis of actors interacting with the entity
type BehaviorAnalysis struct {
	// Actor behavior patterns
	ActorBehaviors     []*ActorBehaviorPattern `json:"actor_behaviors,omitempty"`
	
	// Interaction patterns
	InteractionPatterns []*InteractionPattern   `json:"interaction_patterns,omitempty"`
	
	// Behavioral anomalies
	BehavioralAnomalies []*BehavioralAnomaly   `json:"behavioral_anomalies,omitempty"`
	
	// Behavioral trends
	BehaviorTrends     *BehaviorTrends        `json:"behavior_trends,omitempty"`
	
	// Risk indicators
	BehaviorRiskScore  float64                `json:"behavior_risk_score"` // 0-100
	RiskIndicators     []string               `json:"risk_indicators,omitempty"`
}

// ActorBehaviorPattern represents a behavioral pattern for a specific actor
type ActorBehaviorPattern struct {
	ActorID            string            `json:"actor_id"`
	PatternType        string            `json:"pattern_type"` // regular, irregular, suspicious, automated
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"` // 0-100
	Frequency          float64           `json:"frequency"`
	Consistency        float64           `json:"consistency"` // 0-100
	TypicalActions     []string          `json:"typical_actions"`
	TypicalTiming      []string          `json:"typical_timing,omitempty"`
	Deviations         []string          `json:"deviations,omitempty"`
	RiskLevel          string            `json:"risk_level"`
}

// InteractionPattern represents patterns of interaction between actors and the entity
type InteractionPattern struct {
	PatternType        string            `json:"pattern_type"` // collaborative, sequential, competitive, independent
	Description        string            `json:"description"`
	Participants       []string          `json:"participants"`
	Frequency          float64           `json:"frequency"`
	Duration           time.Duration     `json:"duration,omitempty"`
	Complexity         string            `json:"complexity"` // simple, moderate, complex
	BusinessValue      string            `json:"business_value,omitempty"`
	EfficiencyScore    float64           `json:"efficiency_score,omitempty"` // 0-100
}

// BehavioralAnomaly represents an anomalous behavior pattern
type BehavioralAnomaly struct {
	AnomalyType        string            `json:"anomaly_type"` // timing, frequency, sequence, permission
	ActorID            string            `json:"actor_id,omitempty"`
	Description        string            `json:"description"`
	DetectedAt         time.Time         `json:"detected_at"`
	Severity           string            `json:"severity"`
	Confidence         float64           `json:"confidence"` // 0-100
	ExpectedBehavior   string            `json:"expected_behavior"`
	ObservedBehavior   string            `json:"observed_behavior"`
	DeviationScore     float64           `json:"deviation_score"` // 0-100
	PotentialCauses    []string          `json:"potential_causes,omitempty"`
	RequiresAttention  bool              `json:"requires_attention"`
}

// BehaviorTrends represents trends in behavior over time
type BehaviorTrends struct {
	OverallTrend       string            `json:"overall_trend"` // improving, declining, stable, volatile
	ActivityTrend      string            `json:"activity_trend"`
	ComplexityTrend    string            `json:"complexity_trend"`
	RiskTrend          string            `json:"risk_trend"`
	EfficiencyTrend    string            `json:"efficiency_trend"`
	TrendConfidence    float64           `json:"trend_confidence"` // 0-100
	PredictedChanges   []string          `json:"predicted_changes,omitempty"`
}

// TrailRiskAnalysis provides risk analysis for the audit trail
type TrailRiskAnalysis struct {
	OverallRiskScore   float64           `json:"overall_risk_score"` // 0-100
	RiskLevel          string            `json:"risk_level"` // low, medium, high, critical
	
	// Risk categories
	SecurityRisks      []*TrailSecurityRisk `json:"security_risks,omitempty"`
	ComplianceRisks    []*TrailComplianceRisk `json:"compliance_risks,omitempty"`
	OperationalRisks   []*TrailOperationalRisk `json:"operational_risks,omitempty"`
	DataRisks          []*TrailDataRisk       `json:"data_risks,omitempty"`
	
	// Risk evolution
	RiskTrend          string            `json:"risk_trend"` // increasing, decreasing, stable
	RiskVelocity       float64           `json:"risk_velocity"` // rate of change
	
	// Risk mitigation
	RiskMitigation     *TrailRiskMitigation `json:"risk_mitigation,omitempty"`
	
	// Risk prediction
	PredictedRisks     []string          `json:"predicted_risks,omitempty"`
	RiskForecast       *RiskForecast     `json:"risk_forecast,omitempty"`
}

// TrailSecurityRisk represents a security risk identified in the trail
type TrailSecurityRisk struct {
	RiskType           string            `json:"risk_type"` // unauthorized_access, data_breach, privilege_escalation
	Description        string            `json:"description"`
	RiskScore          float64           `json:"risk_score"` // 0-100
	Likelihood         string            `json:"likelihood"`
	Impact             string            `json:"impact"`
	EvidenceEvents     []uuid.UUID       `json:"evidence_events"`
	FirstDetected      time.Time         `json:"first_detected"`
	LastOccurrence     time.Time         `json:"last_occurrence"`
	IsActive           bool              `json:"is_active"`
	MitigationStatus   string            `json:"mitigation_status"`
	Recommendations    []string          `json:"recommendations,omitempty"`
}

// TrailComplianceRisk represents a compliance risk identified in the trail
type TrailComplianceRisk struct {
	RiskType           string            `json:"risk_type"` // policy_violation, regulatory_breach, audit_failure
	ComplianceFramework string           `json:"compliance_framework"`
	RequirementViolated string           `json:"requirement_violated"`
	RiskScore          float64           `json:"risk_score"` // 0-100
	ViolationSeverity  string            `json:"violation_severity"`
	EvidenceEvents     []uuid.UUID       `json:"evidence_events"`
	PotentialPenalty   string            `json:"potential_penalty,omitempty"`
	RemediationRequired bool             `json:"remediation_required"`
	RemediationPlan    string            `json:"remediation_plan,omitempty"`
}

// TrailOperationalRisk represents an operational risk identified in the trail
type TrailOperationalRisk struct {
	RiskType           string            `json:"risk_type"` // process_failure, system_downtime, human_error
	Description        string            `json:"description"`
	RiskScore          float64           `json:"risk_score"` // 0-100
	BusinessImpact     string            `json:"business_impact"`
	OperationalImpact  string            `json:"operational_impact"`
	EvidenceEvents     []uuid.UUID       `json:"evidence_events"`
	Frequency          float64           `json:"frequency"`
	AffectedProcesses  []string          `json:"affected_processes,omitempty"`
	PreventiveMeasures []string          `json:"preventive_measures,omitempty"`
}

// TrailDataRisk represents a data-related risk identified in the trail
type TrailDataRisk struct {
	RiskType           string            `json:"risk_type"` // data_loss, data_corruption, unauthorized_disclosure
	DataCategory       string            `json:"data_category"`
	DataSensitivity    string            `json:"data_sensitivity"`
	RiskScore          float64           `json:"risk_score"` // 0-100
	ExposureLevel      string            `json:"exposure_level"`
	AffectedRecords    int64             `json:"affected_records,omitempty"`
	EvidenceEvents     []uuid.UUID       `json:"evidence_events"`
	DataProtectionStatus string          `json:"data_protection_status"`
	RequiredActions    []string          `json:"required_actions,omitempty"`
}

// TrailRiskMitigation represents risk mitigation measures
type TrailRiskMitigation struct {
	ImplementedMeasures []string         `json:"implemented_measures"`
	PlannedMeasures     []string         `json:"planned_measures"`
	MitigationCoverage  float64          `json:"mitigation_coverage"` // 0-100
	ResidualRisk        float64          `json:"residual_risk"` // 0-100
	MitigationEffectiveness float64      `json:"mitigation_effectiveness"` // 0-100
	LastReview          time.Time        `json:"last_review"`
	NextReview          time.Time        `json:"next_review"`
}

// RiskForecast represents a forecast of future risks
type RiskForecast struct {
	ForecastHorizon    time.Duration     `json:"forecast_horizon"`
	PredictedRiskLevel string            `json:"predicted_risk_level"`
	RiskProbability    float64           `json:"risk_probability"` // 0-100
	KeyRiskFactors     []string          `json:"key_risk_factors"`
	TriggerEvents      []string          `json:"trigger_events,omitempty"`
	PreventiveActions  []string          `json:"preventive_actions,omitempty"`
	MonitoringKPIs     []string          `json:"monitoring_kpis,omitempty"`
}

// TrailComplianceAnalysis provides compliance analysis for the audit trail
type TrailComplianceAnalysis struct {
	OverallCompliance  float64           `json:"overall_compliance"` // 0-100
	ComplianceStatus   string            `json:"compliance_status"` // compliant, non_compliant, partial
	
	// Framework-specific compliance
	FrameworkCompliance map[string]*FrameworkComplianceStatus `json:"framework_compliance,omitempty"`
	
	// Compliance events
	ComplianceEvents   []*TrailComplianceEvent `json:"compliance_events,omitempty"`
	
	// Violations
	Violations         []*TrailComplianceViolation `json:"violations,omitempty"`
	
	// Evidence and documentation
	Evidence           []*ComplianceEvidence     `json:"evidence,omitempty"`
	DocumentationGaps  []string                  `json:"documentation_gaps,omitempty"`
	
	// Compliance trends
	ComplianceTrend    string            `json:"compliance_trend"` // improving, declining, stable
	
	// Recommendations
	ComplianceRecommendations []string   `json:"compliance_recommendations,omitempty"`
}

// FrameworkComplianceStatus represents compliance status for a specific framework
type FrameworkComplianceStatus struct {
	Framework          string            `json:"framework"`
	ComplianceScore    float64           `json:"compliance_score"` // 0-100
	Status             string            `json:"status"` // compliant, non_compliant, partial
	RequirementsMet    int               `json:"requirements_met"`
	TotalRequirements  int               `json:"total_requirements"`
	CriticalGaps       []string          `json:"critical_gaps,omitempty"`
	LastAssessment     time.Time         `json:"last_assessment"`
	NextAssessment     time.Time         `json:"next_assessment"`
	CertificationStatus string           `json:"certification_status,omitempty"`
}

// TrailComplianceEvent represents a compliance-relevant event in the trail
type TrailComplianceEvent struct {
	EventID            uuid.UUID         `json:"event_id"`
	Timestamp          time.Time         `json:"timestamp"`
	ComplianceType     string            `json:"compliance_type"` // consent, disclosure, access, deletion
	Framework          string            `json:"framework"`
	Requirement        string            `json:"requirement"`
	ComplianceResult   string            `json:"compliance_result"` // met, not_met, partial
	Evidence           []string          `json:"evidence,omitempty"`
	VerificationStatus string            `json:"verification_status"`
	Notes              string            `json:"notes,omitempty"`
}

// TrailComplianceViolation represents a compliance violation identified in the trail
type TrailComplianceViolation struct {
	ViolationID        string            `json:"violation_id"`
	ViolationType      string            `json:"violation_type"`
	Framework          string            `json:"framework"`
	Requirement        string            `json:"requirement"`
	Severity           string            `json:"severity"`
	Description        string            `json:"description"`
	EvidenceEvents     []uuid.UUID       `json:"evidence_events"`
	DetectedAt         time.Time         `json:"detected_at"`
	Status             string            `json:"status"` // open, investigating, resolved
	Impact             string            `json:"impact"`
	RemediationPlan    string            `json:"remediation_plan,omitempty"`
	DueDate            *time.Time        `json:"due_date,omitempty"`
	ResponsibleParty   string            `json:"responsible_party,omitempty"`
}

// TrailQualityAnalysis provides quality analysis for the audit trail
type TrailQualityAnalysis struct {
	OverallQuality     float64           `json:"overall_quality"` // 0-100
	QualityLevel       string            `json:"quality_level"` // poor, fair, good, excellent
	
	// Quality dimensions
	Completeness       float64           `json:"completeness"` // 0-100
	Accuracy           float64           `json:"accuracy"` // 0-100
	Consistency        float64           `json:"consistency"` // 0-100
	Timeliness         float64           `json:"timeliness"` // 0-100
	Integrity          float64           `json:"integrity"` // 0-100
	
	// Quality issues
	QualityIssues      []*TrailQualityIssue `json:"quality_issues,omitempty"`
	
	// Data quality
	DataQuality        *TrailDataQuality    `json:"data_quality,omitempty"`
	
	// Coverage analysis
	CoverageAnalysis   *TrailCoverageAnalysis `json:"coverage_analysis,omitempty"`
	
	// Recommendations
	QualityRecommendations []string         `json:"quality_recommendations,omitempty"`
}

// TrailQualityIssue represents a quality issue in the audit trail
type TrailQualityIssue struct {
	IssueType          string            `json:"issue_type"` // missing_events, duplicate_events, inconsistent_data
	Severity           string            `json:"severity"`
	Description        string            `json:"description"`
	AffectedEvents     []uuid.UUID       `json:"affected_events,omitempty"`
	DetectedAt         time.Time         `json:"detected_at"`
	Impact             string            `json:"impact"`
	ResolutionStatus   string            `json:"resolution_status"`
	RecommendedAction  string            `json:"recommended_action"`
}

// TrailDataQuality provides data quality analysis for the trail
type TrailDataQuality struct {
	DataCompleteness   float64           `json:"data_completeness"` // 0-100
	DataAccuracy       float64           `json:"data_accuracy"` // 0-100
	DataConsistency    float64           `json:"data_consistency"` // 0-100
	DataValidation     float64           `json:"data_validation"` // 0-100
	
	// Field-level quality
	FieldQuality       map[string]float64 `json:"field_quality,omitempty"`
	
	// Quality metrics
	MissingDataPoints  int64             `json:"missing_data_points"`
	InvalidDataPoints  int64             `json:"invalid_data_points"`
	InconsistentData   int64             `json:"inconsistent_data"`
	
	// Quality trends
	QualityTrend       string            `json:"quality_trend"` // improving, declining, stable
}

// TrailCoverageAnalysis provides coverage analysis for the audit trail
type TrailCoverageAnalysis struct {
	OverallCoverage    float64           `json:"overall_coverage"` // 0-100
	
	// Coverage dimensions
	TemporalCoverage   float64           `json:"temporal_coverage"` // 0-100
	FunctionalCoverage float64           `json:"functional_coverage"` // 0-100
	ActorCoverage      float64           `json:"actor_coverage"` // 0-100
	
	// Coverage gaps
	CoverageGaps       []CoverageGap     `json:"coverage_gaps,omitempty"`
	
	// Expected vs actual
	ExpectedEvents     int64             `json:"expected_events"`
	ActualEvents       int64             `json:"actual_events"`
	CoverageRatio      float64           `json:"coverage_ratio"`
	
	// Missing elements
	MissingPeriods     []TimeRange       `json:"missing_periods,omitempty"`
	MissingFunctions   []string          `json:"missing_functions,omitempty"`
	MissingActors      []string          `json:"missing_actors,omitempty"`
}

// CoverageGap represents a gap in audit trail coverage
type CoverageGap struct {
	GapType            string            `json:"gap_type"` // temporal, functional, actor
	Description        string            `json:"description"`
	StartTime          time.Time         `json:"start_time,omitempty"`
	EndTime            time.Time         `json:"end_time,omitempty"`
	MissingElements    []string          `json:"missing_elements"`
	Impact             string            `json:"impact"`
	PossibleCause      string            `json:"possible_cause,omitempty"`
	Criticality        string            `json:"criticality"` // low, medium, high, critical
}

// Missing types for query_repository.go
type ActivityCriteria struct {
	UserIDs         []string    `json:"user_ids,omitempty"`
	StartTime       time.Time   `json:"start_time"`
	EndTime         time.Time   `json:"end_time"`
	EventTypes      []EventType `json:"event_types,omitempty"`
	IncludeFailures bool        `json:"include_failures"`
}

type UserActivityReport struct {
	UserID       string              `json:"user_id"`
	Activities   []*UserActivitySummary  `json:"activities"`
	TotalActions int64               `json:"total_actions"`
	GeneratedAt  time.Time           `json:"generated_at"`
}

type UserActivitySummary struct {
	EventType   EventType `json:"event_type"`
	Count       int64     `json:"count"`
	LastOccurrence time.Time `json:"last_occurrence"`
}

type AnomalyCriteria struct {
	TimeRange    TimeRange   `json:"time_range"`
	AnomalyTypes []string    `json:"anomaly_types,omitempty"`
	Severity     string      `json:"severity,omitempty"`
}

type AnomalyReport struct {
	Anomalies    []*DetectedAnomaly `json:"anomalies"`
	Summary      *AnomalySummary    `json:"summary"`
	GeneratedAt  time.Time          `json:"generated_at"`
}

type DetectedAnomaly struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Description  string    `json:"description"`
	Severity     string    `json:"severity"`
	DetectedAt   time.Time `json:"detected_at"`
	EventID      uuid.UUID `json:"event_id,omitempty"`
}

type AnomalySummary struct {
	TotalAnomalies int                `json:"total_anomalies"`
	BySeverity     map[string]int     `json:"by_severity"`
	ByType         map[string]int     `json:"by_type"`
}

type SecurityEventCriteria struct {
	TimeRange      TimeRange `json:"time_range"`
	SecurityLevels []string  `json:"security_levels,omitempty"`
	ThreatTypes    []string  `json:"threat_types,omitempty"`
}

type SecurityEventReport struct {
	Events       []*SecurityEventSummary `json:"events"`
	ThreatAnalysis *ThreatAnalysisSummary `json:"threat_analysis"`
	GeneratedAt  time.Time               `json:"generated_at"`
}

type SecurityEventSummary struct {
	EventID     uuid.UUID `json:"event_id"`
	EventType   EventType `json:"event_type"`
	ThreatLevel string    `json:"threat_level"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

type ThreatAnalysisSummary struct {
	TotalThreats   int                `json:"total_threats"`
	ByThreatLevel  map[string]int     `json:"by_threat_level"`
	TopThreats     []string           `json:"top_threats"`
}

type FailureCriteria struct {
	TimeRange     TimeRange   `json:"time_range"`
	FailureTypes  []string    `json:"failure_types,omitempty"`
	SystemAreas   []string    `json:"system_areas,omitempty"`
}

type FailureAnalysis struct {
	Failures      []*FailureEvent    `json:"failures"`
	Patterns      []*FailurePattern  `json:"patterns"`
	Recommendations []string         `json:"recommendations"`
	GeneratedAt   time.Time          `json:"generated_at"`
}

type FailureEvent struct {
	EventID     uuid.UUID `json:"event_id"`
	FailureType string    `json:"failure_type"`
	SystemArea  string    `json:"system_area"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	Impact      string    `json:"impact"`
}

type FailurePattern struct {
	PatternType string    `json:"pattern_type"`
	Frequency   float64   `json:"frequency"`
	Description string    `json:"description"`
	RootCause   string    `json:"root_cause,omitempty"`
}

type ReportSchedule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	ReportType  string        `json:"report_type"`
	CronSchedule string       `json:"cron_schedule"`
	Recipients  []string      `json:"recipients"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	IsActive    bool          `json:"is_active"`
	CreatedAt   time.Time     `json:"created_at"`
	NextRun     time.Time     `json:"next_run"`
}

type QueryPerformanceMetrics struct {
	QueryID         string        `json:"query_id"`
	ExecutionTime   time.Duration `json:"execution_time"`
	RowsScanned     int64         `json:"rows_scanned"`
	RowsReturned    int64         `json:"rows_returned"`
	CacheHitRate    float64       `json:"cache_hit_rate"`
	IndexUsage      []string      `json:"index_usage"`
	Timestamp       time.Time     `json:"timestamp"`
}

type QueryOptimization struct {
	QueryID         string   `json:"query_id"`
	Recommendations []string `json:"recommendations"`
	EstimatedImprovement float64 `json:"estimated_improvement"`
	OptimizedQuery  string   `json:"optimized_query,omitempty"`
	IndexSuggestions []string `json:"index_suggestions,omitempty"`
}

// Missing types for archive_repository.go
type ArchiveValidationResult struct {
	ArchiveID      string    `json:"archive_id"`
	IsValid        bool      `json:"is_valid"`
	ValidationTime time.Time `json:"validation_time"`
	Issues         []string  `json:"issues,omitempty"`
	ChecksPerformed []string `json:"checks_performed"`
}

type CompactionResult struct {
	ArchiveID        string    `json:"archive_id"`
	OriginalSize     int64     `json:"original_size"`
	CompactedSize    int64     `json:"compacted_size"`
	CompressionRatio float64   `json:"compression_ratio"`
	CompactionTime   time.Time `json:"compaction_time"`
	EventsProcessed  int64     `json:"events_processed"`
}

type MigrationResult struct {
	SourceArchiveID string    `json:"source_archive_id"`
	TargetArchiveID string    `json:"target_archive_id"`
	EventsMigrated  int64     `json:"events_migrated"`
	MigrationTime   time.Time `json:"migration_time"`
	Status          string    `json:"status"`
	Issues          []string  `json:"issues,omitempty"`
}

type ArchiveStorageMetrics struct {
	TotalSize       int64     `json:"total_size"`
	AvailableSpace  int64     `json:"available_space"`
	UsagePercent    float64   `json:"usage_percent"`
	ArchiveCount    int       `json:"archive_count"`
	LastUpdated     time.Time `json:"last_updated"`
}

type ArchiveComplianceReport struct {
	ArchiveID       string               `json:"archive_id"`
	ComplianceScore float64              `json:"compliance_score"`
	Violations      []string             `json:"violations,omitempty"`
	RetentionStatus *RetentionStatus     `json:"retention_status"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

type RetentionStatus struct {
	RetentionPeriod    time.Duration `json:"retention_period"`
	CurrentAge         time.Duration `json:"current_age"`
	EligibleForDeletion bool         `json:"eligible_for_deletion"`
	LegalHolds         []string      `json:"legal_holds,omitempty"`
}

type ComplianceVerificationResult struct {
	ArchiveID      string    `json:"archive_id"`
	IsCompliant    bool      `json:"is_compliant"`
	Framework      string    `json:"framework"`
	VerifiedAt     time.Time `json:"verified_at"`
	Issues         []string  `json:"issues,omitempty"`
	Score          float64   `json:"score"`
}

// Missing types for integrity_repository.go - will add all of them
type ChainContinuityResult struct {
	IsIntact           bool      `json:"is_intact"`
	GapsDetected       int       `json:"gaps_detected"`
	FirstGap           *SequenceGap `json:"first_gap,omitempty"`
	LastValidEvent     uuid.UUID `json:"last_valid_event,omitempty"`
	VerificationTime   time.Time `json:"verification_time"`
}

type EventHashVerificationResult struct {
	EventID        uuid.UUID `json:"event_id"`
	IsValid        bool      `json:"is_valid"`
	ExpectedHash   string    `json:"expected_hash"`
	ActualHash     string    `json:"actual_hash"`
	VerifiedAt     time.Time `json:"verified_at"`
}

type BatchHashVerificationResult struct {
	BatchID        string    `json:"batch_id"`
	TotalEvents    int64     `json:"total_events"`
	ValidEvents    int64     `json:"valid_events"`
	InvalidEvents  int64     `json:"invalid_events"`
	VerifiedAt     time.Time `json:"verified_at"`
	FailedEvents   []uuid.UUID `json:"failed_events,omitempty"`
}

type HashRecomputeResult struct {
	EventID        uuid.UUID `json:"event_id"`
	OldHash        string    `json:"old_hash"`
	NewHash        string    `json:"new_hash"`
	RecomputedAt   time.Time `json:"recomputed_at"`
	IsChanged      bool      `json:"is_changed"`
}

type EventIntegrityResult struct {
	EventID         uuid.UUID `json:"event_id"`
	IntegrityScore  float64   `json:"integrity_score"`
	Issues          []string  `json:"issues,omitempty"`
	VerifiedAt      time.Time `json:"verified_at"`
	IsIntact        bool      `json:"is_intact"`
}

type SequenceGapReport struct {
	Gaps            []*SequenceGap `json:"gaps"`
	TotalGaps       int            `json:"total_gaps"`
	TotalMissing    int64          `json:"total_missing"`
	GeneratedAt     time.Time      `json:"generated_at"`
}

type DuplicateSequenceReport struct {
	Duplicates      []*DuplicateEvent `json:"duplicates"`
	TotalDuplicates int               `json:"total_duplicates"`
	GeneratedAt     time.Time         `json:"generated_at"`
}

type SequenceOrderCriteria struct {
	StartSequence   *values.SequenceNumber `json:"start_sequence,omitempty"`
	EndSequence     *values.SequenceNumber `json:"end_sequence,omitempty"`
	StrictOrder     bool                   `json:"strict_order"`
}

type SequenceOrderResult struct {
	IsOrdered       bool      `json:"is_ordered"`
	OutOfOrder      int       `json:"out_of_order"`
	FirstViolation  *values.SequenceNumber `json:"first_violation,omitempty"`
	VerifiedAt      time.Time `json:"verified_at"`
}

type CorruptionAnalysis struct {
	CorruptionType  string    `json:"corruption_type"`
	AffectedEvents  int64     `json:"affected_events"`
	Severity        string    `json:"severity"`
	FirstDetected   time.Time `json:"first_detected"`
	PossibleCause   string    `json:"possible_cause,omitempty"`
	Recommendations []string  `json:"recommendations,omitempty"`
}

type CorruptionHistoryFilter struct {
	TimeRange       TimeRange `json:"time_range"`
	CorruptionTypes []string  `json:"corruption_types,omitempty"`
	Severity        string    `json:"severity,omitempty"`
}

type CorruptionHistory struct {
	Events       []*CorruptionEvent `json:"events"`
	Summary      *CorruptionSummary `json:"summary"`
	GeneratedAt  time.Time          `json:"generated_at"`
}

type CorruptionEvent struct {
	EventID        uuid.UUID `json:"event_id"`
	CorruptionType string    `json:"corruption_type"`
	DetectedAt     time.Time `json:"detected_at"`
	Severity       string    `json:"severity"`
	Description    string    `json:"description"`
}

type CorruptionSummary struct {
	TotalEvents    int                `json:"total_events"`
	BySeverity     map[string]int     `json:"by_severity"`
	ByType         map[string]int     `json:"by_type"`
	TrendDirection string             `json:"trend_direction"`
}

type IntegrityMonitoringStatus struct {
	IsEnabled       bool      `json:"is_enabled"`
	LastCheck       time.Time `json:"last_check"`
	NextCheck       time.Time `json:"next_check"`
	CheckInterval   time.Duration `json:"check_interval"`
	HealthStatus    string    `json:"health_status"`
	ActiveMonitors  []string  `json:"active_monitors"`
}

type IntegrityAlertFilter struct {
	TimeRange    TimeRange `json:"time_range"`
	Severity     string    `json:"severity,omitempty"`
	AlertTypes   []string  `json:"alert_types,omitempty"`
	IsResolved   *bool     `json:"is_resolved,omitempty"`
}

type IntegrityAlerts struct {
	Alerts       []*IntegrityAlert  `json:"alerts"`
	Summary      *AlertSummary      `json:"summary"`
	GeneratedAt  time.Time          `json:"generated_at"`
}

type IntegrityAlert struct {
	AlertID      string    `json:"alert_id"`
	AlertType    string    `json:"alert_type"`
	Severity     string    `json:"severity"`
	Description  string    `json:"description"`
	TriggeredAt  time.Time `json:"triggered_at"`
	IsResolved   bool      `json:"is_resolved"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

type AlertSummary struct {
	TotalAlerts    int                `json:"total_alerts"`
	BySeverity     map[string]int     `json:"by_severity"`
	ByType         map[string]int     `json:"by_type"`
	OpenAlerts     int                `json:"open_alerts"`
	ResolvedAlerts int                `json:"resolved_alerts"`
}

type IntegrityOptimizationResult struct {
	OptimizationType string    `json:"optimization_type"`
	PerformanceGain  float64   `json:"performance_gain"`
	AppliedAt        time.Time `json:"applied_at"`
	Description      string    `json:"description"`
}

type IntegrityStats struct {
	TotalEvents      int64     `json:"total_events"`
	ValidEvents      int64     `json:"valid_events"`
	CorruptedEvents  int64     `json:"corrupted_events"`
	IntegrityScore   float64   `json:"integrity_score"`
	LastUpdated      time.Time `json:"last_updated"`
}

type IntegrityCheckSchedule struct {
	ScheduleID      string        `json:"schedule_id"`
	CheckType       string        `json:"check_type"`
	CronExpression  string        `json:"cron_expression"`
	IsEnabled       bool          `json:"is_enabled"`
	LastRun         *time.Time    `json:"last_run,omitempty"`
	NextRun         time.Time     `json:"next_run"`
}

type SignatureValidationResult struct {
	EventID         uuid.UUID `json:"event_id"`
	IsValid         bool      `json:"is_valid"`
	SignatureAlgorithm string `json:"signature_algorithm"`
	ValidatedAt     time.Time `json:"validated_at"`
	ValidationError string    `json:"validation_error,omitempty"`
}

type CryptographicInfo struct {
	Algorithm       string    `json:"algorithm"`
	KeyID           string    `json:"key_id"`
	KeyLength       int       `json:"key_length"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	IsExpired       bool      `json:"is_expired"`
}

type KeyRotationConfig struct {
	KeyType         string        `json:"key_type"`
	RotationInterval time.Duration `json:"rotation_interval"`
	AutoRotate      bool          `json:"auto_rotate"`
	BackupKeys      bool          `json:"backup_keys"`
	NotifyRotation  bool          `json:"notify_rotation"`
}

type KeyRotationResult struct {
	OldKeyID        string    `json:"old_key_id"`
	NewKeyID        string    `json:"new_key_id"`
	RotatedAt       time.Time `json:"rotated_at"`
	RotationType    string    `json:"rotation_type"`
	AffectedEvents  int64     `json:"affected_events"`
}

type BackupIntegrityResult struct {
	BackupID        string    `json:"backup_id"`
	IsIntact        bool      `json:"is_intact"`
	VerifiedAt      time.Time `json:"verified_at"`
	IntegrityScore  float64   `json:"integrity_score"`
	Issues          []string  `json:"issues,omitempty"`
}

type RestoreIntegrityResult struct {
	RestoreID       string    `json:"restore_id"`
	IsSuccessful    bool      `json:"is_successful"`
	RestoredAt      time.Time `json:"restored_at"`
	EventsRestored  int64     `json:"events_restored"`
	IntegrityChecks []string  `json:"integrity_checks"`
}

type CrossValidationCriteria struct {
	ValidationType  string    `json:"validation_type"`
	ReferenceSources []string `json:"reference_sources"`
	TimeRange       TimeRange `json:"time_range"`
	SampleSize      int       `json:"sample_size,omitempty"`
}

type CrossValidationResult struct {
	ValidationID    string    `json:"validation_id"`
	MatchPercentage float64   `json:"match_percentage"`
	ValidatedAt     time.Time `json:"validated_at"`
	Discrepancies   []string  `json:"discrepancies,omitempty"`
	IsValid         bool      `json:"is_valid"`
}

type IntegrityReportCriteria struct {
	ReportType      string    `json:"report_type"`
	TimeRange       TimeRange `json:"time_range"`
	IncludeDetails  bool      `json:"include_details"`
	DetailLevel     string    `json:"detail_level"`
}

type ComprehensiveIntegrityReport struct {
	ReportID        string    `json:"report_id"`
	GeneratedAt     time.Time `json:"generated_at"`
	OverallScore    float64   `json:"overall_score"`
	Summary         *IntegrityReportSummary `json:"summary"`
	Sections        map[string]interface{}  `json:"sections"`
	Recommendations []string  `json:"recommendations"`
}

type IntegrityReportSummary struct {
	TotalChecks     int       `json:"total_checks"`
	PassedChecks    int       `json:"passed_checks"`
	FailedChecks    int       `json:"failed_checks"`
	CriticalIssues  int       `json:"critical_issues"`
	ReportTime      time.Duration `json:"report_time"`
}

type IntegrityComplianceStatus struct {
	Framework       string    `json:"framework"`
	IsCompliant     bool      `json:"is_compliant"`
	ComplianceScore float64   `json:"compliance_score"`
	LastAssessment  time.Time `json:"last_assessment"`
	Requirements    map[string]bool `json:"requirements"`
	Gaps            []string  `json:"gaps,omitempty"`
}

type EvidenceExportCriteria struct {
	EvidenceType    string    `json:"evidence_type"`
	TimeRange       TimeRange `json:"time_range"`
	Format          string    `json:"format"`
	IncludeMetadata bool      `json:"include_metadata"`
	Encryption      bool      `json:"encryption"`
}

type IntegrityEvidence struct {
	EvidenceID      string    `json:"evidence_id"`
	EvidenceType    string    `json:"evidence_type"`
	ExportedAt      time.Time `json:"exported_at"`
	Format          string    `json:"format"`
	Size            int64     `json:"size"`
	Checksum        string    `json:"checksum"`
	Location        string    `json:"location"`
}