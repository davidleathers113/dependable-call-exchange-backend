package telephony

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// service implements the Service interface
type service struct {
	callRepo       CallRepository
	provider       Provider
	eventPublisher EventPublisher
	metrics        MetricsCollector
	mu             sync.RWMutex
	activeCalls    map[uuid.UUID]*CallResponse
}

// NewService creates a new telephony service
func NewService(
	callRepo CallRepository,
	provider Provider,
	eventPublisher EventPublisher,
	metrics MetricsCollector,
) Service {
	return &service{
		callRepo:       callRepo,
		provider:       provider,
		eventPublisher: eventPublisher,
		metrics:        metrics,
		activeCalls:    make(map[uuid.UUID]*CallResponse),
	}
}

// InitiateCall starts a new outbound call
func (s *service) InitiateCall(ctx context.Context, req *InitiateCallRequest) (*CallResponse, error) {
	if req == nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request cannot be nil")
	}

	// Validate request
	if err := validateCallRequest(req); err != nil {
		return nil, err
	}

	// Create call record
	c, err := call.NewCall(req.FromNumber, req.ToNumber, req.BuyerID, call.DirectionOutbound)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_CALL", "failed to create call").
			WithCause(err)
	}
	if req.SellerID != nil {
		c.SellerID = req.SellerID
	}

	// Save call to repository
	if err := s.callRepo.Create(ctx, c); err != nil {
		return nil, errors.NewInternalError("failed to create call record").
			WithCause(err)
	}

	// Initiate call with provider
	start := time.Now()
	callSID, err := s.provider.InitiateCall(ctx, req.FromNumber, req.ToNumber, req.CallbackURL)
	if err != nil {
		// Update call status to failed
		c.UpdateStatus(call.StatusFailed)
		_ = s.callRepo.Update(ctx, c)
		
		if s.metrics != nil {
			s.metrics.RecordCallFailed(ctx, err.Error())
		}
		
		return nil, errors.NewExternalError("telephony provider", "failed to initiate call").
			WithCause(err)
	}

	// Record provider latency
	if s.metrics != nil {
		s.metrics.RecordProviderLatency(ctx, s.provider.GetProviderName(), "initiate_call", time.Since(start))
		s.metrics.RecordCallInitiated(ctx, s.provider.GetProviderName())
	}

	// Update call with provider SID
	c.CallSID = callSID
	c.UpdateStatus(call.StatusQueued)
	if err := s.callRepo.Update(ctx, c); err != nil {
		return nil, errors.NewInternalError("failed to update call with SID").
			WithCause(err)
	}

	// Create response
	response := &CallResponse{
		CallID:    c.ID,
		CallSID:   callSID,
		Status:    c.Status,
		StartTime: c.StartTime,
		Provider:  s.provider.GetProviderName(),
	}

	// Track active call
	s.mu.Lock()
	s.activeCalls[c.ID] = response
	s.mu.Unlock()

	// Publish event
	if s.eventPublisher != nil {
		event := &CallEvent{
			EventID:   uuid.New(),
			CallID:    c.ID,
			EventType: "call.initiated",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"from":     req.FromNumber,
				"to":       req.ToNumber,
				"provider": s.provider.GetProviderName(),
			},
		}
		_ = s.eventPublisher.PublishCallEvent(ctx, event)
	}

	return response, nil
}

// TerminateCall ends an active call
func (s *service) TerminateCall(ctx context.Context, callID uuid.UUID) error {
	// Get call from repository
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return errors.NewNotFoundError("call").
			WithDetails(map[string]interface{}{"call_id": callID}).
			WithCause(err)
	}

	// Check if call is in a terminable state
	if !isTerminableStatus(c.Status) {
		return errors.NewBusinessError("INVALID_STATE", 
			fmt.Sprintf("call cannot be terminated in status: %s", c.Status))
	}

	// Terminate with provider
	if err := s.provider.TerminateCall(ctx, c.CallSID); err != nil {
		return errors.NewExternalError("telephony provider", "failed to terminate call").
			WithCause(err)
	}

	// Update call status
	c.UpdateStatus(call.StatusCanceled)
	c.EndTime = &[]time.Time{time.Now()}[0]
	if c.StartTime.Before(time.Now()) {
		duration := int(time.Since(c.StartTime).Seconds())
		c.Duration = &duration
	}

	if err := s.callRepo.Update(ctx, c); err != nil {
		return errors.NewInternalError("failed to update call status").
			WithCause(err)
	}

	// Remove from active calls
	s.mu.Lock()
	delete(s.activeCalls, callID)
	s.mu.Unlock()

	// Publish event
	if s.eventPublisher != nil {
		event := &CallEvent{
			EventID:   uuid.New(),
			CallID:    callID,
			EventType: "call.terminated",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"duration": c.Duration,
			},
		}
		_ = s.eventPublisher.PublishCallEvent(ctx, event)
	}

	return nil
}

// GetCallStatus returns the current status of a call
func (s *service) GetCallStatus(ctx context.Context, callID uuid.UUID) (*CallStatus, error) {
	// Get call from repository
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, errors.NewNotFoundError("call").
			WithDetails(map[string]interface{}{"call_id": callID}).
			WithCause(err)
	}

	// Get latest status from provider if call is active
	if isActiveStatus(c.Status) && c.CallSID != "" {
		providerStatus, err := s.provider.GetCallStatus(ctx, c.CallSID)
		if err == nil {
			// Update local status if different
			newStatus := mapProviderStatus(providerStatus.Status)
			if newStatus != c.Status {
				c.UpdateStatus(newStatus)
				if providerStatus.Duration > 0 {
					c.Duration = &providerStatus.Duration
				}
				if providerStatus.Price != nil {
					c.Cost = providerStatus.Price
				}
				_ = s.callRepo.Update(ctx, c)
			}
		}
	}

	return &CallStatus{
		CallID:    c.ID,
		Status:    c.Status,
		Duration:  c.Duration,
		StartTime: c.StartTime,
		EndTime:   c.EndTime,
		Cost:      c.Cost,
	}, nil
}

// TransferCall transfers a call to another number
func (s *service) TransferCall(ctx context.Context, callID uuid.UUID, toNumber string) error {
	// Get call from repository
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return errors.NewNotFoundError("call").
			WithDetails(map[string]interface{}{"call_id": callID}).
			WithCause(err)
	}

	// Check if call is in progress
	if c.Status != call.StatusInProgress {
		return errors.NewBusinessError("INVALID_STATE", "call must be in progress to transfer")
	}

	// Transfer with provider
	if err := s.provider.TransferCall(ctx, c.CallSID, toNumber); err != nil {
		return errors.NewExternalError("telephony provider", "failed to transfer call").
			WithCause(err)
	}

	// Publish event
	if s.eventPublisher != nil {
		event := &CallEvent{
			EventID:   uuid.New(),
			CallID:    callID,
			EventType: "call.transferred",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"to": toNumber,
			},
		}
		_ = s.eventPublisher.PublishCallEvent(ctx, event)
	}

	return nil
}

// RecordCall starts or stops call recording
func (s *service) RecordCall(ctx context.Context, callID uuid.UUID, record bool) error {
	// Get call from repository
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return errors.NewNotFoundError("call").
			WithDetails(map[string]interface{}{"call_id": callID}).
			WithCause(err)
	}

	// Check if call is in progress
	if c.Status != call.StatusInProgress {
		return errors.NewBusinessError("INVALID_STATE", "call must be in progress to record")
	}

	// For now, we'll just publish an event
	// Real implementation would interact with provider recording API
	if s.eventPublisher != nil {
		eventType := "call.recording.started"
		if !record {
			eventType = "call.recording.stopped"
		}
		
		event := &CallEvent{
			EventID:   uuid.New(),
			CallID:    callID,
			EventType: eventType,
			Timestamp: time.Now(),
			Data:      map[string]interface{}{},
		}
		_ = s.eventPublisher.PublishCallEvent(ctx, event)
	}

	return nil
}

// SendDTMF sends DTMF tones on a call
func (s *service) SendDTMF(ctx context.Context, callID uuid.UUID, digits string) error {
	// Get call from repository
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return errors.NewNotFoundError("call").
			WithDetails(map[string]interface{}{"call_id": callID}).
			WithCause(err)
	}

	// Check if call is in progress
	if c.Status != call.StatusInProgress {
		return errors.NewBusinessError("INVALID_STATE", "call must be in progress to send DTMF")
	}

	// Send DTMF with provider
	if err := s.provider.SendDTMF(ctx, c.CallSID, digits); err != nil {
		return errors.NewExternalError("telephony provider", "failed to send DTMF").
			WithCause(err)
	}

	return nil
}

// BridgeCalls bridges two calls together
func (s *service) BridgeCalls(ctx context.Context, callID1, callID2 uuid.UUID) error {
	// Get both calls
	call1, err := s.callRepo.GetByID(ctx, callID1)
	if err != nil {
		return errors.NewNotFoundError("call").
			WithDetails(map[string]interface{}{"call_id": callID1}).
			WithCause(err)
	}

	call2, err := s.callRepo.GetByID(ctx, callID2)
	if err != nil {
		return errors.NewNotFoundError("call").
			WithDetails(map[string]interface{}{"call_id": callID2}).
			WithCause(err)
	}

	// Check if both calls are in progress
	if call1.Status != call.StatusInProgress || call2.Status != call.StatusInProgress {
		return errors.NewBusinessError("INVALID_STATE", "both calls must be in progress to bridge")
	}

	// Bridge with provider
	if err := s.provider.BridgeCalls(ctx, call1.CallSID, call2.CallSID); err != nil {
		return errors.NewExternalError("telephony provider", "failed to bridge calls").
			WithCause(err)
	}

	// Publish event
	if s.eventPublisher != nil {
		event := &CallEvent{
			EventID:   uuid.New(),
			CallID:    callID1,
			EventType: "calls.bridged",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"call1": callID1,
				"call2": callID2,
			},
		}
		_ = s.eventPublisher.PublishCallEvent(ctx, event)
	}

	return nil
}

// HandleWebhook processes telephony provider webhooks
func (s *service) HandleWebhook(ctx context.Context, provider string, data interface{}) error {
	// In a real implementation, this would:
	// 1. Parse provider-specific webhook data
	// 2. Update call status based on webhook events
	// 3. Handle recording completions, call completions, etc.
	// 4. Publish appropriate events
	
	// For now, we'll just return success
	return nil
}

// Helper functions

func validateCallRequest(req *InitiateCallRequest) error {
	if req.FromNumber == "" {
		return errors.NewValidationError("INVALID_FROM_NUMBER", "from number is required")
	}
	if req.ToNumber == "" {
		return errors.NewValidationError("INVALID_TO_NUMBER", "to number is required")
	}
	if req.BuyerID == uuid.Nil {
		return errors.NewValidationError("INVALID_BUYER_ID", "buyer ID is required")
	}
	if req.MaxDuration > 0 && req.MaxDuration < 60 {
		return errors.NewValidationError("INVALID_MAX_DURATION", "maximum duration must be at least 60 seconds")
	}
	return nil
}

func isTerminableStatus(status call.Status) bool {
	switch status {
	case call.StatusPending, call.StatusQueued, call.StatusRinging, call.StatusInProgress:
		return true
	default:
		return false
	}
}

func isActiveStatus(status call.Status) bool {
	switch status {
	case call.StatusPending, call.StatusQueued, call.StatusRinging, call.StatusInProgress:
		return true
	default:
		return false
	}
}

func mapProviderStatus(providerStatus string) call.Status {
	// Map provider-specific status to our domain status
	// This would be customized per provider
	switch providerStatus {
	case "queued":
		return call.StatusQueued
	case "ringing":
		return call.StatusRinging
	case "in-progress":
		return call.StatusInProgress
	case "completed":
		return call.StatusCompleted
	case "failed":
		return call.StatusFailed
	case "busy":
		return call.StatusBusy
	case "no-answer":
		return call.StatusNoAnswer
	case "canceled":
		return call.StatusCanceled
	default:
		return call.StatusPending
	}
}