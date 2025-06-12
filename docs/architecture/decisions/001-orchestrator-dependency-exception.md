# ADR-001: Orchestrator Service Dependency Exception

## Status
Accepted

## Context
Our architecture tests enforce a maximum of 5 dependencies per service to prevent high coupling. However, orchestrator services that coordinate multiple subsystems naturally require more dependencies.

The coordinatorService has 8 dependencies:
1. BidManagementService - Core CRUD operations
2. BidValidationService - Business rule validation  
3. AuctionOrchestrationService - Auction lifecycle
4. RateLimitService - Request throttling
5. AccountRepository - Account data access
6. NotificationService - Async notifications
7. MetricsCollector - Performance monitoring
8. ServiceConfig - Configuration

## Decision
Allow orchestrator services to have up to 8 dependencies when they:
- Have names ending with "OrchestrationService" or "CoordinatorService"
- Coordinate 3+ distinct subsystems
- Use interface dependencies, not concrete implementations

## Consequences
- Orchestrators can properly coordinate complex workflows
- Clear naming convention identifies these special services
- Risk of abuse mitigated by explicit criteria
- Architecture tests will have special handling for these services