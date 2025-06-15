package audit

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// QueryOptimizer optimizes audit queries for performance
type QueryOptimizer struct {
	logger       *zap.Logger
	indexHints   map[string][]string
	queryStats   map[string]*QueryStats
	cacheEnabled bool
	maxQueryTime time.Duration
}

// QueryStats tracks query performance statistics
type QueryStats struct {
	QueryHash     string        `json:"query_hash"`
	ExecutionTime time.Duration `json:"execution_time"`
	RowsScanned   int64         `json:"rows_scanned"`
	RowsReturned  int64         `json:"rows_returned"`
	CacheHit      bool          `json:"cache_hit"`
	IndexUsed     string        `json:"index_used"`
	ExecuteCount  int64         `json:"execute_count"`
	LastExecuted  time.Time     `json:"last_executed"`
}

// OptimizationHint provides query optimization hints
type OptimizationHint struct {
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Suggestion  string            `json:"suggestion"`
	Impact      string            `json:"impact"`
	Metadata    map[string]string `json:"metadata"`
}

// QueryPlan represents an optimized query execution plan
type QueryPlan struct {
	OriginalQuery  string             `json:"original_query"`
	OptimizedQuery string             `json:"optimized_query"`
	EstimatedCost  int64              `json:"estimated_cost"`
	Hints          []OptimizationHint `json:"hints"`
	UseCache       bool               `json:"use_cache"`
	CacheKey       string             `json:"cache_key"`
	IndexStrategy  string             `json:"index_strategy"`
}

// NewQueryOptimizer creates a new query optimizer
func NewQueryOptimizer(logger *zap.Logger) *QueryOptimizer {
	optimizer := &QueryOptimizer{
		logger:       logger,
		indexHints:   make(map[string][]string),
		queryStats:   make(map[string]*QueryStats),
		cacheEnabled: true,
		maxQueryTime: 30 * time.Second,
	}

	// Initialize index hints for common query patterns
	optimizer.initializeIndexHints()

	return optimizer
}

// OptimizeQuery optimizes a query for performance
func (opt *QueryOptimizer) OptimizeQuery(ctx context.Context, qb *QueryBuilder) (*QueryPlan, error) {
	originalQuery, params, err := qb.BuildSelectQuery()
	if err != nil {
		return nil, errors.NewValidationError("INVALID_QUERY", "failed to build query").WithCause(err)
	}

	plan := &QueryPlan{
		OriginalQuery: originalQuery,
		Hints:         make([]OptimizationHint, 0),
	}

	// Analyze query patterns
	queryPattern := opt.analyzeQueryPattern(qb)

	// Generate cache key
	plan.CacheKey = opt.generateCacheKey(originalQuery, params)
	plan.UseCache = opt.shouldUseCache(queryPattern)

	// Optimize based on query pattern
	optimizedQuery, hints := opt.optimizeQueryByPattern(originalQuery, queryPattern, qb)
	plan.OptimizedQuery = optimizedQuery
	plan.Hints = hints

	// Estimate query cost
	plan.EstimatedCost = opt.estimateQueryCost(queryPattern, qb)

	// Suggest index strategy
	plan.IndexStrategy = opt.suggestIndexStrategy(queryPattern)

	opt.logger.Debug("Query optimized",
		zap.String("pattern", queryPattern),
		zap.Int64("estimated_cost", plan.EstimatedCost),
		zap.Bool("use_cache", plan.UseCache),
		zap.String("index_strategy", plan.IndexStrategy),
		zap.Int("hints_count", len(plan.Hints)),
	)

	return plan, nil
}

// analyzeQueryPattern identifies the query pattern for optimization
func (opt *QueryOptimizer) analyzeQueryPattern(qb *QueryBuilder) string {
	filter := qb.GetFilter()

	// Time-based patterns
	if filter.StartTime != nil && filter.EndTime != nil {
		timeDiff := filter.EndTime.Sub(*filter.StartTime)
		if timeDiff <= 24*time.Hour {
			return "time_range_day"
		} else if timeDiff <= 7*24*time.Hour {
			return "time_range_week"
		} else if timeDiff <= 30*24*time.Hour {
			return "time_range_month"
		} else {
			return "time_range_long"
		}
	}

	// Sequence-based patterns
	if filter.StartSequence != nil && filter.EndSequence != nil {
		seqDiff := int64(*filter.EndSequence) - int64(*filter.StartSequence)
		if seqDiff <= 1000 {
			return "sequence_range_small"
		} else if seqDiff <= 10000 {
			return "sequence_range_medium"
		} else {
			return "sequence_range_large"
		}
	}

	// Actor-based patterns
	if len(filter.ActorIDs) > 0 {
		if len(filter.ActorIDs) == 1 {
			return "single_actor"
		} else if len(filter.ActorIDs) <= 10 {
			return "multi_actor_small"
		} else {
			return "multi_actor_large"
		}
	}

	// Event type patterns
	if len(filter.Types) > 0 {
		if len(filter.Types) == 1 {
			return "single_event_type"
		} else {
			return "multi_event_type"
		}
	}

	// Compliance patterns
	if len(filter.ComplianceFlags) > 0 {
		return "compliance_query"
	}

	// Default pattern
	return "general_query"
}

// optimizeQueryByPattern applies pattern-specific optimizations
func (opt *QueryOptimizer) optimizeQueryByPattern(originalQuery, pattern string, qb *QueryBuilder) (string, []OptimizationHint) {
	optimizedQuery := originalQuery
	hints := make([]OptimizationHint, 0)

	switch pattern {
	case "time_range_day":
		// For recent data, use timestamp index
		if !strings.Contains(optimizedQuery, "/*+ INDEX") {
			optimizedQuery = strings.Replace(optimizedQuery, "FROM audit_events",
				"FROM audit_events /*+ INDEX(audit_events_timestamp_idx) */", 1)
		}
		hints = append(hints, OptimizationHint{
			Type:        "INDEX_HINT",
			Description: "Using timestamp index for recent time range",
			Suggestion:  "Query optimized for recent data access",
			Impact:      "HIGH",
		})

	case "time_range_long":
		// For older data, suggest partitioning
		hints = append(hints, OptimizationHint{
			Type:        "PARTITIONING",
			Description: "Large time range detected",
			Suggestion:  "Consider using partition pruning or archival",
			Impact:      "MEDIUM",
		})

	case "sequence_range_small":
		// For small sequence ranges, use sequence index
		if !strings.Contains(optimizedQuery, "sequence_num") {
			optimizedQuery = strings.Replace(optimizedQuery, "FROM audit_events",
				"FROM audit_events /*+ INDEX(audit_events_sequence_idx) */", 1)
		}
		hints = append(hints, OptimizationHint{
			Type:        "INDEX_HINT",
			Description: "Using sequence number index for small range",
			Suggestion:  "Sequence-based access is optimal",
			Impact:      "HIGH",
		})

	case "single_actor":
		// For single actor queries, use actor index
		optimizedQuery = strings.Replace(optimizedQuery, "FROM audit_events",
			"FROM audit_events /*+ INDEX(audit_events_actor_idx) */", 1)
		hints = append(hints, OptimizationHint{
			Type:        "INDEX_HINT",
			Description: "Using actor index for single actor query",
			Suggestion:  "Single actor access is optimized",
			Impact:      "HIGH",
		})

	case "multi_actor_large":
		// For many actors, suggest using IN clause optimization
		hints = append(hints, OptimizationHint{
			Type:        "QUERY_REWRITE",
			Description: "Large number of actors detected",
			Suggestion:  "Consider breaking query into smaller batches",
			Impact:      "MEDIUM",
		})

	case "compliance_query":
		// For compliance queries, use compliance index
		optimizedQuery = strings.Replace(optimizedQuery, "FROM audit_events",
			"FROM audit_events /*+ INDEX(audit_events_compliance_idx) */", 1)
		hints = append(hints, OptimizationHint{
			Type:        "INDEX_HINT",
			Description: "Using compliance index for regulatory queries",
			Suggestion:  "Compliance data access is optimized",
			Impact:      "HIGH",
		})

		// Add compliance-specific optimizations
		if strings.Contains(optimizedQuery, "compliance_flags") {
			hints = append(hints, OptimizationHint{
				Type:        "JSON_OPTIMIZATION",
				Description: "JSON compliance flags detected",
				Suggestion:  "Consider using GIN index on compliance_flags",
				Impact:      "MEDIUM",
			})
		}
	}

	// General optimizations

	// Add LIMIT hint if missing and result set might be large
	if !strings.Contains(optimizedQuery, "LIMIT") && !strings.Contains(optimizedQuery, "COUNT") {
		hints = append(hints, OptimizationHint{
			Type:        "PAGINATION",
			Description: "No LIMIT clause detected",
			Suggestion:  "Consider adding LIMIT to prevent large result sets",
			Impact:      "MEDIUM",
		})
	}

	// Check for potential inefficient patterns
	if strings.Contains(optimizedQuery, "metadata") && !strings.Contains(optimizedQuery, "INDEX") {
		hints = append(hints, OptimizationHint{
			Type:        "JSON_ACCESS",
			Description: "JSON metadata access detected",
			Suggestion:  "Consider creating functional indexes on frequently accessed metadata fields",
			Impact:      "LOW",
		})
	}

	return optimizedQuery, hints
}

// estimateQueryCost estimates the cost of executing a query
func (opt *QueryOptimizer) estimateQueryCost(pattern string, qb *QueryBuilder) int64 {
	baseCost := int64(100) // Base cost for any query

	filter := qb.GetFilter()

	// Time range cost
	if filter.StartTime != nil && filter.EndTime != nil {
		timeDiff := filter.EndTime.Sub(*filter.StartTime)
		days := int64(timeDiff.Hours() / 24)
		baseCost += days * 10 // 10 cost units per day
	}

	// Sequence range cost
	if filter.StartSequence != nil && filter.EndSequence != nil {
		seqDiff := int64(*filter.EndSequence) - int64(*filter.StartSequence)
		baseCost += seqDiff / 1000 // 1 cost unit per 1000 sequences
	}

	// Actor count cost
	if len(filter.ActorIDs) > 0 {
		baseCost += int64(len(filter.ActorIDs)) * 5 // 5 cost units per actor
	}

	// Event type cost
	if len(filter.Types) > 0 {
		baseCost += int64(len(filter.Types)) * 2 // 2 cost units per type
	}

	// Compliance flags cost (JSON queries are more expensive)
	if len(filter.ComplianceFlags) > 0 {
		baseCost += int64(len(filter.ComplianceFlags)) * 20 // 20 cost units per flag
	}

	// Data classes cost (array operations)
	if len(filter.DataClasses) > 0 {
		baseCost += int64(len(filter.DataClasses)) * 15 // 15 cost units per class
	}

	// Limit reduces cost
	if qb.limit > 0 && qb.limit < 1000 {
		baseCost = baseCost / 2 // Half cost for limited queries
	}

	// Pattern-specific adjustments
	switch pattern {
	case "time_range_day":
		baseCost = baseCost * 8 / 10 // 20% reduction for recent data
	case "time_range_long":
		baseCost = baseCost * 15 / 10 // 50% increase for old data
	case "sequence_range_small":
		baseCost = baseCost * 7 / 10 // 30% reduction for small ranges
	case "compliance_query":
		baseCost = baseCost * 12 / 10 // 20% increase for compliance queries
	}

	return baseCost
}

// shouldUseCache determines if a query should use caching
func (opt *QueryOptimizer) shouldUseCache(pattern string) bool {
	if !opt.cacheEnabled {
		return false
	}

	// Cache patterns that are frequently accessed
	cachePatterns := map[string]bool{
		"single_actor":         true,
		"single_event_type":    true,
		"time_range_day":       true,
		"sequence_range_small": true,
		"compliance_query":     true,
	}

	return cachePatterns[pattern]
}

// generateCacheKey generates a cache key for the query
func (opt *QueryOptimizer) generateCacheKey(query string, params map[string]interface{}) string {
	// Create a deterministic key based on query and parameters
	key := fmt.Sprintf("query:%s", hashString(query))

	// Add sorted parameters to ensure consistent key
	if len(params) > 0 {
		var paramKeys []string
		for k := range params {
			paramKeys = append(paramKeys, k)
		}
		sort.Strings(paramKeys)

		paramStr := ""
		for _, k := range paramKeys {
			paramStr += fmt.Sprintf("%s:%v", k, params[k])
		}
		key += fmt.Sprintf(":params:%s", hashString(paramStr))
	}

	return key
}

// suggestIndexStrategy suggests the best index strategy for the query
func (opt *QueryOptimizer) suggestIndexStrategy(pattern string) string {
	strategies := map[string]string{
		"time_range_day":        "timestamp_index",
		"time_range_week":       "timestamp_index",
		"time_range_month":      "timestamp_index",
		"time_range_long":       "partition_pruning",
		"sequence_range_small":  "sequence_index",
		"sequence_range_medium": "sequence_index",
		"sequence_range_large":  "sequence_partition",
		"single_actor":          "actor_index",
		"multi_actor_small":     "actor_index",
		"multi_actor_large":     "batch_processing",
		"single_event_type":     "type_index",
		"multi_event_type":      "type_index",
		"compliance_query":      "compliance_gin_index",
		"general_query":         "composite_index",
	}

	strategy, exists := strategies[pattern]
	if !exists {
		return "default_index"
	}

	return strategy
}

// RecordQueryExecution records query execution statistics
func (opt *QueryOptimizer) RecordQueryExecution(ctx context.Context, queryHash string,
	executionTime time.Duration, rowsScanned, rowsReturned int64, cacheHit bool, indexUsed string) {

	stats, exists := opt.queryStats[queryHash]
	if !exists {
		stats = &QueryStats{
			QueryHash: queryHash,
		}
		opt.queryStats[queryHash] = stats
	}

	// Update statistics
	stats.ExecutionTime = executionTime
	stats.RowsScanned = rowsScanned
	stats.RowsReturned = rowsReturned
	stats.CacheHit = cacheHit
	stats.IndexUsed = indexUsed
	stats.ExecuteCount++
	stats.LastExecuted = time.Now()

	// Log slow queries
	if executionTime > opt.maxQueryTime {
		opt.logger.Warn("Slow query detected",
			zap.String("query_hash", queryHash),
			zap.Duration("execution_time", executionTime),
			zap.Int64("rows_scanned", rowsScanned),
			zap.Int64("rows_returned", rowsReturned),
			zap.String("index_used", indexUsed),
		)
	}
}

// GetQueryStats returns query execution statistics
func (opt *QueryOptimizer) GetQueryStats(queryHash string) *QueryStats {
	stats, exists := opt.queryStats[queryHash]
	if !exists {
		return nil
	}

	// Return a copy to prevent modification
	statsCopy := *stats
	return &statsCopy
}

// AnalyzeQueryPerformance analyzes overall query performance
func (opt *QueryOptimizer) AnalyzeQueryPerformance() *QueryPerformanceAnalysis {
	analysis := &QueryPerformanceAnalysis{
		TotalQueries:    int64(len(opt.queryStats)),
		SlowQueries:     0,
		CacheHitRatio:   0,
		AverageLatency:  0,
		TopSlowQueries:  make([]*QueryStats, 0),
		Recommendations: make([]string, 0),
	}

	if analysis.TotalQueries == 0 {
		return analysis
	}

	var totalTime time.Duration
	var cacheHits int64
	var slowQueries []*QueryStats

	for _, stats := range opt.queryStats {
		totalTime += stats.ExecutionTime

		if stats.CacheHit {
			cacheHits++
		}

		if stats.ExecutionTime > opt.maxQueryTime {
			analysis.SlowQueries++
			slowQueries = append(slowQueries, stats)
		}
	}

	// Calculate metrics
	analysis.AverageLatency = totalTime / time.Duration(analysis.TotalQueries)
	if analysis.TotalQueries > 0 {
		analysis.CacheHitRatio = float64(cacheHits) / float64(analysis.TotalQueries)
	}

	// Sort slow queries by execution time
	sort.Slice(slowQueries, func(i, j int) bool {
		return slowQueries[i].ExecutionTime > slowQueries[j].ExecutionTime
	})

	// Take top 10 slow queries
	if len(slowQueries) > 10 {
		analysis.TopSlowQueries = slowQueries[:10]
	} else {
		analysis.TopSlowQueries = slowQueries
	}

	// Generate recommendations
	if analysis.CacheHitRatio < 0.3 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Low cache hit ratio detected. Consider enabling caching for frequent queries.")
	}

	if analysis.SlowQueries > analysis.TotalQueries/4 {
		analysis.Recommendations = append(analysis.Recommendations,
			"High number of slow queries. Consider adding indexes or optimizing query patterns.")
	}

	if analysis.AverageLatency > 5*time.Second {
		analysis.Recommendations = append(analysis.Recommendations,
			"High average latency. Consider query optimization or data archival.")
	}

	return analysis
}

// QueryPerformanceAnalysis represents query performance analysis results
type QueryPerformanceAnalysis struct {
	TotalQueries    int64         `json:"total_queries"`
	SlowQueries     int64         `json:"slow_queries"`
	CacheHitRatio   float64       `json:"cache_hit_ratio"`
	AverageLatency  time.Duration `json:"average_latency"`
	TopSlowQueries  []*QueryStats `json:"top_slow_queries"`
	Recommendations []string      `json:"recommendations"`
}

// initializeIndexHints initializes index hints for common patterns
func (opt *QueryOptimizer) initializeIndexHints() {
	opt.indexHints["timestamp"] = []string{
		"audit_events_timestamp_idx",
		"audit_events_timestamp_type_idx",
	}

	opt.indexHints["actor"] = []string{
		"audit_events_actor_idx",
		"audit_events_actor_timestamp_idx",
	}

	opt.indexHints["sequence"] = []string{
		"audit_events_sequence_idx",
		"audit_events_sequence_hash_idx",
	}

	opt.indexHints["compliance"] = []string{
		"audit_events_compliance_gin_idx",
		"audit_events_compliance_btree_idx",
	}

	opt.indexHints["type"] = []string{
		"audit_events_type_idx",
		"audit_events_type_timestamp_idx",
	}
}

// hashString creates a simple hash of a string for cache keys
func hashString(s string) string {
	// Simple hash implementation - in production, use a proper hash function
	hash := uint32(0)
	for _, char := range s {
		hash = hash*31 + uint32(char)
	}
	return fmt.Sprintf("%x", hash)
}
