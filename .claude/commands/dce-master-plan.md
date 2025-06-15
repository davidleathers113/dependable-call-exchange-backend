# DCE Strategic Master Planner (Parallel Execution)

Ultra-think about orchestrating PARALLEL comprehensive analysis using multiple Task tools for maximum speed and thoroughness. You will deploy specialized analyst agents concurrently to examine the entire DCE codebase from different perspectives.

## 🚀 PARALLEL EXECUTION ADVANTAGE

**CRITICAL**: This planner uses TRUE PARALLEL EXECUTION via Task tools, achieving:
- **5-8x faster analysis** than sequential examination
- **Independent specialist perspectives** running concurrently
- **No context pollution** between different analysis domains
- **Real parallelism**, not role-playing or narrative tricks

**Traditional Sequential**: Domain → Service → API → Infrastructure → Quality (30+ minutes)
**Our Parallel Approach**: All 5 analysts work simultaneously (5-6 minutes)

## HOW IT WORKS

```
Traditional:  [Domain]──►[Service]──►[API]──►[Infrastructure]──►[Quality]
                30min      25min     20min       20min           15min
              Total: ~110 minutes

Parallel:     [Domain Analyst    ] ┐
              [Service Analyst   ] ├─► All complete in ~6 minutes
              [API Analyst       ] │
              [Infrastructure    ] │
              [Quality Analyst   ] ┘
```

Parse the following arguments from "$ARGUMENTS":
1. `analysis_scope` - What to analyze (default: full)
   Options: full, domain-gaps, service-coverage, security-audit, performance-bottlenecks
   
2. `output_dir` - Directory for plan and specs (default: ./.claude/planning/)
   
3. `priority_focus` - Business priority for planning (default: balanced)
   Options: revenue-generation, compliance-critical, performance-optimization, reliability
   
4. `planning_depth` - How deep to analyze (default: thorough)
   Options: quick (1-hour sprint), thorough (half-day), exhaustive (full analysis)

## PHASE 1: PARALLEL CODEBASE ANALYSIS (5 Concurrent Tasks)

Deploy specialized analyst agents IN PARALLEL using Task tools:

**Spawn these 5 Tasks simultaneously:**

1. **Task: Domain Analyst**
   - Description: "Domain Analyst - Analyze domain coverage and gaps"
   - Analyzes: internal/domain/* for entities, value objects, DDD patterns
   - Outputs: Domain coverage report with gaps and opportunities

2. **Task: Service Analyst**
   - Description: "Service Analyst - Analyze service orchestration coverage"
   - Analyzes: internal/service/* for orchestration patterns and gaps
   - Outputs: Service layer assessment with missing orchestrations

3. **Task: API Analyst**
   - Description: "API Analyst - Analyze API surface and gaps"
   - Analyzes: internal/api/* for REST, gRPC, WebSocket coverage
   - Outputs: API completeness report with missing endpoints

4. **Task: Infrastructure Analyst**
   - Description: "Infrastructure Analyst - Analyze data and infrastructure patterns"
   - Analyzes: internal/infrastructure/* for repositories, caching, events
   - Outputs: Infrastructure assessment with performance insights

5. **Task: Quality Analyst**
   - Description: "Quality Analyst - Analyze testing, security, and compliance"
   - Analyzes: test coverage, security implementations, monitoring
   - Outputs: Quality assessment with improvement recommendations

Wait for ALL Tasks to complete before proceeding to Phase 2.

## PHASE 2: CONSOLIDATED ANALYSIS (Single Task)

After all parallel Tasks complete, spawn a consolidation Task:

**Task: Master Plan Consolidator**
- Description: "Consolidate all analyst reports into strategic insights"
- Inputs: All 5 analyst reports from Phase 1
- Process:
  - Synthesize findings across all domains
  - Identify cross-cutting opportunities
  - Map dependencies between improvements
  - Prioritize based on business impact
- Output: Consolidated opportunity matrix

## PHASE 3: PARALLEL SPECIFICATION GENERATION

Based on identified opportunities, spawn multiple Tasks to generate specifications:

**Spawn Tasks in batches based on priority:**

**Batch 1 - Critical Features (3-5 Parallel Tasks):**
- Task per critical feature specification
- Each Task generates complete feature spec
- Include domain models, services, APIs, testing

**Batch 2 - High Priority Features (3-5 Parallel Tasks):**
- Next tier of feature specifications
- Can run after Batch 1 or in parallel if independent

**Batch 3 - Enhancement Features (3-5 Parallel Tasks):**
- Performance, monitoring, and polish features

## PHASE 4: MASTER PLAN ASSEMBLY

Single Task to assemble all outputs:

**Task: Plan Assembler**
- Combines all analyst reports
- Orders feature specifications by dependencies
- Creates master-plan.md with:
  - Executive summary from parallel analyses
  - Architecture health scores per analyst
  - Prioritized feature phases
  - Resource requirements
  - Risk assessments from all perspectives

## PHASE 5: EXECUTION ORCHESTRATION

Generate parallel execution plan:

**Output Files:**
1. `master-plan.md` - Consolidated findings from all parallel analysts
2. `specs/*.md` - Feature specifications (generated in parallel)
3. `execute-plan.sh` - Commands showing parallel execution opportunities
4. `plan-status.md` - Tracking dashboard for parallel work streams

## EXECUTION PATTERN

```
Phase 1: [Analyst1] [Analyst2] [Analyst3] [Analyst4] [Analyst5]
           ↓         ↓         ↓         ↓         ↓
Phase 2: [============ Consolidator ================]
                          ↓
Phase 3: [Spec1] [Spec2] [Spec3] ... (Parallel generation)
           ↓       ↓       ↓
Phase 4: [====== Plan Assembler ======]
                    ↓
         Complete Master Plan with Parallel Execution Strategy
```

## Related Documentation

- **[../COMMAND_REFERENCE.md](../COMMAND_REFERENCE.md)** - Complete command reference
- **[../docs/WORKFLOWS.md](../docs/WORKFLOWS.md)** - Master planning in context
- **[../docs/TROUBLESHOOTING.md](../docs/TROUBLESHOOTING.md)** - Solutions for planning issues

Begin orchestration with PARALLEL Task deployment. Monitor all Tasks and ensure complete execution before proceeding to next phase.