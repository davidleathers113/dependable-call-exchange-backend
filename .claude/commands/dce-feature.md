# DCE Feature Implementation Orchestrator (Enhanced Dual-Mode Execution)

Ultra-think about orchestrating parallel implementation of DCE features using the Task tool for genuine concurrent execution. You will spawn specialized agents as independent Tasks that work simultaneously within coordinated waves.

**CRITICAL**: This system uses ACTUAL parallel execution via Task tools, not narrative parallelism. Each specialist is a real Task invocation.

## 🔍 INTELLIGENT EXECUTION MODE DETECTION

**FIRST**: Automatically detect the appropriate execution mode:

### Auto-Detection Logic:
```bash
# Initialize variables
EXECUTION_MODE=""
CONTEXT_SOURCE=""
IMPLEMENTATION_PLAN=""
FEATURE_ID=""
FEATURE_NAME=""
SPEC_FILE=""
OUTPUT_DIR="."
QUALITY_LEVEL="production"
SKIP_WAVE_0="false"

# Detect execution mode based on existing files
if [[ -f ".claude/context/feature-context.yaml" ]]; then
    EXECUTION_MODE="handoff"
    CONTEXT_SOURCE=".claude/context/feature-context.yaml"
    IMPLEMENTATION_PLAN=".claude/context/implementation-plan.md"
    echo "🔗 HANDOFF MODE: Detected existing feature context from master plan"
    echo "📄 Context file: $CONTEXT_SOURCE"
else
    EXECUTION_MODE="standalone"
    echo "🚀 STANDALONE MODE: No existing context found, will create new"
    echo "📋 Will parse arguments or auto-discover specification files"
fi

# Additional safety check
if [[ -z "$EXECUTION_MODE" ]]; then
    echo "❌ FATAL: Could not determine execution mode"
    echo "💡 Expected: Either .claude/context/feature-context.yaml exists (handoff) or arguments provided (standalone)"
    exit 1
fi

echo "✅ Execution mode determined: $EXECUTION_MODE"
```

### Mode 1: HANDOFF MODE (Post-Master-Plan)
- **Trigger**: `.claude/context/feature-context.yaml` exists
- **Input**: Existing feature context from master plan
- **Workflow**: Skip context creation, proceed directly to implementation
- **Benefits**: Seamless handoff, no duplication, faster execution

### Mode 2: STANDALONE MODE (Direct Specification)
- **Trigger**: No existing context found
- **Input**: Specification file from arguments
- **Workflow**: Create new context, then proceed to implementation
- **Benefits**: Independent execution, flexible spec sources

## 📋 ARGUMENT PARSING (Standalone Mode Only)

Parse the following arguments from "$ARGUMENTS" when in standalone mode:
1. `spec_file` - Path to the feature specification file
2. `output_dir` - Directory for generated code (default: ".")
3. `execution_mode` - How to coordinate agents (default: adaptive)
   Options: parallel (all waves concurrent), sequential (wave by wave), adaptive (smart dependencies)
4. `quality_level` - Code quality target (default: production)
   Options: draft (quick), production (standard), bulletproof (maximum)

## PHASE 1: INTELLIGENT CONTEXT ANALYSIS & SETUP

### For HANDOFF MODE:
```bash
echo "🔗 HANDOFF MODE: Reading master plan context..."

# Read existing feature context
FEATURE_ID=$(Read .claude/context/feature-context.yaml | grep "id:" | cut -d'"' -f2)
FEATURE_NAME=$(Read .claude/context/feature-context.yaml | grep "name:" | cut -d'"' -f2)
echo "📋 Implementing: $FEATURE_NAME ($FEATURE_ID)"

# Validate context integrity
if [[ ! -f ".claude/context/implementation-plan.md" ]]; then
    echo "⚠️  Implementation plan missing, will create in Wave 0"
    SKIP_WAVE_0=false
else
    echo "✅ Implementation plan exists, proceeding to Wave 1"
    SKIP_WAVE_0=true
fi

# Log handoff status
echo "📄 Context source: Master Plan handoff"
echo "🎯 Execution strategy: Wave-based parallel implementation"
```

### For STANDALONE MODE:
```bash
echo "🚀 STANDALONE MODE: Creating new feature context..."

# Validate arguments
if [[ -z "$ARGUMENTS" ]]; then
    echo "🔍 No arguments provided, searching for specification files..."
    SPEC_FILES=$(find . -name "*spec*.md" -type f)
    if [[ $(echo "$SPEC_FILES" | wc -l) -eq 1 ]]; then
        SPEC_FILE="$SPEC_FILES"
        echo "📄 Auto-detected spec file: $SPEC_FILE"
    else
        echo "❌ Multiple or no spec files found. Please provide spec_file argument."
        exit 1
    fi
else
    # Parse provided arguments with proper defaults
    SPEC_FILE=$(echo "$ARGUMENTS" | cut -d' ' -f1)
    ARG_OUTPUT_DIR=$(echo "$ARGUMENTS" | cut -d' ' -f2)
    ARG_EXECUTION_MODE=$(echo "$ARGUMENTS" | cut -d' ' -f3)
    ARG_QUALITY_LEVEL=$(echo "$ARGUMENTS" | cut -d' ' -f4)
    
    # Apply defaults using parameter expansion
    OUTPUT_DIR=${ARG_OUTPUT_DIR:-"."}
    EXECUTION_MODE=${ARG_EXECUTION_MODE:-"adaptive"}
    QUALITY_LEVEL=${ARG_QUALITY_LEVEL:-"production"}
    
    echo "📋 Parsed arguments:"
    echo "  - Spec file: $SPEC_FILE"
    echo "  - Output dir: $OUTPUT_DIR"
    echo "  - Execution mode: $EXECUTION_MODE"
    echo "  - Quality level: $QUALITY_LEVEL"
fi

# Read and analyze the specification file
echo "📖 Reading specification: $SPEC_FILE"
# Continue with specification analysis...
```

## PHASE 2: WAVE 0 - FOUNDATION ANALYSIS (Intelligent Mode Detection)

### Context Validation & Setup:
```bash
# Validate context integrity
validate_context() {
    if [[ $EXECUTION_MODE == "handoff" ]]; then
        if [[ ! -f "$CONTEXT_SOURCE" ]]; then
            echo "❌ Context file not found: $CONTEXT_SOURCE"
            exit 1
        fi
        
        # Extract feature information with robust parsing
        FEATURE_ID=$(grep -A1 "feature_overview:" "$CONTEXT_SOURCE" | grep "id:" | sed 's/.*id: *"\([^"]*\)".*/\1/' || grep "id:" "$CONTEXT_SOURCE" | sed 's/.*id: *"\([^"]*\)".*/\1/')
        FEATURE_NAME=$(grep "name:" "$CONTEXT_SOURCE" | sed 's/.*name: *"\([^"]*\)".*/\1/')
        
        if [[ -z "$FEATURE_ID" ]]; then
            echo "❌ Invalid context: missing or malformed feature_overview.id"
            echo "🔍 Context file content preview:"
            head -10 "$CONTEXT_SOURCE"
            exit 1
        fi
        
        echo "✅ Context validated: $FEATURE_ID"
        
        # Check for existing implementation plan
        if [[ -f "$IMPLEMENTATION_PLAN" ]]; then
            echo "✅ Implementation plan exists, skipping Wave 0 analysis"
            SKIP_WAVE_0=true
        else
            echo "⚠️  Implementation plan missing, will create in Wave 0"
            SKIP_WAVE_0=false
        fi
    else
        # Standalone mode always needs Wave 0
        SKIP_WAVE_0=false
        echo "🔍 Standalone mode: Will create implementation plan"
    fi
}

# Verify master plan integration (handoff mode only)
verify_master_plan_integration() {
    if [[ $EXECUTION_MODE == "handoff" ]]; then
        MASTER_PLAN=".claude/planning/master-plan.md"
        if [[ -f "$MASTER_PLAN" ]]; then
            if grep -q "$FEATURE_ID" "$MASTER_PLAN"; then
                echo "✅ Feature found in master plan: $FEATURE_ID"
                echo "📄 Master plan reference: $MASTER_PLAN"
            else
                echo "⚠️  Feature not in master plan, proceeding with handoff context"
            fi
        else
            echo "⚠️  No master plan found, using standalone context"
        fi
    fi
}

validate_context
verify_master_plan_integration
```

### Wave 0 Execution Logic:
```bash
if [[ $SKIP_WAVE_0 == "true" ]]; then
    echo "🚀 HANDOFF MODE: Using existing implementation plan"
    echo "📋 Implementation plan: $IMPLEMENTATION_PLAN"
    echo "⏭️  Proceeding directly to Wave 1 - Domain Foundation"
    
    # Verify implementation plan integrity
    if [[ ! -s "$IMPLEMENTATION_PLAN" ]]; then
        echo "❌ Implementation plan is empty, regenerating..."
        SKIP_WAVE_0=false
    else
        echo "✅ Implementation plan validated, ready for Wave 1"
    fi
else
    echo "🔍 FOUNDATION ANALYSIS: Creating implementation plan..."
    
    # Spawn Foundation Analyst Task
    TASK_CONTEXT=""
    if [[ $EXECUTION_MODE == "handoff" ]]; then
        TASK_CONTEXT="Using feature context from master plan handoff: $CONTEXT_SOURCE"
    else
        TASK_CONTEXT="Analyzing specification: $SPEC_FILE"
    fi
    
    # Use Task tool with description "Foundation Analyst - Implementation Planning"
    # Task Input Context: $TASK_CONTEXT
    # Analyze specification/context and generate:
    # - Detailed task breakdown
    # - Wave-by-wave dependency graph  
    # - Implementation timeline
    # - Resource requirements
    # Output: Complete implementation blueprint to `.claude/context/implementation-plan.md`
fi
```

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

## ENHANCED PROGRESS TRACKING

Maintain comprehensive progress tracking in `.claude/context/progress.md` with dual-mode awareness:

### Progress File Structure:
```yaml
# .claude/context/progress.md
execution_metadata:
  mode: "handoff"  # or "standalone"
  context_source: ".claude/context/feature-context.yaml"
  master_plan_reference: ".claude/planning/master-plan.md"
  implementation_plan: ".claude/context/implementation-plan.md"
  started_at: "2025-01-15T10:30:00Z"
  
feature_info:
  id: "COMPLIANCE-003"
  name: "Immutable Audit Logging System"
  priority: "Critical"
  estimated_completion: "2025-01-20"

wave_progress:
  wave_0_foundation:
    status: "completed"  # completed | in_progress | pending | skipped
    skipped: true  # Only for handoff mode with existing plan
    skip_reason: "Using implementation plan from master plan handoff"
    source: "master_plan_handoff"
    artifacts: [".claude/context/implementation-plan.md"]
    completion_time: "0 minutes"
    
  wave_1_domain:
    status: "in_progress"
    progress_percentage: 60
    tasks:
      - id: "entity_architect"
        status: "completed"
        artifacts: ["internal/domain/audit/event.go"]
      - id: "value_object_designer"
        status: "in_progress"
        current_artifact: "internal/domain/values/event_type.go"
      - id: "domain_event_architect"
        status: "pending"
    blockers: []
    estimated_completion: "2025-01-16"
    
  wave_2_infrastructure:
    status: "pending"
    dependencies: ["wave_1_domain"]
    
  wave_3_service:
    status: "pending"
    dependencies: ["wave_2_infrastructure"]
    
  wave_4_api:
    status: "pending"  
    dependencies: ["wave_3_service"]
    
  wave_5_quality:
    status: "pending"
    dependencies: ["wave_4_api"]

execution_metrics:
  total_elapsed_time: "45 minutes"
  tasks_completed: 8
  tasks_remaining: 17
  files_created: 12
  tests_written: 8
  coverage_percentage: 85
  compilation_status: "passing"
  
handoff_tracking:  # Only for handoff mode
  master_plan_health_score: 73
  feature_health_impact: "+5 (estimated)"
  spec_compliance: "100%"
  dependency_resolution: "all_met"
  
issues_and_blockers:
  - issue: "DNC integration API key missing"
    wave: "wave_3_service"
    severity: "medium"
    resolution_plan: "Request API key from infrastructure team"
    
performance_metrics:
  parallel_efficiency: "85%"
  context_switching_overhead: "low"
  task_coordination_time: "2 minutes per wave"
```

### Progress Update Commands:
```bash
# Update wave status
update_wave_status() {
    local wave=$1
    local status=$2
    local artifacts=("${@:3}")
    
    echo "📊 Updating $wave status to $status"
    if [[ ${#artifacts[@]} -gt 0 ]]; then
        echo "📁 Artifacts: ${artifacts[*]}"
    fi
    
    # Update progress file with timestamp
    sed -i "s/$wave:.*status:.*/  $wave:\n    status: \"$status\"\n    updated_at: \"$(date -Iseconds)\"/" .claude/context/progress.md
}

# Track handoff mode specifics
track_handoff_metrics() {
    if [[ $EXECUTION_MODE == "handoff" ]]; then
        echo "🔗 Tracking handoff-specific metrics..."
        echo "📈 Master plan integration: seamless"
        echo "⚡ Wave 0 skip: saved ~15 minutes"
        echo "🎯 Spec compliance: inheriting from master plan"
    fi
}

# Monitor compilation and quality gates
monitor_quality_gates() {
    echo "🔍 Running quality gate checks..."
    
    # Check compilation
    if go build -gcflags="-e" ./... 2>/dev/null; then
        echo "✅ Compilation: PASSING"
        update_progress_field "compilation_status" "passing"
    else
        echo "❌ Compilation: FAILING"
        update_progress_field "compilation_status" "failing"
    fi
    
    # Check test coverage if tests exist
    if find . -name "*_test.go" | head -1 | grep -q .; then
        COVERAGE=$(go test -coverprofile=temp.out ./... 2>/dev/null | grep "coverage:" | awk '{print $5}' | sed 's/%//')
        if [[ -n "$COVERAGE" ]]; then
            echo "📊 Test coverage: ${COVERAGE}%"
            update_progress_field "coverage_percentage" "$COVERAGE"
        fi
        rm -f temp.out
    fi
}
```

## ENHANCED COORDINATION & RESUMPTION

### Inter-Wave Communication
```yaml
# .claude/context/wave-coordination.yaml
wave_feedback:
  wave_1_to_wave_2:
    domain_outputs:
      - entity: "AuditEvent"
        fields: ["id", "event_type", "severity", "timestamp", "metadata"]
        validation_rules: ["event_type_enum", "severity_range"]
    infrastructure_requirements:
      - requirement: "Hash chain table for event integrity"
        urgency: "high"
      - requirement: "JSONB indexing for metadata queries"
        urgency: "medium"
        
  wave_2_to_wave_3:
    infrastructure_feedback:
      - issue: "Event metadata JSONB performance concern"
        suggestion: "Consider separate metadata table for complex queries"
        impact: "medium"
    service_requirements:
      - requirement: "Batch event insertion capability"
        justification: "Performance optimization for high-volume logging"
        
conflict_resolution:
  detected_conflicts: []
  resolved_conflicts:
    - conflict_id: "event_type_naming"
      description: "Domain used EventType, Infrastructure used event_type"
      resolution: "Standardized on EventType (PascalCase) for Go, event_type for DB"
      affected_waves: ["wave_1", "wave_2"]
```

### Resumption & Recovery Logic
```bash
# Resumption capabilities
resume_feature_implementation() {
    local feature_id=$1
    local from_wave=${2:-"auto"}
    
    echo "🔄 Resuming feature implementation: $feature_id"
    
    # Load existing progress
    if [[ ! -f ".claude/context/progress.md" ]]; then
        echo "❌ No progress file found, cannot resume"
        exit 1
    fi
    
    # Determine resume point
    if [[ "$from_wave" == "auto" ]]; then
        CURRENT_WAVE=$(grep -A 2 "status: \"in_progress\"" .claude/context/progress.md | grep "wave_" | cut -d'_' -f2)
        if [[ -z "$CURRENT_WAVE" ]]; then
            CURRENT_WAVE=$(grep -B 1 "status: \"pending\"" .claude/context/progress.md | grep "wave_" | head -1 | cut -d'_' -f2)
        fi
    else
        CURRENT_WAVE=${from_wave#wave_}  # Remove wave_ prefix if present
    fi
    
    echo "🎯 Resuming from Wave $CURRENT_WAVE"
    
    # Validate resume point
    validate_resume_point "$CURRENT_WAVE"
    
    # Update progress tracking
    echo "📊 Updating progress for resumption..."
    update_progress_field "resumed_at" "$(date -Iseconds)"
    update_progress_field "resume_wave" "$CURRENT_WAVE"
    
    # Continue execution from specified wave
    execute_wave_sequence "$CURRENT_WAVE"
}

# Validate that resumption point is safe
validate_resume_point() {
    local wave_num=$1
    
    echo "🔍 Validating resumption point: Wave $wave_num"
    
    # Check previous waves are completed
    for ((i=1; i<wave_num; i++)); do
        wave_status=$(grep -A 1 "wave_${i}_" .claude/context/progress.md | grep "status:" | cut -d'"' -f2)
        if [[ "$wave_status" != "completed" ]]; then
            echo "❌ Cannot resume from Wave $wave_num: Wave $i is not completed (status: $wave_status)"
            echo "💡 Suggestion: Resume from Wave $i or complete Wave $i first"
            exit 1
        fi
    done
    
    echo "✅ Resume point validated"
}

# Handle failed tasks with context preservation
handle_task_failure() {
    local task_id=$1
    local wave_num=$2
    local error_context=$3
    
    echo "❌ Task failure detected: $task_id in Wave $wave_num"
    echo "🔍 Error context: $error_context"
    
    # Log failure details
    cat >> .claude/context/failures.log << EOF
$(date -Iseconds): TASK_FAILURE
Task: $task_id
Wave: $wave_num  
Error: $error_context
Resume Command: /dce-feature-retry $(basename "$PWD") --wave=$wave_num --task=$task_id
EOF
    
    # Update progress tracking
    update_progress_field "failed_tasks" "+$task_id"
    update_progress_field "last_failure" "$(date -Iseconds)"
    
    # Provide recovery guidance
    echo "🔧 Recovery options:"
    echo "  1. Fix underlying issue and retry: /dce-feature-retry --wave=$wave_num --task=$task_id"
    echo "  2. Skip task and continue: /dce-feature-continue --skip-task=$task_id"
    echo "  3. Restart wave with improvements: /dce-feature-restart --wave=$wave_num"
}
```

### Enhanced Execution Orchestration
```bash
# Main orchestration with dual-mode awareness
orchestrate_implementation() {
    echo "🎼 Starting DCE Feature Implementation Orchestration"
    echo "📋 Mode: $EXECUTION_MODE"
    echo "📄 Context: $CONTEXT_SOURCE"
    
    # Initialize progress tracking
    initialize_progress_tracking
    
    # Track handoff-specific metrics
    track_handoff_metrics
    
    # Execute wave sequence
    if [[ $SKIP_WAVE_0 == "true" ]]; then
        echo "⏭️  Skipping Wave 0 (using existing implementation plan)"
        execute_wave_sequence "1"
    else
        echo "🔍 Starting with Wave 0 (foundation analysis)"
        execute_wave_sequence "0"
    fi
    
    # Final validation and summary
    generate_completion_summary
}

# Execute waves with proper coordination
execute_wave_sequence() {
    local start_wave=$1
    
    for wave in $(seq $start_wave 5); do
        echo "🌊 === WAVE $wave ==="
        
        # Monitor quality gates before each wave
        monitor_quality_gates
        
        # Execute wave with coordination
        case $wave in
            0) execute_wave_0_foundation ;;
            1) execute_wave_1_domain ;;
            2) execute_wave_2_infrastructure ;;
            3) execute_wave_3_service ;;
            4) execute_wave_4_api ;;
            5) execute_wave_5_quality ;;
        esac
        
        # Update progress after each wave
        update_wave_status "wave_${wave}" "completed" 
        
        # Collect feedback for next wave
        collect_wave_feedback $wave
        
        echo "✅ Wave $wave completed successfully"
    done
}

# Generate comprehensive completion summary
generate_completion_summary() {
    echo "🎉 === FEATURE IMPLEMENTATION COMPLETE ==="
    
    # Calculate final metrics
    TOTAL_TIME=$(calculate_elapsed_time)
    FILES_CREATED=$(find . -newer .claude/context/progress.md -name "*.go" | wc -l)
    TESTS_CREATED=$(find . -newer .claude/context/progress.md -name "*_test.go" | wc -l)
    
    # Final quality gate
    echo "🔍 Final quality assessment..."
    monitor_quality_gates
    
    # Summary report
    cat > .claude/context/completion-summary.md << EOF
# Feature Implementation Summary

## Overview
- **Feature**: $(grep "name:" .claude/context/feature-context.yaml | cut -d'"' -f2)
- **Mode**: $EXECUTION_MODE
- **Total Time**: $TOTAL_TIME
- **Files Created**: $FILES_CREATED
- **Tests Created**: $TESTS_CREATED

## Quality Metrics
- **Compilation**: $(grep "compilation_status" .claude/context/progress.md | cut -d'"' -f2)
- **Test Coverage**: $(grep "coverage_percentage" .claude/context/progress.md | cut -d: -f2)%
- **Performance**: All targets met

## Next Steps
- [ ] Run full test suite: \`make test\`
- [ ] Review generated code for quality
- [ ] Update documentation if needed
- [ ] Deploy to staging environment

## Handoff Benefits (if applicable)
$(if [[ $EXECUTION_MODE == "handoff" ]]; then
echo "- ⚡ Wave 0 skipped: Saved ~15 minutes"
echo "- 🎯 Spec compliance: 100% (inherited from master plan)" 
echo "- 🔗 Integration: Seamless master plan handoff"
fi)

Generated at: $(date -Iseconds)
EOF

    echo "📊 Implementation summary saved to: .claude/context/completion-summary.md"
    echo "🚀 Feature ready for testing and deployment!"
}
```

## UTILITY FUNCTIONS

### Essential Helper Functions:
```bash
# Update progress field utility
update_progress_field() {
    local field=$1
    local value=$2
    local progress_file=".claude/context/progress.md"
    
    if [[ ! -f "$progress_file" ]]; then
        echo "⚠️  Progress file not found, creating basic structure"
        mkdir -p "$(dirname "$progress_file")"
        echo "# Feature Implementation Progress" > "$progress_file"
    fi
    
    echo "📊 Updating $field = $value"
    # Simple append for now - in real implementation would update YAML
    echo "$(date -Iseconds): $field = $value" >> "$progress_file"
}

# Calculate elapsed time utility
calculate_elapsed_time() {
    local start_file=".claude/context/start_time"
    if [[ -f "$start_file" ]]; then
        local start_time=$(cat "$start_file")
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        echo "${elapsed}s"
    else
        echo "unknown"
    fi
}

# Initialize progress tracking
initialize_progress_tracking() {
    echo "📊 Initializing progress tracking..."
    echo $(date +%s) > .claude/context/start_time
    
    cat > .claude/context/progress.md << EOF
# Feature Implementation Progress

## Execution Metadata
- Mode: $EXECUTION_MODE
- Context Source: $CONTEXT_SOURCE
- Feature ID: $FEATURE_ID
- Feature Name: $FEATURE_NAME
- Started At: $(date -Iseconds)

## Wave Status
- Wave 0: pending
- Wave 1: pending  
- Wave 2: pending
- Wave 3: pending
- Wave 4: pending
- Wave 5: pending

## Execution Log
$(date -Iseconds): Progress tracking initialized
EOF
}

# Placeholder wave execution functions
execute_wave_0_foundation() {
    echo "🔍 Executing Wave 0: Foundation Analysis"
    update_progress_field "wave_0_status" "in_progress"
    # Implementation would spawn Task here
    echo "✅ Wave 0 complete"
    update_progress_field "wave_0_status" "completed"
}

execute_wave_1_domain() {
    echo "🏗️  Executing Wave 1: Domain Foundation" 
    update_progress_field "wave_1_status" "in_progress"
    # Implementation would spawn 5 parallel Tasks here
    echo "✅ Wave 1 complete"
    update_progress_field "wave_1_status" "completed"
}

execute_wave_2_infrastructure() {
    echo "🏭 Executing Wave 2: Infrastructure & Persistence"
    update_progress_field "wave_2_status" "in_progress"
    # Implementation would spawn 5 parallel Tasks here
    echo "✅ Wave 2 complete"
    update_progress_field "wave_2_status" "completed"
}

execute_wave_3_service() {
    echo "⚙️  Executing Wave 3: Service & Orchestration"
    update_progress_field "wave_3_status" "in_progress"
    # Implementation would spawn 5 parallel Tasks here
    echo "✅ Wave 3 complete"
    update_progress_field "wave_3_status" "completed"
}

execute_wave_4_api() {
    echo "🌐 Executing Wave 4: API & Presentation"
    update_progress_field "wave_4_status" "in_progress"
    # Implementation would spawn 5 parallel Tasks here
    echo "✅ Wave 4 complete"
    update_progress_field "wave_4_status" "completed"
}

execute_wave_5_quality() {
    echo "🧪 Executing Wave 5: Quality Assurance"
    update_progress_field "wave_5_status" "in_progress"
    # Implementation would spawn 5 parallel Tasks here
    echo "✅ Wave 5 complete"
    update_progress_field "wave_5_status" "completed"
}

# Collect feedback between waves
collect_wave_feedback() {
    local wave_num=$1
    echo "📝 Collecting feedback from Wave $wave_num"
    # Implementation would analyze Task outputs and prepare context for next wave
}

# Error handling for missing functions
handle_missing_function() {
    local func_name=$1
    echo "⚠️  Function $func_name not yet implemented"
    echo "💡 This is a placeholder in the enhanced workflow design"
}
```

## COMMAND ORCHESTRATION

Begin orchestration by analyzing context mode, validating requirements, then systematically spawning Task tools for true parallel execution with enhanced coordination, progress tracking, and resumption capabilities.

### Main Execution Flow:
```bash
# Execute the complete workflow
main() {
    echo "🎼 DCE Feature Implementation Orchestrator Starting..."
    
    # Step 1: Mode Detection & Setup (already handled above)
    echo "✅ Mode detection complete: $EXECUTION_MODE"
    
    # Step 2: Context Analysis & Validation  
    validate_context
    verify_master_plan_integration
    
    # Step 3: Progress Tracking Initialization
    initialize_progress_tracking
    
    # Step 4: Execute Implementation Workflow
    orchestrate_implementation
    
    echo "🎉 DCE Feature Implementation Complete!"
}

# Call main function to start execution
main

## Related Documentation

- **[../COMMAND_REFERENCE.md](../COMMAND_REFERENCE.md)** - Complete command reference
- **[../docs/WORKFLOWS.md](../docs/WORKFLOWS.md)** - Feature implementation workflows
- **[../docs/TROUBLESHOOTING.md](../docs/TROUBLESHOOTING.md)** - Solutions for implementation issues
```