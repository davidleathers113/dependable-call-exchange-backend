package fraud

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/google/uuid"
)

// Service defines the fraud detection service interface
type Service interface {
	// CheckCall validates a call for fraud indicators
	CheckCall(ctx context.Context, call *call.Call) (*FraudCheckResult, error)
	// CheckBid validates a bid for fraud indicators
	CheckBid(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*FraudCheckResult, error)
	// CheckAccount performs fraud check on an account
	CheckAccount(ctx context.Context, account *account.Account) (*FraudCheckResult, error)
	// GetRiskScore returns current risk score for an entity
	GetRiskScore(ctx context.Context, entityID uuid.UUID, entityType string) (float64, error)
	// ReportFraud reports confirmed fraud for learning
	ReportFraud(ctx context.Context, report *FraudReport) error
	// UpdateRules updates fraud detection rules
	UpdateRules(ctx context.Context, rules *FraudRules) error
}

// MLEngine defines the machine learning engine interface
type MLEngine interface {
	// Predict runs fraud prediction on features
	Predict(ctx context.Context, features map[string]interface{}) (*Prediction, error)
	// Train updates the model with new data
	Train(ctx context.Context, samples []*TrainingSample) error
	// GetModelMetrics returns current model performance
	GetModelMetrics(ctx context.Context) (*ModelMetrics, error)
}

// RuleEngine defines the rule-based fraud detection interface
type RuleEngine interface {
	// Evaluate runs rules against data
	Evaluate(ctx context.Context, data map[string]interface{}) (*RuleResult, error)
	// AddRule adds a new fraud detection rule
	AddRule(rule *Rule) error
	// RemoveRule removes a fraud detection rule
	RemoveRule(ruleID string) error
	// ListRules returns all active rules
	ListRules() ([]*Rule, error)
}

// Repository defines the fraud data storage interface
type Repository interface {
	// SaveCheckResult stores fraud check result
	SaveCheckResult(ctx context.Context, result *FraudCheckResult) error
	// GetCheckHistory retrieves fraud check history
	GetCheckHistory(ctx context.Context, entityID uuid.UUID, limit int) ([]*FraudCheckResult, error)
	// SaveFraudReport stores fraud report
	SaveFraudReport(ctx context.Context, report *FraudReport) error
	// GetRiskProfile retrieves risk profile
	GetRiskProfile(ctx context.Context, entityID uuid.UUID) (*RiskProfile, error)
	// UpdateRiskProfile updates risk profile
	UpdateRiskProfile(ctx context.Context, profile *RiskProfile) error
}

// VelocityChecker defines the interface for velocity checks
type VelocityChecker interface {
	// CheckVelocity performs velocity analysis
	CheckVelocity(ctx context.Context, entityID uuid.UUID, action string) (*VelocityResult, error)
	// RecordAction records an action for velocity tracking
	RecordAction(ctx context.Context, entityID uuid.UUID, action string) error
}

// BlacklistChecker defines the interface for blacklist checks
type BlacklistChecker interface {
	// IsBlacklisted checks if entity is blacklisted
	IsBlacklisted(ctx context.Context, identifier string, identifierType string) (bool, string, error)
	// AddToBlacklist adds entity to blacklist
	AddToBlacklist(ctx context.Context, identifier string, identifierType string, reason string) error
	// RemoveFromBlacklist removes entity from blacklist
	RemoveFromBlacklist(ctx context.Context, identifier string, identifierType string) error
}

// FraudCheckResult represents the outcome of a fraud check
type FraudCheckResult struct {
	ID           uuid.UUID
	EntityID     uuid.UUID
	EntityType   string // "call", "bid", "account"
	Timestamp    time.Time
	Approved     bool
	RiskScore    float64 // 0.0 - 1.0
	Confidence   float64 // 0.0 - 1.0
	Reasons      []string
	Flags        []FraudFlag
	RequiresMFA  bool
	RequiresReview bool
	Metadata     map[string]interface{}
}

// FraudFlag represents a specific fraud indicator
type FraudFlag struct {
	Type        string  // "velocity", "pattern", "blacklist", "ml_anomaly"
	Severity    string  // "low", "medium", "high", "critical"
	Description string
	Score       float64
	Evidence    map[string]interface{}
}

// FraudReport represents a confirmed fraud report
type FraudReport struct {
	ID           uuid.UUID
	EntityID     uuid.UUID
	EntityType   string
	ReportedAt   time.Time
	ReportedBy   uuid.UUID
	FraudType    string
	Description  string
	Evidence     map[string]interface{}
	ActionTaken  string
	Status       string // "pending", "confirmed", "false_positive"
}

// FraudRules represents configurable fraud detection rules
type FraudRules struct {
	VelocityLimits    map[string]VelocityLimit
	RiskThresholds    map[string]float64
	BlacklistPatterns []string
	MLEnabled         bool
	RulesEnabled      bool
	RequireMFAScore   float64
	AutoBlockScore    float64
}

// VelocityLimit defines rate limits for velocity checks
type VelocityLimit struct {
	Action       string
	MaxCount     int
	TimeWindow   time.Duration
	UniqueFields []string // Fields that must be unique
}

// Prediction represents ML model prediction
type Prediction struct {
	FraudProbability float64
	Confidence       float64
	Features         map[string]float64
	Explanations     []string
}

// TrainingSample represents data for model training
type TrainingSample struct {
	Features  map[string]interface{}
	Label     bool // true = fraud, false = legitimate
	Weight    float64
	Timestamp time.Time
}

// ModelMetrics represents ML model performance metrics
type ModelMetrics struct {
	Accuracy   float64
	Precision  float64
	Recall     float64
	F1Score    float64
	AUC        float64
	LastTrained time.Time
	SampleCount int
}

// Rule represents a fraud detection rule
type Rule struct {
	ID          string
	Name        string
	Description string
	Conditions  []Condition
	Action      string // "block", "flag", "review"
	Score       float64
	Enabled     bool
}

// Condition represents a rule condition
type Condition struct {
	Field    string
	Operator string // "eq", "gt", "lt", "contains", "regex"
	Value    interface{}
	Logic    string // "AND", "OR"
}

// RuleResult represents rule evaluation outcome
type RuleResult struct {
	Matched      bool
	MatchedRules []string
	TotalScore   float64
	Actions      []string
}

// RiskProfile represents entity risk profile
type RiskProfile struct {
	EntityID         uuid.UUID
	EntityType       string
	CurrentRiskScore float64
	HistoricalScores []RiskScoreEntry
	FraudCount       int
	LastCheckTime    time.Time
	Attributes       map[string]interface{}
}

// RiskScoreEntry represents historical risk score
type RiskScoreEntry struct {
	Score     float64
	Timestamp time.Time
	Reason    string
}

// VelocityResult represents velocity check outcome
type VelocityResult struct {
	Passed      bool
	Count       int
	TimeWindow  time.Duration
	Limit       int
	ViolationType string
}