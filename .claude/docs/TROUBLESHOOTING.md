# 🔧 DCE AI Agent System - Troubleshooting Guide

This guide provides solutions to common issues encountered when using the DCE AI Agent System.

## 📋 Quick Diagnostics

Before diving into specific issues, run these diagnostic commands:

```bash
# Check system status
/dce-system-status

# Verify state files
ls -la .claude/state/
ls -la .claude/context/

# Check permissions
cat .claude/settings.json

# Review recent errors
tail -n 50 .claude/logs/error.log 2>/dev/null || echo "No error log found"
```

## 🚨 Common Issues & Solutions

### 1. Commands Timing Out

**Symptoms**:
- Commands hang indefinitely
- Timeout errors after 300+ seconds
- `-p` flag automation failing

**Solution A - Trust Dialog Issue** (Most Common):
```python
# Fix trust dialogs programmatically
#!/usr/bin/env python3
import json
import os

config_path = os.path.expanduser("~/.claude.json")

with open(config_path, 'r') as f:
    config = json.load(f)

# Update all projects
for project_path in config.get("projects", {}):
    project = config["projects"][project_path]
    project["hasTrustDialogAccepted"] = True
    project["hasCompletedProjectOnboarding"] = True

with open(config_path, 'w') as f:
    json.dump(config, f, indent=2)

print("✅ Trust dialogs fixed!")
```

**Solution B - Reduce Complexity**:
```bash
# Use shorter planning depth
/dce-master-plan full ./planning balanced quick  # not 'thorough'

# Use adaptive mode
/dce-feature ./spec.md . adaptive production  # not 'thorough'

# Increase timeout
/dce-feature ./spec.md . adaptive production --timeout=1800
```

---

### 2. Feature Command Not Using Planning Context

**Symptoms**:
- Feature starts with Wave 0 (research) instead of Wave 1
- Ignores planning analysis
- Duplicates work already done

**Root Cause**: Missing handoff files from planning phase

**Solution**:
```bash
# 1. Check if handoff files exist
ls -la .claude/context/feature-context.yaml
ls -la .claude/planning/implementation-plan.md

# 2. If missing, regenerate from planning
/dce-master-plan handoff

# 3. Or manually create bridge
/dce-context-bridge ./planning consent-management

# 4. Verify and retry
/dce-feature ./planning/specs/feature.md . adaptive production
```

---

### 3. Parallel Tasks Not Executing

**Symptoms**:
- Only one task runs at a time
- No performance improvement
- Sequential execution pattern

**Root Cause**: Task tool permissions or configuration issue

**Solution**:
```bash
# 1. Verify Task tool permission
cat .claude/settings.json
# Should show: "allow": ["Write", "MultiEdit", "Edit", "Bash"]

# 2. Check for Task tool in available tools
claude --help | grep -i task

# 3. Test parallel execution
/dce-parallel-test basic 3

# 4. If still failing, check Claude version
claude --version  # Should be 1.0.21 or later
```

---

### 4. State File Corruption

**Symptoms**:
- "Cannot parse YAML" errors
- Unexpected state transitions
- Lost progress

**Solution**:
```bash
# 1. Backup current state
cp -r .claude/state .claude/state.backup.$(date +%Y%m%d-%H%M%S)

# 2. Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('.claude/state/feature-progress.yaml'))"

# 3. If corrupted, restore from backup
cp .claude/state.backup/feature-progress.yaml .claude/state/

# 4. Or repair with tool
/dce-state-repair --force

# 5. Reset to clean state (last resort)
rm .claude/state/feature-progress.yaml
/dce-feature-resume --reset
```

---

### 5. Work Discovery Returns Empty

**Symptoms**:
- `/dce-find-work` shows no results
- "No ready tasks found" message
- Empty execution queue

**Root Cause**: No planning phase completed

**Solution**:
```bash
# 1. Run master planning first
/dce-master-plan full ./.claude/planning balanced thorough

# 2. Check execution queue was created
cat .claude/context/execution-queue.yaml

# 3. Find all work (including blocked)
/dce-find-work --all

# 4. If still empty, check planning outputs
ls -la .claude/planning/specs/
```

---

### 6. Process Hanging (Test Suite Issue)

**Symptoms**:
- Claude processes accumulate
- System becomes sluggish
- Tests never complete

**Root Cause**: Background processes not cleaned up properly

**Solution**:
```bash
# 1. Find hanging Claude processes
ps aux | grep claude | grep -v grep

# 2. Clean up zombie processes
pkill -f "claude.*-p"  # Kill background Claude processes

# 3. Implement proper cleanup in scripts
cleanup() {
    if [ -n "$CLAUDE_PID" ] && kill -0 "$CLAUDE_PID" 2>/dev/null; then
        kill -TERM "$CLAUDE_PID" 2>/dev/null || true
        sleep 2
        kill -KILL "$CLAUDE_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM
```

---

### 7. Memory/Performance Issues

**Symptoms**:
- High memory usage (>2GB)
- Slow execution
- System unresponsive

**Solution**:
```bash
# 1. Check current usage
/dce-system-status performance

# 2. Limit parallel tasks
export DCE_MAX_PARALLEL_TASKS=3  # Default is 5-8

# 3. Clear caches
rm -rf .claude/cache/*
rm -f .claude/state/analysis-cache.yaml

# 4. Use fast mode for prototyping
/dce-feature ./spec.md . fast development

# 5. Monitor resource usage
top -p $(pgrep -f claude)
```

---

### 8. Missing Dependencies

**Symptoms**:
- "Module not found" errors
- Import failures
- Undefined references

**Root Cause**: Features implemented out of order

**Solution**:
```bash
# 1. Check dependency graph
cat .claude/context/execution-queue.yaml

# 2. Find blocking dependencies
/dce-find-work --show-blocked

# 3. Implement dependencies first
/dce-feature ./planning/specs/dependency-feature.md . adaptive production

# 4. Or force implementation (risky)
/dce-feature ./spec.md . adaptive production --ignore-deps
```

---

### 9. API/Network Errors

**Symptoms**:
- "Failed to connect" errors
- API timeout messages
- Network unreachable

**Solution**:
```bash
# 1. Test Claude connectivity
claude -p "test" --output-format=text

# 2. Check network
ping -c 3 api.anthropic.com

# 3. Verify API credentials
echo $ANTHROPIC_API_KEY | head -c 10  # Should show first 10 chars

# 4. Use retry logic
for i in {1..3}; do
    /dce-feature ./spec.md . adaptive production && break
    echo "Retry $i/3 in 30s..."
    sleep 30
done
```

---

### 10. Handoff Files Not Created

**Symptoms**:
- Planning completes but no handoff
- Missing `feature-context.yaml`
- Phase 5b not executed

**Solution**:
```bash
# 1. Check if Phase 5b ran
grep -i "handoff" .claude/planning/master-plan.md

# 2. Run handoff phase manually
/dce-master-plan handoff

# 3. Create manually from spec
cat > .claude/context/feature-context.yaml << 'EOF'
feature:
  id: "manual-feature"
  name: "Manually Created Feature"
  spec_file: "./planning/specs/feature.md"
  
implementation_waves:
  - wave: 1
    focus: "Core implementation"
EOF

# 4. Verify and proceed
/dce-feature ./planning/specs/feature.md . adaptive production
```

---

## 🔍 Advanced Troubleshooting

### Debug Mode Execution

```bash
# Enable verbose logging
export DCE_DEBUG=true
export DCE_LOG_LEVEL=debug

# Run with debug output
/dce-feature ./spec.md . adaptive production 2>&1 | tee debug.log

# Analyze debug log
grep -i error debug.log
grep -i warning debug.log
```

### State File Analysis

```python
#!/usr/bin/env python3
# analyze_state.py - Diagnose state issues

import yaml
import json
from datetime import datetime

# Load state files
with open('.claude/state/feature-progress.yaml', 'r') as f:
    progress = yaml.safe_load(f)

with open('.claude/state/system-snapshot.yaml', 'r') as f:
    snapshot = yaml.safe_load(f)

# Analyze
print(f"Current Feature: {progress.get('current_feature', {}).get('id', 'None')}")
print(f"Status: {progress.get('current_feature', {}).get('status', 'Unknown')}")
print(f"Waves Completed: {len(progress.get('waves_completed', []))}")

# Check for issues
if progress.get('current_wave', {}).get('status') == 'in_progress':
    started = progress['current_wave'].get('started_at', '')
    if started:
        duration = datetime.now() - datetime.fromisoformat(started.replace('Z', '+00:00'))
        if duration.total_seconds() > 3600:  # 1 hour
            print("⚠️  WARNING: Current wave running for over 1 hour!")
```

### Performance Profiling

```bash
# 1. Create performance baseline
/dce-system-improve --phase=1 --output=baseline.json

# 2. Run feature with profiling
time /dce-feature ./spec.md . adaptive production

# 3. Compare performance
/dce-system-improve --phase=2 --baseline=baseline.json

# 4. Generate optimization report
/dce-system-improve --phase=3 --report
```

## 📊 Error Code Reference

| Code | Meaning | Solution |
|------|---------|----------|
| DCE-001 | Invalid command syntax | Check command format in COMMAND_REFERENCE.md |
| DCE-002 | Missing required files | Run planning phase first |
| DCE-003 | State corruption | Use state repair tool |
| DCE-004 | Permission denied | Update settings.json |
| DCE-005 | Timeout exceeded | Reduce complexity or increase timeout |
| DCE-006 | Dependency conflict | Resolve dependencies first |
| DCE-007 | Resource exhausted | Reduce parallel tasks |
| DCE-008 | Network error | Check connectivity |
| DCE-009 | Invalid configuration | Validate YAML syntax |
| DCE-010 | Version mismatch | Update Claude Code |

## 🛠️ Recovery Procedures

### Complete System Reset

```bash
# 1. Backup everything
tar -czf dce-backup-$(date +%Y%m%d-%H%M%S).tar.gz .claude/

# 2. Clean state
rm -rf .claude/state/*
rm -rf .claude/context/*
rm -rf .claude/cache/*

# 3. Restore configurations
cp .claude/settings.json.default .claude/settings.json

# 4. Start fresh
/dce-master-plan full ./.claude/planning balanced standard
```

### Incremental Recovery

```bash
# 1. Identify last good state
ls -la .claude/state/*.backup

# 2. Restore specific file
cp .claude/state/feature-progress.yaml.backup .claude/state/feature-progress.yaml

# 3. Resume from checkpoint
/dce-feature-resume --from-wave=2
```

## 🆘 Getting Help

If these solutions don't resolve your issue:

1. **Collect Diagnostics**:
   ```bash
   /dce-system-status > diagnostics.txt
   tar -czf dce-logs.tar.gz .claude/logs/
   ```

2. **Check Documentation**:
   - [README.md](../README.md) - System overview
   - [AI_AGENT_GUIDE.md](./AI_AGENT_GUIDE.md) - Technical details
   - [COMMAND_REFERENCE.md](../COMMAND_REFERENCE.md) - Command usage

3. **Community Resources**:
   - GitHub Issues: Report bugs
   - Discord: Real-time help
   - Stack Overflow: `dce-ai-agent` tag

## 🔄 Preventive Measures

1. **Regular Backups**:
   ```bash
   # Add to crontab
   0 */6 * * * tar -czf ~/.claude-backups/dce-$(date +\%Y\%m\%d-\%H).tar.gz ~/.claude/
   ```

2. **State Monitoring**:
   ```bash
   # Monitor state health
   watch -n 30 '/dce-system-status | grep -E "(ERROR|WARNING)"'
   ```

3. **Resource Limits**:
   ```bash
   # Set resource constraints
   export DCE_MAX_MEMORY=2G
   export DCE_MAX_PARALLEL_TASKS=5
   ```

4. **Pre-flight Checks**:
   ```bash
   # Before major operations
   /dce-system-status
   /dce-state-repair --check-only
   ```

---

💡 **Remember**: Most issues stem from:
- Incomplete planning phases
- Missing handoff files  
- Trust dialog configuration
- Resource constraints

Always start troubleshooting with `/dce-system-status` for a quick health check!