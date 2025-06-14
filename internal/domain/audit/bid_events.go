package audit

import (
	"time"

	"github.com/google/uuid"
)

// Bid Domain Events
// These events are published by the bidding domain when audit-worthy actions occur

// BidPlacedEvent is published when a buyer places a bid on a call
type BidPlacedEvent struct {
	*BaseDomainEvent
	BidID        uuid.UUID `json:"bid_id"`
	CallID       uuid.UUID `json:"call_id"`
	BuyerID      uuid.UUID `json:"buyer_id"`
	SellerID     uuid.UUID `json:"seller_id"`
	AuctionID    uuid.UUID `json:"auction_id"`
	Amount       string    `json:"amount"`
	Currency     string    `json:"currency"`
	BidType      string    `json:"bid_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Priority     int       `json:"priority"`
	AutoBid      bool      `json:"auto_bid"`
	MaxAmount    string    `json:"max_amount,omitempty"`
}

// NewBidPlacedEvent creates a new bid placed event
func NewBidPlacedEvent(actorID string, bidID, callID, buyerID, sellerID uuid.UUID, amount string) *BidPlacedEvent {
	base := NewBaseDomainEvent(EventBidPlaced, actorID, bidID.String(), "bid_placed")
	base.TargetType = "bid"
	base.ActorType = "user"

	event := &BidPlacedEvent{
		BaseDomainEvent: base,
		BidID:           bidID,
		CallID:          callID,
		BuyerID:         buyerID,
		SellerID:        sellerID,
		Amount:          amount,
		Currency:        "USD", // Default currency
	}

	// Mark as containing financial data
	event.MarkFinancialData()
	
	// Add relevant data classes
	event.AddDataClass("bid_data")
	event.AddDataClass("financial_data")
	event.AddDataClass("marketplace_data")

	// Set metadata for bid placement
	event.SetMetadata("action_type", "bid_placement")
	event.SetMetadata("call_id", callID.String())
	event.SetMetadata("buyer_id", buyerID.String())
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("bid_amount", amount)

	return event
}

// BidWonEvent is published when a bid wins an auction
type BidWonEvent struct {
	*BaseDomainEvent
	BidID            uuid.UUID `json:"bid_id"`
	CallID           uuid.UUID `json:"call_id"`
	BuyerID          uuid.UUID `json:"buyer_id"`
	SellerID         uuid.UUID `json:"seller_id"`
	AuctionID        uuid.UUID `json:"auction_id"`
	WinningAmount    string    `json:"winning_amount"`
	Currency         string    `json:"currency"`
	CompetingBids    int       `json:"competing_bids"`
	AuctionDuration  int64     `json:"auction_duration_ms"`
	WinMargin        string    `json:"win_margin,omitempty"`
	FinalRank        int       `json:"final_rank"`
}

// NewBidWonEvent creates a new bid won event
func NewBidWonEvent(actorID string, bidID, callID, buyerID, sellerID uuid.UUID, winningAmount string) *BidWonEvent {
	base := NewBaseDomainEvent(EventBidWon, actorID, bidID.String(), "bid_won")
	base.TargetType = "bid"
	base.ActorType = "system"

	event := &BidWonEvent{
		BaseDomainEvent: base,
		BidID:           bidID,
		CallID:          callID,
		BuyerID:         buyerID,
		SellerID:        sellerID,
		WinningAmount:   winningAmount,
		Currency:        "USD",
		FinalRank:       1,
	}

	// Mark as containing financial data and requiring signature
	event.MarkFinancialData()
	event.MarkRequiresSignature()
	
	// Add relevant data classes
	event.AddDataClass("bid_data")
	event.AddDataClass("financial_data")
	event.AddDataClass("marketplace_data")
	event.AddDataClass("auction_data")

	// Set metadata for bid win
	event.SetMetadata("action_type", "bid_win")
	event.SetMetadata("call_id", callID.String())
	event.SetMetadata("buyer_id", buyerID.String())
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("winning_amount", winningAmount)

	return event
}

// BidLostEvent is published when a bid loses an auction
type BidLostEvent struct {
	*BaseDomainEvent
	BidID           uuid.UUID `json:"bid_id"`
	CallID          uuid.UUID `json:"call_id"`
	BuyerID         uuid.UUID `json:"buyer_id"`
	SellerID        uuid.UUID `json:"seller_id"`
	AuctionID       uuid.UUID `json:"auction_id"`
	BidAmount       string    `json:"bid_amount"`
	Currency        string    `json:"currency"`
	WinningAmount   string    `json:"winning_amount"`
	WinningBidID    uuid.UUID `json:"winning_bid_id"`
	FinalRank       int       `json:"final_rank"`
	LossReason      string    `json:"loss_reason"`
	CompetingBids   int       `json:"competing_bids"`
}

// NewBidLostEvent creates a new bid lost event
func NewBidLostEvent(actorID string, bidID, callID, buyerID, sellerID uuid.UUID, bidAmount, winningAmount string) *BidLostEvent {
	base := NewBaseDomainEvent(EventBidLost, actorID, bidID.String(), "bid_lost")
	base.TargetType = "bid"
	base.ActorType = "system"

	event := &BidLostEvent{
		BaseDomainEvent: base,
		BidID:           bidID,
		CallID:          callID,
		BuyerID:         buyerID,
		SellerID:        sellerID,
		BidAmount:       bidAmount,
		WinningAmount:   winningAmount,
		Currency:        "USD",
		LossReason:      "outbid",
	}

	// Mark as containing financial data
	event.MarkFinancialData()
	
	// Add relevant data classes
	event.AddDataClass("bid_data")
	event.AddDataClass("financial_data")
	event.AddDataClass("marketplace_data")
	event.AddDataClass("auction_data")

	// Set metadata for bid loss
	event.SetMetadata("action_type", "bid_loss")
	event.SetMetadata("call_id", callID.String())
	event.SetMetadata("buyer_id", buyerID.String())
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("bid_amount", bidAmount)
	event.SetMetadata("winning_amount", winningAmount)

	return event
}

// AuctionCompletedEvent is published when an auction completes
type AuctionCompletedEvent struct {
	*BaseDomainEvent
	AuctionID        uuid.UUID `json:"auction_id"`
	CallID           uuid.UUID `json:"call_id"`
	SellerID         uuid.UUID `json:"seller_id"`
	WinningBidID     *uuid.UUID `json:"winning_bid_id,omitempty"`
	WinningBuyerID   *uuid.UUID `json:"winning_buyer_id,omitempty"`
	WinningAmount    string     `json:"winning_amount,omitempty"`
	Currency         string     `json:"currency"`
	TotalBids        int        `json:"total_bids"`
	QualifiedBids    int        `json:"qualified_bids"`
	StartTime        time.Time  `json:"start_time"`
	EndTime          time.Time  `json:"end_time"`
	Duration         int64      `json:"duration_ms"`
	AuctionType      string     `json:"auction_type"`
	ReservePrice     string     `json:"reserve_price,omitempty"`
	ReserveMet       bool       `json:"reserve_met"`
}

// NewAuctionCompletedEvent creates a new auction completed event
func NewAuctionCompletedEvent(actorID string, auctionID, callID, sellerID uuid.UUID, totalBids int) *AuctionCompletedEvent {
	base := NewBaseDomainEvent(EventBidWon, actorID, auctionID.String(), "auction_completed")
	base.TargetType = "auction"
	base.ActorType = "system"

	event := &AuctionCompletedEvent{
		BaseDomainEvent: base,
		AuctionID:       auctionID,
		CallID:          callID,
		SellerID:        sellerID,
		TotalBids:       totalBids,
		Currency:        "USD",
		EndTime:         time.Now().UTC(),
		AuctionType:     "realtime",
	}

	// Mark as containing financial and marketplace data
	event.MarkFinancialData()
	
	// Add relevant data classes
	event.AddDataClass("auction_data")
	event.AddDataClass("marketplace_data")
	event.AddDataClass("financial_data")

	// Set metadata for auction completion
	event.SetMetadata("action_type", "auction_completion")
	event.SetMetadata("call_id", callID.String())
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("total_bids", totalBids)

	return event
}

// BidCancelledEvent is published when a buyer cancels their bid
type BidCancelledEvent struct {
	*BaseDomainEvent
	BidID           uuid.UUID `json:"bid_id"`
	CallID          uuid.UUID `json:"call_id"`
	BuyerID         uuid.UUID `json:"buyer_id"`
	SellerID        uuid.UUID `json:"seller_id"`
	AuctionID       uuid.UUID `json:"auction_id"`
	BidAmount       string    `json:"bid_amount"`
	Currency        string    `json:"currency"`
	CancelReason    string    `json:"cancel_reason"`
	CancelledAt     time.Time `json:"cancelled_at"`
	RefundAmount    string    `json:"refund_amount,omitempty"`
	WasLeading      bool      `json:"was_leading"`
}

// NewBidCancelledEvent creates a new bid cancelled event
func NewBidCancelledEvent(actorID string, bidID, callID, buyerID, sellerID uuid.UUID, reason string) *BidCancelledEvent {
	base := NewBaseDomainEvent(EventBidLost, actorID, bidID.String(), "bid_cancelled")
	base.TargetType = "bid"
	base.ActorType = "user"

	event := &BidCancelledEvent{
		BaseDomainEvent: base,
		BidID:           bidID,
		CallID:          callID,
		BuyerID:         buyerID,
		SellerID:        sellerID,
		CancelReason:    reason,
		CancelledAt:     time.Now().UTC(),
		Currency:        "USD",
	}

	// Mark as containing financial data
	event.MarkFinancialData()
	
	// Add relevant data classes
	event.AddDataClass("bid_data")
	event.AddDataClass("financial_data")
	event.AddDataClass("marketplace_data")

	// Set metadata for bid cancellation
	event.SetMetadata("action_type", "bid_cancellation")
	event.SetMetadata("call_id", callID.String())
	event.SetMetadata("buyer_id", buyerID.String())
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("cancel_reason", reason)

	return event
}

// BidModifiedEvent is published when a bid is modified (amount or criteria changed)
type BidModifiedEvent struct {
	*BaseDomainEvent
	BidID              uuid.UUID              `json:"bid_id"`
	CallID             uuid.UUID              `json:"call_id"`
	BuyerID            uuid.UUID              `json:"buyer_id"`
	SellerID           uuid.UUID              `json:"seller_id"`
	AuctionID          uuid.UUID              `json:"auction_id"`
	PreviousAmount     string                 `json:"previous_amount"`
	NewAmount          string                 `json:"new_amount"`
	Currency           string                 `json:"currency"`
	ModificationReason string                 `json:"modification_reason"`
	ModifiedFields     []string               `json:"modified_fields"`
	PreviousValues     map[string]interface{} `json:"previous_values"`
	NewValues          map[string]interface{} `json:"new_values"`
}

// NewBidModifiedEvent creates a new bid modified event
func NewBidModifiedEvent(actorID string, bidID, callID, buyerID, sellerID uuid.UUID, oldAmount, newAmount string) *BidModifiedEvent {
	base := NewBaseDomainEvent(EventBidPlaced, actorID, bidID.String(), "bid_modified")
	base.TargetType = "bid"
	base.ActorType = "user"

	event := &BidModifiedEvent{
		BaseDomainEvent:    base,
		BidID:              bidID,
		CallID:             callID,
		BuyerID:            buyerID,
		SellerID:           sellerID,
		PreviousAmount:     oldAmount,
		NewAmount:          newAmount,
		Currency:           "USD",
		ModificationReason: "amount_adjustment",
		ModifiedFields:     []string{"amount"},
		PreviousValues:     make(map[string]interface{}),
		NewValues:          make(map[string]interface{}),
	}

	// Set previous and new values
	event.PreviousValues["amount"] = oldAmount
	event.NewValues["amount"] = newAmount

	// Mark as containing financial data
	event.MarkFinancialData()
	
	// Add relevant data classes
	event.AddDataClass("bid_data")
	event.AddDataClass("financial_data")
	event.AddDataClass("marketplace_data")

	// Set metadata for bid modification
	event.SetMetadata("action_type", "bid_modification")
	event.SetMetadata("call_id", callID.String())
	event.SetMetadata("buyer_id", buyerID.String())
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("previous_amount", oldAmount)
	event.SetMetadata("new_amount", newAmount)

	return event
}

// BidExpiredEvent is published when a bid expires without being accepted
type BidExpiredEvent struct {
	*BaseDomainEvent
	BidID         uuid.UUID `json:"bid_id"`
	CallID        uuid.UUID `json:"call_id"`
	BuyerID       uuid.UUID `json:"buyer_id"`
	SellerID      uuid.UUID `json:"seller_id"`
	AuctionID     uuid.UUID `json:"auction_id"`
	BidAmount     string    `json:"bid_amount"`
	Currency      string    `json:"currency"`
	PlacedAt      time.Time `json:"placed_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	ExpiredAt     time.Time `json:"expired_at"`
	WasLeading    bool      `json:"was_leading"`
	RefundAmount  string    `json:"refund_amount,omitempty"`
}

// NewBidExpiredEvent creates a new bid expired event
func NewBidExpiredEvent(actorID string, bidID, callID, buyerID, sellerID uuid.UUID, bidAmount string) *BidExpiredEvent {
	base := NewBaseDomainEvent(EventBidLost, actorID, bidID.String(), "bid_expired")
	base.TargetType = "bid"
	base.ActorType = "system"

	event := &BidExpiredEvent{
		BaseDomainEvent: base,
		BidID:           bidID,
		CallID:          callID,
		BuyerID:         buyerID,
		SellerID:        sellerID,
		BidAmount:       bidAmount,
		Currency:        "USD",
		ExpiredAt:       time.Now().UTC(),
	}

	// Mark as containing financial data
	event.MarkFinancialData()
	
	// Add relevant data classes
	event.AddDataClass("bid_data")
	event.AddDataClass("financial_data")
	event.AddDataClass("marketplace_data")

	// Set metadata for bid expiration
	event.SetMetadata("action_type", "bid_expiration")
	event.SetMetadata("call_id", callID.String())
	event.SetMetadata("buyer_id", buyerID.String())
	event.SetMetadata("seller_id", sellerID.String())
	event.SetMetadata("bid_amount", bidAmount)

	return event
}