# 📚 .claude Directory Index - Master Navigation

Welcome to the DCE AI Agent System! This index helps you navigate the powerful automation tools in the `.claude` directory.

## 🚀 Start Here

If you're new to the system:
1. **[QUICKSTART.md](./QUICKSTART.md)** - 5-minute introduction to get you running
2. **[README.md](./README.md)** - Comprehensive system overview
3. **[COMMAND_REFERENCE.md](./COMMAND_REFERENCE.md)** - All commands at a glance

## 📁 Directory Structure

```
.claude/
├── 📋 Navigation & Getting Started
│   ├── INDEX.md                    # You are here
│   ├── QUICKSTART.md              # 5-minute introduction
│   ├── README.md                  # Comprehensive overview
│   └── COMMAND_REFERENCE.md       # All commands reference
│
├── 📂 commands/                   # Command implementations
│   ├── dce-master-plan.md        # Strategic planning
│   ├── dce-feature.md            # Feature implementation
│   ├── dce-check-work.md         # Quality review
│   ├── dce-research.md           # Web research
│   └── [more commands...]
│
├── 📂 docs/                       # Core documentation
│   ├── AI_AGENT_GUIDE.md         # Comprehensive agent guide
│   ├── ARCHITECTURE.md           # System architecture
│   ├── PARALLEL_EXECUTION.md     # Parallel execution details
│   ├── STATE_MANAGEMENT.md       # State persistence
│   ├── HANDOFF_WORKFLOW.md       # Planning → Implementation
│   ├── TROUBLESHOOTING.md        # Problem solutions
│   └── WORKFLOWS.md              # End-to-end examples
│
├── 📂 state/                      # Persistent state files
│   ├── system-snapshot.yaml      # System health
│   ├── feature-progress.yaml     # Implementation tracking
│   └── [state files...]
│
├── 📂 context/                    # Execution contexts
│   ├── feature-context.yaml      # Current feature config
│   └── execution-queue.yaml      # Work queue
│
├── 📂 planning/                   # Planning outputs
│   ├── master-plan.md            # Strategic roadmap
│   ├── specs/                    # Feature specifications
│   └── reports/                  # Analysis reports
│
└── 📂 prompts/specialists/        # Task specialists
    ├── entity-architect.md       # Domain modeling
    ├── service-orchestrator.md   # Service design
    └── [more specialists...]
```

## 🎯 Core Concepts

### 1. **True Parallel Execution**
- Uses Task tool for real concurrent processing
- 5-8x performance improvement over sequential execution
- See: [PARALLEL_EXECUTION.md](./docs/PARALLEL_EXECUTION.md)

### 2. **Wave-Based Orchestration**
- Work organized into dependency-aware waves
- Each wave executes multiple tasks in parallel
- See: [AI_AGENT_GUIDE.md](./docs/AI_AGENT_GUIDE.md)

### 3. **State Persistence**
- Automatic saving and resumption of work
- Incremental analysis (90% faster re-runs)
- See: [STATE_MANAGEMENT.md](./docs/STATE_MANAGEMENT.md)

### 4. **Seamless Handoffs**
- Planning outputs automatically become feature inputs
- Zero manual translation required
- See: [HANDOFF_WORKFLOW.md](./docs/HANDOFF_WORKFLOW.md)

## 🛠️ Essential Commands

| Command | Purpose | Common Usage |
|---------|---------|--------------|
| `/dce-master-plan` | Strategic analysis & planning | Start new projects |
| `/dce-feature` | Implement specifications | Build features |
| `/dce-check-work` | Review for gaps & issues | Quality control |
| `/dce-find-work` | Discover ready tasks | Task selection |
| `/dce-research` | Web research for solutions | Problem solving |

For complete command documentation, see [COMMAND_REFERENCE.md](./COMMAND_REFERENCE.md)

## 📖 Reading Paths

### For Feature Development
1. [QUICKSTART.md](./QUICKSTART.md) → Quick orientation
2. [commands/dce-master-plan.md](./commands/dce-master-plan.md) → Planning
3. [commands/dce-feature.md](./commands/dce-feature.md) → Implementation
4. [docs/HANDOFF_WORKFLOW.md](./docs/HANDOFF_WORKFLOW.md) → Understanding handoffs

### For System Administration
1. [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) → Technical overview
2. [docs/STATE_MANAGEMENT.md](./docs/STATE_MANAGEMENT.md) → State system
3. [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) → Problem solving
4. [my-project-instructions.md](./my-project-instructions.md) → Project specifics

### For Performance Optimization
1. [docs/PARALLEL_EXECUTION.md](./docs/PARALLEL_EXECUTION.md) → Parallelism details
2. [state/performance-metrics.yaml](./state/performance-metrics.yaml) → Metrics
3. [docs/automated-testing-findings.md](./docs/automated-testing-findings.md) → Performance insights

## 🔍 Quick Reference

### State Files
- **Current feature**: `context/feature-context.yaml`
- **Progress tracking**: `state/feature-progress.yaml`
- **Work queue**: `context/execution-queue.yaml`
- **System health**: `state/system-snapshot.yaml`

### Configuration
- **Permissions**: `settings.json` - Tool permissions
- **Local settings**: `settings.local.json` - User overrides
- **Project instructions**: `my-project-instructions.md` - Custom patterns

### Archives & History
- **Archived docs**: `archives/` - Historical documentation
- **Improvement guides**: `archives/docs/SYSTEM_IMPROVEMENT_GUIDE.md`
- **Previous reports**: `archives/readme-update-summary.md`

## 🚦 Getting Help

1. **Common Issues**: See [TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md)
2. **Command Help**: Each command file has detailed documentation
3. **Workflows**: See [WORKFLOWS.md](./docs/WORKFLOWS.md) for examples
4. **Architecture**: See [ARCHITECTURE.md](./docs/ARCHITECTURE.md) for deep dives

## 📊 System Status Indicators

Check these files to understand current system state:
- `state/system-snapshot.yaml` - Overall health
- `state/feature-progress.yaml` - Active work
- `state/analysis-history.yaml` - Cached analyses
- `monitoring/metrics.db` - Performance history

## 🔗 Related Documentation

- **Project README**: [`../README.md`](../README.md) - Overall project docs
- **Project CLAUDE.md**: [`../CLAUDE.md`](../CLAUDE.md) - Project AI instructions
- **Quick Reference**: [`../QUICK_REFERENCE.md`](../QUICK_REFERENCE.md) - Project commands

---

💡 **Pro Tip**: Start with [QUICKSTART.md](./QUICKSTART.md) for the fastest path to productivity!