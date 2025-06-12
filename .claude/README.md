# Claude Code Commands for DCE

This directory contains custom Claude Code commands for the Dependable Call Exchange backend project.

## Available Commands

### `/dce-master-plan`
Strategic master planner that analyzes the codebase and generates a comprehensive development roadmap.

**Usage**: `/dce-master-plan [scope] [output_dir] [priority] [depth]`

**Example**: 
```bash
/dce-master-plan full ./planning compliance-critical thorough
```

### `/dce-feature`
Feature executor that implements specifications across all architectural layers.

**Usage**: `/dce-feature [spec_file] [output_dir] [mode] [quality]`

**Example**:
```bash
/dce-feature ./planning/specs/consent-management-v2.md . adaptive production
```

## Command Files

- `commands/dce-master-plan.md` - Master planning command
- `commands/dce-feature.md` - Feature implementation command

## How It Works

1. **Planning Phase**: Use `/dce-master-plan` to analyze your codebase and generate a roadmap
2. **Specification Phase**: Review generated specifications in the planning directory
3. **Execution Phase**: Use `/dce-feature` to implement features from specifications

## Quick Start

```bash
# 1. Generate a development plan
/dce-master-plan full ./planning balanced thorough

# 2. Review the plan
cat ./planning/master-plan.md

# 3. Implement a feature
/dce-feature ./planning/specs/consent-management-v2.md . adaptive production
```

## Customization

These commands are tailored specifically for the DCE project's:
- Go 1.24+ architecture
- Domain-driven design patterns
- High-performance requirements (< 1ms routing)
- Compliance needs (TCPA, GDPR, DNC)
- Modular monolith structure

To customize further, edit the command files directly.