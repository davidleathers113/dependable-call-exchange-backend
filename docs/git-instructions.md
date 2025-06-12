# Git Instructions

This guide provides essential Git workflows and conventions for the Dependable Call Exchange Backend project.

## Branch Naming Conventions

Use descriptive, kebab-case branch names with appropriate prefixes:

```bash
feat/<short-description>    # New features
fix/<issue-number>         # Bug fixes (reference issue number)
chore/<topic>             # Maintenance tasks
refactor/<component>      # Code refactoring
docs/<topic>              # Documentation updates
test/<component>          # Test additions/improvements
perf/<optimization>       # Performance improvements
```

Examples:
- `feat/add-oauth-flow`
- `fix/42-routing-latency`
- `chore/update-dependencies`
- `refactor/bidding-service`

## Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, missing semicolons, etc.)
- `refactor`: Code refactoring without changing functionality
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Maintenance tasks
- `ci`: CI/CD configuration changes

### Examples

```bash
feat(bidding): add real-time auction algorithm

Implement weighted round-robin algorithm for bid distribution
with sub-millisecond routing decisions.

Closes #123
```

```bash
fix(call): resolve memory leak in call routing

The call routing service was holding references to completed
calls. Added proper cleanup in the finalizer.

Fixes #456
```

## Essential Git Workflows

### Starting New Work

```bash
# Update main branch
git checkout main
git pull origin main

# Create feature branch
git checkout -b feat/your-feature-name

# Make changes and commit
git add .
git commit -m "feat(domain): add new functionality"

# Push to remote
git push -u origin feat/your-feature-name
```

### Pre-Commit Checklist

Before committing, always run:

```bash
# Run all CI checks
make ci

# Or run individually:
go build -gcflags="-e" ./...  # Check all compilation errors
make test                      # Run all tests
make lint                      # Run linter
make test-synctest            # Run concurrent tests
```

### Updating Your Branch

Keep your feature branch up to date with main:

```bash
# Fetch latest changes
git fetch origin

# Rebase on main (preferred)
git checkout feat/your-feature
git rebase origin/main

# Or merge main (if rebase is complex)
git merge origin/main
```

### Handling Conflicts

```bash
# During rebase
git rebase origin/main

# If conflicts occur:
# 1. Fix conflicts in affected files
# 2. Stage resolved files
git add <resolved-files>

# 3. Continue rebase
git rebase --continue

# Or abort if needed
git rebase --abort
```

## Pull Request Process

1. **Before Creating PR**
   ```bash
   # Ensure branch is up to date
   git rebase origin/main
   
   # Run full CI suite
   make ci
   
   # Review your changes
   git diff origin/main
   ```

2. **Create PR**
   - Use descriptive title following commit convention
   - Reference related issues: "Closes #123"
   - Provide clear description of changes
   - Include test results and performance metrics if applicable

3. **PR Checklist**
   - [ ] All tests passing
   - [ ] No compilation errors (`go build -gcflags="-e" ./...`)
   - [ ] Linter passing
   - [ ] Documentation updated
   - [ ] Performance requirements met (< 1ms routing, < 50ms API p99)

## Git Configuration

### Recommended Global Settings

```bash
# Set your identity
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"

# Enable colored output
git config --global color.ui auto

# Set default branch name
git config --global init.defaultBranch main

# Enable rerere (reuse recorded resolution)
git config --global rerere.enabled true

# Set pull strategy to rebase
git config --global pull.rebase true
```

### Useful Aliases

Add these to your `~/.gitconfig`:

```ini
[alias]
    # View pretty log
    lg = log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit
    
    # Show current branch
    br = branch --show-current
    
    # Interactive rebase
    ri = rebase -i
    
    # Amend last commit
    amend = commit --amend --no-edit
    
    # Undo last commit (keep changes)
    undo = reset HEAD~1 --soft
    
    # Clean up merged branches
    cleanup = "!git branch --merged | grep -v '\\*\\|main' | xargs -n 1 git branch -d"
```

## Common Scenarios

### Squashing Commits

Before merging, squash related commits:

```bash
# Interactive rebase for last 3 commits
git rebase -i HEAD~3

# Mark commits to squash in editor
# Change 'pick' to 'squash' for commits to combine
```

### Cherry-Picking

Apply specific commits from another branch:

```bash
# Find commit hash
git log --oneline other-branch

# Cherry-pick commit
git cherry-pick <commit-hash>
```

### Stashing Changes

Temporarily save work in progress:

```bash
# Stash current changes
git stash push -m "WIP: feature description"

# List stashes
git stash list

# Apply latest stash
git stash pop

# Apply specific stash
git stash apply stash@{1}
```

### Reverting Changes

```bash
# Revert a merged commit
git revert <commit-hash>

# Revert a merge commit
git revert -m 1 <merge-commit-hash>
```

## Best Practices

1. **Commit Often**: Make small, logical commits
2. **Write Clear Messages**: Future developers (including you) will thank you
3. **Review Before Pushing**: Use `git diff` and `git status`
4. **Keep History Clean**: Squash WIP commits before merging
5. **Never Force Push to Main**: Only force push to your own feature branches
6. **Test Before Committing**: Run `make ci` to catch issues early
7. **Update Documentation**: Keep docs in sync with code changes

## Troubleshooting

### Accidentally Committed to Main

```bash
# Create a new branch with your changes
git branch feat/my-feature

# Reset main to origin
git reset --hard origin/main

# Switch to your feature branch
git checkout feat/my-feature
```

### Lost Commits After Reset

```bash
# Find lost commits
git reflog

# Restore commit
git cherry-pick <commit-hash>
```

### Large Files Causing Issues

```bash
# Remove large file from history
git filter-branch --tree-filter 'rm -f path/to/large/file' HEAD

# Or use BFG Repo-Cleaner for better performance
bfg --delete-files large-file.bin
```

## Additional Resources

- [Pro Git Book](https://git-scm.com/book)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Git Flow](https://nvie.com/posts/a-successful-git-branching-model/)
- Project's [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines