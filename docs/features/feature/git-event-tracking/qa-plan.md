---
status: proposed
scope: feature/git-event-tracking
ai-priority: high
tags: [v7, testing]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-git-event-tracking
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Git Event Tracking — QA Plan

## Test Scenarios

### Scenario 1: Hook Installation & Simple Commit
#### Setup:
- Test Git repository initialized
- Kaboom MCP server running
- Hooks not yet installed

#### Steps:
1. Run `kaboom install-hooks /path/to/repo`
2. Verify hooks installed in `.git/hooks/`
3. Make a simple commit: `git commit -m "Test commit"`
4. Query `observe({what: 'git-events', type: 'git:commit'})`

#### Expected Result:
- Hooks installed successfully
- Post-commit hook executed
- Event emitted with commit metadata
- Event queryable and contains correct data

#### Acceptance Criteria:
- [ ] `.git/hooks/post-commit` exists and is executable
- [ ] Post-commit hook exits with status 0 (doesn't block git)
- [ ] git:commit event emitted within 1 second of commit
- [ ] Event contains: commit_hash, author, message, branch
- [ ] Event queryable via observe()

---

### Scenario 2: Commit Metadata Extraction
#### Setup:
- Hooks installed
- Repository with multiple files

#### Steps:
1. Modify 3 files: `src/file1.js`, `src/file2.js`, `src/file3.js`
2. Add new file: `src/file4.js`
3. Delete old file: `src/old.js`
4. Commit with message: "Refactor components"
5. Query git:commit event

#### Expected Result:
- Event contains all modified files with change type (M, A, D)
- Insertions and deletions counted correctly
- Author extracted from Git config
- Message preserved exactly

#### Acceptance Criteria:
- [ ] All 5 files listed in event
- [ ] File statuses correct: 3 M's, 1 A, 1 D
- [ ] Insertions/deletions are accurate (within 10% variance)
- [ ] Author name correct
- [ ] Commit message preserved

---

### Scenario 3: Branch Switching (Checkout)
#### Setup:
- Multiple branches exist in repo: stable, feature-x, feature-y

#### Steps:
1. Start on stable branch
2. Run `git checkout feature-x`
3. Query `observe({what: 'git-events', type: 'git:checkout'})`
4. Verify branch switch event

#### Expected Result:
- git:checkout event emitted
- Event shows: from_branch="stable", to_branch="feature-x"
- Commit hash of feature-x head included
- Event timestamp matches checkout time

#### Acceptance Criteria:
- [ ] Checkout event emitted within 1 second
- [ ] from_branch and to_branch are correct
- [ ] Commit hash is feature-x's HEAD
- [ ] Event immediately available in query

---

### Scenario 4: Detached HEAD State
#### Setup:
- Repository with multiple commits

#### Steps:
1. Check out a specific commit: `git checkout abc1234`
2. Verify detached HEAD state
3. Query git events
4. Verify git:checkout event handles detached HEAD

#### Expected Result:
- git:checkout event emitted
- Branch field shows "HEAD" or empty (detached indicator)
- Commit hash is the checked-out commit
- No errors or warnings from hook

#### Acceptance Criteria:
- [ ] Event emitted despite detached state
- [ ] Branch field indicates detached state
- [ ] Commit hash is correct
- [ ] Hook doesn't fail

---

### Scenario 5: Merge Operation
#### Setup:
- Two branches with independent commits

#### Steps:
1. On stable branch: `git merge feature-x`
2. Query for merge events: `observe({what: 'git-events', type: 'git:merge*'})`

#### Expected Result:
- git:merge:start event when merge begins
- git:merge:complete event on success
- Merge event contains: merge_branch, commit_hash (merge commit)
- Total merge time tracked

#### Acceptance Criteria:
- [ ] Both start and complete events emitted
- [ ] Merge commit hash captured
- [ ] merge_branch is "feature-x"
- [ ] No conflict events (clean merge)

---

### Scenario 6: Merge Conflict
#### Setup:
- Two branches with conflicting changes

#### Steps:
1. On stable branch: `git merge feature-x`
2. Conflicts detected
3. Resolve conflicts manually
4. Query merge events

#### Expected Result:
- git:merge:start event emitted
- git:merge:conflict event emitted with conflicted files listed
- After resolution: git:merge:complete event
- Timeline shows conflict resolution time

#### Acceptance Criteria:
- [ ] Conflict event lists all conflicted files
- [ ] Conflict event emitted before completion
- [ ] Completion event emitted after manual resolution
- [ ] Total merge time includes conflict resolution

---

### Scenario 7: Rebase Operation
#### Setup:
- Feature branch with 3 commits, main has new commits

#### Steps:
1. On feature branch: `git rebase stable`
2. All commits replay cleanly
3. Query for rebase events

#### Expected Result:
- git:rebase:start event with base_branch="stable", commits=3
- No conflict events
- git:rebase:complete event on success
- Rebase time captured

#### Acceptance Criteria:
- [ ] Rebase start event shows correct number of commits
- [ ] Base branch is "stable"
- [ ] Complete event emitted after rebase finishes
- [ ] All commits replayed (verified by commit hash)

---

### Scenario 8: Rebase with Conflicts
#### Setup:
- Feature branch with conflicts when rebasing onto stable

#### Steps:
1. On feature branch: `git rebase stable`
2. Conflict detected, interactive resolution required
3. Resolve conflicts, continue rebase
4. Query rebase events

#### Expected Result:
- git:rebase:start event
- git:rebase:conflict event with conflicted files
- git:rebase:complete event after resolution
- Timeline shows conflict resolution time

#### Acceptance Criteria:
- [ ] Conflict event lists all conflicted files
- [ ] Start event shows correct commit count
- [ ] Complete event shows actual replayed commits
- [ ] Time tracking includes conflict resolution

---

### Scenario 9: Amend Commit
#### Setup:
- Recent commit with a mistake

#### Steps:
1. Make changes to fix recent commit
2. Run `git commit --amend`
3. Query git events

#### Expected Result:
- git:commit event emitted for amended commit
- Event shows files in amended commit
- Previous commit is still visible (history preserved)

#### Acceptance Criteria:
- [ ] Amend commit event emitted
- [ ] Updated files listed
- [ ] New commit hash captured
- [ ] Event queryable

---

### Scenario 10: Test Session Correlation with Git
#### Setup:
- Test running with Git hooks installed
- Developer commits code
- Test starts after commit

#### Steps:
1. Commit code: `git commit -m "Fix feature"`
2. Commit hash recorded: "abc123"
3. Run test: `jest some.spec.js`
4. Test reads Git state and captures branch, commit
5. Query test session

#### Expected Result:
- Test session metadata includes:
  - git_branch: "feature-branch"
  - git_commit: "abc123"
  - git_dirty: false (no uncommitted changes)
- Test session correlated with git:commit event
- Timeline shows commit → test run

#### Acceptance Criteria:
- [ ] Test session has correct branch and commit
- [ ] Commit event and test event share timeline
- [ ] Dirty flag accurate (true if uncommitted changes)
- [ ] Can query: "all tests on this branch"

---

### Scenario 11: Uncommitted Changes Detection
#### Setup:
- Test running with uncommitted changes in working directory

#### Steps:
1. Modify file without committing
2. Run test
3. Query test session

#### Expected Result:
- Test session git_dirty: true
- Changed files listed
- Developer aware test ran with uncommitted code

#### Acceptance Criteria:
- [ ] git_dirty flag is true
- [ ] Changed files list is accurate
- [ ] Test still completes (dirty state doesn't block test)

---

### Scenario 12: Hook Performance
#### Setup:
- Large repository (1000+ files)
- Repository with deep history

#### Steps:
1. Make 50 commits in quick succession
2. Measure hook execution time for each
3. Measure total impact on git performance

#### Expected Result:
- Average hook time: <10ms per commit
- Git operations not noticeably slowed
- No queue buildup in events
- All events emitted

#### Acceptance Criteria:
- [ ] Hook execution: <10ms (median)
- [ ] Git commit time not increased >5%
- [ ] No events lost due to hook slowness
- [ ] Works with large diffs (1000+ files)

---

## Acceptance Criteria (Overall)
- [ ] All 12 scenarios pass
- [ ] Hooks installed without breaking Git
- [ ] All Git operations tracked: commit, checkout, merge, rebase
- [ ] Metadata accurate: commit hash, author, files, branch
- [ ] Performance: <10ms hook execution
- [ ] Test sessions correlated with Git state
- [ ] Edge cases handled: detached HEAD, conflicts, amend
- [ ] Hooks can be uninstalled cleanly

## Test Data

### Fixture: Sample Git Commit Event
```json
{
  "type": "git:commit",
  "service": "git",
  "fields": {
    "commit_hash": "a7f8e3d9c1b2e4f6abc1234567890def",
    "author": "Alice Developer",
    "email": "alice@example.com",
    "message": "Fix payment timeout handling",
    "branch": "feature/payment-timeout",
    "files_changed": 3,
    "insertions": 45,
    "deletions": 12,
    "files": [
      {
        "path": "src/utils/payment.js",
        "status": "M",
        "additions": 30,
        "deletions": 8
      },
      {
        "path": "src/tests/payment.spec.js",
        "status": "M",
        "additions": 15,
        "deletions": 4
      }
    ]
  }
}
```

### Fixture: Sample Git Checkout Event
```json
{
  "type": "git:checkout",
  "service": "git",
  "fields": {
    "from_branch": "feature/payment-timeout",
    "to_branch": "stable",
    "commit": "abc1234567890def"
  }
}
```

### Fixture: Sample Git Merge Conflict Event
```json
{
  "type": "git:merge:conflict",
  "service": "git",
  "fields": {
    "merge_branch": "feature/new-ui",
    "conflicts": [
      "src/components/Header.js",
      "src/styles/main.css"
    ]
  }
}
```

### Test Repository Setup
```bash
# Create test repo with multiple branches
git init test-repo
cd test-repo
echo "initial" > file.txt
git add file.txt
git commit -m "Initial commit"

# Create feature branch
git checkout -b feature-branch
echo "feature code" >> file.txt
git commit -am "Add feature"

# Go back to main and make conflicting change
git checkout stable
echo "main code" >> file.txt
git commit -am "Main change"

# Attempt merge to trigger conflict
git merge feature-branch  # Will have conflict
```

## Regression Tests

### Hook Reliability
- [ ] Hook executes even with large diffs (1000+ files)
- [ ] Hook doesn't block Git on network errors
- [ ] Hook works with automated commits (CI/CD, git bisect)
- [ ] Hook works with rebase -i (interactive rebase)

### Event Accuracy
- [ ] Commit hash matches `git rev-parse HEAD`
- [ ] Author matches `git log -1 --format=%an`
- [ ] Files match `git diff HEAD~1 HEAD --name-status`
- [ ] Branch matches `git rev-parse --abbrev-ref HEAD`

### Edge Cases
- [ ] Empty commits (--allow-empty): Event emitted
- [ ] Merge commits: Multiple parents handled
- [ ] Squash merge: Single commit or multiple?
- [ ] Fast-forward merge: No new commit hash?

### Performance & Stability
- [ ] Hook doesn't accumulate memory over time
- [ ] No file descriptor leaks
- [ ] Works across multiple concurrent Git operations
- [ ] Graceful handling of repo corruption scenarios
