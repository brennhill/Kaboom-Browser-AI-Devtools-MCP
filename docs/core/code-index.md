---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# code-index.md

**Quick lookup**: Which features/docs are affected when you modify specific code files?

## See Also

- [codebase-canon-v5.3.md](codebase-canon-v5.3.md) - v5.3 baseline (now superseded by v5.4)
- [known-issues.md](known-issues.md) - Current blockers and workarounds
- [feature-to-strategy.md](feature-to-strategy.md) - Feature → product strategy alignment

---

## Critical Core Files

### internal/capture/queries.go
**What it does:** Async queue-and-poll implementation

**Affects:**
- 🔥 **CRITICAL:** All `interact()` commands (execute_js, navigate, etc.)
- Feature: [Async Queue Pattern](architecture/ADR-001-async-queue-pattern.md)
- Feature: [Correlation ID Tracking](async-queue-correlation-tracking.md)
- Protection: [5-Layer Enforcement](architecture/ARCHITECTURE-ENFORCEMENT.md)
- Diagram: [Async Queue Flow](architecture/diagrams/async-queue-flow.md)

**Tests:**
- internal/capture/async_queue_integration_test.go
- internal/capture/async_queue_reliability_test.go
- internal/capture/correlation_tracking_test.go

**Protected by:** Pre-commit hook, CI validation, ADR-002

---

### internal/capture/handlers.go
**What it does:** HTTP endpoints for extension polling

**Affects:**
- Extension polling (`GET /pending-queries`)
- Result posting (`POST /dom-result`, `/execute-result`, etc.)
- Pilot status (`GET /pilot-status`)

**Tests:**

- cmd/browser-agent/tools_test.go (22 tests covering all tools)

**Protected by:** Pre-commit hook, stub detection

---

### cmd/browser-agent/tools_*.go

**What it does:** MCP tool implementations (split into 7 focused files)

**File Structure:**

- `tools_core.go` (680 lines) - ToolHandler struct, dispatch, response helpers
- `internal/toolobserve/dispatcher.go` (326 lines) - Observe tool and mode handlers
- `internal/toolgenerate/dispatcher.go` (130 lines) - Generate tool (HAR, SARIF, CSP, SRI, tests)
- `tools_configure.go` (412 lines) - Configure tool (store, noise, streaming, etc.)
- `tools_interact.go` (256 lines) - Interact tool (AI Web Pilot browser actions)
- `tools_security.go` (85 lines) - Security audit wrappers
- `tools_schema.go` (493 lines) - MCP tool schema definitions

**Affects:**
- All MCP tools available to AI agents
- Correlation ID status checking
- Async command queuing

**Docs:**
- [MCP Integration](../mcp-integration/index.md)
- [Interact Tool](../features/feature/interact-explore/)
- [Observe Tool](../features/feature/observe/)

**Tests:**
- cmd/browser-agent/tools_test.go (comprehensive coverage)

**Protected by:** Pre-commit hook, GitHub Actions, architecture validation

---

### src/content/favicon-replacer.ts
**What it does:** Flickering flame visual indicator

**Affects:**
- Feature: [Tab Tracking UX](features/feature/tab-tracking-ux/product-spec.md)
- Feature: [AI Web Pilot Visual](features/feature/ai-web-pilot/)
- Visual: [Flame Flicker Diagram](architecture/diagrams/flame-flicker-visual.md)

**Assets:**
- extension/icons/icon-flicker-*.svg (8 frames)
- extension/icons/icon-glow.svg (static)

**Tests:**
- tests/extension/favicon-replacer.test.js

---

### cmd/browser-agent/bridge.go
**What it does:** MCP stdio ↔ HTTP bridge

**Affects:**
- All MCP communication
- Error propagation to AI
- JSON-RPC protocol handling

**Diagram:** [System Architecture - Bridge Process](architecture/diagrams/system-architecture.md)

---

## Extension Files

### src/background/message-handlers.ts
**What it does:** Routes messages from popup/content scripts

**Affects:**
- AI Pilot toggle
- Tab tracking state broadcasts
- Favicon flicker triggers

**Related:**
- src/background/init.ts (wires up handlers)
- src/popup/ai-web-pilot.ts (sends toggle messages)

---

### src/content.ts
**What it does:** Content script entry point

**Affects:**
- All page-level capture (logs, network, WebSocket)
- Favicon replacement
- DOM query execution

**Initializes:**
- Tab tracking
- Favicon replacer
- Script injection
- Message listeners

---

## Configuration Files

### extension/manifest.json
**What it does:** Extension configuration

**Change impact:**
- Version bumps → Need to rebuild extension
- `web_accessible_resources` → Affects what content scripts can load
- Permissions → Affects what extension can do

**Build:** `make compile-ts` after changes

---

### Makefile
**What it does:** Build configuration

**Change impact:**
- `VERSION` → All version bumps
- Build targets → Platform support
- Test targets → CI/CD

---

## Documentation Structure

### docs/features/
**Contents:** Feature specifications (71 features)

**When to check:**
- Adding new MCP tool
- Changing extension behavior
- Adding new capture type

**Navigation:** [docs/features/feature-navigation.md](features/feature-navigation.md)

---

### docs/architecture/
**Contents:** ADRs, architecture diagrams, enforcement guides

**When to check:**
- Modifying async queue
- Changing core patterns
- Major refactoring

**Key files:**
- ADR-001: Async Queue Pattern
- ADR-002: Async Queue Immutability
- ARCHITECTURE-ENFORCEMENT.md

---

### docs/core/
**Contents:** Release process, known issues, codebase canon

**When to check:**
- Preparing release
- Fixing known bugs
- Understanding v5.3 baseline

---

## Feature-to-Code Mapping

### Async Queue Pattern
**Code:**
- internal/capture/queries.go (303 lines)
- internal/capture/handlers.go (polling endpoints)
- cmd/browser-agent/tools_core.go (MCP handlers)
- internal/queries/types.go (type definitions)

**Docs:**
- architecture/ADR-001-async-queue-pattern.md
- architecture/diagrams/async-queue-flow.md
- async-queue-correlation-tracking.md

**Tests:**
- internal/capture/async_queue_integration_test.go
- internal/capture/async_queue_reliability_test.go

---

### Correlation ID Tracking
**Code:**
- internal/capture/queries.go (RegisterCommand, CompleteCommand, ExpireCommand)
- cmd/browser-agent/tools_core.go (toolObserveCommandResult, toolObservePendingCommands)

**Docs:**
- async-queue-correlation-tracking.md
- architecture/diagrams/correlation-id-lifecycle.md

**Tests:**
- internal/capture/correlation_tracking_test.go

---

### Flickering Flame Visual
**Code:**
- src/content/favicon-replacer.ts
- src/background/message-handlers.ts (broadcastTrackingState)
- src/background/init.ts (wires up broadcasts)
- extension/icons/icon-flicker-*.svg (8 frames)

**Docs:**
- features/feature/tab-tracking-ux/product-spec.md
- architecture/diagrams/flame-flicker-visual.md

**Tests:**
- tests/extension/favicon-replacer.test.js

---

### 5-Layer Protection
**Code:**
- .git/hooks/pre-commit (119 lines)
- scripts/validate-architecture.sh (225 lines)
- .github/workflows/architecture-validation.yml (103 lines)
- internal/capture/async_queue_integration_test.go

**Docs:**
- architecture/ADR-002-async-queue-immutability.md
- architecture/ARCHITECTURE-ENFORCEMENT.md
- architecture/diagrams/5-layer-protection.md

**Tests:**
- internal/capture/async_queue_integration_test.go (exercises full flow)

---

## Package Documentation

Each internal package now has `doc.go` with comprehensive package overview:

- `internal/analysis/apicontract/contract.go` - API contract baselines and drift detection
- `internal/analysis/apischema/schema.go` - API schema inference from observed traffic
- `internal/analysis/clustering/clustering.go` - Error clustering by root cause
- `internal/analysis/thirdparty/audit.go` - Third-party origin auditing
- `internal/capture/capture.go` - Real-time browser telemetry capture
- `internal/pagination/doc.go` - Cursor-based pagination
- `internal/security/scan/doc.go` - Aggregate security audit (credentials, PII, headers, cookies, transport)
- `internal/security/csp/doc.go` - CSP generation from observed traffic
- `internal/security/sri/doc.go` - Subresource Integrity hash generation
- `internal/session/doc.go` - Named browser-state snapshots and comparisons
- `internal/types/doc.go` - Core type definitions

**Type-Safe Dependencies:**
- `internal/types/interfaces.go` - Interfaces for ToolHandler dependencies

---

## Common Change Scenarios

### "I want to add a new MCP tool"
**Check:**
1. docs/features/ - See existing tool patterns
2. cmd/browser-agent/tools_core.go - Add handler
3. cmd/browser-agent/handler.go - Register in tool list
4. docs/features/feature/{tool-name}/ - Create spec

---

### "I want to change the async queue"
**STOP!** Read first:
1. architecture/ADR-002-async-queue-immutability.md (WHY immutable)
2. architecture/ARCHITECTURE-ENFORCEMENT.md (Bypass procedure)
3. architecture/diagrams/async-queue-flow.md (Understand current design)

**If approved:**
1. Update all 5 enforcement layers
2. Add compensating tests
3. Update ADR-002 with rationale

---

### "I want to add extension UI"
**Check:**
1. src/popup/ - Popup UI components
2. src/background/message-handlers.ts - Add message handler
3. extension/manifest.json - Permissions needed?
4. docs/features/feature/browser-extension-enhancement/

---

### "I want to change timeout values"
**Affects:**
- internal/queries/types.go (AsyncCommandTimeout constant)
- Async queue reliability (see tests)
- Production stability (see async-queue-correlation-tracking.md)

**Must update:**
- Architecture validation (checks timeout = 30s)
- Documentation explaining rationale

---

## Automated Tools

### Find what uses a file
```bash
# Search all docs
grep -r "queries.go" docs/

# Search all code imports
grep -r "internal/capture" . --include="*.go"

# Search test references
grep -r "TestAsyncQueue" . --include="*_test.go"
```

### Find feature by keyword
```bash
# Search feature specs
grep -r "async queue" docs/features/

# Search architecture docs
grep -r "correlation" docs/architecture/
```

---

## Quick Reference

| I'm changing... | Check these docs... | Run these tests... |
|-----------------|---------------------|--------------------|
| queries.go | ADR-001, ADR-002, async-queue-flow.md | TestAsyncQueue*, validate-architecture.sh |
| handlers.go | async-queue-flow.md | ./scripts/validate-architecture.sh |
| tools_core.go | MCP integration docs, feature specs | ./scripts/validate-architecture.sh |
| favicon-replacer.ts | flame-flicker-visual.md, tab-tracking-ux | npm run test:ext |
| manifest.json | Extension build docs | make compile-ts |
| Any internal/capture/* | Architecture diagrams | go test ./internal/capture |

---

## Future Improvements

**Suggested additions to this index:**
- [ ] Automated dependency graph generator
- [ ] Feature → code mapping tool
- [ ] Impact analysis script (given a file, show all affected features)
- [ ] Reverse index (feature → all code files)
- [ ] Test coverage by feature

**To contribute:**
Update this file when you add new features or change architecture.
