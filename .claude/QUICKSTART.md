# 🚀 DCE AI Agent System - 5-Minute Quick Start

Get up and running with the DCE AI Agent System in 5 minutes!

## What Is This?

The DCE AI Agent System is a **parallel execution framework** that makes Claude Code 5-8x faster at implementing features by running multiple specialized AI agents concurrently.

### Key Benefits
- ⚡ **5-8x faster** feature implementation
- 🔄 **Automatic handoffs** between planning and coding
- 💾 **State persistence** - never lose progress
- 🎯 **Smart work discovery** - finds what to do next

## Your First Command

```bash
# Start with a simple feature implementation
/dce-feature ./docs/specs/my-feature.md . adaptive production
```

That's it! The system will:
1. 📋 Load the specification
2. 🚀 Spawn 5 parallel AI agents
3. 🏗️ Build your feature across all layers
4. ✅ Validate everything works

## The Power Workflow

For maximum efficiency, use the full workflow:

```bash
# 1. Plan your project (analyzes codebase, generates roadmap)
/dce-master-plan full ./.claude/planning balanced thorough

# 2. Find ready work (discovers implementable tasks)
/dce-find-work --ready

# 3. Implement top priority (automatic handoff from planning!)
/dce-feature ./planning/specs/consent-management-v2.md . adaptive production

# 4. Check your work (finds gaps and issues)
/dce-check-work
```

## How It Works (30-Second Version)

```
Traditional Sequential:
[Analysis] → [Domain] → [Service] → [API] → [Tests]
  15min      20min      15min       10min    10min  = 70 minutes

DCE Parallel System:
[Analysis] → [Domain|Service|API|Tests|Docs] (all at once!)
  5min              10min parallel            = 15 minutes
```

Real parallelism through the Task tool - not simulation!

## Essential Commands

| What You Want | Command to Use |
|--------------|----------------|
| Plan a project | `/dce-master-plan full ./.claude/planning balanced thorough` |
| Build a feature | `/dce-feature [spec-file] . adaptive production` |
| Find next task | `/dce-find-work --ready` |
| Check quality | `/dce-check-work` |
| Research solutions | `/dce-research "your technical question"` |

## Understanding Output

When you run a command, you'll see:

```
🌊 Wave 1: Domain Layer (5 parallel tasks)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚀 Spawning 5 parallel Tasks...

Task 1 (Entities): 🔄 Running... [1m 23s]
Task 2 (Values): ✅ Complete [2m 01s]
Task 3 (Events): 🔄 Running... [1m 45s]
Task 4 (Repository): ✅ Complete [1m 52s]
Task 5 (Tests): 🔄 Running... [1m 38s]

Progress: 40% | ETA: ~2m remaining
```

Each wave completes before the next begins, ensuring proper dependencies.

## Common Workflows

### Start Fresh Project
```bash
/dce-master-plan full ./.claude/planning compliance-critical thorough
/dce-find-work --ready
# Pick a feature from the list
/dce-feature ./planning/specs/[chosen-feature].md . adaptive production
```

### Resume Interrupted Work
```bash
# Check what was in progress
cat .claude/state/feature-progress.yaml

# Resume from where you left off
/dce-feature-resume consent-management-v2
```

### Quick Feature (No Planning)
```bash
# Direct implementation from existing spec
/dce-feature ./my-spec.md . adaptive production
```

## Pro Tips

1. **Always check work**: Run `/dce-check-work` before committing
2. **Use adaptive mode**: Balances speed and quality automatically  
3. **Trust the handoff**: Planning outputs become feature inputs automatically
4. **Monitor progress**: Check `.claude/state/feature-progress.yaml`
5. **Research when stuck**: `/dce-research "specific error or pattern"`

## What's Next?

- 📖 Read the full [README.md](./README.md) for comprehensive details
- 🏗️ Explore [AI_AGENT_GUIDE.md](./docs/AI_AGENT_GUIDE.md) to understand the architecture
- 🔧 Check [COMMAND_REFERENCE.md](./COMMAND_REFERENCE.md) for all available commands
- 🐛 See [TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) if you hit issues

## Quick Troubleshooting

**Feature command not using planning context?**
- Make sure you ran `/dce-master-plan` first
- Check `.claude/context/feature-context.yaml` exists

**Commands timing out?**
- Use shorter planning depth: `quick` instead of `thorough`
- Run `/dce-system-improve --phase=5` for performance analysis

**Work discovery empty?**
- Run master plan to populate: `/dce-master-plan full ./.claude/planning balanced thorough`

---

🎉 **Congratulations!** You're ready to accelerate development with true parallel AI execution!

Need more details? See [INDEX.md](./INDEX.md) for complete navigation.