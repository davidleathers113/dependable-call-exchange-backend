package service

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	domainConsent "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/repository"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/fraud"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/telephony"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ServiceFactories holds all service factory functions
type ServiceFactories struct {
	repositories *repository.Repositories
}

// NewServiceFactories creates a new service factory collection
func NewServiceFactories(repos *repository.Repositories) *ServiceFactories {
	return &ServiceFactories{
		repositories: repos,
	}
}

// Simple mock implementations to avoid import cycles
type mockNotificationService struct{}

func (m *mockNotificationService) NotifyBidPlaced(ctx context.Context, bid *bid.Bid) error {
	return nil
}
func (m *mockNotificationService) NotifyBidWon(ctx context.Context, bid *bid.Bid) error  { return nil }
func (m *mockNotificationService) NotifyBidLost(ctx context.Context, bid *bid.Bid) error { return nil }
func (m *mockNotificationService) NotifyBidExpired(ctx context.Context, bid *bid.Bid) error {
	return nil
}
func (m *mockNotificationService) NotifyAuctionStarted(ctx context.Context, callID uuid.UUID) error {
	return nil
}
func (m *mockNotificationService) NotifyAuctionClosed(ctx context.Context, result any) error {
	return nil
}

type mockMetricsCollector struct{}

func (m *mockMetricsCollector) RecordBidPlaced(ctx context.Context, bid *bid.Bid) {}
func (m *mockMetricsCollector) RecordAuctionDuration(ctx context.Context, callID uuid.UUID, duration time.Duration) {
}
func (m *mockMetricsCollector) RecordBidAmount(ctx context.Context, amount float64) {}
func (m *mockMetricsCollector) RecordBidValidation(ctx context.Context, bidID uuid.UUID, valid bool, reason string) {
}
func (m *mockMetricsCollector) RecordAuctionParticipants(ctx context.Context, callID uuid.UUID, count int) {
}

type mockFraudChecker struct{}

func (m *mockFraudChecker) CheckBid(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*bidding.FraudCheckResult, error) {
	return &bidding.FraudCheckResult{
		Approved:    true,
		RiskScore:   0.1,
		Reasons:     []string{},
		Flags:       []string{},
		RequiresMFA: false,
	}, nil
}
func (m *mockFraudChecker) GetRiskScore(ctx context.Context, buyerID uuid.UUID) (float64, error) {
	return 0.1, nil
}

type mockRoutingMetricsCollector struct{}

func (m *mockRoutingMetricsCollector) RecordRoutingDecision(ctx context.Context, decision *callrouting.RoutingDecision) {
}
func (m *mockRoutingMetricsCollector) RecordRoutingLatency(ctx context.Context, algorithm string, latency time.Duration) {
}

type mockTelephonyProvider struct{}

func (m *mockTelephonyProvider) InitiateCall(ctx context.Context, from, to, callbackURL string) (string, error) {
	return "mock-call-sid-" + uuid.New().String(), nil
}
func (m *mockTelephonyProvider) TerminateCall(ctx context.Context, callSID string) error { return nil }
func (m *mockTelephonyProvider) GetCallStatus(ctx context.Context, callSID string) (*telephony.ProviderCallStatus, error) {
	return &telephony.ProviderCallStatus{
		Status:   "in-progress",
		Duration: 30,
		Price:    nil,
	}, nil
}
func (m *mockTelephonyProvider) TransferCall(ctx context.Context, callSID, toNumber string) error {
	return nil
}
func (m *mockTelephonyProvider) SendDTMF(ctx context.Context, callSID, digits string) error {
	return nil
}
func (m *mockTelephonyProvider) BridgeCalls(ctx context.Context, callSID1, callSID2 string) error {
	return nil
}
func (m *mockTelephonyProvider) GetProviderName() string { return "mock-provider" }

type mockEventPublisher struct{}

func (m *mockEventPublisher) PublishCallEvent(ctx context.Context, event *telephony.CallEvent) error {
	return nil
}

type mockTelephonyMetricsCollector struct{}

func (m *mockTelephonyMetricsCollector) RecordCallInitiated(ctx context.Context, provider string) {}
func (m *mockTelephonyMetricsCollector) RecordCallCompleted(ctx context.Context, duration time.Duration, cost float64) {
}
func (m *mockTelephonyMetricsCollector) RecordCallFailed(ctx context.Context, reason string) {}
func (m *mockTelephonyMetricsCollector) RecordProviderLatency(ctx context.Context, provider, operation string, duration time.Duration) {
}

// Fraud service mocks
type mockMLEngine struct{}

func (m *mockMLEngine) Predict(ctx context.Context, features fraud.MLFeatures) (*fraud.Prediction, error) {
	return &fraud.Prediction{
		FraudProbability: 0.1,
		Confidence:       0.8,
		Features:         map[string]float64{},
		Explanations:     []string{"Low risk pattern"},
	}, nil
}

func (m *mockMLEngine) PredictLegacy(ctx context.Context, features map[string]interface{}) (*fraud.Prediction, error) {
	return &fraud.Prediction{
		FraudProbability: 0.1,
		Confidence:       0.8,
		Features:         map[string]float64{},
		Explanations:     []string{"Low risk pattern (legacy)"},
	}, nil
}
func (m *mockMLEngine) Train(ctx context.Context, samples []*fraud.TrainingSample) error { return nil }
func (m *mockMLEngine) GetModelMetrics(ctx context.Context) (*fraud.ModelMetrics, error) {
	return &fraud.ModelMetrics{
		Accuracy:    0.95,
		Precision:   0.92,
		Recall:      0.88,
		F1Score:     0.90,
		AUC:         0.93,
		LastTrained: time.Now().Add(-24 * time.Hour),
		SampleCount: 10000,
	}, nil
}

type mockRuleEngine struct{}

func (m *mockRuleEngine) Evaluate(ctx context.Context, features fraud.MLFeatures) (*fraud.RuleResult, error) {
	return &fraud.RuleResult{
		Matched:      false,
		MatchedRules: []string{},
		TotalScore:   0.0,
		Actions:      []string{},
	}, nil
}

func (m *mockRuleEngine) EvaluateLegacy(ctx context.Context, data map[string]interface{}) (*fraud.RuleResult, error) {
	return &fraud.RuleResult{
		Matched:      false,
		MatchedRules: []string{},
		TotalScore:   0.0,
		Actions:      []string{},
	}, nil
}
func (m *mockRuleEngine) AddRule(rule *fraud.Rule) error    { return nil }
func (m *mockRuleEngine) RemoveRule(ruleID string) error    { return nil }
func (m *mockRuleEngine) ListRules() ([]*fraud.Rule, error) { return []*fraud.Rule{}, nil }

type mockVelocityChecker struct{}

func (m *mockVelocityChecker) CheckVelocity(ctx context.Context, entityID uuid.UUID, action string) (*fraud.VelocityResult, error) {
	return &fraud.VelocityResult{
		Passed:     true,
		Count:      1,
		TimeWindow: time.Hour,
	}, nil
}
func (m *mockVelocityChecker) RecordAction(ctx context.Context, entityID uuid.UUID, action string) error {
	return nil
}

type mockBlacklistChecker struct{}

func (m *mockBlacklistChecker) IsBlacklisted(ctx context.Context, identifier, identifierType string) (bool, string, error) {
	return false, "", nil
}
func (m *mockBlacklistChecker) AddToBlacklist(ctx context.Context, identifier, identifierType, reason string) error {
	return nil
}
func (m *mockBlacklistChecker) RemoveFromBlacklist(ctx context.Context, identifier, identifierType string) error {
	return nil
}

type mockFraudRepository struct{}

func (m *mockFraudRepository) SaveCheckResult(ctx context.Context, result *fraud.FraudCheckResult) error {
	return nil
}
func (m *mockFraudRepository) GetCheckHistory(ctx context.Context, entityID uuid.UUID, limit int) ([]*fraud.FraudCheckResult, error) {
	return []*fraud.FraudCheckResult{}, nil
}
func (m *mockFraudRepository) GetRiskProfile(ctx context.Context, entityID uuid.UUID) (*fraud.RiskProfile, error) {
	return &fraud.RiskProfile{
		EntityID:         entityID,
		CurrentRiskScore: 0.1,
		HistoricalScores: []fraud.RiskScoreEntry{},
		LastCheckTime:    time.Now(),
	}, nil
}
func (m *mockFraudRepository) UpdateRiskProfile(ctx context.Context, profile *fraud.RiskProfile) error {
	return nil
}
func (m *mockFraudRepository) SaveFraudReport(ctx context.Context, report *fraud.FraudReport) error {
	return nil
}

// Mock consent repositories
type mockConsentRepository struct{}

// Implement consent.Repository interface
func (m *mockConsentRepository) Save(ctx context.Context, consent *domainConsent.ConsentAggregate) error {
	return nil
}

func (m *mockConsentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domainConsent.ConsentAggregate, error) {
	// Return a mock consent aggregate
	return &domainConsent.ConsentAggregate{}, nil
}

func (m *mockConsentRepository) GetByConsumerAndType(ctx context.Context, consumerID uuid.UUID, consentType domainConsent.Type) (*domainConsent.ConsentAggregate, error) {
	// Return nil to simulate no existing consent
	return nil, nil
}

func (m *mockConsentRepository) GetByConsumerAndBusiness(ctx context.Context, consumerID, businessID uuid.UUID) ([]*domainConsent.ConsentAggregate, error) {
	return []*domainConsent.ConsentAggregate{}, nil
}

func (m *mockConsentRepository) FindActiveConsent(ctx context.Context, consumerID, businessID uuid.UUID, channel domainConsent.Channel) (*domainConsent.ConsentAggregate, error) {
	return nil, nil
}

func (m *mockConsentRepository) FindByPhoneNumber(ctx context.Context, phoneNumber string, businessID uuid.UUID) ([]*domainConsent.ConsentAggregate, error) {
	return []*domainConsent.ConsentAggregate{}, nil
}

func (m *mockConsentRepository) ListExpired(ctx context.Context, before time.Time) ([]*domainConsent.ConsentAggregate, error) {
	return []*domainConsent.ConsentAggregate{}, nil
}

func (m *mockConsentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

// Mock consumer repository
type mockConsumerRepository struct{}

// Implement consent.ConsumerRepository interface
func (m *mockConsumerRepository) Save(ctx context.Context, consumer *domainConsent.Consumer) error {
	return nil
}

func (m *mockConsumerRepository) GetByID(ctx context.Context, id uuid.UUID) (*domainConsent.Consumer, error) {
	return &domainConsent.Consumer{}, nil
}

func (m *mockConsumerRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*domainConsent.Consumer, error) {
	// Return nil to simulate no existing consumer
	return nil, nil
}

func (m *mockConsumerRepository) GetByEmail(ctx context.Context, email string) (*domainConsent.Consumer, error) {
	return nil, nil
}

func (m *mockConsumerRepository) FindOrCreate(ctx context.Context, phoneNumber string, email *string, firstName, lastName string) (*domainConsent.Consumer, error) {
	// Create a mock consumer
	consumer := &domainConsent.Consumer{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Create PhoneNumber value object if phoneNumber is provided
	if phoneNumber != "" {
		phone, err := values.NewPhoneNumber(phoneNumber)
		if err != nil {
			return nil, err
		}
		consumer.PhoneNumber = &phone
	}
	
	if email != nil {
		consumer.Email = email
	}
	consumer.FirstName = firstName
	consumer.LastName = lastName
	return consumer, nil
}

// Mock consent query repository
type mockConsentQueryRepository struct{}

// Implement consent.QueryRepository interface
func (m *mockConsentQueryRepository) Find(ctx context.Context, filter domainConsent.ConsentFilter) ([]*domainConsent.ConsentAggregate, error) {
	return []*domainConsent.ConsentAggregate{}, nil
}

func (m *mockConsentQueryRepository) FindActiveByConsumer(ctx context.Context, consumerID uuid.UUID) ([]*domainConsent.ConsentAggregate, error) {
	return []*domainConsent.ConsentAggregate{}, nil
}

func (m *mockConsentQueryRepository) FindByFilters(ctx context.Context, filters domainConsent.QueryFilters) ([]*domainConsent.ConsentAggregate, error) {
	return []*domainConsent.ConsentAggregate{}, nil
}

func (m *mockConsentQueryRepository) Count(ctx context.Context, filter domainConsent.ConsentFilter) (int64, error) {
	return 0, nil
}

func (m *mockConsentQueryRepository) GetConsentHistory(ctx context.Context, consentID uuid.UUID) ([]domainConsent.ConsentVersion, error) {
	return []domainConsent.ConsentVersion{}, nil
}

func (m *mockConsentQueryRepository) GetProofs(ctx context.Context, consentID uuid.UUID) ([]domainConsent.ConsentProof, error) {
	return []domainConsent.ConsentProof{}, nil
}

func (m *mockConsentQueryRepository) GetMetrics(ctx context.Context, query domainConsent.MetricsQuery) (*domainConsent.ConsentMetrics, error) {
	return &domainConsent.ConsentMetrics{
		TotalGrants:  0,
		TotalRevokes: 0,
		ActiveCount:  0,
		Trends:       []domainConsent.MetricTrend{},
	}, nil
}

func (m *mockConsentQueryRepository) FindExpiring(ctx context.Context, days int) ([]*domainConsent.ConsentAggregate, error) {
	return []*domainConsent.ConsentAggregate{}, nil
}

// Mock compliance checker
type mockComplianceChecker struct{}

func (m *mockComplianceChecker) CheckConsentRequirements(ctx context.Context, phoneNumber string, consentType domainConsent.Type) (*compliance.ComplianceRule, error) {
	// Return a basic compliance rule
	return &compliance.ComplianceRule{
		ID:          uuid.New(),
		Name:        "TCPA Basic",
		Type:        compliance.RuleTypeTCPA,
		Status:      compliance.RuleStatusActive,
		Description: "Basic TCPA compliance",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		EffectiveAt: time.Now(),
		CreatedBy:   uuid.New(),
		Priority:    1,
	}, nil
}

func (m *mockComplianceChecker) ValidateConsentGrant(ctx context.Context, req consent.GrantConsentRequest) error {
	// Always pass validation for now
	return nil
}

// Mock consent event publisher
type mockConsentEventPublisher struct{}

func (m *mockConsentEventPublisher) PublishConsentGranted(ctx context.Context, event domainConsent.ConsentCreatedEvent) error {
	return nil
}

func (m *mockConsentEventPublisher) PublishConsentRevoked(ctx context.Context, event domainConsent.ConsentRevokedEvent) error {
	return nil
}

func (m *mockConsentEventPublisher) PublishConsentUpdated(ctx context.Context, event domainConsent.ConsentUpdatedEvent) error {
	return nil
}

// CreateBiddingService creates a new bidding service with all dependencies
func (f *ServiceFactories) CreateBiddingService() bidding.Service {
	// Create external service dependencies (mocks for now)
	fraudChecker := &mockFraudChecker{}
	notifier := &mockNotificationService{}
	metrics := &mockMetricsCollector{}

	// Use repository interfaces from the repositories collection
	bidRepo := f.repositories.Bid
	callRepo := f.repositories.Call
	accountRepo := f.repositories.Account

	// Create service with proper dependency injection
	return bidding.NewService(
		bidRepo,      // BidRepository
		callRepo,     // CallRepository
		accountRepo,  // AccountRepository
		fraudChecker, // FraudChecker
		notifier,     // NotificationService
		metrics,      // MetricsCollector
	)
}

// CreateCallRoutingService creates a new call routing service with all dependencies
func (f *ServiceFactories) CreateCallRoutingService(consentService consent.Service) callrouting.Service {
	// Create external service dependencies (mocks for now)
	metrics := &mockRoutingMetricsCollector{}

	// Create default routing rules
	initialRules := &callrouting.RoutingRules{
		Algorithm:      "round-robin",
		QualityWeight:  0.4,
		PriceWeight:    0.4,
		CapacityWeight: 0.2,
	}
	
	// Create consent adapter
	consentAdapter := consent.NewRoutingAdapter(consentService)

	// Create service with proper dependency injection using adapters
	return callrouting.NewService(
		f.repositories.Call, // CallRepository
		repository.NewCallRoutingBidRepository(f.repositories.Bid),         // BidRepository
		repository.NewCallRoutingAccountRepository(f.repositories.Account), // AccountRepository
		consentAdapter, // ConsentService
		metrics,        // MetricsCollector
		initialRules,   // RoutingRules
	)
}

// CreateFraudService creates a new fraud detection service with all dependencies
func (f *ServiceFactories) CreateFraudService() fraud.Service {
	// Create external service dependencies (mocks for now)
	repo := &mockFraudRepository{}
	mlEngine := &mockMLEngine{}
	ruleEngine := &mockRuleEngine{}
	velocityChecker := &mockVelocityChecker{}
	blacklistChecker := &mockBlacklistChecker{}

	// Create default fraud rules
	initialRules := &fraud.FraudRules{
		VelocityLimits: map[string]fraud.VelocityLimit{
			"call_placement": {
				Action:       "call_placement",
				MaxCount:     100,
				TimeWindow:   time.Hour,
				UniqueFields: []string{},
			},
			"bid_placement": {
				Action:       "bid_placement",
				MaxCount:     200,
				TimeWindow:   time.Hour,
				UniqueFields: []string{},
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

	// Create service with proper dependency injection
	return fraud.NewService(
		repo,             // Repository
		mlEngine,         // MLEngine
		ruleEngine,       // RuleEngine
		velocityChecker,  // VelocityChecker
		blacklistChecker, // BlacklistChecker
		initialRules,     // FraudRules
	)
}

// CreateTelephonyService creates a new telephony service with all dependencies
func (f *ServiceFactories) CreateTelephonyService() telephony.Service {
	// Create external service dependencies (mocks for now)
	provider := &mockTelephonyProvider{}
	eventPublisher := &mockEventPublisher{}
	metrics := &mockTelephonyMetricsCollector{}

	// Create service with proper dependency injection
	return telephony.NewService(
		f.repositories.Call, // CallRepository
		provider,            // Provider
		eventPublisher,      // EventPublisher
		metrics,             // MetricsCollector
	)
}

// CreateConsentService creates a new consent service with all dependencies
func (f *ServiceFactories) CreateConsentService() consent.Service {
	// Create logger
	logger, _ := zap.NewProduction()
	
	// Create mock dependencies
	consentRepo := &mockConsentRepository{}
	consumerRepo := &mockConsumerRepository{}
	queryRepo := &mockConsentQueryRepository{}
	complianceChecker := &mockComplianceChecker{}
	eventPublisher := &mockConsentEventPublisher{}
	
	// Create service with proper dependency injection
	return consent.NewService(
		logger,
		consentRepo,       // ConsentRepository
		consumerRepo,      // ConsumerRepository  
		queryRepo,         // ConsentQueryRepository
		complianceChecker, // ComplianceChecker
		eventPublisher,    // EventPublisher
	)
}
