package bid

import (
	"time"

	"github.com/google/uuid"
)

// BidProfile represents a seller's bidding profile
type BidProfile struct {
	ID        uuid.UUID    `json:"id"`
	SellerID  uuid.UUID    `json:"seller_id"`
	Criteria  BidCriteria  `json:"criteria"`
	Active    bool         `json:"active"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

