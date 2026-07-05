---
status: proposed
scope: feature/git-event-tracking
ai-priority: high
tags: [v7, developer-workflows]
relates-to: [product-spec.md, ../custom-event-api/tech-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-git-event-tracking
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Git Event Tracking — Technical Specification

## Architecture

### Data Flow
```
Local Git Repo
    ↓ .git/hooks/post-commit, post-checkout, etc.
Hook Scripts (shell/Go)
    ↓ Execute git commands, extract metadata
Custom Event API
    ↓ git:commit, git:checkout, git:merge events
Kaboom Event Store
    ↓ indexed by branch, commit hash
Test Session Correlator
    ↓ tag test runs with current branch + commit
Timeline View
    ↓ Show code changes alongside test results
```

### Components
1. **Git Hook Scripts** (`cmd/kaboom-install-hooks/`)
   - Post-commit hook: Extract commit info, emit event
   - Post-checkout hook: Detect branch switch, emit event
   - Post-merge hook: Emit merge completion event
   - Post-rebase hook: Emit rebase completion event
   - Scripts are small shell/Go binaries, <10KB each

2. **Git Metadata Extractor** (`server/git/extractor.go`)
   - Query `git log`, `git status`, `git branch` to extract metadata
   - Parse commit diff to get files changed, insertions, deletions
   - Extract author info from Git config
   - Handles edge cases: detached HEAD, empty commits, merges

3. **Test Session Enricher** (`server/test-session.go`)
   - When test starts, query current Git state
   - Capture: current branch, current commit, uncommitted changes
   - Tag test session with this metadata
   - Correlate test failure with changes in modified files

4. **Git Event Handler** (Custom Event API)
   - Git events emitted as regular custom events
   - Type: `git:commit`, `git:checkout`, `git:rebase:*`, `git:merge:*`
   - Indexed and queryable like any custom event

## Implementation Plan

### Phase 1: Hook Installation & Events (Week 1)
1. Create hook installation command
   - `kaboom install-hooks <repo-path>`
   - Creates `.git/hooks/post-commit`, post-checkout, post-merge
2. Implement Git metadata extractor
   - Parse `git log -1` for commit info
   - Parse `git diff HEAD~1 HEAD --stat` for file changes
3. Implement hook shell scripts that emit events via HTTP/gRPC

### Phase 2: Event Storage & Querying (Week 2)
1. Integrate with Custom Event API
   - Git events use `git:*` type prefixes
   - Store in event store alongside other events
2. Index by branch, commit hash
3. Implement query handler for git events

### Phase 3: Test Session Integration (Week 3)
1. On test start, query Git state
2. Capture: branch, commit, uncommitted changes
3. Tag test session with this metadata
4. Store in test session record

### Phase 4: Timeline & Visualization (Week 4)
1. Extend timeline view to show Git events
2. Show branch switches
3. Show commits with changed files
4. Link test failures to changed files

## API Changes

### Git Event Emission (Custom Event API)
```go
// Emitted as custom events with type "git:*"
emit({
    type: "git:commit",
    service: "git",
    fields: {
        commit_hash: "a7f8e3d9c1b2e4f6",
        author: "Alice Developer",
        email: "alice@example.com",
        message: "Fix payment timeout handling",
        branch: "feature/payment-timeout",
        files_changed: 1,
        insertions: 10,
        deletions: 5,
        files: [
            { path: "src/utils/payment.js", status: "M", additions: 10, deletions: 5 }
        ]
    }
})
```

### Git Hook Scripts
```bash
#!/bin/bash
# .git/hooks/post-commit

# Extract commit info
COMMIT_HASH=$(git rev-parse HEAD)
AUTHOR=$(git log -1 --format=%an)
EMAIL=$(git log -1 --format=%ae)
MESSAGE=$(git log -1 --format=%s)
BRANCH=$(git rev-parse --abbrev-ref HEAD)
STATS=$(git diff HEAD~1 HEAD --stat | tail -1)

# Emit event via HTTP
curl -X POST http://localhost:9090/events \
  -H "Content-Type: application/json" \
  -d @- <<EOF
{
  "type": "git:commit",
  "service": "git",
  "fields": {
    "commit_hash": "$COMMIT_HASH",
    "author": "$AUTHOR",
    "email": "$EMAIL",
    "message": "$MESSAGE",
    "branch": "$BRANCH"
  }
}
EOF
```

### Test Session Metadata
```go
type TestSession struct {
    // ... existing fields ...
    GitBranch      string      // "feature/payment-timeout"
    GitCommit      string      // "a7f8e3d"
    GitDirty       bool        // Uncommitted changes present
    ChangedFiles   []string    // Files modified since last commit
}
```

### MCP Query Handler
```go
// observe({what: 'git-events', type: 'git:commit', since: timestamp})
func handleGitEvents(req *GitEventsRequest) (*GitEventsResponse, error) {
    // Query custom events with type matching "git:*"
    // Support filtering by:
    // - type: "git:commit", "git:checkout", etc.
    // - branch: "feature/*"
    // - since: timestamp
    // - limit: max results
}
```

## Code References
- **Git hook installer:** `/Users/brenn/dev/kaboom/cmd/kaboom-install-hooks/main.go` (new)
- **Git metadata extractor:** `/Users/brenn/dev/kaboom/server/git/extractor.go` (new)
- **Hook shell scripts:** `/Users/brenn/dev/kaboom/scripts/hooks/` (new)
- **Test session enricher:** `/Users/brenn/dev/kaboom/server/test-session.go` (modified)
- **Custom event API:** Used as-is (git events are regular events)

## Performance Requirements
- **Hook execution:** <10ms per operation (commit, checkout)
- **Metadata extraction:** <50ms (git log parsing)
- **Event emission:** <1ms (async via custom event API)
- **Query git events:** <50ms for 1000 events
- **No impact on Git operations:** Hook must not slow down git itself

## Testing Strategy

### Unit Tests
- Mock Git commands, verify metadata extraction
- Test file change parsing (additions, deletions)
- Test branch name extraction
- Test commit message parsing (multiline, special characters)

### Integration Tests
- Create real Git repo, perform operations
- Verify hooks are called
- Verify events are emitted with correct metadata
- Test edge cases: empty commits, merge commits, rebases

### Performance Tests
- Measure hook execution time for 1000 commits
- Verify no slowdown to git operations
- Test with large diffs (1000+ file changes)

### E2E Tests
- Real developer workflow: clone repo, commit, branch switch
- Verify test sessions capture Git metadata
- Verify Git events appear in timeline
- Verify changed files are tracked accurately

## Dependencies
- **Git:** Must be installed and in PATH
- **Custom Event API:** Must be enabled to emit events
- **curl or similar:** To emit events from hooks

## Risks & Mitigation
1. **Hook installation fragile** (overwrites existing hooks)
   - Mitigation: Backup existing hooks, allow append mode
2. **Hook failure blocks Git operation**
   - Mitigation: Wrap in error handling, silent failure, don't block git
3. **Performance impact on frequent Git operations**
   - Mitigation: Async event emission, minimize git command calls
4. **Large diffs cause slow event emission**
   - Mitigation: Limit file list to first 100 files, indicate truncation
5. **Detached HEAD state edge cases**
   - Mitigation: Handle gracefully, emit branch="" for detached

## Hook Robustness

### Error Handling
```bash
#!/bin/bash
set -e  # Exit on error
trap 'exit 0' ERR  # Don't block git on failure

COMMIT_HASH=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "detached")

# Emit event, but don't fail if emission fails
curl -s -X POST http://localhost:9090/events \
  -H "Content-Type: application/json" \
  -d "{...}" \
  || true  # Ignore curl failure
```

### Uninstall & Cleanup
```bash
kaboom uninstall-hooks <repo-path>
# → Removes hook scripts safely
# → Restores backed-up hooks if present
```

## Backward Compatibility
- Git hooks are optional (install via command)
- Without hooks, Git events are simply not emitted
- Existing repos unaffected until user runs install-hooks
- Hooks can be uninstalled cleanly
