# Wave Synchronization and Execution Flow

This document provides detailed diagrams showing the 5-wave execution model, synchronization mechanisms, and state management in the DCE parallel execution system.

## Overview: 5-Wave Execution Model

```mermaid
graph TB
    subgraph "Wave 0: Analysis"
        W0[Analysis Phase<br/>1 task, 2 min]
        W0 --> W0O[wave-0-output.yaml]
    end
    
    subgraph "Wave 1: Domain Foundation"
        W1[Domain Tasks<br/>5 tasks, 3 min]
        W0O --> W1
        W1 --> W1O[wave-1-output.yaml]
    end
    
    subgraph "Wave 2: Infrastructure"
        W2[Infrastructure Tasks<br/>5 tasks, 3 min]
        W1O --> W2
        W2 --> W2O[wave-2-output.yaml]
    end
    
    subgraph "Wave 3: Services"
        W3[Service Tasks<br/>5 tasks, 3 min]
        W2O --> W3
        W3 --> W3O[wave-3-output.yaml]
    end
    
    subgraph "Wave 4: Quality Assurance"
        W4[QA Tasks<br/>5 tasks, 3 min]
        W3O --> W4
        W4 --> W4O[wave-4-output.yaml]
    end
    
    W4O --> COMPLETE[Project Complete<br/>Total: 14 min]
    
    style W0 fill:#e1f5fe
    style W1 fill:#c8e6c9
    style W2 fill:#fff3b2
    style W3 fill:#ffccbc
    style W4 fill:#d1c4e9
    style COMPLETE fill:#a5d6a7
```

## Wave Execution Timeline

```mermaid
gantt
    title DCE Wave Execution Timeline
    dateFormat mm:ss
    axisFormat %M:%S
    
    section Wave 0
    Analysis & Planning    :active, w0, 00:00, 2m
    
    section Wave 1
    Domain Models         :w1a, after w0, 3m
    Value Objects        :w1b, after w0, 3m
    Domain Events        :w1c, after w0, 3m
    Domain Tests         :w1d, after w0, 3m
    Domain Validation    :w1e, after w0, 3m
    
    section Wave 2
    Database Layer       :w2a, after w1a, 3m
    Cache Layer         :w2b, after w1a, 3m
    Message Queue       :w2c, after w1a, 3m
    Config System       :w2d, after w1a, 3m
    Monitoring          :w2e, after w1a, 3m
    
    section Wave 3
    Call Service        :w3a, after w2a, 3m
    Bid Service         :w3b, after w2a, 3m
    Compliance Service  :w3c, after w2a, 3m
    Analytics Service   :w3d, after w2a, 3m
    Financial Service   :w3e, after w2a, 3m
    
    section Wave 4
    Integration Tests   :w4a, after w3a, 3m
    Contract Tests      :w4b, after w3a, 3m
    Performance Tests   :w4c, after w3a, 3m
    Security Scan       :w4d, after w3a, 3m
    Documentation       :w4e, after w3a, 3m
```

## Wave Synchronization Mechanisms

```mermaid
flowchart TB
    subgraph "Wave Synchronization"
        START[Start Wave N] --> BARRIER[Wave Barrier]
        
        BARRIER --> CHECK{All Previous<br/>Waves Complete?}
        CHECK -->|No| WAIT[Wait for Dependencies]
        WAIT --> CHECK
        CHECK -->|Yes| LOAD[Load Previous Wave Outputs]
        
        LOAD --> VALIDATE{Validate<br/>Dependencies?}
        VALIDATE -->|No| ERROR[Dependency Error]
        VALIDATE -->|Yes| EXECUTE[Execute Wave Tasks]
        
        EXECUTE --> SYNC[Synchronization Point]
        
        subgraph "Parallel Execution"
            TASK1[Task 1]
            TASK2[Task 2]
            TASK3[Task 3]
            TASK4[Task 4]
            TASK5[Task 5]
        end
        
        SYNC --> TASK1
        SYNC --> TASK2
        SYNC --> TASK3
        SYNC --> TASK4
        SYNC --> TASK5
        
        TASK1 --> COLLECT[Collect Results]
        TASK2 --> COLLECT
        TASK3 --> COLLECT
        TASK4 --> COLLECT
        TASK5 --> COLLECT
        
        COLLECT --> GATE{Quality Gate<br/>Passed?}
        GATE -->|No| RETRY[Retry Failed Tasks]
        RETRY --> SYNC
        GATE -->|Yes| OUTPUT[Generate wave-N-output.yaml]
        
        OUTPUT --> NEXT[Signal Next Wave]
    end
    
    style BARRIER fill:#ff9800
    style SYNC fill:#2196f3
    style GATE fill:#f44336
    style OUTPUT fill:#4caf50
```

## Context Propagation via YAML Files

```mermaid
flowchart LR
    subgraph "Wave 0 Output"
        W0Y[wave-0-output.yaml<br/>- Project structure<br/>- Dependencies<br/>- Task breakdown<br/>- Risk assessment]
    end
    
    subgraph "Wave 1 Output"
        W1Y[wave-1-output.yaml<br/>- Domain models<br/>- Value objects<br/>- Event definitions<br/>- Validation rules]
    end
    
    subgraph "Wave 2 Output"
        W2Y[wave-2-output.yaml<br/>- DB schemas<br/>- Cache keys<br/>- Queue topics<br/>- Config values]
    end
    
    subgraph "Wave 3 Output"
        W3Y[wave-3-output.yaml<br/>- Service APIs<br/>- Integration points<br/>- Business logic<br/>- Error handling]
    end
    
    subgraph "Wave 4 Output"
        W4Y[wave-4-output.yaml<br/>- Test results<br/>- Coverage metrics<br/>- Performance data<br/>- Security report]
    end
    
    W0Y -->|Context| W1Y
    W1Y -->|Context| W2Y
    W2Y -->|Context| W3Y
    W3Y -->|Context| W4Y
    
    style W0Y fill:#e3f2fd
    style W1Y fill:#e8f5e9
    style W2Y fill:#fffde7
    style W3Y fill:#fff3e0
    style W4Y fill:#f3e5f5
```

## Quality Gates Between Waves

```mermaid
flowchart TB
    subgraph "Quality Gate Process"
        WAVE_COMPLETE[Wave N Complete] --> METRICS[Collect Metrics]
        
        METRICS --> CHECKS{Quality Checks}
        
        CHECKS --> TEST[Test Coverage > 80%]
        CHECKS --> LINT[Linting Passed]
        CHECKS --> BUILD[Build Success]
        CHECKS --> PERF[Performance Targets Met]
        
        TEST --> EVAL{All Checks<br/>Passed?}
        LINT --> EVAL
        BUILD --> EVAL
        PERF --> EVAL
        
        EVAL -->|No| BLOCK[Block Next Wave]
        BLOCK --> FIX[Fix Issues]
        FIX --> METRICS
        
        EVAL -->|Yes| APPROVE[Approve Next Wave]
        APPROVE --> RECORD[Record Gate Status]
        RECORD --> PROCEED[Proceed to Wave N+1]
    end
    
    style CHECKS fill:#ffc107
    style EVAL fill:#ff5722
    style APPROVE fill:#4caf50
    style BLOCK fill:#f44336
```

## State Management and Progress Tracking

```mermaid
stateDiagram-v2
    [*] --> Initialized: Project Start
    
    Initialized --> Wave0_Running: Start Analysis
    Wave0_Running --> Wave0_Complete: Analysis Done
    Wave0_Complete --> Wave0_Validated: Quality Gate Passed
    
    Wave0_Validated --> Wave1_Running: Start Domain
    Wave1_Running --> Wave1_Complete: Domain Done
    Wave1_Complete --> Wave1_Validated: Quality Gate Passed
    
    Wave1_Validated --> Wave2_Running: Start Infrastructure
    Wave2_Running --> Wave2_Complete: Infrastructure Done
    Wave2_Complete --> Wave2_Validated: Quality Gate Passed
    
    Wave2_Validated --> Wave3_Running: Start Services
    Wave3_Running --> Wave3_Complete: Services Done
    Wave3_Complete --> Wave3_Validated: Quality Gate Passed
    
    Wave3_Validated --> Wave4_Running: Start QA
    Wave4_Running --> Wave4_Complete: QA Done
    Wave4_Complete --> Wave4_Validated: Quality Gate Passed
    
    Wave4_Validated --> Project_Complete: All Waves Done
    Project_Complete --> [*]
    
    note right of Wave0_Running
        State stored in:
        .claude/state/wave-status.yaml
    end note
    
    note right of Wave1_Running
        Progress tracked in:
        .claude/metrics/progress.yaml
    end note
```

## Error Handling and Recovery Patterns

```mermaid
flowchart TB
    subgraph "Error Detection and Recovery"
        TASK[Task Execution] --> MONITOR{Monitor Status}
        
        MONITOR -->|Success| SUCCESS[Task Complete]
        MONITOR -->|Failure| ERROR[Error Detected]
        
        ERROR --> CLASSIFY{Classify Error}
        
        CLASSIFY -->|Transient| RETRY_POLICY[Apply Retry Policy]
        CLASSIFY -->|Permanent| FAIL_TASK[Mark Task Failed]
        CLASSIFY -->|Dependency| WAIT_DEP[Wait for Dependency]
        
        RETRY_POLICY --> BACKOFF[Exponential Backoff]
        BACKOFF --> RETRY_COUNT{Retry Count<br/>< Max?}
        
        RETRY_COUNT -->|Yes| TASK
        RETRY_COUNT -->|No| FAIL_TASK
        
        WAIT_DEP --> DEP_CHECK{Dependency<br/>Available?}
        DEP_CHECK -->|No| WAIT_DEP
        DEP_CHECK -->|Yes| TASK
        
        FAIL_TASK --> COMPENSATE[Compensation Logic]
        COMPENSATE --> ROLLBACK[Rollback Changes]
        ROLLBACK --> NOTIFY[Notify Coordinator]
        
        NOTIFY --> WAVE_DECISION{Wave Decision}
        WAVE_DECISION -->|Abort Wave| ABORT[Abort All Tasks]
        WAVE_DECISION -->|Continue| DEGRADED[Continue Degraded]
        WAVE_DECISION -->|Retry Wave| RESET[Reset Wave State]
        
        SUCCESS --> CHECKPOINT[Save Checkpoint]
        CHECKPOINT --> NEXT[Next Task/Wave]
    end
    
    style ERROR fill:#ff5252
    style RETRY_POLICY fill:#ff9800
    style SUCCESS fill:#4caf50
    style COMPENSATE fill:#9c27b0
```

## Wave Dependency Graph

```mermaid
graph TD
    subgraph "Detailed Wave Dependencies"
        W0_ANALYSIS[Wave 0: Analysis<br/>- Project structure analysis<br/>- Dependency mapping<br/>- Risk assessment]
        
        W0_ANALYSIS --> W1_DOMAIN[Wave 1: Domain Models]
        W0_ANALYSIS --> W1_VALUES[Wave 1: Value Objects]
        W0_ANALYSIS --> W1_EVENTS[Wave 1: Domain Events]
        W0_ANALYSIS --> W1_TESTS[Wave 1: Domain Tests]
        W0_ANALYSIS --> W1_VALID[Wave 1: Validation Rules]
        
        W1_DOMAIN --> W2_DB[Wave 2: Database Layer]
        W1_VALUES --> W2_DB
        W1_EVENTS --> W2_QUEUE[Wave 2: Message Queue]
        W1_DOMAIN --> W2_CACHE[Wave 2: Cache Layer]
        W1_VALUES --> W2_CONFIG[Wave 2: Config System]
        W1_TESTS --> W2_MONITOR[Wave 2: Monitoring]
        
        W2_DB --> W3_CALL[Wave 3: Call Service]
        W2_DB --> W3_BID[Wave 3: Bid Service]
        W2_DB --> W3_COMPLY[Wave 3: Compliance Service]
        W2_CACHE --> W3_ANALYTICS[Wave 3: Analytics Service]
        W2_QUEUE --> W3_FINANCIAL[Wave 3: Financial Service]
        
        W3_CALL --> W4_INTEGRATION[Wave 4: Integration Tests]
        W3_BID --> W4_CONTRACT[Wave 4: Contract Tests]
        W3_COMPLY --> W4_PERF[Wave 4: Performance Tests]
        W3_ANALYTICS --> W4_SECURITY[Wave 4: Security Scan]
        W3_FINANCIAL --> W4_DOCS[Wave 4: Documentation]
    end
    
    style W0_ANALYSIS fill:#e1f5fe
    style W1_DOMAIN fill:#c8e6c9
    style W2_DB fill:#fff3b2
    style W3_CALL fill:#ffccbc
    style W4_INTEGRATION fill:#d1c4e9
```

## State File Structure

```yaml
# .claude/context/wave-0-output.yaml
wave: 0
status: complete
duration: "2m15s"
tasks_completed: 1
output:
  project_analysis:
    structure:
      - domain_layers: 5
      - service_count: 8
      - api_endpoints: 42
    dependencies:
      - external: ["postgres", "redis", "kafka"]
      - internal: ["domain", "infrastructure", "service"]
    risks:
      - complexity: "high"
      - critical_paths: ["call_routing", "bid_processing"]

# .claude/context/wave-1-output.yaml
wave: 1
status: complete
duration: "3m02s"
tasks_completed: 5
dependencies:
  - wave: 0
    artifacts: ["project_analysis"]
output:
  domain_models:
    - call: "internal/domain/call/call.go"
    - bid: "internal/domain/bid/bid.go"
    - buyer: "internal/domain/account/buyer.go"
  value_objects:
    - money: "internal/domain/values/money.go"
    - phone_number: "internal/domain/values/phone_number.go"
  tests:
    coverage: 85.3
    passing: 142

# .claude/context/wave-2-output.yaml
wave: 2
status: complete
duration: "3m18s"
tasks_completed: 5
dependencies:
  - wave: 1
    artifacts: ["domain_models", "value_objects"]
output:
  infrastructure:
    database:
      - migrations: 15
      - repositories: 8
    cache:
      - strategy: "write-through"
      - ttl: "5m"
    monitoring:
      - metrics: ["latency", "throughput", "errors"]
      - dashboards: 3

# Continue for waves 3 and 4...
```

## Wave Coordination Protocol

```mermaid
sequenceDiagram
    participant C as Coordinator
    participant W0 as Wave 0
    participant W1 as Wave 1
    participant W2 as Wave 2
    participant QG as Quality Gate
    participant ST as State Manager
    
    C->>W0: Start Wave 0
    W0->>W0: Execute Analysis
    W0->>ST: Save Progress
    W0->>C: Complete Signal
    
    C->>QG: Check Wave 0 Quality
    QG->>QG: Run Validations
    QG->>C: Gate Passed
    
    C->>ST: Save wave-0-output.yaml
    C->>W1: Start Wave 1 (with context)
    
    par Parallel Tasks in Wave 1
        W1->>W1: Domain Models
        W1->>W1: Value Objects
        W1->>W1: Domain Events
        W1->>W1: Domain Tests
        W1->>W1: Validation Rules
    end
    
    W1->>ST: Save Progress
    W1->>C: Complete Signal
    
    C->>QG: Check Wave 1 Quality
    QG->>C: Gate Passed
    
    C->>ST: Save wave-1-output.yaml
    C->>W2: Start Wave 2 (with context)
    
    Note over C,W2: Process continues through all waves
```

## Performance Metrics

```mermaid
graph LR
    subgraph "Wave Performance Targets"
        W0P[Wave 0<br/>Target: 2 min<br/>Actual: 2:15]
        W1P[Wave 1<br/>Target: 3 min<br/>Actual: 3:02]
        W2P[Wave 2<br/>Target: 3 min<br/>Actual: 3:18]
        W3P[Wave 3<br/>Target: 3 min<br/>Actual: 2:55]
        W4P[Wave 4<br/>Target: 3 min<br/>Actual: 3:10]
        
        W0P --> TOTAL[Total Time<br/>Target: 14 min<br/>Actual: 14:40]
    end
    
    style W0P fill:#4caf50
    style W1P fill:#4caf50
    style W2P fill:#ff9800
    style W3P fill:#4caf50
    style W4P fill:#4caf50
    style TOTAL fill:#4caf50
```

## Key Benefits

1. **Predictable Execution**: 14-minute total runtime with clear milestones
2. **Parallel Efficiency**: 5 tasks per wave execute simultaneously
3. **Context Preservation**: YAML files maintain state between waves
4. **Quality Assurance**: Gates ensure each wave meets standards
5. **Error Recovery**: Robust handling of failures with compensation
6. **Progress Visibility**: Real-time tracking of wave completion

## Implementation Notes

- Each wave has a dedicated coordinator thread
- Tasks within a wave share a common context
- State files are atomic and versioned
- Quality gates are configurable per project
- Recovery strategies can be customized per task type
- Metrics are collected in real-time for optimization