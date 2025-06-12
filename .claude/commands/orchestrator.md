# **INTELLIGENT DCE FEATURE DEVELOPMENT ORCHESTRATOR**

Ultra-think about comprehensive feature development for the Dependable Call Exchange platform. You will orchestrate the creation of complete features across all architectural layers by coordinating specialized Go development agents that understand DDD principles, clean architecture, and the specific patterns used in this high-performance telephony system.

## **COMMAND VARIABLES**

```
feature_spec: $ARGUMENTS       # Feature specification file or description
feature_type: $ARGUMENTS       # entity|service|api|integration|compliance
target_domain: $ARGUMENTS      # account|bid|call|compliance|financial
generation_mode: $ARGUMENTS    # scaffold|complete|test-only|full-stack
```

## **ARGUMENT PARSING**

Parse from "$ARGUMENTS":
1. `feature_spec` - Path to feature specification or inline description
   Examples: "specs/buyer-preferences.md", "Add call recording consent management"
   
2. `feature_type` - Type of feature to generate
   Options: entity, service, api, integration, compliance, full-stack
   
3. `target_domain` - Target domain context (default: auto-detect)
   Options: account, bid, call, compliance, financial, marketplace
   
4. `generation_mode` - How much to generate
   Options: scaffold (structure only), complete (full implementation), test-only, full-stack (all layers)

## **PHASE 1: DCE ARCHITECTURE ANALYSIS**

**Deep Project Understanding Protocol:**

1. Analyze the feature specification in context of:
   - Existing domain boundaries and ubiquitous language
   - Current service capabilities and integration points
   - Performance requirements (sub-millisecond routing, 100K+ bids/sec)
   - Compliance constraints (TCPA, GDPR, DNC)
   - Security requirements (fraud detection, JWT auth)

2. Map feature to DCE's architecture:
   ```
   Domain Layer → Which bounded context?
   Service Layer → Which orchestration services?
   API Layer → REST, gRPC, or WebSocket?
   Infrastructure → Database, cache, messaging needs?
   ```

3. Identify cross-cutting concerns:
   - Telemetry and observability requirements
   - Rate limiting and performance constraints
   - Security and compliance validations
   - Integration with existing telephony systems

## **PHASE 2: DOMAIN MODELING**

**DDD-Driven Entity Design:**

Analyze and design domain components following DCE patterns:

```go
// Pattern Recognition Checklist
domain_patterns:
  entities:
    - Constructor with validation
    - Value objects for type safety
    - Domain events for state changes
    - Business invariant enforcement
    
  value_objects:
    - Immutable design
    - Self-validation
    - Type safety (Money, PhoneNumber, Email)
    - Comparison and equality methods
    
  aggregates:
    - Clear aggregate boundaries
    - Consistency enforcement
    - Event sourcing readiness
    
  domain_services:
    - Complex business logic
    - Cross-aggregate operations
    - External service abstractions
```

## **PHASE 3: INTELLIGENT TASK DECOMPOSITION**

**Feature Implementation Strategy:**

Based on feature type and domain, generate tasks:

```yaml
entity_feature_tasks:
  - Create domain entity with validation
  - Design value objects for type safety
  - Define repository interface
  - Implement repository with transactions
  - Create domain events
  - Add factory methods
  - Generate comprehensive tests

service_feature_tasks:
  - Define service interface
  - Implement service with dependency injection
  - Add transaction management
  - Integrate with existing services
  - Implement caching strategy
  - Add telemetry and metrics
  - Create service tests

api_feature_tasks:
  - Create REST handlers
  - Define request/response DTOs
  - Add validation middleware
  - Implement rate limiting
  - Generate OpenAPI specs
  - Create integration tests
  - Add security checks

compliance_feature_tasks:
  - Implement compliance rules
  - Add audit logging
  - Create validation services
  - Update consent management
  - Add regulatory reporting
  - Test edge cases
  - Document compliance
```

## **PHASE 4: SPECIALIZED AGENT DEPLOYMENT**

**DCE-Specific Agent Profiles:**

**DomainExpert Agent:**
- **Expertise**: DDD, Go idioms, DCE domain language
- **Focus**: Entity design, value objects, business invariants
- **Knowledge**: 
  - DCE's existing patterns (Money, PhoneNumber types)
  - Domain event patterns
  - Aggregate design principles
- **Output**: Domain entities, value objects, domain services

**ServiceArchitect Agent:**
- **Expertise**: Clean architecture, dependency injection, Go interfaces
- **Focus**: Service orchestration, transaction management, performance
- **Knowledge**:
  - DCE's service factory pattern
  - Interface-based design
  - Context propagation patterns
- **Output**: Service interfaces, implementations, factories

**APIDesigner Agent:**
- **Expertise**: REST/gRPC/WebSocket, OpenAPI, Go HTTP patterns
- **Focus**: API consistency, validation, security
- **Knowledge**:
  - DCE's handler patterns
  - Middleware chain design
  - Contract testing approach
- **Output**: Handlers, DTOs, middleware, API specs

**RepositoryBuilder Agent:**
- **Expertise**: PostgreSQL, transactions, Go SQL patterns
- **Focus**: Data persistence, query optimization, migrations
- **Knowledge**:
  - DCE's repository patterns
  - Transaction handling
  - Migration strategies
- **Output**: Repository implementations, migrations, indexes

**TestEngineer Agent:**
- **Expertise**: Go testing, testcontainers, property-based testing
- **Focus**: Comprehensive test coverage, edge cases, performance
- **Knowledge**:
  - DCE's test patterns
  - Integration test setup
  - Benchmark patterns
- **Output**: Unit tests, integration tests, benchmarks

**ComplianceGuardian Agent:**
- **Expertise**: TCPA, GDPR, DNC rules, audit trails
- **Focus**: Regulatory compliance, consent management, data retention
- **Knowledge**:
  - Telephony regulations
  - DCE's compliance framework
  - Audit requirements
- **Output**: Compliance validations, audit logs, consent flows

**PerformanceOptimizer Agent:**
- **Expertise**: Go profiling, caching, database optimization
- **Focus**: Sub-millisecond latency, high throughput, resource efficiency
- **Knowledge**:
  - DCE's performance targets
  - Redis caching patterns
  - Query optimization
- **Output**: Optimized code, caching strategies, benchmarks

**SecuritySentinel Agent:**
- **Expertise**: JWT auth, RBAC, input validation, fraud detection
- **Focus**: Security hardening, vulnerability prevention, access control
- **Knowledge**:
  - DCE's security model
  - Common telephony fraud patterns
  - OWASP best practices
- **Output**: Security validations, auth checks, fraud rules

## **PHASE 5: COORDINATED IMPLEMENTATION**

**Multi-Layer Implementation Protocol:**

```python
def implement_dce_feature(feature_spec):
    # Stage 1: Domain Layer
    domain_tasks = decompose_domain_requirements(feature_spec)
    domain_agents = [
        DomainExpert(task) for task in domain_tasks
    ]
    
    # Stage 2: Repository Layer
    repo_tasks = generate_repository_tasks(domain_results)
    repo_agents = [
        RepositoryBuilder(task) for task in repo_tasks
    ]
    
    # Stage 3: Service Layer
    service_tasks = design_service_orchestration(feature_spec)
    service_agents = [
        ServiceArchitect(task) for task in service_tasks
    ]
    
    # Stage 4: API Layer
    api_tasks = create_api_tasks(service_interfaces)
    api_agents = [
        APIDesigner(task) for task in api_tasks
    ]
    
    # Stage 5: Testing Layer
    test_tasks = comprehensive_test_planning(all_components)
    test_agents = [
        TestEngineer(task) for task in test_tasks
    ]
    
    # Stage 6: Cross-Cutting Concerns
    quality_agents = [
        ComplianceGuardian(compliance_requirements),
        PerformanceOptimizer(performance_targets),
        SecuritySentinel(security_requirements)
    ]
```

**Agent Coordination Channels:**

```yaml
coordination_protocol:
  domain_channel:
    purpose: "Share entity designs and domain events"
    messages:
      - "Created Call.RecordingConsent value object"
      - "Added ConsentGranted domain event"
      - "Defined RecordingConsentRepository interface"
  
  api_contract_channel:
    purpose: "Coordinate API contracts across services"
    messages:
      - "POST /api/v1/calls/{id}/recording-consent"
      - "Response: 201 with consent object"
      - "Rate limit: 100 req/sec per buyer"
  
  performance_channel:
    purpose: "Share performance requirements and optimizations"
    messages:
      - "Consent check must be < 1ms"
      - "Cache consent for 24 hours"
      - "Index on (call_id, buyer_id) for fast lookup"
  
  compliance_channel:
    purpose: "Ensure regulatory requirements are met"
    messages:
      - "Two-party consent required for CA calls"
      - "Audit trail required for all consent changes"
      - "GDPR: consent must be revocable"
```

## **PHASE 6: DCE-SPECIFIC QUALITY GATES**

**Multi-Dimensional Quality Framework:**

```yaml
quality_dimensions:
  functional_quality:
    - Domain invariants enforced
    - Service contracts honored
    - API specifications met
    - Integration points tested
    
  performance_quality:
    - Routing decision < 1ms
    - Bid processing > 100K/sec
    - API response < 50ms p99
    - Database queries < 5ms
    
  security_quality:
    - JWT validation on all endpoints
    - RBAC permissions enforced
    - Input validation complete
    - SQL injection prevention
    
  compliance_quality:
    - TCPA rules implemented
    - GDPR requirements met
    - Audit trail complete
    - Consent management working
    
  operational_quality:
    - Prometheus metrics exposed
    - Structured logging implemented
    - Health checks added
    - Graceful shutdown supported
    
  code_quality:
    - golangci-lint passing
    - Test coverage > 80%
    - No security vulnerabilities
    - Documentation complete
```

**Progressive Validation Gates:**

```bash
# Gate 1: Domain Validation
- Valid Go code compiles
- Domain tests pass
- Business rules enforced

# Gate 2: Integration Validation  
- Repository tests pass
- Service integration works
- Database migrations run

# Gate 3: API Validation
- Handler tests pass
- OpenAPI spec valid
- Contract tests succeed

# Gate 4: Performance Validation
- Benchmarks meet targets
- Load tests pass
- No performance regression

# Gate 5: Security & Compliance
- Security scan clean
- Compliance checks pass
- Audit trail verified
```

## **PHASE 7: CONTINUOUS GENERATION MODE**

**Infinite Feature Development:**

For continuous feature generation:

```yaml
wave_strategy:
  wave_1_foundation:
    agents: [DomainExpert, RepositoryBuilder]
    focus: "Core domain models and persistence"
    duration: "Until domain complete"
    
  wave_2_orchestration:
    agents: [ServiceArchitect, TestEngineer]
    focus: "Service layer and basic tests"
    duration: "Until services tested"
    
  wave_3_exposure:
    agents: [APIDesigner, SecuritySentinel]
    focus: "API endpoints and security"
    duration: "Until APIs secured"
    
  wave_4_optimization:
    agents: [PerformanceOptimizer, ComplianceGuardian]
    focus: "Performance tuning and compliance"
    duration: "Until targets met"
    
  wave_5_enhancement:
    agents: [All specialist agents]
    focus: "Advanced features and edge cases"
    duration: "Until context exhausted"
```

## **EXECUTION PRINCIPLES**

**DCE Development Standards:**

1. **Domain-First Design**
   - Start with domain models and business logic
   - Enforce invariants at the domain layer
   - Use value objects for type safety

2. **Performance by Design**
   - Consider latency in every decision
   - Use caching strategically
   - Optimize database queries early

3. **Compliance Integration**
   - Build compliance into the domain
   - Audit everything that matters
   - Make consent explicit

4. **Test-Driven Development**
   - Write tests alongside implementation
   - Use table-driven tests for edge cases
   - Benchmark performance-critical code

5. **Operational Excellence**
   - Instrument everything with metrics
   - Use structured logging throughout
   - Design for graceful degradation

## **DCE CODE GENERATION TEMPLATES**

**Domain Entity Template:**
```go
// internal/domain/{domain}/{entity}.go
package {domain}

import (
    "time"
    "github.com/google/uuid"
    "github.com/davidleathers113/dce-backend/internal/domain/values"
    "github.com/davidleathers113/dce-backend/internal/errors"
)

type {Entity} struct {
    ID          uuid.UUID
    // Add fields with value objects
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

func New{Entity}(/* parameters */) (*{Entity}, error) {
    // Validation and construction
    return &{Entity}{
        ID:        uuid.New(),
        CreatedAt: time.Now(),
    }, nil
}

// Domain methods with business logic
```

**Service Interface Template:**
```go
// internal/service/{service}/service.go
package {service}

import (
    "context"
    "github.com/davidleathers113/dce-backend/internal/domain/{domain}"
)

type Service interface {
    // Define service methods
}

type service struct {
    repo           {domain}.Repository
    // Other dependencies
}

func NewService(deps Dependencies) Service {
    return &service{
        repo: deps.Repository,
    }
}
```

**Repository Pattern Template:**
```go
// internal/domain/{domain}/repository.go
package {domain}

import (
    "context"
    "github.com/google/uuid"
)

type Repository interface {
    Create(ctx context.Context, entity *{Entity}) error
    Get(ctx context.Context, id uuid.UUID) (*{Entity}, error)
    Update(ctx context.Context, entity *{Entity}) error
    Delete(ctx context.Context, id uuid.UUID) error
    // Domain-specific queries
}
```

Begin execution with ultra-deep analysis of the feature specification and DCE's architecture. Deploy specialized agents in coordinated waves to deliver production-ready features that meet all performance, security, and compliance requirements while maintaining the high standards of the Dependable Call Exchange platform.