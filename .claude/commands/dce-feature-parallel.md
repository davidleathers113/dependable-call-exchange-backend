# DCE Feature Executor - True Parallel Implementation

Ultra-think about executing feature specifications using ACTUAL PARALLEL EXECUTION. You will spawn real concurrent tasks via the Task tool, achieving 5-8x performance improvements.

Parse the following arguments from "$ARGUMENTS":
1. `spec_file` - Path to the feature specification file
2. `output_dir` - Directory for generated code (default: project root)
3. `execution_mode` - parallel|sequential|adaptive (default: parallel) 
4. `quality_level` - draft|production|bulletproof (default: production)

## PHASE 1: SPECIFICATION ANALYSIS & SHARED CONTEXT

Read the specification and create a shared context file for wave coordination:
- Create `.claude-context/feature-implementation.yaml`
- Extract all requirements and technical specifications
- Plan wave-based execution strategy

## PHASE 2: WAVE 0 - PROJECT ANALYSIS (Single Task)

Spawn a single analysis task:

**Task - Deep Project Analyzer**:
- Description: "Analyze DCE patterns for [feature]"
- Examine existing code patterns
- Identify integration points
- Document conventions
- Output: Updated shared context with discoveries

Wait for completion before proceeding.

## PHASE 3: WAVE 1 - DOMAIN FOUNDATION (5 Parallel Tasks)

Spawn 5 CONCURRENT tasks using the Task tool:

**Task 1 - Entity Architect**:
- Description: "Create domain entities for [feature]"
- Location: `internal/domain/{domain}/`
- Create all entities with DDD patterns
- Include validation and business methods

**Task 2 - Value Object Designer**:
- Description: "Create value objects for [feature]"
- Location: `internal/domain/values/`
- Implement immutability
- Follow Money, PhoneNumber patterns

**Task 3 - Domain Event Creator**:
- Description: "Define domain events for [feature]"
- Create all event definitions
- Follow event sourcing patterns
- Include proper metadata

**Task 4 - Repository Interface Designer**:
- Description: "Design repository interfaces"
- Define all query methods
- Follow repository pattern
- Include transaction support

**Task 5 - Domain Test Engineer**:
- Description: "Create domain unit tests"
- Test all business rules
- Include edge cases
- Aim for 100% coverage

ALL 5 TASKS EXECUTE SIMULTANEOUSLY! Wait for all to complete.

## PHASE 4: WAVE 2 - INFRASTRUCTURE & SERVICES (5 Parallel Tasks)

After Wave 1 validation, spawn next parallel wave:

**Task 1 - Repository Implementer**:
- Description: "Implement PostgreSQL repositories"
- Location: `internal/infrastructure/repository/`
- Efficient queries with indexes
- Transaction handling

**Task 2 - Migration Architect**:
- Description: "Create database migrations"
- Location: `migrations/`
- Include indexes and constraints
- Provide rollback scripts

**Task 3 - Service Orchestrator**:
- Description: "Build service layer"
- Location: `internal/service/{service}/`
- Business orchestration
- Transaction boundaries

**Task 4 - Integration Test Developer**:
- Description: "Create integration tests"
- Test with real database
- Mock external services
- Transaction scenarios

**Task 5 - Cache Strategist**:
- Description: "Implement caching layer"
- Redis integration
- Cache invalidation logic
- Performance optimization

ALL 5 TASKS EXECUTE SIMULTANEOUSLY!

## PHASE 5: WAVE 3 - API & QUALITY (5 Parallel Tasks)

Final parallel wave after Wave 2 validation:

**Task 1 - REST API Builder**:
- Description: "Create REST endpoints"
- Location: `internal/api/rest/handlers/`
- Request/response DTOs
- Validation middleware

**Task 2 - API Test Creator**:
- Description: "Build API tests"
- Test all endpoints
- Performance benchmarks
- Error scenarios

**Task 3 - Security Implementer**:
- Description: "Add security layer"
- JWT validation
- RBAC checks
- Input sanitization

**Task 4 - Compliance Guardian**:
- Description: "Implement compliance"
- TCPA/GDPR requirements
- Audit logging
- Consent management

**Task 5 - Documentation Generator**:
- Description: "Create documentation"
- OpenAPI specs
- Developer guides
- Code comments

ALL 5 TASKS EXECUTE SIMULTANEOUSLY!

## WAVE VALIDATION PROTOCOL

Between each wave:
1. Verify all tasks completed successfully
2. Check code compilation
3. Run basic integration tests
4. Update shared context
5. Prepare next wave inputs

## PERFORMANCE EXPECTATIONS

With true parallel execution:
- Wave 1: ~3 minutes (vs 15 sequential)
- Wave 2: ~3 minutes (vs 15 sequential)
- Wave 3: ~3 minutes (vs 15 sequential)
- Total: ~10 minutes (vs 45+ sequential)

## CRITICAL SUCCESS FACTORS

1. **Task Independence**: Each task must be self-contained
2. **Clear Outputs**: Specify exact files to create
3. **No Communication**: Tasks cannot interact during execution
4. **Wave Synchronization**: Validate between waves
5. **Context Isolation**: Each task gets minimal necessary context

Begin execution with specification analysis, then spawn ACTUAL PARALLEL TASKS for massive performance gains!