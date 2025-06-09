package fraud

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// service implements the Service interface
type service struct {
	repo             Repository
	mlEngine         MLEngine
	ruleEngine       RuleEngine
	velocityChecker  VelocityChecker
	blacklistChecker BlacklistChecker
	
	// Configuration
	rules            *FraudRules
	mu               sync.RWMutex
	
	// Caching
	riskCache        map[uuid.UUID]*cachedRisk
	cacheMu          sync.RWMutex
}

// cachedRisk represents cached risk score
type cachedRisk struct {
	score     float64
	timestamp time.Time
}

// NewService creates a new fraud detection service
func NewService(
	repo Repository,
	mlEngine MLEngine,
	ruleEngine RuleEngine,
	velocityChecker VelocityChecker,
	blacklistChecker BlacklistChecker,
	initialRules *FraudRules,
) Service {
	if initialRules == nil {
		initialRules = defaultFraudRules()
	}
	
	return &service{
		repo:             repo,
		mlEngine:         mlEngine,
		ruleEngine:       ruleEngine,
		velocityChecker:  velocityChecker,
		blacklistChecker: blacklistChecker,
		rules:            initialRules,
		riskCache:        make(map[uuid.UUID]*cachedRisk),
	}
}

// CheckCall validates a call for fraud indicators
func (s *service) CheckCall(ctx context.Context, c *call.Call) (*FraudCheckResult, error) {
	result := &FraudCheckResult{
		ID:         uuid.New(),
		EntityID:   c.ID,
		EntityType: "call",
		Timestamp:  time.Now(),
		Approved:   true,
		Flags:      []FraudFlag{},
		Metadata:   make(map[string]interface{}),
	}
	
	// Blacklist check
	if s.blacklistChecker != nil {
		if isBlacklisted, reason, err := s.blacklistChecker.IsBlacklisted(ctx, c.FromNumber, "phone"); err == nil && isBlacklisted {
			result.Approved = false
			result.Reasons = append(result.Reasons, fmt.Sprintf("From number blacklisted: %s", reason))
			result.Flags = append(result.Flags, FraudFlag{
				Type:        "blacklist",
				Severity:    "critical",
				Description: "Blacklisted phone number",
				Score:       1.0,
			})
			result.RiskScore = 1.0
			// Save result before returning
			if s.repo != nil {
				s.repo.SaveCheckResult(ctx, result)
			}
			return result, nil
		}
		
		if isBlacklisted, reason, err := s.blacklistChecker.IsBlacklisted(ctx, c.ToNumber, "phone"); err == nil && isBlacklisted {
			result.Approved = false
			result.Reasons = append(result.Reasons, fmt.Sprintf("To number blacklisted: %s", reason))
			result.Flags = append(result.Flags, FraudFlag{
				Type:        "blacklist",
				Severity:    "critical",
				Description: "Blacklisted phone number",
				Score:       1.0,
			})
			result.RiskScore = 1.0
			// Save result before returning
			if s.repo != nil {
				s.repo.SaveCheckResult(ctx, result)
			}
			return result, nil
		}
	}
	
	// Velocity check
	if s.velocityChecker != nil {
		velocityResult, err := s.velocityChecker.CheckVelocity(ctx, c.BuyerID, "call_placement")
		if err == nil && !velocityResult.Passed {
			result.Flags = append(result.Flags, FraudFlag{
				Type:        "velocity",
				Severity:    "high",
				Description: fmt.Sprintf("High call velocity: %d calls in %v", velocityResult.Count, velocityResult.TimeWindow),
				Score:       0.8,
			})
			result.RiskScore = math.Max(result.RiskScore, 0.8)
		}
		
		// Record the action
		s.velocityChecker.RecordAction(ctx, c.BuyerID, "call_placement")
	}
	
	// Pattern analysis
	features := s.extractCallFeatures(c)
	
	// ML prediction if enabled
	if s.mlEngine != nil && s.rules.MLEnabled {
		prediction, err := s.mlEngine.Predict(ctx, features)
		if err == nil {
			result.RiskScore = math.Max(result.RiskScore, prediction.FraudProbability)
			result.Confidence = prediction.Confidence
			
			if prediction.FraudProbability > 0.7 {
				result.Flags = append(result.Flags, FraudFlag{
					Type:        "ml_anomaly",
					Severity:    "high",
					Description: "ML model detected anomaly",
					Score:       prediction.FraudProbability,
					Evidence:    map[string]interface{}{
						"features":     prediction.Features,
						"explanations": prediction.Explanations,
					},
				})
			}
		}
	}
	
	// Rule-based checks if enabled
	if s.ruleEngine != nil && s.rules.RulesEnabled {
		ruleResult, err := s.ruleEngine.Evaluate(ctx, features)
		if err == nil && ruleResult.Matched {
			for _, rule := range ruleResult.MatchedRules {
				result.Flags = append(result.Flags, FraudFlag{
					Type:        "pattern",
					Severity:    "medium",
					Description: fmt.Sprintf("Rule matched: %s", rule),
					Score:       ruleResult.TotalScore,
				})
			}
			result.RiskScore = math.Max(result.RiskScore, ruleResult.TotalScore)
		}
	}
	
	// Apply thresholds
	s.applyThresholds(result)
	
	// Save result
	if s.repo != nil {
		s.repo.SaveCheckResult(ctx, result)
	}
	
	// Update risk profile
	s.updateRiskProfile(ctx, c.BuyerID, result.RiskScore)
	
	return result, nil
}

// CheckBid validates a bid for fraud indicators
func (s *service) CheckBid(ctx context.Context, b *bid.Bid, buyer *account.Account) (*FraudCheckResult, error) {
	result := &FraudCheckResult{
		ID:         uuid.New(),
		EntityID:   b.ID,
		EntityType: "bid",
		Timestamp:  time.Now(),
		Approved:   true,
		Flags:      []FraudFlag{},
		Metadata:   make(map[string]interface{}),
	}
	
	// Account quality check
	if buyer.QualityScore < 50 {
		result.Flags = append(result.Flags, FraudFlag{
			Type:        "pattern",
			Severity:    "medium",
			Description: "Low account quality score",
			Score:       0.6,
		})
		result.RiskScore = math.Max(result.RiskScore, 0.6)
	}
	
	// Suspicious bid amount patterns
	if s.isSuspiciousBidAmount(b.Amount) {
		result.Flags = append(result.Flags, FraudFlag{
			Type:        "pattern",
			Severity:    "low",
			Description: "Suspicious bid amount pattern",
			Score:       0.3,
		})
		result.RiskScore = math.Max(result.RiskScore, 0.3)
	}
	
	// Velocity check for bids
	if s.velocityChecker != nil {
		velocityResult, err := s.velocityChecker.CheckVelocity(ctx, b.BuyerID, "bid_placement")
		if err == nil && !velocityResult.Passed {
			result.Flags = append(result.Flags, FraudFlag{
				Type:        "velocity",
				Severity:    "high",
				Description: fmt.Sprintf("High bid velocity: %d bids in %v", velocityResult.Count, velocityResult.TimeWindow),
				Score:       0.7,
			})
			result.RiskScore = math.Max(result.RiskScore, 0.7)
		}
		
		// Record the action
		s.velocityChecker.RecordAction(ctx, b.BuyerID, "bid_placement")
	}
	
	// Extract features for ML/rules
	features := s.extractBidFeatures(b, buyer)
	
	// ML prediction if enabled
	if s.mlEngine != nil && s.rules.MLEnabled {
		prediction, err := s.mlEngine.Predict(ctx, features)
		if err == nil {
			result.RiskScore = math.Max(result.RiskScore, prediction.FraudProbability)
			result.Confidence = prediction.Confidence
			
			if prediction.FraudProbability > 0.6 {
				result.Flags = append(result.Flags, FraudFlag{
					Type:        "ml_anomaly",
					Severity:    "medium",
					Description: "ML model detected potential fraud",
					Score:       prediction.FraudProbability,
				})
			}
		}
	}
	
	// Apply thresholds
	s.applyThresholds(result)
	
	// Save result
	if s.repo != nil {
		s.repo.SaveCheckResult(ctx, result)
	}
	
	return result, nil
}

// CheckAccount performs fraud check on an account
func (s *service) CheckAccount(ctx context.Context, acc *account.Account) (*FraudCheckResult, error) {
	result := &FraudCheckResult{
		ID:         uuid.New(),
		EntityID:   acc.ID,
		EntityType: "account",
		Timestamp:  time.Now(),
		Approved:   true,
		Flags:      []FraudFlag{},
		Metadata:   make(map[string]interface{}),
	}
	
	// Email domain check
	if s.isSuspiciousEmailDomain(acc.Email) {
		result.Flags = append(result.Flags, FraudFlag{
			Type:        "pattern",
			Severity:    "low",
			Description: "Suspicious email domain",
			Score:       0.4,
		})
		result.RiskScore = math.Max(result.RiskScore, 0.4)
	}
	
	// Phone number validation
	if !s.isValidPhoneFormat(acc.PhoneNumber) {
		result.Flags = append(result.Flags, FraudFlag{
			Type:        "pattern",
			Severity:    "medium",
			Description: "Invalid phone number format",
			Score:       0.5,
		})
		result.RiskScore = math.Max(result.RiskScore, 0.5)
	}
	
	// Historical fraud check
	if s.repo != nil {
		history, err := s.repo.GetCheckHistory(ctx, acc.ID, 10)
		if err == nil {
			fraudCount := 0
			for _, check := range history {
				if check.RiskScore > 0.8 {
					fraudCount++
				}
			}
			
			if fraudCount > 2 {
				result.Flags = append(result.Flags, FraudFlag{
					Type:        "pattern",
					Severity:    "high",
					Description: fmt.Sprintf("Historical fraud indicators: %d high-risk events", fraudCount),
					Score:       0.9,
				})
				result.RiskScore = math.Max(result.RiskScore, 0.9)
			}
		}
	}
	
	// Apply thresholds
	s.applyThresholds(result)
	
	// Save result
	if s.repo != nil {
		s.repo.SaveCheckResult(ctx, result)
	}
	
	return result, nil
}

// GetRiskScore returns current risk score for an entity
func (s *service) GetRiskScore(ctx context.Context, entityID uuid.UUID, entityType string) (float64, error) {
	// Check cache first
	s.cacheMu.RLock()
	if cached, exists := s.riskCache[entityID]; exists {
		if time.Since(cached.timestamp) < 5*time.Minute {
			s.cacheMu.RUnlock()
			return cached.score, nil
		}
	}
	s.cacheMu.RUnlock()
	
	// Get from repository
	if s.repo != nil {
		profile, err := s.repo.GetRiskProfile(ctx, entityID)
		if err == nil {
			// Cache the result
			s.cacheMu.Lock()
			s.riskCache[entityID] = &cachedRisk{
				score:     profile.CurrentRiskScore,
				timestamp: time.Now(),
			}
			s.cacheMu.Unlock()
			
			return profile.CurrentRiskScore, nil
		}
	}
	
	return 0.0, errors.NewNotFoundError("risk profile")
}

// ReportFraud reports confirmed fraud for learning
func (s *service) ReportFraud(ctx context.Context, report *FraudReport) error {
	if report == nil {
		return errors.NewValidationError("INVALID_REPORT", "fraud report cannot be nil")
	}
	
	report.ID = uuid.New()
	report.ReportedAt = time.Now()
	report.Status = "pending"
	
	// Save report
	if s.repo != nil {
		if err := s.repo.SaveFraudReport(ctx, report); err != nil {
			return errors.NewInternalError("failed to save fraud report").WithCause(err)
		}
	}
	
	// Update risk profile
	s.updateRiskProfile(ctx, report.EntityID, 1.0)
	
	// Add to blacklist if confirmed fraud
	if report.FraudType == "confirmed" && s.blacklistChecker != nil {
		// Blacklist based on entity type
		switch report.EntityType {
		case "account":
			// Would need to get account details to blacklist email/phone
		case "call":
			// Would need to get call details to blacklist phone numbers
		}
	}
	
	return nil
}

// UpdateRules updates fraud detection rules
func (s *service) UpdateRules(ctx context.Context, rules *FraudRules) error {
	if rules == nil {
		return errors.NewValidationError("INVALID_RULES", "rules cannot be nil")
	}
	
	s.mu.Lock()
	s.rules = rules
	s.mu.Unlock()
	
	return nil
}

// Helper methods

func (s *service) extractCallFeatures(c *call.Call) map[string]interface{} {
	features := make(map[string]interface{})
	
	features["buyer_id"] = c.BuyerID.String()
	features["from_number"] = c.FromNumber
	features["to_number"] = c.ToNumber
	features["direction"] = c.Direction.String()
	features["hour_of_day"] = c.StartTime.Hour()
	features["day_of_week"] = int(c.StartTime.Weekday())
	
	// Extract area codes
	if len(c.FromNumber) >= 10 {
		features["from_area_code"] = c.FromNumber[1:4]
	}
	if len(c.ToNumber) >= 10 {
		features["to_area_code"] = c.ToNumber[1:4]
	}
	
	return features
}

func (s *service) extractBidFeatures(b *bid.Bid, buyer *account.Account) map[string]interface{} {
	features := make(map[string]interface{})
	
	features["buyer_id"] = b.BuyerID.String()
	features["bid_amount"] = b.Amount
	features["quality_score"] = b.Quality.HistoricalRating
	features["hour_of_day"] = b.PlacedAt.Hour()
	features["day_of_week"] = int(b.PlacedAt.Weekday())
	
	if buyer != nil {
		features["account_age_days"] = int(time.Since(buyer.CreatedAt).Hours() / 24)
		features["account_type"] = buyer.Type
		features["account_status"] = buyer.Status
	}
	
	return features
}

func (s *service) applyThresholds(result *FraudCheckResult) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Check if MFA required
	if result.RiskScore >= s.rules.RequireMFAScore {
		result.RequiresMFA = true
	}
	
	// Check if auto-block
	if result.RiskScore >= s.rules.AutoBlockScore {
		result.Approved = false
		result.Reasons = append(result.Reasons, "Risk score exceeds auto-block threshold")
	}
	
	// Check if review required
	if result.RiskScore >= 0.6 && result.RiskScore < s.rules.AutoBlockScore {
		result.RequiresReview = true
	}
}

func (s *service) updateRiskProfile(ctx context.Context, entityID uuid.UUID, newScore float64) {
	if s.repo == nil {
		return
	}
	
	// Get existing profile
	profile, err := s.repo.GetRiskProfile(ctx, entityID)
	if err != nil {
		// Create new profile
		profile = &RiskProfile{
			EntityID:         entityID,
			CurrentRiskScore: newScore,
			HistoricalScores: []RiskScoreEntry{},
			LastCheckTime:    time.Now(),
		}
	}
	
	// Update score with exponential moving average
	alpha := 0.3 // Weight for new score
	profile.CurrentRiskScore = alpha*newScore + (1-alpha)*profile.CurrentRiskScore
	
	// Add to history
	profile.HistoricalScores = append(profile.HistoricalScores, RiskScoreEntry{
		Score:     newScore,
		Timestamp: time.Now(),
	})
	
	// Keep only last 100 entries
	if len(profile.HistoricalScores) > 100 {
		profile.HistoricalScores = profile.HistoricalScores[len(profile.HistoricalScores)-100:]
	}
	
	profile.LastCheckTime = time.Now()
	
	// Save updated profile
	s.repo.UpdateRiskProfile(ctx, profile)
	
	// Update cache
	s.cacheMu.Lock()
	s.riskCache[entityID] = &cachedRisk{
		score:     profile.CurrentRiskScore,
		timestamp: time.Now(),
	}
	s.cacheMu.Unlock()
}

func (s *service) isSuspiciousBidAmount(amount float64) bool {
	// Check for common test amounts
	testAmounts := []float64{1.00, 0.01, 9.99, 99.99, 100.00, 1000.00}
	for _, test := range testAmounts {
		if math.Abs(amount-test) < 0.001 {
			return true
		}
	}
	
	// Check for repeating digits (e.g., 11.11, 22.22)
	cents := int((amount - float64(int(amount))) * 100)
	if cents%11 == 0 && cents > 0 {
		return true
	}
	
	return false
}

func (s *service) isSuspiciousEmailDomain(email string) bool {
	suspiciousDomains := []string{
		"tempmail.com",
		"guerrillamail.com",
		"mailinator.com",
		"10minutemail.com",
		"throwaway.email",
	}
	
	email = strings.ToLower(email)
	for _, domain := range suspiciousDomains {
		if strings.Contains(email, domain) {
			return true
		}
	}
	
	return false
}

func (s *service) isValidPhoneFormat(phone string) bool {
	// Basic validation - should start with + and have 10-15 digits
	if !strings.HasPrefix(phone, "+") {
		return false
	}
	
	digits := 0
	for _, ch := range phone[1:] {
		if ch >= '0' && ch <= '9' {
			digits++
		}
	}
	
	return digits >= 10 && digits <= 15
}

func defaultFraudRules() *FraudRules {
	return &FraudRules{
		VelocityLimits: map[string]VelocityLimit{
			"call_placement": {
				Action:     "call_placement",
				MaxCount:   100,
				TimeWindow: 1 * time.Hour,
			},
			"bid_placement": {
				Action:     "bid_placement",
				MaxCount:   200,
				TimeWindow: 1 * time.Hour,
			},
		},
		RiskThresholds: map[string]float64{
			"low":      0.3,
			"medium":   0.6,
			"high":     0.8,
			"critical": 0.95,
		},
		MLEnabled:       true,
		RulesEnabled:    true,
		RequireMFAScore: 0.7,
		AutoBlockScore:  0.9,
	}
}