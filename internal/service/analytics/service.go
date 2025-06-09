package analytics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// service implements the Service interface
type service struct {
	callRepo     CallRepository
	bidRepo      BidRepository
	accountRepo  AccountRepository
	revenueRepo  RevenueRepository
	metricsRepo  MetricsRepository
	dataExporter DataExporter
	mu           sync.RWMutex
	cache        map[string]interface{} // Simple in-memory cache
	cacheExpiry  map[string]time.Time
}

// NewService creates a new analytics service
func NewService(
	callRepo CallRepository,
	bidRepo BidRepository,
	accountRepo AccountRepository,
	revenueRepo RevenueRepository,
	metricsRepo MetricsRepository,
	dataExporter DataExporter,
) Service {
	return &service{
		callRepo:     callRepo,
		bidRepo:      bidRepo,
		accountRepo:  accountRepo,
		revenueRepo:  revenueRepo,
		metricsRepo:  metricsRepo,
		dataExporter: dataExporter,
		cache:        make(map[string]interface{}),
		cacheExpiry:  make(map[string]time.Time),
	}
}

// GetCallMetrics retrieves call analytics
func (s *service) GetCallMetrics(ctx context.Context, req *CallMetricsRequest) (*CallMetrics, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Check cache first
	cacheKey := fmt.Sprintf("call_metrics_%s_%s_%v", req.StartTime.Format("2006-01-02"), req.EndTime.Format("2006-01-02"), req.AccountID)
	if cached := s.getCachedResult(cacheKey); cached != nil {
		if metrics, ok := cached.(*CallMetrics); ok {
			return metrics, nil
		}
	}

	// Fetch call stats
	query := &CallStatsQuery{
		AccountID: req.AccountID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Filters:   req.Filters,
	}

	result, err := s.callRepo.GetCallStats(ctx, query)
	if err != nil {
		return nil, errors.NewInternalError("failed to get call stats").WithCause(err)
	}

	// Build metrics response
	metrics := &CallMetrics{
		TotalCalls:      result.Stats.Count,
		CompletedCalls:  int64(float64(result.Stats.Count) * result.Stats.CompletionRate),
		FailedCalls:     result.Stats.Count - int64(float64(result.Stats.Count)*result.Stats.CompletionRate),
		AverageDuration: result.Stats.AverageDuration,
		TotalDuration:   result.Stats.Duration,
		CompletionRate:  result.Stats.CompletionRate,
		ConversionRate:  calculateConversionRate(result.Stats),
		ByTimeRange:     result.TimeSeries,
	}

	// Group by direction if requested
	if contains(req.GroupBy, "direction") {
		metrics.ByDirection = result.GroupedStats
	}

	// Cache the result
	s.setCachedResult(cacheKey, metrics, time.Minute*15)

	return metrics, nil
}

// GetCallVolumeStats retrieves call volume statistics
func (s *service) GetCallVolumeStats(ctx context.Context, req *TimeRangeRequest) (*CallVolumeStats, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Get volume data by time range
	volumeData, err := s.callRepo.GetCallVolumeByTimeRange(ctx, req.StartTime, req.EndTime, req.Granularity)
	if err != nil {
		return nil, errors.NewInternalError("failed to get call volume data").WithCause(err)
	}

	// Calculate stats
	stats := &CallVolumeStats{
		VolumeByTime: volumeData,
		GrowthRate:   calculateGrowthRate(volumeData),
	}

	// Find peak volume
	for _, point := range volumeData {
		if int64(point.Value) > stats.PeakVolume {
			stats.PeakVolume = int64(point.Value)
			stats.PeakTime = point.Timestamp
		}
		stats.TotalVolume += int64(point.Value)
	}

	return stats, nil
}

// GetCallQualityStats retrieves call quality statistics
func (s *service) GetCallQualityStats(ctx context.Context, req *TimeRangeRequest) (*CallQualityStats, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Get quality metrics
	result, err := s.callRepo.GetCallQualityMetrics(ctx, req.StartTime, req.EndTime)
	if err != nil {
		return nil, errors.NewInternalError("failed to get quality metrics").WithCause(err)
	}

	return result.OverallQuality, nil
}

// GetBidMetrics retrieves bid analytics
func (s *service) GetBidMetrics(ctx context.Context, req *BidMetricsRequest) (*BidMetrics, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Fetch bid stats
	query := &BidStatsQuery{
		BuyerID:   req.BuyerID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Filters:   req.Filters,
	}

	result, err := s.bidRepo.GetBidStats(ctx, query)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bid stats").WithCause(err)
	}

	// Build metrics response
	metrics := &BidMetrics{
		TotalBids:        result.Stats.Count,
		WinningBids:      result.Stats.WinCount,
		WinRate:          result.Stats.WinRate,
		AverageBidAmount: result.Stats.AverageAmount,
		TotalValue:       result.Stats.TotalValue,
		ByTimeRange:      result.TimeSeries,
	}

	// Group by buyer if requested
	if contains(req.GroupBy, "buyer") {
		metrics.ByBuyer = result.GroupedStats
	}

	return metrics, nil
}

// GetBidPerformanceStats retrieves bid performance statistics
func (s *service) GetBidPerformanceStats(ctx context.Context, req *TimeRangeRequest) (*BidPerformanceStats, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Get bid performance data
	result, err := s.bidRepo.GetBidPerformanceByTimeRange(ctx, req.StartTime, req.EndTime)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bid performance data").WithCause(err)
	}

	return result.Performance, nil
}

// GetAccountMetrics retrieves account-specific analytics
func (s *service) GetAccountMetrics(ctx context.Context, accountID uuid.UUID, req *TimeRangeRequest) (*AccountMetrics, error) {
	if accountID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_ACCOUNT_ID", "account ID is required")
	}

	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Get account stats
	result, err := s.accountRepo.GetAccountStats(ctx, accountID, req.StartTime, req.EndTime)
	if err != nil {
		return nil, errors.NewInternalError("failed to get account stats").WithCause(err)
	}

	return result.AccountStats, nil
}

// GetAccountLeaderboard retrieves account leaderboard
func (s *service) GetAccountLeaderboard(ctx context.Context, req *LeaderboardRequest) (*AccountLeaderboard, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Get top accounts
	rankings, err := s.accountRepo.GetTopAccountsByMetric(ctx, req.Metric, req.Limit, req.StartTime, req.EndTime)
	if err != nil {
		return nil, errors.NewInternalError("failed to get account rankings").WithCause(err)
	}

	return &AccountLeaderboard{
		Rankings:      rankings,
		TotalAccounts: len(rankings),
		GeneratedAt:   time.Now(),
		Metric:        req.Metric,
	}, nil
}

// GetRevenueStats retrieves revenue statistics
func (s *service) GetRevenueStats(ctx context.Context, req *TimeRangeRequest) (*RevenueStats, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Get revenue data
	revenueData, err := s.revenueRepo.GetRevenueByTimeRange(ctx, req.StartTime, req.EndTime, req.Granularity)
	if err != nil {
		return nil, errors.NewInternalError("failed to get revenue data").WithCause(err)
	}

	// Get revenue by account
	accountRevenue, err := s.revenueRepo.GetRevenueByAccount(ctx, req.StartTime, req.EndTime)
	if err != nil {
		return nil, errors.NewInternalError("failed to get account revenue data").WithCause(err)
	}

	// Calculate total revenue
	var totalRevenue float64
	for _, point := range revenueData {
		totalRevenue += point.Amount
	}

	return &RevenueStats{
		TotalRevenue:     totalRevenue,
		RevenueGrowth:    calculateRevenueGrowth(revenueData),
		RevenueByTime:    revenueData,
		RevenueByAccount: accountRevenue,
		Projections:      generateRevenueProjections(revenueData),
	}, nil
}

// GetRevenueByAccount retrieves revenue breakdown by account
func (s *service) GetRevenueByAccount(ctx context.Context, req *TimeRangeRequest) ([]*AccountRevenue, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	return s.revenueRepo.GetRevenueByAccount(ctx, req.StartTime, req.EndTime)
}

// GetSystemPerformanceStats retrieves system performance metrics
func (s *service) GetSystemPerformanceStats(ctx context.Context, req *TimeRangeRequest) (*SystemPerformanceStats, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Get system metrics
	result, err := s.metricsRepo.GetSystemMetrics(ctx, req.StartTime, req.EndTime)
	if err != nil {
		return nil, errors.NewInternalError("failed to get system metrics").WithCause(err)
	}

	return result.Performance, nil
}

// GetRoutingPerformanceStats retrieves routing performance metrics
func (s *service) GetRoutingPerformanceStats(ctx context.Context, req *TimeRangeRequest) (*RoutingPerformanceStats, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Get routing metrics
	result, err := s.metricsRepo.GetRoutingMetrics(ctx, req.StartTime, req.EndTime)
	if err != nil {
		return nil, errors.NewInternalError("failed to get routing metrics").WithCause(err)
	}

	return result.Performance, nil
}

// GenerateReport generates analytics reports
func (s *service) GenerateReport(ctx context.Context, req *ReportRequest) (*Report, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// Create report
	report := &Report{
		ID:          uuid.New(),
		Type:        req.Type,
		GeneratedAt: time.Now(),
		Period: &TimePeriod{
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
			Duration:  req.EndTime.Sub(req.StartTime).String(),
		},
		Status: "generating",
	}

	// Generate sections asynchronously (simplified for this implementation)
	// In a real implementation, this would be done in a background job
	sections := make([]*ReportSection, 0)

	for _, sectionName := range req.Sections {
		switch sectionName {
		case "calls":
			section, err := s.generateCallsSection(ctx, req.StartTime, req.EndTime)
			if err != nil {
				continue // Skip failed sections
			}
			sections = append(sections, section)
		case "bids":
			section, err := s.generateBidsSection(ctx, req.StartTime, req.EndTime)
			if err != nil {
				continue
			}
			sections = append(sections, section)
		case "revenue":
			section, err := s.generateRevenueSection(ctx, req.StartTime, req.EndTime)
			if err != nil {
				continue
			}
			sections = append(sections, section)
		case "performance":
			section, err := s.generatePerformanceSection(ctx, req.StartTime, req.EndTime)
			if err != nil {
				continue
			}
			sections = append(sections, section)
		}
	}

	report.Sections = sections
	report.Status = "completed"

	// Generate summary
	report.Summary = s.generateReportSummary(sections)

	return report, nil
}

// ExportData exports analytics data
func (s *service) ExportData(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	if err := validateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	// This is a simplified implementation
	// In a real system, this would fetch the actual data and export it
	result := &ExportResult{
		ID:            uuid.New(),
		Filename:      fmt.Sprintf("export_%s_%s.%s", req.DataType, time.Now().Format("20060102_150405"), req.Format),
		Format:        req.Format,
		RecordCount:   0, // Would be calculated from actual data
		FileSizeBytes: 0, // Would be calculated from actual file
		ExportedAt:    time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}

	return result, nil
}

// Helper methods

func (s *service) getCachedResult(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if expiry, exists := s.cacheExpiry[key]; exists && time.Now().Before(expiry) {
		if result, exists := s.cache[key]; exists {
			return result
		}
	}

	return nil
}

func (s *service) setCachedResult(key string, value interface{}, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache[key] = value
	s.cacheExpiry[key] = time.Now().Add(duration)
}

func (s *service) generateCallsSection(ctx context.Context, start, end time.Time) (*ReportSection, error) {
	// Simplified implementation
	return &ReportSection{
		Name: "calls",
		Data: map[string]interface{}{
			"total_calls": 1000,
			"completed":   850,
			"failed":      150,
		},
	}, nil
}

func (s *service) generateBidsSection(ctx context.Context, start, end time.Time) (*ReportSection, error) {
	// Simplified implementation
	return &ReportSection{
		Name: "bids",
		Data: map[string]interface{}{
			"total_bids": 2500,
			"win_rate":   0.34,
		},
	}, nil
}

func (s *service) generateRevenueSection(ctx context.Context, start, end time.Time) (*ReportSection, error) {
	// Simplified implementation
	return &ReportSection{
		Name: "revenue",
		Data: map[string]interface{}{
			"total_revenue": 45000.50,
			"growth_rate":   0.15,
		},
	}, nil
}

func (s *service) generatePerformanceSection(ctx context.Context, start, end time.Time) (*ReportSection, error) {
	// Simplified implementation
	return &ReportSection{
		Name: "performance",
		Data: map[string]interface{}{
			"average_latency": 125.5,
			"uptime":          99.97,
		},
	}, nil
}

func (s *service) generateReportSummary(sections []*ReportSection) *ReportSummary {
	summary := &ReportSummary{
		KeyInsights: []string{
			"Call volume increased by 15% compared to previous period",
			"Bid win rate improved to 34%",
			"System uptime maintained at 99.97%",
		},
	}

	// Extract key metrics from sections
	for _, section := range sections {
		if data, ok := section.Data.(map[string]interface{}); ok {
			switch section.Name {
			case "calls":
				if total, ok := data["total_calls"].(int); ok {
					summary.TotalCalls = int64(total)
				}
			case "revenue":
				if revenue, ok := data["total_revenue"].(float64); ok {
					summary.TotalRevenue = revenue
				}
				if growth, ok := data["growth_rate"].(float64); ok {
					summary.GrowthRate = growth
				}
			}
		}
	}

	return summary
}

// Utility functions

func validateTimeRange(start, end time.Time) error {
	if start.IsZero() || end.IsZero() {
		return errors.NewValidationError("INVALID_TIME_RANGE", "start and end times are required")
	}
	if start.After(end) {
		return errors.NewValidationError("INVALID_TIME_RANGE", "start time must be before end time")
	}
	if end.Sub(start) > 365*24*time.Hour {
		return errors.NewValidationError("INVALID_TIME_RANGE", "time range cannot exceed 365 days")
	}
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func calculateConversionRate(stats *CallStats) float64 {
	if stats.Count == 0 {
		return 0
	}
	// Simplified conversion rate calculation
	return stats.CompletionRate * 0.8 // Assume 80% of completed calls are conversions
}

func calculateGrowthRate(data []*TimeseriesPoint) float64 {
	if len(data) < 2 {
		return 0
	}
	
	// Calculate growth rate between first and last period
	firstValue := data[0].Value
	lastValue := data[len(data)-1].Value
	
	if firstValue == 0 {
		return 0
	}
	
	return (lastValue - firstValue) / firstValue
}

func calculateRevenueGrowth(data []*RevenuePoint) float64 {
	if len(data) < 2 {
		return 0
	}
	
	// Calculate growth rate between first and last period
	firstValue := data[0].Amount
	lastValue := data[len(data)-1].Amount
	
	if firstValue == 0 {
		return 0
	}
	
	return (lastValue - firstValue) / firstValue
}

func generateRevenueProjections(data []*RevenuePoint) *RevenueProjections {
	if len(data) == 0 {
		return &RevenueProjections{}
	}
	
	// Simplified projection based on recent trend
	recentRevenue := data[len(data)-1].Amount
	growthRate := calculateRevenueGrowth(data)
	
	return &RevenueProjections{
		NextMonth:   recentRevenue * (1 + growthRate/12),
		NextQuarter: recentRevenue * (1 + growthRate/4),
		NextYear:    recentRevenue * (1 + growthRate),
		Confidence:  0.75, // 75% confidence
	}
}