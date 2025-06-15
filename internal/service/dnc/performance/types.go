package performance

import (
	"context"
	"database/sql"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// QueryOptimization contains all optimization strategies for a DNC query
type QueryOptimization struct {
	PhoneNumber       *values.PhoneNumber
	Timestamp         time.Time
	
	// Cache strategies
	CacheStrategy     CacheStrategy
	L1CacheHit        bool
	L2CacheHit        bool
	BloomFilterResult BloomFilterResult
	
	// Resource allocation
	Connection        *sql.Conn
	Worker            *Worker
	RequiresDBQuery   bool
	
	// Results
	Result interface{}
	
	// Performance metrics
	Duration          time.Duration
	CacheLatency      time.Duration
	QueryLatency      time.Duration
}

// CacheStrategy defines caching approach for a query
type CacheStrategy int

const (
	CacheStrategySkip CacheStrategy = iota
	CacheStrategyL1Hit
	CacheStrategyL2Hit
	CacheStrategyMiss
	CacheStrategyRefresh
)

func (cs CacheStrategy) String() string {
	switch cs {
	case CacheStrategySkip:
		return "skip"
	case CacheStrategyL1Hit:
		return "l1_hit"
	case CacheStrategyL2Hit:
		return "l2_hit"
	case CacheStrategyMiss:
		return "miss"
	case CacheStrategyRefresh:
		return "refresh"
	default:
		return "unknown"
	}
}

// BloomFilterResult indicates bloom filter check result
type BloomFilterResult int

const (
	BloomFilterNotChecked BloomFilterResult = iota
	BloomFilterPositive
	BloomFilterNegative
)

func (bfr BloomFilterResult) String() string {
	switch bfr {
	case BloomFilterNotChecked:
		return "not_checked"
	case BloomFilterPositive:
		return "positive"
	case BloomFilterNegative:
		return "negative"
	default:
		return "unknown"
	}
}

// OptimizationStats provides comprehensive statistics about optimizer performance
type OptimizationStats struct {
	ConnectionPool *ConnectionPoolStats
	WorkerPool     *WorkerPoolStats
	MemoryPool     *MemoryPoolStats
	L1Cache        *L1CacheStats
	BloomFilter    *BloomFilterStats
	Timestamp      time.Time
}

// ConnectionPoolStats tracks connection pool performance
type ConnectionPoolStats struct {
	Active         int
	Idle           int
	Total          int
	MaxConnections int
	WaitingQueries int
	AverageWaitTime time.Duration
	TotalCreated   int64
	TotalClosed    int64
	TotalErrors    int64
}

// WorkerPoolStats tracks worker pool performance
type WorkerPoolStats struct {
	Active       int
	Idle         int
	Total        int
	Queued       int
	MaxQueueSize int
	TotalTasks   int64
	CompletedTasks int64
	FailedTasks  int64
	AverageTaskDuration time.Duration
}

// MemoryPoolStats tracks memory pool performance
type MemoryPoolStats struct {
	Allocated    int
	Available    int
	Total        int
	TotalAllocs  int64
	TotalFrees   int64
	BytesInUse   int64
	BytesTotal   int64
}

// L1CacheStats tracks L1 cache performance
type L1CacheStats struct {
	Size         int
	MaxSize      int
	HitRate      float64
	TotalHits    int64
	TotalMisses  int64
	TotalQueries int64
	AverageLatency time.Duration
	EvictionCount int64
}

// BloomFilterStats tracks bloom filter performance
type BloomFilterStats struct {
	Size           uint
	HashFunctions  uint
	EstimatedItems uint
	FalsePositiveRate float64
	TotalChecks    int64
	TotalAdds      int64
	MemoryUsage    int64
}

// LoadBalancingStrategy defines how to distribute load across resources
type LoadBalancingStrategy int

const (
	LoadBalancingRoundRobin LoadBalancingStrategy = iota
	LoadBalancingLeastConnections
	LoadBalancingWeightedRoundRobin
	LoadBalancingResourceBased
	LoadBalancingLatencyBased
)

func (lbs LoadBalancingStrategy) String() string {
	switch lbs {
	case LoadBalancingRoundRobin:
		return "round_robin"
	case LoadBalancingLeastConnections:
		return "least_connections"
	case LoadBalancingWeightedRoundRobin:
		return "weighted_round_robin"
	case LoadBalancingResourceBased:
		return "resource_based"
	case LoadBalancingLatencyBased:
		return "latency_based"
	default:
		return "unknown"
	}
}

// Worker represents a worker in the worker pool
type Worker struct {
	ID       int
	Active   bool
	TaskChan chan Task
	QuitChan chan bool
	Stats    WorkerStats
}

// WorkerStats tracks individual worker performance
type WorkerStats struct {
	TasksCompleted int64
	TasksFailed    int64
	TotalDuration  time.Duration
	LastActive     time.Time
	Created        time.Time
}

// Task represents a unit of work for the worker pool
type Task struct {
	ID       string
	Type     TaskType
	Priority int
	Payload  interface{}
	Context  context.Context
	ResultCh chan TaskResult
	Created  time.Time
}

// TaskType defines the type of work to be performed
type TaskType int

const (
	TaskTypeDNCQuery TaskType = iota
	TaskTypeCacheWarmup
	TaskTypeCacheEviction
	TaskTypeMetricsCollection
	TaskTypeConnectionMaintenance
)

func (tt TaskType) String() string {
	switch tt {
	case TaskTypeDNCQuery:
		return "dnc_query"
	case TaskTypeCacheWarmup:
		return "cache_warmup"
	case TaskTypeCacheEviction:
		return "cache_eviction"
	case TaskTypeMetricsCollection:
		return "metrics_collection"
	case TaskTypeConnectionMaintenance:
		return "connection_maintenance"
	default:
		return "unknown"
	}
}

// TaskResult represents the result of a completed task
type TaskResult struct {
	Success  bool
	Data     interface{}
	Error    error
	Duration time.Duration
	Worker   int
}

// CacheEntry represents an entry in the cache
type CacheEntry struct {
	Key       string
	Value     interface{}
	TTL       time.Duration
	CreatedAt time.Time
	AccessedAt time.Time
	AccessCount int64
}

// IsExpired checks if the cache entry has expired
func (ce *CacheEntry) IsExpired() bool {
	return time.Since(ce.CreatedAt) > ce.TTL
}

// ShouldEvict determines if entry should be evicted based on access patterns
func (ce *CacheEntry) ShouldEvict(threshold time.Duration) bool {
	return time.Since(ce.AccessedAt) > threshold && ce.AccessCount < 2
}

// PerformanceProfile defines performance characteristics and targets
type PerformanceProfile struct {
	Name                string
	TargetLatencyP50    time.Duration
	TargetLatencyP95    time.Duration
	TargetLatencyP99    time.Duration
	TargetThroughput    int
	MaxConcurrentQueries int
	CacheHitRateTarget  float64
	ResourceLimits      ResourceLimits
}

// ResourceLimits defines resource usage constraints
type ResourceLimits struct {
	MaxMemoryMB     int
	MaxCPUPercent   int
	MaxConnections  int
	MaxWorkers      int
	MaxCacheSize    int
}

// AlertThreshold defines when to trigger performance alerts
type AlertThreshold struct {
	MetricName      string
	ThresholdType   ThresholdType
	Value           float64
	Duration        time.Duration
	Severity        AlertSeverity
	Description     string
}

// ThresholdType defines how threshold is evaluated
type ThresholdType int

const (
	ThresholdTypeGreater ThresholdType = iota
	ThresholdTypeLess
	ThresholdTypeEqual
	ThresholdTypePercentile
)

// AlertSeverity defines alert severity levels
type AlertSeverity int

const (
	AlertSeverityInfo AlertSeverity = iota
	AlertSeverityWarning
	AlertSeverityCritical
	AlertSeverityEmergency
)

func (as AlertSeverity) String() string {
	switch as {
	case AlertSeverityInfo:
		return "info"
	case AlertSeverityWarning:
		return "warning"
	case AlertSeverityCritical:
		return "critical"
	case AlertSeverityEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// CacheWarmerConfig configures cache warming behavior
type CacheWarmerConfig struct {
	Enabled         bool
	WarmupInterval  time.Duration
	BatchSize       int
	MaxConcurrency  int
	PredictiveMode  bool
	AnalyticsWindow time.Duration
}

// PrewarmTarget represents data to preload into cache
type PrewarmTarget struct {
	Pattern     string
	Priority    int
	TTL         time.Duration
	Source      PrewarmSource
	LastWarmed  time.Time
	Success     bool
	ErrorCount  int
}

// PrewarmSource defines where to get prewarming data
type PrewarmSource int

const (
	PrewarmSourceAnalytics PrewarmSource = iota
	PrewarmSourceStatic
	PrewarmSourcePredictive
	PrewarmSourceExternal
)

func (ps PrewarmSource) String() string {
	switch ps {
	case PrewarmSourceAnalytics:
		return "analytics"
	case PrewarmSourceStatic:
		return "static"
	case PrewarmSourcePredictive:
		return "predictive"
	case PrewarmSourceExternal:
		return "external"
	default:
		return "unknown"
	}
}