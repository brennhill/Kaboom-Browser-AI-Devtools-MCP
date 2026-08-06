---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-03-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Architecture Diagrams

Visual guides to understanding Kaboom's core architecture.

## Quick Navigation

### 🏗️ C4 System Architecture

**[C1: System Context](system-architecture.md)**
- High-level system boundary
- External actors (AI Agent, Developer, Browser)
- Kaboom system as black box
- Data flows at system level

**[C2: Containers](c2-containers.md)** ⭐ **NEW - START HERE**
- 5 main containers: AI Agent, Wrapper, Go Server, Extension, Browser
- Container responsibilities and interfaces
- HTTP/stdio communication between containers
- Data flow patterns (telemetry, queries, results)

**[C3: Components](c3-components.md)** ⭐ **NEW**
- Go package architecture (5 layers)
- Foundation types, domain packages, tools, HTTP server, utilities
- Dependency flow (downward only)
- Interaction patterns between layers

### 📊 Core System Flows

**[Request-Response Cycle](request-response-cycle.md)** ⭐ **NEW - ESSENTIAL**
- Complete MCP command flow from AI → Wrapper → Server → Extension → Result
- 4 patterns: Immediate, Query (with polling), One-way, Timeout/Error scenarios
- Data flow summary table
- Multi-client safety and resilience

**[Extension Message Protocol](extension-message-protocol.md)** ⭐ **NEW**
- All 6 HTTP message types (GET /pending-queries, POST /dom-result, etc.)
- Complete request/response JSON schemas
- State machine for extension connection lifecycle
- Reliability patterns (idempotent IDs, timeout escalation, circuit breaker)

**[Data Capture Pipeline](data-capture-pipeline.md)** ⭐ **NEW**
- Continuous telemetry flow from page → extension → server
- How page observers work (console, fetch, XHR, WebSocket, performance, errors)
- Serialization, redaction, enrichment pipeline
- Ring buffer management and memory enforcement
- Detailed event type specifications (console, network, actions, WS, perf, errors)

### ⚡ Advanced Patterns

**[Async Queue-and-Poll Flow](async-queue-flow.md)**
- Core non-blocking pattern explained
- End-to-end sequence diagram
- Queue state machine
- Multi-client isolation
- Performance characteristics (< 10ms response to AI)

**[Correlation ID Lifecycle](correlation-id-lifecycle.md)**
- How AI agents track command status (pending/complete/expired)
- State transitions and AI polling patterns
- Data structures (completedResults, expiredQueries)
- Lock hierarchy and thread safety
- Troubleshooting guide

### 🛡️ Infrastructure & Quality

**[5-Layer Architectural Protection](quality/5-layer-protection.md)**
- Defense-in-depth system preventing architecture deletion
- All 5 enforcement layers (pre-commit hook, tests, validation, CI)
- Protection matrix showing what each layer checks
- Bypass requirements and justification
- Historical context and lessons learned

**[Flame Flicker Visual Indicator](ui/flame-flicker-visual.md)**
- Browser tab favicon animation showing AI Pilot active state
- Visual state machine (NotTracking → TrackingOnly → AIPilotActive)
- 8-frame animation sequence and SVG structure
- Message flow (popup → background → content)
- Performance considerations

## Diagram Types

### Mermaid Sequence Diagrams

- Async queue-and-poll flow
- Correlation ID polling
- Flame flicker message flow

### Mermaid State Machines

- Queue states (pending/complete/expired)
- Correlation ID lifecycle
- Flame visual states

### Mermaid Flowcharts

- 5-layer protection
- Data capture flow
- Browser control flow

### C4 Architecture Diagrams

- System context
- Container details
- Component relationships

## Usage

All diagrams use [Mermaid](https://mermaid.js.org/) syntax, which **renders automatically on GitHub**.

To view locally:

1. Install Mermaid CLI: `npm install -g @mermaid-js/mermaid-cli`
2. Generate PNG: `mmdc -i async-queue-flow.md -o async-queue-flow.png`

Or use VS Code extension: [Mermaid Preview](https://marketplace.visualstudio.com/items?itemName=bierner.markdown-mermaid)

## Contributing

When adding new diagrams:

- Use consistent color scheme (see existing diagrams)
- Include both overview + detailed views
- Add "References" section linking to code
- Update this README

## Color Scheme

Standard Kaboom colors used in diagrams:

| Color | Hex | Usage |
| --- | --- | --- |
| 🟠 Orange | #fb923c | Primary elements, warnings |
| 🟡 Yellow | #fde047 | Secondary elements, in-progress |
| 🟢 Green | #3fb950 | Success states, safe operations |
| 🔵 Blue | #58a6ff | Info, neutral states |
| 🔴 Red | #f85149 | Errors, failures, dangerous |
| ⚫ Dark | #1a1a1a | Backgrounds, inactive |

## References

- [ADR Index](../README.md)
- [Codebase Canon](../../core/architecture/codebase-canon-v5.3.md)
- [Feature Documentation](../../features/README.md)
