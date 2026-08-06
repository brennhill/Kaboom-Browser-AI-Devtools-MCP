---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Kaboom MCP Diagram Inventory

**Last Updated:** 2026-02-08
**Total Tech Specs:** 88
**Tech Specs WITH Diagrams:** 4 (4.5%)
**Tech Specs WITHOUT Diagrams:** 84 (95.5%)

---

## EXISTING DIAGRAMS

### Architecture Diagrams (`docs/architecture/diagrams/`)

| Diagram | Type | Purpose | Location |
|---------|------|---------|----------|
| **System Architecture** | C4 Context + Container | Overall system design, components, data flows | system-architecture.md |
| **Async Queue-and-Poll Flow** | Sequence + State Machine | Core async pattern for browser control | async-queue-flow.md |
| **Correlation ID Lifecycle** | State Machine | Command status tracking (pending/complete/expired) | correlation-id-lifecycle.md |
| **Flame Flicker Visual** | State Machine + Message Flow | Favicon animation states and message routing | flame-flicker-visual.md |
| **5-Layer Protection** | Flowchart | Defense-in-depth enforcement (hooks, tests, validation, CI) | 5-layer-protection.md |

### Feature Tech Specs WITH Diagrams

| Feature | Diagram Type | Count | Location |
|---------|--------------|-------|----------|
| **Tab Recording** | Sequence (3 diagrams) | 3 | feature/tab-recording/tech-spec.md |
| **Reproduction Scripts** | Sequence + Flowchart | 2+ | feature/reproduction-scripts/tech-spec.md |
| **Error Bundling** | Sequence + State | 2+ | feature/error-bundling/tech-spec.md |
| **Performance Experimentation** | Sequence | 1+ | feature/perf-experimentation/tech-spec.md |

### MCP Server Architecture

| Diagram | Type | Diagrams | Location |
|---------|------|----------|----------|
| **MCP Persistent Server** | Sequence (3 scenarios) | 3 | features/mcp-persistent-server/architecture.md |
| - Cold Start | Sequence | - | - |
| - Warm Start | Sequence | - | - |
| - Concurrent Clients Race | Sequence | - | - |

**Total Existing Diagrams:** ~22 across 5 architecture docs + 4 tech specs

---

## CRITICAL GAPS

### Missing Core Diagrams (HIGH PRIORITY)

| Missing Diagram | Should Show | Located In | Why Critical |
|-----------------|------------|-----------|--------------|
| **Extension Message Protocol** | Message routing flow, message types | docs/core/protocol/extension-message-protocol.md | Core docs exist but NO visual diagram |
| **Capture Data Pipeline** | WS → Network → Actions → Logs | internal/capture/ | How telemetry flows from tab to server |
| **Query Dispatcher** | Query routing logic | internal/queries/dispatcher_queries.go | How commands route to extension |
| **Buffer Architecture** | Ring buffer, TTL, eviction | internal/buffers/, internal/capture/ | Memory management strategy |
| **Session/Client Registry** | Multi-client isolation | internal/session/clientreg/ | How concurrent clients are managed |
| **Security Analysis Pipeline** | CSP → SRI → Threat detection | internal/security/ | Security workflow |
| **Error Clustering** | Error grouping by root cause | internal/analysis/clustering/clustering.go | How errors are deduplicated |
| **API Contract Analysis** | Schema inference from network | internal/analysis/apicontract/contract.go | How API contracts are learned |
| **Recording Lifecycle** | Start → Capture → Save → Persist | cmd/browser-agent/tools_recording_video.go + internal/recording/ | Full recording flow |

---

## RECOMMENDED PRIORITY

### Phase 1: CRITICAL
1. **Extension Message Protocol** - How extension ↔ server communicate (sequence)
2. **Data Pipeline** - How telemetry flows (sequence with buffers highlighted)
3. **Query System** - How DOM queries reach extension (sequence)
4. **Recording Flow** - Complete end-to-end (sequence + state machine)

### Phase 2: HIGH
1. **C4 L2/L3** - System decomposition
2. **Security Analysis** - CSP → Threat detection (flowchart)
3. **Error Clustering** - Grouping logic (flowchart + example)
4. **Session/Registry** - Multi-client isolation (sequence + state)

### Phase 3: MEDIUM
1. **Test Generation** - Full pipeline (flowchart)
2. **Buffer Management** - TTL, eviction, memory (state machine)
3. **Circuit Breaker** - Resilience pattern (state machine)
4. **Selector Healing** - DOM selector repair (flowchart)

---

## DOCUMENTATION STANDARDS

### Every Feature Tech Spec MUST Include:
1. **Sequence Diagram** showing cold start, warm start, edge case/error path, concurrent scenarios
2. **State Machine** (if applicable): valid states, transitions, triggers, terminal/error states
3. **Flowchart** (if complex decision logic): decision points, alternative paths, error recovery

**Reference:** See `docs/features/mcp-persistent-server/architecture.md` and `docs/features/feature/tab-recording/tech-spec.md`
