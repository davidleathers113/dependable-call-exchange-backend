package performance

import (
	"time"
)

// OperationType defines the type of operation being monitored
type OperationType int

const (
	OperationDNCQuery OperationType = iota
	OperationCacheQuery
	OperationDatabaseQuery
	OperationValidation
	OperationBloomFilter
)

func (ot OperationType) String() string {
	switch ot {
	case OperationDNCQuery:
		return "dnc_query"
	case OperationCacheQuery:
		return "cache_query"
	case OperationDatabaseQuery:
		return "database_query"
	case OperationValidation:
		return "validation"
	case OperationBloomFilter:
		return "bloom_filter"
	default:
		return "unknown"
	}
}

// LatencyMeasurement represents a single latency measurement
type LatencyMeasurement struct {
	Operation OperationType
	Duration  time.Duration
	CacheHit  bool
	Timestamp time.Time
	Metadata  map[string]interface{}
}

// CacheStats provides cache-specific latency statistics
type CacheStats struct {
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
}

// SLAViolation represents a violation of service level agreement
type SLAViolation struct {
	Type      string
	Operation OperationType
	Duration  time.Duration
	Threshold time.Duration
	CacheHit  bool
	Timestamp time.Time
	Severity  AlertSeverity
}

// AlertType defines the type of alert
type AlertType int

const (
	AlertTypeSLAViolation AlertType = iota
	AlertTypeSLACompliance
	AlertTypePerformanceDegradation
	AlertTypeThroughputDrop
	AlertTypeResourceExhaustion
	AlertTypeErrorRate
)

func (at AlertType) String() string {
	switch at {
	case AlertTypeSLAViolation:
		return "sla_violation"
	case AlertTypeSLACompliance:
		return "sla_compliance"
	case AlertTypePerformanceDegradation:
		return "performance_degradation"
	case AlertTypeThroughputDrop:
		return "throughput_drop"
	case AlertTypeResourceExhaustion:
		return "resource_exhaustion"
	case AlertTypeErrorRate:
		return "error_rate"
	default:
		return "unknown"
	}
}

// Alert represents a performance alert
type Alert struct {
	ID        string
	Type      AlertType
	Severity  AlertSeverity
	Message   string
	Timestamp time.Time
	Metadata  map[string]interface{}
	Resolved  bool
	ResolvedAt *time.Time
}

// PerformanceDegradation represents detected performance degradation
type PerformanceDegradation struct {
	StartTime       time.Time
	DetectedAt      time.Time
	Severity        float64 // multiplier compared to baseline
	BaselineLatency time.Duration
	CurrentLatency  time.Duration
	AffectedOps     []OperationType
	Confidence      float64 // 0.0 to 1.0
}

// PerformanceTrend provides performance trend analysis
type PerformanceTrend struct {
	Window        time.Duration
	StartTime     time.Time
	EndTime       time.Time
	TrendSlope    float64 // positive = getting slower, negative = getting faster
	Confidence    float64
	Measurements  int
	Baseline      *PerformanceBaseline
	Current       *PerformanceSnapshot
	Prediction    *PerformancePrediction
}

// PerformanceBaseline represents baseline performance characteristics
type PerformanceBaseline struct {
	StartTime   time.Time
	EndTime     time.Time
	P50Latency  time.Duration
	P95Latency  time.Duration
	P99Latency  time.Duration
	Throughput  float64
	Measurements int
	StdDev      time.Duration
}

// PerformanceSnapshot represents current performance state
type PerformanceSnapshot struct {
	Timestamp   time.Time
	P50Latency  time.Duration
	P95Latency  time.Duration
	P99Latency  time.Duration
	Throughput  float64
	ActiveConns int
	CacheHitRate float64
}

// PerformancePrediction provides predicted performance metrics
type PerformancePrediction struct {
	TimeHorizon     time.Duration
	PredictedP50    time.Duration
	PredictedP95    time.Duration
	PredictedP99    time.Duration
	PredictedThroughput float64
	Confidence      float64
	ModelAccuracy   float64
}

// LatencyBufferConfig configures the latency measurement buffer
type LatencyBufferConfig struct {
	Size            int
	FlushInterval   time.Duration
	RetentionPeriod time.Duration
}

// PercentileConfig configures percentile calculation
type PercentileConfig struct {
	Accuracy    float64 // accuracy of percentile estimates
	Compression float64 // compression factor for data structure
}

// SLAViolationConfig configures SLA violation tracking
type SLAViolationConfig struct {
	RetentionPeriod time.Duration
	AlertWindow     time.Duration
}

// AlertManagerConfig configures alert management
type AlertManagerConfig struct {
	Cooldown time.Duration
	Window   time.Duration
}

// DegradationConfig configures performance degradation detection
type DegradationConfig struct {
	Window     time.Duration
	Threshold  float64 // threshold multiplier for detecting degradation
	MinSamples int     // minimum samples required for reliable detection
}

// LatencyBuffer interface for buffering latency measurements
type LatencyBuffer interface {
	Add(measurement *LatencyMeasurement)
	Flush() []*LatencyMeasurement
	Size() int
	IsFull() bool
}

// PercentileTracker interface for tracking percentiles
type PercentileTracker interface {
	Add(duration time.Duration)
	Quantile(q float64) time.Duration
	Count() int64
	Reset()
}

// SLAViolationTracker interface for tracking SLA violations
type SLAViolationTracker interface {
	Record(violation *SLAViolation)
	GetViolations(window time.Duration) []*SLAViolation
	GetViolationRate(window time.Duration) float64
	Clear()
}

// AlertManager interface for managing alerts
type AlertManager interface {
	TriggerAlert(alert *Alert) error
	ResolveAlert(alertID string) error
	GetActiveAlerts() []*Alert
	IsInCooldown(alertType AlertType) bool
	SetCooldown(alertType AlertType, duration time.Duration)
}

// DegradationDetector interface for detecting performance degradation
type DegradationDetector interface {
	Add(measurement *LatencyMeasurement)
	CheckDegradation() *PerformanceDegradation
	GetTrend(window time.Duration) *PerformanceTrend
	SetBaseline(baseline *PerformanceBaseline)
	Reset()
}

// MonitoringEvent represents a monitoring event
type MonitoringEvent struct {
	Type      MonitoringEventType
	Timestamp time.Time
	Source    string
	Data      interface{}
	Severity  AlertSeverity
}

// MonitoringEventType defines the type of monitoring event
type MonitoringEventType int

const (
	EventTypeLatencyMeasurement MonitoringEventType = iota
	EventTypeSLAViolation
	EventTypePerformanceDegradation
	EventTypeAlert
	EventTypeThresholdExceeded
	EventTypeSystemHealthCheck
)

func (met MonitoringEventType) String() string {
	switch met {
	case EventTypeLatencyMeasurement:
		return "latency_measurement"
	case EventTypeSLAViolation:
		return "sla_violation"
	case EventTypePerformanceDegradation:
		return "performance_degradation"
	case EventTypeAlert:
		return "alert"
	case EventTypeThresholdExceeded:
		return "threshold_exceeded"
	case EventTypeSystemHealthCheck:
		return "system_health_check"
	default:
		return "unknown"
	}
}

// PerformanceReport provides comprehensive performance analysis
type PerformanceReport struct {
	GeneratedAt   time.Time
	ReportWindow  time.Duration
	Summary       *PerformanceSummary
	Trends        *PerformanceTrend
	SLACompliance *SLAComplianceReport
	Alerts        []*Alert
	Recommendations []*PerformanceRecommendation
}

// PerformanceSummary provides high-level performance metrics
type PerformanceSummary struct {
	TotalQueries     int64
	AverageLatency   time.Duration
	P50Latency       time.Duration
	P95Latency       time.Duration
	P99Latency       time.Duration
	MaxLatency       time.Duration
	MinLatency       time.Duration
	Throughput       float64
	CacheHitRate     float64
	ErrorRate        float64
	SLACompliance    float64
}

// SLAComplianceReport provides detailed SLA compliance analysis
type SLAComplianceReport struct {
	OverallCompliance   float64
	P50Compliance       float64
	P95Compliance       float64
	P99Compliance       float64
	ThroughputCompliance float64
	ViolationCount      int
	ViolationDetails    []*SLAViolation
	ComplianceTrend     []ComplianceDataPoint
}

// ComplianceDataPoint represents a point in compliance trend
type ComplianceDataPoint struct {
	Timestamp  time.Time
	Compliance float64
	Violations int
}

// PerformanceRecommendation provides actionable performance improvement suggestions
type PerformanceRecommendation struct {
	Type        RecommendationType
	Priority    RecommendationPriority
	Title       string
	Description string
	Impact      string
	Effort      string
	Category    string
	Metadata    map[string]interface{}
}

// RecommendationType defines the type of performance recommendation
type RecommendationType int

const (
	RecommendationTypeConfigOptimization RecommendationType = iota
	RecommendationTypeResourceScaling
	RecommendationTypeCacheOptimization
	RecommendationTypeQueryOptimization
	RecommendationTypeInfrastructureUpgrade
	RecommendationTypeApplicationTuning
)

func (rt RecommendationType) String() string {
	switch rt {
	case RecommendationTypeConfigOptimization:
		return "config_optimization"
	case RecommendationTypeResourceScaling:
		return "resource_scaling"
	case RecommendationTypeCacheOptimization:
		return "cache_optimization"
	case RecommendationTypeQueryOptimization:
		return "query_optimization"
	case RecommendationTypeInfrastructureUpgrade:
		return "infrastructure_upgrade"
	case RecommendationTypeApplicationTuning:
		return "application_tuning"
	default:
		return "unknown"
	}
}

// RecommendationPriority defines the priority of a recommendation
type RecommendationPriority int

const (
	RecommendationPriorityLow RecommendationPriority = iota
	RecommendationPriorityMedium
	RecommendationPriorityHigh
	RecommendationPriorityCritical
)

func (rp RecommendationPriority) String() string {
	switch rp {
	case RecommendationPriorityLow:
		return "low"
	case RecommendationPriorityMedium:
		return "medium"
	case RecommendationPriorityHigh:
		return "high"
	case RecommendationPriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// HealthCheck represents a system health check result
type HealthCheck struct {
	Timestamp     time.Time
	ComponentName string
	Status        HealthStatus
	Latency       time.Duration
	Message       string
	Details       map[string]interface{}
}

// HealthStatus represents the health status of a component
type HealthStatus int

const (
	HealthStatusHealthy HealthStatus = iota
	HealthStatusDegraded
	HealthStatusUnhealthy
	HealthStatusUnknown
)

func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	case HealthStatusUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

func (cbs CircuitBreakerState) String() string {
	switch cbs {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}