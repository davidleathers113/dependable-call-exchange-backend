package telephony

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations

type MockCallRepository struct {
	mock.Mock
}

func (m *MockCallRepository) GetByID(ctx context.Context, callID uuid.UUID) (*call.Call, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*call.Call), args.Error(1)
}

func (m *MockCallRepository) GetByCallSID(ctx context.Context, callSID string) (*call.Call, error) {
	args := m.Called(ctx, callSID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*call.Call), args.Error(1)
}

func (m *MockCallRepository) Update(ctx context.Context, c *call.Call) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *MockCallRepository) Create(ctx context.Context, c *call.Call) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) InitiateCall(ctx context.Context, from, to string, callbackURL string) (string, error) {
	args := m.Called(ctx, from, to, callbackURL)
	return args.String(0), args.Error(1)
}

func (m *MockProvider) TerminateCall(ctx context.Context, callSID string) error {
	args := m.Called(ctx, callSID)
	return args.Error(0)
}

func (m *MockProvider) GetCallStatus(ctx context.Context, callSID string) (*ProviderCallStatus, error) {
	args := m.Called(ctx, callSID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ProviderCallStatus), args.Error(1)
}

func (m *MockProvider) TransferCall(ctx context.Context, callSID string, to string) error {
	args := m.Called(ctx, callSID, to)
	return args.Error(0)
}

func (m *MockProvider) SendDTMF(ctx context.Context, callSID string, digits string) error {
	args := m.Called(ctx, callSID, digits)
	return args.Error(0)
}

func (m *MockProvider) BridgeCalls(ctx context.Context, callSID1, callSID2 string) error {
	args := m.Called(ctx, callSID1, callSID2)
	return args.Error(0)
}

func (m *MockProvider) GetProviderName() string {
	args := m.Called()
	return args.String(0)
}

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishCallEvent(ctx context.Context, event *CallEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) RecordCallInitiated(ctx context.Context, provider string) {
	m.Called(ctx, provider)
}

func (m *MockMetricsCollector) RecordCallCompleted(ctx context.Context, duration time.Duration, cost float64) {
	m.Called(ctx, duration, cost)
}

func (m *MockMetricsCollector) RecordCallFailed(ctx context.Context, reason string) {
	m.Called(ctx, reason)
}

func (m *MockMetricsCollector) RecordProviderLatency(ctx context.Context, provider string, operation string, latency time.Duration) {
	m.Called(ctx, provider, operation, latency)
}

// Tests

func TestService_InitiateCall(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMocks    func(*MockCallRepository, *MockProvider, *MockEventPublisher, *MockMetricsCollector)
		request       *InitiateCallRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *CallResponse)
	}{
		{
			name: "successful call initiation",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher, m *MockMetricsCollector) {
				cr.On("Create", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
				p.On("GetProviderName").Return("twilio")
				p.On("InitiateCall", ctx, "+11234567890", "+19876543210", "https://callback.url").
					Return("CALL123", nil)
				cr.On("Update", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
				m.On("RecordProviderLatency", ctx, "twilio", "initiate_call", mock.AnythingOfType("time.Duration"))
				m.On("RecordCallInitiated", ctx, "twilio")
				ep.On("PublishCallEvent", ctx, mock.AnythingOfType("*telephony.CallEvent")).Return(nil)
			},
			request: &InitiateCallRequest{
				FromNumber:  "+11234567890",
				ToNumber:    "+19876543210",
				BuyerID:     uuid.New(),
				CallbackURL: "https://callback.url",
			},
			expectedError: false,
			validate: func(t *testing.T, resp *CallResponse) {
				assert.NotNil(t, resp)
				assert.NotEqual(t, uuid.Nil, resp.CallID)
				assert.Equal(t, "CALL123", resp.CallSID)
				assert.Equal(t, call.StatusQueued, resp.Status)
				assert.Equal(t, "twilio", resp.Provider)
			},
		},
		{
			name: "nil request",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher, m *MockMetricsCollector) {
				// No mocks needed
			},
			request:       nil,
			expectedError: true,
			errorContains: "request cannot be nil",
		},
		{
			name: "invalid from number",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher, m *MockMetricsCollector) {
				// No mocks needed
			},
			request: &InitiateCallRequest{
				FromNumber: "",
				ToNumber:   "+19876543210",
				BuyerID:    uuid.New(),
			},
			expectedError: true,
			errorContains: "from number is required",
		},
		{
			name: "provider failure",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher, m *MockMetricsCollector) {
				cr.On("Create", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
				p.On("InitiateCall", ctx, "+11234567890", "+19876543210", "").
					Return("", fmt.Errorf("provider error"))
				cr.On("Update", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
				m.On("RecordCallFailed", ctx, "provider error")
			},
			request: &InitiateCallRequest{
				FromNumber: "+11234567890",
				ToNumber:   "+19876543210",
				BuyerID:    uuid.New(),
			},
			expectedError: true,
			errorContains: "failed to initiate call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(MockCallRepository)
			provider := new(MockProvider)
			eventPub := new(MockEventPublisher)
			metrics := new(MockMetricsCollector)

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(callRepo, provider, eventPub, metrics)
			}

			// Create service
			svc := NewService(callRepo, provider, eventPub, metrics)

			// Execute
			resp, err := svc.InitiateCall(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, resp)
				}
			}

			// Assert expectations
			callRepo.AssertExpectations(t)
			provider.AssertExpectations(t)
			eventPub.AssertExpectations(t)
			metrics.AssertExpectations(t)
		})
	}
}

func TestService_TerminateCall(t *testing.T) {
	ctx := context.Background()
	callID := uuid.New()

	tests := []struct {
		name          string
		setupMocks    func(*MockCallRepository, *MockProvider, *MockEventPublisher)
		callID        uuid.UUID
		expectedError bool
		errorContains string
	}{
		{
			name: "successful termination",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher) {
				testCall := &call.Call{
					ID:        callID,
					CallSID:   "CALL123",
					Status:    call.StatusInProgress,
					StartTime: time.Now().Add(-2 * time.Minute),
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				p.On("TerminateCall", ctx, "CALL123").Return(nil)
				cr.On("Update", ctx, mock.MatchedBy(func(c *call.Call) bool {
					return c.Status == call.StatusCanceled && c.EndTime != nil
				})).Return(nil)
				ep.On("PublishCallEvent", ctx, mock.AnythingOfType("*telephony.CallEvent")).Return(nil)
			},
			callID:        callID,
			expectedError: false,
		},
		{
			name: "call not found",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher) {
				cr.On("GetByID", ctx, callID).Return(nil, fmt.Errorf("not found"))
			},
			callID:        callID,
			expectedError: true,
			errorContains: "not found",
		},
		{
			name: "invalid call state",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher) {
				testCall := &call.Call{
					ID:      callID,
					CallSID: "CALL123",
					Status:  call.StatusCompleted,
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
			},
			callID:        callID,
			expectedError: true,
			errorContains: "cannot be terminated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(MockCallRepository)
			provider := new(MockProvider)
			eventPub := new(MockEventPublisher)

			// Setup mocks
			tt.setupMocks(callRepo, provider, eventPub)

			// Create service
			svc := NewService(callRepo, provider, eventPub, nil)

			// Execute
			err := svc.TerminateCall(ctx, tt.callID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}

			// Assert expectations
			callRepo.AssertExpectations(t)
			provider.AssertExpectations(t)
			eventPub.AssertExpectations(t)
		})
	}
}

func TestService_GetCallStatus(t *testing.T) {
	ctx := context.Background()
	callID := uuid.New()

	tests := []struct {
		name          string
		setupMocks    func(*MockCallRepository, *MockProvider)
		callID        uuid.UUID
		expectedError bool
		errorContains string
		validate      func(*testing.T, *CallStatus)
	}{
		{
			name: "get status of active call",
			setupMocks: func(cr *MockCallRepository, p *MockProvider) {
				duration := 300
				cost := values.MustNewMoneyFromFloat(0.05, "USD")
				testCall := &call.Call{
					ID:        callID,
					CallSID:   "CALL123",
					Status:    call.StatusInProgress,
					StartTime: time.Now().Add(-5 * time.Minute),
					Duration:  &duration,
					Cost:      &cost,
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)

				providerStatus := &ProviderCallStatus{
					CallSID:  "CALL123",
					Status:   "in-progress",
					Duration: 300,
					Price:    floatPtr(0.05),
				}
				p.On("GetCallStatus", ctx, "CALL123").Return(providerStatus, nil)
				cr.On("Update", ctx, mock.AnythingOfType("*call.Call")).Maybe().Return(nil)
			},
			callID:        callID,
			expectedError: false,
			validate: func(t *testing.T, status *CallStatus) {
				assert.NotNil(t, status)
				assert.Equal(t, callID, status.CallID)
				assert.Equal(t, call.StatusInProgress, status.Status)
				require.NotNil(t, status.Duration)
				assert.Equal(t, 300, *status.Duration)
				require.NotNil(t, status.Cost)
				assert.Equal(t, 0.05, *status.Cost)
			},
		},
		{
			name: "get status of completed call",
			setupMocks: func(cr *MockCallRepository, p *MockProvider) {
				endTime := time.Now()
				testCall := &call.Call{
					ID:        callID,
					CallSID:   "CALL123",
					Status:    call.StatusCompleted,
					StartTime: time.Now().Add(-10 * time.Minute),
					EndTime:   &endTime,
					Duration:  intPtr(600),
					Cost:      moneyPtr(values.MustNewMoneyFromFloat(0.10, "USD")),
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				// No provider call for completed calls
			},
			callID:        callID,
			expectedError: false,
			validate: func(t *testing.T, status *CallStatus) {
				assert.NotNil(t, status)
				assert.Equal(t, call.StatusCompleted, status.Status)
				assert.NotNil(t, status.Duration)
				assert.Equal(t, 600, *status.Duration)
				assert.NotNil(t, status.Cost)
				assert.Equal(t, 0.10, *status.Cost)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(MockCallRepository)
			provider := new(MockProvider)

			// Setup mocks
			tt.setupMocks(callRepo, provider)

			// Create service
			svc := NewService(callRepo, provider, nil, nil)

			// Execute
			status, err := svc.GetCallStatus(ctx, tt.callID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, status)
				}
			}

			// Assert expectations
			callRepo.AssertExpectations(t)
			provider.AssertExpectations(t)
		})
	}
}

func TestService_TransferCall(t *testing.T) {
	ctx := context.Background()
	callID := uuid.New()

	tests := []struct {
		name          string
		setupMocks    func(*MockCallRepository, *MockProvider, *MockEventPublisher)
		callID        uuid.UUID
		toNumber      string
		expectedError bool
		errorContains string
	}{
		{
			name: "successful transfer",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher) {
				testCall := &call.Call{
					ID:      callID,
					CallSID: "CALL123",
					Status:  call.StatusInProgress,
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				p.On("TransferCall", ctx, "CALL123", "+19876543210").Return(nil)
				ep.On("PublishCallEvent", ctx, mock.AnythingOfType("*telephony.CallEvent")).Return(nil)
			},
			callID:        callID,
			toNumber:      "+19876543210",
			expectedError: false,
		},
		{
			name: "call not in progress",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher) {
				testCall := &call.Call{
					ID:      callID,
					CallSID: "CALL123",
					Status:  call.StatusQueued,
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
			},
			callID:        callID,
			toNumber:      "+19876543210",
			expectedError: true,
			errorContains: "must be in progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(MockCallRepository)
			provider := new(MockProvider)
			eventPub := new(MockEventPublisher)

			// Setup mocks
			tt.setupMocks(callRepo, provider, eventPub)

			// Create service
			svc := NewService(callRepo, provider, eventPub, nil)

			// Execute
			err := svc.TransferCall(ctx, tt.callID, tt.toNumber)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}

			// Assert expectations
			callRepo.AssertExpectations(t)
			provider.AssertExpectations(t)
			eventPub.AssertExpectations(t)
		})
	}
}

func TestService_BridgeCalls(t *testing.T) {
	ctx := context.Background()
	callID1 := uuid.New()
	callID2 := uuid.New()

	tests := []struct {
		name          string
		setupMocks    func(*MockCallRepository, *MockProvider, *MockEventPublisher)
		callID1       uuid.UUID
		callID2       uuid.UUID
		expectedError bool
		errorContains string
	}{
		{
			name: "successful bridge",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher) {
				call1 := &call.Call{
					ID:      callID1,
					CallSID: "CALL123",
					Status:  call.StatusInProgress,
				}
				call2 := &call.Call{
					ID:      callID2,
					CallSID: "CALL456",
					Status:  call.StatusInProgress,
				}
				cr.On("GetByID", ctx, callID1).Return(call1, nil)
				cr.On("GetByID", ctx, callID2).Return(call2, nil)
				p.On("BridgeCalls", ctx, "CALL123", "CALL456").Return(nil)
				ep.On("PublishCallEvent", ctx, mock.AnythingOfType("*telephony.CallEvent")).Return(nil)
			},
			callID1:       callID1,
			callID2:       callID2,
			expectedError: false,
		},
		{
			name: "first call not in progress",
			setupMocks: func(cr *MockCallRepository, p *MockProvider, ep *MockEventPublisher) {
				call1 := &call.Call{
					ID:      callID1,
					CallSID: "CALL123",
					Status:  call.StatusQueued,
				}
				call2 := &call.Call{
					ID:      callID2,
					CallSID: "CALL456",
					Status:  call.StatusInProgress,
				}
				cr.On("GetByID", ctx, callID1).Return(call1, nil)
				cr.On("GetByID", ctx, callID2).Return(call2, nil)
			},
			callID1:       callID1,
			callID2:       callID2,
			expectedError: true,
			errorContains: "both calls must be in progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(MockCallRepository)
			provider := new(MockProvider)
			eventPub := new(MockEventPublisher)

			// Setup mocks
			tt.setupMocks(callRepo, provider, eventPub)

			// Create service
			svc := NewService(callRepo, provider, eventPub, nil)

			// Execute
			err := svc.BridgeCalls(ctx, tt.callID1, tt.callID2)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}

			// Assert expectations
			callRepo.AssertExpectations(t)
			provider.AssertExpectations(t)
			eventPub.AssertExpectations(t)
		})
	}
}

// Helper functions

func floatPtr(f float64) *float64 {
	return &f
}

func moneyPtr(m values.Money) *values.Money {
	return &m
}

func intPtr(i int) *int {
	return &i
}

// Benchmarks

func BenchmarkService_InitiateCall(b *testing.B) {
	ctx := context.Background()

	// Create mocks
	callRepo := new(MockCallRepository)
	provider := new(MockProvider)
	eventPub := new(MockEventPublisher)
	metrics := new(MockMetricsCollector)

	// Setup mocks to always succeed
	callRepo.On("Create", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	provider.On("GetProviderName").Return("twilio")
	provider.On("InitiateCall", ctx, mock.Anything, mock.Anything, mock.Anything).Return("CALL123", nil)
	callRepo.On("Update", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	metrics.On("RecordProviderLatency", ctx, mock.Anything, mock.Anything, mock.Anything)
	metrics.On("RecordCallInitiated", ctx, mock.Anything)
	eventPub.On("PublishCallEvent", ctx, mock.Anything).Return(nil)

	// Create service
	svc := NewService(callRepo, provider, eventPub, metrics)

	// Create request
	req := &InitiateCallRequest{
		FromNumber:  "+11234567890",
		ToNumber:    "+19876543210",
		BuyerID:     uuid.New(),
		CallbackURL: "https://callback.url",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = svc.InitiateCall(ctx, req)
	}
}
