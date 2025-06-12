package analytics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations

type MockCallRepository struct {
	mock.Mock
}

func (m *MockCallRepository) GetCallStats(ctx context.Context, req *CallStatsQuery) (*CallStatsResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CallStatsResult), args.Error(1)
}

func (m *MockCallRepository) GetCallVolumeByTimeRange(ctx context.Context, start, end time.Time, granularity string) ([]*TimeseriesPoint, error) {
	args := m.Called(ctx, start, end, granularity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*TimeseriesPoint), args.Error(1)
}

func (m *MockCallRepository) GetCallQualityMetrics(ctx context.Context, start, end time.Time) (*QualityMetricsResult, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*QualityMetricsResult), args.Error(1)
}

type MockBidRepository struct {
	mock.Mock
}

func (m *MockBidRepository) GetBidStats(ctx context.Context, req *BidStatsQuery) (*BidStatsResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*BidStatsResult), args.Error(1)
}

func (m *MockBidRepository) GetBidPerformanceByTimeRange(ctx context.Context, start, end time.Time) (*BidPerformanceResult, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*BidPerformanceResult), args.Error(1)
}

type MockAccountRepository struct {
	mock.Mock
}

func (m *MockAccountRepository) GetAccountStats(ctx context.Context, accountID uuid.UUID, start, end time.Time) (*AccountStatsResult, error) {
	args := m.Called(ctx, accountID, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AccountStatsResult), args.Error(1)
}

func (m *MockAccountRepository) GetTopAccountsByMetric(ctx context.Context, metric string, limit int, start, end time.Time) ([]*AccountRanking, error) {
	args := m.Called(ctx, metric, limit, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AccountRanking), args.Error(1)
}

type MockRevenueRepository struct {
	mock.Mock
}

func (m *MockRevenueRepository) GetRevenueByTimeRange(ctx context.Context, start, end time.Time, granularity string) ([]*RevenuePoint, error) {
	args := m.Called(ctx, start, end, granularity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*RevenuePoint), args.Error(1)
}

func (m *MockRevenueRepository) GetRevenueByAccount(ctx context.Context, start, end time.Time) ([]*AccountRevenue, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AccountRevenue), args.Error(1)
}

type MockMetricsRepository struct {
	mock.Mock
}

func (m *MockMetricsRepository) GetSystemMetrics(ctx context.Context, start, end time.Time) (*SystemMetricsResult, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SystemMetricsResult), args.Error(1)
}

func (m *MockMetricsRepository) GetRoutingMetrics(ctx context.Context, start, end time.Time) (*RoutingMetricsResult, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RoutingMetricsResult), args.Error(1)
}

type MockDataExporter struct {
	mock.Mock
}

func (m *MockDataExporter) ExportToCSV(ctx context.Context, data interface{}, filename string) (string, error) {
	args := m.Called(ctx, data, filename)
	return args.String(0), args.Error(1)
}

func (m *MockDataExporter) ExportToJSON(ctx context.Context, data interface{}, filename string) (string, error) {
	args := m.Called(ctx, data, filename)
	return args.String(0), args.Error(1)
}

func (m *MockDataExporter) ExportToPDF(ctx context.Context, report *Report, filename string) (string, error) {
	args := m.Called(ctx, report, filename)
	return args.String(0), args.Error(1)
}

// Tests

func TestService_GetCallMetrics(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()
	accountID := uuid.New()

	tests := []struct {
		name          string
		setupMocks    func(*MockCallRepository)
		request       *CallMetricsRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *CallMetrics)
	}{
		{
			name: "successful call metrics retrieval",
			setupMocks: func(cr *MockCallRepository) {
				callStats := &CallStats{
					Count:           1000,
					Duration:        300000,
					CompletionRate:  0.85,
					AverageDuration: 300.0,
				}
				result := &CallStatsResult{
					Stats: callStats,
					TimeSeries: []*TimeseriesPoint{
						{Timestamp: startTime, Value: 100},
						{Timestamp: endTime, Value: 150},
					},
					GroupedStats: map[string]*CallStats{
						"inbound":  {Count: 600, CompletionRate: 0.9},
						"outbound": {Count: 400, CompletionRate: 0.8},
					},
				}
				cr.On("GetCallStats", ctx, mock.MatchedBy(func(query *CallStatsQuery) bool {
					return query.AccountID != nil && *query.AccountID == accountID
				})).Return(result, nil)
			},
			request: &CallMetricsRequest{
				AccountID: &accountID,
				StartTime: startTime,
				EndTime:   endTime,
				GroupBy:   []string{"direction"},
			},
			expectedError: false,
			validate: func(t *testing.T, metrics *CallMetrics) {
				assert.NotNil(t, metrics)
				assert.Equal(t, int64(1000), metrics.TotalCalls)
				assert.Equal(t, int64(850), metrics.CompletedCalls)
				assert.Equal(t, int64(150), metrics.FailedCalls)
				assert.Equal(t, 300.0, metrics.AverageDuration)
				assert.Equal(t, 0.85, metrics.CompletionRate)
				assert.NotNil(t, metrics.ByDirection)
				assert.Len(t, metrics.ByTimeRange, 2)
			},
		},
		{
			name:          "nil request",
			setupMocks:    func(cr *MockCallRepository) {},
			request:       nil,
			expectedError: true,
			errorContains: "request cannot be nil",
		},
		{
			name:       "invalid time range",
			setupMocks: func(cr *MockCallRepository) {},
			request: &CallMetricsRequest{
				StartTime: endTime,
				EndTime:   startTime,
			},
			expectedError: true,
			errorContains: "start time must be before end time",
		},
		{
			name: "repository error",
			setupMocks: func(cr *MockCallRepository) {
				cr.On("GetCallStats", ctx, mock.Anything).Return(nil, fmt.Errorf("database error"))
			},
			request: &CallMetricsRequest{
				StartTime: startTime,
				EndTime:   endTime,
			},
			expectedError: true,
			errorContains: "failed to get call stats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(MockCallRepository)
			bidRepo := new(MockBidRepository)
			accountRepo := new(MockAccountRepository)
			revenueRepo := new(MockRevenueRepository)
			metricsRepo := new(MockMetricsRepository)
			dataExporter := new(MockDataExporter)

			// Setup mocks
			tt.setupMocks(callRepo)

			// Create service
			svc := NewService(callRepo, bidRepo, accountRepo, revenueRepo, metricsRepo, dataExporter)

			// Execute
			result, err := svc.GetCallMetrics(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			// Assert expectations
			callRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetCallVolumeStats(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-7 * 24 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name          string
		setupMocks    func(*MockCallRepository)
		request       *TimeRangeRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *CallVolumeStats)
	}{
		{
			name: "successful volume stats retrieval",
			setupMocks: func(cr *MockCallRepository) {
				volumeData := []*TimeseriesPoint{
					{Timestamp: startTime.Add(24 * time.Hour), Value: 100},
					{Timestamp: startTime.Add(48 * time.Hour), Value: 150},
					{Timestamp: startTime.Add(72 * time.Hour), Value: 200},
				}
				cr.On("GetCallVolumeByTimeRange", ctx, startTime, endTime, "day").Return(volumeData, nil)
			},
			request: &TimeRangeRequest{
				StartTime:   startTime,
				EndTime:     endTime,
				Granularity: "day",
			},
			expectedError: false,
			validate: func(t *testing.T, stats *CallVolumeStats) {
				assert.NotNil(t, stats)
				assert.Equal(t, int64(450), stats.TotalVolume)
				assert.Equal(t, int64(200), stats.PeakVolume)
				assert.Len(t, stats.VolumeByTime, 3)
				assert.True(t, stats.GrowthRate > 0)
			},
		},
		{
			name:          "nil request",
			setupMocks:    func(cr *MockCallRepository) {},
			request:       nil,
			expectedError: true,
			errorContains: "request cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(MockCallRepository)

			// Setup mocks
			tt.setupMocks(callRepo)

			// Create service
			svc := NewService(callRepo, nil, nil, nil, nil, nil)

			// Execute
			result, err := svc.GetCallVolumeStats(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			// Assert expectations
			callRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetBidMetrics(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()
	buyerID := uuid.New()

	tests := []struct {
		name          string
		setupMocks    func(*MockBidRepository)
		request       *BidMetricsRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *BidMetrics)
	}{
		{
			name: "successful bid metrics retrieval",
			setupMocks: func(br *MockBidRepository) {
				bidStats := &BidStats{
					Count:         500,
					WinCount:      170,
					WinRate:       0.34,
					TotalValue:    25000.0,
					AverageAmount: 50.0,
				}
				result := &BidStatsResult{
					Stats: bidStats,
					TimeSeries: []*TimeseriesPoint{
						{Timestamp: startTime, Value: 250},
						{Timestamp: endTime, Value: 250},
					},
					GroupedStats: map[string]*BidStats{
						buyerID.String(): bidStats,
					},
				}
				br.On("GetBidStats", ctx, mock.MatchedBy(func(query *BidStatsQuery) bool {
					return query.BuyerID != nil && *query.BuyerID == buyerID
				})).Return(result, nil)
			},
			request: &BidMetricsRequest{
				BuyerID:   &buyerID,
				StartTime: startTime,
				EndTime:   endTime,
				GroupBy:   []string{"buyer"},
			},
			expectedError: false,
			validate: func(t *testing.T, metrics *BidMetrics) {
				assert.NotNil(t, metrics)
				assert.Equal(t, int64(500), metrics.TotalBids)
				assert.Equal(t, int64(170), metrics.WinningBids)
				assert.Equal(t, 0.34, metrics.WinRate)
				assert.Equal(t, 50.0, metrics.AverageBidAmount)
				assert.Equal(t, 25000.0, metrics.TotalValue)
				assert.NotNil(t, metrics.ByBuyer)
				assert.Len(t, metrics.ByTimeRange, 2)
			},
		},
		{
			name:          "nil request",
			setupMocks:    func(br *MockBidRepository) {},
			request:       nil,
			expectedError: true,
			errorContains: "request cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			bidRepo := new(MockBidRepository)

			// Setup mocks
			tt.setupMocks(bidRepo)

			// Create service
			svc := NewService(nil, bidRepo, nil, nil, nil, nil)

			// Execute
			result, err := svc.GetBidMetrics(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			// Assert expectations
			bidRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetAccountMetrics(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	startTime := time.Now().Add(-30 * 24 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name          string
		setupMocks    func(*MockAccountRepository)
		accountID     uuid.UUID
		request       *TimeRangeRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *AccountMetrics)
	}{
		{
			name: "successful account metrics retrieval",
			setupMocks: func(ar *MockAccountRepository) {
				accountMetrics := &AccountMetrics{
					AccountID:     accountID,
					TotalCalls:    1500,
					TotalRevenue:  75000.0,
					AverageRating: 4.2,
					QualityScore:  85.5,
					CallStats: &CallStats{
						Count:           1500,
						CompletionRate:  0.88,
						AverageDuration: 280.0,
					},
					PerformanceData: []*TimeseriesPoint{
						{Timestamp: startTime, Value: 4.0},
						{Timestamp: endTime, Value: 4.4},
					},
				}
				result := &AccountStatsResult{
					AccountStats: accountMetrics,
				}
				ar.On("GetAccountStats", ctx, accountID, startTime, endTime).Return(result, nil)
			},
			accountID: accountID,
			request: &TimeRangeRequest{
				StartTime: startTime,
				EndTime:   endTime,
			},
			expectedError: false,
			validate: func(t *testing.T, metrics *AccountMetrics) {
				assert.NotNil(t, metrics)
				assert.Equal(t, accountID, metrics.AccountID)
				assert.Equal(t, int64(1500), metrics.TotalCalls)
				assert.Equal(t, 75000.0, metrics.TotalRevenue)
				assert.Equal(t, 4.2, metrics.AverageRating)
				assert.Equal(t, 85.5, metrics.QualityScore)
				assert.NotNil(t, metrics.CallStats)
				assert.Len(t, metrics.PerformanceData, 2)
			},
		},
		{
			name:          "nil account ID",
			setupMocks:    func(ar *MockAccountRepository) {},
			accountID:     uuid.Nil,
			request:       &TimeRangeRequest{StartTime: startTime, EndTime: endTime},
			expectedError: true,
			errorContains: "account ID is required",
		},
		{
			name:          "nil request",
			setupMocks:    func(ar *MockAccountRepository) {},
			accountID:     accountID,
			request:       nil,
			expectedError: true,
			errorContains: "request cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			accountRepo := new(MockAccountRepository)

			// Setup mocks
			tt.setupMocks(accountRepo)

			// Create service
			svc := NewService(nil, nil, accountRepo, nil, nil, nil)

			// Execute
			result, err := svc.GetAccountMetrics(ctx, tt.accountID, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			// Assert expectations
			accountRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetRevenueStats(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-30 * 24 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name          string
		setupMocks    func(*MockRevenueRepository)
		request       *TimeRangeRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *RevenueStats)
	}{
		{
			name: "successful revenue stats retrieval",
			setupMocks: func(rr *MockRevenueRepository) {
				revenueData := []*RevenuePoint{
					{Timestamp: startTime.Add(10 * 24 * time.Hour), Amount: 5000.0, TransactionCount: 100},
					{Timestamp: startTime.Add(20 * 24 * time.Hour), Amount: 6000.0, TransactionCount: 120},
					{Timestamp: endTime, Amount: 7000.0, TransactionCount: 140},
				}
				accountRevenue := []*AccountRevenue{
					{AccountID: uuid.New(), AccountName: "Test Account", Revenue: 18000.0, TransactionCount: 360},
				}
				rr.On("GetRevenueByTimeRange", ctx, startTime, endTime, "").Return(revenueData, nil)
				rr.On("GetRevenueByAccount", ctx, startTime, endTime).Return(accountRevenue, nil)
			},
			request: &TimeRangeRequest{
				StartTime: startTime,
				EndTime:   endTime,
			},
			expectedError: false,
			validate: func(t *testing.T, stats *RevenueStats) {
				assert.NotNil(t, stats)
				assert.Equal(t, 18000.0, stats.TotalRevenue)
				assert.True(t, stats.RevenueGrowth > 0)
				assert.Len(t, stats.RevenueByTime, 3)
				assert.Len(t, stats.RevenueByAccount, 1)
				assert.NotNil(t, stats.Projections)
				assert.True(t, stats.Projections.NextMonth > 0)
			},
		},
		{
			name:          "nil request",
			setupMocks:    func(rr *MockRevenueRepository) {},
			request:       nil,
			expectedError: true,
			errorContains: "request cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			revenueRepo := new(MockRevenueRepository)

			// Setup mocks
			tt.setupMocks(revenueRepo)

			// Create service
			svc := NewService(nil, nil, nil, revenueRepo, nil, nil)

			// Execute
			result, err := svc.GetRevenueStats(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			// Assert expectations
			revenueRepo.AssertExpectations(t)
		})
	}
}

func TestService_GenerateReport(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-7 * 24 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name          string
		request       *ReportRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *Report)
	}{
		{
			name: "successful report generation",
			request: &ReportRequest{
				Type:      "weekly",
				StartTime: startTime,
				EndTime:   endTime,
				Sections:  []string{"calls", "revenue", "performance"},
				Format:    "pdf",
			},
			expectedError: false,
			validate: func(t *testing.T, report *Report) {
				assert.NotNil(t, report)
				assert.NotEqual(t, uuid.Nil, report.ID)
				assert.Equal(t, "weekly", report.Type)
				assert.Equal(t, "completed", report.Status)
				assert.NotNil(t, report.Period)
				assert.Equal(t, startTime, report.Period.StartTime)
				assert.Equal(t, endTime, report.Period.EndTime)
				assert.NotNil(t, report.Summary)
				assert.Len(t, report.Sections, 3)
			},
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: true,
			errorContains: "request cannot be nil",
		},
		{
			name: "invalid time range",
			request: &ReportRequest{
				StartTime: endTime,
				EndTime:   startTime,
			},
			expectedError: true,
			errorContains: "start time must be before end time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service
			svc := NewService(nil, nil, nil, nil, nil, nil)

			// Execute
			result, err := svc.GenerateReport(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestService_ExportData(t *testing.T) {
	ctx := context.Background()
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name          string
		request       *ExportRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *ExportResult)
	}{
		{
			name: "successful data export",
			request: &ExportRequest{
				DataType:  "calls",
				StartTime: startTime,
				EndTime:   endTime,
				Format:    "csv",
				Columns:   []string{"id", "from", "to", "duration", "status"},
			},
			expectedError: false,
			validate: func(t *testing.T, result *ExportResult) {
				assert.NotNil(t, result)
				assert.NotEqual(t, uuid.Nil, result.ID)
				assert.Contains(t, result.Filename, "export_calls_")
				assert.Contains(t, result.Filename, ".csv")
				assert.Equal(t, "csv", result.Format)
				assert.True(t, result.ExpiresAt.After(time.Now()))
			},
		},
		{
			name:          "nil request",
			request:       nil,
			expectedError: true,
			errorContains: "request cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service
			svc := NewService(nil, nil, nil, nil, nil, nil)

			// Execute
			result, err := svc.ExportData(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// Test utility functions

func TestValidateTimeRange(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name          string
		start         time.Time
		end           time.Time
		expectedError bool
		errorContains string
	}{
		{
			name:          "valid time range",
			start:         now.Add(-24 * time.Hour),
			end:           now,
			expectedError: false,
		},
		{
			name:          "zero start time",
			start:         time.Time{},
			end:           now,
			expectedError: true,
			errorContains: "start and end times are required",
		},
		{
			name:          "zero end time",
			start:         now.Add(-24 * time.Hour),
			end:           time.Time{},
			expectedError: true,
			errorContains: "start and end times are required",
		},
		{
			name:          "start after end",
			start:         now,
			end:           now.Add(-24 * time.Hour),
			expectedError: true,
			errorContains: "start time must be before end time",
		},
		{
			name:          "range too large",
			start:         now.Add(-400 * 24 * time.Hour),
			end:           now,
			expectedError: true,
			errorContains: "time range cannot exceed 365 days",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimeRange(tt.start, tt.end)
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCalculateGrowthRate(t *testing.T) {
	tests := []struct {
		name     string
		data     []*TimeseriesPoint
		expected float64
	}{
		{
			name:     "empty data",
			data:     []*TimeseriesPoint{},
			expected: 0,
		},
		{
			name: "single point",
			data: []*TimeseriesPoint{
				{Value: 100},
			},
			expected: 0,
		},
		{
			name: "growth rate calculation",
			data: []*TimeseriesPoint{
				{Value: 100},
				{Value: 150},
			},
			expected: 0.5,
		},
		{
			name: "zero initial value",
			data: []*TimeseriesPoint{
				{Value: 0},
				{Value: 100},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateGrowthRate(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCacheOperations(t *testing.T) {
	// Create service
	svc := NewService(nil, nil, nil, nil, nil, nil).(*service)

	// Test cache set and get
	key := "test_key"
	value := "test_value"
	duration := time.Minute

	// Set cache
	svc.setCachedResult(key, value, duration)

	// Get from cache
	cached := svc.getCachedResult(key)
	assert.Equal(t, value, cached)

	// Test cache expiry
	svc.setCachedResult(key, value, -time.Minute) // Expired
	cached = svc.getCachedResult(key)
	assert.Nil(t, cached)
}

// Benchmarks

func BenchmarkService_GetCallMetrics(b *testing.B) {
	ctx := context.Background()
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	// Create mocks
	callRepo := new(MockCallRepository)

	// Setup mocks to always succeed
	callStats := &CallStats{
		Count:           1000,
		CompletionRate:  0.85,
		AverageDuration: 300.0,
	}
	result := &CallStatsResult{
		Stats:      callStats,
		TimeSeries: []*TimeseriesPoint{{Timestamp: startTime, Value: 100}},
	}
	callRepo.On("GetCallStats", ctx, mock.Anything).Return(result, nil)

	// Create service
	svc := NewService(callRepo, nil, nil, nil, nil, nil)

	// Create request
	req := &CallMetricsRequest{
		StartTime: startTime,
		EndTime:   endTime,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = svc.GetCallMetrics(ctx, req)
	}
}

func BenchmarkCacheOperations(b *testing.B) {
	svc := NewService(nil, nil, nil, nil, nil, nil).(*service)

	b.Run("SetCache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key_%d", i)
			svc.setCachedResult(key, "value", time.Minute)
		}
	})

	b.Run("GetCache", func(b *testing.B) {
		// Pre-populate cache
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("key_%d", i)
			svc.setCachedResult(key, "value", time.Minute)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key_%d", i%1000)
			_ = svc.getCachedResult(key)
		}
	})
}
