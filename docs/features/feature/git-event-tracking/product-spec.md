---
status: proposed
scope: feature/git-event-tracking
ai-priority: high
tags: [v7, developer-workflows, context, ears]
relates-to: [../test-execution-capture.md, ../../core/architecture.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-git-event-tracking
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Git Event Tracking

## Overview
Git Event Tracking automatically captures Git operations (commits, branch switches, rebases, merges) and emits them as events into Kaboom. When a developer commits code, Kaboom records the commit hash, message, changed files, and author. This provides crucial context: "Did the test failure happen before or after the code change? On which branch? After rebasing onto stable?" Git events are correlated with test runs, network requests, and backend logs to answer questions like "When did I introduce this bug?" or "Does the failure only happen on this branch?". The complete development workflow—code changes, test executions, feature flag toggles—is visible in a single timeline.

## Problem
Developers use Git to manage code and branches, but Kaboom has no visibility into these operations:
- A test fails, but was the code change applied? On which branch?
- Kaboom shows a backend error starting at timestamp X, but which commit introduced it?
- A developer switches branches, but Kaboom doesn't know which code is running
- Test flakiness analysis can't distinguish "flaky before my change" vs. "flaky after my change"
- Integration with CI/CD requires manual correlation: "Did the test failure happen before or after this commit?"

## Solution
Git Event Tracking integrates with Git via:
1. **Git Hooks:** Post-commit, post-checkout hooks emit events
2. **Git API:** Query Git status on demand to determine current branch, uncommitted changes
3. **Event Correlation:** Each code change is tagged with commit hash, test runs are tagged with current branch
4. **Timeline View:** Developers see code changes aligned with test results and system behavior

## User Stories
- As a developer, I want to see my Git commits aligned with test execution so that I can immediately spot when a code change broke a test
- As a QA engineer, I want to know which branch and commit a test ran against so that I can reproduce failures accurately
- As a DevOps engineer, I want to correlate CI/CD deployments with Git commits so that I can track which code is running in production
- As a team lead, I want to see which developer's commit caused a regression so that I can facilitate knowledge sharing

## Acceptance Criteria
- [ ] Git post-commit hook emits event with commit hash, message, changed files, author
- [ ] Git post-checkout hook emits event with branch switch (from/to branches)
- [ ] Git rebase/merge events captured (start, abort, complete)
- [ ] Each test run tagged with current branch and commit hash
- [ ] Git status queryable: `observe({what: 'git-events', since: <timestamp>})`
- [ ] Changed files correlated with test failures (if test touches modified files)
- [ ] Handles detached HEAD, rebases, merges gracefully
- [ ] Integrates with CI/CD: git info available during test runs
- [ ] Performance: Hook execution <10ms

## Not In Scope
- Git repo creation or management
- Forced pushes or destructive operations (just log them)
- Remote tracking (only local operations)
- Blame analysis (which commit introduced a bug)
- Code review integration

## Data Structures

### Git Event Types
```go
// Commit Event
type GitCommitEvent struct {
    Timestamp    time.Time
    Type         string           // "git:commit"
    CommitHash   string           // Full SHA-1 hash
    Author       string           // Commit author name
    Email        string           // Author email
    Message      string           // Commit message (full)
    FilesChanged int              // Number of files changed
    Files        []FileChange     // List of changed files
    Insertions   int              // Lines added
    Deletions    int              // Lines deleted
    Branch       string           // Current branch at commit time
}

type FileChange struct {
    Path      string // "src/components/Button.js"
    Status    string // "A" (added), "M" (modified), "D" (deleted)
    Additions int
    Deletions int
}

// Branch Switch Event
type GitCheckoutEvent struct {
    Timestamp   time.Time
    Type        string    // "git:checkout"
    FromBranch  string
    ToBranch    string
    Commit      string    // Commit hash of new branch head
}

// Rebase Event
type GitRebaseEvent struct {
    Timestamp  time.Time
    Type       string    // "git:rebase:start", "git:rebase:complete", "git:rebase:abort"
    BaseBranch string
    Commits    int       // Number of commits being rebased
    Error      string    // If aborted
}

// Merge Event
type GitMergeEvent struct {
    Timestamp    time.Time
    Type         string    // "git:merge:start", "git:merge:complete", "git:merge:conflict"
    MergeBranch  string    // Branch being merged in
    CommitHash   string    // Merge commit hash (if successful)
    Conflicts    []string  // Files with conflicts
}
```

## Examples

### Example 1: Developer Commits Code, Test Fails
#### Git operation:
```bash
$ git add src/utils/payment.js
$ git commit -m "Fix payment timeout handling"
```

#### Kaboom events:
```
[10:15:23.100] git:commit
  - hash: a7f8e3d
  - message: "Fix payment timeout handling"
  - files_changed: 1
  - files: [{path: "src/utils/payment.js", status: "M"}]
  - branch: "feature/payment-timeout"

[10:15:35.200] test:started (trace-abc)
  - branch: "feature/payment-timeout"
  - commit: a7f8e3d

[10:15:37.500] test:completed (FAILED)
  - assertion: "expect(timeout).toBe(30000)"
```

**Developer insight:** "Oh, the test failed right after my commit. The timeout logic still isn't right. Let me check what else I modified."

### Example 2: Branch Switch Invalidates Test
#### Git operation:
```bash
$ git checkout stable
```

#### Kaboom event:
```
[10:16:00.100] git:checkout
  - from_branch: "feature/payment-timeout"
  - to_branch: "main"
  - commit: "abc1234"

[10:16:10.200] test:started (trace-def)
  - branch: "main"
  - commit: "abc1234"

[10:16:12.500] test:completed (PASSED)
```

**Developer insight:** "Interesting, the same test passes on stable but fails on my feature branch. Let me compare commits."

### Example 3: Rebase Conflicts
#### Git operation:
```bash
$ git rebase stable
# Conflicts detected
```

## Kaboom event:
```
[10:20:00.100] git:rebase:start
  - base_branch: "main"
  - commits: 5

[10:20:05.200] git:rebase:conflict
  - files: ["src/utils/payment.js", "src/api/checkout.js"]

[10:20:15.300] test:started (trace-ghi)
  - branch: "feature/payment-timeout (rebasing)"
  - commit: "manual-resolution"
```

## Integration with CI/CD
In CI, Git information is available as environment variables:
```bash
# GitHub Actions
export GIT_COMMIT="${GITHUB_SHA}"
export GIT_BRANCH="${GITHUB_HEAD_REF}"

# GitLab CI
export GIT_COMMIT="${CI_COMMIT_SHA}"
export GIT_BRANCH="${CI_COMMIT_REF_NAME}"

# Kaboom captures these and associates with test runs
```

## MCP Changes
New observable type:
```javascript
observe({
  what: 'git-events',
  since: '2026-01-31T10:00:00Z',
  type: 'git:commit',  // Optional: filter by event type
  branch: 'feature/*'  // Optional: wildcard filtering
})

// Returns git events with correlation to tests and other observables
```

## Hook Installation
Kaboom provides a setup command:
```bash
kaboom install-hooks <path-to-repo>
# → Creates .git/hooks/post-commit, post-checkout, etc.
# → Emits events to Kaboom MCP server
```

## Frontend Extension Integration
The extension detects Git events and updates UI:
- Shows current branch in header
- Highlights changed files in code view
- Shows commit message when hovering over timeline events
- Marks test runs with associated commit hash
