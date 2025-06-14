# 📖 DCE AI Agent System - Command Reference

This reference provides comprehensive documentation for all commands in the DCE AI Agent System, organized by category and use case.

## 🎯 Quick Command Finder

| Need | Command | Example |
|------|---------|---------|
| Plan a project | [`/dce-master-plan`](#dce-master-plan) | `/dce-master-plan full ./.claude/planning balanced thorough` |
| Build a feature | [`/dce-feature`](#dce-feature) | `/dce-feature ./specs/feature.md . adaptive production` |
| Find next task | [`/dce-find-work`](#dce-find-work) | `/dce-find-work --ready` |
| Check quality | [`/dce-check-work`](#dce-check-work) | `/dce-check-work` |
| Research solutions | [`/dce-research`](#dce-research) | `/dce-research "Go performance optimization"` |
| Resume work | [`/dce-feature-resume`](#dce-feature-resume) | `/dce-feature-resume consent-management-v2` |
| System status | [`/dce-system-status`](#dce-system-status) | `/dce-system-status` |
| Performance analysis | [`/dce-system-improve`](#dce-system-improve) | `/dce-system-improve --phase=5` |

## 📚 Command Categories

### 🏗️ Core Feature Development

#### `/dce-master-plan`
**Purpose**: Strategic project analysis and planning with 5 parallel AI agents

**Syntax**:
```bash
/dce-master-plan <analysis-mode> <output-dir> <priority-focus> <analysis-depth>
```

**Parameters**:
- `analysis-mode`: `full` | `targeted` | `quick`
- `output-dir`: Directory for planning outputs (e.g., `./.claude/planning`)
- `priority-focus`: `balanced` | `compliance-critical` | `performance-critical` | `scalability`
- `analysis-depth`: `thorough` | `standard` | `quick`

**Examples**:
```bash
# Full project analysis
/dce-master-plan full ./.claude/planning balanced thorough

# Quick compliance check
/dce-master-plan targeted ./.claude/planning compliance-critical quick

# Performance-focused planning
/dce-master-plan full ./.claude/planning performance-critical standard
```

**Output**:
- Master plan roadmap in `planning/master-plan.md`
- Feature specifications in `planning/specs/`
- Analysis reports in `planning/reports/`
- Handoff files for feature implementation

---

#### `/dce-feature`
**Purpose**: Implement features from specifications using parallel Task execution

**Syntax**:
```bash
/dce-feature <spec-file> <project-root> <execution-mode> <environment>
```

**Parameters**:
- `spec-file`: Path to feature specification (`.md` or `.yaml`)
- `project-root`: Project root directory (usually `.`)
- `execution-mode`: `adaptive` | `fast` | `thorough`
- `environment`: `production` | `staging` | `development`

**Examples**:
```bash
# Standard feature implementation
/dce-feature ./planning/specs/consent-management.md . adaptive production

# Fast prototype mode
/dce-feature ./specs/quick-feature.md . fast development

# Thorough implementation with extra validation
/dce-feature ./critical-feature-spec.md . thorough production
```

**Execution Modes**:
- `adaptive`: Balances speed and quality (recommended)
- `fast`: Prioritizes speed, minimal validation
- `thorough`: Maximum validation and testing

---

#### `/dce-feature-resume`
**Purpose**: Resume interrupted feature implementation from saved state

**Syntax**:
```bash
/dce-feature-resume <feature-id>
```

**Parameters**:
- `feature-id`: Feature identifier from `state/feature-progress.yaml`

**Examples**:
```bash
# Resume specific feature
/dce-feature-resume consent-management-v2

# Resume with status check
/dce-feature-resume payment-processing --check-first
```

---

### 🔍 Work Discovery & Management

#### `/dce-find-work`
**Purpose**: Discover ready-to-implement tasks from planning outputs

**Syntax**:
```bash
/dce-find-work [options]
```

**Options**:
- `--ready`: Show only unblocked tasks
- `--all`: Show all tasks including blocked
- `--priority=<level>`: Filter by priority (high|medium|low)
- `--estimate=<hours>`: Filter by time estimate

**Examples**:
```bash
# Find ready work
/dce-find-work --ready

# Show all high-priority tasks
/dce-find-work --priority=high

# Find quick wins (< 4 hours)
/dce-find-work --ready --estimate=4
```

**Output Format**:
```
📋 Ready to Implement:
1. Enhanced Validation Engine (8h) - Priority: HIGH
2. Parallel Processing Framework (12h) - Priority: HIGH
3. Monitoring Dashboard (4h) - Priority: MEDIUM

🚧 Blocked (3 tasks waiting on dependencies)
```

---

#### `/dce-check-work`
**Purpose**: Review completed work for gaps, issues, and quality

**Syntax**:
```bash
/dce-check-work [options]
```

**Options**:
- `--feature=<name>`: Check specific feature
- `--all`: Check all recent work
- `--fix`: Auto-fix minor issues

**Examples**:
```bash
# Check current work
/dce-check-work

# Check specific feature
/dce-check-work --feature=consent-management

# Check and fix minor issues
/dce-check-work --fix
```

---

### 🔬 Research & Analysis

#### `/dce-research`
**Purpose**: Research technical solutions using web search

**Syntax**:
```bash
/dce-research "<query>" [options]
```

**Options**:
- `--deep`: Extensive research with multiple sources
- `--code-examples`: Focus on code implementations
- `--performance`: Focus on performance comparisons

**Examples**:
```bash
# Basic research
/dce-research "Go channels vs goroutines performance"

# Deep research with examples
/dce-research "TCPA compliance implementation patterns" --deep --code-examples

# Performance comparison
/dce-research "PostgreSQL vs MongoDB for time series" --performance
```

---

### 🛠️ System Management

#### `/dce-system-status`
**Purpose**: Display current system status and health metrics

**Syntax**:
```bash
/dce-system-status [component]
```

**Components**:
- `all`: Complete system overview (default)
- `planning`: Planning phase status
- `features`: Active feature implementations
- `performance`: Performance metrics
- `state`: State file integrity

**Examples**:
```bash
# Full system status
/dce-system-status

# Check feature progress
/dce-system-status features

# Performance metrics
/dce-system-status performance
```

---

#### `/dce-system-improve`
**Purpose**: Analyze and improve system performance

**Syntax**:
```bash
/dce-system-improve --phase=<number> [options]
```

**Phases**:
1. Performance baseline measurement
2. Bottleneck identification
3. Optimization recommendations
4. Implementation of improvements
5. Validation and reporting

**Examples**:
```bash
# Run complete improvement cycle
/dce-system-improve --phase=all

# Just analyze performance
/dce-system-improve --phase=1

# Generate optimization report
/dce-system-improve --phase=3 --report
```

---

### 🚀 Advanced Commands

#### `/dce-parallel-test`
**Purpose**: Test parallel execution capabilities

**Syntax**:
```bash
/dce-parallel-test <test-type> <iterations>
```

**Test Types**:
- `basic`: Simple parallel task execution
- `complex`: Multi-wave dependency testing
- `stress`: Maximum parallel load testing

---

#### `/dce-context-bridge`
**Purpose**: Manually create handoff files between planning and implementation

**Syntax**:
```bash
/dce-context-bridge <planning-dir> <feature-name>
```

---

#### `/dce-state-repair`
**Purpose**: Repair corrupted state files

**Syntax**:
```bash
/dce-state-repair [--backup] [--force]
```

---

## 🔄 Workflow Commands

### Complete Feature Workflow
```bash
# 1. Strategic planning
/dce-master-plan full ./.claude/planning balanced thorough

# 2. Find ready work
/dce-find-work --ready

# 3. Implement top priority feature
/dce-feature ./planning/specs/top-priority.md . adaptive production

# 4. Check implementation quality
/dce-check-work

# 5. System performance check
/dce-system-status performance
```

### Quick Iteration Workflow
```bash
# 1. Quick planning for specific area
/dce-master-plan targeted ./.claude/planning performance-critical quick

# 2. Direct feature implementation
/dce-feature ./specs/performance-fix.md . fast development

# 3. Validate changes
/dce-check-work --feature=performance-fix
```

### Research-Driven Development
```bash
# 1. Research best practices
/dce-research "microservices error handling patterns" --deep

# 2. Generate implementation plan
/dce-master-plan targeted ./planning error-handling standard

# 3. Implement with research context
/dce-feature ./planning/specs/error-handling.md . thorough production
```

## ⚙️ Command Options & Flags

### Global Options
Available for all commands:
- `--verbose`: Detailed output
- `--quiet`: Minimal output
- `--dry-run`: Preview without execution
- `--config=<file>`: Use custom configuration
- `--timeout=<seconds>`: Override default timeout

### Execution Modes

| Mode | Speed | Quality | Use Case |
|------|-------|---------|----------|
| `adaptive` | Medium | High | Most features (default) |
| `fast` | High | Medium | Prototypes, experiments |
| `thorough` | Low | Maximum | Critical features |
| `production` | Medium | Maximum | Production deployments |

### Priority Levels

| Level | Description | Response Time |
|-------|-------------|---------------|
| `critical` | System breaking | Immediate |
| `high` | Major feature | < 1 day |
| `medium` | Enhancement | < 1 week |
| `low` | Nice to have | As available |

## 📁 State Files Reference

### Key State Files
- `state/feature-progress.yaml` - Active feature tracking
- `state/system-snapshot.yaml` - System health metrics
- `context/feature-context.yaml` - Current feature configuration
- `context/execution-queue.yaml` - Task queue and dependencies

### Monitoring State
```bash
# Watch feature progress
watch cat .claude/state/feature-progress.yaml

# Monitor system health
tail -f .claude/state/system-snapshot.yaml
```

## 🚨 Troubleshooting

### Command Not Found
```bash
# Ensure you're in project root
pwd  # Should show project directory

# Check command file exists
ls .claude/commands/
```

### Timeout Issues
```bash
# Use shorter depth for planning
/dce-master-plan full ./planning balanced quick

# Increase timeout for complex features
/dce-feature ./spec.md . thorough production --timeout=1800
```

### State Corruption
```bash
# Backup and repair
cp -r .claude/state .claude/state.backup
/dce-state-repair --force
```

### Missing Context
```bash
# Regenerate handoff files
/dce-context-bridge ./planning consent-management

# Verify context files
ls -la .claude/context/
```

## 📊 Performance Benchmarks

| Command | Typical Duration | Parallel Tasks |
|---------|------------------|----------------|
| master-plan (full) | 15-20 min | 5 agents |
| master-plan (quick) | 5-7 min | 5 agents |
| feature (adaptive) | 10-15 min | 15-20 tasks |
| feature (fast) | 5-8 min | 10-15 tasks |
| check-work | 2-3 min | 3-5 validators |
| research | 1-2 min | 1 task |

## 🔗 Related Documentation

- [QUICKSTART.md](./QUICKSTART.md) - 5-minute introduction
- [AI_AGENT_GUIDE.md](./docs/AI_AGENT_GUIDE.md) - Deep dive into Task system
- [PARALLEL_EXECUTION.md](./docs/PARALLEL_EXECUTION.md) - Parallelism details
- [STATE_MANAGEMENT.md](./docs/STATE_MANAGEMENT.md) - State system
- [TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) - Common issues

---

💡 **Pro Tips**:
1. Always run `/dce-check-work` before committing
2. Use `adaptive` mode for best results
3. Let planning complete fully for optimal handoff
4. Monitor `feature-progress.yaml` during execution
5. Research before implementing complex features

Need help? Check [TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) or run `/dce-system-status` for diagnostics.