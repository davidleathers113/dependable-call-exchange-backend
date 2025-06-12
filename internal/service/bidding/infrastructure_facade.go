package bidding

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/google/uuid"
)

// InfrastructureServices combines notification and metrics operations to reduce
// coordinator dependencies from 8 to 7. This facade delegates to the actual
// services without adding business logic.
type InfrastructureServices interface {
	// Notification operations
	NotifyBidPlaced(ctx context.Context, bid *bid.Bid) error
	NotifyBidWon(ctx context.Context, bid *bid.Bid) error
	NotifyBidLost(ctx context.Context, bid *bid.Bid) error
	NotifyBidExpired(ctx context.Context, bid *bid.Bid) error
	NotifyAuctionStarted(ctx context.Context, callID uuid.UUID) error
	NotifyAuctionClosed(ctx context.Context, result any) error

	// Metrics operations
	RecordBidPlaced(ctx context.Context, bid *bid.Bid)
	RecordBidAmount(ctx context.Context, amount float64)
	RecordAuctionDuration(ctx context.Context, callID uuid.UUID, duration time.Duration)
	RecordBidValidation(ctx context.Context, bidID uuid.UUID, valid bool, reason string)
	RecordAuctionParticipants(ctx context.Context, callID uuid.UUID, count int)
}

// infrastructureServicesImpl provides a thin facade over notification and metrics
type infrastructureServicesImpl struct {
	notifier NotificationService
	metrics  MetricsCollector
}

// NewInfrastructureServices creates a new infrastructure facade
func NewInfrastructureServices(notifier NotificationService, metrics MetricsCollector) InfrastructureServices {
	return &infrastructureServicesImpl{
		notifier: notifier,
		metrics:  metrics,
	}
}

// Notification delegations
func (i *infrastructureServicesImpl) NotifyBidPlaced(ctx context.Context, bid *bid.Bid) error {
	if i.notifier == nil {
		return nil
	}
	return i.notifier.NotifyBidPlaced(ctx, bid)
}

func (i *infrastructureServicesImpl) NotifyBidWon(ctx context.Context, bid *bid.Bid) error {
	if i.notifier == nil {
		return nil
	}
	return i.notifier.NotifyBidWon(ctx, bid)
}

func (i *infrastructureServicesImpl) NotifyBidLost(ctx context.Context, bid *bid.Bid) error {
	if i.notifier == nil {
		return nil
	}
	return i.notifier.NotifyBidLost(ctx, bid)
}

func (i *infrastructureServicesImpl) NotifyBidExpired(ctx context.Context, bid *bid.Bid) error {
	if i.notifier == nil {
		return nil
	}
	return i.notifier.NotifyBidExpired(ctx, bid)
}

func (i *infrastructureServicesImpl) NotifyAuctionStarted(ctx context.Context, callID uuid.UUID) error {
	if i.notifier == nil {
		return nil
	}
	return i.notifier.NotifyAuctionStarted(ctx, callID)
}

func (i *infrastructureServicesImpl) NotifyAuctionClosed(ctx context.Context, result any) error {
	if i.notifier == nil {
		return nil
	}
	return i.notifier.NotifyAuctionClosed(ctx, result)
}

// Metrics delegations
func (i *infrastructureServicesImpl) RecordBidPlaced(ctx context.Context, bid *bid.Bid) {
	if i.metrics == nil {
		return
	}
	i.metrics.RecordBidPlaced(ctx, bid)
}

func (i *infrastructureServicesImpl) RecordBidAmount(ctx context.Context, amount float64) {
	if i.metrics == nil {
		return
	}
	i.metrics.RecordBidAmount(ctx, amount)
}

func (i *infrastructureServicesImpl) RecordAuctionDuration(ctx context.Context, callID uuid.UUID, duration time.Duration) {
	if i.metrics == nil {
		return
	}
	i.metrics.RecordAuctionDuration(ctx, callID, duration)
}

func (i *infrastructureServicesImpl) RecordBidValidation(ctx context.Context, bidID uuid.UUID, valid bool, reason string) {
	if i.metrics == nil {
		return
	}
	i.metrics.RecordBidValidation(ctx, bidID, valid, reason)
}

func (i *infrastructureServicesImpl) RecordAuctionParticipants(ctx context.Context, callID uuid.UUID, count int) {
	if i.metrics == nil {
		return
	}
	i.metrics.RecordAuctionParticipants(ctx, callID, count)
}
