# 🏗️ DCE AI Agent System Architecture

This document provides a comprehensive technical overview of the DCE AI Agent System's architecture, design principles, and implementation details.

## 📋 Table of Contents

1. [System Overview](#system-overview)
2. [Core Architecture](#core-architecture)
3. [Parallel Execution Engine](#parallel-execution-engine)
4. [State Management System](#state-management-system)
5. [Command Processing Pipeline](#command-processing-pipeline)
6. [Wave-Based Orchestration](#wave-based-orchestration)
7. [Context Bridge System](#context-bridge-system)
8. [Performance Architecture](#performance-architecture)
9. [Security & Permissions](#security--permissions)
10. [Extension Points](#extension-points)

## System Overview

The DCE AI Agent System is a **parallel execution framework** built on top of Claude Code that achieves 5-8x performance improvements through true concurrent Task processing.

### Key Architectural Principles

1. **True Parallelism**: Leverages Claude's Task tool for real concurrent execution
2. **Wave-Based Dependencies**: Manages complex dependencies through orchestrated waves
3. **State Persistence**: Maintains comprehensive state for resumability and efficiency
4. **Context Preservation**: Seamlessly bridges planning and implementation phases
5. **Incremental Processing**: Caches results for 90% faster re-runs

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User Interface Layer                      │
│                 (Slash Commands & CLI)                       │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│                 Command Processing Layer                     │
│         (Parsing, Validation, Mode Detection)                │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│              Orchestration Engine Layer                      │
│        (Wave Management, Dependency Resolution)              │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│              Parallel Execution Layer                        │
│            (Task Tool, Agent Spawning)                       │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────────┐
│               State Management Layer                         │
│         (Persistence, Caching, Recovery)                     │
└─────────────────────────────────────────────────────────────┘
```

## Core Architecture

### Component Diagram

```
.claude/
├── commands/              # Command Definitions
│   ├── dce-master-plan   ├─→ Strategic Planner
│   ├── dce-feature       ├─→ Feature Builder
│   └── dce-check-work    └─→ Quality Validator
│
├── prompts/specialists/   # AI Agent Templates
│   ├── entity-architect  ├─→ Domain Specialist
│   ├── api-designer      ├─→ API Specialist
│   └── test-engineer     └─→ Testing Specialist
│
├── state/                 # Persistent State
│   ├── feature-progress  ├─→ Execution Tracker
│   └── system-snapshot   └─→ Health Monitor
│
└── context/              # Execution Context
    ├── feature-context   ├─→ Current Feature
    └── execution-queue   └─→ Task Queue
```

### Key Components

#### 1. Command Processor
- **Purpose**: Parse and validate user commands
- **Technology**: Markdown-based command definitions
- **Features**:
  - Argument validation
  - Mode detection (standalone vs handoff)
  - Permission checking

#### 2. Orchestration Engine
- **Purpose**: Manage execution flow and dependencies
- **Technology**: Wave-based task scheduling
- **Features**:
  - Dependency graph resolution
  - Parallel wave execution
  - Progress tracking

#### 3. Task Execution Engine
- **Purpose**: Spawn and manage parallel AI agents
- **Technology**: Claude's native Task tool
- **Features**:
  - True concurrent execution
  - Agent lifecycle management
  - Result aggregation

#### 4. State Manager
- **Purpose**: Persist and retrieve system state
- **Technology**: YAML-based state files
- **Features**:
  - Atomic updates
  - Crash recovery
  - Performance caching

## Parallel Execution Engine

### Task Tool Integration

The system leverages Claude's Task tool for true parallelism:

```python
# Conceptual representation
class ParallelExecutor:
    def execute_wave(self, wave_tasks):
        """Execute multiple tasks in parallel"""
        task_handles = []
        
        for task in wave_tasks:
            # Each Task runs as independent agent
            handle = Task.spawn(
                prompt=task.specialist_prompt,
                context=task.execution_context,
                timeout=task.timeout
            )
            task_handles.append(handle)
        
        # Wait for all tasks in wave
        results = Task.wait_all(task_handles)
        return self.aggregate_results(results)
```

### Performance Characteristics

| Metric | Sequential | Parallel | Improvement |
|--------|------------|----------|-------------|
| 5-task execution | 50 min | 10 min | 5x |
| 20-task feature | 200 min | 25 min | 8x |
| Context switching | High | None | ∞ |
| Memory usage | Low | Medium | Acceptable |

### Agent Specialization

Each parallel agent is specialized for its domain:

1. **Entity Architect**: Domain modeling, value objects
2. **Service Orchestrator**: Business logic coordination
3. **API Designer**: REST/gRPC endpoint design
4. **Test Engineer**: Comprehensive test coverage
5. **Infrastructure Specialist**: Database, caching, config

## State Management System

### State File Architecture

```yaml
# state/feature-progress.yaml
current_feature:
  id: "consent-management-v2"
  status: "in_progress"
  started_at: "2024-01-15T10:00:00Z"
  
waves_completed:
  - wave: 1
    status: "complete"
    duration: "10m 23s"
    tasks:
      - name: "Entity Design"
        status: "complete"
        artifacts: ["domain/consent.go"]
      
current_wave:
  wave: 2
  status: "in_progress"
  started_at: "2024-01-15T10:15:00Z"
  tasks:
    - name: "Service Layer"
      status: "running"
      agent_id: "task-123"
```

### State Transitions

```
┌─────────┐     ┌────────────┐     ┌──────────┐
│  IDLE   │────▶│  PLANNING  │────▶│ BUILDING │
└─────────┘     └────────────┘     └──────────┘
                      │                   │
                      ▼                   ▼
                ┌────────────┐     ┌──────────┐
                │  HANDOFF   │     │ COMPLETE │
                └────────────┘     └──────────┘
```

### Caching Strategy

The system implements multi-level caching:

1. **Analysis Cache**: Codebase analysis results (24h TTL)
2. **Dependency Cache**: Resolved dependencies (1h TTL)
3. **Template Cache**: Compiled agent prompts (∞ TTL)
4. **Result Cache**: Completed task outputs (7d TTL)

## Command Processing Pipeline

### Pipeline Stages

```
User Input
    │
    ▼
┌─────────────────┐
│ Parse Command   │──→ Extract arguments, validate syntax
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Load Context    │──→ Read state files, detect mode
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Plan Execution  │──→ Create waves, resolve dependencies
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Execute Waves   │──→ Spawn parallel Tasks
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Aggregate       │──→ Collect results, update state
└────────┬────────┘
         │
         ▼
    Output
```

### Mode Detection

The system automatically detects execution context:

```python
def detect_mode():
    if exists("context/feature-context.yaml"):
        return "handoff"  # From master-plan
    elif exists("state/feature-progress.yaml"):
        return "resume"   # Continuing work
    else:
        return "fresh"    # New execution
```

## Wave-Based Orchestration

### Wave Definition

Waves ensure proper dependency management:

```yaml
# Wave configuration
waves:
  - id: 1
    name: "Foundation"
    parallel_tasks:
      - entity_design
      - value_objects
      - repository_interfaces
    dependencies: []
    
  - id: 2
    name: "Business Logic"
    parallel_tasks:
      - service_layer
      - validation_rules
      - event_handlers
    dependencies: [1]  # Requires wave 1
    
  - id: 3
    name: "Integration"
    parallel_tasks:
      - api_endpoints
      - database_impl
      - cache_layer
    dependencies: [1, 2]  # Requires waves 1 & 2
```

### Dependency Resolution

```
Wave 1: [A, B, C] ──────────────┐
                                │
Wave 2: [D, E]    ──────┐       │
                        ▼       ▼
Wave 3: [F, G, H] ──────────────┤
                                │
Wave 4: [I]       ◄─────────────┘
```

## Context Bridge System

### Planning to Implementation

The context bridge automatically transforms planning outputs:

```
Planning Outputs                  Bridge Transform              Feature Inputs
────────────────                  ────────────────              ──────────────
specs/feature.md        ──────→   Parse & Extract    ──────→   feature-context.yaml
reports/analysis.md     ──────→   Aggregate Data     ──────→   implementation-plan.md
dependencies.json       ──────→   Build Queue        ──────→   execution-queue.yaml
```

### Context Preservation

Key information preserved across phases:
- Technical requirements
- Performance constraints
- Dependency relationships
- Architecture decisions
- Risk assessments

## Performance Architecture

### Optimization Strategies

1. **Parallel by Default**: Maximum concurrent execution
2. **Smart Caching**: Reuse previous analysis results
3. **Incremental Updates**: Only process changes
4. **Resource Pooling**: Reuse agent connections
5. **Async I/O**: Non-blocking file operations

### Performance Metrics

```yaml
# monitoring/metrics.db structure
performance_metrics:
  command_execution:
    dce_master_plan:
      avg_duration: "15m 30s"
      p95_duration: "18m 45s"
      parallel_efficiency: 0.85
      
    dce_feature:
      avg_duration: "12m 15s"
      p95_duration: "15m 00s"
      parallel_efficiency: 0.92
      
  resource_usage:
    peak_parallel_tasks: 8
    avg_memory_per_task: "150MB"
    cpu_utilization: "60%"
```

### Bottleneck Analysis

Common bottlenecks and solutions:

| Bottleneck | Impact | Solution |
|------------|--------|----------|
| Task startup | 2-3s per agent | Pre-warm agent pool |
| State I/O | 100-500ms | Async writes, caching |
| Large outputs | Memory pressure | Stream processing |
| Dependencies | Serialization | Better wave planning |

## Security & Permissions

### Permission Model

```json
{
  "permissions": {
    "allow": ["Write", "MultiEdit", "Edit", "Bash"],
    "deny": ["Delete", "SystemExecute"]
  },
  "constraints": {
    "max_parallel_tasks": 10,
    "max_file_size": "10MB",
    "allowed_directories": ["./src", "./test"]
  }
}
```

### Security Layers

1. **Command Validation**: Syntax and argument checking
2. **Permission Checking**: Tool access control
3. **Resource Limits**: Memory and CPU constraints
4. **Audit Logging**: All operations tracked
5. **Sandboxing**: Agent isolation

## Extension Points

### Adding New Commands

1. Create command definition in `commands/`
2. Define argument schema
3. Implement command logic
4. Add to command reference

### Custom Specialists

1. Create specialist prompt in `prompts/specialists/`
2. Define expertise area
3. Add to wave configuration
4. Test parallel execution

### State Extensions

1. Define new state schema
2. Add to state management
3. Implement persistence logic
4. Update recovery procedures

## Future Architecture Directions

### Planned Enhancements

1. **Distributed Execution**: Multi-machine parallelism
2. **Smart Scheduling**: ML-based task optimization
3. **Real-time Collaboration**: Multiple users/agents
4. **Plugin System**: External tool integration
5. **Visual Monitoring**: Real-time execution dashboard

### Scalability Roadmap

- **Phase 1**: Current (5-10 parallel tasks)
- **Phase 2**: Enhanced (20-50 parallel tasks)
- **Phase 3**: Distributed (100+ parallel tasks)
- **Phase 4**: Cloud-native (unlimited scale)

## Architecture Decision Records

### ADR-001: Task Tool for Parallelism
- **Decision**: Use Claude's Task tool instead of simulated parallelism
- **Rationale**: True concurrent execution, 5-8x performance gain
- **Trade-offs**: Higher memory usage, complexity

### ADR-002: YAML for State Management
- **Decision**: Use YAML files for state persistence
- **Rationale**: Human-readable, merge-friendly, simple
- **Trade-offs**: Parsing overhead, size limitations

### ADR-003: Wave-Based Orchestration
- **Decision**: Organize work into dependency waves
- **Rationale**: Ensures correct execution order
- **Trade-offs**: Some serialization points

---

For implementation details, see:
- [PARALLEL_EXECUTION.md](./PARALLEL_EXECUTION.md) - Deep dive into Task system
- [STATE_MANAGEMENT.md](./STATE_MANAGEMENT.md) - State implementation
- [AI_AGENT_GUIDE.md](./AI_AGENT_GUIDE.md) - Agent architecture