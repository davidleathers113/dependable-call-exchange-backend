package audit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"github.com/google/uuid"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

// Mock implementations for testing

type MockHashChainService struct {
	mock.Mock
}

func (m *MockHashChainService) VerifyChain(ctx context.Context, start, end values.SequenceNumber) (*audit.HashChainVerificationResult, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).(*audit.HashChainVerificationResult), args.Error(1)
}

func (m *MockHashChainService) RepairChain(ctx context.Context, start, end values.SequenceNumber) (*audit.HashChainRepairResult, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).(*audit.HashChainRepairResult), args.Error(1)
}

type MockIntegrityCheckService struct {
	mock.Mock
}

func (m *MockIntegrityCheckService) PerformIntegrityCheck(ctx context.Context, criteria audit.IntegrityCriteria) (*audit.IntegrityReport, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(*audit.IntegrityReport), args.Error(1)
}

type MockEventRepository struct {
	mock.Mock
}

func (m *MockEventRepository) GetLatestSequenceNumber(ctx context.Context) (values.SequenceNumber, error) {
	args := m.Called(ctx)
	return args.Get(0).(values.SequenceNumber), args.Error(1)
}

func (m *MockEventRepository) GetByID(ctx context.Context, id string) (*audit.Event, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*audit.Event), args.Error(1)
}

func (m *MockEventRepository) GetSequenceRange(ctx context.Context, start, end values.SequenceNumber) ([]*audit.Event, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).([]*audit.Event), args.Error(1)
}

type MockIntegrityRepository struct {
	mock.Mock
}

func (m *MockIntegrityRepository) VerifySequenceIntegrity(ctx context.Context, criteria audit.SequenceIntegrityCriteria) (*audit.SequenceIntegrityResult, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(*audit.SequenceIntegrityResult), args.Error(1)
}

func (m *MockIntegrityRepository) DetectCorruption(ctx context.Context, criteria audit.CorruptionDetectionCriteria) (*audit.CorruptionReport, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(*audit.CorruptionReport), args.Error(1)
}

func (m *MockIntegrityRepository) ScheduleIntegrityCheck(ctx context.Context, schedule *audit.IntegrityCheckSchedule) (string, error) {
	args := m.Called(ctx, schedule)
	return args.String(0), args.Error(1)
}

func (m *MockIntegrityRepository) GetIntegrityMonitoringStatus(ctx context.Context) (*audit.IntegrityMonitoringStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(*audit.IntegrityMonitoringStatus), args.Error(1)
}

type MockMonitor struct {
	mock.Mock
}

func (m *MockMonitor) RecordCounter(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMonitor) RecordHistogram(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMonitor) RecordGauge(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

// Test setup helper
func setupTestIntegrityService(t *testing.T) (*IntegrityService, *MockHashChainService, *MockIntegrityCheckService, *MockEventRepository, *MockIntegrityRepository, *MockMonitor) {
	logger := zaptest.NewLogger(t)

	mockHashChain := &MockHashChainService{}
	mockIntegrityCheck := &MockIntegrityCheckService{}
	mockEventRepo := &MockEventRepository{}
	mockIntegrityRepo := &MockIntegrityRepository{}
	mockMonitor := &MockMonitor{}

	config := DefaultIntegrityConfig()
	config.EnableBackgroundChecks = false // Disable for tests

	service := NewIntegrityService(
		mockHashChain,
		mockIntegrityCheck,
		nil, // compliance service
		nil, // recovery service
		nil, // crypto service
		mockEventRepo,
		mockIntegrityRepo,
		nil, // audit cache
		mockMonitor,
		logger,
		config,
	)

	return service, mockHashChain, mockIntegrityCheck, mockEventRepo, mockIntegrityRepo, mockMonitor
}

func TestNewIntegrityService(t *testing.T) {
	service, _, _, _, _, _ := setupTestIntegrityService(t)

	assert.NotNil(t, service)
	assert.NotNil(t, service.config)
	assert.NotNil(t, service.workerPool)
	assert.NotNil(t, service.scheduler)
	assert.NotNil(t, service.alertManager)
	assert.NotNil(t, service.metrics)
	assert.False(t, service.isRunning)
}

func TestIntegrityService_StartStop(t *testing.T) {
	service, _, _, _, _, _ := setupTestIntegrityService(t)
	ctx := context.Background()

	// Test start
	err := service.Start(ctx)
	require.NoError(t, err)
	assert.True(t, service.isRunning)

	// Test double start (should fail)
	err = service.Start(ctx)
	assert.Error(t, err)

	// Test stop
	err = service.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, service.isRunning)

	// Test double stop (should succeed)
	err = service.Stop(ctx)
	assert.NoError(t, err)
}

func TestIntegrityService_VerifyHashChain(t *testing.T) {
	service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
	ctx := context.Background()

	start := values.SequenceNumber(1)
	end := values.SequenceNumber(100)

	expectedResult := &audit.HashChainVerificationResult{
		StartSequence:  start,
		EndSequence:    end,
		IsValid:        true,
		IntegrityScore: 0.99,
		EventsVerified: 100,
		VerifiedAt:     time.Now(),
		VerificationID: "test-verification",
		Method:         "full",
	}

	// Setup mocks
	mockHashChain.On("VerifyChain", ctx, start, end).Return(expectedResult, nil)
	mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
	mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

	// Test hash chain verification
	result, err := service.VerifyHashChain(ctx, start, end)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResult.StartSequence, result.StartSequence)
	assert.Equal(t, expectedResult.EndSequence, result.EndSequence)
	assert.Equal(t, expectedResult.IsValid, result.IsValid)
	assert.Equal(t, expectedResult.IntegrityScore, result.IntegrityScore)

	// Verify metrics updated
	assert.Equal(t, int64(1), service.metrics.TotalChecks)
	assert.Equal(t, int64(1), service.metrics.SuccessfulChecks)
	assert.Equal(t, int64(1), service.metrics.HashChainChecks)

	mockHashChain.AssertExpectations(t)
	mockMonitor.AssertExpectations(t)
}

func TestIntegrityService_VerifySequenceIntegrity(t *testing.T) {
	service, _, _, _, mockIntegrityRepo, mockMonitor := setupTestIntegrityService(t)
	ctx := context.Background()

	start := values.SequenceNumber(1)
	end := values.SequenceNumber(100)

	criteria := audit.SequenceIntegrityCriteria{
		StartSequence:   &start,
		EndSequence:     &end,
		CheckGaps:       true,
		CheckDuplicates: true,
		CheckOrder:      true,
	}

	expectedResult := &audit.SequenceIntegrityResult{
		IsValid:          true,
		IntegrityScore:   1.0,
		SequencesChecked: 100,
		GapsFound:        0,
		DuplicatesFound:  0,
		CheckedAt:        time.Now(),
	}

	// Setup mocks
	mockIntegrityRepo.On("VerifySequenceIntegrity", ctx, criteria).Return(expectedResult, nil)
	mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
	mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

	// Test sequence integrity verification
	result, err := service.VerifySequenceIntegrity(ctx, criteria)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsValid)
	assert.Equal(t, int64(100), result.SequencesChecked)
	assert.Equal(t, int64(0), result.GapsFound)
	assert.Equal(t, int64(0), result.DuplicatesFound)

	// Verify metrics updated
	assert.Equal(t, int64(1), service.metrics.TotalChecks)
	assert.Equal(t, int64(1), service.metrics.SuccessfulChecks)
	assert.Equal(t, int64(1), service.metrics.SequenceChecks)

	mockIntegrityRepo.AssertExpectations(t)
	mockMonitor.AssertExpectations(t)
}

func TestIntegrityService_PerformIntegrityCheck(t *testing.T) {
	service, _, mockIntegrityCheck, _, _, mockMonitor := setupTestIntegrityService(t)
	ctx := context.Background()

	criteria := audit.IntegrityCriteria{
		CheckHashChain:  true,
		CheckSequencing: true,
		CheckCompliance: true,
	}

	expectedReport := &audit.IntegrityReport{
		OverallStatus:  "HEALTHY",
		IsHealthy:      true,
		TotalEvents:    1000,
		VerifiedEvents: 1000,
		FailedEvents:   0,
		GeneratedAt:    time.Now(),
	}

	// Setup mocks
	mockIntegrityCheck.On("PerformIntegrityCheck", ctx, criteria).Return(expectedReport, nil)
	mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
	mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

	// Test comprehensive integrity check
	report, err := service.PerformIntegrityCheck(ctx, criteria)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, "HEALTHY", report.OverallStatus)
	assert.True(t, report.IsHealthy)
	assert.Equal(t, int64(1000), report.TotalEvents)
	assert.Equal(t, int64(1000), report.VerifiedEvents)
	assert.Equal(t, int64(0), report.FailedEvents)

	mockIntegrityCheck.AssertExpectations(t)
	mockMonitor.AssertExpectations(t)
}

func TestIntegrityService_DetectCorruption(t *testing.T) {
	service, _, _, _, mockIntegrityRepo, mockMonitor := setupTestIntegrityService(t)
	ctx := context.Background()

	criteria := audit.CorruptionDetectionCriteria{
		CheckHashes:     true,
		CheckMetadata:   true,
		CheckReferences: true,
		DeepScan:        false,
	}

	expectedReport := &audit.CorruptionReport{
		ReportID:        "test-corruption-report",
		CorruptionFound: false,
		CorruptionLevel: "none",
		EventsScanned:   1000,
		EventsCorrupted: 0,
		ScannedAt:       time.Now(),
	}

	// Setup mocks
	mockIntegrityRepo.On("DetectCorruption", ctx, criteria).Return(expectedReport, nil)
	mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
	mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

	// Test corruption detection
	report, err := service.DetectCorruption(ctx, criteria)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.False(t, report.CorruptionFound)
	assert.Equal(t, "none", report.CorruptionLevel)
	assert.Equal(t, int64(1000), report.EventsScanned)
	assert.Equal(t, int64(0), report.EventsCorrupted)

	// Verify metrics updated
	assert.Equal(t, int64(1), service.metrics.CorruptionScans)

	mockIntegrityRepo.AssertExpectations(t)
	mockMonitor.AssertExpectations(t)
}

func TestIntegrityService_RepairChain(t *testing.T) {
	service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
	ctx := context.Background()

	start := values.SequenceNumber(1)
	end := values.SequenceNumber(10)
	options := &RepairOptions{
		DryRun:      false,
		ForceRepair: false,
	}

	expectedResult := &audit.HashChainRepairResult{
		RepairID:           "test-repair",
		EventsRepaired:     5,
		EventsFailed:       0,
		HashesRecalculated: 5,
		ChainLinksRepaired: 5,
		RepairTime:         time.Second,
		RepairedAt:         time.Now(),
	}

	// Setup mocks
	mockHashChain.On("RepairChain", ctx, start, end).Return(expectedResult, nil)
	mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
	mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

	// Test chain repair
	result, err := service.RepairChain(ctx, start, end, options)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(5), result.EventsRepaired)
	assert.Equal(t, int64(0), result.EventsFailed)
	assert.Equal(t, int64(5), result.HashesRecalculated)

	mockHashChain.AssertExpectations(t)
	mockMonitor.AssertExpectations(t)
}

func TestIntegrityService_RepairChainDisabled(t *testing.T) {
	service, _, _, _, _, _ := setupTestIntegrityService(t)
	ctx := context.Background()

	// Disable chain repair
	service.config.ChainRepairEnabled = false

	start := values.SequenceNumber(1)
	end := values.SequenceNumber(10)
	options := &RepairOptions{}

	// Test repair when disabled
	result, err := service.RepairChain(ctx, start, end, options)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "chain repair is disabled")
}

func TestIntegrityService_ScheduleIntegrityCheck(t *testing.T) {
	service, _, _, _, mockIntegrityRepo, _ := setupTestIntegrityService(t)
	ctx := context.Background()

	schedule := &audit.IntegrityCheckSchedule{
		CheckType:      "hash_chain",
		CronExpression: "0 */5 * * * *", // Every 5 minutes
		IsEnabled:      true,
		NextRun:        time.Now().Add(5 * time.Minute),
	}

	expectedScheduleID := "test-schedule-id"

	// Setup mocks
	mockIntegrityRepo.On("ScheduleIntegrityCheck", ctx, schedule).Return(expectedScheduleID, nil)

	// Test scheduling
	scheduleID, err := service.ScheduleIntegrityCheck(ctx, schedule)

	require.NoError(t, err)
	assert.Equal(t, expectedScheduleID, scheduleID)

	mockIntegrityRepo.AssertExpectations(t)
}

func TestIntegrityService_GetIntegrityStatus(t *testing.T) {
	service, _, _, _, mockIntegrityRepo, _ := setupTestIntegrityService(t)
	ctx := context.Background()

	expectedMonitoringStatus := &audit.IntegrityMonitoringStatus{
		IsEnabled:     true,
		LastCheck:     time.Now().Add(-5 * time.Minute),
		NextCheck:     time.Now().Add(5 * time.Minute),
		CheckInterval: 10 * time.Minute,
		HealthStatus:  "healthy",
	}

	// Setup mocks
	mockIntegrityRepo.On("GetIntegrityMonitoringStatus", ctx).Return(expectedMonitoringStatus, nil)

	// Test status retrieval
	status, err := service.GetIntegrityStatus(ctx)

	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.False(t, status.IsRunning) // Service not started in test
	assert.NotNil(t, status.WorkerPoolStatus)
	assert.NotNil(t, status.SchedulerStatus)
	assert.NotNil(t, status.MonitoringStatus)
	assert.NotNil(t, status.Metrics)

	mockIntegrityRepo.AssertExpectations(t)
}

func TestIntegrityService_AlertThresholds(t *testing.T) {
	service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
	ctx := context.Background()

	start := values.SequenceNumber(1)
	end := values.SequenceNumber(100)

	// Set low threshold to trigger alert
	service.config.IntegrityScoreThreshold = 0.99

	lowScoreResult := &audit.HashChainVerificationResult{
		StartSequence:  start,
		EndSequence:    end,
		IsValid:        false,
		IntegrityScore: 0.95, // Below threshold
		EventsVerified: 100,
		VerifiedAt:     time.Now(),
		VerificationID: "test-verification",
		Method:         "full",
	}

	// Setup mocks
	mockHashChain.On("VerifyChain", ctx, start, end).Return(lowScoreResult, nil)
	mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
	mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

	// Test hash chain verification with low score
	result, err := service.VerifyHashChain(ctx, start, end)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsValid)
	assert.Equal(t, 0.95, result.IntegrityScore)

	// Check that alert was triggered
	alerts, err := service.alertManager.GetActiveAlerts(ctx)
	require.NoError(t, err)
	assert.Len(t, alerts, 1)
	assert.Equal(t, "integrity_score_low", alerts[0].AlertType)
	assert.Equal(t, "warning", alerts[0].Severity)

	mockHashChain.AssertExpectations(t)
	mockMonitor.AssertExpectations(t)
}

func TestWorkerPool_TaskProcessing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := NewWorkerPool(2, ctx)
	err := pool.Start()
	require.NoError(t, err)

	// Submit a test task
	task := IntegrityTask{
		TaskID:    "test-task",
		TaskType:  "hash_chain",
		StartSeq:  1,
		EndSeq:    10,
		Priority:  1,
		CreatedAt: time.Now(),
	}

	submitted := pool.SubmitTask(task)
	assert.True(t, submitted)

	// Wait for task processing
	time.Sleep(100 * time.Millisecond)

	status := pool.GetStatus()
	assert.Equal(t, 2, status.ActiveWorkers)
	assert.GreaterOrEqual(t, status.CompletedTasks, int64(0))

	pool.Stop()
}

func TestIntegrityScheduler_ScheduleManagement(t *testing.T) {
	service, _, _, _, _, _ := setupTestIntegrityService(t)
	scheduler := service.scheduler

	// Add a test schedule
	schedule := &ScheduledCheck{
		ID:        "test-schedule",
		Name:      "Test Schedule",
		Type:      "hash_chain",
		Interval:  5 * time.Minute,
		NextRun:   time.Now().Add(5 * time.Minute),
		IsEnabled: true,
	}

	scheduler.AddSchedule(schedule)

	// Verify schedule was added
	schedules := scheduler.GetSchedules()
	assert.Len(t, schedules, 1)
	assert.Equal(t, "test-schedule", schedules["test-schedule"].ID)
	assert.Equal(t, "Test Schedule", schedules["test-schedule"].Name)

	// Update schedule
	updates := map[string]interface{}{
		"name":     "Updated Test Schedule",
		"enabled":  false,
		"interval": 10 * time.Minute,
	}

	err := scheduler.UpdateSchedule("test-schedule", updates)
	require.NoError(t, err)

	// Verify updates
	schedules = scheduler.GetSchedules()
	updatedSchedule := schedules["test-schedule"]
	assert.Equal(t, "Updated Test Schedule", updatedSchedule.Name)
	assert.False(t, updatedSchedule.IsEnabled)
	assert.Equal(t, 10*time.Minute, updatedSchedule.Interval)

	// Remove schedule
	scheduler.RemoveSchedule("test-schedule")
	schedules = scheduler.GetSchedules()
	assert.Len(t, schedules, 0)
}

func TestAlertManager_AlertLifecycle(t *testing.T) {
	service, _, _, _, _, _ := setupTestIntegrityService(t)
	alertManager := service.alertManager
	ctx := context.Background()

	// Create test alert
	alert := &IntegrityAlert{
		AlertID:     "test-alert",
		AlertType:   "test_alert",
		Severity:    "warning",
		Title:       "Test Alert",
		Description: "This is a test alert",
		TriggeredAt: time.Now(),
		IsResolved:  false,
	}

	// Trigger alert
	alertManager.TriggerAlert(ctx, alert)

	// Verify alert exists and is active
	activeAlerts, err := alertManager.GetActiveAlerts(ctx)
	require.NoError(t, err)
	assert.Len(t, activeAlerts, 1)
	assert.Equal(t, "test-alert", activeAlerts[0].AlertID)
	assert.False(t, activeAlerts[0].IsResolved)

	// Get specific alert
	retrievedAlert, err := alertManager.GetAlert(ctx, "test-alert")
	require.NoError(t, err)
	assert.Equal(t, "test-alert", retrievedAlert.AlertID)
	assert.Equal(t, "test_alert", retrievedAlert.AlertType)

	// Resolve alert
	err = alertManager.ResolveAlert(ctx, "test-alert", "test-user")
	require.NoError(t, err)

	// Verify alert is resolved
	resolvedAlert, err := alertManager.GetAlert(ctx, "test-alert")
	require.NoError(t, err)
	assert.True(t, resolvedAlert.IsResolved)
	assert.NotNil(t, resolvedAlert.ResolvedAt)

	// Verify no active alerts
	activeAlerts, err = alertManager.GetActiveAlerts(ctx)
	require.NoError(t, err)
	assert.Len(t, activeAlerts, 0)

	// Get alert summary
	summary := alertManager.GetAlertSummary(ctx)
	assert.Equal(t, 1, summary.TotalAlerts)
	assert.Equal(t, 0, summary.OpenAlerts)
	assert.Equal(t, 1, summary.ResolvedAlerts)
	assert.Equal(t, 1, summary.BySeverity["warning"])
	assert.Equal(t, 1, summary.ByType["test_alert"])
}

func TestAlertManager_Cooldown(t *testing.T) {
	service, _, _, _, _, _ := setupTestIntegrityService(t)
	service.config.AlertCooldown = 1 * time.Second // Short cooldown for test
	alertManager := service.alertManager
	ctx := context.Background()

	// Create two identical alerts
	alert1 := &IntegrityAlert{
		AlertID:     "test-alert-1",
		AlertType:   "test_alert",
		Severity:    "warning",
		Title:       "Test Alert 1",
		Description: "This is test alert 1",
		TriggeredAt: time.Now(),
		IsResolved:  false,
	}

	alert2 := &IntegrityAlert{
		AlertID:     "test-alert-2",
		AlertType:   "test_alert",
		Severity:    "warning",
		Title:       "Test Alert 2",
		Description: "This is test alert 2",
		TriggeredAt: time.Now(),
		IsResolved:  false,
	}

	// Trigger first alert
	alertManager.TriggerAlert(ctx, alert1)

	// Immediately trigger second alert (should be suppressed due to cooldown)
	alertManager.TriggerAlert(ctx, alert2)

	// Verify only first alert exists
	activeAlerts, err := alertManager.GetActiveAlerts(ctx)
	require.NoError(t, err)
	assert.Len(t, activeAlerts, 1)
	assert.Equal(t, "test-alert-1", activeAlerts[0].AlertID)

	// Wait for cooldown to expire
	time.Sleep(1200 * time.Millisecond)

	// Trigger third alert (should succeed after cooldown)
	alert3 := &IntegrityAlert{
		AlertID:     "test-alert-3",
		AlertType:   "test_alert",
		Severity:    "warning",
		Title:       "Test Alert 3",
		Description: "This is test alert 3",
		TriggeredAt: time.Now(),
		IsResolved:  false,
	}

	alertManager.TriggerAlert(ctx, alert3)

	// Verify both alerts exist now
	activeAlerts, err = alertManager.GetActiveAlerts(ctx)
	require.NoError(t, err)
	assert.Len(t, activeAlerts, 2)
}

// Benchmark tests

func BenchmarkIntegrityService_VerifyHashChain(b *testing.B) {
	service, mockHashChain, _, _, _, _ := setupTestIntegrityService(&testing.T{})
	ctx := context.Background()

	start := values.SequenceNumber(1)
	end := values.SequenceNumber(1000)

	result := &audit.HashChainVerificationResult{
		StartSequence:  start,
		EndSequence:    end,
		IsValid:        true,
		IntegrityScore: 1.0,
		EventsVerified: 1000,
		VerifiedAt:     time.Now(),
		VerificationID: "benchmark",
		Method:         "full",
	}

	mockHashChain.On("VerifyChain", mock.Anything, mock.Anything, mock.Anything).Return(result, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.VerifyHashChain(ctx, start, end)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWorkerPool_TaskSubmission(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := NewWorkerPool(10, ctx)
	err := pool.Start()
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Stop()

	task := IntegrityTask{
		TaskID:    "benchmark-task",
		TaskType:  "hash_chain",
		StartSeq:  1,
		EndSeq:    100,
		Priority:  1,
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.SubmitTask(task)
	}
}

// ==================== COMPREHENSIVE PROPERTY-BASED TESTS ====================

// TestPropertyIntegrityServiceMetricsConsistency tests that metrics are always consistent across operations
func TestPropertyIntegrityServiceMetricsConsistency(t *testing.T) {
	for i := 0; i < 1000; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
			ctx := context.Background()

			// Generate random but valid sequence numbers
			startVal := uint64(i%1000 + 1)
			endVal := startVal + uint64(i%100 + 1)
			start := values.SequenceNumber(startVal)
			end := values.SequenceNumber(endVal)

			// Create verification result with deterministic properties
			isValid := i%2 == 0
			eventsVerified := int64(endVal - startVal + 1)
			integrityScore := 0.95 + float64(i%5)*0.01 // 0.95 to 0.99

			verificationResult := &audit.HashChainVerificationResult{
				StartSequence:  start,
				EndSequence:    end,
				IsValid:        isValid,
				EventsVerified: eventsVerified,
				IntegrityScore: integrityScore,
				VerifiedAt:     time.Now(),
				VerificationID: fmt.Sprintf("test-verification-%d", i),
				Method:         "full",
			}

			// Setup mocks
			mockHashChain.On("VerifyChain", ctx, start, end).Return(verificationResult, nil)
			mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
			mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

			// Perform verification
			result, err := service.VerifyHashChain(ctx, start, end)

			// Verify invariants
			require.NoError(t, err, "verification should never fail with valid inputs")
			require.NotNil(t, result, "result should never be nil on success")
			assert.Equal(t, isValid, result.IsValid, "validity should match expected")
			assert.Equal(t, eventsVerified, result.EventsVerified, "events verified should match range")
			assert.Equal(t, start, result.StartSequence, "start sequence should match")
			assert.Equal(t, end, result.EndSequence, "end sequence should match")
			assert.GreaterOrEqual(t, result.IntegrityScore, 0.0, "integrity score should be non-negative")
			assert.LessOrEqual(t, result.IntegrityScore, 1.0, "integrity score should not exceed 1.0")

			// Verify metrics were recorded
			assert.Equal(t, int64(1), service.metrics.TotalChecks, "total checks should increment")
			if isValid {
				assert.Equal(t, int64(1), service.metrics.SuccessfulChecks, "successful checks should increment for valid chains")
			} else {
				assert.Equal(t, int64(0), service.metrics.SuccessfulChecks, "successful checks should not increment for invalid chains")
			}

			mockHashChain.AssertExpectations(t)
			mockMonitor.AssertExpectations(t)
		})
	}
}

// TestPropertySequenceIntegrityInvariants tests sequence integrity validation invariants
func TestPropertySequenceIntegrityInvariants(t *testing.T) {
	for i := 0; i < 1000; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			service, _, _, _, mockIntegrityRepo, mockMonitor := setupTestIntegrityService(t)
			ctx := context.Background()

			// Generate random valid criteria
			startVal := uint64(i%500 + 1)
			endVal := startVal + uint64(i%200 + 1)
			start := values.SequenceNumber(startVal)
			end := values.SequenceNumber(endVal)

			criteria := audit.SequenceIntegrityCriteria{
				StartSequence:   &start,
				EndSequence:     &end,
				CheckGaps:       i%2 == 0,
				CheckDuplicates: i%3 == 0,
				CheckOrder:      i%4 == 0,
			}

			// Generate result with controlled randomness
			totalSequences := endVal - startVal + 1
			gapsFound := int64(i % 3) // 0, 1, or 2 gaps
			duplicatesFound := int64(i % 2) // 0 or 1 duplicate
			validSequences := int64(totalSequences) - gapsFound

			expectedResult := &audit.SequenceIntegrityResult{
				IsValid:          gapsFound == 0 && duplicatesFound == 0,
				IntegrityScore:   float64(validSequences) / float64(totalSequences),
				SequencesChecked: int64(totalSequences),
				GapsFound:        gapsFound,
				DuplicatesFound:  duplicatesFound,
				CheckedAt:        time.Now(),
			}

			// Setup mocks
			mockIntegrityRepo.On("VerifySequenceIntegrity", ctx, criteria).Return(expectedResult, nil)
			mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
			mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

			// Perform verification
			result, err := service.VerifySequenceIntegrity(ctx, criteria)

			// Verify invariants
			require.NoError(t, err, "sequence integrity check should not fail with valid criteria")
			require.NotNil(t, result, "result should never be nil")
			assert.Equal(t, int64(totalSequences), result.SequencesChecked, "sequences checked should match range")
			assert.GreaterOrEqual(t, result.IntegrityScore, 0.0, "integrity score should be non-negative")
			assert.LessOrEqual(t, result.IntegrityScore, 1.0, "integrity score should not exceed 1.0")
			assert.GreaterOrEqual(t, result.GapsFound, int64(0), "gaps found should be non-negative")
			assert.GreaterOrEqual(t, result.DuplicatesFound, int64(0), "duplicates found should be non-negative")
			
			// Integrity should be false if gaps or duplicates found
			if result.GapsFound > 0 || result.DuplicatesFound > 0 {
				assert.False(t, result.IsValid, "result should be invalid if gaps or duplicates found")
			}

			// Integrity score should reflect the ratio of valid sequences
			expectedScore := float64(result.SequencesChecked-result.GapsFound) / float64(result.SequencesChecked)
			assert.InDelta(t, expectedScore, result.IntegrityScore, 0.001, "integrity score should reflect valid sequence ratio")

			mockIntegrityRepo.AssertExpectations(t)
			mockMonitor.AssertExpectations(t)
		})
	}
}

// TestPropertyCorruptionDetectionThoroughness tests corruption detection completeness
func TestPropertyCorruptionDetectionThoroughness(t *testing.T) {
	for i := 0; i < 1000; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			service, _, _, _, mockIntegrityRepo, mockMonitor := setupTestIntegrityService(t)
			ctx := context.Background()

			// Generate random criteria
			checkTypes := []string{"hash_mismatch", "missing_field", "invalid_timestamp", "broken_chain"}
			selectedTypes := make([]string, 0)
			for j, checkType := range checkTypes {
				if (i>>j)&1 == 1 { // Use bits of i to select check types
					selectedTypes = append(selectedTypes, checkType)
				}
			}
			if len(selectedTypes) == 0 {
				selectedTypes = []string{"hash_mismatch"} // Ensure at least one type
			}

			criteria := audit.CorruptionDetectionCriteria{
				CheckTypes: selectedTypes,
				DeepScan:   i%2 == 0,
			}

			// Generate result with controlled corruption instances
			corruptionCount := i % 5 // 0 to 4 corruptions
			corruptions := make([]*audit.CorruptionInstance, corruptionCount)
			for j := 0; j < corruptionCount; j++ {
				corruptions[j] = &audit.CorruptionInstance{
					CorruptionID:   uuid.New().String(),
					EventID:        uuid.New(),
					CorruptionType: selectedTypes[j%len(selectedTypes)],
					Severity:       []string{"low", "medium", "high", "critical"}[j%4],
					RepairPossible: j%2 == 0,
					DetectedAt:     time.Now(),
				}
			}

			expectedReport := &audit.CorruptionReport{
				ReportID:        uuid.New().String(),
				CorruptionFound: corruptionCount > 0,
				CorruptionLevel: map[bool]string{true: "detected", false: "none"}[corruptionCount > 0],
				EventsScanned:   int64(i%1000 + 100),
				EventsCorrupted: int64(corruptionCount),
				Corruptions:     corruptions,
				ScannedAt:       time.Now(),
			}

			// Setup mocks
			mockIntegrityRepo.On("DetectCorruption", ctx, criteria).Return(expectedReport, nil)
			mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
			mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

			// Perform corruption detection
			report, err := service.DetectCorruption(ctx, criteria)

			// Verify invariants
			require.NoError(t, err, "corruption detection should not fail with valid criteria")
			require.NotNil(t, report, "report should never be nil")
			assert.Equal(t, int64(corruptionCount), report.EventsCorrupted, "corrupted events should match corruption count")
			assert.GreaterOrEqual(t, report.EventsScanned, report.EventsCorrupted, "scanned events should be >= corrupted events")
			assert.Equal(t, corruptionCount > 0, report.CorruptionFound, "corruption found should match corruption count > 0")
			assert.Len(t, report.Corruptions, corruptionCount, "corruption instances should match expected count")

			// Each corruption should have valid properties
			for _, corruption := range report.Corruptions {
				assert.NotEmpty(t, corruption.CorruptionID, "corruption ID should not be empty")
				assert.NotEqual(t, uuid.Nil, corruption.EventID, "event ID should be valid UUID")
				assert.Contains(t, selectedTypes, corruption.CorruptionType, "corruption type should be from selected types")
				assert.Contains(t, []string{"low", "medium", "high", "critical"}, corruption.Severity, "severity should be valid")
				assert.False(t, corruption.DetectedAt.IsZero(), "detected time should be set")
			}

			mockIntegrityRepo.AssertExpectations(t)
			mockMonitor.AssertExpectations(t)
		})
	}
}

// TestPropertyConcurrencyInvariants tests that concurrent operations maintain invariants
func TestPropertyConcurrencyInvariants(t *testing.T) {
	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
			ctx := context.Background()

			concurrency := 10 + (i % 10) // 10 to 19 concurrent operations
			
			// Set up mock expectations for concurrent calls
			mockHashChain.On("VerifyChain", mock.Anything, mock.Anything, mock.Anything).Return(
				&audit.HashChainVerificationResult{
					IsValid:        true,
					EventsVerified: 100,
					IntegrityScore: 1.0,
					VerifiedAt:     time.Now(),
					VerificationID: mock.AnythingOfType("string"),
					Method:         "full",
				}, nil).Times(concurrency)
			mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string")).Times(concurrency)
			mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Times(concurrency)

			var wg sync.WaitGroup
			results := make([]*audit.HashChainVerificationResult, concurrency)
			errors := make([]error, concurrency)

			// Start concurrent verification operations
			for j := 0; j < concurrency; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					start := values.SequenceNumber(uint64(idx*100 + 1))
					end := values.SequenceNumber(uint64(idx*100 + 100))
					result, err := service.VerifyHashChain(ctx, start, end)
					results[idx] = result
					errors[idx] = err
				}(j)
			}

			wg.Wait()

			// Verify all operations succeeded
			for j := 0; j < concurrency; j++ {
				assert.NoError(t, errors[j], "concurrent operation %d should not fail", j)
				assert.NotNil(t, results[j], "concurrent operation %d should return result", j)
				if results[j] != nil {
					assert.True(t, results[j].IsValid, "concurrent operation %d should have valid result", j)
					assert.Equal(t, int64(100), results[j].EventsVerified, "concurrent operation %d should verify 100 events", j)
				}
			}

			// Verify metrics are consistent (should equal concurrency count)
			assert.Equal(t, int64(concurrency), service.metrics.TotalChecks, "total checks should equal concurrency count")
			assert.Equal(t, int64(concurrency), service.metrics.SuccessfulChecks, "successful checks should equal concurrency count")

			mockHashChain.AssertExpectations(t)
			mockMonitor.AssertExpectations(t)
		})
	}
}

// TestPropertyServiceStateConsistency tests service state management invariants
func TestPropertyServiceStateConsistency(t *testing.T) {
	for i := 0; i < 1000; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			service, _, _, _, _, _ := setupTestIntegrityService(t)
			ctx := context.Background()

			// Test various state transitions
			assert.False(t, service.IsRunning(), "service should start in stopped state")

			// Start service
			err := service.Start(ctx)
			require.NoError(t, err, "service start should succeed")
			assert.True(t, service.IsRunning(), "service should be running after start")

			// Attempt to start again (should fail)
			err = service.Start(ctx)
			assert.Error(t, err, "starting already running service should fail")
			assert.True(t, service.IsRunning(), "service should still be running after failed start")

			// Get status while running
			status, err := service.GetIntegrityStatus(ctx)
			require.NoError(t, err, "getting status should succeed")
			assert.True(t, status.IsRunning, "status should show service as running")
			assert.NotNil(t, status.WorkerPoolStatus, "worker pool status should be available")
			assert.NotNil(t, status.SchedulerStatus, "scheduler status should be available")
			assert.NotNil(t, status.Metrics, "metrics should be available")

			// Stop service
			err = service.Stop(ctx)
			require.NoError(t, err, "service stop should succeed")
			assert.False(t, service.IsRunning(), "service should be stopped after stop")

			// Stop again (should succeed)
			err = service.Stop(ctx)
			assert.NoError(t, err, "stopping already stopped service should succeed")
			assert.False(t, service.IsRunning(), "service should still be stopped after redundant stop")

			// Get status while stopped
			status, err = service.GetIntegrityStatus(ctx)
			require.NoError(t, err, "getting status should succeed even when stopped")
			assert.False(t, status.IsRunning, "status should show service as stopped")
		})
	}
}

// TestPropertyAlertThresholdConsistency tests alert threshold behavior
func TestPropertyAlertThresholdConsistency(t *testing.T) {
	for i := 0; i < 1000; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
			ctx := context.Background()

			// Set random but valid threshold
			threshold := 0.5 + float64(i%50)/100.0 // 0.5 to 0.99
			service.config.IntegrityScoreThreshold = threshold

			// Generate score relative to threshold
			var score float64
			var shouldAlert bool
			if i%2 == 0 {
				// Below threshold - should trigger alert
				score = threshold - 0.01 - float64(i%10)/1000.0
				shouldAlert = true
			} else {
				// Above threshold - should not trigger alert
				score = threshold + 0.01 + float64(i%10)/1000.0
				shouldAlert = false
			}

			start := values.SequenceNumber(1)
			end := values.SequenceNumber(100)

			verificationResult := &audit.HashChainVerificationResult{
				StartSequence:  start,
				EndSequence:    end,
				IsValid:        !shouldAlert, // Invalid if alerting
				IntegrityScore: score,
				EventsVerified: 100,
				VerifiedAt:     time.Now(),
				VerificationID: fmt.Sprintf("threshold-test-%d", i),
				Method:         "full",
			}

			// Setup mocks
			mockHashChain.On("VerifyChain", ctx, start, end).Return(verificationResult, nil)
			mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
			mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

			// Perform verification
			result, err := service.VerifyHashChain(ctx, start, end)

			// Verify result properties
			require.NoError(t, err, "verification should succeed")
			require.NotNil(t, result, "result should not be nil")
			assert.Equal(t, score, result.IntegrityScore, "score should match expected")
			assert.Equal(t, !shouldAlert, result.IsValid, "validity should match alert expectation")

			// Check alert state
			alerts, err := service.alertManager.GetActiveAlerts(ctx)
			require.NoError(t, err, "getting alerts should succeed")

			if shouldAlert {
				assert.Greater(t, len(alerts), 0, "should have active alerts when score below threshold")
				// Find the integrity score alert
				var foundAlert bool
				for _, alert := range alerts {
					if alert.AlertType == "integrity_score_low" {
						foundAlert = true
						assert.Equal(t, "warning", alert.Severity, "integrity alert should be warning severity")
						assert.Contains(t, alert.Description, fmt.Sprintf("%.2f", score), "alert should contain actual score")
						break
					}
				}
				assert.True(t, foundAlert, "should find integrity score alert")
			} else {
				// Should not have integrity score alerts
				for _, alert := range alerts {
					assert.NotEqual(t, "integrity_score_low", alert.AlertType, "should not have integrity score alerts when above threshold")
				}
			}

			mockHashChain.AssertExpectations(t)
			mockMonitor.AssertExpectations(t)
		})
	}
}

// ==================== EDGE CASE AND ERROR CONDITION TESTS ====================

func TestIntegrityService_EdgeCases(t *testing.T) {
	t.Run("zero_sequence_range", func(t *testing.T) {
		service, _, _, _, _, _ := setupTestIntegrityService(t)
		ctx := context.Background()

		// Test with same start and end sequence
		seq := values.SequenceNumber(1)
		_, err := service.VerifyHashChain(ctx, seq, seq)
		assert.NoError(t, err, "single sequence verification should succeed")
	})

	t.Run("invalid_sequence_range", func(t *testing.T) {
		service, _, _, _, _, _ := setupTestIntegrityService(t)
		ctx := context.Background()

		// Test with start > end
		start := values.SequenceNumber(100)
		end := values.SequenceNumber(1)
		_, err := service.VerifyHashChain(ctx, start, end)
		assert.Error(t, err, "invalid range should fail")
		assert.Contains(t, err.Error(), "invalid", "error should mention invalid range")
	})

	t.Run("large_sequence_range", func(t *testing.T) {
		service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
		ctx := context.Background()

		// Test with very large range
		start := values.SequenceNumber(1)
		end := values.SequenceNumber(1000000)

		largeResult := &audit.HashChainVerificationResult{
			StartSequence:  start,
			EndSequence:    end,
			IsValid:        true,
			EventsVerified: 1000000,
			IntegrityScore: 1.0,
			VerifiedAt:     time.Now(),
			VerificationID: "large-range-test",
			Method:         "full",
		}

		mockHashChain.On("VerifyChain", ctx, start, end).Return(largeResult, nil)
		mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
		mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

		result, err := service.VerifyHashChain(ctx, start, end)
		require.NoError(t, err, "large range verification should succeed")
		assert.Equal(t, int64(1000000), result.EventsVerified, "should verify all events in large range")
	})

	t.Run("context_cancellation", func(t *testing.T) {
		service, mockHashChain, _, _, _, _ := setupTestIntegrityService(t)
		
		// Create context that's already cancelled
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		start := values.SequenceNumber(1)
		end := values.SequenceNumber(100)

		// Mock should handle context cancellation
		mockHashChain.On("VerifyChain", cancelledCtx, start, end).Return(
			(*audit.HashChainVerificationResult)(nil), context.Canceled)

		_, err := service.VerifyHashChain(cancelledCtx, start, end)
		assert.Error(t, err, "cancelled context should fail")
		assert.Equal(t, context.Canceled, err, "should return context cancellation error")
	})

	t.Run("timeout_handling", func(t *testing.T) {
		service, mockHashChain, _, _, _, _ := setupTestIntegrityService(t)
		
		// Create context with very short timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		
		// Allow context to timeout
		time.Sleep(1 * time.Millisecond)

		start := values.SequenceNumber(1)
		end := values.SequenceNumber(100)

		// Mock should handle timeout
		mockHashChain.On("VerifyChain", timeoutCtx, start, end).Return(
			(*audit.HashChainVerificationResult)(nil), context.DeadlineExceeded)

		_, err := service.VerifyHashChain(timeoutCtx, start, end)
		assert.Error(t, err, "timeout context should fail")
		assert.Equal(t, context.DeadlineExceeded, err, "should return timeout error")
	})
}

func TestIntegrityService_ErrorHandling(t *testing.T) {
	t.Run("service_dependency_failure", func(t *testing.T) {
		service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
		ctx := context.Background()

		start := values.SequenceNumber(1)
		end := values.SequenceNumber(100)

		// Mock dependency failure
		expectedError := errors.NewInternalError("database connection failed")
		mockHashChain.On("VerifyChain", ctx, start, end).Return(
			(*audit.HashChainVerificationResult)(nil), expectedError)
		mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
		mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

		result, err := service.VerifyHashChain(ctx, start, end)
		assert.Error(t, err, "dependency failure should propagate")
		assert.Nil(t, result, "result should be nil on error")
		assert.Equal(t, expectedError, err, "should return original error")

		// Verify error metrics
		assert.Equal(t, int64(1), service.metrics.TotalChecks, "should count failed checks")
		assert.Equal(t, int64(0), service.metrics.SuccessfulChecks, "should not count as successful")

		mockHashChain.AssertExpectations(t)
		mockMonitor.AssertExpectations(t)
	})

	t.Run("partial_verification_failure", func(t *testing.T) {
		service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(t)
		ctx := context.Background()

		start := values.SequenceNumber(1)
		end := values.SequenceNumber(100)

		// Result with partial failure
		partialResult := &audit.HashChainVerificationResult{
			StartSequence:  start,
			EndSequence:    end,
			IsValid:        false,
			EventsVerified: 75,  // Only 75 out of 100 verified
			HashesValid:    70,
			HashesInvalid:  5,
			IntegrityScore: 0.7,
			VerifiedAt:     time.Now(),
			VerificationID: "partial-failure-test",
			Method:         "full",
		}

		mockHashChain.On("VerifyChain", ctx, start, end).Return(partialResult, nil)
		mockMonitor.On("RecordCounter", "integrity_checks_total", float64(1), mock.AnythingOfType("map[string]string"))
		mockMonitor.On("RecordHistogram", "integrity_check_duration_seconds", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))

		result, err := service.VerifyHashChain(ctx, start, end)
		require.NoError(t, err, "should not return error for partial failure")
		require.NotNil(t, result, "should return result even with partial failure")
		assert.False(t, result.IsValid, "result should be invalid")
		assert.Equal(t, int64(75), result.EventsVerified, "should reflect partial verification")
		assert.Equal(t, 0.7, result.IntegrityScore, "should have reduced integrity score")

		mockHashChain.AssertExpectations(t)
		mockMonitor.AssertExpectations(t)
	})
}

// ==================== BENCHMARK TESTS FOR PERFORMANCE VALIDATION ====================

func BenchmarkIntegrityService_ConcurrentVerifications(b *testing.B) {
	service, mockHashChain, _, _, _, _ := setupTestIntegrityService(&testing.T{})
	ctx := context.Background()

	// Set up mock expectations for concurrent calls
	mockHashChain.On("VerifyChain", mock.Anything, mock.Anything, mock.Anything).Return(
		&audit.HashChainVerificationResult{
			IsValid:        true,
			EventsVerified: 100,
			IntegrityScore: 1.0,
			VerifiedAt:     time.Now(),
			VerificationID: "benchmark",
			Method:         "full",
		}, nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			start := values.SequenceNumber(1)
			end := values.SequenceNumber(100)
			_, err := service.VerifyHashChain(ctx, start, end)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkIntegrityService_LargeSequenceRanges(b *testing.B) {
	service, mockHashChain, _, _, _, _ := setupTestIntegrityService(&testing.T{})
	ctx := context.Background()

	// Test with increasingly large ranges
	ranges := []int{100, 1000, 10000, 100000}
	
	for _, rangeSize := range ranges {
		b.Run(fmt.Sprintf("range_%d", rangeSize), func(b *testing.B) {
			start := values.SequenceNumber(1)
			end := values.SequenceNumber(uint64(rangeSize))

			result := &audit.HashChainVerificationResult{
				StartSequence:  start,
				EndSequence:    end,
				IsValid:        true,
				EventsVerified: int64(rangeSize),
				IntegrityScore: 1.0,
				VerifiedAt:     time.Now(),
				VerificationID: fmt.Sprintf("benchmark-range-%d", rangeSize),
				Method:         "full",
			}

			mockHashChain.On("VerifyChain", ctx, start, end).Return(result, nil)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := service.VerifyHashChain(ctx, start, end)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkIntegrityService_MetricsRecording(b *testing.B) {
	service, mockHashChain, _, _, _, mockMonitor := setupTestIntegrityService(&testing.T{})
	ctx := context.Background()

	start := values.SequenceNumber(1)
	end := values.SequenceNumber(100)

	result := &audit.HashChainVerificationResult{
		StartSequence:  start,
		EndSequence:    end,
		IsValid:        true,
		EventsVerified: 100,
		IntegrityScore: 1.0,
		VerifiedAt:     time.Now(),
		VerificationID: "metrics-benchmark",
		Method:         "full",
	}

	mockHashChain.On("VerifyChain", mock.Anything, mock.Anything, mock.Anything).Return(result, nil)
	mockMonitor.On("RecordCounter", mock.Anything, mock.Anything, mock.Anything)
	mockMonitor.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.VerifyHashChain(ctx, start, end)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ==================== HELPER FUNCTIONS ====================

func createTestVerificationResult(start, end values.SequenceNumber, valid bool) *audit.HashChainVerificationResult {
	eventsCount := int64(end) - int64(start) + 1
	return &audit.HashChainVerificationResult{
		StartSequence:  start,
		EndSequence:    end,
		IsValid:        valid,
		EventsVerified: eventsCount,
		HashesValid:    eventsCount,
		HashesInvalid:  0,
		IntegrityScore: map[bool]float64{true: 1.0, false: 0.95}[valid],
		VerifiedAt:     time.Now(),
		VerificationID: uuid.New().String(),
		Method:         "full",
	}
}

func createTestCorruptionReport(corruptionCount int) *audit.CorruptionReport {
	corruptions := make([]*audit.CorruptionInstance, corruptionCount)
	for i := 0; i < corruptionCount; i++ {
		corruptions[i] = &audit.CorruptionInstance{
			CorruptionID:   uuid.New().String(),
			EventID:        uuid.New(),
			CorruptionType: []string{"hash_mismatch", "missing_field", "invalid_timestamp"}[i%3],
			Severity:       []string{"low", "medium", "high", "critical"}[i%4],
			RepairPossible: i%2 == 0,
			DetectedAt:     time.Now(),
		}
	}

	return &audit.CorruptionReport{
		ReportID:        uuid.New().String(),
		CorruptionFound: corruptionCount > 0,
		CorruptionLevel: map[bool]string{true: "detected", false: "none"}[corruptionCount > 0],
		EventsScanned:   1000,
		EventsCorrupted: int64(corruptionCount),
		Corruptions:     corruptions,
		ScannedAt:       time.Now(),
	}
}
