package fraud

import (
	"context"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_CheckCall(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMocks    func(*mockRepo, *mockMLEngine, *mockRuleEngine, *mockVelocityChecker, *mockBlacklistChecker)
		call          *call.Call
		expectedApproved bool
		expectedMinScore float64
		expectedFlags    int
	}{
		{
			name: "clean call passes all checks",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, re *mockRuleEngine, vc *mockVelocityChecker, bc *mockBlacklistChecker) {
				bc.On("IsBlacklisted", ctx, "+15551234567", "phone").Return(false, "", nil)
				bc.On("IsBlacklisted", ctx, "+15559876543", "phone").Return(false, "", nil)
				
				vc.On("CheckVelocity", ctx, mock.AnythingOfType("uuid.UUID"), "call_placement").Return(&VelocityResult{
					Passed: true,
					Count:  5,
					Limit:  100,
				}, nil)
				vc.On("RecordAction", ctx, mock.AnythingOfType("uuid.UUID"), "call_placement").Return(nil)
				
				ml.On("Predict", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&Prediction{
					FraudProbability: 0.1,
					Confidence:       0.9,
				}, nil)
				
				re.On("Evaluate", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&RuleResult{
					Matched: false,
				}, nil)
				
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
				r.On("GetRiskProfile", ctx, mock.AnythingOfType("uuid.UUID")).Return(&RiskProfile{
					CurrentRiskScore: 0.2,
				}, nil)
				r.On("UpdateRiskProfile", ctx, mock.AnythingOfType("*fraud.RiskProfile")).Return(nil)
			},
			call: &call.Call{
				ID:         uuid.New(),
				FromNumber: values.MustNewPhoneNumber("+15551234567"),
				ToNumber:   values.MustNewPhoneNumber("+15559876543"),
				BuyerID:    uuid.New(),
				StartTime:  time.Now(),
			},
			expectedApproved: true,
			expectedMinScore: 0.0,
			expectedFlags:    0,
		},
		{
			name: "blacklisted phone number blocks call",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, re *mockRuleEngine, vc *mockVelocityChecker, bc *mockBlacklistChecker) {
				bc.On("IsBlacklisted", ctx, "+15551234567", "phone").Return(true, "Spam caller", nil)
				
				// Only SaveCheckResult is called when blacklisted (returns early)
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
			},
			call: &call.Call{
				ID:         uuid.New(),
				FromNumber: values.MustNewPhoneNumber("+15551234567"),
				ToNumber:   values.MustNewPhoneNumber("+15559876543"),
				BuyerID:    uuid.New(),
			},
			expectedApproved: false,
			expectedMinScore: 1.0,
			expectedFlags:    1,
		},
		{
			name: "high velocity triggers flag",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, re *mockRuleEngine, vc *mockVelocityChecker, bc *mockBlacklistChecker) {
				bc.On("IsBlacklisted", ctx, mock.Anything, "phone").Return(false, "", nil)
				
				vc.On("CheckVelocity", ctx, mock.AnythingOfType("uuid.UUID"), "call_placement").Return(&VelocityResult{
					Passed:     false,
					Count:      150,
					Limit:      100,
					TimeWindow: 1 * time.Hour,
				}, nil)
				vc.On("RecordAction", ctx, mock.AnythingOfType("uuid.UUID"), "call_placement").Return(nil)
				
				ml.On("Predict", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&Prediction{
					FraudProbability: 0.2,
					Confidence:       0.8,
				}, nil)
				
				re.On("Evaluate", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&RuleResult{
					Matched: false,
				}, nil)
				
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
				r.On("GetRiskProfile", ctx, mock.AnythingOfType("uuid.UUID")).Return(&RiskProfile{}, nil)
				r.On("UpdateRiskProfile", ctx, mock.AnythingOfType("*fraud.RiskProfile")).Return(nil)
			},
			call: &call.Call{
				ID:         uuid.New(),
				FromNumber: values.MustNewPhoneNumber("+15551234567"),
				ToNumber:   values.MustNewPhoneNumber("+15559876543"),
				BuyerID:    uuid.New(),
				StartTime:  time.Now(),
			},
			expectedApproved: true, // High velocity alone doesn't block
			expectedMinScore: 0.8,
			expectedFlags:    1,
		},
		{
			name: "ML anomaly detection",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, re *mockRuleEngine, vc *mockVelocityChecker, bc *mockBlacklistChecker) {
				bc.On("IsBlacklisted", ctx, mock.Anything, "phone").Return(false, "", nil)
				
				vc.On("CheckVelocity", ctx, mock.AnythingOfType("uuid.UUID"), "call_placement").Return(&VelocityResult{
					Passed: true,
				}, nil)
				vc.On("RecordAction", ctx, mock.AnythingOfType("uuid.UUID"), "call_placement").Return(nil)
				
				ml.On("Predict", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&Prediction{
					FraudProbability: 0.85,
					Confidence:       0.95,
					Explanations:     []string{"Unusual calling pattern", "Time anomaly"},
				}, nil)
				
				re.On("Evaluate", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&RuleResult{
					Matched: false,
				}, nil)
				
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
				r.On("GetRiskProfile", ctx, mock.AnythingOfType("uuid.UUID")).Return(&RiskProfile{}, nil)
				r.On("UpdateRiskProfile", ctx, mock.AnythingOfType("*fraud.RiskProfile")).Return(nil)
			},
			call: &call.Call{
				ID:         uuid.New(),
				FromNumber: values.MustNewPhoneNumber("+15551234567"),
				ToNumber:   values.MustNewPhoneNumber("+15559876543"),
				BuyerID:    uuid.New(),
				StartTime:  time.Now(),
			},
			expectedApproved: true, // Below auto-block threshold
			expectedMinScore: 0.85,
			expectedFlags:    1,
		},
		{
			name: "rule engine match",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, re *mockRuleEngine, vc *mockVelocityChecker, bc *mockBlacklistChecker) {
				bc.On("IsBlacklisted", ctx, mock.Anything, "phone").Return(false, "", nil)
				
				vc.On("CheckVelocity", ctx, mock.AnythingOfType("uuid.UUID"), "call_placement").Return(&VelocityResult{
					Passed: true,
				}, nil)
				vc.On("RecordAction", ctx, mock.AnythingOfType("uuid.UUID"), "call_placement").Return(nil)
				
				ml.On("Predict", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&Prediction{
					FraudProbability: 0.2,
				}, nil)
				
				re.On("Evaluate", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&RuleResult{
					Matched:      true,
					MatchedRules: []string{"international_call_spike", "night_time_pattern"},
					TotalScore:   0.65,
				}, nil)
				
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
				r.On("GetRiskProfile", ctx, mock.AnythingOfType("uuid.UUID")).Return(&RiskProfile{}, nil)
				r.On("UpdateRiskProfile", ctx, mock.AnythingOfType("*fraud.RiskProfile")).Return(nil)
			},
			call: &call.Call{
				ID:         uuid.New(),
				FromNumber: values.MustNewPhoneNumber("+15551234567"),
				ToNumber:   values.MustNewPhoneNumber("+15559876543"),
				BuyerID:    uuid.New(),
				StartTime:  time.Now(),
			},
			expectedApproved: true,
			expectedMinScore: 0.65,
			expectedFlags:    2, // One flag per matched rule
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			repo := new(mockRepo)
			mlEngine := new(mockMLEngine)
			ruleEngine := new(mockRuleEngine)
			velocityChecker := new(mockVelocityChecker)
			blacklistChecker := new(mockBlacklistChecker)
			
			// Setup mocks
			tt.setupMocks(repo, mlEngine, ruleEngine, velocityChecker, blacklistChecker)
			
			// Create service
			svc := NewService(repo, mlEngine, ruleEngine, velocityChecker, blacklistChecker, nil)
			
			// Execute
			result, err := svc.CheckCall(ctx, tt.call)
			
			// Validate
			require.NoError(t, err)
			assert.Equal(t, tt.expectedApproved, result.Approved)
			assert.GreaterOrEqual(t, result.RiskScore, tt.expectedMinScore)
			assert.Len(t, result.Flags, tt.expectedFlags)
			
			// Assert expectations
			repo.AssertExpectations(t)
			mlEngine.AssertExpectations(t)
			ruleEngine.AssertExpectations(t)
			velocityChecker.AssertExpectations(t)
			blacklistChecker.AssertExpectations(t)
		})
	}
}

func TestService_CheckBid(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		setupMocks       func(*mockRepo, *mockMLEngine, *mockVelocityChecker)
		bid              *bid.Bid
		buyer            *account.Account
		expectedApproved bool
		expectedMinScore float64
		expectedFlags    int
	}{
		{
			name: "high quality buyer bid approved",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, vc *mockVelocityChecker) {
				vc.On("CheckVelocity", ctx, mock.AnythingOfType("uuid.UUID"), "bid_placement").Return(&VelocityResult{
					Passed: true,
				}, nil)
				vc.On("RecordAction", ctx, mock.AnythingOfType("uuid.UUID"), "bid_placement").Return(nil)
				
				ml.On("Predict", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&Prediction{
					FraudProbability: 0.05,
					Confidence:       0.95,
				}, nil)
				
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
			},
			bid: &bid.Bid{
				ID:      uuid.New(),
				BuyerID: uuid.New(),
				Amount:  values.MustNewMoneyFromFloat(5.50, "USD"),
				PlacedAt: time.Now(),
			},
			buyer: &account.Account{
				ID:           uuid.New(),
				QualityMetrics: values.QualityMetrics{QualityScore: 85.0},
				CreatedAt:    time.Now().Add(-30 * 24 * time.Hour),
			},
			expectedApproved: true,
			expectedMinScore: 0.0,
			expectedFlags:    0,
		},
		{
			name: "low quality buyer flagged",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, vc *mockVelocityChecker) {
				vc.On("CheckVelocity", ctx, mock.AnythingOfType("uuid.UUID"), "bid_placement").Return(&VelocityResult{
					Passed: true,
				}, nil)
				vc.On("RecordAction", ctx, mock.AnythingOfType("uuid.UUID"), "bid_placement").Return(nil)
				
				ml.On("Predict", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&Prediction{
					FraudProbability: 0.1,
				}, nil)
				
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
			},
			bid: &bid.Bid{
				ID:      uuid.New(),
				BuyerID: uuid.New(),
				Amount:  values.MustNewMoneyFromFloat(5.00, "USD"),
			},
			buyer: &account.Account{
				ID:           uuid.New(),
				QualityMetrics: values.QualityMetrics{QualityScore: 35.0}, // Low quality
			},
			expectedApproved: true,
			expectedMinScore: 0.6,
			expectedFlags:    1,
		},
		{
			name: "suspicious bid amount pattern",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, vc *mockVelocityChecker) {
				vc.On("CheckVelocity", ctx, mock.AnythingOfType("uuid.UUID"), "bid_placement").Return(&VelocityResult{
					Passed: true,
				}, nil)
				vc.On("RecordAction", ctx, mock.AnythingOfType("uuid.UUID"), "bid_placement").Return(nil)
				
				ml.On("Predict", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&Prediction{
					FraudProbability: 0.1,
				}, nil)
				
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
			},
			bid: &bid.Bid{
				ID:      uuid.New(),
				BuyerID: uuid.New(),
				Amount:  values.MustNewMoneyFromFloat(99.99, "USD"), // Suspicious test amount
			},
			buyer: &account.Account{
				ID:           uuid.New(),
				QualityMetrics: values.QualityMetrics{QualityScore: 70.0},
			},
			expectedApproved: true,
			expectedMinScore: 0.3,
			expectedFlags:    1,
		},
		{
			name: "high bid velocity",
			setupMocks: func(r *mockRepo, ml *mockMLEngine, vc *mockVelocityChecker) {
				vc.On("CheckVelocity", ctx, mock.AnythingOfType("uuid.UUID"), "bid_placement").Return(&VelocityResult{
					Passed:     false,
					Count:      250,
					Limit:      200,
					TimeWindow: 1 * time.Hour,
				}, nil)
				vc.On("RecordAction", ctx, mock.AnythingOfType("uuid.UUID"), "bid_placement").Return(nil)
				
				ml.On("Predict", ctx, mock.AnythingOfType("map[string]interface {}")).Return(&Prediction{
					FraudProbability: 0.1,
				}, nil)
				
				r.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
			},
			bid: &bid.Bid{
				ID:      uuid.New(),
				BuyerID: uuid.New(),
				Amount:  values.MustNewMoneyFromFloat(5.00, "USD"),
			},
			buyer: &account.Account{
				ID:           uuid.New(),
				QualityMetrics: values.QualityMetrics{QualityScore: 75.0},
			},
			expectedApproved: true,
			expectedMinScore: 0.7,
			expectedFlags:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			repo := new(mockRepo)
			mlEngine := new(mockMLEngine)
			velocityChecker := new(mockVelocityChecker)
			
			// Setup mocks
			tt.setupMocks(repo, mlEngine, velocityChecker)
			
			// Create service
			svc := NewService(repo, mlEngine, nil, velocityChecker, nil, nil)
			
			// Execute
			result, err := svc.CheckBid(ctx, tt.bid, tt.buyer)
			
			// Validate
			require.NoError(t, err)
			assert.Equal(t, tt.expectedApproved, result.Approved)
			assert.GreaterOrEqual(t, result.RiskScore, tt.expectedMinScore)
			assert.Len(t, result.Flags, tt.expectedFlags)
			
			// Assert expectations
			repo.AssertExpectations(t)
			mlEngine.AssertExpectations(t)
			velocityChecker.AssertExpectations(t)
		})
	}
}

func TestService_CheckAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("suspicious email domain", func(t *testing.T) {
		repo := new(mockRepo)
		repo.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
		repo.On("GetCheckHistory", ctx, mock.AnythingOfType("uuid.UUID"), 10).Return([]*FraudCheckResult{}, nil)
		
		svc := NewService(repo, nil, nil, nil, nil, nil)
		
		acc := &account.Account{
			ID:    uuid.New(),
			Email: values.MustNewEmail("test@tempmail.com"), // Suspicious domain
			PhoneNumber: values.MustNewPhoneNumber("+15551234567"),
		}
		
		result, err := svc.CheckAccount(ctx, acc)
		require.NoError(t, err)
		assert.True(t, result.Approved)
		assert.GreaterOrEqual(t, result.RiskScore, 0.4)
		assert.Len(t, result.Flags, 1)
		assert.Equal(t, "Suspicious email domain", result.Flags[0].Description)
	})
	
	t.Run("invalid phone format", func(t *testing.T) {
		t.Skip("Cannot test invalid phone numbers - domain value objects prevent creation of invalid phone numbers")
		repo := new(mockRepo)
		repo.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
		repo.On("GetCheckHistory", ctx, mock.AnythingOfType("uuid.UUID"), 10).Return([]*FraudCheckResult{}, nil)
		
		svc := NewService(repo, nil, nil, nil, nil, nil)
		
		acc := &account.Account{
			ID:    uuid.New(),
			Email: values.MustNewEmail("test@example.com"),
			PhoneNumber: values.MustNewPhoneNumber("+11234567890"), // Fixed + prefix
		}
		
		result, err := svc.CheckAccount(ctx, acc)
		require.NoError(t, err)
		assert.True(t, result.Approved)
		assert.GreaterOrEqual(t, result.RiskScore, 0.5)
		assert.Len(t, result.Flags, 1)
		assert.Equal(t, "Invalid phone number format", result.Flags[0].Description)
	})
	
	t.Run("historical fraud indicators", func(t *testing.T) {
		repo := new(mockRepo)
		
		// Mock history with high-risk events
		history := []*FraudCheckResult{
			{RiskScore: 0.9},
			{RiskScore: 0.85},
			{RiskScore: 0.95},
			{RiskScore: 0.2},
		}
		
		repo.On("GetCheckHistory", ctx, mock.AnythingOfType("uuid.UUID"), 10).Return(history, nil)
		repo.On("SaveCheckResult", ctx, mock.AnythingOfType("*fraud.FraudCheckResult")).Return(nil)
		
		svc := NewService(repo, nil, nil, nil, nil, nil)
		
		acc := &account.Account{
			ID:    uuid.New(),
			Email: values.MustNewEmail("test@example.com"),
			PhoneNumber: values.MustNewPhoneNumber("+15551234567"),
		}
		
		result, err := svc.CheckAccount(ctx, acc)
		require.NoError(t, err)
		assert.False(t, result.Approved) // Should be blocked due to high risk (0.9 == auto-block threshold)
		assert.GreaterOrEqual(t, result.RiskScore, 0.9)
		assert.Len(t, result.Flags, 1)
		assert.Contains(t, result.Flags[0].Description, "Historical fraud indicators")
	})
}

func TestService_UpdateRules(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil)
	
	newRules := &FraudRules{
		MLEnabled:       false,
		RulesEnabled:    true,
		RequireMFAScore: 0.8,
		AutoBlockScore:  0.95,
	}
	
	err := svc.UpdateRules(context.Background(), newRules)
	require.NoError(t, err)
	
	// Verify rules were updated
	s := svc.(*service)
	s.mu.RLock()
	assert.False(t, s.rules.MLEnabled)
	assert.True(t, s.rules.RulesEnabled)
	assert.Equal(t, 0.8, s.rules.RequireMFAScore)
	assert.Equal(t, 0.95, s.rules.AutoBlockScore)
	s.mu.RUnlock()
	
	// Test nil rules
	err = svc.UpdateRules(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rules cannot be nil")
}

func TestService_Thresholds(t *testing.T) {
	
	tests := []struct {
		name         string
		riskScore    float64
		rules        *FraudRules
		expectMFA    bool
		expectBlock  bool
		expectReview bool
	}{
		{
			name:      "low risk - no actions",
			riskScore: 0.2,
			rules: &FraudRules{
				RequireMFAScore: 0.7,
				AutoBlockScore:  0.9,
			},
			expectMFA:    false,
			expectBlock:  false,
			expectReview: false,
		},
		{
			name:      "medium risk - requires review",
			riskScore: 0.65,
			rules: &FraudRules{
				RequireMFAScore: 0.7,
				AutoBlockScore:  0.9,
			},
			expectMFA:    false,
			expectBlock:  false,
			expectReview: true,
		},
		{
			name:      "high risk - requires MFA",
			riskScore: 0.75,
			rules: &FraudRules{
				RequireMFAScore: 0.7,
				AutoBlockScore:  0.9,
			},
			expectMFA:    true,
			expectBlock:  false,
			expectReview: true,
		},
		{
			name:      "critical risk - auto block",
			riskScore: 0.95,
			rules: &FraudRules{
				RequireMFAScore: 0.7,
				AutoBlockScore:  0.9,
			},
			expectMFA:    true,
			expectBlock:  true,
			expectReview: false, // No point in review if blocked
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &service{
				rules: tt.rules,
			}
			
			result := &FraudCheckResult{
				RiskScore: tt.riskScore,
				Approved:  true,
				Reasons:   []string{},
			}
			
			svc.applyThresholds(result)
			
			assert.Equal(t, tt.expectMFA, result.RequiresMFA)
			assert.Equal(t, !tt.expectBlock, result.Approved)
			assert.Equal(t, tt.expectReview, result.RequiresReview)
		})
	}
}

// Mock implementations

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) SaveCheckResult(ctx context.Context, result *FraudCheckResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *mockRepo) GetCheckHistory(ctx context.Context, entityID uuid.UUID, limit int) ([]*FraudCheckResult, error) {
	args := m.Called(ctx, entityID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*FraudCheckResult), args.Error(1)
}

func (m *mockRepo) SaveFraudReport(ctx context.Context, report *FraudReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *mockRepo) GetRiskProfile(ctx context.Context, entityID uuid.UUID) (*RiskProfile, error) {
	args := m.Called(ctx, entityID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RiskProfile), args.Error(1)
}

func (m *mockRepo) UpdateRiskProfile(ctx context.Context, profile *RiskProfile) error {
	args := m.Called(ctx, profile)
	return args.Error(0)
}

type mockMLEngine struct {
	mock.Mock
}

func (m *mockMLEngine) Predict(ctx context.Context, features map[string]interface{}) (*Prediction, error) {
	args := m.Called(ctx, features)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Prediction), args.Error(1)
}

func (m *mockMLEngine) Train(ctx context.Context, samples []*TrainingSample) error {
	args := m.Called(ctx, samples)
	return args.Error(0)
}

func (m *mockMLEngine) GetModelMetrics(ctx context.Context) (*ModelMetrics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ModelMetrics), args.Error(1)
}

type mockRuleEngine struct {
	mock.Mock
}

func (m *mockRuleEngine) Evaluate(ctx context.Context, data map[string]interface{}) (*RuleResult, error) {
	args := m.Called(ctx, data)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RuleResult), args.Error(1)
}

func (m *mockRuleEngine) AddRule(rule *Rule) error {
	args := m.Called(rule)
	return args.Error(0)
}

func (m *mockRuleEngine) RemoveRule(ruleID string) error {
	args := m.Called(ruleID)
	return args.Error(0)
}

func (m *mockRuleEngine) ListRules() ([]*Rule, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Rule), args.Error(1)
}

type mockVelocityChecker struct {
	mock.Mock
}

func (m *mockVelocityChecker) CheckVelocity(ctx context.Context, entityID uuid.UUID, action string) (*VelocityResult, error) {
	args := m.Called(ctx, entityID, action)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*VelocityResult), args.Error(1)
}

func (m *mockVelocityChecker) RecordAction(ctx context.Context, entityID uuid.UUID, action string) error {
	args := m.Called(ctx, entityID, action)
	return args.Error(0)
}

type mockBlacklistChecker struct {
	mock.Mock
}

func (m *mockBlacklistChecker) IsBlacklisted(ctx context.Context, identifier string, identifierType string) (bool, string, error) {
	args := m.Called(ctx, identifier, identifierType)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *mockBlacklistChecker) AddToBlacklist(ctx context.Context, identifier string, identifierType string, reason string) error {
	args := m.Called(ctx, identifier, identifierType, reason)
	return args.Error(0)
}

func (m *mockBlacklistChecker) RemoveFromBlacklist(ctx context.Context, identifier string, identifierType string) error {
	args := m.Called(ctx, identifier, identifierType)
	return args.Error(0)
}