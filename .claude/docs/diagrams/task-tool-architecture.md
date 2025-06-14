# Claude Task Tool Architecture

## Overview

This document illustrates the technical architecture of Claude's Task tool spawning mechanism, demonstrating how true parallel execution is achieved through independent Task instances.

## 1. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Claude Main Orchestrator                         │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │                     Task Spawning Engine                         │  │
│  │  • Parses user request                                          │  │
│  │  • Identifies parallelizable work                               │  │
│  │  • Spawns independent Task instances                            │  │
│  │  • Manages lifecycle and aggregation                            │  │
│  └─────────────────────────────────────────────────────────────────┘  │
└─────────────────────────┬───────────────────────────────────────────────┘
                          │ Spawns Tasks
    ┌─────────────────────┼─────────────────────┐
    │                     │                     │
    ▼                     ▼                     ▼
┌─────────┐         ┌─────────┐         ┌─────────┐
│ Task #1 │         │ Task #2 │         │ Task #3 │
│ Process │         │ Process │         │ Process │
├─────────┤         ├─────────┤         ├─────────┤
│ Memory  │         │ Memory  │         │ Memory  │
│ Space 1 │         │ Space 2 │         │ Space 3 │
├─────────┤         ├─────────┤         ├─────────┤
│ Tools   │         │ Tools   │         │ Tools   │
│ Access  │         │ Access  │         │ Access  │
└─────────┘         └─────────┘         └─────────┘
     │                   │                   │
     └───────────────────┴───────────────────┘
                         │
                    File System
                 (Shared Context)
```

## 2. Task Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Spawn: User Request
    
    Spawn --> Execute: Task Created
    note right of Spawn
        - Independent process
        - Isolated memory
        - Own tool access
    end note
    
    Execute --> Working: Start Processing
    
    Working --> Working: Tool Invocations
    note right of Working
        - Read files
        - Write files
        - Execute commands
        - No direct IPC
    end note
    
    Working --> Complete: Task Finished
    
    Complete --> Aggregate: Return Results
    note left of Aggregate
        - Results collected
        - Context merged
        - Response formed
    end note
    
    Aggregate --> [*]: User Response
```

## 3. Memory Isolation Model

```
┌──────────────────────────────────────────────────────────────┐
│                     Operating System                          │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │   Task #1       │  │   Task #2       │  │   Task #3   │ │
│  │                 │  │                 │  │             │ │
│  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────┐ │ │
│  │ │   Memory    │ │  │ │   Memory    │ │  │ │ Memory  │ │ │
│  │ │   Space     │ │  │ │   Space     │ │  │ │ Space   │ │ │
│  │ │             │ │  │ │             │ │  │ │         │ │ │
│  │ │ • Variables │ │  │ │ • Variables │ │  │ │ • Vars  │ │ │
│  │ │ • State     │ │  │ │ • State     │ │  │ │ • State │ │ │
│  │ │ • Stack     │ │  │ │ • Stack     │ │  │ │ • Stack │ │ │
│  │ └─────────────┘ │  │ └─────────────┘ │  │ └─────────┘ │ │
│  │                 │  │                 │  │             │ │
│  │   ❌ No IPC    │  │   ❌ No IPC    │  │  ❌ No IPC │ │
│  │   ❌ No Shared │  │   ❌ No Shared │  │  ❌ No     │ │
│  │      Memory    │  │      Memory    │  │    Shared   │ │
│  └────────┬────────┘  └────────┬────────┘  └──────┬──────┘ │
│           │                    │                   │         │
│           └────────────────────┼───────────────────┘         │
│                               │                              │
│                          File System                         │
│                     ✓ Shared Access                         │
└──────────────────────────────────────────────────────────────┘
```

## 4. Context Sharing Through Files

```mermaid
graph TB
    subgraph "Claude Orchestrator"
        O[Main Process]
    end
    
    subgraph "File System"
        F1[context.md]
        F2[results.json]
        F3[shared_data.txt]
        F4[task_status.log]
    end
    
    subgraph "Task Instances"
        T1[Task 1]
        T2[Task 2]
        T3[Task 3]
    end
    
    O -->|"Write Initial Context"| F1
    
    T1 -->|Read| F1
    T2 -->|Read| F1
    T3 -->|Read| F1
    
    T1 -->|Write| F2
    T1 -->|Write| F4
    
    T2 -->|Write| F3
    T2 -->|Append| F4
    
    T3 -->|Read| F3
    T3 -->|Append| F2
    T3 -->|Append| F4
    
    F2 -->|Aggregate| O
    F3 -->|Aggregate| O
    F4 -->|Aggregate| O
```

## 5. Task Spawning Syntax and Patterns

### Basic Task Spawning
```python
# Claude's internal Task spawning (conceptual representation)
task = Task(
    description="Analyze security vulnerabilities in authentication module",
    context_files=[
        "/project/auth/*.go",
        "/project/docs/security.md"
    ],
    tools_available=["Read", "Grep", "Edit"],
    independent=True,
    timeout=300  # seconds
)

# Multiple parallel Tasks
tasks = [
    Task("Review domain model", context="/domain/"),
    Task("Analyze API endpoints", context="/api/"),
    Task("Check test coverage", context="/test/")
]

# Execute in parallel
results = execute_parallel(tasks)
```

### Result Aggregation Pattern
```python
# Aggregation happens after all Tasks complete
aggregated_results = {
    "task_1": {
        "status": "complete",
        "findings": [...],
        "files_modified": [...]
    },
    "task_2": {
        "status": "complete",
        "findings": [...],
        "files_created": [...]
    }
}

# Main orchestrator synthesizes final response
final_response = synthesize_results(aggregated_results)
```

## 6. True Concurrency vs Narrative Parallelism

### True Concurrency (Task Tool)
```
Time →
T0 ─────┬─────────┬──────────┬───────── Main Process
        │         │          │
        ├─────────┴──────────┤ Task 1 (Real Process)
        │                    │
        ├────────────────────┤ Task 2 (Real Process)
        │                    │
        └──────────┬─────────┘ Task 3 (Real Process)
                   │
                   └─ All Complete Simultaneously
```

### Narrative Parallelism (Sequential)
```
Time →
T0 ─────┬─────────┬──────────┬───────── Single Process
        │         │          │
        ├─────────┘          │ "Task 1" (Narrative)
        │                    │
        ├────────────────────┘ "Task 2" (Narrative)
        │                    
        └──────────────────── "Task 3" (Narrative)
        
        Sequential execution with parallel narrative
```

## 7. Task Characteristics Reference

Per PARALLEL_EXECUTION.md, Tasks exhibit:

1. **Independent Execution**
   - Separate process space
   - No shared memory
   - Own tool invocation context

2. **File-Based Context**
   - Read shared files for input
   - Write results to filesystem
   - No direct inter-process communication

3. **Parallel Completion**
   - Tasks complete independently
   - No ordering constraints
   - Aggregation only after all complete

4. **Tool Access**
   - Each Task has full tool access
   - Independent file handles
   - Separate execution contexts

## 8. Implementation Example

```mermaid
sequenceDiagram
    participant User
    participant Claude
    participant TaskEngine
    participant Task1
    participant Task2
    participant Task3
    participant FileSystem
    
    User->>Claude: Complex request requiring analysis
    Claude->>TaskEngine: Spawn parallel Tasks
    
    par Task 1 Execution
        TaskEngine->>Task1: Create with context
        Task1->>FileSystem: Read context files
        Task1->>Task1: Process independently
        Task1->>FileSystem: Write results
        Task1->>TaskEngine: Complete
    and Task 2 Execution
        TaskEngine->>Task2: Create with context
        Task2->>FileSystem: Read context files
        Task2->>Task2: Process independently
        Task2->>FileSystem: Write results
        Task2->>TaskEngine: Complete
    and Task 3 Execution
        TaskEngine->>Task3: Create with context
        Task3->>FileSystem: Read context files
        Task3->>Task3: Process independently
        Task3->>FileSystem: Write results
        Task3->>TaskEngine: Complete
    end
    
    TaskEngine->>FileSystem: Collect all results
    TaskEngine->>Claude: Aggregate findings
    Claude->>User: Synthesized response
```

## Key Takeaways

1. **Tasks are real parallel processes**, not simulated concurrency
2. **Memory isolation is absolute** - no shared state between Tasks
3. **File system is the only communication channel**
4. **Aggregation happens post-completion**, not during execution
5. **Each Task has independent tool access** and execution context

This architecture enables Claude to perform genuinely concurrent analysis, significantly reducing response time for complex, parallelizable requests while maintaining clean separation of concerns and predictable behavior.