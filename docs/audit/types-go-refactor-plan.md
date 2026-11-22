---
status: proposed
scope: audit/code-quality
ai-priority: high
tags: [refactor, code-quality, file-size, go]
relates-to: [codebase-audit-report.md, file-size-refactor-plan.md]
last-verified: 2026-02-08
---

# types.go Refactoring Plan (653 LOC → ~400 LOC)

**File:** [`internal/composition/capture/types.go`](internal/composition/capture/types.go) (653 lines)

**Goal:** Split into focused files by domain/concern.

---

## Current Structure Analysis

### Sections (by line numbers):
1. **Imports** (lines 6-14) - 9 lines
2. **Abstracted Component Interfaces** (lines 17-50) - 34 lines
   - SchemaStore interface
   - CSPGenerator interface
   - ClientRegistry interface
3. **Type Aliases** (lines 52-66) - 15 lines
4. **Session Tracking Types** (lines 68-77) - 10 lines
   - SessionTracker struct
5. **Security Threat Flagging** (lines 79-92) - 14 lines
   - SecurityFlag struct
6. **Network Waterfall Types** (lines 94-119) - 26 lines
   - NetworkWaterfallEntry struct
   - NetworkWaterfallPayload struct
7. **WebSocket Types** (lines 121-242) - 122 lines
   - WebSocketEvent struct
   - WebSocketEventFilter struct
   - WebSocketStatusFilter struct
   - WebSocketStatusResponse struct
   - WebSocketConnection struct
   - WebSocketClosedConnection struct
   - WebSocketMessageRate struct
   - WebSocketDirectionStats struct
   - WebSocketLastMessage struct
   - WebSocketMessagePreview struct
   - WebSocketSchema struct
   - WebSocketSamplingStatus struct
8. **Network Body Types** (lines 244-250) - 7 lines
   - NetworkBody type alias
   - NetworkBodyFilter type alias
9. **Extension Logging Types** (lines 252-289) - 38 lines
   - ExtensionLog struct
   - PollingLogEntry struct
   - HTTPDebugEntry struct
10. **Enhanced Actions Types** (lines 291-322) - 32 lines
   - EnhancedAction struct
   - EnhancedActionFilter struct
   - Selectors map type
11. **Internal Types** (lines 324-399) - 76 lines
   - connectionState struct
   - directionStats struct
   - WebSocketMessagePreview struct
   - WebSocketLastMessage struct
   - WebSocketSchema struct
   - WebSocketSamplingStatus struct
12. **Constants** (lines 401-379) - 79 lines
   - Buffer capacity constants
   - Network waterfall capacity configuration
   - Circuit breaker configuration
   - Rate limiting configuration
13. **Sub-structs for Capture Composition** (lines 381-499) - 119 lines
   - NetworkWaterfallBuffer struct
   - ExtensionLogBuffer struct
   - WSConnectionTracker struct
14. **Timing and Performance Data** (lines 501-599) - 99 lines
   - A11yCache struct
   - A11yCacheEntry struct
   - A11yInflightEntry struct
   - PerformanceStore struct
15. **Session Data** (lines 601-607) - 7 lines
   - SessionTracker struct
16. **Composed Sub-structs** (lines 609-653) - 45 lines
   - Capture struct (main composition)
   - NewCapture() function
   - SetLifecycleCallback() function
   - emitLifecycleEvent() function
   - SetServerVersion() function
   - GetServerVersion() function
   - Close() function

---

## Proposed File Structure

### 1. `internal/composition/capture/interfaces.go` (~80 LOC)
**Purpose:** Abstracted component interfaces

**Contents:**
- SchemaStore interface
- CSPGenerator interface
- ClientRegistry interface

**Rationale:** These are abstracted interfaces that are implemented by other packages (analysis, security, session). Keeping them in a separate file makes the dependency graph clearer.

---

### 2. `internal/composition/capture/type-aliases.go` (~30 LOC)
**Purpose:** Type aliases for imported packages

**Contents:**
- All type aliases from lines 52-66
  - PerformanceSnapshot, PerformanceBaseline, PerformanceRegression
  - ResourceEntry, ResourceDiff, CausalDiffResult
  - Recording, RecordingAction
  - PendingQueryResponse, PendingQuery, CommandResult

**Rationale:** Type aliases are purely organizational. They don't contain logic, just type definitions for convenience.

---

### 3. `internal/composition/capture/session-types.go` (~50 LOC)
**Purpose:** Session tracking types

**Contents:**
- SessionTracker struct
- Related session tracking logic

**Rationale:** Session tracking is a distinct domain from capture types. It's used by session comparison logic.

---

### 4. `internal/composition/capture/security-types.go` (~50 LOC)
**Purpose:** Security threat flagging

**Contents:**
- SecurityFlag struct
- Related security type definitions

**Rationale:** Security threat detection is a distinct domain. It's used by security scanner.

---

### 5. `internal/composition/capture/network-types.go` (~200 LOC)
**Purpose:** Network waterfall types

**Contents:**
- NetworkWaterfallEntry struct
- NetworkWaterfallPayload struct
- NetworkWaterfallBuffer struct
- NetworkBody type alias
- NetworkBodyFilter type alias
- Related network type definitions

**Rationale:** All network-related types grouped together. Makes it easier to find network-related code.

---

### 6. `internal/composition/capture/websocket-types.go` (~250 LOC)
**Purpose:** WebSocket event and connection tracking types

**Contents:**
- WebSocketEvent struct
- WebSocketEventFilter struct
- WebSocketStatusFilter struct
- WebSocketStatusResponse struct
- WebSocketConnection struct
- WebSocketClosedConnection struct
- WebSocketMessageRate struct
- WebSocketDirectionStats struct
- WebSocketLastMessage struct
- WebSocketMessagePreview struct
- WebSocketSchema struct
- WebSocketSamplingStatus struct
- WSConnectionTracker struct
- connectionState struct
- directionStats struct
- Related WebSocket internal types

**Rationale:** All WebSocket-related types grouped together. This is the largest group (~250 lines).

---

### 7. `internal/composition/capture/extension-logging-types.go` (~100 LOC)
**Purpose:** Extension logging types

**Contents:**
- ExtensionLog struct
- PollingLogEntry struct
- HTTPDebugEntry struct
- ExtensionLogBuffer struct
- Related logging types

**Rationale:** Extension logging is a distinct concern from capture types.

---

### 8. `internal/composition/capture/enhanced-actions-types.go` (~80 LOC)
**Purpose:** Enhanced actions types

**Contents:**
- EnhancedAction struct
- EnhancedActionFilter struct
- Selectors map type
- Related action type definitions

**Rationale:** Enhanced actions have their own domain with specific selector strategies.

---

### 9. `internal/composition/capture/internal-types.go` (~150 LOC)
**Purpose:** Internal types used by Capture struct

**Contents:**
- A11yCache struct
- A11yCacheEntry struct
- A11yInflightEntry struct
- PerformanceStore struct
- SessionTracker struct (moved from session-types.go)
- Other internal helper types

**Rationale:** These are internal implementation details of Capture, not exposed externally.

---

### 10. `internal/composition/capture/constants.go` (~80 LOC)
**Purpose:** Buffer capacity and configuration constants

**Contents:**
- MaxWSEvents, MaxNetworkBodies, MaxExtensionLogs, MaxEnhancedActions
- RateLimitThreshold, RecordingStorageWarningLevel
- DefaultNetworkWaterfallCapacity, MinNetworkWaterfallCapacity, MaxNetworkWaterfallCapacity
- DefaultWSLimit, DefaultBodyLimit, MaxRequestBodySize, MaxResponseBodySize
- WsBufferMemoryLimit, NbBufferMemoryLimit
- CircuitBreaker configuration constants
- Rate limiting configuration constants

**Rationale:** Constants are configuration values. They should be in their own file for easy discovery and modification.

---

### 11. `internal/composition/capture/buffer-types.go` (~120 LOC)
**Purpose:** Ring buffer types for Capture composition

**Contents:**
- NetworkWaterfallBuffer struct
- ExtensionLogBuffer struct
- WSConnectionTracker struct
- Related buffer management types

**Rationale:** These are the ring buffer implementations used by Capture struct.

---

### 12. `internal/composition/capture/capture-struct.go` (~100 LOC)
**Purpose:** Main Capture struct and factory function

**Contents:**
- Capture struct (main composition)
- NewCapture() function
- SetLifecycleCallback() function
- emitLifecycleEvent() function
- SetServerVersion() function
- GetServerVersion() function
- Close() function

**Rationale:** The main Capture struct and its factory function. This is the core of the capture package.

---

## Refactoring Strategy

### Phase 1: Create New Files (No Breaking Changes)

1. Create all 12 new files alongside existing `types.go`
2. Copy relevant code sections to each new file
3. Keep `types.go` intact with all original code

### Phase 2: Update Imports

1. Update `types.go` to import from new files where needed
2. Run `go build` to verify no circular dependencies

### Phase 3: Test

1. Run `go test ./internal/composition/capture/...` to ensure no regressions
2. Run full test suite to verify overall functionality

### Phase 4: Remove Old Code

1. Remove moved code sections from `types.go`
2. Ensure `types.go` only contains:
   - Imports
   - Any remaining shared types that don't fit elsewhere
   - Main Capture struct and functions

### Phase 5: Final Verification

1. Run full test suite
2. Run `make compile-ts` if TypeScript files affected
3. Run `make test` for Go tests
4. Update documentation

---

## Import Structure After Refactoring

### In `internal/composition/capture/types.go`:
```go
import (
	"encoding/json"
	"sync"
	"time"

	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/performance"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/queries"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/recording"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/types"

	// Import from new type files
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/interfaces"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/type-aliases"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/session-types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/security-types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/network-types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/websocket-types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/extension-logging-types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/enhanced-actions-types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/internal-types"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/constants"
	"github.com/brennhill/kaboom-agentic-browser-devtools-mcp/internal/capture/buffer-types"
)
```

### In other files:
Files that import from `types.go` will need to update their imports to use the new, more specific files.

---

## Migration Checklist

For each new file:

- [ ] Create new file with appropriate name
- [ ] Copy relevant code to new file
- [ ] Add package declaration (`package capture`)
- [ ] Add necessary imports
- [ ] Verify no circular dependencies
- [ ] Run `go test ./internal/composition/capture/[new-file].go`
- [ ] Update imports in `types.go`
- [ ] Run `go build ./internal/composition/capture/`
- [ ] Run full test suite
- [ ] Remove moved code from `types.go`
- [ ] Update documentation if needed

---

## Estimated Effort

| File | Estimated Time | Complexity |
|-------|----------------|------------|
| `types.go` | 6-8 hours | High |
| Total | 6-8 hours | High |

**Rationale:** This is a complex refactoring because:
- Many interdependent types
- Multiple files import from types.go
- Need to ensure no circular dependencies
- Need to verify all tests still pass

---

## Risks and Mitigations

### Risk: Circular dependencies
**Mitigation:** Carefully plan import structure. Use type aliases where appropriate to avoid circular imports.

### Risk: Breaking imports
**Mitigation:** Keep original `types.go` intact during Phase 1. Only remove code after all imports are updated.

### Risk: Test failures
**Mitigation:** Run tests after each file split, not just at the end.

### Risk: Missing exports
**Mitigation:** Verify all public types and functions are still exported after refactoring.

---

## Success Criteria

1. All new files are under 500 LOC
2. All tests pass after refactoring
3. No circular dependencies
4. All public APIs are still accessible
5. Documentation is updated
6. Code review approved

---

## Benefits

1. **Better organization:** Types grouped by domain/concern
2. **Easier navigation:** Smaller files are easier to navigate and understand
3. **Reduced cognitive load:** Developers only need to load relevant files
4. **Better testability:** Smaller files are easier to test in isolation
5. **Clearer dependencies:** Import structure is more explicit
