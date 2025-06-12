package seller_distribution

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	domainErrors "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// Service implements the SellerDistributionService interface
type Service struct {
	callRepo        CallRepository
	accountRepo     AccountRepository
	notificationSvc NotificationService
	metrics         SellerMetrics
	rules           *SellerDistributionRules
}

// NewService creates a new seller distribution service
func NewService(
	callRepo CallRepository,
	accountRepo AccountRepository,
	notificationSvc NotificationService,
	metrics SellerMetrics,
	rules *SellerDistributionRules,
) *Service {
	if rules == nil {
		rules = &SellerDistributionRules{
			Algorithm:         "broadcast",
			MaxSellers:        10,
			MinQualityScore:   0.7,
			QualityWeight:     0.4,
			CapacityWeight:    0.3,
			GeographyWeight:   0.3,
			AuctionDuration:   5 * time.Minute,
			RequireSkillMatch: true,
		}
	}

	return &Service{
		callRepo:        callRepo,
		accountRepo:     accountRepo,
		notificationSvc: notificationSvc,
		metrics:         metrics,
		rules:           rules,
	}
}

// DistributeCall distributes an incoming call to available sellers for bidding
func (s *Service) DistributeCall(ctx context.Context, callID uuid.UUID) (*SellerDistributionDecision, error) {
	// Retrieve the call
	incomingCall, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, domainErrors.WrapWithCode(err, "CALL_RETRIEVAL_ERROR", "failed to retrieve call")
	}

	// Validate call is eligible for distribution
	if err := s.validateCallForDistribution(incomingCall); err != nil {
		return nil, err
	}

	// Build criteria based on the call
	criteria := s.buildSellerCriteria(incomingCall)

	// Get available sellers
	availableSellers, err := s.accountRepo.GetAvailableSellers(ctx, criteria)
	if err != nil {
		return nil, domainErrors.WrapWithCode(err, "SELLER_RETRIEVAL_ERROR", "failed to retrieve available sellers")
	}

	if len(availableSellers) == 0 {
		return nil, domainErrors.NewBusinessError(
			"NO_SELLERS_AVAILABLE",
			"no sellers available for call distribution",
		)
	}

	// Select sellers based on algorithm
	selectedSellers, score, metadata, err := s.selectSellers(ctx, availableSellers, incomingCall)
	if err != nil {
		return nil, err
	}

	// Update call status to indicate distribution has started
	incomingCall.UpdateStatus(call.StatusQueued)
	if err := s.callRepo.Update(ctx, incomingCall); err != nil {
		return nil, domainErrors.WrapWithCode(err, "CALL_UPDATE_ERROR", "failed to update call status")
	}

	// Extract seller IDs
	sellerIDs := make([]uuid.UUID, len(selectedSellers))
	for i, seller := range selectedSellers {
		sellerIDs[i] = seller.ID
	}

	// Calculate auction timing
	auctionStartTime := time.Now()
	auctionDuration := s.rules.AuctionDuration

	// Create distribution decision
	decision := &SellerDistributionDecision{
		CallID:           callID,
		Algorithm:        s.rules.Algorithm,
		SelectedSellers:  sellerIDs,
		NotifiedCount:    0, // Will be updated after notification
		Score:            score,
		Metadata:         metadata,
		ProcessedAt:      auctionStartTime,
		AuctionStartTime: auctionStartTime,
		AuctionDuration:  auctionDuration,
	}

	// Notify sellers about the call
	notifiedCount, err := s.notifySellers(ctx, callID, sellerIDs, auctionDuration)
	if err != nil {
		// Don't fail the distribution if notification fails, but log it
		// In production, this would be logged
		decision.Metadata["notification_error"] = err.Error()
	}
	decision.NotifiedCount = notifiedCount

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordDistribution(ctx, decision)
	}

	return decision, nil
}

// GetAvailableSellers returns sellers who can accept calls based on criteria
func (s *Service) GetAvailableSellers(ctx context.Context, criteria *SellerCriteria) ([]*account.Account, error) {
	sellers, err := s.accountRepo.GetAvailableSellers(ctx, criteria)
	if err != nil {
		return nil, domainErrors.WrapWithCode(err, "SELLER_RETRIEVAL_ERROR", "failed to retrieve available sellers")
	}

	return sellers, nil
}

// NotifySellers sends call availability notifications to eligible sellers
func (s *Service) NotifySellers(ctx context.Context, callID uuid.UUID, sellerIDs []uuid.UUID) error {
	_, err := s.notifySellers(ctx, callID, sellerIDs, s.rules.AuctionDuration)
	return err
}

// validateCallForDistribution ensures the call can be distributed to sellers
func (s *Service) validateCallForDistribution(c *call.Call) error {
	// Only pending calls can be distributed
	if c.Status != call.StatusPending {
		return domainErrors.NewValidationError(
			"INVALID_CALL_STATE",
			fmt.Sprintf("call must be in pending status for distribution, current status: %s", c.Status.String()),
		)
	}

	// Call should not already have a seller assigned (for marketplace model)
	if c.SellerID != nil {
		return domainErrors.NewValidationError(
			"CALL_ALREADY_ASSIGNED",
			"call already has seller assigned",
		)
	}

	return nil
}

// buildSellerCriteria creates seller selection criteria based on the call
func (s *Service) buildSellerCriteria(c *call.Call) *SellerCriteria {
	criteria := &SellerCriteria{
		CallType:     []string{c.Direction.String()},
		MinQuality:   s.rules.MinQualityScore,
		AvailableNow: true,
	}

	// Add geographic criteria if call has location info
	if c.Location != nil {
		criteria.Geography = &GeoCriteria{
			Countries: []string{c.Location.Country},
			States:    []string{c.Location.State},
			Cities:    []string{c.Location.City},
			Latitude:  c.Location.Latitude,
			Longitude: c.Location.Longitude,
			Radius:    100.0, // 100km radius
		}
	}

	return criteria
}

// selectSellers chooses sellers based on the configured algorithm
func (s *Service) selectSellers(ctx context.Context, availableSellers []*account.Account, incomingCall *call.Call) ([]*account.Account, float64, map[string]interface{}, error) {
	switch s.rules.Algorithm {
	case "broadcast":
		return s.selectSellersBroadcast(ctx, availableSellers)
	case "targeted":
		return s.selectSellersTargeted(ctx, availableSellers, incomingCall)
	case "capacity-based":
		return s.selectSellersCapacityBased(ctx, availableSellers, incomingCall)
	default:
		return s.selectSellersBroadcast(ctx, availableSellers)
	}
}

// selectSellersBroadcast notifies all available sellers (up to max limit)
func (s *Service) selectSellersBroadcast(ctx context.Context, sellers []*account.Account) ([]*account.Account, float64, map[string]interface{}, error) {
	maxSellers := s.rules.MaxSellers
	if len(sellers) <= maxSellers {
		return sellers, 1.0, map[string]interface{}{
			"algorithm":     "broadcast",
			"total_sellers": len(sellers),
		}, nil
	}

	// Sort by quality score and take top sellers
	sort.Slice(sellers, func(i, j int) bool {
		return sellers[i].QualityMetrics.OverallScore() > sellers[j].QualityMetrics.OverallScore()
	})

	selected := sellers[:maxSellers]
	return selected, 1.0, map[string]interface{}{
		"algorithm":      "broadcast",
		"total_sellers":  len(sellers),
		"selected_count": len(selected),
	}, nil
}

// selectSellersTargeted selects sellers based on weighted scoring
func (s *Service) selectSellersTargeted(ctx context.Context, sellers []*account.Account, incomingCall *call.Call) ([]*account.Account, float64, map[string]interface{}, error) {
	type sellerScore struct {
		seller *account.Account
		score  float64
	}

	scores := make([]sellerScore, 0, len(sellers))

	for _, seller := range sellers {
		score := s.calculateSellerScore(ctx, seller, incomingCall)
		scores = append(scores, sellerScore{
			seller: seller,
			score:  score,
		})
	}

	// Sort by score (highest first)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Select top sellers up to max limit
	maxSellers := s.rules.MaxSellers
	if len(scores) > maxSellers {
		scores = scores[:maxSellers]
	}

	// Extract sellers and calculate average score
	selected := make([]*account.Account, len(scores))
	totalScore := 0.0
	for i, s := range scores {
		selected[i] = s.seller
		totalScore += s.score
	}

	avgScore := totalScore / float64(len(scores))

	return selected, avgScore, map[string]interface{}{
		"algorithm": "targeted",
		"avg_score": avgScore,
		"weights": map[string]float64{
			"quality":   s.rules.QualityWeight,
			"capacity":  s.rules.CapacityWeight,
			"geography": s.rules.GeographyWeight,
		},
	}, nil
}

// selectSellersCapacityBased prioritizes sellers with available capacity
func (s *Service) selectSellersCapacityBased(ctx context.Context, sellers []*account.Account, incomingCall *call.Call) ([]*account.Account, float64, map[string]interface{}, error) {
	type sellerCapacityScore struct {
		seller        *account.Account
		capacity      *SellerCapacity
		capacityScore float64
		combinedScore float64
	}

	scores := make([]sellerCapacityScore, 0, len(sellers))

	for _, seller := range sellers {
		capacity, err := s.accountRepo.GetSellerCapacity(ctx, seller.ID)
		if err != nil {
			// If we can't get capacity, assume minimal capacity
			capacity = &SellerCapacity{
				SellerID:           seller.ID,
				MaxConcurrentCalls: 1,
				CurrentCalls:       0,
				AvailableSlots:     1,
				LastUpdated:        time.Now(),
			}
		}

		capacityScore := s.calculateCapacityScore(capacity)
		qualityScore := seller.QualityMetrics.OverallScore()

		// Combine capacity and quality
		combinedScore := (capacityScore * s.rules.CapacityWeight) +
			(qualityScore * s.rules.QualityWeight)

		scores = append(scores, sellerCapacityScore{
			seller:        seller,
			capacity:      capacity,
			capacityScore: capacityScore,
			combinedScore: combinedScore,
		})
	}

	// Sort by combined score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].combinedScore > scores[j].combinedScore
	})

	// Select top sellers
	maxSellers := s.rules.MaxSellers
	if len(scores) > maxSellers {
		scores = scores[:maxSellers]
	}

	// Extract sellers and calculate metrics
	selected := make([]*account.Account, len(scores))
	totalScore := 0.0
	totalCapacity := 0

	for i, s := range scores {
		selected[i] = s.seller
		totalScore += s.combinedScore
		totalCapacity += s.capacity.AvailableSlots
	}

	avgScore := totalScore / float64(len(scores))

	return selected, avgScore, map[string]interface{}{
		"algorithm":      "capacity-based",
		"avg_score":      avgScore,
		"total_capacity": totalCapacity,
		"selected_count": len(selected),
	}, nil
}

// calculateSellerScore computes a weighted score for seller selection
func (s *Service) calculateSellerScore(ctx context.Context, seller *account.Account, incomingCall *call.Call) float64 {
	qualityScore := seller.QualityMetrics.OverallScore() * s.rules.QualityWeight

	// Get capacity score
	capacity, err := s.accountRepo.GetSellerCapacity(ctx, seller.ID)
	capacityScore := 0.5 // Default middle score
	if err == nil {
		capacityScore = s.calculateCapacityScore(capacity)
	}
	capacityScore *= s.rules.CapacityWeight

	// Calculate geography score if applicable
	geographyScore := 0.5 * s.rules.GeographyWeight // Default middle score
	if incomingCall.Location != nil {
		// This would need implementation based on seller location settings
		// For now, default to middle score
	}

	return qualityScore + capacityScore + geographyScore
}

// calculateCapacityScore normalizes capacity to a 0-1 score
func (s *Service) calculateCapacityScore(capacity *SellerCapacity) float64 {
	if capacity.MaxConcurrentCalls == 0 {
		return 0.0
	}

	utilizationRate := float64(capacity.CurrentCalls) / float64(capacity.MaxConcurrentCalls)

	// Invert utilization so lower utilization = higher score
	capacityScore := 1.0 - utilizationRate

	// Ensure score is between 0 and 1
	return math.Max(0.0, math.Min(1.0, capacityScore))
}

// notifySellers sends notifications to selected sellers
func (s *Service) notifySellers(ctx context.Context, callID uuid.UUID, sellerIDs []uuid.UUID, auctionDuration time.Duration) (int, error) {
	if s.notificationSvc == nil {
		return 0, nil // No notification service configured
	}

	notifiedCount := 0
	var lastError error

	// Send notifications to all selected sellers
	for _, sellerID := range sellerIDs {
		if err := s.notificationSvc.NotifyCallAvailable(ctx, sellerID, callID); err != nil {
			lastError = err
			// Record failed notification in metrics
			if s.metrics != nil {
				s.metrics.RecordSellerNotification(ctx, sellerID, callID, false)
			}
			continue
		}

		notifiedCount++
		// Record successful notification in metrics
		if s.metrics != nil {
			s.metrics.RecordSellerNotification(ctx, sellerID, callID, true)
		}
	}

	// Also send auction started notification
	if err := s.notificationSvc.NotifyAuctionStarted(ctx, sellerIDs, callID, auctionDuration); err != nil {
		lastError = err
	}

	// Return error only if no notifications succeeded
	if notifiedCount == 0 && lastError != nil {
		return 0, domainErrors.NewExternalError("notification", "failed to notify any sellers").WithCause(lastError)
	}

	return notifiedCount, nil
}
