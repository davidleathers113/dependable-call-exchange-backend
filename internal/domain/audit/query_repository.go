package audit

import (
	"context"
	"io"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// QueryRepository defines the interface for advanced audit event queries and reporting
// Optimized for read operations, compliance reporting, and analytics
type QueryRepository interface {
	// Advanced search operations
	
	// Search performs full-text search across audit events
	Search(ctx context.Context, criteria SearchCriteria) (*SearchResult, error)
	
	// AdvancedQuery executes complex queries with multiple filters and aggregations
	AdvancedQuery(ctx context.Context, query *AdvancedQuery) (*QueryResult, error)
	
	// TimeSeriesQuery retrieves events aggregated by time intervals
	TimeSeriesQuery(ctx context.Context, criteria TimeSeriesCriteria) (*TimeSeriesResult, error)
	
	// Aggregation operations
	
	// CountByField counts events grouped by a specific field
	CountByField(ctx context.Context, field string, filter EventFilter) (map[string]int64, error)
	
	// CountByTimeRange counts events in time buckets
	CountByTimeRange(ctx context.Context, interval time.Duration, start, end time.Time, filter EventFilter) (*TimeRangeCount, error)
	
	// CountByMultipleFields provides multi-dimensional event counting
	CountByMultipleFields(ctx context.Context, fields []string, filter EventFilter) (*MultiFieldCount, error)
	
	// GetEventTrends analyzes event patterns and trends over time
	GetEventTrends(ctx context.Context, criteria TrendCriteria) (*TrendAnalysis, error)
	
	// Compliance reporting operations
	
	// GenerateComplianceReport creates comprehensive compliance reports
	GenerateComplianceReport(ctx context.Context, criteria ComplianceReportCriteria) (*ComplianceReport, error)
	
	// GetGDPRReport generates GDPR-specific audit report
	GetGDPRReport(ctx context.Context, dataSubject string, criteria GDPRReportCriteria) (*GDPRReport, error)
	
	// GetTCPAReport generates TCPA compliance report
	GetTCPAReport(ctx context.Context, phoneNumber string, criteria TCPAReportCriteria) (*TCPAReport, error)
	
	// GetAccessAuditReport generates data access audit report
	GetAccessAuditReport(ctx context.Context, criteria AccessAuditCriteria) (*AccessAuditReport, error)
	
	// Export operations
	
	// ExportEvents exports events in specified format
	ExportEvents(ctx context.Context, filter EventFilter, format values.ExportFormat) (io.Reader, error)
	
	// ExportToStream streams export data for large datasets
	ExportToStream(ctx context.Context, filter EventFilter, format values.ExportFormat, writer io.Writer) error
	
	// ExportReport exports a pre-generated report
	ExportReport(ctx context.Context, reportID string, format values.ExportFormat) (io.Reader, error)
	
	// StreamEvents provides real-time event streaming
	StreamEvents(ctx context.Context, filter EventFilter) (<-chan *Event, error)
	
	// Performance-optimized operations
	
	// GetEventsSummary returns lightweight event summaries for dashboards
	GetEventsSummary(ctx context.Context, filter EventFilter) (*EventSummary, error)
	
	// GetEventHints returns minimal event data for quick browsing
	GetEventHints(ctx context.Context, filter EventFilter) ([]*EventHint, error)
	
	// GetActivitySummary provides activity overview for dashboards
	GetActivitySummary(ctx context.Context, timeRange TimeRange) (*ActivitySummary, error)
	
	// Analysis operations
	
	// AnalyzeEventPatterns identifies patterns and anomalies in events
	AnalyzeEventPatterns(ctx context.Context, criteria PatternCriteria) (*PatternAnalysis, error)
	
	// GetEventCorrelations finds related events based on various criteria
	GetEventCorrelations(ctx context.Context, eventID uuid.UUID, criteria CorrelationCriteria) (*CorrelationResult, error)
	
	// GetAuditTrail constructs a complete audit trail for a specific entity
	GetAuditTrail(ctx context.Context, entityID string, entityType string, criteria TrailCriteria) (*AuditTrail, error)
	
	// GetUserActivity provides comprehensive user activity analysis
	GetUserActivity(ctx context.Context, userID string, criteria ActivityCriteria) (*UserActivityReport, error)
	
	// Security and monitoring operations
	
	// DetectAnomalies identifies unusual patterns in audit events
	DetectAnomalies(ctx context.Context, criteria AnomalyCriteria) (*AnomalyReport, error)
	
	// GetSecurityEvents retrieves security-relevant events
	GetSecurityEvents(ctx context.Context, criteria SecurityEventCriteria) (*SecurityEventReport, error)
	
	// GetFailureAnalysis analyzes failure patterns and rates
	GetFailureAnalysis(ctx context.Context, criteria FailureCriteria) (*FailureAnalysis, error)
	
	// Report management operations
	
	// SaveReport saves a query result as a named report
	SaveReport(ctx context.Context, report *SavedReport) (string, error)
	
	// GetSavedReport retrieves a previously saved report
	GetSavedReport(ctx context.Context, reportID string) (*SavedReport, error)
	
	// ListSavedReports lists all saved reports with filtering
	ListSavedReports(ctx context.Context, filter SavedReportFilter) ([]*SavedReportInfo, error)
	
	// DeleteSavedReport removes a saved report
	DeleteSavedReport(ctx context.Context, reportID string) error
	
	// Scheduled reporting operations
	
	// CreateScheduledReport creates a recurring report
	CreateScheduledReport(ctx context.Context, schedule *ReportSchedule) (string, error)
	
	// GetScheduledReports lists all scheduled reports
	GetScheduledReports(ctx context.Context) ([]*ReportSchedule, error)
	
	// UpdateScheduledReport modifies a scheduled report
	UpdateScheduledReport(ctx context.Context, scheduleID string, schedule *ReportSchedule) error
	
	// DeleteScheduledReport removes a scheduled report
	DeleteScheduledReport(ctx context.Context, scheduleID string) error
	
	// Performance monitoring
	
	// GetQueryPerformance returns query performance metrics
	GetQueryPerformance(ctx context.Context) (*QueryPerformanceMetrics, error)
	
	// OptimizeQuery suggests optimizations for slow queries
	OptimizeQuery(ctx context.Context, query *AdvancedQuery) (*QueryOptimization, error)
}

// FullTextSearchCriteria defines criteria for full-text search operations
type FullTextSearchCriteria struct {
	// Search terms
	Query          string   `json:"query"`                    // Main search query
	Fields         []string `json:"fields,omitempty"`         // Fields to search in
	ExactMatch     bool     `json:"exact_match,omitempty"`    // Exact phrase matching
	CaseSensitive  bool     `json:"case_sensitive,omitempty"` // Case sensitive search
	
	// Advanced search options
	BooleanQuery   string   `json:"boolean_query,omitempty"`  // Boolean query syntax
	WildcardSearch bool     `json:"wildcard_search,omitempty"`// Allow wildcards
	FuzzySearch    bool     `json:"fuzzy_search,omitempty"`   // Fuzzy matching
	FuzzyDistance  int      `json:"fuzzy_distance,omitempty"` // Edit distance for fuzzy
	
	// Result filtering
	EventFilter    EventFilter `json:"event_filter,omitempty"`
	
	// Time range
	StartTime      *time.Time `json:"start_time,omitempty"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	
	// Result options
	HighlightMatches bool `json:"highlight_matches"`
	IncludeMetadata  bool `json:"include_metadata"`
	MaxResults       int  `json:"max_results,omitempty"`
	
	// Performance options
	Timeout        time.Duration `json:"timeout,omitempty"`
	UseCache       bool          `json:"use_cache"`
	
	// Pagination
	Limit          int    `json:"limit,omitempty"`
	Offset         int    `json:"offset,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
}

// SearchResult represents the result of a search operation
type SearchResult struct {
	Matches        []*SearchMatch `json:"matches"`
	TotalCount     int64          `json:"total_count"`
	SearchTime     time.Duration  `json:"search_time"`
	HasMore        bool           `json:"has_more"`
	NextCursor     string         `json:"next_cursor,omitempty"`
	
	// Search metadata
	Query          SearchCriteria `json:"query"`
	ExecutedAt     time.Time      `json:"executed_at"`
	CacheHit       bool           `json:"cache_hit"`
	
	// Performance information
	IndexHits      int            `json:"index_hits"`
	DatabaseHits   int            `json:"database_hits"`
}

// SearchMatch represents a single search result
type SearchMatch struct {
	Event          *Event            `json:"event"`
	Score          float64           `json:"score"`
	Highlights     map[string]string `json:"highlights,omitempty"`
	MatchedFields  []string          `json:"matched_fields"`
	Context        map[string]interface{} `json:"context,omitempty"`
}

// AdvancedQuery represents a complex query with multiple conditions
type AdvancedQuery struct {
	// Query structure
	Conditions     []*QueryCondition   `json:"conditions"`
	Aggregations   []*QueryAggregation `json:"aggregations,omitempty"`
	Grouping       *QueryGrouping      `json:"grouping,omitempty"`
	Sorting        []*QuerySort        `json:"sorting,omitempty"`
	
	// Limits and pagination
	Limit          int                 `json:"limit,omitempty"`
	Offset         int                 `json:"offset,omitempty"`
	
	// Performance options
	Timeout        time.Duration       `json:"timeout,omitempty"`
	UseCache       bool                `json:"use_cache"`
	ExplainPlan    bool                `json:"explain_plan,omitempty"`
}

// QueryCondition represents a single condition in an advanced query
type QueryCondition struct {
	Field         string                 `json:"field"`
	Operator      string                 `json:"operator"` // eq, ne, gt, lt, gte, lte, in, like, exists
	Value         interface{}            `json:"value"`
	Values        []interface{}          `json:"values,omitempty"` // For 'in' operator
	LogicalOp     string                 `json:"logical_op,omitempty"` // AND, OR
	SubConditions []*QueryCondition      `json:"sub_conditions,omitempty"`
}

// QueryAggregation represents an aggregation operation
type QueryAggregation struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"` // count, sum, avg, min, max, distinct_count
	Field         string                 `json:"field,omitempty"`
	Interval      string                 `json:"interval,omitempty"` // For date histogram
	SubAggregations []*QueryAggregation  `json:"sub_aggregations,omitempty"`
}

// QueryGrouping represents grouping options
type QueryGrouping struct {
	Fields        []string               `json:"fields"`
	Limit         int                    `json:"limit,omitempty"`
	OrderBy       string                 `json:"order_by,omitempty"`
	OrderDesc     bool                   `json:"order_desc,omitempty"`
}

// QuerySort represents sorting options
type QuerySort struct {
	Field         string                 `json:"field"`
	Descending    bool                   `json:"descending,omitempty"`
	Missing       string                 `json:"missing,omitempty"` // first, last
}

// QueryResult represents the result of an advanced query
type QueryResult struct {
	Events        []*Event               `json:"events,omitempty"`
	Aggregations  map[string]interface{} `json:"aggregations,omitempty"`
	TotalCount    int64                  `json:"total_count"`
	QueryTime     time.Duration          `json:"query_time"`
	
	// Query information
	Query         *AdvancedQuery         `json:"query"`
	ExecutedAt    time.Time              `json:"executed_at"`
	CacheHit      bool                   `json:"cache_hit"`
	
	// Performance information
	ExecutionPlan string                 `json:"execution_plan,omitempty"`
	IndexesUsed   []string               `json:"indexes_used,omitempty"`
}

// TimeSeriesCriteria defines criteria for time-series queries
type TimeSeriesCriteria struct {
	// Time configuration
	StartTime     time.Time             `json:"start_time"`
	EndTime       time.Time             `json:"end_time"`
	Interval      time.Duration         `json:"interval"` // Bucket size
	
	// Aggregation
	Aggregation   string                `json:"aggregation"` // count, avg, sum, min, max
	GroupBy       []string              `json:"group_by,omitempty"`
	
	// Filtering
	EventFilter   EventFilter           `json:"event_filter,omitempty"`
	
	// Options
	FillGaps      bool                  `json:"fill_gaps"`      // Fill missing time buckets
	FillValue     interface{}           `json:"fill_value,omitempty"` // Value for gaps
}

// TimeSeriesResult represents time-series query results
type TimeSeriesResult struct {
	Series        []*TimeSeries         `json:"series"`
	TimeRange     TimeRange             `json:"time_range"`
	Interval      time.Duration         `json:"interval"`
	QueryTime     time.Duration         `json:"query_time"`
	
	// Query metadata
	Criteria      TimeSeriesCriteria    `json:"criteria"`
	ExecutedAt    time.Time             `json:"executed_at"`
}

// TimeSeries represents a single time series
type TimeSeries struct {
	Name          string                `json:"name"`
	GroupBy       map[string]string     `json:"group_by,omitempty"`
	Points        []*TimeSeriesPoint    `json:"points"`
	TotalValue    float64               `json:"total_value"`
	AverageValue  float64               `json:"average_value"`
}

// TimeSeriesPoint represents a single point in time series
type TimeSeriesPoint struct {
	Timestamp     time.Time             `json:"timestamp"`
	Value         float64               `json:"value"`
	Count         int64                 `json:"count,omitempty"`
}

// Additional types for different kinds of reports and analysis would continue...
// Including ComplianceReport, GDPRReport, TCPAReport, etc.

// EventSummary provides a lightweight overview of events
type EventSummary struct {
	TotalEvents   int64                 `json:"total_events"`
	EventTypes    map[EventType]int64   `json:"event_types"`
	Severities    map[Severity]int64    `json:"severities"`
	Categories    map[string]int64      `json:"categories"`
	Actors        map[string]int64      `json:"actors"`
	
	// Time distribution
	Today         int64                 `json:"today"`
	ThisWeek      int64                 `json:"this_week"`
	ThisMonth     int64                 `json:"this_month"`
	
	// Error summary
	ErrorRate     float64               `json:"error_rate"`
	CriticalCount int64                 `json:"critical_count"`
	
	// Compliance summary
	GDPREvents    int64                 `json:"gdpr_events"`
	TCPAEvents    int64                 `json:"tcpa_events"`
	
	// Generated metadata
	GeneratedAt   time.Time             `json:"generated_at"`
	QueryTime     time.Duration         `json:"query_time"`
}

// EventHint provides minimal event information for quick browsing
type EventHint struct {
	ID            uuid.UUID             `json:"id"`
	Sequence      values.SequenceNumber `json:"sequence"`
	Timestamp     time.Time             `json:"timestamp"`
	Type          EventType             `json:"type"`
	Severity      Severity              `json:"severity"`
	ActorID       string                `json:"actor_id"`
	TargetID      string                `json:"target_id"`
	Action        string                `json:"action"`
	Result        string                `json:"result"`
	
	// Quick preview
	Summary       string                `json:"summary,omitempty"`
	HasErrors     bool                  `json:"has_errors"`
	IsCompliance  bool                  `json:"is_compliance"`
}

// SavedReport represents a saved query result
type SavedReport struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	
	// Query information
	Query         interface{}           `json:"query"` // Original query
	Result        interface{}           `json:"result"` // Saved result
	
	// Metadata
	CreatedBy     string                `json:"created_by"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	AccessCount   int64                 `json:"access_count"`
	LastAccessed  *time.Time            `json:"last_accessed,omitempty"`
	
	// Configuration
	ExpiresAt     *time.Time            `json:"expires_at,omitempty"`
	IsPublic      bool                  `json:"is_public"`
	Tags          []string              `json:"tags,omitempty"`
	
	// Format information
	Format        values.ExportFormat   `json:"format"`
	Size          int64                 `json:"size"`
}

// SavedReportInfo provides metadata about saved reports
type SavedReportInfo struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	CreatedBy     string                `json:"created_by"`
	CreatedAt     time.Time             `json:"created_at"`
	Size          int64                 `json:"size"`
	Format        values.ExportFormat   `json:"format"`
	Tags          []string              `json:"tags,omitempty"`
	AccessCount   int64                 `json:"access_count"`
	LastAccessed  *time.Time            `json:"last_accessed,omitempty"`
}

// SavedReportFilter defines filtering for saved reports
type SavedReportFilter struct {
	CreatedBy     string                `json:"created_by,omitempty"`
	Tags          []string              `json:"tags,omitempty"`
	CreatedAfter  *time.Time            `json:"created_after,omitempty"`
	CreatedBefore *time.Time            `json:"created_before,omitempty"`
	IsPublic      *bool                 `json:"is_public,omitempty"`
	
	// Text search
	SearchText    string                `json:"search_text,omitempty"`
	
	// Pagination
	Limit         int                   `json:"limit,omitempty"`
	Offset        int                   `json:"offset,omitempty"`
	
	// Sorting
	OrderBy       string                `json:"order_by,omitempty"`
	OrderDesc     bool                  `json:"order_desc,omitempty"`
}