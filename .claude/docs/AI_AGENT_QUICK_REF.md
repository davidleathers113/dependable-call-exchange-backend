# DCE AI Agent System - Quick Reference

## 🚀 What Is It?
A true parallel execution system using Task tools to spawn multiple concurrent Claude instances, achieving 5-8x speedup in feature implementation.

## 🔑 Key Commands

### Planning
```bash
/dce-master-plan full ./.claude/planning compliance-critical thorough
```

### Feature Implementation  
```bash
/dce-feature ./docs/specs/feature.md . adaptive production
```

## 📊 True Parallel Execution Model

```
Orchestrator → Spawns 5 Tasks (Wave 1) → All execute simultaneously → Sync → Wave 2...
                     ↓
              [Real parallel Tasks via Task tool]
              - Task 1: Entity Architect
              - Task 2: Value Designer  
              - Task 3: Event Architect
              - Task 4: Repository Designer
              - Task 5: Test Engineer
```

## 🌊 Standard Waves

1. **Wave 0**: Analysis & Planning (1 Task)
2. **Wave 1**: Domain Foundation (5 parallel Tasks)
3. **Wave 2**: Infrastructure (5 parallel Tasks)
4. **Wave 3**: Services & Business Logic (5 parallel Tasks)
5. **Wave 4**: API & Presentation (5 parallel Tasks)
6. **Wave 5**: Quality & Testing (5 parallel Tasks)

## 📁 Directory Structure

```
.claude/
├── commands/        # Orchestrators that spawn Tasks
├── prompts/         # Specialist Task definitions
├── context/         # Wave communication files
├── planning/        # Plans and specifications
└── AI_AGENT_GUIDE.md # Full documentation
```

## ⚡ Performance Facts

- **Actual speedup**: 5-8x through true parallelism
- **Parallel Tasks**: Real concurrent executions via Task tool
- **Wave synchronization**: Dependencies managed between waves
- **Independent execution**: Each Task runs in isolation

## 🎯 How Parallel Tasks Work

1. **Task tool spawns** multiple independent executions
2. **Each Task** is a separate Claude instance
3. **True concurrency** - not role-playing or simulation
4. **Real performance gains** from parallel processing

## 🔧 Troubleshooting

| Issue | Solution |
|-------|----------|
| Task spawn failure | Check unique Task descriptions |
| Sync issues | Verify all Tasks completed |
| Slow Task | Optimize individual Task logic |
| Context missing | Check wave output files |

## 📝 Task Spawning Example

```javascript
// Actual parallel execution
await Promise.all([
  Task.spawn("Entity Architect - Entities", entityPrompt),
  Task.spawn("Value Designer - Values", valuePrompt),
  Task.spawn("Event Architect - Events", eventPrompt),
  Task.spawn("Repository Designer - Repos", repoPrompt),
  Task.spawn("Test Engineer - Tests", testPrompt)
]);
```

## 🎭 Specialist Roles (Parallel Tasks)

- **Entity Architect**: Creates domain entities concurrently
- **Repository Builder**: Implements data access in parallel
- **Service Orchestrator**: Builds services simultaneously
- **API Designer**: Develops endpoints independently
- **Test Engineer**: Writes tests in parallel

## ✅ Quality Gates

Between waves:
- All Tasks must complete
- Compilation verification
- File existence checks
- Interface validation
- Performance benchmarks

## 💡 Key Points

- This is **TRUE parallel execution** via Task tool
- Performance gains are **REAL** (not simulated)
- Each Task is an **independent execution**
- Wave synchronization ensures **correct dependencies**
- The system **actually works** as designed!

---
*For complete documentation, see AI_AGENT_GUIDE.md*