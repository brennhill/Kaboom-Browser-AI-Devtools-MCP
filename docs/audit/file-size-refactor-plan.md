---
status: proposed
scope: audit/code-quality
ai-priority: high
tags: [refactor, code-quality, file-size]
relates-to: [codebase-audit-report.md]
last-verified: 2026-02-08
---

# File Size Violation Refactoring Plan

**Issue:** Issue #5.1 - File Size Violations (5 files exceed 800 LOC limit)

**Goal:** Split large files into smaller, focused modules following single responsibility principle.

---

## Files to Refactor

| File | Current LOC | Target LOC | Status |
|-------|-------------|-------------|--------|
| `src/background/pending-queries.ts` | 952 | < 500 | ❌ Exceeds |
| `internal/capture/types.go` | 653 | < 500 | ❌ Exceeds |
| `internal/session/sessions.go` | 694 | < 500 | ❌ Exceeds |
| `src/background/message-handlers.ts` | 552 | < 500 | ❌ Exceeds |
| `src/lib/net/websocket.ts` | 776 | < 500 | ⚠️ Near limit |

---

## 1. `src/background/pending-queries.ts` (952 LOC → ~400 LOC)

**Current concerns:**
- Constants and timeouts
- Result helper functions (sendResult, sendAsyncResult, actionToast)
- Script execution utilities (executeViaScriptingAPI, serialize)
- Main query handler (handlePendingQuery)
- State query handler (handleStateQuery)
- Browser action handler (handleBrowserAction)
- Async execute handler (handleAsyncExecuteCommand)
- Pilot command handler (handlePilotCommand)

**Proposed split:**

### New Files:
1. `src/background/query-result-utils.ts` (~80 LOC)
   - `sendResult()` - Send query result via /sync
   - `sendAsyncResult()` - Send async command result via /sync
   - `actionToast()` - Show visual action toast
   - `PRETTY_LABELS` - Action name to label mapping

2. `src/background/script-execution.ts` (~120 LOC)
   - `ASYNC_EXECUTE_TIMEOUT_MS` - Timeout constant
   - `ASYNC_BROWSER_ACTION_TIMEOUT_MS` - Timeout constant
   - `executeViaScriptingAPI()` - Execute JS via chrome.scripting API
   - `serialize()` - Safe serialization for values

3. `src/background/state-query-handler.ts` (~100 LOC)
   - `handleStateQuery()` - Handle state management queries
   - Actions: capture, save, load

4. `src/background/browser-action-handler.ts` (~150 LOC)
   - `handleBrowserAction()` - Handle browser action queries
   - Actions: navigate, refresh, execute_js, click, type, select, check, focus, scroll_to, wait_for, key_press

5. `src/background/async-execute-handler.ts` (~80 LOC)
   - `handleAsyncExecuteCommand()` - Handle async execute commands
   - Timeout handling
   - Result serialization

6. `src/background/pilot-command-handler.ts` (~60 LOC)
   - `handlePilotCommand()` - Handle AI pilot commands

7. `src/background/pending-queries.ts` (~300 LOC)
   - Constants (if any remain)
   - Imports
   - `handlePendingQuery()` - Main entry point and dispatcher

**Benefits:**
- Each file has single, clear responsibility
- Easier to test individual components
- Better code organization and discoverability
- Reduces cognitive load when navigating code

---

## 2. `internal/capture/types.go` (653 LOC → ~400 LOC)

**Refactoring Plan:** See [`types-go-refactor-plan.md`](types-go-refactor-plan.md) for detailed split strategy including:
- 12 new files organized by domain/concern
- Phase-by-phase refactoring approach
- Testing strategy
- Migration checklist
- Estimated effort (6.8 hours)

## 3. `internal/session/sessions.go` (694 LOC → ~400 LOC)

**Refactoring Plan:** See [`sessions-go-refactor-plan.md`](sessions-go-refactor-plan.md) for detailed split strategy including:
- 11 new files organized by domain/concern
- Phase-by-phase refactoring approach
- Testing strategy
- Migration checklist
- Estimated effort (4.6 hours)

**Current concerns:**
- Abstracted component interfaces (SchemaStore, CSPGenerator, ClientRegistry)
- Type aliases for imported packages
- Session tracking types (SessionTracker)
- Security threat flagging (SecurityFlag)
- Network waterfall types (NetworkWaterfallEntry)
- Many other capture-specific types

**Proposed split:**

### New Files:
1. `internal/capture/interfaces.go` (~80 LOC)
   - `SchemaStore` interface
   - `CSPGenerator` interface
   - `ClientRegistry` interface

2. `internal/capture/aliases.go` (~30 LOC)
   - All type aliases for imported packages
   - PerformanceSnapshot, PerformanceBaseline, etc.
   - Recording, RecordingAction, etc.
   - PendingQueryResponse, PendingQuery, CommandResult

3. `internal/capture/session-types.go` (~50 LOC)
   - `SessionTracker` struct

4. `internal/capture/security-types.go` (~50 LOC)
   - `SecurityFlag` struct

5. `internal/capture/network-types.go` (~200 LOC)
   - `NetworkWaterfallEntry` struct
   - `NetworkWaterfallPayload` struct
   - Related network types

6. `internal/capture/types.go` (~350 LOC)
   - Capture struct
   - Extension state types
   - WebSocket event types
   - User action types
   - Enhanced action types
   - Network body types
   - Other remaining capture-specific types

**Benefits:**
- Logical grouping by domain (interfaces, aliases, session, security, network)
- Smaller, more focused files
- Easier to locate specific type definitions
- Better for import organization

---

## 3. `internal/session/sessions.go` (694 LOC → ~400 LOC)

**Current concerns:**
- Session comparison logic
- Diff computation (network, actions, performance, DOM)
- Fragile selector detection
- Playback session management

**Proposed split:**

### New Files:
1. `internal/session/comparison.go` (~100 LOC)
   - `Compare()` - Main comparison entry point
   - Result aggregation

2. `internal/session/network-diff.go` (~100 LOC)
   - `diffNetwork()` - Network diff computation
   - Entry comparison logic

3. `internal/session/actions-diff.go` (~80 LOC)
   - `diffActions()` - Actions diff computation
   - Action comparison logic

4. `internal/session/performance-diff.go` (~100 LOC)
   - `diffPerformance()` - Performance diff computation
   - Baseline comparison logic

5. `internal/session/dom-diff.go` (~80 LOC)
   - `diffDOM()` - DOM diff computation
   - Element comparison logic

6. `internal/session/fragile-selectors.go` (~80 LOC)
   - `DetectFragileSelectors()` - Find fragile selectors
   - Selector analysis logic

7. `internal/session/playback.go` (~100 LOC)
   - Playback session management
   - Execution tracking

8. `internal/session/sessions.go` (~200 LOC)
   - SessionManager struct
   - Snapshot capture logic
   - Main exports

**Benefits:**
- Each diff type has its own file
- Easier to test individual diff algorithms
- Clearer separation of concerns
- Better for adding new diff types

---

## 4. `src/background/message-handlers.ts` (552 LOC → ~400 LOC)

**Current concerns:**
- Message listener installation
- Message validation
- Log message handling
- Clear logs handling
- Set log level handling
- Set screenshot on error handling
- Set AI Web Pilot enabled handling
- Get tracking state handling
- Capture screenshot handling
- Forwarded setting handling
- Set server URL handling
- State snapshot management (save, load, list, delete)
- Error group flush handling

**Proposed split:**

### New Files:
1. `src/background/message-listener.ts` (~80 LOC)
   - `installMessageListener()` - Install runtime message listener
   - Message validation

2. `src/background/log-message-handler.ts` (~60 LOC)
   - `handleLogMessage()` - Handle log messages
   - Log level validation

3. `src/background/setting-handlers.ts` (~150 LOC)
   - `handleSetLogLevel()` - Set log level
   - `handleSetScreenshotOnError()` - Set screenshot on error
   - `handleSetAiWebPilotEnabled()` - Set AI pilot enabled
   - `handleSetServerUrl()` - Set server URL
   - `handleForwardedSetting()` - Handle forwarded settings

4. `src/background/snapshot-handlers.ts` (~120 LOC)
   - `saveStateSnapshot()` - Save state snapshot
   - `loadStateSnapshot()` - Load state snapshot
   - `listStateSnapshots()` - List state snapshots
   - `deleteStateSnapshot()` - Delete state snapshot
   - `broadcastTrackingState()` - Broadcast tracking state

5. `src/background/screenshot-handler.ts` (~60 LOC)
   - `handleCaptureScreenshot()` - Capture screenshot
   - Rate limiting

6. `src/background/error-group-handler.ts` (~40 LOC)
   - `handleErrorGroupFlush()` - Handle error group flush

7. `src/background/message-handlers.ts` (~200 LOC)
   - Main exports
   - Imports
   - Helper functions

**Benefits:**
- Each handler type has its own file
- Easier to test individual handlers
- Better organization by functionality
- Clearer separation of concerns

---

## 5. `src/lib/net/websocket.ts` (776 LOC → ~500 LOC)

**Current concerns:**
- Connection tracking (createConnectionTracker)
- WebSocket constructor wrapping
- Message interception
- Connection lifecycle (open, close, error)
- Adaptive sampling
- Schema detection
- Binary message handling
- Message truncation
- Connection stats
- Early connection adoption

**Proposed split:**

### New Files:
1. `src/lib/websocket-tracker.ts` (~250 LOC)
   - `createConnectionTracker()` - Create connection tracker
   - Connection lifecycle management
   - Message recording
   - Stats tracking

2. `src/lib/websocket-interception.ts` (~150 LOC)
   - `installWebSocketCapture()` - Install WebSocket capture
   - WebSocket constructor wrapping
   - Message interception (send, message events)
   - Early connection adoption

3. `src/lib/websocket-sampling.ts` (~100 LOC)
   - Adaptive sampling logic
   - Rate calculation
   - Sampling decisions

4. `src/lib/websocket-schema.ts` (~80 LOC)
   - Schema detection
   - Schema consistency tracking
   - Schema change detection

5. `src/lib/websocket-binary.ts` (~80 LOC)
   - Binary message formatting
   - Size calculation
   - Magic byte extraction

6. `src/lib/websocket-utils.ts` (~60 LOC)
   - `truncateWsMessage()` - Message truncation
   - `formatPayload()` - Payload formatting
   - `getSize()` - Size calculation

7. `src/lib/net/websocket.ts` (~200 LOC)
   - Main exports
   - Constants
   - Connection state management

**Benefits:**
- Clear separation of WebSocket concerns
- Easier to test individual components
- Better for understanding WebSocket capture flow
- More modular and maintainable

---

## Refactoring Strategy

### Phase 1: Create New Files (No Breaking Changes)
1. Create new files alongside existing ones
2. Copy relevant code to new files
3. Add exports from new files
4. Keep existing files intact

### Phase 2: Update Imports
1. Update imports in files that use the split modules
2. Run `make compile-ts` to verify
3. Run tests to ensure no regressions

### Phase 3: Remove Old Code
1. Remove code from original files that was moved
2. Update original file to only contain remaining code
3. Run final tests

### Phase 4: Cleanup
1. Remove any unused imports
2. Run `npm run lint`
3. Run `npm run typecheck`
4. Update documentation

---

## Testing Strategy

1. **Before refactoring:** Run full test suite to establish baseline
2. **During refactoring:** Run tests after each file split
3. **After refactoring:** Run full test suite to ensure no regressions

```bash
# Test commands
make test
npm run lint
npm run typecheck
make compile-ts
```

---

## Migration Checklist

For each file split:

- [ ] Create new file with appropriate name
- [ ] Copy relevant code to new file
- [ ] Add exports from new file
- [ ] Update imports in consuming files
- [ ] Run `make compile-ts`
- [ ] Run tests for affected modules
- [ ] Remove moved code from original file
- [ ] Run full test suite
- [ ] Update documentation if needed
- [ ] Update audit report with completion status

---

## Estimated Effort

| File | Estimated Time | Complexity |
|-------|----------------|------------|
| `pending-queries.ts` | 4-6 hours | Medium |
| `types.go` | 3-4 hours | Medium |
| `sessions.go` | 4-6 hours | Medium-High |
| `message-handlers.ts` | 3-4 hours | Medium |
| `websocket.ts` | 3-5 hours | Medium |

**Total:** 17-25 hours

---

## Risks and Mitigations

### Risk: Breaking imports
**Mitigation:** Keep original files intact during Phase 1, only remove code after all imports are updated

### Risk: Test failures
**Mitigation:** Run tests after each file split, not just at the end

### Risk: Circular dependencies
**Mitigation:** Carefully plan import structure, avoid circular dependencies

### Risk: Missing exports
**Mitigation:** Verify all public APIs are still exported after refactoring

---

## Success Criteria

1. All split files are under 500 LOC
2. All tests pass after refactoring
3. No linting errors
4. No TypeScript errors
5. Documentation is updated
6. Code review approved
