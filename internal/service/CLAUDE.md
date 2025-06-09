# Service Layer Context

## Service Structure
- `analytics/` - Metrics and reporting aggregation
- `bidding/` - Real-time auction orchestration
- `callrouting/` - Routing algorithm implementations
- `fraud/` - ML models and rule engine
- `telephony/` - SIP/WebRTC protocol handling

## Service Layer Principles

### Service Responsibilities
- Orchestrate domain objects and infrastructure
- Implement complex business workflows
- Handle cross-cutting concerns (not business logic)
- Manage transactions across aggregates
- NO validation logic (belongs in domains)

### Dependency Guidelines
- Maximum 5 dependencies per service
- Depend on interfaces, not concrete types
- Use constructor injection
- Mock dependencies in tests

### Current Refactoring Needs

**Analytics Service**
- Currently has 14 methods - violates single responsibility
- Split into: CallAnalytics, BidAnalytics, AccountAnalytics
- Each focused service should have 4-6 methods max

**Bidding Service**
- Remove rate limiting logic (infrastructure concern)
- Move bid validation to Bid domain
- Focus only on auction orchestration

**Telephony Service**
- Add protocol-specific logic beyond pass-through
- Implement codec negotiation
- Handle connection state management

### Testing Services
- Mock all dependencies using interfaces
- Test orchestration logic, not business rules
- Use table-driven tests for different scenarios
- Include integration tests with real infrastructure

## Routing Algorithms

### Available Algorithms
- **Round Robin**: Even distribution across buyers
- **Skill-Based**: Match call attributes to buyer skills
- **Cost-Based**: Optimize for lowest cost routing
- **Geographic**: Route based on proximity

### Algorithm Implementation
- Each algorithm implements `RoutingAlgorithm` interface
- Algorithms are stateless and concurrent-safe
- Return ranked list of potential buyers
- Include scoring/reasoning in results