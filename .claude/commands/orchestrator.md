# **DCE FEATURE SPECIFICATION EXECUTOR (True Parallel Execution)**

Ultra-think about executing feature specifications using REAL PARALLEL EXECUTION via Task tools. You will spawn multiple specialized agents SIMULTANEOUSLY to achieve 5-8x faster implementation than sequential approaches.

## 🚀 **PARALLEL EXECUTION ADVANTAGE**

**CRITICAL**: This executor uses TRUE PARALLEL EXECUTION, not narrative parallelism:
- **5-8x faster implementation** than sequential execution
- **Genuine concurrent Tasks** via Task tool invocations
- **Independent specialist execution** with no blocking
- **Wave-based synchronization** for dependency management

```
Sequential:  [Domain]──►[Repository]──►[Service]──►[API]──►[Tests]
              15min      15min         15min      15min    15min = 75min

Parallel:    [Domain Expert    ]┐
             [Value Designer   ]├─► Wave 1: 3min
             [Event Architect  ]│
             [Repo Designer    ]│
             [Domain Tester    ]┘
                      ↓
             [Repo Builder     ]┐
             [Migration Eng    ]├─► Wave 2: 3min  
             [Query Optimizer  ]│
             [Cache Builder    ]│
             [Event Publisher  ]┘
                      ↓
             Total: ~10min vs 75min sequential!
```

## **COMMAND VARIABLES**

```
spec_file: $ARGUMENTS          # Path to feature specification
output_dir: $ARGUMENTS         # Where to generate code
execution_mode: $ARGUMENTS     # parallel|sequential|adaptive
quality_level: $ARGUMENTS      # draft|production|bulletproof
```

## **ARGUMENT PARSING**

Parse from "$ARGUMENTS":
1. `spec_file` - Path to the feature specification file
   Examples: "specs/consent-management-v2.md", "specs/dynamic-pricing.md"
   
2. `output_dir` - Directory for generated code (default: project root following DCE structure)
   
3. `execution_mode` - How to coordinate Tasks (default: adaptive)
   Options: parallel (maximum speed), sequential (safer), adaptive (smart dependencies)
   
4. `quality_level` - Code quality target (default: production)
   Options: draft (quick prototype), production (normal), bulletproof (maximum quality)

## **PHASE 1: SPECIFICATION ANALYSIS**

**Deep Spec Understanding Protocol:**

Read and analyze the specification file to extract all requirements. Create a shared context file at `.claude/context/feature-implementation.yaml` containing:

```yaml
feature_overview:
  name: [from spec]
  domain: [target domain]
  priority: [critical|high|medium]
  
technical_requirements:
  entities: [list of entities to create]
  value_objects: [list of value objects]
  services: [required services]
  apis: [endpoints to implement]
  repositories: [data access needs]
  
quality_targets:
  performance: [latency requirements]
  security: [auth/authz needs]
  compliance: [TCPA/GDPR requirements]
  testing: [coverage targets]
```

## **PHASE 2: WAVE 0 - FOUNDATION ANALYSIS (Single Task)**

**Spawn initial analysis Task to prepare for parallel execution:**

Use Task tool with description "Foundation Analyst - Deep Specification Analysis":
- Analyze specification in detail
- Map dependencies between components
- Identify parallelization opportunities
- Create wave execution plan
- Output: `.claude/context/wave-execution-plan.md`

Wait for Task completion before proceeding to parallel waves.

## **PHASE 3: WAVE 1 - DOMAIN FOUNDATION (5 Parallel Tasks)**

**Spawn these 5 Tasks SIMULTANEOUSLY using Task tool:**

**Task 1 - Domain Expert**:
- Description: "Domain Expert - Create [feature] entities with DDD patterns"
- Responsibilities:
  - Create all entities in `internal/domain/{domain}/`
  - Implement constructors with validation
  - Add business methods and invariants
  - Follow existing Call, Bid patterns

**Task 2 - Value Object Designer**:
- Description: "Value Designer - Create value objects for [feature]"
- Responsibilities:
  - Design immutable value objects
  - Implement validation logic
  - Follow Money, PhoneNumber patterns
  - Place in `internal/domain/values/`

**Task 3 - Domain Event Architect**:
- Description: "Event Architect - Define domain events for [feature]"
- Responsibilities:
  - Create event definitions
  - Include proper metadata
  - Follow event sourcing patterns
  - Support event replay

**Task 4 - Repository Interface Designer**:
- Description: "Repository Designer - Define data access contracts"
- Responsibilities:
  - Define repository interfaces
  - Include transaction methods
  - Add query specifications
  - Follow repository pattern

**Task 5 - Domain Unit Tester**:
- Description: "Domain Tester - Create comprehensive domain tests"
- Responsibilities:
  - Unit tests for all entities
  - Property-based tests for invariants
  - Test value object validation
  - Achieve 95%+ coverage

**ALL 5 TASKS EXECUTE IN PARALLEL!** Monitor and wait for all completions.

## **PHASE 4: WAVE 2 - INFRASTRUCTURE & PERSISTENCE (5 Parallel Tasks)**

**After Wave 1 validation, spawn next parallel wave:**

**Task 1 - Repository Implementer**:
- Description: "Repository Builder - PostgreSQL implementation"
- Responsibilities:
  - Implement all repository interfaces
  - Use sqlc for queries
  - Add transaction support
  - Include batch operations

**Task 2 - Migration Engineer**:
- Description: "Migration Engineer - Database schema and indexes"
- Responsibilities:
  - Create migration scripts
  - Design optimal indexes
  - Include constraints
  - Provide rollback scripts

**Task 3 - Query Optimizer**:
- Description: "Query Optimizer - Performance-tuned queries"
- Responsibilities:
  - Analyze query patterns
  - Create composite indexes
  - Optimize for < 1ms latency
  - Add query hints

**Task 4 - Cache Layer Builder**:
- Description: "Cache Builder - Redis integration layer"
- Responsibilities:
  - Design cache keys
  - Implement TTL strategies
  - Add cache warming
  - Handle invalidation

**Task 5 - Event Infrastructure**:
- Description: "Event Publisher - Domain event handling"
- Responsibilities:
  - Implement event publishing
  - Create event handlers
  - Add event replay support
  - Ensure ordering guarantees

**ALL 5 TASKS EXECUTE IN PARALLEL!**

## **PHASE 5: WAVE 3 - SERVICE & ORCHESTRATION (5 Parallel Tasks)**

**After Wave 2 validation, spawn service layer wave:**

**Task 1 - Service Orchestrator**:
- Description: "Service Orchestrator - Business logic coordination"
- Responsibilities:
  - Create service implementations
  - Handle transaction boundaries
  - Implement use cases
  - Max 5 dependencies rule

**Task 2 - DTO Designer**:
- Description: "DTO Designer - Request/Response models"
- Responsibilities:
  - Create API DTOs
  - Add validation tags
  - Implement converters
  - Document with examples

**Task 3 - Integration Coordinator**:
- Description: "Integration Coordinator - External service calls"
- Responsibilities:
  - Integrate with other services
  - Add circuit breakers
  - Implement retries
  - Handle timeouts

**Task 4 - Compliance Enforcer**:
- Description: "Compliance Enforcer - TCPA/GDPR implementation"
- Responsibilities:
  - Add compliance checks
  - Implement audit logging
  - Ensure data privacy
  - Handle consent

**Task 5 - Performance Optimizer**:
- Description: "Performance Optimizer - Sub-millisecond latency"
- Responsibilities:
  - Profile critical paths
  - Add caching strategies
  - Optimize algorithms
  - Reduce allocations

**ALL 5 TASKS EXECUTE IN PARALLEL!**

## **PHASE 6: WAVE 4 - API & QUALITY (5 Parallel Tasks)**

**Final implementation wave with API and quality tasks:**

**Task 1 - REST API Builder**:
- Description: "API Builder - REST endpoints implementation"
- Responsibilities:
  - Create handlers
  - Add middleware
  - Implement rate limiting
  - Follow REST patterns

**Task 2 - API Test Engineer**:
- Description: "API Tester - Integration and contract tests"
- Responsibilities:
  - API integration tests
  - Contract validation
  - Performance benchmarks
  - Error scenario tests

**Task 3 - Security Auditor**:
- Description: "Security Auditor - Authentication and authorization"
- Responsibilities:
  - Implement auth checks
  - Add input validation
  - Prevent injections
  - Security headers

**Task 4 - Documentation Writer**:
- Description: "Documentation Writer - API and developer docs"
- Responsibilities:
  - OpenAPI specifications
  - Developer guides
  - Architecture decisions
  - Deployment instructions

**Task 5 - Quality Validator**:
- Description: "Quality Validator - Final quality checks"
- Responsibilities:
  - Run all linters
  - Check test coverage
  - Validate performance
  - Ensure standards

**ALL 5 TASKS EXECUTE IN PARALLEL!**

## **WAVE SYNCHRONIZATION PROTOCOL**

Between each wave:
1. Verify all Tasks completed successfully
2. Run `go build -gcflags="-e" ./...` to check compilation
3. Validate expected outputs exist
4. Update shared context for next wave
5. Only proceed if quality gates pass

## **EXECUTION MONITORING**

Track progress in `.claude/context/execution-progress.md`:

```markdown
## Feature: [Name] - Parallel Execution Progress

### Wave 0: Analysis ✅ (2 min)
- [x] Specification analyzed
- [x] Dependencies mapped
- [x] Execution plan created

### Wave 1: Domain (5 Tasks) 🔄 (3 min)
- [x] Task 1: Domain Expert - 2 entities created
- [x] Task 2: Value Designer - 3 value objects
- [x] Task 3: Event Architect - 5 events defined
- [ ] Task 4: Repository Designer - In progress...
- [x] Task 5: Domain Tester - 95% coverage

### Wave 2: Infrastructure (5 Tasks) ⏸️
- Waiting for Wave 1 completion...

### Performance Metrics
- Total elapsed: 5 minutes
- Parallel speedup: 6.2x
- Tasks completed: 9/25
```

## **QUALITY GATES**

Each wave must pass before proceeding:
- **Compilation**: No errors with `go build -gcflags="-e"`
- **Linting**: golangci-lint passes
- **Tests**: All unit tests green
- **Coverage**: Meets targets (80%+ minimum)
- **Performance**: Sub-millisecond for critical paths

## **EXECUTION PATTERNS BY MODE**

### Parallel Mode (Maximum Speed)
- Launch all waves immediately
- No dependency checking
- Risk of integration issues
- Best for prototypes

### Sequential Mode (Maximum Safety)
- Complete wave before starting next
- Full validation between waves
- Slower but safer
- Best for critical features

### Adaptive Mode (Recommended)
- Analyze dependencies
- Parallelize within waves
- Sequential between dependent waves
- Best balance of speed and safety

## **CRITICAL SUCCESS FACTORS**

1. **Task Independence**: Each Task must be self-contained
2. **Clear Boundaries**: No overlap between Task responsibilities  
3. **Context Isolation**: Tasks cannot communicate during execution
4. **Wave Validation**: Always validate before next wave
5. **Progress Tracking**: Monitor all Task completions

Begin execution by analyzing the specification, then unleash the power of TRUE PARALLEL EXECUTION with coordinated Task waves!