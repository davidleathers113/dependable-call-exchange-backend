package audit

import (
	"time"

	"github.com/google/uuid"
)

// TimeRangeCount represents event counts within specific time buckets
type TimeRangeCount struct {
	// Time buckets with counts
	Buckets       []*TimeBucket     `json:"buckets"`
	
	// Total and aggregate statistics
	TotalCount    int64             `json:"total_count"`
	TotalDuration time.Duration     `json:"total_duration"`
	BucketSize    time.Duration     `json:"bucket_size"`
	
	// Query metadata
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	QueryTime     time.Duration     `json:"query_time"`
	GeneratedAt   time.Time         `json:"generated_at"`
}

// TimeBucket represents a single time bucket with event count
type TimeBucket struct {
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	Count         int64             `json:"count"`
	
	// Additional statistics
	AverageCount  float64           `json:"average_count,omitempty"`
	UniqueActors  int64             `json:"unique_actors,omitempty"`
	UniqueTargets int64             `json:"unique_targets,omitempty"`
}

// MultiFieldCount represents event counts grouped by multiple fields simultaneously
type MultiFieldCount struct {
	// Field definitions
	Fields        []string                    `json:"fields"`
	
	// Multi-dimensional counts
	Counts        map[string]map[string]int64 `json:"counts"`
	
	// Total counts per primary field
	FieldTotals   map[string]int64            `json:"field_totals"`
	
	// Overall statistics
	TotalEvents   int64                       `json:"total_events"`
	UniqueValues  map[string]int              `json:"unique_values"`
	
	// Query metadata
	QueryTime     time.Duration               `json:"query_time"`
	GeneratedAt   time.Time                   `json:"generated_at"`
}

// TrendCriteria defines criteria for trend analysis
type TrendCriteria struct {
	// Time range for analysis
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	
	// Trend analysis configuration
	Granularity   string            `json:"granularity"` // hour, day, week, month
	MetricType    string            `json:"metric_type"` // count, rate, average
	
	// Event filtering
	EventFilter   EventFilter       `json:"event_filter,omitempty"`
	
	// Trend detection settings
	SensitivityLevel   string       `json:"sensitivity_level,omitempty"` // low, medium, high
	DetectAnomalies    bool         `json:"detect_anomalies"`
	CompareWithPeriod  bool         `json:"compare_with_period"`
	ComparisonDays     int          `json:"comparison_days,omitempty"`
	
	// Analysis options
	IncludeForecasting bool         `json:"include_forecasting"`
	ForecastDays       int          `json:"forecast_days,omitempty"`
	CalculateSeasonality bool       `json:"calculate_seasonality"`
}

// TrendAnalysis represents the result of trend analysis
type TrendAnalysis struct {
	// Analysis metadata
	Criteria      TrendCriteria     `json:"criteria"`
	GeneratedAt   time.Time         `json:"generated_at"`
	AnalysisTime  time.Duration     `json:"analysis_time"`
	
	// Time series data
	DataPoints    []*TrendPoint     `json:"data_points"`
	
	// Trend analysis results
	OverallTrend  string            `json:"overall_trend"` // increasing, decreasing, stable, volatile
	TrendStrength float64           `json:"trend_strength"` // 0-1 scale
	Volatility    float64           `json:"volatility"`
	
	// Statistical analysis
	Average       float64           `json:"average"`
	StandardDev   float64           `json:"standard_deviation"`
	MinValue      float64           `json:"min_value"`
	MaxValue      float64           `json:"max_value"`
	
	// Anomaly detection
	Anomalies     []*AnomalyPoint   `json:"anomalies,omitempty"`
	
	// Comparison with previous period
	Comparison    *PeriodComparison `json:"comparison,omitempty"`
	
	// Forecasting results
	Forecast      *ForecastResult   `json:"forecast,omitempty"`
	
	// Seasonality analysis
	Seasonality   *SeasonalityInfo  `json:"seasonality,omitempty"`
	
	// Insights and recommendations
	Insights      []string          `json:"insights,omitempty"`
	Recommendations []string        `json:"recommendations,omitempty"`
}

// TrendPoint represents a single point in trend analysis
type TrendPoint struct {
	Timestamp     time.Time         `json:"timestamp"`
	Value         float64           `json:"value"`
	SmoothedValue float64           `json:"smoothed_value,omitempty"`
	IsAnomaly     bool              `json:"is_anomaly"`
	AnomalyScore  float64           `json:"anomaly_score,omitempty"`
}

// AnomalyPoint represents a detected anomaly
type AnomalyPoint struct {
	Timestamp     time.Time         `json:"timestamp"`
	Value         float64           `json:"value"`
	ExpectedValue float64           `json:"expected_value"`
	Deviation     float64           `json:"deviation"`
	Severity      string            `json:"severity"` // low, medium, high, critical
	Reason        string            `json:"reason"`
}

// PeriodComparison compares current period with previous period
type PeriodComparison struct {
	PreviousPeriodStart time.Time   `json:"previous_period_start"`
	PreviousPeriodEnd   time.Time   `json:"previous_period_end"`
	
	CurrentAverage      float64     `json:"current_average"`
	PreviousAverage     float64     `json:"previous_average"`
	PercentChange       float64     `json:"percent_change"`
	
	IsSignificant       bool        `json:"is_significant"`
	Interpretation      string      `json:"interpretation"`
}

// ForecastResult contains forecasting results
type ForecastResult struct {
	ForecastPoints  []*ForecastPoint  `json:"forecast_points"`
	Confidence      float64           `json:"confidence"`
	Method          string            `json:"method"`
	ConfidenceBands *ConfidenceBands  `json:"confidence_bands,omitempty"`
}

// ForecastPoint represents a forecasted value
type ForecastPoint struct {
	Timestamp       time.Time         `json:"timestamp"`
	ForecastValue   float64           `json:"forecast_value"`
	LowerBound      float64           `json:"lower_bound"`
	UpperBound      float64           `json:"upper_bound"`
}

// ConfidenceBands provides confidence intervals for forecasting
type ConfidenceBands struct {
	Confidence80    []*ConfidenceInterval  `json:"confidence_80"`
	Confidence95    []*ConfidenceInterval  `json:"confidence_95"`
}

// ConfidenceInterval represents a confidence interval
type ConfidenceInterval struct {
	Timestamp       time.Time         `json:"timestamp"`
	LowerBound      float64           `json:"lower_bound"`
	UpperBound      float64           `json:"upper_bound"`
}

// SeasonalityInfo provides seasonality analysis
type SeasonalityInfo struct {
	HasSeasonality  bool              `json:"has_seasonality"`
	SeasonalPeriod  time.Duration     `json:"seasonal_period,omitempty"`
	SeasonalStrength float64          `json:"seasonal_strength,omitempty"`
	
	// Seasonal patterns
	HourlyPattern   map[int]float64   `json:"hourly_pattern,omitempty"`
	DailyPattern    map[string]float64 `json:"daily_pattern,omitempty"`
	MonthlyPattern  map[string]float64 `json:"monthly_pattern,omitempty"`
}

// ComplianceReportCriteria defines criteria for compliance reports
type ComplianceReportCriteria struct {
	// Time range for the report
	StartTime       time.Time         `json:"start_time"`
	EndTime         time.Time         `json:"end_time"`
	
	// Compliance scope
	ComplianceTypes []string          `json:"compliance_types"` // gdpr, tcpa, ccpa, pipeda
	Jurisdictions   []string          `json:"jurisdictions,omitempty"`
	
	// Event filtering
	EventFilter     EventFilter       `json:"event_filter,omitempty"`
	
	// Report configuration
	IncludeViolations    bool         `json:"include_violations"`
	IncludeStatistics    bool         `json:"include_statistics"`
	IncludeRemediation   bool         `json:"include_remediation"`
	IncludeTrends        bool         `json:"include_trends"`
	
	// Detail level
	DetailLevel     string            `json:"detail_level"` // summary, detailed, comprehensive
	
	// Export options
	Format          string            `json:"format,omitempty"` // pdf, excel, csv, json
}

// ComplianceReport represents a comprehensive compliance report
type ComplianceReport struct {
	// Report metadata
	ID              string                    `json:"id"`
	GeneratedAt     time.Time                 `json:"generated_at"`
	Criteria        ComplianceReportCriteria  `json:"criteria"`
	
	// Executive summary
	Summary         *ComplianceSummary        `json:"summary"`
	
	// Detailed findings
	Violations      []*ComplianceViolation    `json:"violations,omitempty"`
	Statistics      *ComplianceStatistics     `json:"statistics,omitempty"`
	Trends          *ComplianceTrends         `json:"trends,omitempty"`
	
	// Remediation actions
	Recommendations []*RemediationAction      `json:"recommendations,omitempty"`
	
	// Risk assessment
	RiskAssessment  *ComplianceRiskAssessment `json:"risk_assessment,omitempty"`
	
	// Attestations and certifications
	Attestations    []*ComplianceAttestation  `json:"attestations,omitempty"`
	
	// Report metadata
	ReportTime      time.Duration             `json:"report_time"`
	DataSources     []string                  `json:"data_sources"`
	Version         string                    `json:"version"`
}

// ComplianceSummary provides high-level compliance overview
type ComplianceSummary struct {
	OverallStatus      string            `json:"overall_status"` // compliant, non_compliant, partial
	ComplianceScore    float64           `json:"compliance_score"` // 0-100
	TotalEvents        int64             `json:"total_events"`
	ViolationCount     int64             `json:"violation_count"`
	ViolationRate      float64           `json:"violation_rate"`
	
	// By compliance type
	ComplianceBreakdown map[string]*ComplianceTypeStatus `json:"compliance_breakdown"`
	
	// Risk indicators
	HighRiskItems      int               `json:"high_risk_items"`
	CriticalIssues     int               `json:"critical_issues"`
	
	// Improvement indicators
	TrendDirection     string            `json:"trend_direction"` // improving, declining, stable
	RecentChanges      string            `json:"recent_changes"`
}

// ComplianceTypeStatus represents status for a specific compliance type
type ComplianceTypeStatus struct {
	Status             string            `json:"status"`
	Score              float64           `json:"score"`
	ViolationCount     int64             `json:"violation_count"`
	LastViolation      *time.Time        `json:"last_violation,omitempty"`
	NextAuditDue       *time.Time        `json:"next_audit_due,omitempty"`
}

// ComplianceViolation represents a specific compliance violation
type ComplianceViolation struct {
	ID                 uuid.UUID         `json:"id"`
	EventID            uuid.UUID         `json:"event_id"`
	ViolationType      string            `json:"violation_type"`
	ComplianceType     string            `json:"compliance_type"`
	Severity           string            `json:"severity"`
	
	Description        string            `json:"description"`
	Impact             string            `json:"impact"`
	Recommendation     string            `json:"recommendation"`
	
	DetectedAt         time.Time         `json:"detected_at"`
	ResolvedAt         *time.Time        `json:"resolved_at,omitempty"`
	Status             string            `json:"status"` // open, in_progress, resolved, false_positive
	
	// Context information
	ActorID            string            `json:"actor_id"`
	TargetID           string            `json:"target_id"`
	AffectedRecords    int64             `json:"affected_records,omitempty"`
	
	// Remediation tracking
	RemediationActions []string          `json:"remediation_actions,omitempty"`
	ResponsibleParty   string            `json:"responsible_party,omitempty"`
}

// ComplianceStatistics provides detailed compliance statistics
type ComplianceStatistics struct {
	// Event statistics
	EventCounts        map[EventType]int64    `json:"event_counts"`
	EventTrends        map[string]float64     `json:"event_trends"`
	
	// Actor statistics
	ActorCompliance    map[string]float64     `json:"actor_compliance"`
	TopViolators       []ActorViolationStats  `json:"top_violators,omitempty"`
	
	// Geographic statistics
	RegionalCompliance map[string]float64     `json:"regional_compliance,omitempty"`
	
	// Time-based statistics
	HourlyViolations   map[int]int64          `json:"hourly_violations"`
	DailyViolations    map[string]int64       `json:"daily_violations"`
	MonthlyViolations  map[string]int64       `json:"monthly_violations"`
	
	// Severity distribution
	SeverityDistribution map[string]int64     `json:"severity_distribution"`
	
	// Resolution statistics
	ResolutionTimes    *ResolutionTimeStats   `json:"resolution_times,omitempty"`
	ResolutionRates    map[string]float64     `json:"resolution_rates"`
}

// ActorViolationStats represents violation statistics for an actor
type ActorViolationStats struct {
	ActorID            string            `json:"actor_id"`
	ActorType          string            `json:"actor_type"`
	ViolationCount     int64             `json:"violation_count"`
	ViolationRate      float64           `json:"violation_rate"`
	LastViolation      time.Time         `json:"last_violation"`
	SeverityBreakdown  map[string]int64  `json:"severity_breakdown"`
}

// ResolutionTimeStats provides statistics on violation resolution times
type ResolutionTimeStats struct {
	AverageResolutionTime time.Duration   `json:"average_resolution_time"`
	MedianResolutionTime  time.Duration   `json:"median_resolution_time"`
	MinResolutionTime     time.Duration   `json:"min_resolution_time"`
	MaxResolutionTime     time.Duration   `json:"max_resolution_time"`
	
	// Resolution time by severity
	BySeverity            map[string]time.Duration `json:"by_severity"`
	
	// SLA compliance
	SLATarget             time.Duration   `json:"sla_target,omitempty"`
	SLAComplianceRate     float64         `json:"sla_compliance_rate,omitempty"`
}

// ComplianceTrends provides trend analysis for compliance metrics
type ComplianceTrends struct {
	ViolationTrend     *TrendAnalysis    `json:"violation_trend"`
	ComplianceScore    *TrendAnalysis    `json:"compliance_score"`
	ResolutionTime     *TrendAnalysis    `json:"resolution_time"`
	
	// Predictions
	PredictedViolations *ForecastResult  `json:"predicted_violations,omitempty"`
	RiskTrajectory     string            `json:"risk_trajectory"` // improving, stable, worsening
}

// RemediationAction represents a recommended or required action
type RemediationAction struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"` // immediate, short_term, long_term
	Priority           string            `json:"priority"` // low, medium, high, critical
	
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	Rationale          string            `json:"rationale"`
	
	// Implementation details
	EstimatedEffort    string            `json:"estimated_effort,omitempty"`
	RequiredResources  []string          `json:"required_resources,omitempty"`
	Timeline           string            `json:"timeline,omitempty"`
	
	// Tracking
	Status             string            `json:"status"` // pending, in_progress, completed, cancelled
	AssignedTo         string            `json:"assigned_to,omitempty"`
	DueDate            *time.Time        `json:"due_date,omitempty"`
	CompletedAt        *time.Time        `json:"completed_at,omitempty"`
	
	// Impact assessment
	ExpectedImprovement string           `json:"expected_improvement,omitempty"`
	RiskReduction      float64           `json:"risk_reduction,omitempty"`
}

// ComplianceRiskAssessment provides risk analysis
type ComplianceRiskAssessment struct {
	OverallRiskLevel   string            `json:"overall_risk_level"` // low, medium, high, critical
	RiskScore          float64           `json:"risk_score"` // 0-100
	
	// Risk factors
	RiskFactors        []*RiskFactor     `json:"risk_factors"`
	
	// Risk by category
	CategoryRisks      map[string]float64 `json:"category_risks"`
	
	// Probability assessments
	ViolationProbability map[string]float64 `json:"violation_probability"`
	
	// Impact assessments
	PotentialImpact    *ImpactAssessment  `json:"potential_impact"`
	
	// Mitigation status
	MitigationCoverage float64           `json:"mitigation_coverage"` // % of risks mitigated
	
	// Assessment metadata
	AssessedAt         time.Time         `json:"assessed_at"`
	AssessedBy         string            `json:"assessed_by"`
	NextReviewDue      time.Time         `json:"next_review_due"`
}

// RiskFactor represents an individual risk factor
type RiskFactor struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	Category           string            `json:"category"`
	Likelihood         float64           `json:"likelihood"` // 0-1
	Impact             float64           `json:"impact"` // 0-1
	RiskScore          float64           `json:"risk_score"`
	
	// Mitigation
	IsMitigated        bool              `json:"is_mitigated"`
	MitigationActions  []string          `json:"mitigation_actions,omitempty"`
	ResidualRisk       float64           `json:"residual_risk,omitempty"`
}

// ImpactAssessment provides impact analysis
type ImpactAssessment struct {
	FinancialImpact    *FinancialImpact  `json:"financial_impact,omitempty"`
	ReputationalImpact string            `json:"reputational_impact,omitempty"`
	OperationalImpact  string            `json:"operational_impact,omitempty"`
	LegalImpact        string            `json:"legal_impact,omitempty"`
	
	// Quantified impacts
	EstimatedFines     float64           `json:"estimated_fines,omitempty"`
	EstimatedCosts     float64           `json:"estimated_costs,omitempty"`
	BusinessDisruption string            `json:"business_disruption,omitempty"`
}

// FinancialImpact provides financial impact details
type FinancialImpact struct {
	MinImpact          float64           `json:"min_impact"`
	MaxImpact          float64           `json:"max_impact"`
	ExpectedImpact     float64           `json:"expected_impact"`
	Currency           string            `json:"currency"`
	ImpactCategories   map[string]float64 `json:"impact_categories"`
}

// ComplianceAttestation represents a compliance attestation
type ComplianceAttestation struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"`
	IssuedBy           string            `json:"issued_by"`
	IssuedAt           time.Time         `json:"issued_at"`
	ValidUntil         time.Time         `json:"valid_until"`
	
	// Attestation details
	Statement          string            `json:"statement"`
	Scope              string            `json:"scope"`
	Evidence           []string          `json:"evidence,omitempty"`
	
	// Status
	Status             string            `json:"status"` // valid, expired, revoked
	RevokedAt          *time.Time        `json:"revoked_at,omitempty"`
	RevocationReason   string            `json:"revocation_reason,omitempty"`
}

// ActivitySummary provides high-level activity overview for dashboards
type ActivitySummary struct {
	// Time period
	TimeRange          TimeRange         `json:"time_range"`
	
	// Event activity
	TotalEvents        int64             `json:"total_events"`
	EventsToday        int64             `json:"events_today"`
	EventGrowthRate    float64           `json:"event_growth_rate"` // % change from previous period
	
	// Actor activity
	ActiveActors       int64             `json:"active_actors"`
	NewActors          int64             `json:"new_actors"`
	TopActors          []ActorActivity   `json:"top_actors"`
	
	// Event type distribution
	EventTypeBreakdown map[EventType]int64 `json:"event_type_breakdown"`
	TopEventTypes      []EventTypeActivity `json:"top_event_types"`
	
	// Severity distribution
	SeverityBreakdown  map[Severity]int64  `json:"severity_breakdown"`
	CriticalEvents     int64             `json:"critical_events"`
	ErrorEvents        int64             `json:"error_events"`
	
	// Compliance activity
	ComplianceEvents   int64             `json:"compliance_events"`
	ViolationEvents    int64             `json:"violation_events"`
	ConsentEvents      int64             `json:"consent_events"`
	
	// System health indicators
	ErrorRate          float64           `json:"error_rate"`
	SuccessRate        float64           `json:"success_rate"`
	SystemUptime       float64           `json:"system_uptime,omitempty"`
	
	// Performance metrics
	AverageEventSize   int64             `json:"average_event_size"`
	EventThroughput    float64           `json:"event_throughput"` // Events per second
	
	// Trends and patterns
	HourlyDistribution map[int]int64     `json:"hourly_distribution"`
	ActivityTrend      string            `json:"activity_trend"` // increasing, decreasing, stable
	
	// Generated metadata
	GeneratedAt        time.Time         `json:"generated_at"`
	QueryTime          time.Duration     `json:"query_time"`
	CacheHit           bool              `json:"cache_hit"`
}

// ActorActivity represents activity statistics for an actor
type ActorActivity struct {
	ActorID            string            `json:"actor_id"`
	ActorType          string            `json:"actor_type"`
	EventCount         int64             `json:"event_count"`
	LastActivity       time.Time         `json:"last_activity"`
	EventTypes         []EventType       `json:"event_types,omitempty"`
	ErrorRate          float64           `json:"error_rate,omitempty"`
}

// EventTypeActivity represents activity statistics for an event type
type EventTypeActivity struct {
	EventType          EventType         `json:"event_type"`
	Count              int64             `json:"count"`
	Percentage         float64           `json:"percentage"`
	GrowthRate         float64           `json:"growth_rate,omitempty"`
	LastOccurrence     time.Time         `json:"last_occurrence"`
}

// PatternCriteria defines criteria for pattern analysis
type PatternCriteria struct {
	// Time range for analysis
	StartTime          time.Time         `json:"start_time"`
	EndTime            time.Time         `json:"end_time"`
	
	// Pattern detection configuration
	PatternTypes       []string          `json:"pattern_types"` // temporal, behavioral, sequence, anomaly
	MinOccurrences     int               `json:"min_occurrences"` // Minimum occurrences to consider a pattern
	ConfidenceLevel    float64           `json:"confidence_level"` // 0-1
	
	// Event filtering
	EventFilter        EventFilter       `json:"event_filter,omitempty"`
	
	// Analysis focus
	FocusFields        []string          `json:"focus_fields,omitempty"` // Fields to analyze for patterns
	GroupBy            []string          `json:"group_by,omitempty"`    // Fields to group analysis by
	
	// Advanced options
	DetectAnomalies    bool              `json:"detect_anomalies"`
	IncludeStatistics  bool              `json:"include_statistics"`
	AnalyzeSequences   bool              `json:"analyze_sequences"`
	
	// Performance options
	MaxPatterns        int               `json:"max_patterns,omitempty"`
	SamplingRate       float64           `json:"sampling_rate,omitempty"` // 0-1 for large datasets
}

// PatternAnalysis represents the result of pattern analysis
type PatternAnalysis struct {
	// Analysis metadata
	Criteria           PatternCriteria   `json:"criteria"`
	GeneratedAt        time.Time         `json:"generated_at"`
	AnalysisTime       time.Duration     `json:"analysis_time"`
	EventsAnalyzed     int64             `json:"events_analyzed"`
	
	// Detected patterns
	TemporalPatterns   []*TemporalPattern   `json:"temporal_patterns,omitempty"`
	BehavioralPatterns []*BehavioralPattern `json:"behavioral_patterns,omitempty"`
	SequencePatterns   []*SequencePattern   `json:"sequence_patterns,omitempty"`
	AnomalyPatterns    []*AnomalyPattern    `json:"anomaly_patterns,omitempty"`
	
	// Pattern statistics
	PatternCount       int               `json:"pattern_count"`
	UniquePatternsFound int              `json:"unique_patterns_found"`
	
	// Insights
	KeyInsights        []string          `json:"key_insights,omitempty"`
	Recommendations    []string          `json:"recommendations,omitempty"`
	
	// Risk indicators
	RiskPatterns       []*RiskPattern    `json:"risk_patterns,omitempty"`
	SecurityConcerns   []string          `json:"security_concerns,omitempty"`
}

// TemporalPattern represents a time-based pattern
type TemporalPattern struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"` // recurring, cyclical, seasonal, burst
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"`
	
	// Pattern details
	Frequency          string            `json:"frequency,omitempty"` // hourly, daily, weekly, monthly
	RecurrenceRule     string            `json:"recurrence_rule,omitempty"`
	TimePeriods        []TimeRange       `json:"time_periods"`
	
	// Statistics
	Occurrences        int               `json:"occurrences"`
	AverageInterval    time.Duration     `json:"average_interval,omitempty"`
	StdDevInterval     time.Duration     `json:"std_dev_interval,omitempty"`
	
	// Associated events
	EventTypes         []EventType       `json:"event_types"`
	AffectedActors     []string          `json:"affected_actors,omitempty"`
}

// BehavioralPattern represents user/actor behavioral patterns
type BehavioralPattern struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"` // normal, suspicious, error-prone, efficient
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"`
	
	// Pattern characteristics
	ActorPattern       string            `json:"actor_pattern"`
	ActionSequence     []string          `json:"action_sequence,omitempty"`
	TypicalFrequency   float64           `json:"typical_frequency"`
	
	// Affected entities
	ActorIDs           []string          `json:"actor_ids"`
	ActorTypes         []string          `json:"actor_types"`
	
	// Risk assessment
	RiskLevel          string            `json:"risk_level,omitempty"`
	SecurityRelevance  bool              `json:"security_relevance"`
	
	// Statistics
	Occurrences        int               `json:"occurrences"`
	FirstSeen          time.Time         `json:"first_seen"`
	LastSeen           time.Time         `json:"last_seen"`
}

// SequencePattern represents event sequence patterns
type SequencePattern struct {
	ID                 string            `json:"id"`
	Description        string            `json:"description"`
	Confidence         float64           `json:"confidence"`
	
	// Sequence definition
	EventSequence      []EventType       `json:"event_sequence"`
	MaxTimespan        time.Duration     `json:"max_timespan"`
	MinTimespan        time.Duration     `json:"min_timespan"`
	
	// Pattern statistics
	Occurrences        int               `json:"occurrences"`
	SuccessRate        float64           `json:"success_rate,omitempty"`
	AverageCompletion  time.Duration     `json:"average_completion,omitempty"`
	
	// Common variations
	Variations         []SequenceVariation `json:"variations,omitempty"`
	
	// Business relevance
	BusinessProcess    string            `json:"business_process,omitempty"`
	CriticalityLevel   string            `json:"criticality_level,omitempty"`
}

// SequenceVariation represents a variation of a sequence pattern
type SequenceVariation struct {
	EventSequence      []EventType       `json:"event_sequence"`
	Frequency          int               `json:"frequency"`
	SuccessRate        float64           `json:"success_rate,omitempty"`
}

// AnomalyPattern represents detected anomalous patterns
type AnomalyPattern struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"` // volume, timing, behavior, data
	Description        string            `json:"description"`
	Severity           string            `json:"severity"`
	Confidence         float64           `json:"confidence"`
	
	// Anomaly details
	ExpectedBehavior   string            `json:"expected_behavior"`
	ObservedBehavior   string            `json:"observed_behavior"`
	Deviation          float64           `json:"deviation"`
	
	// Temporal information
	DetectedAt         time.Time         `json:"detected_at"`
	AnomalyWindow      TimeRange         `json:"anomaly_window"`
	
	// Affected entities
	AffectedActors     []string          `json:"affected_actors,omitempty"`
	AffectedTargets    []string          `json:"affected_targets,omitempty"`
	AffectedEvents     []uuid.UUID       `json:"affected_events,omitempty"`
	
	// Impact assessment
	PotentialImpact    string            `json:"potential_impact,omitempty"`
	Recommendations    []string          `json:"recommendations,omitempty"`
}

// RiskPattern represents patterns that indicate potential risks
type RiskPattern struct {
	ID                 string            `json:"id"`
	RiskType           string            `json:"risk_type"` // security, compliance, operational, financial
	Description        string            `json:"description"`
	RiskLevel          string            `json:"risk_level"`
	Confidence         float64           `json:"confidence"`
	
	// Pattern indicators
	Indicators         []RiskIndicator   `json:"indicators"`
	
	// Risk assessment
	Likelihood         float64           `json:"likelihood"`
	Impact             float64           `json:"impact"`
	RiskScore          float64           `json:"risk_score"`
	
	// Mitigation
	MitigationActions  []string          `json:"mitigation_actions,omitempty"`
	IsAddressed        bool              `json:"is_addressed"`
}

// RiskIndicator represents an individual risk indicator
type RiskIndicator struct {
	Name               string            `json:"name"`
	Value              interface{}       `json:"value"`
	Threshold          interface{}       `json:"threshold,omitempty"`
	IsTriggered        bool              `json:"is_triggered"`
	Severity           string            `json:"severity"`
}

// CorrelationCriteria defines criteria for event correlation analysis
type CorrelationCriteria struct {
	// Correlation configuration
	CorrelationTypes   []string          `json:"correlation_types"` // temporal, causal, actor_based, target_based
	MaxCorrelations    int               `json:"max_correlations,omitempty"`
	MinConfidence      float64           `json:"min_confidence,omitempty"`
	
	// Time window for correlation
	TimeWindow         time.Duration     `json:"time_window"`
	MaxTimeDistance    time.Duration     `json:"max_time_distance,omitempty"`
	
	// Correlation scope
	SameActor          bool              `json:"same_actor,omitempty"`
	SameTarget         bool              `json:"same_target,omitempty"`
	SameSession        bool              `json:"same_session,omitempty"`
	
	// Event filtering
	EventTypes         []EventType       `json:"event_types,omitempty"`
	ExcludeEventTypes  []EventType       `json:"exclude_event_types,omitempty"`
	
	// Analysis options
	IncludeStatistics  bool              `json:"include_statistics"`
	CalculateStrength  bool              `json:"calculate_strength"`
	DetectCausality    bool              `json:"detect_causality"`
	
	// Performance options
	UseCache           bool              `json:"use_cache"`
	MaxDepth           int               `json:"max_depth,omitempty"`
}

// CorrelationResult represents the result of event correlation analysis
type CorrelationResult struct {
	// Source event
	SourceEventID      uuid.UUID         `json:"source_event_id"`
	
	// Analysis metadata
	Criteria           CorrelationCriteria `json:"criteria"`
	GeneratedAt        time.Time         `json:"generated_at"`
	AnalysisTime       time.Duration     `json:"analysis_time"`
	
	// Correlated events
	Correlations       []*EventCorrelation `json:"correlations"`
	
	// Correlation statistics
	TotalCorrelations  int               `json:"total_correlations"`
	StrongCorrelations int               `json:"strong_correlations"`
	WeakCorrelations   int               `json:"weak_correlations"`
	
	// Analysis insights
	Patterns           []string          `json:"patterns,omitempty"`
	Insights           []string          `json:"insights,omitempty"`
	Recommendations    []string          `json:"recommendations,omitempty"`
	
	// Network information
	CorrelationNetwork *CorrelationNetwork `json:"correlation_network,omitempty"`
}

// EventCorrelation represents a correlation between two events
type EventCorrelation struct {
	TargetEventID      uuid.UUID         `json:"target_event_id"`
	CorrelationType    string            `json:"correlation_type"`
	Confidence         float64           `json:"confidence"`
	Strength           float64           `json:"strength"`
	
	// Temporal relationship
	TimeDistance       time.Duration     `json:"time_distance"`
	RelationDirection  string            `json:"relation_direction"` // before, after, concurrent
	
	// Common attributes
	CommonFields       map[string]interface{} `json:"common_fields,omitempty"`
	DifferingFields    map[string]interface{} `json:"differing_fields,omitempty"`
	
	// Causal relationship
	CausalRelation     string            `json:"causal_relation,omitempty"` // causes, caused_by, correlated
	CausalConfidence   float64           `json:"causal_confidence,omitempty"`
	
	// Context
	Context            string            `json:"context,omitempty"`
	BusinessRelevance  string            `json:"business_relevance,omitempty"`
}

// CorrelationNetwork represents a network of correlated events
type CorrelationNetwork struct {
	Nodes              []*NetworkNode    `json:"nodes"`
	Edges              []*NetworkEdge    `json:"edges"`
	
	// Network statistics
	NodeCount          int               `json:"node_count"`
	EdgeCount          int               `json:"edge_count"`
	ConnectedComponents int              `json:"connected_components"`
	
	// Network metrics
	Density            float64           `json:"density"`
	Clustering         float64           `json:"clustering"`
	
	// Central nodes
	MostConnected      []uuid.UUID       `json:"most_connected,omitempty"`
	MostInfluential    []uuid.UUID       `json:"most_influential,omitempty"`
}

// NetworkNode represents a node in the correlation network
type NetworkNode struct {
	EventID            uuid.UUID         `json:"event_id"`
	EventType          EventType         `json:"event_type"`
	Timestamp          time.Time         `json:"timestamp"`
	
	// Network properties
	Degree             int               `json:"degree"`
	BetweennessCentrality float64        `json:"betweenness_centrality,omitempty"`
	ClosenessCentrality   float64        `json:"closeness_centrality,omitempty"`
	
	// Node attributes
	ActorID            string            `json:"actor_id"`
	TargetID           string            `json:"target_id"`
	IsAnomaly          bool              `json:"is_anomaly"`
}

// NetworkEdge represents an edge in the correlation network
type NetworkEdge struct {
	SourceEventID      uuid.UUID         `json:"source_event_id"`
	TargetEventID      uuid.UUID         `json:"target_event_id"`
	CorrelationType    string            `json:"correlation_type"`
	Strength           float64           `json:"strength"`
	Direction          string            `json:"direction"` // directed, undirected
}