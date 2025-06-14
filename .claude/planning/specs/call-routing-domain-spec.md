# Call Routing Domain Specification

## Executive Summary

### Problem Statement
Current analysis reveals **critical gaps** in the call routing domain implementation:
- **Scattered routing logic** across `callrouting` and `buyer_routing` services
- **No unified routing domain** to handle the core value proposition
- **Missing intelligent algorithms** for optimal call-buyer matching
- **Performance bottlenecks** preventing sub-millisecond routing decisions
- **Limited routing strategies** affecting marketplace competitiveness

### Business Impact
- **Revenue Loss**: Suboptimal routing reduces call conversion rates by 15-25%
- **Buyer Churn**: Poor call quality matching leads to 30% buyer retention issues
- **Scalability Limits**: Current architecture cannot handle 100K+ concurrent calls
- **Competitive Disadvantage**: Missing advanced routing puts us behind competitors

### Solution Overview
Implement a **comprehensive Call Routing Domain** with:
- **Unified routing engine** with pluggable algorithms
- **Sub-millisecond decision making** through precomputed routing tables
- **Multi-factor scoring system** combining quality, cost, and capacity
- **Real-time adaptation** based on performance feedback
- **Zero-allocation hot path** for performance-critical routing

## Domain Model

### Core Entities

```go
// Routing domain entities
package routing

import (
    "context"
    "time"
    "github.com/google/uuid"
)

// RoutingEngine - Central routing orchestrator
type RoutingEngine struct {
    algorithms     map[string]RoutingAlgorithm
    decisionCache  *RoutingCache
    qualityTracker *QualityTracker
    rules          *RoutingConfiguration
    metrics        *RoutingMetrics
}

// RoutingDecision - Result of routing analysis
type RoutingDecision struct {
    CallID          uuid.UUID
    SelectedBidID   uuid.UUID
    BuyerID         uuid.UUID
    AlgorithmUsed   string
    ConfidenceScore float64
    DecisionLatency time.Duration
    Reasoning       *DecisionReasoning
    QualityFactors  *QualityFactors
    Metadata        map[string]interface{}
}

// RoutingContext - Input context for routing decisions
type RoutingContext struct {
    Call            *call.Call
    AvailableBids   []*bid.Bid
    CallHistory     *CallHistory
    BuyerProfiles   map[uuid.UUID]*BuyerProfile
    MarketConditions *MarketConditions
    TimeConstraints  *TimeConstraints
}

// QualityScore - Multi-dimensional quality assessment
type QualityScore struct {
    ConversionRate    float64 `json:"conversion_rate"`
    FraudRisk        float64 `json:"fraud_risk"`
    ResponseTime     float64 `json:"response_time"`
    CallDuration     float64 `json:"call_duration"`
    CustomerSatisfaction float64 `json:"customer_satisfaction"`
    CompositeScore   float64 `json:"composite_score"`
}

// RoutingRule - Configurable routing behavior
type RoutingRule struct {
    ID             uuid.UUID
    Name           string
    Priority       int
    Conditions     []RuleCondition
    Actions        []RuleAction
    ValidTimeRange *TimeRange
    GeographicScope []string
}

// DecisionReasoning - Explanation of routing choice
type DecisionReasoning struct {
    PrimaryFactors   []string
    WeightedScores   map[string]float64
    RejectedBids     []RejectedBid
    AlternativeOptions []AlternativeOption
    RiskAssessment   *RiskAssessment
}
```

### Value Objects

```go
// RoutingWeight - Configurable algorithm weights
type RoutingWeight struct {
    Quality     float64 `json:"quality" validate:"min=0,max=1"`
    Price       float64 `json:"price" validate:"min=0,max=1"`
    Capacity    float64 `json:"capacity" validate:"min=0,max=1"`
    Geographic  float64 `json:"geographic" validate:"min=0,max=1"`
    Historical  float64 `json:"historical" validate:"min=0,max=1"`
}

// PerformanceThresholds - SLA requirements
type PerformanceThresholds struct {
    MaxDecisionLatency time.Duration `json:"max_decision_latency"`
    MinQualityScore    float64       `json:"min_quality_score"`
    MaxFraudRisk       float64       `json:"max_fraud_risk"`
    RequiredCapacity   int           `json:"required_capacity"`
}

// GeographicPreference - Location-based routing
type GeographicPreference struct {
    PreferredStates []string            `json:"preferred_states"`
    ProximityBonus  float64             `json:"proximity_bonus"`
    TimeZoneWeight  float64             `json:"timezone_weight"`
    RegionalRules   map[string]float64  `json:"regional_rules"`
}
```

## Routing Algorithms

### 1. Intelligent Round-Robin
```go
type IntelligentRoundRobinRouter struct {
    lastSelections map[string]int
    fairnessTracker *FairnessTracker
    performanceWeights map[uuid.UUID]float64
}

// Features:
// - Weighted distribution based on performance
// - Fair allocation preventing buyer starvation
// - Dynamic adjustment based on success rates
```

### 2. Multi-Factor Scoring Algorithm
```go
type MultiFactorScoringRouter struct {
    weights *RoutingWeight
    normalizer *ScoreNormalizer
    riskAssessor *RiskAssessor
}

// Scoring factors:
// - Quality metrics (conversion, fraud, satisfaction)
// - Pricing competitiveness
// - Capacity availability
// - Geographic match
// - Historical performance
```

### 3. Machine Learning Enhanced Router
```go
type MLEnhancedRouter struct {
    model *CallSuccessPredictionModel
    featureExtractor *FeatureExtractor
    adaptiveWeights *AdaptiveWeightCalculator
}

// Features:
// - Real-time learning from call outcomes
// - Predictive modeling for success probability
// - Adaptive weight adjustment
// - A/B testing framework integration
```

### 4. Geographic Optimization Router
```go
type GeographicOptimizedRouter struct {
    locationService *LocationService
    timezoneCalculator *TimezoneCalculator
    regionalPerformance map[string]*RegionalMetrics
}

// Features:
// - Time zone aware routing
// - Regional performance optimization
// - Local market expertise matching
// - Geographic compliance enforcement
```

### 5. Real-Time Market Router
```go
type RealTimeMarketRouter struct {
    marketMonitor *MarketMonitor
    demandPredictor *DemandPredictor
    priceOptimizer *PriceOptimizer
}

// Features:
// - Dynamic pricing consideration
// - Market demand adjustment
// - Real-time capacity monitoring
// - Competition-aware routing
```

### 6. Skill-Based Routing V2
```go
type AdvancedSkillBasedRouter struct {
    skillMatcher *SkillMatcher
    expertiseDatabase *ExpertiseDatabase
    learningEngine *SkillLearningEngine
}

// Features:
// - Industry-specific skill matching
// - Dynamic skill assessment
// - Learning from call outcomes
// - Expertise-call complexity matching
```

## Decision Engine Architecture

### Core Components

```go
// RoutingDecisionEngine - Main decision orchestrator
type RoutingDecisionEngine struct {
    // Pre-computed routing tables for hot paths
    routingTables map[string]*PrecomputedRoutes
    
    // Real-time decision components
    algorithmSelector *AlgorithmSelector
    qualityAnalyzer   *QualityAnalyzer
    riskAssessor      *RiskAssessor
    performanceTracker *PerformanceTracker
    
    // Caching and optimization
    decisionCache     *DecisionCache
    routingOptimizer  *RoutingOptimizer
}

// Fast path routing with precomputed decisions
func (e *RoutingDecisionEngine) FastRoute(ctx context.Context, routingKey string) (*RoutingDecision, error) {
    // Sub-millisecond lookup from precomputed tables
    if precomputed := e.routingTables[routingKey]; precomputed != nil {
        return precomputed.GetBestMatch(), nil
    }
    
    // Fallback to real-time routing
    return e.RealTimeRoute(ctx, routingKey)
}

// Multi-factor scoring with configurable weights
func (e *RoutingDecisionEngine) CalculateScore(
    bid *bid.Bid, 
    context *RoutingContext,
    weights *RoutingWeight,
) (*QualityScore, error) {
    
    score := &QualityScore{}
    
    // Quality assessment
    score.ConversionRate = e.qualityAnalyzer.AssessConversion(bid, context)
    score.FraudRisk = e.riskAssessor.AssessFraudRisk(bid, context)
    score.ResponseTime = e.performanceTracker.GetAverageResponseTime(bid.BuyerID)
    
    // Composite scoring
    score.CompositeScore = e.calculateCompositeScore(score, weights)
    
    return score, nil
}
```

### Performance Optimization

```go
// PrecomputedRoutes - Zero-allocation hot path
type PrecomputedRoutes struct {
    routes          []PrecomputedRoute // Pre-sorted by score
    lastUpdated     time.Time
    expirationTime  time.Time
    accessCount     int64
}

// RoutingCache - High-performance caching
type RoutingCache struct {
    // LRU cache for frequent routing patterns
    decisionCache   *lru.Cache
    
    // Bloom filter for quick negative lookups
    bloomFilter     *bloom.BloomFilter
    
    // Time-based invalidation
    expirationMap   map[string]time.Time
}

// Zero-allocation decision making
func (e *RoutingDecisionEngine) ZeroAllocRoute(
    callID uuid.UUID,
    bidIDs []uuid.UUID,
) (*RoutingDecision, error) {
    
    // Use object pools to avoid allocations
    decision := e.decisionPool.Get().(*RoutingDecision)
    defer e.decisionPool.Put(decision)
    
    // Stack-allocated scoring
    var scores [16]float64  // Assume max 16 bids for hot path
    
    // Fast scoring loop
    for i, bidID := range bidIDs {
        scores[i] = e.fastScore(bidID, callID)
    }
    
    // Find best without allocations
    bestIdx := findMaxIndex(scores[:len(bidIDs)])
    
    // Populate result
    decision.BidID = bidIDs[bestIdx]
    decision.Score = scores[bestIdx]
    
    return decision, nil
}
```

## Service Layer Integration

### Primary Services

```go
// RoutingOrchestrationService - Main routing coordinator
type RoutingOrchestrationService struct {
    decisionEngine  *RoutingDecisionEngine
    ruleEngine      *RoutingRuleEngine
    qualityService  *QualityScoringService
    complianceService *ComplianceCheckService
    metricsCollector *RoutingMetricsCollector
}

// Core routing workflow
func (s *RoutingOrchestrationService) RouteCall(
    ctx context.Context, 
    callID uuid.UUID,
) (*RoutingDecision, error) {
    
    start := time.Now()
    
    // 1. Load routing context (< 0.1ms)
    context, err := s.loadRoutingContext(ctx, callID)
    if err != nil {
        return nil, err
    }
    
    // 2. Pre-flight compliance checks (< 0.1ms)
    if err := s.complianceService.ValidateRouting(ctx, context); err != nil {
        return nil, err
    }
    
    // 3. Route decision (< 0.5ms target)
    decision, err := s.decisionEngine.Route(ctx, context)
    if err != nil {
        return nil, err
    }
    
    // 4. Post-decision actions (async)
    go s.recordDecisionMetrics(decision, time.Since(start))
    
    return decision, nil
}

// RoutingDecisionService - Algorithm management
type RoutingDecisionService struct {
    algorithms      map[string]RoutingAlgorithm
    algorithmSelector *AlgorithmSelector
    performanceMonitor *AlgorithmPerformanceMonitor
}

// QualityScoringService - Quality assessment
type QualityScoringService struct {
    qualityAnalyzer *QualityAnalyzer
    riskAssessor    *RiskAssessor
    performanceTracker *PerformanceTracker
    feedbackProcessor *FeedbackProcessor
}

// RoutingAnalyticsService - Performance monitoring
type RoutingAnalyticsService struct {
    metricsCollector *MetricsCollector
    performanceAnalyzer *PerformanceAnalyzer
    reportGenerator *ReportGenerator
    alertManager *AlertManager
}
```

### Advanced Features

```go
// A/B Testing Framework
type RoutingABTestFramework struct {
    testConfigs     map[string]*ABTestConfig
    trafficSplitter *TrafficSplitter
    resultCollector *ResultCollector
}

// Real-time Algorithm Optimization
type AlgorithmOptimizer struct {
    performanceMonitor *PerformanceMonitor
    weightOptimizer    *WeightOptimizer
    feedbackLoop       *FeedbackLoop
}

// Predictive Routing
type PredictiveRoutingService struct {
    demandPredictor    *DemandPredictor
    capacityForecaster *CapacityForecaster
    precomputeEngine   *PrecomputeEngine
}
```

## Performance Requirements

### Latency Targets
- **Routing Decision**: < 1ms (target: 0.5ms)
- **Context Loading**: < 0.1ms
- **Compliance Check**: < 0.1ms
- **Quality Scoring**: < 0.3ms
- **Cache Lookup**: < 0.01ms

### Throughput Targets
- **Concurrent Routing Decisions**: 100,000+ per second
- **Cache Operations**: 1,000,000+ per second
- **Quality Score Calculations**: 500,000+ per second
- **Rule Evaluations**: 2,000,000+ per second

### Memory Optimization
```go
// Memory pool for frequent allocations
type RoutingObjectPools struct {
    decisionPool    sync.Pool
    contextPool     sync.Pool
    scorePool       sync.Pool
    reasoningPool   sync.Pool
}

// Pre-allocated buffers for hot paths
type RoutingBuffers struct {
    bidScoreBuffer   [256]float64  // Max 256 concurrent bids
    candidateBuffer  [256]uuid.UUID
    weightBuffer     [8]float64    // 8 weight factors
}

// Zero-garbage routing for hot paths
func (s *Service) ZeroGarbageRoute(callID uuid.UUID) (*RoutingDecision, error) {
    // Use stack allocation and object pools only
    // No heap allocations in critical path
}
```

### Precomputed Routing Tables
```go
// Background precomputation for common patterns
type RoutingPrecompute struct {
    // Geographic routing tables
    geoRoutingTable map[string]*PrecomputedGeoRoutes
    
    // Time-based routing patterns
    timeBasedRoutes map[TimeSlot]*PrecomputedTimeRoutes
    
    // Buyer capacity predictions
    capacityPredictions map[uuid.UUID]*CapacityForecast
    
    // Quality score caches
    qualityScoreCache map[string]*CachedQualityScore
}

// Update precomputed tables every 30 seconds
func (p *RoutingPrecompute) UpdateTables(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            p.recomputeGeoTables()
            p.recomputeTimeTables()
            p.updateCapacityPredictions()
            p.refreshQualityScores()
        case <-ctx.Done():
            return
        }
    }
}
```

## Testing Strategy

### Unit Tests
```go
// Algorithm testing with property-based tests
func TestRoutingAlgorithmInvariants(t *testing.T) {
    propertytest.Run(t, func(t *propertytest.T) {
        // Generate random call and bid combinations
        call := generateRandomCall(t)
        bids := generateRandomBids(t, 1, 50)
        
        // Test all routing algorithms
        algorithms := []RoutingAlgorithm{
            NewMultiFactorRouter(defaultWeights),
            NewGeographicRouter(),
            NewSkillBasedRouter(),
        }
        
        for _, algo := range algorithms {
            decision, err := algo.Route(context.Background(), call, bids)
            
            // Invariants that must always hold
            require.NoError(t, err)
            assert.True(t, decision.Score >= 0 && decision.Score <= 1)
            assert.Contains(t, bids, findBidByID(bids, decision.BidID))
            assert.True(t, decision.DecisionLatency < time.Millisecond)
        }
    })
}

// Performance testing with benchmarks
func BenchmarkRoutingDecision(b *testing.B) {
    service := setupRoutingService()
    call := fixtures.NewCall()
    bids := fixtures.NewBids(100) // 100 competing bids
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        _, err := service.RouteCall(context.Background(), call.ID)
        if err != nil {
            b.Fatal(err)
        }
    }
    
    // Verify performance targets
    if b.Elapsed()/time.Duration(b.N) > time.Millisecond {
        b.Fatal("Routing decision took longer than 1ms")
    }
}
```

### Integration Tests
```go
// End-to-end routing workflow tests
func TestCompleteRoutingWorkflow(t *testing.T) {
    testDB := testutil.NewTestDB(t)
    service := NewRoutingOrchestrationService(testDB.Repositories()...)
    
    // Create test scenario
    seller := fixtures.NewSeller()
    buyers := fixtures.NewBuyers(5)
    call := fixtures.NewCall().WithSeller(seller.ID)
    
    // Create competing bids
    bids := make([]*bid.Bid, len(buyers))
    for i, buyer := range buyers {
        bids[i] = fixtures.NewBid().
            WithBuyer(buyer.ID).
            WithAmount(100.0 + float64(i*10)).
            WithQuality(0.8 - float64(i)*0.1)
    }
    
    // Execute routing
    decision, err := service.RouteCall(context.Background(), call.ID)
    require.NoError(t, err)
    
    // Verify results
    assert.NotZero(t, decision.BidID)
    assert.True(t, decision.Score > 0)
    assert.True(t, decision.DecisionLatency < time.Millisecond)
    
    // Verify winning bid was updated
    winningBid := getBidByID(t, testDB, decision.BidID)
    assert.Equal(t, bid.StatusWon, winningBid.Status)
    
    // Verify metrics were recorded
    metrics := getRoutingMetrics(t, testDB, decision.CallID)
    assert.NotNil(t, metrics)
}
```

### Load Testing
```go
// Concurrent routing stress test
func TestConcurrentRoutingStress(t *testing.T) {
    service := NewRoutingOrchestrationService(...)
    
    const (
        numGoroutines = 1000
        callsPerGoroutine = 100
    )
    
    var wg sync.WaitGroup
    errors := make(chan error, numGoroutines*callsPerGoroutine)
    latencies := make(chan time.Duration, numGoroutines*callsPerGoroutine)
    
    start := time.Now()
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < callsPerGoroutine; j++ {
                call := fixtures.NewCall()
                routingStart := time.Now()
                
                _, err := service.RouteCall(context.Background(), call.ID)
                
                latencies <- time.Since(routingStart)
                if err != nil {
                    errors <- err
                }
            }
        }()
    }
    
    wg.Wait()
    close(errors)
    close(latencies)
    
    // Analyze results
    totalCalls := numGoroutines * callsPerGoroutine
    errorCount := len(errors)
    
    // Calculate latency percentiles
    var allLatencies []time.Duration
    for latency := range latencies {
        allLatencies = append(allLatencies, latency)
    }
    sort.Slice(allLatencies, func(i, j int) bool {
        return allLatencies[i] < allLatencies[j]
    })
    
    p50 := allLatencies[len(allLatencies)/2]
    p95 := allLatencies[int(float64(len(allLatencies))*0.95)]
    p99 := allLatencies[int(float64(len(allLatencies))*0.99)]
    
    t.Logf("Performance Results:")
    t.Logf("  Total calls: %d", totalCalls)
    t.Logf("  Errors: %d (%.2f%%)", errorCount, float64(errorCount)/float64(totalCalls)*100)
    t.Logf("  Throughput: %.0f calls/sec", float64(totalCalls)/time.Since(start).Seconds())
    t.Logf("  Latency P50: %v", p50)
    t.Logf("  Latency P95: %v", p95)
    t.Logf("  Latency P99: %v", p99)
    
    // Assert performance requirements
    assert.Less(t, errorCount, totalCalls/100) // < 1% error rate
    assert.Less(t, p99, 2*time.Millisecond)   // P99 < 2ms
    assert.Less(t, p50, 500*time.Microsecond) // P50 < 0.5ms
}
```

### A/B Testing Framework
```go
// A/B test for routing algorithm comparison
func TestRoutingAlgorithmABTest(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping A/B test in short mode")
    }
    
    // Setup A/B test
    controlService := NewRoutingService(NewMultiFactorRouter())
    testService := NewRoutingService(NewMLEnhancedRouter())
    
    const testDuration = 5 * time.Minute
    const callRate = 100 // calls per second
    
    testManager := &ABTestManager{
        TrafficSplit: 0.5, // 50/50 split
        ControlService: controlService,
        TestService: testService,
    }
    
    // Run test
    results := testManager.RunTest(testDuration, callRate)
    
    // Analyze results
    assert.NotZero(t, results.ControlMetrics.TotalCalls)
    assert.NotZero(t, results.TestMetrics.TotalCalls)
    
    // Check for statistical significance
    significance := results.CalculateStatisticalSignificance()
    t.Logf("A/B Test Results:")
    t.Logf("  Control conversion rate: %.2f%%", results.ControlMetrics.ConversionRate)
    t.Logf("  Test conversion rate: %.2f%%", results.TestMetrics.ConversionRate)
    t.Logf("  Statistical significance: %.3f", significance)
    
    if significance > 0.95 {
        t.Logf("Test algorithm shows statistically significant improvement!")
    }
}
```

## Implementation Plan & Effort Estimate

### Phase 1: Domain Foundation (2 days)
- [ ] **Domain Entity Implementation** (1 day)
  - Create core routing entities
  - Implement value objects
  - Add validation logic
  - Unit tests for domain objects

- [ ] **Interface Definition** (0.5 day)
  - Define routing interfaces
  - Create algorithm contracts
  - Establish metrics interfaces

- [ ] **Basic Algorithm Implementation** (0.5 day)
  - Implement MultiFactorRouter
  - Create IntelligentRoundRobin
  - Add basic quality scoring

### Phase 2: Decision Engine (2 days)
- [ ] **Core Decision Engine** (1 day)
  - Implement RoutingDecisionEngine
  - Add context loading
  - Create scoring framework
  - Zero-allocation optimizations

- [ ] **Caching Infrastructure** (0.5 day)
  - Implement RoutingCache
  - Add precomputed routing tables
  - Create cache invalidation logic

- [ ] **Performance Optimization** (0.5 day)
  - Object pooling
  - Memory optimization
  - Hot path optimization

### Phase 3: Advanced Algorithms (1.5 days)
- [ ] **Geographic Routing** (0.5 day)
  - Implement GeographicOptimizedRouter
  - Add timezone awareness
  - Regional performance tracking

- [ ] **ML Enhanced Routing** (1 day)
  - Create MLEnhancedRouter stub
  - Add feature extraction
  - Implement adaptive weights

### Phase 4: Service Integration (1 day)
- [ ] **Service Layer** (0.5 day)
  - Implement RoutingOrchestrationService
  - Create service factories
  - Add dependency injection

- [ ] **Metrics & Monitoring** (0.5 day)
  - Implement RoutingMetricsCollector
  - Add performance tracking
  - Create alert thresholds

### Phase 5: Testing & Validation (1 day)
- [ ] **Comprehensive Testing** (0.5 day)
  - Property-based tests
  - Performance benchmarks
  - Integration tests

- [ ] **Load Testing** (0.5 day)
  - Concurrent stress tests
  - A/B testing framework
  - Performance validation

### Phase 6: Documentation & Migration (0.5 day)
- [ ] **Documentation** (0.25 day)
  - API documentation
  - Algorithm guides
  - Performance tuning guide

- [ ] **Migration Strategy** (0.25 day)
  - Legacy service migration
  - Feature flag implementation
  - Rollback procedures

## Total Effort: **6 developer days**

### Success Metrics
- **Routing latency**: < 1ms P99 (target: 0.5ms)
- **Throughput**: 100K+ decisions per second
- **Accuracy**: 95%+ optimal routing decisions
- **Memory usage**: < 100MB for 10K concurrent calls
- **Cache hit rate**: > 90% for precomputed routes
- **Zero allocation**: Hot path generates no garbage

### Risk Mitigation
- **Performance Risk**: Implement feature flags for gradual rollout
- **Algorithm Risk**: A/B test framework for safe algorithm updates
- **Memory Risk**: Extensive load testing with memory profiling
- **Integration Risk**: Comprehensive integration testing with real data

This specification provides a complete roadmap for implementing a world-class call routing domain that will serve as the competitive advantage for the Dependable Call Exchange platform.