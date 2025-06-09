package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
)

// CallRoutingBidRepository adapts bidding.BidRepository to callrouting.BidRepository
type CallRoutingBidRepository struct {
	bidding.BidRepository
}

// NewCallRoutingBidRepository creates a new adapter
func NewCallRoutingBidRepository(repo bidding.BidRepository) callrouting.BidRepository {
	return &CallRoutingBidRepository{repo}
}

// GetBidByID implements callrouting.BidRepository
func (r *CallRoutingBidRepository) GetBidByID(ctx context.Context, bidID uuid.UUID) (*bid.Bid, error) {
	return r.BidRepository.GetByID(ctx, bidID)
}

// Update implements callrouting.BidRepository
func (r *CallRoutingBidRepository) Update(ctx context.Context, bid *bid.Bid) error {
	return r.BidRepository.Update(ctx, bid)
}

// CallRoutingAccountRepository adapts bidding.AccountRepository to callrouting.AccountRepository
type CallRoutingAccountRepository struct {
	bidding.AccountRepository
}

// NewCallRoutingAccountRepository creates a new adapter
func NewCallRoutingAccountRepository(repo bidding.AccountRepository) callrouting.AccountRepository {
	return &CallRoutingAccountRepository{repo}
}

// GetByID implements callrouting.AccountRepository
func (r *CallRoutingAccountRepository) GetByID(ctx context.Context, accountID uuid.UUID) (*account.Account, error) {
	return r.AccountRepository.GetByID(ctx, accountID)
}

// UpdateQualityScore implements callrouting.AccountRepository
func (r *CallRoutingAccountRepository) UpdateQualityScore(ctx context.Context, accountID uuid.UUID, score float64) error {
	// For now, we'll use the underlying repository if it supports this method
	// Otherwise, this is a no-op
	if updater, ok := r.AccountRepository.(interface {
		UpdateQualityScore(context.Context, uuid.UUID, float64) error
	}); ok {
		return updater.UpdateQualityScore(ctx, accountID, score)
	}
	return nil
}