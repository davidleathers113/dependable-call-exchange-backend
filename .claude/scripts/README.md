# DCE Scripts

This directory contains automation scripts for various Claude Code operations.

## Available Scripts

### bridge-converter.sh
Converts planning documents to context-ready YAML format.

**Usage:**
```bash
./bridge-converter.sh [planning-file.md]
```

**Purpose:**
- Transforms markdown planning documents into structured YAML
- Extracts work items, dependencies, and milestones
- Generates execution-ready context files

### reorganize.sh
Reorganizes .claude directory structure for better maintainability.

**Usage:**
```bash
./reorganize.sh
```

**Purpose:**
- Moves files to appropriate subdirectories
- Archives obsolete files with timestamps
- Updates references in configuration files
- Creates necessary directory structures

## Script Guidelines

1. **Executable**: All scripts must be executable (`chmod +x script.sh`)
2. **Documentation**: Include usage comments at the top of each script
3. **Error Handling**: Scripts should handle errors gracefully
4. **Idempotent**: Scripts should be safe to run multiple times
5. **Logging**: Important operations should be logged

## Adding New Scripts

When adding a new script:
1. Place it in this directory
2. Make it executable: `chmod +x script-name.sh`
3. Add documentation to this README
4. Include clear usage instructions in the script itself
5. Test thoroughly before committing