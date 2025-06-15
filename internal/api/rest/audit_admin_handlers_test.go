package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	auditService "github.com/davidleathers/dependable-call-exchange-backend/internal/service/audit"
)

// Mock implementations for testing

type MockIntegrityService struct {
	mock.Mock
}

func (m *MockIntegrityService) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockIntegrityService) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockIntegrityService) VerifyHashChain(ctx context.Context, start, end values.SequenceNumber) (*audit.HashChainVerificationResult, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).(*audit.HashChainVerificationResult), args.Error(1)
}

func (m *MockIntegrityService) RepairChain(ctx context.Context, start, end values.SequenceNumber, options *auditService.RepairOptions) (*audit.HashChainRepairResult, error) {
	args := m.Called(ctx, start, end, options)
	return args.Get(0).(*audit.HashChainRepairResult), args.Error(1)
}

func (m *MockIntegrityService) VerifySequenceIntegrity(ctx context.Context, criteria audit.SequenceIntegrityCriteria) (*audit.SequenceIntegrityResult, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(*audit.SequenceIntegrityResult), args.Error(1)
}

func (m *MockIntegrityService) PerformIntegrityCheck(ctx context.Context, criteria audit.IntegrityCriteria) (*audit.IntegrityReport, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(*audit.IntegrityReport), args.Error(1)
}

func (m *MockIntegrityService) DetectCorruption(ctx context.Context, criteria audit.CorruptionDetectionCriteria) (*audit.CorruptionReport, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(*audit.CorruptionReport), args.Error(1)
}

func (m *MockIntegrityService) ScheduleIntegrityCheck(ctx context.Context, schedule *audit.IntegrityCheckSchedule) (string, error) {
	args := m.Called(ctx, schedule)
	return args.String(0), args.Error(1)
}

func (m *MockIntegrityService) GetIntegrityStatus(ctx context.Context) (*auditService.IntegrityServiceStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(*auditService.IntegrityServiceStatus), args.Error(1)
}

type MockComplianceService struct {
	mock.Mock
}

func (m *MockComplianceService) GetSystemStatus(ctx context.Context) (*ComplianceSystemStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(*ComplianceSystemStatus), args.Error(1)
}

func (m *MockComplianceService) GetStatistics(ctx context.Context, period string) (*ComplianceStatistics, error) {
	args := m.Called(ctx, period)
	return args.Get(0).(*ComplianceStatistics), args.Error(1)
}

type MockAuditLogger struct {
	mock.Mock
}

func (m *MockAuditLogger) GetStats() *auditService.LoggerStats {
	args := m.Called()
	return args.Get(0).(*auditService.LoggerStats)
}

func (m *MockAuditLogger) GetStatus() string {
	args := m.Called()
	return args.String(0)
}

// Test setup helpers

func setupAuditAdminHandler() (*AuditAdminHandler, *MockIntegrityService, *MockComplianceService, *MockAuditLogger) {
	baseHandler := NewBaseHandler("v1", "https://api.test.com")
	
	mockIntegrity := &MockIntegrityService{}
	mockCompliance := &MockComplianceService{}
	mockLogger := &MockAuditLogger{}
	
	handler := NewAuditAdminHandler(baseHandler, mockIntegrity, mockCompliance, mockLogger)
	
	return handler, mockIntegrity, mockCompliance, mockLogger
}

// Test cases

func TestAuditAdminHandler_TriggerIntegrityCheck_Success(t *testing.T) {
	handler, mockIntegrity, _, _ := setupAuditAdminHandler()

	// Mock the integrity check
	expectedReport := &audit.IntegrityReport{
		CheckID:        "test-check-001",
		OverallScore:   0.9987,
		TotalEvents:    1000,
		VerifiedEvents: 999,
		Issues:         []audit.IntegrityIssue{},
		Duration:       45 * time.Second,
	}

	mockIntegrity.On("PerformIntegrityCheck", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("audit.IntegrityCriteria")).
		Return(expectedReport, nil)

	// Create test request
	reqBody := TriggerIntegrityCheckRequest{
		CheckType: "hash_chain",
		Priority:  "normal",
		AsyncMode: false,
	}
	reqJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/admin/audit/verify", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	// Add mock request metadata to context
	ctx := context.WithValue(req.Context(), contextKeyRequestMeta, &RequestMeta{
		RequestID: "test-request-001",
	})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Call the handler
	handlerFunc := handler.TriggerIntegrityCheck()
	handlerFunc.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	
	// Verify response data
	responseData, ok := response.Data.(*IntegrityCheckResponse)
	require.True(t, ok)
	assert.Equal(t, "hash_chain", responseData.CheckType)
	assert.Equal(t, "completed", responseData.Status)
	assert.NotNil(t, responseData.Result)
	
	mockIntegrity.AssertExpectations(t)
}

func TestAuditAdminHandler_TriggerIntegrityCheck_AsyncMode(t *testing.T) {
	handler, _, _, _ := setupAuditAdminHandler()

	// Create async request
	reqBody := TriggerIntegrityCheckRequest{
		CheckType: "comprehensive",
		Priority:  "high",
		AsyncMode: true,
	}
	reqJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/admin/audit/verify", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	ctx := context.WithValue(req.Context(), contextKeyRequestMeta, &RequestMeta{
		RequestID: "test-request-002",
	})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Call the handler
	handlerFunc := handler.TriggerIntegrityCheck()
	handlerFunc.ServeHTTP(w, req)

	// Verify async response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	
	responseData, ok := response.Data.(*IntegrityCheckResponse)
	require.True(t, ok)
	assert.Equal(t, "comprehensive", responseData.CheckType)
	assert.Equal(t, "queued", responseData.Status)
	assert.NotEmpty(t, responseData.CheckID)
	assert.Nil(t, responseData.Result) // No result yet for async
}

func TestAuditAdminHandler_GetSystemHealth_Success(t *testing.T) {
	handler, mockIntegrity, mockCompliance, mockLogger := setupAuditAdminHandler()

	// Setup mocks
	mockIntegrityStatus := &auditService.IntegrityServiceStatus{
		IsRunning:     true,
		LastCheck:     time.Now().Add(-5 * time.Minute),
		ChecksToday:   25,
		FailedChecks:  0,
		HealthStatus:  "healthy",
	}
	
	mockIntegrity.On("GetIntegrityStatus", mock.AnythingOfType("*context.valueCtx")).
		Return(mockIntegrityStatus, nil)

	mockLoggerStats := &auditService.LoggerStats{
		TotalEvents:        50000,
		DroppedEvents:      0,
		BufferSize:         150,
		BufferCapacity:     10000,
		WorkersActive:      4,
		BatchWorkersActive: 2,
		IsRunning:          true,
		CircuitState:       auditService.CircuitStateClosed,
	}
	
	mockLogger.On("GetStats").Return(mockLoggerStats)
	mockLogger.On("GetStatus").Return("healthy")

	req := httptest.NewRequest("GET", "/api/v1/admin/audit/health", nil)
	w := httptest.NewRecorder()

	// Call the handler
	handlerFunc := handler.GetSystemHealth()
	handlerFunc.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	
	healthData, ok := response.Data.(*SystemHealthResponse)
	require.True(t, ok)
	assert.Equal(t, "healthy", healthData.OverallStatus)
	assert.NotNil(t, healthData.IntegrityStatus)
	assert.NotNil(t, healthData.LoggerStatus)
	assert.Equal(t, "healthy", healthData.LoggerStatus.Status)
	assert.Equal(t, 6, healthData.LoggerStatus.WorkersActive) // 4 + 2
	
	mockIntegrity.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestAuditAdminHandler_RepairChain_DryRun(t *testing.T) {
	handler, _, _, _ := setupAuditAdminHandler()

	// Create repair request with dry run
	reqBody := ChainRepairRequest{
		StartSequence:  values.SequenceNumber(1000),
		EndSequence:    values.SequenceNumber(2000),
		RepairStrategy: "rebuild",
		DryRun:         true,
		BackupData:     true,
	}
	reqJSON, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/admin/audit/repair", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	ctx := context.WithValue(req.Context(), contextKeyRequestMeta, &RequestMeta{
		RequestID: "test-request-003",
	})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Call the handler
	handlerFunc := handler.RepairChain()
	handlerFunc.ServeHTTP(w, req)

	// Verify dry run response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	
	repairData, ok := response.Data.(*ChainRepairResponse)
	require.True(t, ok)
	assert.Equal(t, "completed", repairData.Status)
	assert.True(t, repairData.DryRun)
	assert.NotEmpty(t, repairData.RepairID)
}

func TestAuditAdminHandler_GetDetailedStats_Success(t *testing.T) {
	handler, _, _, _ := setupAuditAdminHandler()

	req := httptest.NewRequest("GET", "/api/v1/admin/audit/stats?period=last_24h", nil)
	w := httptest.NewRecorder()

	// Call the handler
	handlerFunc := handler.GetDetailedStats()
	handlerFunc.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	
	statsData, ok := response.Data.(*DetailedStatsResponse)
	require.True(t, ok)
	assert.Equal(t, "last_24h", statsData.Period)
	assert.NotNil(t, statsData.EventStatistics)
	assert.NotNil(t, statsData.IntegrityStats)
	assert.NotNil(t, statsData.ComplianceStats)
	assert.NotNil(t, statsData.PerformanceStats)
	assert.NotNil(t, statsData.TrendAnalysis)
}

func TestAuditAdminHandler_GetCorruptionReport_Success(t *testing.T) {
	handler, mockIntegrity, _, _ := setupAuditAdminHandler()

	// Mock corruption detection
	mockCorruptionReport := &audit.CorruptionReport{
		ScanID:             "scan-001",
		ScanPeriod:         time.Hour * 24,
		TotalIncidents:     3,
		HighSeverityCount:  0,
		MediumSeverityCount: 1,
		LowSeverityCount:   2,
		AutoResolvedCount:  2,
		ManualInterventionCount: 1,
	}

	mockIntegrity.On("DetectCorruption", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("audit.CorruptionDetectionCriteria")).
		Return(mockCorruptionReport, nil)

	req := httptest.NewRequest("GET", "/api/v1/admin/audit/corruption?period=last_7d&severity=medium", nil)
	w := httptest.NewRecorder()

	// Call the handler
	handlerFunc := handler.GetCorruptionReport()
	handlerFunc.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	
	corruptionData, ok := response.Data.(*CorruptionReportResponse)
	require.True(t, ok)
	assert.Equal(t, "last_7d", corruptionData.ScanPeriod)
	assert.NotNil(t, corruptionData.CorruptionSummary)
	assert.NotEmpty(t, corruptionData.CorruptionIncidents)
	assert.NotEmpty(t, corruptionData.RecommendedActions)
	
	mockIntegrity.AssertExpectations(t)
}

func TestAuditAdminHandler_GetIntegrityCheckStatus_Success(t *testing.T) {
	handler, _, _, _ := setupAuditAdminHandler()

	req := httptest.NewRequest("GET", "/api/v1/admin/audit/verify/check-123", nil)
	req.SetPathValue("checkId", "check-123")
	
	w := httptest.NewRecorder()

	// Call the handler
	handlerFunc := handler.GetIntegrityCheckStatus()
	handlerFunc.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	
	statusData, ok := response.Data.(*IntegrityCheckResponse)
	require.True(t, ok)
	assert.Equal(t, "check-123", statusData.CheckID)
	assert.Equal(t, "completed", statusData.Status)
	assert.NotNil(t, statusData.Links)
}

func TestAuditAdminHandler_GetRepairProgress_Success(t *testing.T) {
	handler, _, _, _ := setupAuditAdminHandler()

	req := httptest.NewRequest("GET", "/api/v1/admin/audit/repair/repair-456/progress", nil)
	req.SetPathValue("repairId", "repair-456")
	
	w := httptest.NewRecorder()

	// Call the handler
	handlerFunc := handler.GetRepairProgress()
	handlerFunc.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	
	progressData, ok := response.Data.(*ChainRepairProgress)
	require.True(t, ok)
	assert.Equal(t, 67.5, progressData.Percentage)
	assert.Equal(t, int64(6750), progressData.EventsRepaired)
	assert.Equal(t, int64(10000), progressData.EventsTotal)
	assert.Equal(t, "hash_chain_repair", progressData.CurrentPhase)
	assert.NotNil(t, progressData.EstimatedTimeLeft)
}

// Benchmark tests for performance validation

func BenchmarkAuditAdminHandler_GetSystemHealth(b *testing.B) {
	handler, mockIntegrity, _, mockLogger := setupAuditAdminHandler()

	// Setup mocks for benchmark
	mockIntegrityStatus := &auditService.IntegrityServiceStatus{
		IsRunning:    true,
		LastCheck:    time.Now(),
		ChecksToday:  100,
		FailedChecks: 0,
		HealthStatus: "healthy",
	}
	
	mockLoggerStats := &auditService.LoggerStats{
		TotalEvents:        100000,
		DroppedEvents:      0,
		BufferSize:         200,
		BufferCapacity:     10000,
		WorkersActive:      4,
		BatchWorkersActive: 2,
		IsRunning:          true,
		CircuitState:       auditService.CircuitStateClosed,
	}
	
	mockIntegrity.On("GetIntegrityStatus", mock.Anything).Return(mockIntegrityStatus, nil)
	mockLogger.On("GetStats").Return(mockLoggerStats)
	mockLogger.On("GetStatus").Return("healthy")

	req := httptest.NewRequest("GET", "/api/v1/admin/audit/health", nil)
	handlerFunc := handler.GetSystemHealth()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handlerFunc.ServeHTTP(w, req)
		
		// Verify response is successful
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkAuditAdminHandler_GetDetailedStats(b *testing.B) {
	handler, _, _, _ := setupAuditAdminHandler()

	req := httptest.NewRequest("GET", "/api/v1/admin/audit/stats", nil)
	handlerFunc := handler.GetDetailedStats()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handlerFunc.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}