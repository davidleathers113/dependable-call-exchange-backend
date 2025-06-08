package callrouting

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
)

// RoundRobinRouter implements round-robin routing
type RoundRobinRouter struct {
	lastIndex int
}

// NewRoundRobinRouter creates a new round-robin router
func NewRoundRobinRouter() *RoundRobinRouter {
	return &RoundRobinRouter{}
}

// Route implements the Router interface for round-robin
func (r *RoundRobinRouter) Route(ctx context.Context, c *call.Call, bids []*bid.Bid) (*RoutingDecision, error) {
	if len(bids) == 0 {
		return nil, fmt.Errorf("no bids available for routing")
	}

	// Filter active bids
	activeBids := filterActiveBids(bids)
	if len(activeBids) == 0 {
		return nil, fmt.Errorf("no active bids available")
	}

	// Round-robin selection
	r.lastIndex = (r.lastIndex + 1) % len(activeBids)
	selectedBid := activeBids[r.lastIndex]

	return &RoutingDecision{
		CallID:    c.ID,
		BidID:     selectedBid.ID,
		BuyerID:   selectedBid.BuyerID,
		Algorithm: "round-robin",
		Score:     1.0,
		Reason:    "Round-robin selection",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"index": r.lastIndex,
			"total": len(activeBids),
		},
	}, nil
}

// GetAlgorithm returns the algorithm name
func (r *RoundRobinRouter) GetAlgorithm() string {
	return "round-robin"
}

// SkillBasedRouter implements skill-based routing
type SkillBasedRouter struct {
	skillWeights map[string]float64
}

// NewSkillBasedRouter creates a new skill-based router
func NewSkillBasedRouter(skillWeights map[string]float64) *SkillBasedRouter {
	if skillWeights == nil {
		skillWeights = make(map[string]float64)
	}
	return &SkillBasedRouter{
		skillWeights: skillWeights,
	}
}

// Route implements the Router interface for skill-based routing
func (r *SkillBasedRouter) Route(ctx context.Context, c *call.Call, bids []*bid.Bid) (*RoutingDecision, error) {
	if len(bids) == 0 {
		return nil, fmt.Errorf("no bids available for routing")
	}

	activeBids := filterActiveBids(bids)
	if len(activeBids) == 0 {
		return nil, fmt.Errorf("no active bids available")
	}

	// Score each bid based on skill match
	type scoredBid struct {
		bid   *bid.Bid
		score float64
	}

	scoredBids := make([]scoredBid, 0, len(activeBids))
	
	for _, b := range activeBids {
		score := r.calculateSkillScore(c, b)
		scoredBids = append(scoredBids, scoredBid{bid: b, score: score})
	}

	// Sort by score (highest first)
	sort.Slice(scoredBids, func(i, j int) bool {
		return scoredBids[i].score > scoredBids[j].score
	})

	// Select the best match
	best := scoredBids[0]
	
	return &RoutingDecision{
		CallID:    c.ID,
		BidID:     best.bid.ID,
		BuyerID:   best.bid.BuyerID,
		Algorithm: "skill-based",
		Score:     best.score,
		Reason:    fmt.Sprintf("Best skill match (score: %.2f)", best.score),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"skill_scores": scoredBids,
		},
	}, nil
}

// calculateSkillScore calculates the skill match score
func (r *SkillBasedRouter) calculateSkillScore(c *call.Call, b *bid.Bid) float64 {
	score := 0.0
	matchCount := 0

	// Extract required skills from call metadata
	requiredSkills, ok := c.Metadata["required_skills"].([]string)
	if !ok {
		return 1.0 // Default score if no skills specified
	}

	// Extract buyer skills from bid criteria
	buyerSkills, ok := b.Criteria["skills"].([]string)
	if !ok {
		return 0.5 // Partial score if buyer has no skills listed
	}

	// Calculate match score
	for _, required := range requiredSkills {
		weight := r.skillWeights[required]
		if weight == 0 {
			weight = 1.0 // Default weight
		}
		
		for _, skill := range buyerSkills {
			if skill == required {
				score += weight
				matchCount++
				break
			}
		}
	}

	// Normalize score
	if len(requiredSkills) > 0 {
		score = score / float64(len(requiredSkills))
	}

	return score
}

// GetAlgorithm returns the algorithm name
func (r *SkillBasedRouter) GetAlgorithm() string {
	return "skill-based"
}

// CostBasedRouter implements cost-based routing
type CostBasedRouter struct {
	qualityWeight  float64
	priceWeight    float64
	capacityWeight float64
}

// NewCostBasedRouter creates a new cost-based router
func NewCostBasedRouter(qualityWeight, priceWeight, capacityWeight float64) *CostBasedRouter {
	// Normalize weights
	total := qualityWeight + priceWeight + capacityWeight
	if total == 0 {
		qualityWeight, priceWeight, capacityWeight = 0.33, 0.33, 0.34
	} else {
		qualityWeight /= total
		priceWeight /= total
		capacityWeight /= total
	}

	return &CostBasedRouter{
		qualityWeight:  qualityWeight,
		priceWeight:    priceWeight,
		capacityWeight: capacityWeight,
	}
}

// Route implements the Router interface for cost-based routing
func (r *CostBasedRouter) Route(ctx context.Context, c *call.Call, bids []*bid.Bid) (*RoutingDecision, error) {
	if len(bids) == 0 {
		return nil, fmt.Errorf("no bids available for routing")
	}

	activeBids := filterActiveBids(bids)
	if len(activeBids) == 0 {
		return nil, fmt.Errorf("no active bids available")
	}

	// Calculate composite score for each bid
	type scoredBid struct {
		bid            *bid.Bid
		score          float64
		qualityScore   float64
		priceScore     float64
		capacityScore  float64
	}

	scoredBids := make([]scoredBid, 0, len(activeBids))
	
	// Find min/max values for normalization
	minPrice, maxPrice := findPriceRange(activeBids)
	
	for _, b := range activeBids {
		quality := b.QualityScore / 100.0 // Normalize to 0-1
		price := normalizePriceScore(b.Amount, minPrice, maxPrice)
		capacity := calculateCapacityScore(b)
		
		totalScore := r.qualityWeight*quality + 
			r.priceWeight*price + 
			r.capacityWeight*capacity
		
		scoredBids = append(scoredBids, scoredBid{
			bid:           b,
			score:         totalScore,
			qualityScore:  quality,
			priceScore:    price,
			capacityScore: capacity,
		})
	}

	// Sort by total score (highest first)
	sort.Slice(scoredBids, func(i, j int) bool {
		return scoredBids[i].score > scoredBids[j].score
	})

	best := scoredBids[0]
	
	return &RoutingDecision{
		CallID:    c.ID,
		BidID:     best.bid.ID,
		BuyerID:   best.bid.BuyerID,
		Algorithm: "cost-based",
		Score:     best.score,
		Reason:    fmt.Sprintf("Optimal cost-quality balance (score: %.2f)", best.score),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"quality_score":  best.qualityScore,
			"price_score":    best.priceScore,
			"capacity_score": best.capacityScore,
			"weights": map[string]float64{
				"quality":  r.qualityWeight,
				"price":    r.priceWeight,
				"capacity": r.capacityWeight,
			},
		},
	}, nil
}

// GetAlgorithm returns the algorithm name
func (r *CostBasedRouter) GetAlgorithm() string {
	return "cost-based"
}

// Helper functions

func filterActiveBids(bids []*bid.Bid) []*bid.Bid {
	active := make([]*bid.Bid, 0, len(bids))
	for _, b := range bids {
		if b.Status == bid.StatusActive {
			active = append(active, b)
		}
	}
	return active
}

func findPriceRange(bids []*bid.Bid) (float64, float64) {
	if len(bids) == 0 {
		return 0, 0
	}
	
	min := bids[0].Amount
	max := bids[0].Amount
	
	for _, b := range bids[1:] {
		if b.Amount < min {
			min = b.Amount
		}
		if b.Amount > max {
			max = b.Amount
		}
	}
	
	return min, max
}

func normalizePriceScore(price, min, max float64) float64 {
	if max == min {
		return 1.0
	}
	// Lower price = higher score
	return 1.0 - (price-min)/(max-min)
}

func calculateCapacityScore(b *bid.Bid) float64 {
	// Extract capacity metrics from bid criteria
	capacity, ok := b.Criteria["available_capacity"].(float64)
	if !ok {
		return 0.5 // Default middle score
	}
	
	// Normalize capacity to 0-1 range (assuming max capacity of 1000)
	return math.Min(capacity/1000.0, 1.0)
}