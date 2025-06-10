package bidding

import (
	"context"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// bidValidationService handles business rule validation
type bidValidationService struct {
	minBidAmount float64
	maxBidAmount float64
	fraudChecker FraudChecker
	accountRepo  AccountRepository
}

// NewBidValidationService creates a new bid validation service
func NewBidValidationService(
	minBidAmount, maxBidAmount float64,
	fraudChecker FraudChecker,
	accountRepo AccountRepository,
) BidValidationService {
	return &bidValidationService{
		minBidAmount: minBidAmount,
		maxBidAmount: maxBidAmount,
		fraudChecker: fraudChecker,
		accountRepo:  accountRepo,
	}
}

// ValidateBidRequest validates a bid placement request
func (s *bidValidationService) ValidateBidRequest(ctx context.Context, req *PlaceBidRequest) error {
	// Validate required fields
	if req.CallID == uuid.Nil {
		return errors.NewValidationError("INVALID_CALL_ID", "call ID is required")
	}
	
	if req.BuyerID == uuid.Nil {
		return errors.NewValidationError("INVALID_BUYER_ID", "buyer ID is required")
	}
	
	// Validate amount
	if err := s.ValidateBidAmount(req.Amount); err != nil {
		return err
	}
	
	// Validate auto-bidding parameters
	if req.AutoRenew && req.MaxAmount > 0 {
		if req.MaxAmount < req.Amount {
			return errors.NewValidationError("INVALID_MAX_AMOUNT", 
				"max amount must be greater than or equal to initial amount")
		}
		
		if err := s.ValidateBidAmount(req.MaxAmount); err != nil {
			return errors.NewValidationError("INVALID_MAX_AMOUNT", 
				fmt.Sprintf("max amount validation failed: %v", err))
		}
	}
	
	// Validate criteria if present
	if req.Criteria != nil {
		// Basic validation - ensure it's not empty if provided
		if len(req.Criteria) == 0 {
			return errors.NewValidationError("INVALID_CRITERIA", 
				"criteria cannot be empty if provided")
		}
	}
	
	return nil
}

// ValidateBidAmount checks if bid amount is within allowed range
func (s *bidValidationService) ValidateBidAmount(amount float64) error {
	if amount <= 0 {
		return errors.NewValidationError("INVALID_BID_AMOUNT", "bid amount must be positive")
	}
	
	if amount < s.minBidAmount {
		return errors.NewValidationError("BID_AMOUNT_TOO_LOW", 
			fmt.Sprintf("bid amount must be at least %.2f", s.minBidAmount))
	}
	
	if amount > s.maxBidAmount {
		return errors.NewValidationError("BID_AMOUNT_TOO_HIGH", 
			fmt.Sprintf("bid amount must not exceed %.2f", s.maxBidAmount))
	}
	
	return nil
}

// ValidateBuyerEligibility checks if buyer can place bids
func (s *bidValidationService) ValidateBuyerEligibility(ctx context.Context, buyer *account.Account) error {
	// Check account status
	if buyer.Status != account.StatusActive {
		return errors.NewForbiddenError("buyer account is not active")
	}
	
	// Check account type
	if buyer.Type != account.TypeBuyer {
		return errors.NewForbiddenError("only buyer accounts can place bids")
	}
	
	// Check if account has required consents
	if !buyer.TCPAConsent {
		return errors.NewForbiddenError("TCPA consent required")
	}
	
	// Check compliance flags
	if len(buyer.ComplianceFlags) > 0 {
		for _, flag := range buyer.ComplianceFlags {
			if flag == "SUSPENDED" || flag == "BANNED" {
				return errors.NewForbiddenError(
					fmt.Sprintf("account has compliance flag: %s", flag))
			}
		}
	}
	
	// Check fraud score threshold
	if buyer.QualityMetrics.FraudScore > 0.8 {
		return errors.NewForbiddenError("account fraud score exceeds threshold")
	}
	
	return nil
}

// ValidateBidUpdate validates bid modification request
func (s *bidValidationService) ValidateBidUpdate(ctx context.Context, b *bid.Bid, updates *BidUpdate) error {
	// Check if bid is modifiable
	if b.Status != bid.StatusActive && b.Status != bid.StatusPending {
		return errors.NewValidationError("INVALID_BID_STATUS", 
			fmt.Sprintf("bid cannot be modified in status: %s", b.Status))
	}
	
	// Validate new amount if provided
	if updates.Amount != nil {
		if err := s.ValidateBidAmount(*updates.Amount); err != nil {
			return err
		}
	}
	
	// Validate max amount if provided
	if updates.MaxAmount != nil {
		if err := s.ValidateBidAmount(*updates.MaxAmount); err != nil {
			return errors.NewValidationError("INVALID_MAX_AMOUNT", 
				fmt.Sprintf("max amount validation failed: %v", err))
		}
		
		// Ensure max amount is greater than current amount
		currentAmount := b.Amount
		if updates.Amount != nil {
			currentAmount, _ = values.NewMoneyFromFloat(*updates.Amount, "USD")
		}
		
		if maxMoney, _ := values.NewMoneyFromFloat(*updates.MaxAmount, "USD"); maxMoney.Compare(currentAmount) < 0 {
			return errors.NewValidationError("INVALID_MAX_AMOUNT", 
				"max amount must be greater than or equal to bid amount")
		}
	}
	
	// Validate extension duration
	if updates.ExtendBy != nil {
		if *updates.ExtendBy <= 0 {
			return errors.NewValidationError("INVALID_EXTENSION", 
				"extension duration must be positive")
		}
		
		// Check maximum extension (e.g., 24 hours)
		const maxExtension = 24 * 60 * 60 // 24 hours in seconds
		if updates.ExtendBy.Seconds() > maxExtension {
			return errors.NewValidationError("INVALID_EXTENSION", 
				"extension duration exceeds maximum allowed")
		}
	}
	
	return nil
}

// CheckFraud performs fraud detection on bid
func (s *bidValidationService) CheckFraud(ctx context.Context, b *bid.Bid, buyer *account.Account) (*FraudCheckResult, error) {
	if s.fraudChecker == nil {
		// If no fraud checker configured, approve by default
		return &FraudCheckResult{
			Approved:    true,
			RiskScore:   0.0,
			RequiresMFA: false,
		}, nil
	}
	
	result, err := s.fraudChecker.CheckBid(ctx, b, buyer)
	if err != nil {
		return nil, errors.NewInternalError("fraud check failed").WithCause(err)
	}
	
	// Update buyer's fraud score if significantly different
	if result.RiskScore > 0 && result.RiskScore != buyer.QualityMetrics.FraudScore {
		// In production, this would trigger an async update
		// to the buyer's fraud score
	}
	
	return result, nil
}