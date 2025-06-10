# Buyer/Seller Naming Clarification Refactoring Report

## Executive Summary

The current codebase has ambiguous naming conventions that don't clearly distinguish between buyer and seller operations, particularly in call routing and bidding contexts. This report outlines the necessary changes to improve code clarity and prevent confusion.

## Current Issues

### 1. Ambiguous Service Names
- `callrouting` service - Unclear whether this routes calls TO sellers or FROM sellers to buyers
- `bidding` service - Doesn't specify that only buyers place bids

### 2. Confusing Domain Relationships
- Calls have both `BuyerID` and `SellerID` fields, but their usage context is unclear
- Bids have `BuyerID` and `SellerID`, but the relationship semantics are not obvious

### 3. Test Data Confusion
- Tests create "suspended seller accounts" but use them as buyers in bids
- Foreign key violations due to buyer/seller confusion

## Proposed Changes

### 1. Service Layer Refactoring

#### Current Structure
```
internal/service/
├── callrouting/     # Ambiguous
├── bidding/         # Unclear who bids
└── analytics/       # Generic
```

#### Proposed Structure
```
internal/service/
├── buyer_routing/           # Routes calls from sellers TO buyers
│   ├── service.go          # Main routing logic for buyer acquisition
│   ├── algorithms.go       # Buyer selection algorithms
│   └── interfaces.go       # Clear buyer-specific interfaces
├── seller_distribution/     # Routes incoming calls TO sellers
│   ├── service.go          # Distribution logic for sellers
│   ├── load_balancer.go    # Seller capacity management
│   └── interfaces.go       # Seller-specific interfaces
├── buyer_bidding/          # Buyers bid on seller calls
│   ├── auction.go          # Auction logic for buyers
│   ├── service.go          # Bid management for buyers
│   └── interfaces.go       # Buyer bidding interfaces
└── marketplace_analytics/   # Analytics for the call marketplace
```

### 2. Domain Model Clarifications

#### Call Entity
```go
type Call struct {
    ID          uuid.UUID
    FromNumber  string
    ToNumber    string
    Direction   Direction    
    Status      Status
    
    // Clarified fields with comments
    SellerID    uuid.UUID   // The seller who owns/generated this call
    BuyerID     *uuid.UUID  // The buyer who wins the bid (nullable until routed)
    
    // Alternative naming for clarity
    OwnerID     uuid.UUID   // The seller who owns this call
    WinnerID    *uuid.UUID  // The winning buyer (after routing)
}
```

#### Bid Entity
```go
type Bid struct {
    ID       uuid.UUID
    CallID   uuid.UUID
    
    // Current (confusing)
    BuyerID  uuid.UUID   // Who is bidding
    SellerID uuid.UUID   // Redundant with Call.SellerID
    
    // Proposed (clear)
    BidderID uuid.UUID   // The buyer placing the bid
    // Remove SellerID - get from Call entity
}
```

### 3. Repository Interface Updates

#### Current
```go
type CallRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*Call, error)
    Update(ctx context.Context, call *Call) error
    // Generic, no buyer/seller context
}
```

#### Proposed
```go
type CallRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*Call, error)
    GetSellerCalls(ctx context.Context, sellerID uuid.UUID, filter CallFilter) ([]*Call, error)
    GetBuyerWonCalls(ctx context.Context, buyerID uuid.UUID, filter CallFilter) ([]*Call, error)
    AssignCallToBuyer(ctx context.Context, callID, buyerID uuid.UUID) error
    Update(ctx context.Context, call *Call) error
}
```

### 4. Service Method Naming

#### Current (Ambiguous)
```go
// callrouting service
func (s *service) RouteCall(ctx context.Context, callID uuid.UUID) (*RoutingDecision, error)
```

#### Proposed (Clear)
```go
// buyer_routing service
func (s *buyerRoutingService) RouteCallToBuyer(ctx context.Context, callID uuid.UUID) (*BuyerRoutingDecision, error)

// seller_distribution service  
func (s *sellerDistributionService) DistributeCallToSeller(ctx context.Context, call *IncomingCall) (*SellerAssignment, error)
```

### 5. Test Data Builders

#### Current Issues
```go
// Confusing test data
suspendedSeller := fixtures.NewAccountBuilder(testDB).
    WithType(account.TypeSeller).
    WithStatus(account.StatusSuspended).
    Build(t)

// Then used as buyer!
fixtures.NewBidBuilder(testDB).
    WithBuyerID(suspendedSeller.ID)  // Wrong!
```

#### Proposed Fix
```go
// Clear test data creation
suspendedBuyer := fixtures.NewAccountBuilder(testDB).
    WithType(account.TypeBuyer).
    WithStatus(account.StatusSuspended).
    Build(t)

activeSeller := fixtures.NewAccountBuilder(testDB).
    WithType(account.TypeSeller).
    Build(t)

// Create call from seller
sellerCall := fixtures.NewCallBuilder(t).
    WithSellerID(activeSeller.ID).
    Build()

// Create bid from buyer
buyerBid := fixtures.NewBidBuilder(testDB).
    WithCallID(sellerCall.ID).
    WithBidderID(suspendedBuyer.ID).
    Build(t)
```

### 6. Configuration and Rules

#### Current
```go
type RoutingRules struct {
    Algorithm string
    // No context of buyer vs seller routing
}
```

#### Proposed
```go
type BuyerRoutingRules struct {
    Algorithm      string  // round-robin, highest-bid, quality-based
    MinBidAmount   float64
    QualityWeight  float64
}

type SellerDistributionRules struct {
    Algorithm     string  // least-loaded, skill-based, geographic
    MaxConcurrent int
    SkillMatching bool
}
```

## Implementation Plan

### Phase 1: Domain Model Clarification (1-2 days)
1. Add clear comments to existing fields
2. Create migration to rename ambiguous columns
3. Update repository interfaces with clearer method names

### Phase 2: Service Layer Refactoring (3-4 days)
1. Create new service packages with clear names
2. Move existing logic to appropriate services
3. Update all imports and dependencies
4. Ensure backward compatibility with deprecation notices

### Phase 3: Test Suite Updates (2-3 days)
1. Fix all buyer/seller confusion in tests
2. Create separate test scenarios for buyer and seller flows
3. Add integration tests for both routing directions

### Phase 4: Documentation (1 day)
1. Update API documentation
2. Create clear diagrams showing buyer/seller flows
3. Update code comments and examples

## Migration Strategy

To minimize disruption:

1. **Parallel Implementation**: Create new services alongside old ones
2. **Deprecation Notices**: Mark old services as deprecated
3. **Gradual Migration**: Update consumers one at a time
4. **Feature Flags**: Use flags to switch between old/new implementations
5. **Backward Compatibility**: Maintain old APIs for transition period

## Expected Benefits

1. **Reduced Confusion**: Clear distinction between buyer and seller operations
2. **Fewer Bugs**: Eliminate buyer/seller mix-ups in business logic
3. **Better Testing**: Clearer test scenarios and data
4. **Easier Onboarding**: New developers understand the system faster
5. **Maintainability**: Clear boundaries between different routing systems

## Risks and Mitigation

1. **Risk**: Breaking existing integrations
   - **Mitigation**: Maintain backward compatibility layer

2. **Risk**: Database migration issues
   - **Mitigation**: Use additive changes, avoid breaking schema changes

3. **Risk**: Performance regression
   - **Mitigation**: Benchmark before/after, optimize critical paths

## Conclusion

The current naming conventions create significant confusion about the direction of call flows and the roles of buyers vs sellers. This refactoring will create clear boundaries and naming that accurately reflects the business domain, reducing bugs and improving developer productivity.