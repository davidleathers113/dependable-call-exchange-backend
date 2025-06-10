package seller_distribution

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
)

// DistributionAlgorithm defines the interface for seller distribution algorithms
type DistributionAlgorithm interface {
	// SelectSellers chooses which sellers to notify based on the algorithm
	SelectSellers(ctx context.Context, availableSellers []*account.Account, incomingCall *call.Call, rules *SellerDistributionRules) ([]*account.Account, float64, map[string]interface{}, error)
	
	// Name returns the algorithm name
	Name() string
	
	// Description returns a human-readable description
	Description() string
}

// BroadcastAlgorithm notifies all available sellers up to the maximum limit
type BroadcastAlgorithm struct {
	accountRepo AccountRepository
}

// NewBroadcastAlgorithm creates a new broadcast algorithm
func NewBroadcastAlgorithm(accountRepo AccountRepository) *BroadcastAlgorithm {
	return &BroadcastAlgorithm{
		accountRepo: accountRepo,
	}
}

func (a *BroadcastAlgorithm) Name() string {
	return "broadcast"
}

func (a *BroadcastAlgorithm) Description() string {
	return "Notifies all available sellers up to the maximum limit, prioritizing by quality score"
}

func (a *BroadcastAlgorithm) SelectSellers(ctx context.Context, availableSellers []*account.Account, incomingCall *call.Call, rules *SellerDistributionRules) ([]*account.Account, float64, map[string]interface{}, error) {
	maxSellers := rules.MaxSellers
	if len(availableSellers) <= maxSellers {
		return availableSellers, 1.0, map[string]interface{}{
			"algorithm":     "broadcast",
			"total_sellers": len(availableSellers),
		}, nil
	}
	
	// Sort by quality score descending
	sortedSellers := make([]*account.Account, len(availableSellers))
	copy(sortedSellers, availableSellers)
	
	sort.Slice(sortedSellers, func(i, j int) bool {
		return sortedSellers[i].QualityMetrics.OverallScore() > sortedSellers[j].QualityMetrics.OverallScore()
	})
	
	selected := sortedSellers[:maxSellers]
	
	return selected, 1.0, map[string]interface{}{
		"algorithm":      "broadcast",
		"total_sellers":  len(availableSellers),
		"selected_count": len(selected),
		"quality_range": map[string]float64{
			"highest": selected[0].QualityMetrics.OverallScore(),
			"lowest":  selected[len(selected)-1].QualityMetrics.OverallScore(),
		},
	}, nil
}

// TargetedAlgorithm selects sellers based on weighted scoring criteria
type TargetedAlgorithm struct {
	accountRepo AccountRepository
}

// NewTargetedAlgorithm creates a new targeted algorithm
func NewTargetedAlgorithm(accountRepo AccountRepository) *TargetedAlgorithm {
	return &TargetedAlgorithm{
		accountRepo: accountRepo,
	}
}

func (a *TargetedAlgorithm) Name() string {
	return "targeted"
}

func (a *TargetedAlgorithm) Description() string {
	return "Selects sellers based on weighted scoring of quality, capacity, and geographic proximity"
}

func (a *TargetedAlgorithm) SelectSellers(ctx context.Context, availableSellers []*account.Account, incomingCall *call.Call, rules *SellerDistributionRules) ([]*account.Account, float64, map[string]interface{}, error) {
	type sellerScore struct {
		seller        *account.Account
		qualityScore  float64
		capacityScore float64
		geoScore      float64
		totalScore    float64
	}
	
	scores := make([]sellerScore, 0, len(availableSellers))
	
	for _, seller := range availableSellers {
		qs := seller.QualityMetrics.OverallScore() * rules.QualityWeight
		cs := a.calculateCapacityScore(ctx, seller) * rules.CapacityWeight
		gs := a.calculateGeographyScore(seller, incomingCall) * rules.GeographyWeight
		total := qs + cs + gs
		
		scores = append(scores, sellerScore{
			seller:        seller,
			qualityScore:  qs,
			capacityScore: cs,
			geoScore:      gs,
			totalScore:    total,
		})
	}
	
	// Sort by total score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].totalScore > scores[j].totalScore
	})
	
	// Select top sellers up to max limit
	maxSellers := rules.MaxSellers
	if len(scores) > maxSellers {
		scores = scores[:maxSellers]
	}
	
	// Extract selected sellers and calculate metrics
	selected := make([]*account.Account, len(scores))
	totalScore := 0.0
	qualitySum := 0.0
	capacitySum := 0.0
	geoSum := 0.0
	
	for i, s := range scores {
		selected[i] = s.seller
		totalScore += s.totalScore
		qualitySum += s.qualityScore
		capacitySum += s.capacityScore
		geoSum += s.geoScore
	}
	
	avgScore := totalScore / float64(len(scores))
	
	metadata := map[string]interface{}{
		"algorithm":     "targeted",
		"avg_score":     avgScore,
		"score_breakdown": map[string]float64{
			"avg_quality":  qualitySum / float64(len(scores)),
			"avg_capacity": capacitySum / float64(len(scores)),
			"avg_geography": geoSum / float64(len(scores)),
		},
		"weights": map[string]float64{
			"quality":   rules.QualityWeight,
			"capacity":  rules.CapacityWeight,
			"geography": rules.GeographyWeight,
		},
		"selected_count": len(selected),
	}
	
	return selected, avgScore, metadata, nil
}

func (a *TargetedAlgorithm) calculateCapacityScore(ctx context.Context, seller *account.Account) float64 {
	capacity, err := a.accountRepo.GetSellerCapacity(ctx, seller.ID)
	if err != nil {
		// Default to middle score if capacity unavailable
		return 0.5
	}
	
	if capacity.MaxConcurrentCalls == 0 {
		return 0.0
	}
	
	utilizationRate := float64(capacity.CurrentCalls) / float64(capacity.MaxConcurrentCalls)
	
	// Invert utilization: lower utilization = higher score
	capacityScore := 1.0 - utilizationRate
	
	// Ensure score is between 0 and 1
	return math.Max(0.0, math.Min(1.0, capacityScore))
}

func (a *TargetedAlgorithm) calculateGeographyScore(seller *account.Account, incomingCall *call.Call) float64 {
	// If no location data available, return neutral score
	if incomingCall.Location == nil {
		return 0.5
	}
	
	// This is a simplified implementation
	// In practice, this would calculate geographic distance/proximity
	// and factor in seller's service areas from their settings
	
	// For now, return a base score that could be enhanced
	// with actual geographic calculations
	return 0.75 // Assume reasonable geographic match
}

// CapacityBasedAlgorithm prioritizes sellers with the most available capacity
type CapacityBasedAlgorithm struct {
	accountRepo AccountRepository
}

// sellerCapacityScore represents a seller with their capacity and scoring information
type sellerCapacityScore struct {
	seller         *account.Account
	capacity       *SellerCapacity
	capacityScore  float64
	qualityScore   float64
	combinedScore  float64
}

// NewCapacityBasedAlgorithm creates a new capacity-based algorithm
func NewCapacityBasedAlgorithm(accountRepo AccountRepository) *CapacityBasedAlgorithm {
	return &CapacityBasedAlgorithm{
		accountRepo: accountRepo,
	}
}

func (a *CapacityBasedAlgorithm) Name() string {
	return "capacity-based"
}

func (a *CapacityBasedAlgorithm) Description() string {
	return "Prioritizes sellers with the most available capacity while considering quality scores"
}

func (a *CapacityBasedAlgorithm) SelectSellers(ctx context.Context, availableSellers []*account.Account, incomingCall *call.Call, rules *SellerDistributionRules) ([]*account.Account, float64, map[string]interface{}, error) {
	
	scores := make([]sellerCapacityScore, 0, len(availableSellers))
	
	for _, seller := range availableSellers {
		capacity, err := a.accountRepo.GetSellerCapacity(ctx, seller.ID)
		if err != nil {
			// Assume minimal capacity if unavailable
			capacity = &SellerCapacity{
				SellerID:           seller.ID,
				MaxConcurrentCalls: 1,
				CurrentCalls:       0,
				AvailableSlots:     1,
				LastUpdated:        time.Now(),
			}
		}
		
		capacityScore := a.calculateCapacityScore(capacity)
		qualityScore := seller.QualityMetrics.OverallScore()
		
		// Combine capacity and quality based on weights
		combinedScore := (capacityScore * rules.CapacityWeight) + 
						(qualityScore * rules.QualityWeight)
		
		scores = append(scores, sellerCapacityScore{
			seller:        seller,
			capacity:      capacity,
			capacityScore: capacityScore,
			qualityScore:  qualityScore,
			combinedScore: combinedScore,
		})
	}
	
	// Sort by combined score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].combinedScore > scores[j].combinedScore
	})
	
	// Select top sellers up to max limit
	maxSellers := rules.MaxSellers
	if len(scores) > maxSellers {
		scores = scores[:maxSellers]
	}
	
	// Extract sellers and calculate metrics
	selected := make([]*account.Account, len(scores))
	totalScore := 0.0
	totalCapacity := 0
	totalQuality := 0.0
	
	for i, s := range scores {
		selected[i] = s.seller
		totalScore += s.combinedScore
		totalCapacity += s.capacity.AvailableSlots
		totalQuality += s.qualityScore
	}
	
	avgScore := totalScore / float64(len(scores))
	avgQuality := totalQuality / float64(len(scores))
	
	metadata := map[string]interface{}{
		"algorithm":       "capacity-based",
		"avg_score":       avgScore,
		"avg_quality":     avgQuality,
		"total_capacity":  totalCapacity,
		"selected_count":  len(selected),
		"capacity_distribution": a.getCapacityDistribution(scores),
	}
	
	return selected, avgScore, metadata, nil
}

func (a *CapacityBasedAlgorithm) calculateCapacityScore(capacity *SellerCapacity) float64 {
	if capacity.MaxConcurrentCalls == 0 {
		return 0.0
	}
	
	utilizationRate := float64(capacity.CurrentCalls) / float64(capacity.MaxConcurrentCalls)
	
	// Invert utilization so lower utilization = higher score
	capacityScore := 1.0 - utilizationRate
	
	// Boost score for sellers with more absolute capacity
	capacityBonus := math.Min(float64(capacity.AvailableSlots)/10.0, 0.2) // Up to 20% bonus
	
	finalScore := capacityScore + capacityBonus
	
	// Ensure score is between 0 and 1
	return math.Max(0.0, math.Min(1.0, finalScore))
}

func (a *CapacityBasedAlgorithm) getCapacityDistribution(scores []sellerCapacityScore) map[string]interface{} {
	if len(scores) == 0 {
		return map[string]interface{}{}
	}
	
	minSlots := scores[0].capacity.AvailableSlots
	maxSlots := scores[0].capacity.AvailableSlots
	totalSlots := 0
	
	for _, s := range scores {
		slots := s.capacity.AvailableSlots
		if slots < minSlots {
			minSlots = slots
		}
		if slots > maxSlots {
			maxSlots = slots
		}
		totalSlots += slots
	}
	
	avgSlots := float64(totalSlots) / float64(len(scores))
	
	return map[string]interface{}{
		"min_available_slots": minSlots,
		"max_available_slots": maxSlots,
		"avg_available_slots": avgSlots,
		"total_available":     totalSlots,
	}
}

// AlgorithmFactory creates distribution algorithms based on name
type AlgorithmFactory struct {
	accountRepo AccountRepository
}

// NewAlgorithmFactory creates a new algorithm factory
func NewAlgorithmFactory(accountRepo AccountRepository) *AlgorithmFactory {
	return &AlgorithmFactory{
		accountRepo: accountRepo,
	}
}

// CreateAlgorithm creates an algorithm instance by name
func (f *AlgorithmFactory) CreateAlgorithm(algorithmName string) DistributionAlgorithm {
	switch algorithmName {
	case "broadcast":
		return NewBroadcastAlgorithm(f.accountRepo)
	case "targeted":
		return NewTargetedAlgorithm(f.accountRepo)
	case "capacity-based":
		return NewCapacityBasedAlgorithm(f.accountRepo)
	default:
		// Default to broadcast
		return NewBroadcastAlgorithm(f.accountRepo)
	}
}

// GetAvailableAlgorithms returns a list of available algorithm names and descriptions
func (f *AlgorithmFactory) GetAvailableAlgorithms() map[string]string {
	return map[string]string{
		"broadcast":      "Notifies all available sellers up to the maximum limit, prioritizing by quality score",
		"targeted":       "Selects sellers based on weighted scoring of quality, capacity, and geographic proximity",
		"capacity-based": "Prioritizes sellers with the most available capacity while considering quality scores",
	}
}