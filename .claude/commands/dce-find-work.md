# DCE Smart Work Discovery

Intelligently find implementation-ready work based on various criteria including business priority, technical readiness, team availability, and dependencies.

## Usage

```bash
/dce-find-work [options]
```

## Options

- `--ready` - Find features with no blockers that are ready to implement
- `--no-blockers` - Same as --ready
- `--criteria="<query>"` - Complex query for work discovery
  - Examples:
    - `"revenue-critical AND effort:low"`
    - `"compliance-required AND ready"`
    - `"priority:critical AND team:backend"`
- `--technical="<filter>"` - Technical criteria
  - Examples:
    - `"missing-tests"`
    - `"performance-issues"`
    - `"security-gaps"`
- `--team="<team>"` - Filter by team assignment
  - Options: backend, frontend, devops, fullstack
- `--capacity="<hours>"` - Find work fitting team capacity
- `--depends-on="<feature>"` - Find work depending on a feature
- `--status="<status>"` - Filter by dependency status
  - Options: completed, in-progress, pending

## Execution Flow

1. **Load State**: Read current system state and feature progress
2. **Apply Filters**: Process query criteria and filters
3. **Calculate Readiness**: Score each feature based on:
   - Dependency completion
   - Blocker resolution
   - Team availability
   - Business priority
4. **Generate Report**: Ranked list of work items with:
   - Implementation readiness score
   - Effort estimates
   - Dependency status
   - Next steps

## Output Format

```yaml
work_discovery_results:
  query: "revenue-critical AND effort:low"
  timestamp: "2025-01-15T10:00:00Z"
  total_matches: 5
  
  ready_work:
    - id: "AUTH-001"
      name: "Enhanced Authentication"
      readiness_score: 0.95
      business_value: "high"
      effort_estimate: "2 weeks"
      team_required: ["backend"]
      dependencies_met: true
      blockers: []
      next_steps:
        - "Run /dce-feature AUTH-001"
        - "Assign to backend team"
      
    - id: "CACHE-001"
      name: "Advanced Caching Layer"
      readiness_score: 0.90
      business_value: "medium"
      effort_estimate: "3 weeks"
      team_required: ["backend", "devops"]
      dependencies_met: true
      blockers: []
      
  blocked_work:
    - id: "FINANCE-001"
      name: "Financial Service"
      readiness_score: 0.30
      blockers: ["audit_logging_not_complete"]
      blocking_feature: "AUDIT-001"
      estimated_unblock_date: "2025-02-01"
```

## Implementation

Parse arguments and execute work discovery logic:

1. Load `.claude/state/system-snapshot.yaml`
2. Load `.claude/state/feature-progress.yaml`
3. Load `.claude/state/dependency-graph.yaml`
4. Load `.claude/context/execution-queue.yaml`

Apply filters based on provided criteria and generate a prioritized work list that helps teams find the most valuable work they can start immediately.