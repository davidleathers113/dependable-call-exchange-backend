package buyer_routing

import (
	"context"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// CallRepository provides access to seller calls for routing
type CallRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error)
	Update(ctx context.Context, c *call.Call) error
	UpdateWithStatusCheck(ctx context.Context, c *call.Call, expectedStatus call.Status) error
	GetActiveCallsForSeller(ctx context.Context, sellerID uuid.UUID) ([]*call.Call, error)
}

// BidRepository provides access to buyer bids on seller calls
type BidRepository interface {
	GetBidByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error)
	GetActiveBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error)
	Update(ctx context.Context, b *bid.Bid) error
}

// AccountRepository provides access to buyer and seller accounts
type AccountRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error)
	GetBuyerQualityMetrics(ctx context.Context, buyerID uuid.UUID) (*values.QualityMetrics, error)
}
