---
status: proposed
scope: audit/code-quality
ai-priority: high
tags: [refactor, code-quality, file-size, go]
relates-to: [codebase-audit-report.md, file-size-refactor-plan.md]
last-verified: 2026-02-08
---

# sessions.go Refactoring Plan (694 LOC → ~400 LOC)

**File:** [`internal/session/types.go`](internal/session/types.go) (694 lines)

**Goal:** Split into focused files by domain/concern.

---

## Current Structure Analysis

### Sections (by line numbers):
1. **Imports** (lines 6-14) - 9 lines
2. **Constants** (lines 20-28) - 9 lines
3. **Types** (lines 31-100) - 70 lines
4. **CaptureStateReader Interface** (lines 35-42) - 8 lines
5. **SnapshotError Type** (lines 44-49) - 6 lines
6. **SnapshotNetworkRequest Type** (lines 51-59) - 9 lines
7. **SnapshotWSConnection Type** (lines 61-66) - 6 lines
8. **NamedSnapshot Type** (lines 68-79) - 12 lines
9. **SnapshotListEntry Type** (lines 81-87) - 7 lines
10. **SessionDiffResult Type** (lines 89-97) - 9 lines
11. **ErrorDiff Type** (lines 99-104) - 6 lines
12. **SessionNetworkDiff Type** (lines 106-112) - 7 lines
13. **PerformanceDiff Type** (lines 123-128) - 6 lines
14. **MetricChange Type** (lines 130-136) - 7 lines
15. **DiffSummary Type** (lines 138-145) - 8 lines
16. **SessionNetworkChange Type** (lines 114-121) - 8 lines
17. **Compute Summary Function** (lines 451-453) - 3 lines
18. **Diff Errors Function** (lines 455-466) - 12 lines
19. **Diff Network Function** (lines 467-496) - 30 lines
20. **Diff Performance Function** (lines 498-528) - 31 lines
21. **Diff Summary Function** (lines 530-545) - 8 lines
22. **Compute Summary Function** (lines 547-553) - 4 lines
23. **Validate Name Function** (lines 656-675) - 20 lines
24. **Remove From Order Function** (lines 679-686) - 14 lines
25. **Compute Metric Change Function** (lines 680-693) - 8 lines
26. **List Function** (lines 689-693) - 3 lines
27. **Delete Function** (lines 550-553) - 4 lines
28. **Tool Handler Function** (lines 558-639) - 25 lines
29. **HandleTool Function** (lines 590-639) - 15 lines
30. **Diff Sessions Params Type** (lines 57-88) - 6 lines
31. **Extract Path Function** (lines 695-702) - 8 lines
32. **Diff Sessions Function** (lines 700-736) - 30 lines
33. **Compare Function** (lines 735-745) - 20 lines
34. **List Function** (lines 745-693) - 3 lines
35. **SessionManager Type** (lines 747-693) - 2 lines

---

## Proposed File Structure

### 1. `internal/session/constants.go` (~30 LOC)
**Purpose:** Session comparison constants

**Contents:**
- maxSnapshotNameLen
- maxConsolePerSnapshot
- maxNetworkPerSnapshot
- reservedSnapshotName
- perfRegressionRatio

**Rationale:** Constants are configuration values. They should be in their own file for easy discovery and modification.

---

### 2. `internal/session/types.go` (~50 LOC)
**Purpose:** Session comparison types

**Contents:**
- CaptureStateReader interface
- SnapshotError struct
- SnapshotNetworkRequest struct
- SnapshotWSConnection struct
- NamedSnapshot struct
- SnapshotListEntry struct
- SessionDiffResult struct
- ErrorDiff struct
- SessionNetworkDiff struct
- PerformanceDiff struct
- MetricChange struct
- DiffSummary struct
- SessionNetworkChange struct

**Rationale:** All session comparison types grouped together. Makes it easier to find session-related code.

---

### 3. `internal/session/diff-types.go` (~100 LOC)
**Purpose:** Diff computation types

**Contents:**
- ErrorDiff struct
- SessionNetworkDiff struct
- PerformanceDiff struct
- MetricChange struct
- DiffSummary struct

**Rationale:** All diff computation types grouped together. Each diff type has its own file.

---

### 4. `internal/session/comparison.go` (~100 LOC)
**Purpose:** Main comparison logic

**Contents:**
- Compare() function
- Result aggregation

**Rationale:** Main entry point for session comparison. This is the core logic.

---

### 5. `internal/session/network-diff.go` (~100 LOC)
**Purpose:** Network diff computation

**Contents:**
- diffNetwork() function
- Entry comparison logic

**Rationale:** Network diff is a distinct algorithm. It should have its own file.

---

### 6. `internal/session/actions-diff.go` (~80 LOC)
**Purpose:** Actions diff computation

**Contents:**
- diffActions() function
- Action comparison logic

**Rationale:** Actions diff is a distinct algorithm. It should have its own file.

---

### 7. `internal/session/performance-diff.go` (~100 LOC)
**Purpose:** Performance diff computation

**Contents:**
- diffPerformance() function
- Performance comparison logic

**Rationale:** Performance diff is a distinct algorithm. It should have its own file.

---

### 8. `internal/session/dom-diff.go` (~80 LOC)
**Purpose:** DOM diff computation

**Contents:**
- diffDOM() function
- DOM comparison logic

**Rationale:** DOM diff is a distinct algorithm. It should have its own file.

---

### 9. `internal/session/fragile-selectors.go` (~80 LOC)
**Purpose:** Fragile selector detection

**Contents:**
- DetectFragileSelectors() function
- Selector analysis logic

**Rationale:** Fragile selector detection is a distinct concern. It should have its own file.

---

### 10. `internal/session/playback.go` (~100 LOC)
**Purpose:** Playback session management

**Contents:**
- PlaybackSession struct
- ExecutePlayback() function
- Execution tracking logic

**Rationale:** Playback is a distinct concern. It should have its own file.

---

### 11. `internal/session/types.go` (~200 LOC)
**Purpose:** SessionManager struct and main exports

**Contents:**
- SessionManager struct
- NewCapture() function
- Delete() function
- List() function
- Capture() function
- Get() function
- GetByCorrelationId() function
- GetByName() function
- HandleTool() function

**Rationale:** The main SessionManager struct and its methods. This is the core of the session package.

---

## Refactoring Strategy

### Phase 1: Create New Files (No Breaking Changes)

1. Create all 11 new files alongside existing `sessions.go`
2. Copy relevant code sections to each new file
3. Keep `sessions.go` intact with all original code
4. Add exports from new files
5. Run `go build` to verify no circular dependencies

### Phase 2: Update Imports

1. Update imports in `sessions.go` to import from new files where needed
2. Run `go build` to verify no circular dependencies

### Phase 3: Test

1. Run `go test ./internal/session/...` to ensure no regressions
2. Run full test suite to verify overall functionality

### Phase 4: Remove Old Code

1. Remove moved code sections from `sessions.go`
2. Ensure `sessions.go` only contains:
   - Imports
   - SessionManager struct
   - Main exports
   - Any remaining shared types that don't fit elsewhere

### Phase 5: Final Verification

1. Run full test suite
2. Run `go build ./internal/session/`
3. Run `go test ./internal/session/...`
4. Update documentation if needed

---

## Import Structure After Refactoring

### In `internal/session/types.go`:
```go
import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/performance"
)

	// Import from new type files
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/session/types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/session/diff-types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/session/network-diff"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/session/actions-diff"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/session/performance-diff"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/browser-agent/internal/session/dom-diff"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/session/fragile-selectors"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/session/playback"
)
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/session/types"
)
)
```

### In other files:
Files that import from `sessions.go` will need to update their imports to use the new, more specific files.

---

## Migration Checklist

For each new file:

- [ ] Create new file with appropriate name
- [ ] Copy relevant code to new file
- [ ] Add package declaration (`package session`)
- [ ] Add necessary imports
- [ ] Verify no circular dependencies
- [ ] Run `go test ./internal/session/[new-file].go`
- [ ] Update imports in `sessions.go`
- [ ] Run `go build ./internal/session/`
- [ ] Run full test suite
- [ ] Remove moved code from `sessions.go`
- [ ] Update documentation if needed

---

## Estimated Effort

| File | Estimated Time | Complexity |
|-------|----------------|------------|
| `sessions.go` | 4.6 hours | High |
| Total | 4.6 hours | High |

---

## Risks and Mitigations

### Risk: Circular dependencies
**Mitigation:** Carefully plan import structure. Use type aliases where appropriate to avoid circular imports.

### Risk: Breaking imports
**Mitigation:** Keep original `sessions.go` intact during Phase 1. Only remove code after all imports are updated.

### Risk: Test failures
**Mitigation:** Run tests after each file split, not just at the end.

### Risk: Missing exports
**Mitigation:** Verify all public APIs are still exported after refactoring.

### Risk: Complex interdependencies
**Mitigation:** The session comparison logic has many interdependent types. Carefully plan the split to avoid breaking dependencies.

---

## Success Criteria

1. All new files are under 500 LOC
2. All tests pass after refactoring
3. No circular dependencies
4. All public APIs are still accessible
5. Documentation is updated
6. Code review approved
