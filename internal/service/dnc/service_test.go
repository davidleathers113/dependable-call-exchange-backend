package dnc

import (
	"context"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Mock implementations

type MockDNCEntryRepository struct {
	mock.Mock
}

func (m *MockDNCEntryRepository) Save(ctx context.Context, entry *dnc.DNCEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockDNCEntryRepository) SaveWithTx(ctx context.Context, tx dnc.Transaction, entry *dnc.DNCEntry) error {
	args := m.Called(ctx, tx, entry)
	return args.Error(0)
}

func (m *MockDNCEntryRepository) GetByID(ctx context.Context, id uuid.UUID) (*dnc.DNCEntry, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dnc.DNCEntry), args.Error(1)
}

func (m *MockDNCEntryRepository) Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	args := m.Called(ctx, id, deletedBy)
	return args.Error(0)
}

func (m *MockDNCEntryRepository) FindByPhone(ctx context.Context, phoneNumber *values.PhoneNumber) ([]*dnc.DNCEntry, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dnc.DNCEntry), args.Error(1)
}

func (m *MockDNCEntryRepository) FindWithFilter(ctx context.Context, filter dnc.DNCEntryFilter) ([]*dnc.DNCEntry, int, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*dnc.DNCEntry), args.Int(1), args.Error(2)
}

func (m *MockDNCEntryRepository) CountByListSource(ctx context.Context, source values.ListSource) (int, error) {
	args := m.Called(ctx, source)
	return args.Int(0), args.Error(1)
}

func (m *MockDNCEntryRepository) BulkInsert(ctx context.Context, entries []*dnc.DNCEntry) error {
	args := m.Called(ctx, entries)
	return args.Error(0)
}

func (m *MockDNCEntryRepository) BulkInsertWithTx(ctx context.Context, tx dnc.Transaction, entries []*dnc.DNCEntry) error {
	args := m.Called(ctx, tx, entries)
	return args.Error(0)
}

func (m *MockDNCEntryRepository) CleanupExpired(ctx context.Context, batchSize int) (int, error) {
	args := m.Called(ctx, batchSize)
	return args.Int(0), args.Error(1)
}

type MockDNCProviderRepository struct {
	mock.Mock
}

func (m *MockDNCProviderRepository) Save(ctx context.Context, provider *dnc.DNCProvider) error {
	args := m.Called(ctx, provider)
	return args.Error(0)
}

func (m *MockDNCProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*dnc.DNCProvider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dnc.DNCProvider), args.Error(1)
}

func (m *MockDNCProviderRepository) FindByStatus(ctx context.Context, status dnc.ProviderStatus) ([]*dnc.DNCProvider, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dnc.DNCProvider), args.Error(1)
}

func (m *MockDNCProviderRepository) FindNeedingSync(ctx context.Context) ([]*dnc.DNCProvider, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dnc.DNCProvider), args.Error(1)
}

func (m *MockDNCProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockDNCCheckResultRepository struct {
	mock.Mock
}

func (m *MockDNCCheckResultRepository) Save(ctx context.Context, result *dnc.DNCCheckResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *MockDNCCheckResultRepository) GetByPhone(ctx context.Context, phoneNumber *values.PhoneNumber) (*dnc.DNCCheckResult, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dnc.DNCCheckResult), args.Error(1)
}

func (m *MockDNCCheckResultRepository) FindExpired(ctx context.Context, limit int) ([]*dnc.DNCCheckResult, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dnc.DNCCheckResult), args.Error(1)
}

func (m *MockDNCCheckResultRepository) DeleteExpired(ctx context.Context, before time.Time) (int, error) {
	args := m.Called(ctx, before)
	return args.Int(0), args.Error(1)
}

type MockDNCCache struct {
	mock.Mock
}

func (m *MockDNCCache) GetCheckResult(ctx context.Context, phoneNumber *values.PhoneNumber) (*dnc.DNCCheckResult, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dnc.DNCCheckResult), args.Error(1)
}

func (m *MockDNCCache) SetCheckResult(ctx context.Context, result *dnc.DNCCheckResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *MockDNCCache) InvalidateProvider(ctx context.Context, providerID uuid.UUID) error {
	args := m.Called(ctx, providerID)
	return args.Error(0)
}

func (m *MockDNCCache) InvalidateSource(ctx context.Context, source values.ListSource) error {
	args := m.Called(ctx, source)
	return args.Error(0)
}

func (m *MockDNCCache) GetStats(ctx context.Context) (*CacheStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CacheStats), args.Error(1)
}

func (m *MockDNCCache) Clear(ctx context.Context, pattern string) error {
	args := m.Called(ctx, pattern)
	return args.Error(0)
}

func (m *MockDNCCache) WarmCache(ctx context.Context, phoneNumbers []*values.PhoneNumber) error {
	args := m.Called(ctx, phoneNumbers)
	return args.Error(0)
}

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishDNCCheckPerformed(ctx context.Context, event *dnc.DNCCheckPerformedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishNumberSuppressed(ctx context.Context, event *dnc.NumberSuppressedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishNumberReleased(ctx context.Context, event *dnc.NumberReleasedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishDNCListSynced(ctx context.Context, event *dnc.DNCListSyncedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// Test helpers

func createTestService(t *testing.T) (*service, *MockDNCEntryRepository, *MockDNCProviderRepository, *MockDNCCheckResultRepository, *MockDNCCache, *MockEventPublisher) {
	logger := zaptest.NewLogger(t)
	config := &Config{
		CheckTimeoutMs:        10,
		BulkCheckTimeoutMs:    50,
		CacheDefaultTTL:       6 * time.Hour,
		SlowQueryThresholdMs:  5,
	}

	entryRepo := &MockDNCEntryRepository{}
	providerRepo := &MockDNCProviderRepository{}
	checkResultRepo := &MockDNCCheckResultRepository{}
	cache := &MockDNCCache{}
	eventPublisher := &MockEventPublisher{}

	svc, err := NewService(logger, config, entryRepo, providerRepo, checkResultRepo, cache, eventPublisher)
	require.NoError(t, err)

	return svc.(*service), entryRepo, providerRepo, checkResultRepo, cache, eventPublisher
}

func createTestPhoneNumber(t *testing.T) *values.PhoneNumber {
	phoneNumber, err := values.NewPhoneNumber("+14155551234")
	require.NoError(t, err)
	return phoneNumber
}

func createTestDNCEntry(t *testing.T, phoneNumber *values.PhoneNumber) *dnc.DNCEntry {
	if phoneNumber == nil {
		phoneNumber = createTestPhoneNumber(t)
	}

	entry, err := dnc.NewDNCEntry(
		phoneNumber,
		values.ListSourceFederal,
		values.FederalDNCSuppressReason(),
		nil, // No expiration
		uuid.New(),
		"Test entry",
		nil,
	)
	require.NoError(t, err)
	return entry
}

func createTestDNCCheckResult(t *testing.T, phoneNumber *values.PhoneNumber, isBlocked bool) *dnc.DNCCheckResult {
	if phoneNumber == nil {
		phoneNumber = createTestPhoneNumber(t)
	}

	result, err := dnc.NewDNCCheckResult(
		phoneNumber,
		time.Now(),
		1*time.Hour,
		dnc.ComplianceLevelStandard,
	)
	require.NoError(t, err)

	if isBlocked {
		blockReason := dnc.BlockReason{
			Source:      values.ListSourceFederal,
			Reason:      values.FederalDNCSuppressReason(),
			AddedAt:     time.Now(),
			Description: "Federal DNC listing",
		}
		result.AddBlockReason(blockReason)
	}

	return result
}

// Tests

func TestNewService(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &Config{}
	entryRepo := &MockDNCEntryRepository{}
	providerRepo := &MockDNCProviderRepository{}
	checkResultRepo := &MockDNCCheckResultRepository{}
	cache := &MockDNCCache{}
	eventPublisher := &MockEventPublisher{}

	tests := []struct {
		name        string
		logger      *zap.Logger
		config      *Config
		entryRepo   DNCEntryRepository
		expectError bool
		errorCode   string
	}{
		{
			name:        "valid service creation",
			logger:      logger,
			config:      config,
			entryRepo:   entryRepo,
			expectError: false,
		},
		{
			name:        "nil logger",
			logger:      nil,
			config:      config,
			entryRepo:   entryRepo,
			expectError: true,
			errorCode:   "INVALID_LOGGER",
		},
		{
			name:        "nil config",
			logger:      logger,
			config:      nil,
			entryRepo:   entryRepo,
			expectError: true,
			errorCode:   "INVALID_CONFIG",
		},
		{
			name:        "nil entry repository",
			logger:      logger,
			config:      config,
			entryRepo:   nil,
			expectError: true,
			errorCode:   "INVALID_ENTRY_REPO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewService(tt.logger, tt.config, tt.entryRepo, providerRepo, checkResultRepo, cache, eventPublisher)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, svc)
				if tt.errorCode != "" {
					appErr, ok := err.(*errors.AppError)
					assert.True(t, ok)
					assert.Equal(t, tt.errorCode, appErr.Code)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, svc)
			}
		})
	}
}

func TestService_CheckDNC(t *testing.T) {
	svc, entryRepo, _, _, cache, _ := createTestService(t)
	ctx := context.Background()
	phoneNumber := createTestPhoneNumber(t)
	callTime := time.Now()

	tests := []struct {
		name           string
		phoneNumber    *values.PhoneNumber
		callTime       time.Time
		setupMocks     func()
		expectedBlocked bool
		expectError    bool
	}{
		{
			name:        "cache hit - not blocked",
			phoneNumber: phoneNumber,
			callTime:    callTime,
			setupMocks: func() {
				// Setup cache hit
				cachedResult := createTestDNCCheckResult(t, phoneNumber, false)
				cache.On("GetCheckResult", mock.Anything, phoneNumber).Return(cachedResult, nil)
			},
			expectedBlocked: false,
			expectError:     false,
		},
		{
			name:        "cache miss - blocked entry",
			phoneNumber: phoneNumber,
			callTime:    callTime,
			setupMocks: func() {
				// Setup cache miss
				cache.On("GetCheckResult", mock.Anything, phoneNumber).Return(nil, errors.NewNotFoundError("NOT_FOUND", "not in cache"))

				// Setup repository response with blocked entry
				entry := createTestDNCEntry(t, phoneNumber)
				entryRepo.On("FindByPhone", mock.Anything, phoneNumber).Return([]*dnc.DNCEntry{entry}, nil)

				// Setup cache set (async)
				cache.On("SetCheckResult", mock.Anything, mock.AnythingOfType("*dnc.DNCCheckResult")).Return(nil)
			},
			expectedBlocked: true,
			expectError:     false,
		},
		{
			name:        "cache miss - not blocked",
			phoneNumber: phoneNumber,
			callTime:    callTime,
			setupMocks: func() {
				// Setup cache miss
				cache.On("GetCheckResult", mock.Anything, phoneNumber).Return(nil, errors.NewNotFoundError("NOT_FOUND", "not in cache"))

				// Setup repository response with no entries
				entryRepo.On("FindByPhone", mock.Anything, phoneNumber).Return([]*dnc.DNCEntry{}, nil)

				// Setup cache set (async)
				cache.On("SetCheckResult", mock.Anything, mock.AnythingOfType("*dnc.DNCCheckResult")).Return(nil)
			},
			expectedBlocked: false,
			expectError:     false,
		},
		{
			name:        "repository error",
			phoneNumber: phoneNumber,
			callTime:    callTime,
			setupMocks: func() {
				// Setup cache miss
				cache.On("GetCheckResult", mock.Anything, phoneNumber).Return(nil, errors.NewNotFoundError("NOT_FOUND", "not in cache"))

				// Setup repository error
				entryRepo.On("FindByPhone", mock.Anything, phoneNumber).Return(nil, errors.NewInternalError("DB_ERROR", "database connection failed"))
			},
			expectedBlocked: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			cache.ExpectedCalls = nil
			entryRepo.ExpectedCalls = nil

			tt.setupMocks()

			result, err := svc.CheckDNC(ctx, tt.phoneNumber, tt.callTime)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedBlocked, result.IsBlocked)
				assert.Equal(t, tt.phoneNumber, result.PhoneNumber)
			}

			// Verify mock expectations
			cache.AssertExpectations(t)
			entryRepo.AssertExpectations(t)
		})
	}
}

func TestService_CheckDNCBulk(t *testing.T) {
	svc, entryRepo, _, _, cache, _ := createTestService(t)
	ctx := context.Background()
	callTime := time.Now()

	phoneNumber1 := createTestPhoneNumber(t)
	phoneNumber2, _ := values.NewPhoneNumber("+14155555678")
	phoneNumbers := []*values.PhoneNumber{phoneNumber1, phoneNumber2}

	t.Run("mixed cache hits and misses", func(t *testing.T) {
		// Setup cache: first number hit, second number miss
		cachedResult1 := createTestDNCCheckResult(t, phoneNumber1, false)
		cache.On("GetCheckResult", mock.Anything, phoneNumber1).Return(cachedResult1, nil)
		cache.On("GetCheckResult", mock.Anything, phoneNumber2).Return(nil, errors.NewNotFoundError("NOT_FOUND", "not in cache"))

		// Setup repository for uncached number
		entry2 := createTestDNCEntry(t, phoneNumber2)
		entryRepo.On("FindByPhone", mock.Anything, phoneNumber2).Return([]*dnc.DNCEntry{entry2}, nil)

		// Setup cache set for uncached result
		cache.On("SetCheckResult", mock.Anything, mock.AnythingOfType("*dnc.DNCCheckResult")).Return(nil)

		results, err := svc.CheckDNCBulk(ctx, phoneNumbers, callTime)

		assert.NoError(t, err)
		require.Len(t, results, 2)

		// First result should be from cache (not blocked)
		assert.False(t, results[0].IsBlocked)
		assert.True(t, results[0].CachedResult)

		// Second result should be from repository (blocked)
		assert.True(t, results[1].IsBlocked)
		assert.False(t, results[1].CachedResult)

		cache.AssertExpectations(t)
		entryRepo.AssertExpectations(t)
	})

	t.Run("empty phone numbers list", func(t *testing.T) {
		results, err := svc.CheckDNCBulk(ctx, []*values.PhoneNumber{}, callTime)

		assert.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestService_AddToSuppressionList(t *testing.T) {
	svc, entryRepo, _, _, cache, eventPublisher := createTestService(t)
	ctx := context.Background()
	phoneNumber := createTestPhoneNumber(t)
	userID := uuid.New()

	tests := []struct {
		name        string
		request     AddSuppressionRequest
		setupMocks  func()
		expectError bool
		errorCode   string
	}{
		{
			name: "successful addition",
			request: AddSuppressionRequest{
				PhoneNumber:    phoneNumber,
				ListSource:     values.ListSourceInternal,
				SuppressReason: values.CompanyPolicySuppressReason(),
				AddedBy:        userID,
				Notes:          "Test suppression",
			},
			setupMocks: func() {
				entryRepo.On("Save", mock.Anything, mock.AnythingOfType("*dnc.DNCEntry")).Return(nil)
				cache.On("InvalidateSource", mock.Anything, values.ListSourceInternal).Return(nil)
				eventPublisher.On("PublishNumberSuppressed", mock.Anything, mock.AnythingOfType("*dnc.NumberSuppressedEvent")).Return(nil)
			},
			expectError: false,
		},
		{
			name: "nil phone number",
			request: AddSuppressionRequest{
				PhoneNumber:    nil,
				ListSource:     values.ListSourceInternal,
				SuppressReason: values.CompanyPolicySuppressReason(),
				AddedBy:        userID,
			},
			setupMocks: func() {},
			expectError: true,
			errorCode:   "INVALID_PHONE_NUMBER",
		},
		{
			name: "nil user ID",
			request: AddSuppressionRequest{
				PhoneNumber:    phoneNumber,
				ListSource:     values.ListSourceInternal,
				SuppressReason: values.CompanyPolicySuppressReason(),
				AddedBy:        uuid.Nil,
			},
			setupMocks: func() {},
			expectError: true,
			errorCode:   "INVALID_USER_ID",
		},
		{
			name: "repository save error",
			request: AddSuppressionRequest{
				PhoneNumber:    phoneNumber,
				ListSource:     values.ListSourceInternal,
				SuppressReason: values.CompanyPolicySuppressReason(),
				AddedBy:        userID,
			},
			setupMocks: func() {
				entryRepo.On("Save", mock.Anything, mock.AnythingOfType("*dnc.DNCEntry")).Return(errors.NewInternalError("DB_ERROR", "save failed"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			entryRepo.ExpectedCalls = nil
			cache.ExpectedCalls = nil
			eventPublisher.ExpectedCalls = nil

			tt.setupMocks()

			result, err := svc.AddToSuppressionList(ctx, tt.request)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errorCode != "" {
					appErr, ok := err.(*errors.AppError)
					assert.True(t, ok)
					assert.Equal(t, tt.errorCode, appErr.Code)
				}
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.request.PhoneNumber, result.PhoneNumber)
				assert.Equal(t, tt.request.ListSource, result.ListSource)
				assert.Equal(t, tt.request.SuppressReason, result.SuppressReason)
				assert.Equal(t, tt.request.AddedBy, result.AddedBy)
			}

			// Verify expectations for successful cases
			if !tt.expectError {
				// Give async operations time to complete
				time.Sleep(10 * time.Millisecond)
			}

			entryRepo.AssertExpectations(t)
		})
	}
}

func TestService_RemoveFromSuppressionList(t *testing.T) {
	svc, entryRepo, _, _, cache, eventPublisher := createTestService(t)
	ctx := context.Background()
	phoneNumber := createTestPhoneNumber(t)
	userID := uuid.New()

	tests := []struct {
		name        string
		phoneNumber *values.PhoneNumber
		removedBy   uuid.UUID
		reason      string
		setupMocks  func()
		expectError bool
		errorCode   string
	}{
		{
			name:        "successful removal",
			phoneNumber: phoneNumber,
			removedBy:   userID,
			reason:      "Customer request",
			setupMocks: func() {
				// Return active internal entry
				entry := createTestDNCEntry(t, phoneNumber)
				entry.ListSource = values.ListSourceInternal
				entryRepo.On("FindByPhone", mock.Anything, phoneNumber).Return([]*dnc.DNCEntry{entry}, nil)
				entryRepo.On("Delete", mock.Anything, entry.ID, userID).Return(nil)
				cache.On("InvalidateSource", mock.Anything, values.ListSourceInternal).Return(nil)
				eventPublisher.On("PublishNumberReleased", mock.Anything, mock.AnythingOfType("*dnc.NumberReleasedEvent")).Return(nil)
			},
			expectError: false,
		},
		{
			name:        "nil phone number",
			phoneNumber: nil,
			removedBy:   userID,
			reason:      "Test",
			setupMocks:  func() {},
			expectError: true,
			errorCode:   "INVALID_PHONE_NUMBER",
		},
		{
			name:        "nil user ID",
			phoneNumber: phoneNumber,
			removedBy:   uuid.Nil,
			reason:      "Test",
			setupMocks:  func() {},
			expectError: true,
			errorCode:   "INVALID_USER_ID",
		},
		{
			name:        "no entries found",
			phoneNumber: phoneNumber,
			removedBy:   userID,
			reason:      "Test",
			setupMocks: func() {
				entryRepo.On("FindByPhone", mock.Anything, phoneNumber).Return([]*dnc.DNCEntry{}, nil)
			},
			expectError: true,
			errorCode:   "ENTRY_NOT_FOUND",
		},
		{
			name:        "no active internal entries",
			phoneNumber: phoneNumber,
			removedBy:   userID,
			reason:      "Test",
			setupMocks: func() {
				// Return federal entry (not internal)
				entry := createTestDNCEntry(t, phoneNumber)
				entry.ListSource = values.ListSourceFederal
				entryRepo.On("FindByPhone", mock.Anything, phoneNumber).Return([]*dnc.DNCEntry{entry}, nil)
			},
			expectError: true,
			errorCode:   "NO_ACTIVE_ENTRIES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mocks
			entryRepo.ExpectedCalls = nil
			cache.ExpectedCalls = nil
			eventPublisher.ExpectedCalls = nil

			tt.setupMocks()

			err := svc.RemoveFromSuppressionList(ctx, tt.phoneNumber, tt.removedBy, tt.reason)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != "" {
					appErr, ok := err.(*errors.AppError)
					assert.True(t, ok)
					assert.Equal(t, tt.errorCode, appErr.Code)
				}
			} else {
				assert.NoError(t, err)
				// Give async operations time to complete
				time.Sleep(10 * time.Millisecond)
			}

			entryRepo.AssertExpectations(t)
		})
	}
}

func TestService_SyncWithProviders(t *testing.T) {
	svc, _, providerRepo, _, _, eventPublisher := createTestService(t)
	ctx := context.Background()

	t.Run("no active providers", func(t *testing.T) {
		providerRepo.On("FindByStatus", mock.Anything, dnc.ProviderStatusActive).Return([]*dnc.DNCProvider{}, nil)

		response, err := svc.SyncWithProviders(ctx)

		assert.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, 0, response.TotalProviders)
		assert.Equal(t, 0, response.SuccessCount)
		assert.Equal(t, 0, response.FailureCount)

		providerRepo.AssertExpectations(t)
	})

	t.Run("repository error", func(t *testing.T) {
		providerRepo.ExpectedCalls = nil
		providerRepo.On("FindByStatus", mock.Anything, dnc.ProviderStatusActive).Return(nil, errors.NewInternalError("DB_ERROR", "failed to get providers"))

		response, err := svc.SyncWithProviders(ctx)

		assert.Error(t, err)
		assert.Nil(t, response)

		providerRepo.AssertExpectations(t)
	})
}

func TestService_HealthCheck(t *testing.T) {
	svc, entryRepo, _, _, cache, _ := createTestService(t)
	ctx := context.Background()

	t.Run("all dependencies healthy", func(t *testing.T) {
		// Setup database health check (returns not found, which indicates connectivity)
		entryRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil, errors.NewNotFoundError("NOT_FOUND", "entry not found"))

		// Setup cache health check
		cacheStats := &CacheStats{
			Hits:   100,
			Misses: 10,
		}
		cache.On("GetStats", mock.Anything).Return(cacheStats, nil)

		response, err := svc.HealthCheck(ctx)

		assert.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, "healthy", response.Status)
		assert.NotEmpty(t, response.Dependencies)

		// Check that database dependency is present
		var dbHealth *DependencyHealth
		for _, dep := range response.Dependencies {
			if dep.Name == "database" {
				dbHealth = &dep
				break
			}
		}
		require.NotNil(t, dbHealth)
		assert.Equal(t, "healthy", dbHealth.Status)

		entryRepo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})
}

// Benchmark tests

func BenchmarkService_CheckDNC_CacheHit(b *testing.B) {
	svc, _, _, _, cache, _ := createTestService(b)
	ctx := context.Background()
	phoneNumber := createTestPhoneNumber(b)
	callTime := time.Now()

	// Setup cache hit
	cachedResult := createTestDNCCheckResult(b, phoneNumber, false)
	cache.On("GetCheckResult", mock.Anything, phoneNumber).Return(cachedResult, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.CheckDNC(ctx, phoneNumber, callTime)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkService_CheckDNCBulk(b *testing.B) {
	svc, entryRepo, _, _, cache, _ := createTestService(b)
	ctx := context.Background()
	callTime := time.Now()

	// Create test phone numbers
	phoneNumbers := make([]*values.PhoneNumber, 100)
	for i := 0; i < 100; i++ {
		phoneNumber, _ := values.NewPhoneNumber(fmt.Sprintf("+1415555%04d", i))
		phoneNumbers[i] = phoneNumber

		// Setup cache miss and repository response
		cache.On("GetCheckResult", mock.Anything, phoneNumber).Return(nil, errors.NewNotFoundError("NOT_FOUND", "not in cache"))
		entryRepo.On("FindByPhone", mock.Anything, phoneNumber).Return([]*dnc.DNCEntry{}, nil)
		cache.On("SetCheckResult", mock.Anything, mock.AnythingOfType("*dnc.DNCCheckResult")).Return(nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.CheckDNCBulk(ctx, phoneNumbers, callTime)
		if err != nil {
			b.Fatal(err)
		}
	}
}