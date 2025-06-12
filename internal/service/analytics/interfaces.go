package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service defines the analytics service interface
type Service interface {
	// Call Analytics
	GetCallMetrics(ctx context.Context, req *CallMetricsRequest) (*CallMetrics, error)
	GetCallVolumeStats(ctx context.Context, req *TimeRangeRequest) (*CallVolumeStats, error)
	GetCallQualityStats(ctx context.Context, req *TimeRangeRequest) (*CallQualityStats, error)

	// Bid Analytics
	GetBidMetrics(ctx context.Context, req *BidMetricsRequest) (*BidMetrics, error)
	GetBidPerformanceStats(ctx context.Context, req *TimeRangeRequest) (*BidPerformanceStats, error)

	// Account Analytics
	GetAccountMetrics(ctx context.Context, accountID uuid.UUID, req *TimeRangeRequest) (*AccountMetrics, error)
	GetAccountLeaderboard(ctx context.Context, req *LeaderboardRequest) (*AccountLeaderboard, error)

	// Revenue Analytics
	GetRevenueStats(ctx context.Context, req *TimeRangeRequest) (*RevenueStats, error)
	GetRevenueByAccount(ctx context.Context, req *TimeRangeRequest) ([]*AccountRevenue, error)

	// Performance Analytics
	GetSystemPerformanceStats(ctx context.Context, req *TimeRangeRequest) (*SystemPerformanceStats, error)
	GetRoutingPerformanceStats(ctx context.Context, req *TimeRangeRequest) (*RoutingPerformanceStats, error)

	// Reports
	GenerateReport(ctx context.Context, req *ReportRequest) (*Report, error)
	ExportData(ctx context.Context, req *ExportRequest) (*ExportResult, error)
}

// Repository interfaces for data access
type CallRepository interface {
	GetCallStats(ctx context.Context, req *CallStatsQuery) (*CallStatsResult, error)
	GetCallVolumeByTimeRange(ctx context.Context, start, end time.Time, granularity string) ([]*TimeseriesPoint, error)
	GetCallQualityMetrics(ctx context.Context, start, end time.Time) (*QualityMetricsResult, error)
}

type BidRepository interface {
	GetBidStats(ctx context.Context, req *BidStatsQuery) (*BidStatsResult, error)
	GetBidPerformanceByTimeRange(ctx context.Context, start, end time.Time) (*BidPerformanceResult, error)
}

type AccountRepository interface {
	GetAccountStats(ctx context.Context, accountID uuid.UUID, start, end time.Time) (*AccountStatsResult, error)
	GetTopAccountsByMetric(ctx context.Context, metric string, limit int, start, end time.Time) ([]*AccountRanking, error)
}

type RevenueRepository interface {
	GetRevenueByTimeRange(ctx context.Context, start, end time.Time, granularity string) ([]*RevenuePoint, error)
	GetRevenueByAccount(ctx context.Context, start, end time.Time) ([]*AccountRevenue, error)
}

type MetricsRepository interface {
	GetSystemMetrics(ctx context.Context, start, end time.Time) (*SystemMetricsResult, error)
	GetRoutingMetrics(ctx context.Context, start, end time.Time) (*RoutingMetricsResult, error)
}

// Data storage interface for exports and reports
type DataExporter interface {
	ExportToCSV(ctx context.Context, data interface{}, filename string) (string, error)
	ExportToJSON(ctx context.Context, data interface{}, filename string) (string, error)
	ExportToPDF(ctx context.Context, report *Report, filename string) (string, error)
}

// Request types
type CallMetricsRequest struct {
	AccountID *uuid.UUID
	StartTime time.Time
	EndTime   time.Time
	GroupBy   []string // hour, day, week, month, account, direction
	Filters   map[string]interface{}
}

type BidMetricsRequest struct {
	BuyerID   *uuid.UUID
	StartTime time.Time
	EndTime   time.Time
	GroupBy   []string
	Filters   map[string]interface{}
}

type TimeRangeRequest struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity string // hour, day, week, month
	Timezone    string
}

type LeaderboardRequest struct {
	TimeRangeRequest
	Metric      string // volume, revenue, quality, conversion_rate
	Limit       int
	AccountType string // buyer, seller, all
}

type ReportRequest struct {
	Type       string // daily, weekly, monthly, custom
	StartTime  time.Time
	EndTime    time.Time
	Sections   []string // calls, bids, revenue, performance
	Recipients []string // email addresses
	Format     string   // pdf, html, csv
}

type ExportRequest struct {
	DataType  string // calls, bids, accounts, revenue
	StartTime time.Time
	EndTime   time.Time
	Format    string // csv, json
	Filters   map[string]interface{}
	Columns   []string
}

// Response types
type CallMetrics struct {
	TotalCalls      int64                 `json:"total_calls"`
	CompletedCalls  int64                 `json:"completed_calls"`
	FailedCalls     int64                 `json:"failed_calls"`
	AverageDuration float64               `json:"average_duration"`
	TotalDuration   int64                 `json:"total_duration"`
	ConversionRate  float64               `json:"conversion_rate"`
	CompletionRate  float64               `json:"completion_rate"`
	ByDirection     map[string]*CallStats `json:"by_direction,omitempty"`
	ByTimeRange     []*TimeseriesPoint    `json:"by_time_range,omitempty"`
	TopRoutes       []*RouteStats         `json:"top_routes,omitempty"`
}

type CallVolumeStats struct {
	TotalVolume    int64              `json:"total_volume"`
	PeakVolume     int64              `json:"peak_volume"`
	PeakTime       time.Time          `json:"peak_time"`
	VolumeByTime   []*TimeseriesPoint `json:"volume_by_time"`
	VolumeByRegion map[string]int64   `json:"volume_by_region"`
	GrowthRate     float64            `json:"growth_rate"`
}

type CallQualityStats struct {
	AverageRating    float64                `json:"average_rating"`
	QualityScore     float64                `json:"quality_score"`
	FraudRate        float64                `json:"fraud_rate"`
	ComplianceScore  float64                `json:"compliance_score"`
	QualityByTime    []*TimeseriesPoint     `json:"quality_by_time"`
	QualityByAccount []*AccountQualityStats `json:"quality_by_account"`
}

type BidMetrics struct {
	TotalBids        int64                `json:"total_bids"`
	WinningBids      int64                `json:"winning_bids"`
	WinRate          float64              `json:"win_rate"`
	AverageBidAmount float64              `json:"average_bid_amount"`
	TotalValue       float64              `json:"total_value"`
	ByBuyer          map[string]*BidStats `json:"by_buyer,omitempty"`
	ByTimeRange      []*TimeseriesPoint   `json:"by_time_range,omitempty"`
}

type BidPerformanceStats struct {
	BidVolume     []*TimeseriesPoint `json:"bid_volume"`
	WinRates      []*TimeseriesPoint `json:"win_rates"`
	AveragePrices []*TimeseriesPoint `json:"average_prices"`
	TopBuyers     []*BuyerStats      `json:"top_buyers"`
	Conversion    *ConversionStats   `json:"conversion"`
}

type AccountMetrics struct {
	AccountID       uuid.UUID          `json:"account_id"`
	TotalCalls      int64              `json:"total_calls"`
	TotalRevenue    float64            `json:"total_revenue"`
	AverageRating   float64            `json:"average_rating"`
	QualityScore    float64            `json:"quality_score"`
	CallStats       *CallStats         `json:"call_stats"`
	BidStats        *BidStats          `json:"bid_stats,omitempty"`
	PerformanceData []*TimeseriesPoint `json:"performance_data"`
}

type AccountLeaderboard struct {
	Rankings      []*AccountRanking `json:"rankings"`
	TotalAccounts int               `json:"total_accounts"`
	GeneratedAt   time.Time         `json:"generated_at"`
	Metric        string            `json:"metric"`
}

type RevenueStats struct {
	TotalRevenue     float64             `json:"total_revenue"`
	RevenueGrowth    float64             `json:"revenue_growth"`
	RevenueByTime    []*RevenuePoint     `json:"revenue_by_time"`
	RevenueByAccount []*AccountRevenue   `json:"revenue_by_account"`
	Projections      *RevenueProjections `json:"projections"`
}

type SystemPerformanceStats struct {
	AverageLatency  float64             `json:"average_latency"`
	ThroughputRPS   float64             `json:"throughput_rps"`
	ErrorRate       float64             `json:"error_rate"`
	UpTime          float64             `json:"uptime"`
	ResourceUsage   *ResourceUsageStats `json:"resource_usage"`
	PerformanceData []*TimeseriesPoint  `json:"performance_data"`
}

type RoutingPerformanceStats struct {
	AverageRoutingTime float64                    `json:"average_routing_time"`
	SuccessRate        float64                    `json:"success_rate"`
	AlgorithmStats     map[string]*AlgorithmStats `json:"algorithm_stats"`
	RoutingData        []*TimeseriesPoint         `json:"routing_data"`
}

type Report struct {
	ID          uuid.UUID        `json:"id"`
	Type        string           `json:"type"`
	GeneratedAt time.Time        `json:"generated_at"`
	Period      *TimePeriod      `json:"period"`
	Summary     *ReportSummary   `json:"summary"`
	Sections    []*ReportSection `json:"sections"`
	Status      string           `json:"status"`
	URL         string           `json:"url,omitempty"`
}

type ExportResult struct {
	ID            uuid.UUID `json:"id"`
	Filename      string    `json:"filename"`
	URL           string    `json:"url"`
	Format        string    `json:"format"`
	RecordCount   int64     `json:"record_count"`
	FileSizeBytes int64     `json:"file_size_bytes"`
	ExportedAt    time.Time `json:"exported_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// Supporting types
type CallStats struct {
	Count           int64   `json:"count"`
	Duration        int64   `json:"duration"`
	CompletionRate  float64 `json:"completion_rate"`
	AverageDuration float64 `json:"average_duration"`
}

type BidStats struct {
	Count         int64   `json:"count"`
	WinCount      int64   `json:"win_count"`
	WinRate       float64 `json:"win_rate"`
	TotalValue    float64 `json:"total_value"`
	AverageAmount float64 `json:"average_amount"`
}

type TimeseriesPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Value     float64                `json:"value"`
	Label     string                 `json:"label,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type RevenuePoint struct {
	Timestamp        time.Time `json:"timestamp"`
	Amount           float64   `json:"amount"`
	TransactionCount int64     `json:"transaction_count"`
}

type AccountRevenue struct {
	AccountID        uuid.UUID `json:"account_id"`
	AccountName      string    `json:"account_name"`
	Revenue          float64   `json:"revenue"`
	TransactionCount int64     `json:"transaction_count"`
	GrowthRate       float64   `json:"growth_rate"`
}

type AccountRanking struct {
	Rank        int       `json:"rank"`
	AccountID   uuid.UUID `json:"account_id"`
	AccountName string    `json:"account_name"`
	Value       float64   `json:"value"`
	Change      float64   `json:"change"`
}

type RouteStats struct {
	FromRegion   string  `json:"from_region"`
	ToRegion     string  `json:"to_region"`
	CallCount    int64   `json:"call_count"`
	Revenue      float64 `json:"revenue"`
	QualityScore float64 `json:"quality_score"`
}

type AccountQualityStats struct {
	AccountID    uuid.UUID `json:"account_id"`
	QualityScore float64   `json:"quality_score"`
	Rating       float64   `json:"rating"`
	CallCount    int64     `json:"call_count"`
}

type BuyerStats struct {
	BuyerID    uuid.UUID `json:"buyer_id"`
	BuyerName  string    `json:"buyer_name"`
	BidCount   int64     `json:"bid_count"`
	WinCount   int64     `json:"win_count"`
	WinRate    float64   `json:"win_rate"`
	TotalSpent float64   `json:"total_spent"`
}

type ConversionStats struct {
	BidToCall      float64 `json:"bid_to_call"`
	CallToComplete float64 `json:"call_to_complete"`
	Overall        float64 `json:"overall"`
}

type RevenueProjections struct {
	NextMonth   float64 `json:"next_month"`
	NextQuarter float64 `json:"next_quarter"`
	NextYear    float64 `json:"next_year"`
	Confidence  float64 `json:"confidence"`
}

type ResourceUsageStats struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetworkIO   float64 `json:"network_io"`
}

type AlgorithmStats struct {
	Name           string  `json:"name"`
	UsageCount     int64   `json:"usage_count"`
	SuccessRate    float64 `json:"success_rate"`
	AverageLatency float64 `json:"average_latency"`
}

type TimePeriod struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`
}

type ReportSummary struct {
	TotalCalls   int64    `json:"total_calls"`
	TotalRevenue float64  `json:"total_revenue"`
	GrowthRate   float64  `json:"growth_rate"`
	KeyInsights  []string `json:"key_insights"`
}

type ReportSection struct {
	Name   string       `json:"name"`
	Data   interface{}  `json:"data"`
	Charts []*ChartData `json:"charts,omitempty"`
}

type ChartData struct {
	Type    string                 `json:"type"` // line, bar, pie, etc.
	Title   string                 `json:"title"`
	Data    interface{}            `json:"data"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// Query types for repositories
type CallStatsQuery struct {
	AccountID *uuid.UUID
	StartTime time.Time
	EndTime   time.Time
	Direction *string
	Status    *string
	Filters   map[string]interface{}
}

type CallStatsResult struct {
	Stats        *CallStats
	TimeSeries   []*TimeseriesPoint
	GroupedStats map[string]*CallStats
}

type BidStatsQuery struct {
	BuyerID   *uuid.UUID
	StartTime time.Time
	EndTime   time.Time
	Status    *string
	Filters   map[string]interface{}
}

type BidStatsResult struct {
	Stats        *BidStats
	TimeSeries   []*TimeseriesPoint
	GroupedStats map[string]*BidStats
}

type AccountStatsResult struct {
	AccountStats *AccountMetrics
	CallStats    *CallStats
	BidStats     *BidStats
	RevenueStats *AccountRevenue
}

type BidPerformanceResult struct {
	Performance *BidPerformanceStats
	Trends      []*TimeseriesPoint
}

type QualityMetricsResult struct {
	OverallQuality *CallQualityStats
	ByAccount      []*AccountQualityStats
	Trends         []*TimeseriesPoint
}

type SystemMetricsResult struct {
	Performance   *SystemPerformanceStats
	ResourceUsage *ResourceUsageStats
	Trends        []*TimeseriesPoint
}

type RoutingMetricsResult struct {
	Performance    *RoutingPerformanceStats
	AlgorithmStats map[string]*AlgorithmStats
	Trends         []*TimeseriesPoint
}
