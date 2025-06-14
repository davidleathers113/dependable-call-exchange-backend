# Fraud Detection Domain Specification

## Executive Summary

### Problem Statement
The Dependable Call Exchange platform currently lacks fraud detection capabilities, leaving it vulnerable to:
- **Revenue Loss**: Fraudulent calls can result in chargebacks and disputed payments
- **Platform Abuse**: Bad actors can manipulate bidding systems and routing algorithms
- **Reputation Damage**: Unchecked fraud erodes trust between buyers and sellers
- **Compliance Risk**: Failure to detect fraudulent activity may violate regulatory requirements

### Business Impact
- **Financial**: Estimated 3-5% revenue loss from undetected fraud
- **Operational**: Manual fraud investigation consuming 15+ hours/week
- **Growth**: Platform reputation limiting new buyer/seller acquisition

### Solution Overview
Implement a comprehensive ML-powered fraud detection domain that provides:
- Real-time fraud scoring with < 5ms latency
- Multi-dimensional pattern recognition
- Network graph analysis for relationship detection
- Behavioral anomaly detection
- Explainable AI decisions for compliance

## Domain Model

```go
package fraud

import (
    "time"
    "github.com/google/uuid"
    "github.com/shopspring/decimal"
)

// Core Entities

// FraudScore represents the real-time fraud assessment of an entity or transaction
type FraudScore struct {
    ID              uuid.UUID
    EntityType      EntityType      // CALL, BUYER, SELLER, CAMPAIGN
    EntityID        uuid.UUID
    Score           float64         // 0.0-1.0 (1.0 = highest risk)
    Confidence      float64         // Model confidence in score
    RiskLevel       RiskLevel       // LOW, MEDIUM, HIGH, CRITICAL
    Factors         []RiskFactor    // Contributing factors
    ModelVersion    string
    ComputedAt      time.Time
    ExpiresAt       time.Time       // Scores have TTL for caching
}

// FraudPattern represents a detected pattern of fraudulent behavior
type FraudPattern struct {
    ID              uuid.UUID
    PatternType     PatternType     // VELOCITY, GEOGRAPHIC, BEHAVIORAL, NETWORK
    Name            string
    Description     string
    Severity        Severity        // INFO, WARNING, CRITICAL
    MatchCriteria   MatchCriteria   // Pattern matching rules
    DetectionCount  int64
    LastDetectedAt  time.Time
    IsActive        bool
}

// RiskProfile aggregates historical risk data for an entity
type RiskProfile struct {
    ID                  uuid.UUID
    EntityType          EntityType
    EntityID            uuid.UUID
    OverallRiskScore    float64
    TotalTransactions   int64
    FraudulentCount     int64
    DisputedCount       int64
    VelocityMetrics     VelocityMetrics
    BehavioralMetrics   BehavioralMetrics
    NetworkMetrics      NetworkMetrics
    LastUpdatedAt       time.Time
    CreatedAt           time.Time
}

// VelocityCheck tracks rate-based fraud indicators
type VelocityCheck struct {
    ID              uuid.UUID
    EntityID        uuid.UUID
    CheckType       VelocityType    // CALLS_PER_HOUR, BIDS_PER_MINUTE, etc.
    WindowStart     time.Time
    WindowEnd       time.Time
    Count           int64
    Threshold       int64
    IsViolation     bool
    ViolationScore  float64         // Severity of violation
}

// NetworkGraph represents relationships between entities for fraud ring detection
type NetworkGraph struct {
    ID              uuid.UUID
    RootEntityID    uuid.UUID
    GraphType       GraphType       // BUYER_NETWORK, SELLER_NETWORK, PHONE_NETWORK
    Nodes           []NetworkNode
    Edges           []NetworkEdge
    RiskScore       float64
    SuspiciousNodes []uuid.UUID
    GeneratedAt     time.Time
}

// Value Objects

type EntityType string
const (
    EntityTypeCall     EntityType = "CALL"
    EntityTypeBuyer    EntityType = "BUYER"
    EntityTypeSeller   EntityType = "SELLER"
    EntityTypeCampaign EntityType = "CAMPAIGN"
)

type RiskLevel string
const (
    RiskLevelLow      RiskLevel = "LOW"
    RiskLevelMedium   RiskLevel = "MEDIUM"
    RiskLevelHigh     RiskLevel = "HIGH"
    RiskLevelCritical RiskLevel = "CRITICAL"
)

type PatternType string
const (
    PatternTypeVelocity    PatternType = "VELOCITY"
    PatternTypeGeographic  PatternType = "GEOGRAPHIC"
    PatternTypeBehavioral  PatternType = "BEHAVIORAL"
    PatternTypeNetwork     PatternType = "NETWORK"
)

// RiskFactor explains a contributing factor to the fraud score
type RiskFactor struct {
    Type        string
    Description string
    Impact      float64  // Contribution to overall score
    Evidence    map[string]interface{}
}

// VelocityMetrics tracks rate-based metrics
type VelocityMetrics struct {
    CallsPerHour        int64
    CallsPerDay         int64
    UniquePhonesPerHour int64
    BidsPerMinute       int64
    MaxBurstSize        int64
}

// BehavioralMetrics tracks behavioral patterns
type BehavioralMetrics struct {
    AvgCallDuration     time.Duration
    AvgBidAmount        decimal.Decimal
    CallTimePattern     []int // Hours of day distribution
    GeographicSpread    int   // Number of unique states/regions
    DeviceFingerprints  int   // Number of unique devices
}

// NetworkMetrics tracks network-based risk indicators
type NetworkMetrics struct {
    ConnectedEntities   int
    SharedAttributes    int     // Shared phones, IPs, etc.
    ClusterCoefficient  float64 // Network density
    SuspiciousLinks     int
}

// NetworkNode represents an entity in the fraud network graph
type NetworkNode struct {
    EntityID    uuid.UUID
    EntityType  EntityType
    RiskScore   float64
    Attributes  map[string]interface{}
}

// NetworkEdge represents a relationship between entities
type NetworkEdge struct {
    FromNodeID      uuid.UUID
    ToNodeID        uuid.UUID
    RelationType    string  // SHARES_PHONE, SHARES_IP, SIMILAR_PATTERN
    Strength        float64 // Relationship strength
    SharedAttributes []string
}
```

## Detection Strategies

### 1. Velocity Checking
Monitor transaction rates to detect abuse patterns:

```go
// Velocity rules configuration
type VelocityRule struct {
    Name            string
    EntityType      EntityType
    MetricType      string          // calls_count, unique_phones, bid_count
    Window          time.Duration
    Threshold       int64
    Action          VelocityAction  // ALERT, BLOCK, THROTTLE
    ScoreImpact     float64
}

// Example rules:
// - More than 100 calls/hour from single buyer → HIGH risk
// - More than 50 unique phone numbers/hour → CRITICAL risk
// - More than 1000 bids/minute → MEDIUM risk
```

### 2. Pattern Recognition
Identify known fraud patterns through ML and rules:

```go
// Pattern types:
// - Call Pumping: Repeated calls to premium numbers
// - Click Fraud: Automated clicking without conversion
// - Lead Recycling: Same lead sold multiple times
// - Geographic Impossibility: Calls from impossible locations
// - Time-based Anomalies: Calls outside business hours
```

### 3. Network Analysis
Detect fraud rings through relationship mapping:

```go
// Network indicators:
// - Multiple buyers sharing phone numbers
// - IP address clustering
// - Payment method reuse
// - Similar naming patterns
// - Coordinated activity timing
```

### 4. Behavioral Anomalies
ML-based detection of unusual behavior:

```go
// Behavioral features:
// - Sudden change in call volume
// - Deviation from historical patterns
// - Unusual bid amounts
// - Atypical call durations
// - Strange geographic distributions
```

### 5. Geographic Impossibilities
Detect physically impossible scenarios:

```go
// Geographic rules:
// - Calls from same number in different states within minutes
// - International calls marked as domestic
// - Calls from known VoIP/proxy locations
// - Mismatched area codes and locations
```

## ML Components

### Feature Extraction Pipeline

```go
type FeatureExtractor interface {
    ExtractCallFeatures(call *call.Call) ([]Feature, error)
    ExtractBuyerFeatures(buyer *account.Buyer) ([]Feature, error)
    ExtractSellerFeatures(seller *account.Seller) ([]Feature, error)
    ExtractNetworkFeatures(entityID uuid.UUID) ([]Feature, error)
}

type Feature struct {
    Name        string
    Value       float64
    Type        FeatureType // NUMERIC, CATEGORICAL, TEMPORAL
    Importance  float64
}
```

### Model Training Pipeline

```go
type ModelTrainer interface {
    // Train new model version
    TrainModel(trainingData []LabeledExample) (*FraudModel, error)
    
    // Evaluate model performance
    EvaluateModel(model *FraudModel, testData []LabeledExample) (*ModelMetrics, error)
    
    // A/B test models
    CompareModels(modelA, modelB *FraudModel) (*ComparisonResult, error)
    
    // Deploy model to production
    DeployModel(model *FraudModel) error
}

type ModelMetrics struct {
    Accuracy        float64
    Precision       float64
    Recall          float64
    F1Score         float64
    FalsePositiveRate float64
    AUC             float64
}
```

### Real-time Scoring Engine

```go
type ScoringEngine interface {
    // Score single transaction
    ScoreCall(ctx context.Context, call *call.Call) (*FraudScore, error)
    
    // Batch scoring for efficiency
    ScoreBatch(ctx context.Context, calls []*call.Call) ([]*FraudScore, error)
    
    // Get explanation for score
    ExplainScore(score *FraudScore) (*ScoreExplanation, error)
}

type ScoreExplanation struct {
    MainFactors     []RiskFactor
    FeatureImpacts  map[string]float64
    DecisionPath    []DecisionNode
    Recommendation  string
}
```

### Feedback Loop

```go
type FeedbackCollector interface {
    // Record actual fraud outcomes
    RecordOutcome(entityID uuid.UUID, wasFraud bool) error
    
    // Update model with feedback
    UpdateModel(feedback []FraudOutcome) error
    
    // Track model drift
    MonitorDrift() (*DriftReport, error)
}
```

## Service Implementation

### FraudDetectionService

```go
type FraudDetectionService interface {
    // Pre-call fraud check
    CheckCallFraud(ctx context.Context, call *call.Call) (*FraudCheckResult, error)
    
    // Real-time monitoring
    MonitorCall(ctx context.Context, callID uuid.UUID) error
    
    // Post-call analysis
    AnalyzeCompletedCall(ctx context.Context, callID uuid.UUID) (*FraudAnalysis, error)
    
    // Get risk profile
    GetRiskProfile(ctx context.Context, entityID uuid.UUID) (*RiskProfile, error)
    
    // Report fraud
    ReportFraud(ctx context.Context, report *FraudReport) error
}

type FraudCheckResult struct {
    Score           float64
    RiskLevel       RiskLevel
    ShouldBlock     bool
    ShouldThrottle  bool
    RequiresReview  bool
    Reasons         []string
}
```

### RiskScoringService

```go
type RiskScoringService interface {
    // Calculate risk score
    CalculateRiskScore(ctx context.Context, features []Feature) (*FraudScore, error)
    
    // Get historical scores
    GetScoreHistory(ctx context.Context, entityID uuid.UUID) ([]*FraudScore, error)
    
    // Update risk thresholds
    UpdateThresholds(ctx context.Context, thresholds *RiskThresholds) error
    
    // Get score explanation
    ExplainScore(ctx context.Context, scoreID uuid.UUID) (*ScoreExplanation, error)
}
```

### PatternAnalysisService

```go
type PatternAnalysisService interface {
    // Detect patterns in real-time
    DetectPatterns(ctx context.Context, event Event) ([]*FraudPattern, error)
    
    // Create custom pattern
    CreatePattern(ctx context.Context, pattern *FraudPattern) error
    
    // Analyze historical data
    AnalyzeHistoricalPatterns(ctx context.Context, timeRange TimeRange) (*PatternReport, error)
    
    // Get pattern matches
    GetPatternMatches(ctx context.Context, patternID uuid.UUID) ([]*PatternMatch, error)
}
```

### NetworkGraphService

```go
type NetworkGraphService interface {
    // Build entity network
    BuildNetwork(ctx context.Context, rootEntityID uuid.UUID) (*NetworkGraph, error)
    
    // Find fraud rings
    DetectFraudRings(ctx context.Context) ([]*FraudRing, error)
    
    // Analyze relationships
    AnalyzeRelationships(ctx context.Context, entityA, entityB uuid.UUID) (*RelationshipAnalysis, error)
    
    // Update network
    UpdateNetwork(ctx context.Context, event Event) error
}
```

## Integration Points

### 1. Pre-call Fraud Check
```go
// In CallRoutingService
func (s *CallRoutingService) RouteCall(ctx context.Context, call *call.Call) (*RouteDecision, error) {
    // Check fraud before routing
    fraudResult, err := s.fraudDetection.CheckCallFraud(ctx, call)
    if err != nil {
        return nil, err
    }
    
    if fraudResult.ShouldBlock {
        return &RouteDecision{
            Action: ActionBlock,
            Reason: "Failed fraud check: " + strings.Join(fraudResult.Reasons, ", "),
        }, nil
    }
    
    if fraudResult.ShouldThrottle {
        // Apply rate limiting
        s.applyThrottling(call.BuyerID, fraudResult.Score)
    }
    
    // Continue with normal routing...
}
```

### 2. Real-time Monitoring
```go
// In CallService
func (s *CallService) HandleCallEvent(ctx context.Context, event CallEvent) error {
    // Monitor ongoing calls
    if event.Type == CallEventAnswered {
        go s.fraudDetection.MonitorCall(ctx, event.CallID)
    }
    
    // Check for suspicious events
    if event.Type == CallEventDurationExceeded {
        s.fraudDetection.ReportSuspiciousActivity(ctx, event)
    }
    
    return nil
}
```

### 3. Post-call Analysis
```go
// In CallService
func (s *CallService) CompleteCall(ctx context.Context, callID uuid.UUID) error {
    // Run fraud analysis
    analysis, err := s.fraudDetection.AnalyzeCompletedCall(ctx, callID)
    if err != nil {
        s.logger.Error("fraud analysis failed", "error", err)
    }
    
    if analysis.IsSuspicious {
        // Flag for manual review
        s.flagForReview(callID, analysis)
    }
    
    return nil
}
```

### 4. Billing Integration
```go
// In BillingService
func (s *BillingService) ProcessPayment(ctx context.Context, transaction *Transaction) error {
    // Check fraud score before processing
    score, err := s.fraudDetection.GetLatestScore(ctx, transaction.BuyerID)
    if err != nil {
        return err
    }
    
    if score.RiskLevel == RiskLevelCritical {
        return &PaymentError{
            Code:    "FRAUD_RISK_TOO_HIGH",
            Message: "Payment declined due to fraud risk",
        }
    }
    
    // Add fraud score to transaction for audit
    transaction.FraudScore = score.Score
    
    return nil
}
```

## Performance Requirements

### Latency Targets
- **Pre-call scoring**: < 5ms p99
- **Feature extraction**: < 2ms p99
- **Model inference**: < 3ms p99
- **Network graph query**: < 10ms p99
- **Pattern matching**: < 5ms p99

### Throughput Requirements
- **Concurrent scoring**: 10,000 requests/second
- **Batch processing**: 100,000 calls/minute
- **Real-time monitoring**: 50,000 active calls
- **Pattern detection**: 1M events/minute

### Accuracy Targets
- **Overall accuracy**: > 99.9%
- **False positive rate**: < 0.1%
- **False negative rate**: < 0.5%
- **Model precision**: > 95%
- **Model recall**: > 90%

### Optimization Strategies

```go
// 1. Caching layer
type FraudCache struct {
    scores      *cache.LRU  // Recent scores
    profiles    *cache.LRU  // Risk profiles
    patterns    *cache.LRU  // Pattern matches
    ttl         time.Duration
}

// 2. Batch processing
func (s *ScoringEngine) ScoreBatch(calls []*call.Call) ([]*FraudScore, error) {
    // Extract features in parallel
    features := s.parallelFeatureExtraction(calls)
    
    // Batch model inference
    scores := s.model.PredictBatch(features)
    
    return scores, nil
}

// 3. Async processing
func (s *FraudDetectionService) ProcessAsync(event Event) {
    select {
    case s.eventQueue <- event:
        // Queued for processing
    default:
        // Queue full, log and metric
        s.metrics.IncrementDroppedEvents()
    }
}
```

## Implementation Plan

### Phase 1: Foundation (2 days)
- [ ] Domain entities and value objects
- [ ] Repository interfaces
- [ ] Basic service structure
- [ ] Database schema

### Phase 2: Detection Engine (2 days)
- [ ] Velocity checking implementation
- [ ] Pattern matching engine
- [ ] Rule-based detection
- [ ] Scoring algorithm

### Phase 3: ML Pipeline (2 days)
- [ ] Feature extraction
- [ ] Model training framework
- [ ] Scoring engine
- [ ] Model versioning

### Phase 4: Integration (1 day)
- [ ] Call routing integration
- [ ] Billing integration
- [ ] API endpoints
- [ ] Event streaming

### Total Effort: 7 developer days

## Risk Mitigation

### False Positives
- Implement manual review queue
- Provide detailed explanations
- Allow merchant appeals
- Track false positive metrics

### Performance Impact
- Use caching aggressively
- Implement circuit breakers
- Async processing where possible
- Graceful degradation

### Model Drift
- Continuous monitoring
- A/B testing framework
- Regular retraining
- Feature importance tracking

## Success Metrics

### Business Metrics
- Fraud loss reduction: > 80%
- False positive rate: < 0.1%
- Manual review time: < 5 hours/week
- Platform trust score: > 95%

### Technical Metrics
- API latency: < 5ms p99
- Model accuracy: > 99.9%
- System uptime: > 99.99%
- Cache hit rate: > 90%

## Future Enhancements

### Advanced ML Features
- Deep learning models
- Graph neural networks
- Ensemble methods
- Online learning

### Additional Detection Methods
- Voice biometrics
- Device fingerprinting
- Behavioral biometrics
- Social network analysis

### Integration Expansions
- Third-party fraud services
- Industry blacklists
- Regulatory databases
- Cross-platform sharing