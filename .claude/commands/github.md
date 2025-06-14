Use GitHub CLI commands to analyze the repo, changes, create logical commits, and merge them to the main branch via a PR from current branch.

Branch handling: If on main, create feature branch using $ARGUMENTS or auto-generate name. If on feature branch, continue working there.

## Initial Setup

```bash
# Get repository info
REPO_NAME=$(gh repo view --json name -q .name)
REPO_OWNER=$(gh repo view --json owner -q .owner.login)
DEFAULT_BRANCH=$(gh repo view --json defaultBranch -q .defaultBranch)
CURRENT_BRANCH=$(git branch --show-current)

# Determine working branch
if [ "$CURRENT_BRANCH" = "$DEFAULT_BRANCH" ] || [ "$CURRENT_BRANCH" = "main" ] || [ "$CURRENT_BRANCH" = "master" ]; then
    # We're on main, need to create a feature branch
    BRANCH_NAME="${ARGUMENTS:-feature/auto-$(date +%Y%m%d-%H%M%S)}"
    echo "Currently on $CURRENT_BRANCH, creating feature branch: $BRANCH_NAME"
    git checkout -b "$BRANCH_NAME"
else
    # Already on a feature branch, stay here
    BRANCH_NAME="$CURRENT_BRANCH"
    echo "Working on existing branch: $BRANCH_NAME"
fi
```

## Analyze Current Status

```bash
echo -e "\n=== Current Repository Status ==="
git status

# Get lists of different types of changes
UNTRACKED_FILES=$(git ls-files --others --exclude-standard)
MODIFIED_FILES=$(git diff --name-only)
DELETED_FILES=$(git ls-files --deleted)
STAGED_FILES=$(git diff --cached --name-only)

# Show summary
echo -e "\n=== Change Summary ==="
[ -n "$UNTRACKED_FILES" ] && echo "New files: $(echo "$UNTRACKED_FILES" | wc -l)"
[ -n "$MODIFIED_FILES" ] && echo "Modified files: $(echo "$MODIFIED_FILES" | wc -l)"
[ -n "$DELETED_FILES" ] && echo "Deleted files: $(echo "$DELETED_FILES" | wc -l)"
[ -n "$STAGED_FILES" ] && echo "Already staged: $(echo "$STAGED_FILES" | wc -l)"

# Exit if no changes
if [ -z "$UNTRACKED_FILES" ] && [ -z "$MODIFIED_FILES" ] && [ -z "$DELETED_FILES" ] && [ -z "$STAGED_FILES" ]; then
    echo "No changes to commit"
    exit 0
fi
```

## Group and Commit Changes

### Group 1: Configuration and Setup Files

```bash
# Stage config files
CONFIG_FILES=$(git status --porcelain | grep -E "(\.(json|yml|yaml|toml|ini|env|config)|package\.json|requirements\.txt|Gemfile|pom\.xml|build\.gradle)" | awk '{print $2}')

if [ -n "$CONFIG_FILES" ]; then
    echo -e "\n=== Committing configuration changes ==="
    echo "$CONFIG_FILES" | xargs -r git add
    git diff --cached --stat

    # Determine specific config type for message
    if echo "$CONFIG_FILES" | grep -q "package.json"; then
        MSG="chore: update dependencies"
    elif echo "$CONFIG_FILES" | grep -q "requirements.txt"; then
        MSG="chore: update Python dependencies"
    else
        MSG="chore: update configuration files"
    fi

    git commit -m "$MSG"
fi
```

### Group 2: Documentation Changes

```bash
# Stage documentation
DOC_FILES=$(git status --porcelain | grep -E "(\.(md|rst|txt)|README|CHANGELOG|LICENSE|docs/)" | grep -v "requirements.txt" | awk '{print $2}')

if [ -n "$DOC_FILES" ]; then
    echo -e "\n=== Committing documentation changes ==="
    echo "$DOC_FILES" | xargs -r git add
    git diff --cached --stat

    if echo "$DOC_FILES" | grep -q "README"; then
        MSG="docs: update README"
    elif echo "$DOC_FILES" | grep -q "CHANGELOG"; then
        MSG="docs: update CHANGELOG"
    else
        MSG="docs: update documentation"
    fi

    git commit -m "$MSG"
fi
```

### Group 3: Test Files

```bash
# Stage test files
TEST_FILES=$(git status --porcelain | grep -E "(test_|_test\.|\.test\.|spec\.|test/|tests/|__tests__|\.spec\.)" | awk '{print $2}')

if [ -n "$TEST_FILES" ]; then
    echo -e "\n=== Committing test changes ==="
    echo "$TEST_FILES" | xargs -r git add
    git diff --cached --stat

    git commit -m "test: update test cases"
fi
```

### Group 4: Source Code by Directory

```bash
# Group source files by top-level directory
SOURCE_DIRS=$(git status --porcelain | grep -vE "(test_|_test|spec|/tests?/|__tests__|docs/|\.(md|txt|json|ya?ml|toml)$)" | awk '{print $2}' | grep -v "^[.]" | cut -d'/' -f1 | sort -u)

for DIR in $SOURCE_DIRS; do
    DIR_FILES=$(git status --porcelain | grep "^.. $DIR" | awk '{print $2}')

    if [ -n "$DIR_FILES" ]; then
        echo -e "\n=== Committing changes in $DIR ==="
        echo "$DIR_FILES" | xargs -r git add
        git diff --cached --stat

        # Analyze the type of changes
        if git diff --cached --name-only | grep -q "\.tsx\?$\|\.jsx\?$"; then
            TYPE="feat"
            SCOPE="$DIR"
        elif git diff --cached --name-only | grep -q "\.css$\|\.scss$\|\.sass$"; then
            TYPE="style"
            SCOPE="$DIR"
        else
            TYPE="feat"
            SCOPE="$DIR"
        fi

        # Create descriptive message based on changes
        ADDED=$(git diff --cached --numstat | awk '{added+=$1} END {print added}')
        REMOVED=$(git diff --cached --numstat | awk '{removed+=$2} END {print removed}')

        if [ "$REMOVED" -gt "$ADDED" ]; then
            ACTION="refactor"
        else
            ACTION="update"
        fi

        git commit -m "$TYPE($SCOPE): $ACTION $(echo "$DIR_FILES" | wc -l) files"
    fi
done
```

### Group 5: Remaining Files

```bash
# Commit any remaining files
REMAINING=$(git status --porcelain | grep -v "^??" | awk '{print $2}')

if [ -n "$REMAINING" ]; then
    echo -e "\n=== Committing remaining changes ==="
    git add -A
    git diff --cached --stat

    FILE_COUNT=$(git diff --cached --name-only | wc -l)
    git commit -m "chore: update $FILE_COUNT miscellaneous files"
fi
```

### Group 6: Handle Untracked Files

```bash
# Handle new files that weren't caught above
UNTRACKED=$(git ls-files --others --exclude-standard)

if [ -n "$UNTRACKED" ]; then
    echo -e "\n=== Adding new files ==="
    echo "$UNTRACKED"
    git add -A
    git diff --cached --stat

    NEW_COUNT=$(git diff --cached --name-only | wc -l)
    git commit -m "feat: add $NEW_COUNT new files"
fi
```

## Push to Remote

```bash
echo -e "\n=== Pushing to remote ==="

# Check if remote branch exists
if git ls-remote --heads origin "$BRANCH_NAME" | grep -q "$BRANCH_NAME"; then
    echo "Pushing to existing remote branch"
    git push
else
    echo "Creating new remote branch"
    git push -u origin "$BRANCH_NAME"
fi

# Show commit history
echo -e "\n=== Recent Commits ==="
git log --oneline -10
```

## Create or Update Pull Request

```bash
echo -e "\n=== Checking for existing PR ==="

# Check if PR already exists for this branch
EXISTING_PR=$(gh pr list --head "$BRANCH_NAME" --json number,state -q '.[] | select(.state == "OPEN") | .number')

if [ -n "$EXISTING_PR" ]; then
    echo "Found existing PR #$EXISTING_PR"
    PR_NUMBER=$EXISTING_PR

    # Update PR description with latest commits
    echo "Updating PR description..."

    PR_BODY="## Summary
Automated PR with systematic commits grouped by type.

## Latest Commits
$(git log $DEFAULT_BRANCH..$BRANCH_NAME --pretty=format:"- %s" | head -20)

## Files Changed
$(git diff $DEFAULT_BRANCH...$BRANCH_NAME --stat | tail -20)

## Checklist
- [ ] Code review approved
- [ ] CI/CD checks pass
- [ ] Ready to merge"

    gh pr edit $PR_NUMBER --body "$PR_BODY"
    PR_URL=$(gh pr view $PR_NUMBER --json url -q .url)

else
    echo "Creating new PR..."

    # Generate PR body with all commits
    PR_BODY="## Summary
Automated PR with systematic commits grouped by type.

## Commits
$(git log $DEFAULT_BRANCH..$BRANCH_NAME --pretty=format:"- %s")

## Files Changed
$(git diff $DEFAULT_BRANCH...$BRANCH_NAME --stat)

## Checklist
- [ ] Code review approved
- [ ] CI/CD checks pass
- [ ] Ready to merge"

    # Create PR
    PR_URL=$(gh pr create \
        --title "Update from $BRANCH_NAME" \
        --body "$PR_BODY" \
        --base "$DEFAULT_BRANCH")

    PR_NUMBER=$(echo $PR_URL | grep -oE '[0-9]+$')
    echo "Created PR #$PR_NUMBER"
fi

echo "PR URL: $PR_URL"
```

## Wait for Checks

```bash
echo -e "\n=== Waiting for CI checks ==="
gh pr checks $PR_NUMBER --watch --timeout 300 || {
    echo "CI checks failed or timed out"
    echo "PR URL: $PR_URL"
    echo "You can continue working on branch: $BRANCH_NAME"
    exit 1
}
```

## Merge Using GitHub MCP

```bash
echo -e "\n=== Merging PR via GitHub MCP ==="

# First check PR status
echo "Checking PR mergeable status..."
gh pr view $PR_NUMBER --json mergeable,mergeStateStatus,isDraft

# Check if it's a draft
if gh pr view $PR_NUMBER --json isDraft -q .isDraft | grep -q "true"; then
    echo "PR is still in draft state. Cannot merge."
    exit 1
fi
```

Request the merge using the GitHub MCP `merge_pull_request` tool with these parameters:

- `owner`: `$REPO_OWNER`
- `repo`: `$REPO_NAME`
- `pullNumber`: `$PR_NUMBER`
- `commit_title`: “Merge pull request #$PR_NUMBER from $BRANCH_NAME”
- `commit_message`: “Automated merge with $(git rev-list $DEFAULT_BRANCH..$BRANCH_NAME –count) commits”
- `merge_method`: “squash”

## Post-Merge Actions

```bash
# Check if merge was successful
if gh pr view $PR_NUMBER --json state -q .state | grep -q "MERGED"; then
    echo -e "\n=== PR merged successfully ==="

    # Switch to default branch and update
    git checkout $DEFAULT_BRANCH
    git pull origin $DEFAULT_BRANCH

    # Delete local feature branch
    git branch -D "$BRANCH_NAME"

    echo "✓ Successfully merged all changes to $DEFAULT_BRANCH"
    echo "✓ Deleted feature branch: $BRANCH_NAME"
    echo "✓ Now on branch: $DEFAULT_BRANCH"

    # Show the merge commit
    git log -1 --oneline
else
    echo "⚠️  PR was not merged. Manual intervention required."
    echo "PR URL: $PR_URL"
    echo "Staying on branch: $BRANCH_NAME"
fi
```

## Final Status

```bash
echo -e "\n=== Final Repository Status ==="
git status
echo -e "\nCurrent branch: $(git branch --show-current)"
```
