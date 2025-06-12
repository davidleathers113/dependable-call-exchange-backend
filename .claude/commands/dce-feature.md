# DCE Feature Implementation Orchestrator (True Parallel Execution)

Ultra-think about orchestrating parallel implementation of DCE features using the Task tool for genuine concurrent execution. You will spawn specialized agents as independent Tasks that work simultaneously within coordinated waves.

**CRITICAL**: This system uses ACTUAL parallel execution via Task tools, not narrative parallelism. Each specialist is a real Task invocation.

Parse the following arguments from "$ARGUMENTS":
1. `spec_file` - Path to the feature specification file
2. `output_dir` - Directory for generated code (default: ".")
3. `execution_mode` - How to coordinate agents (default: adaptive)
   Options: parallel (all waves concurrent), sequential (wave by wave), adaptive (smart dependencies)
4. `quality_level` - Code quality target (default: production)
   Options: draft (quick), production (standard), bulletproof (maximum)

## PHASE 1: SPECIFICATION ANALYSIS & PLANNING

Read and deeply analyze the specification file:
- Parse all domain model requirements
- Identify service layer needs
- Extract API specifications
- Map dependencies between components
- Plan optimal wave execution strategy

Create a shared context file at `.claude/context/feature-context.yaml` containing:
- Feature overview
- Domain models to create
- Service requirements
- API endpoints needed
- Quality standards
- Dependencies map

## PHASE 2: WAVE 0 - FOUNDATION ANALYSIS (Single Task)

Spawn initial analysis task to prepare detailed implementation plan:

Use Task tool with description "Foundation Analyst - Implementation Planning" to:
- Deep dive into specification
- Generate detailed task breakdown
- Create dependency graph
- Output implementation blueprint to `.claude/context/implementation-plan.md`

## PHASE 3: WAVE 1 - DOMAIN FOUNDATION (Parallel Tasks)

After Wave 0 completes, spawn 5 parallel domain tasks:

**Task 1 - Entity Architect**:
- Description: "Entity Architect - Core Domain Entities"
- Create all entities in `internal/domain/{domain}/`
- Follow DDD patterns with proper constructors
- Include business validation

**Task 2 - Value Object Designer**:
- Description: "Value Object Designer - Domain Values"
- Create value objects in `internal/domain/values/`
- Implement validation and immutability
- Follow existing patterns (Money, PhoneNumber)

**Task 3 - Domain Event Architect**:
- Description: "Domain Event Architect - Event Definitions"
- Define domain events for the feature
- Create event structures with proper fields
- Place in appropriate domain package

**Task 4 - Repository Interface Designer**:
- Description: "Repository Interface Designer - Contracts"
- Define repository interfaces in domain
- Include all CRUD and query methods
- Follow existing interface patterns

**Task 5 - Domain Service Designer**:
- Description: "Domain Service Designer - Business Logic"
- Create domain services for complex logic
- Implement business rules and invariants
- Keep services focused and testable

Wait for all Wave 1 tasks to complete. Validate outputs before proceeding.

## PHASE 4: WAVE 2 - INFRASTRUCTURE & PERSISTENCE (Parallel Tasks)

After Wave 1 validation, spawn 5 parallel infrastructure tasks:

**Task 1 - Repository Implementer**:
- Description: "Repository Builder - PostgreSQL Implementation"
- Implement all repository interfaces
- Use sqlc for query generation
- Include transaction support

**Task 2 - Migration Engineer**:
- Description: "Migration Engineer - Database Schema"
- Create migrations in `migrations/`
- Follow naming convention
- Include indexes and constraints

**Task 3 - Query Optimizer**:
- Description: "Query Optimizer - Performance Queries"
- Create optimized queries
- Add appropriate indexes
- Consider query patterns

**Task 4 - Cache Layer Builder**:
- Description: "Cache Layer Builder - Redis Integration"
- Implement caching strategies
- Create cache keys and TTLs
- Integrate with repositories

**Task 5 - Event Publisher**:
- Description: "Event Publisher - Domain Event Handling"
- Implement event publishing
- Create event handlers
- Integrate with infrastructure

## PHASE 5: WAVE 3 - SERVICE & ORCHESTRATION (Parallel Tasks)

After Wave 2 validation, spawn 5 parallel service tasks:

**Task 1 - Service Orchestrator**:
- Description: "Service Orchestrator - Business Logic"
- Create service layer in `internal/service/`
- Implement use cases from spec
- Handle transactions and errors

**Task 2 - DTO Designer**:
- Description: "DTO Designer - Data Transfer Objects"
- Create request/response DTOs
- Implement validation tags
- Add conversion methods

**Task 3 - Integration Coordinator**:
- Description: "Integration Coordinator - External Services"
- Integrate with other services
- Handle external API calls
- Implement circuit breakers

**Task 4 - Compliance Enforcer**:
- Description: "Compliance Enforcer - TCPA/GDPR"
- Add compliance checks
- Implement audit logging
- Ensure data privacy

**Task 5 - Performance Tuner**:
- Description: "Performance Tuner - Optimization"
- Add performance optimizations
- Implement caching strategies
- Ensure sub-millisecond latency

## PHASE 6: WAVE 4 - API & PRESENTATION (Parallel Tasks)

After Wave 3 validation, spawn 5 parallel API tasks:

**Task 1 - REST API Builder**:
- Description: "REST API Builder - HTTP Endpoints"
- Create handlers in `internal/api/rest/`
- Implement all endpoints from spec
- Add proper error handling

**Task 2 - GraphQL Schema Designer**:
- Description: "GraphQL Designer - Schema & Resolvers"
- Create GraphQL schema if needed
- Implement resolvers
- Add DataLoader for N+1 prevention

**Task 3 - WebSocket Handler**:
- Description: "WebSocket Handler - Real-time Events"
- Implement WebSocket endpoints
- Create event streaming
- Handle connection management

**Task 4 - API Documentation**:
- Description: "API Documenter - OpenAPI Specs"
- Generate OpenAPI documentation
- Create example requests
- Document all endpoints

**Task 5 - Middleware Engineer**:
- Description: "Middleware Engineer - Cross-cutting"
- Add authentication middleware
- Implement rate limiting
- Add request validation

## PHASE 7: WAVE 5 - QUALITY ASSURANCE (Parallel Tasks)

After Wave 4 validation, spawn 5 parallel QA tasks:

**Task 1 - Unit Test Engineer**:
- Description: "Unit Test Engineer - Domain & Service Tests"
- Create comprehensive unit tests
- Achieve 90%+ coverage
- Use table-driven tests

**Task 2 - Integration Tester**:
- Description: "Integration Tester - E2E Tests"
- Create integration tests
- Test full workflows
- Use testcontainers

**Task 3 - Performance Tester**:
- Description: "Performance Tester - Benchmarks"
- Create performance benchmarks
- Validate latency requirements
- Test under load

**Task 4 - Security Auditor**:
- Description: "Security Auditor - Vulnerability Tests"
- Perform security testing
- Check authentication/authorization
- Test input validation

**Task 5 - Documentation Writer**:
- Description: "Documentation Writer - Technical Docs"
- Create README files
- Document architecture decisions
- Write deployment guides

## QUALITY GATES BETWEEN WAVES

After each wave, before proceeding:
1. Run `go build -gcflags="-e" ./...` to check compilation
2. Validate all expected files were created
3. Check that interfaces match implementations
4. Ensure consistent naming and patterns
5. Verify performance constraints

## EXECUTION CONTROL

Based on execution_mode:
- **parallel**: Launch all waves immediately (risky but fast)
- **sequential**: Wait for each wave to complete (safe but slower)
- **adaptive**: Analyze dependencies and parallelize where possible

Based on quality_level:
- **draft**: Skip some tests, basic implementation
- **production**: Full implementation with standard quality
- **bulletproof**: Maximum quality with extensive testing

## PROGRESS TRACKING

Maintain progress in `.claude/context/progress.md`:
- Track completed tasks
- Note any issues or blockers
- Record performance metrics
- List remaining work

Begin orchestration by deeply analyzing the specification, then systematically spawn Task tools for true parallel execution. Monitor progress and ensure quality at each wave.