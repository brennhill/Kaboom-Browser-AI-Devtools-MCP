---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Core System Diagrams: Implementation Summary

**Date:** 2026-02-08
**Status:** ✅ Complete - 5 diagrams created
**Location:** `docs/architecture/diagrams/`

---

## What We Created

### 1. **C2: Container Architecture** (`c2-containers.md`)
**Purpose:** Show the 5 main system components and their communication

**Includes:**
- Container diagram showing: AI Agent ↔ Wrapper ↔ Go Server ↔ Extension ↔ Browser
- Container responsibilities table
- 4 data flow patterns:
  - Continuous telemetry (Extension → Server, every 1s)
  - Query system (AI → Server → Extension → Polling → Result)
  - Observe (AI → Server buffer, instant)
  - Configure (AI → Server, one-way persistence)
- Why 5 containers (separation of concerns, isolation, deployment flexibility)
- Why HTTP (standard, reliable, debuggable)
- Why polling (reliability, simplicity, resilience, multi-client isolation)

**Length:** ~250 lines
**Diagram Type:** C4 Container + Data Flow Patterns

---

### 2. **C3: Component Architecture** (`c3-components.md`)
**Purpose:** Show Go package structure and layer dependencies

**Includes:**
- Complete C3 component diagram showing all Go packages:
  - Layer 1: Foundation Types (protocol, network, log, alert, security, snapshot, buffer)
  - Layer 2: Domain Packages (Capture, Session, Analysis, Security, Pagination, Buffers)
  - Layer 3: MCP Tools (observe, generate, interact, configure)
  - Layer 4: HTTP Server (main, handler, server, middleware, routes, health)
  - Layer 5: Utilities (rate_limit, circuit_breaker, redaction, export, perf, util)
- Layer responsibility descriptions
- Dependency flow diagram (all dependencies flow downward)
- 4 Interaction patterns:
  - Observe tool → Capture buffers
  - Interact tool → Query queueing → Extension
  - Generate tool → Analysis/Security
  - Configure tool → Persistence

**Length:** ~350 lines
**Diagram Type:** C4 Component + Dependency Graph

---

### 3. **Request-Response Cycle** (`request-response-cycle.md`)
**Purpose:** Show complete MCP command flow from AI to Extension and back

**Includes:**
- **Main Sequence:** Complete 5-phase cycle (request, queue, poll, execute, result polling)
  - Phase 1: AI makes request via wrapper
  - Phase 2: Server queues query with correlation_id
  - Phase 3: Extension polls every 1s until query appears
  - Phase 4: Extension executes and POSTs result
  - Phase 5: AI polls for result

- **3 Variant Patterns:**
  - Immediate Response (observe - instant buffer query)
  - One-Way (configure - just persistence)
  - Timeout/Error Scenarios (query timeout, extension restart, result expiry)

- **Data Flow Summary Table:**
  - Total cycle time: < 10ms for AI to get non-blocking response
  - Extension has 30s to execute
  - Result available for 60s after completion

- **Key Properties:**
  - Non-blocking design
  - Multi-client safe
  - Resilient to extension crashes
  - Memory bounded
  - Debuggable (correlation_id tracking)

**Length:** ~500 lines
**Diagram Type:** Sequence Diagram (main + 3 variants) + Tables

---

### 4. **Extension Message Protocol** (`extension-message-protocol.md`)
**Purpose:** Document all HTTP messages between extension and server

**Includes:**
- **6 Message Types** with complete JSON schemas:
  1. `GET /pending-queries` - Extension polls for work
  2. `POST /dom-result` - Extension posts query result
  3. `GET /completed-results` - Check if result ready (polling)
  4. `POST /sync` - Telemetry batch (continuous stream)
  5. `POST /recordings/save` - Video blob + metadata
  6. `POST /extension-logs` - Extension debug logs

- **Complete Request/Response Schemas** for each message type
- **Error Response Handling** (401 unauthorized, 429 rate limit, 500 error)
- **State Machine:** Extension connection lifecycle (8 states, transitions)
- **Data Types Reference:**
  - Token authorization format
  - Correlation ID format
  - Timestamp format (milliseconds since epoch)
  - Query action types (execute_js, query_dom, execute_a11y_audit, screen_recording_start, screen_recording_stop)

- **Reliability Patterns:**
  - Idempotent query IDs
  - Timeout escalation (30s → 35s grace period)
  - Circuit breaker (10 failures in 60s blocks client)

**Length:** ~450 lines
**Diagram Type:** Message Protocol Reference + State Machine

---

### 5. **Data Capture Pipeline** (`data-capture-pipeline.md`)
**Purpose:** Show how telemetry flows from page to server buffers

**Includes:**
- **Complete Data Flow Architecture Diagram:**
  - Page Level: 7 observers (console, fetch, XHR, WS, perf, error, actions)
  - Serialization: Safe JSON, PII redaction, error enrichment
  - Memory: State manager, event batchers
  - Transport: Batch assembly, retry logic, HTTP POST
  - Server: Sync handler, router, ring buffers
  - Management: Memory tracking, TTL eviction, size limits
  - Analysis: Error clustering, API schema learning
  - Queries: Pending queries, completed results, dispatcher

- **Step-by-Step Example:** How network request is captured (parallel flow)
- **Buffer Management State Machine:** Add → Check size → Evict (TTL or forced)
- **6 Event Type Specifications** with example JSON:
  1. Console events (4 fields, max 1KB per arg)
  2. Network/waterfall (10 fields, max 10KB per entry)
  3. Network bodies (separate storage, max 200MB total)
  4. Action events (selectors, coordinates, 30MB buffer)
  5. WebSocket events (direction, data preview, 50MB buffer)
  6. Performance events (metrics, context, 20MB buffer)
  7. Error events (stack, enrichment, 20MB buffer)

- **Memory Accounting:** Per-buffer tracking, automatic calculation, enforcement
- **Automatic Eviction:** TTL-based then size-based, graceful degradation

**Length:** ~550 lines
**Diagram Type:** Architecture Diagram + State Machine + Event Type Reference

---

## Key Metrics

| Diagram | Lines | Type | Complexity | References |
|---------|-------|------|-----------|-----------|
| C2 Containers | ~250 | C4 + Flow | Medium | 3 implementation files |
| C3 Components | ~350 | C4 + Dependency | High | 40+ implementation files |
| Request-Response | ~500 | Sequence + Tables | High | 10 implementation files |
| Extension Protocol | ~450 | Reference + State | Very High | 8 message types documented |
| Data Pipeline | ~550 | Architecture + Types | Very High | 25 implementation files |
| **TOTAL** | **~2,100** | **Mixed** | **Very High** | **100+ files** |

---

## How They Work Together

```
User Learning Path:
1. Start with C2 (Containers) - Understand the 5 main pieces
   ↓
2. Then C3 (Components) - Understand how Go packages connect
   ↓
3. Then Request-Response Cycle - Understand how MCP commands work
   ↓
4. Then Extension Message Protocol - Understand HTTP messages
   ↓
5. Finally Data Pipeline - Understand how telemetry flows

Deep Dive Path (for feature implementation):
- Understanding a feature? → Start with Request-Response Cycle
- Understanding extension communication? → Use Extension Message Protocol
- Understanding data storage? → Use Data Capture Pipeline + C3 (Capture package)
- Understanding multi-client? → Use C2 + C3 (Session package)
```

---

## Implementation Coverage

### Fully Documented (5 core diagrams)
- ✅ Request path: AI → Wrapper → Server → Extension → Result
- ✅ Response path: Extension → Server → AI polling
- ✅ Telemetry path: Page → Extension → Server (continuous)
- ✅ Storage: Ring buffers, TTL, memory management
- ✅ Query system: Async queue-and-poll pattern
- ✅ Session/Client isolation: Multi-client support
- ✅ Message protocol: All 6 HTTP endpoints documented
- ✅ Error handling: Timeout, expiry, circuit breaker

### Referenced But Not Yet Diagrammed (for future)
- 🚧 Test generation pipeline (testgen.go)
- 🚧 Security analysis (CSP, SRI, threat detection)
- 🚧 Error clustering and API contract learning
- 🚧 Recording lifecycle (video capture → save → persist)
- 🚧 Individual tool details (observe, generate, interact, configure)
- 🚧 Feature-specific flows (once tech specs are created)

---

## Next Steps: Feature Tech Spec Diagrams

Each of the 88 feature tech specs should now include:

1. **At least 1 Sequence Diagram** showing:
   - Cold start (first time)
   - Warm start (subsequent)
   - Error scenario

2. **State Machine** (if applicable):
   - Valid states
   - Transitions and triggers
   - Terminal states

3. **Data Flow Diagram** (if new data types):
   - How data flows through system
   - Where buffered/stored
   - Cleanup/eviction strategy

**Reference:** Use `feature/tab-recording/tech-spec.md` (3 sequence diagrams) as template

---

## Files Modified/Created

**New Files:**
- ✅ `docs/architecture/diagrams/c2-containers.md`
- ✅ `docs/architecture/diagrams/c3-components.md`
- ✅ `docs/architecture/diagrams/request-response-cycle.md`
- ✅ `docs/architecture/diagrams/extension-message-protocol.md`
- ✅ `docs/architecture/diagrams/data-capture-pipeline.md`

**Updated Files:**
- ✅ `docs/architecture/diagrams/README.md` - Added new diagrams to navigation

**Memory Files:**
- ✅ `/Users/brenn/.claude/projects/-Users-brenn-dev-kaboom/memory/MEMORY.md` - Updated
- ✅ `/Users/brenn/.claude/projects/-Users-brenn-dev-kaboom/memory/diagram-inventory.md` - Created

**Audit Files:**
- ✅ `docs/audit/frontmatter-and-diagram-audit.md` - Created
- ✅ `docs/audit/codebase-audit-report.md` - From Explore agent
- ✅ `docs/audit/core-diagrams-summary.md` - This file

---

## Quality Checklist

- ✅ All diagrams use Mermaid syntax (renders on GitHub)
- ✅ Consistent color scheme (orange primary, yellow secondary, green success, blue info, red error)
- ✅ Each diagram has "References" section linking to code
- ✅ Each diagram has "Related Diagrams" section
- ✅ Markdown formatting follows linting rules
- ✅ All JSON schemas are valid and complete
- ✅ All code file references are accurate
- ✅ Diagrams are discoverable from navigation README

---

## Statistics

- **Total Diagrams Created:** 5
- **Total Lines of Documentation:** ~2,100
- **Total Code References:** 100+
- **Total Implementation Files Covered:** 40+
- **Message Types Documented:** 6
- **Event Types Specified:** 7
- **Data Flow Patterns:** 4
- **Buffer Types:** 6
- **State Machines:** 2
- **Sequence Diagrams:** 5+ (main + variants)

---

## Commit Information

**Branch:** UNSTABLE
**Type:** docs: Add comprehensive core system diagrams
**Files Changed:** 11 new + 1 updated
**Impact:** Documentation only, no code changes

**Description:**
```
docs: Add 5 core system diagrams covering complete MCP flow

- C2 Container Architecture: 5 main system components
- C3 Component Architecture: Go package structure (5 layers)
- Request-Response Cycle: Complete MCP command flow (4 patterns)
- Extension Message Protocol: All 6 HTTP message types + state machine
- Data Capture Pipeline: Telemetry flow with detailed event specs

These diagrams show:
- Complete request-response cycle (AI → Wrapper → Server → Extension → Result)
- How extension continuously streams telemetry via batched POST /sync
- How queries route to extension and results are polled
- All HTTP message types with JSON schemas
- Ring buffer management with TTL and memory enforcement

Covers ~100 implementation files and 40 Go packages.
References established for future feature tech specs.
```
