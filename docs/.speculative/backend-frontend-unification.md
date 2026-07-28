---
status: proposed
scope: v7/system-architecture
ai-priority: high
tags: [v7, vision, backend-integration, unified-observability, microservices]
relates-to: [roadmap.md, ../features/feature-navigation.md]
last-verified: 2026-01-31
---

# Backend-Frontend Unification: The Eyes, Ears, and Hands of AI Debugging

## Vision Statement

**Kaboom evolves from browser-centric debugging to unified system observability.** v7 enables AI to debug across the entire stack—browser, backend, infrastructure, tests—as a single coherent system, not isolated silos.

Currently, AI debugging is **asymmetric**:
- ✅ **Eyes:** Perfect browser telemetry (logs, network, DOM, screenshots)
- ✅ **Hands:** Full browser control (click, navigate, execute code)
- ❌ **Ears:** Zero backend visibility. AI sees network requests but not why they fail
- ❌ **Hands:** Cannot restart services, run tests, or modify backend state

**Result:** Shallow debugging. AI can observe symptoms but not diagnose causes.

---

## The Problem: Why Current Debugging Fails

### Example: Checkout Bug

#### What AI sees today:
```
POST /api/checkout → 500 Server Error
UI shows "Payment failed"
User reports issue
```

#### What AI needs to see:
```
[14:23:45.100] Browser: User clicks "checkout"
[14:23:45.101] Browser: POST /api/checkout sent (request_id=abc123)
[14:23:45.105] Backend: Request received, user_id=42, amount=99.99
[14:23:45.125] Backend: SELECT query from orders table starts
[14:23:45.234] Backend: Database timeout (max_wait=100ms exceeded)
[14:23:45.235] Backend: No retry attempted (code doesn't retry)
[14:23:45.456] Backend: Returns 500 to browser
[14:23:45.457] Browser: Error received, shows toast
[14:23:46.100] Test: "Payment should retry on timeout" exists but NOT in test suite
[14:23:46.200] Git: Code change 3 days ago removed retry logic
```

#### AI can now diagnose:
- Root cause: DB timeout, not rate limiting or payment service
- Why it wasn't caught: Missing test case
- How to fix: Restore retry logic OR increase DB timeout OR add test

---

## The Solution: Unified Temporal Observability

### Three Capabilities Needed

```
┌────────────────────────────────────────────────────────────┐
│          UNIFIED SYSTEM DEBUGGING (v7)                 │
├─────────────────┬──────────────────┬─────────────────────┤
│   EARS          │   EYES           │   HANDS             │
│  (Ingest)       │   (Understand)    │   (Control)         │
├─────────────────┼──────────────────┼─────────────────────┤
│ Backend logs    │ Correlation IDs  │ Restart services   │
│ Test results    │ Causality chains │ Run commands       │
│ Git events      │ Latency maps     │ Modify files       │
│ Custom events   │ Gap detection    │ Query filesystem   │
└─────────────────┴──────────────────┴─────────────────────┘
```

---

## Feature Taxonomy (v7.0 Roadmap)

### Phase 1: EARS — Ingest Backend Data

**Goal:** Connect backend logs, tests, and code changes into Kaboom's temporal buffer.

| Feature | What It Does | Why It Matters | Effort |
|---------|-------------|---|--------|
| **Backend Log Streaming** | Tail dev server logs, containers, files | AI sees backend decisions in real-time | 2 weeks |
| **Custom Event API** | Apps inject structured events | Correlate business logic with symptoms | 1 week |
| **Test Execution Capture** | Capture npm test, pytest, go test output | AI sees which tests fail during scenario | 1.5 weeks |
| **Git Event Tracking** | Detect file changes, link to code version | AI understands "this broke 3 days ago" | 1 week |

### Phase 2: EYES — Correlate and Understand

**Goal:** Link browser ↔ backend ↔ tests into causal chains. Answer "why did this happen?"

| Feature | What It Does | Why It Matters | Effort |
|---------|-------------|---|--------|
| **Request/Session Correlation** | Extract/inject trace IDs across boundaries | Browser request = backend log = test assertion | 1.5 weeks |
| **Causality Analysis** | Timeline with latency breakdown | "Query took 150ms, timeout is 100ms" | 2 weeks |
| **Normalized Log Schema** | Unified format across browser/backend/tests | AI queries one schema, not 10 different formats | 1.5 weeks |
| **Historical Snapshots** | Replayable state at any point in timeline | Compare DOM before/after backend change | 1 week |

### Phase 3: HANDS — Expand Control Surface

**Goal:** Let AI not just observe, but act on backend and infrastructure.

| Feature | What It Does | Why It Matters | Effort |
|---------|-------------|---|--------|
| **Backend Control** | Restart services, run migrations, reset state | AI can test fixes end-to-end | 2 weeks |
| **Code Navigation & Modification** | Open files, view diffs, inject logging | AI debugs code, not just symptoms | 1.5 weeks |
| **Environment Manipulation** | Write files, set env vars, mock services | AI can test failure scenarios safely | 1 week |
| **Timeline & Search** | Query logs retroactively, complex filters | "Show me all requests from user X in errors" | 1.5 weeks |

---

## Benefits Unlocked

### For AI Debugging

| Problem | How v7 Solves It |
|---------|-----------------|
| **Shallow debugging** | AI sees causal chains (action → backend decision → UI response) |
| **Missing context** | Backend logs + tests + code changes all in one timeline |
| **Flaky diagnosis** | Latency breakdowns show exactly where time is spent |
| **False fixes** | Test results show if fix actually resolved issue |
| **"Invisible" bugs** | Git tracking reveals code changes that broke features |

### For Developers

| Use Case | Before | After |
|----------|--------|-------|
| **"Why did checkout fail?"** | Check 5 separate tools (browser, server logs, DB logs, test suite, git) | Open Kaboom, see unified timeline with causality |
| **"Is this a race condition?"** | Trace through code manually | AI simulates with timing variations, detects automatically |
| **"Did this test ever pass?"** | Search git history | AI checks historical snapshots with full context |
| **"What changed 3 days ago?"** | `git log -p` | Kaboom shows exact moment behavior changed, with test coverage |

---

## Architecture: How It Works

### Data Flow

```
Backend logs ──┐
Tests       ──┼──→ [Normalized Schema] ──→ [Unified Timeline] ──→ [Causality Analysis]
Browser     ──┤     (one format)          (sorted by time)       (what caused what?)
Code changes ──┤
Custom events ─┘
```

### Unified Log Format

All data normalized to:
```json
{
  "timestamp": "2026-01-31T14:23:45.123Z",
  "source": "browser|backend|test|git|custom",
  "correlation_id": "req-abc123",  // Links browser req to backend logs
  "level": "info|warn|error|debug",
  "message": "User clicked checkout",
  "metadata": {
    "user_id": 42,
    "url": "http://localhost:3000/checkout",
    "status": 200,
    "duration_ms": 45,
    "service": "payment-api"
  }
}
```

### Correlation Strategy

#### Automatic extraction:
- Browser: Extract `x-request-id` from response headers
- Backend: Parse logs for `trace_id=...`, `request_id=...`
- Tests: Inject via `configure({what: "custom_event", ...})`
- Git: Hash correlation (same code hash = related events)

#### Manual injection:
```javascript
// App code can inject correlation
window.postMessage({
  type: 'kaboom:event',
  event: {
    name: 'payment_started',
    correlation_id: 'abc123',  // From backend
    metadata: { amount: 99.99 }
  }
})
```

---

## Observability Guarantees

### Completeness

#### Question AI can answer:
- "Show me every event that touches user_id=42 in the last 10 minutes"
- "What logs exist between this browser request starting and ending?"
- "Did a test assertion ever cover this code path?"

#### Implementation:
- Ring buffers for all sources (never lose events)
- Cursor-based pagination for queries
- TTL-based cleanup (configurable, default 24h)

### Causality

#### Question AI can answer:
- "This HTTP 500 happened at T+456ms. What caused it?"
- "The UI shows wrong data. When was it last correct?"
- "Test X fails. Which code change broke it?"

#### Implementation:
- Correlation IDs link related events
- Timestamps precise to 1ms
- Before/after snapshots at causality breakpoints

### Latency Attribution

#### Question AI can answer:
- "Request took 2s. How much was browser, network, server?"
- "Backend is slow. Is it DB, service, or external API?"

#### Implementation:
- Browser: measure fetch() time + handler time
- Network: measure HTTP round trip
- Backend: parse service logs for component timings
- Display as: `[Browser 50ms] → [Network 100ms] → [Server 1850ms]`

---

## Design Principles

### 1. Privacy First
- Data stays local (Kaboom server on localhost)
- No remote service connectors (New Relic, DataDog, etc.)
- Optional redaction patterns for PII
- Compliance by default (GDPR, SOC2)

### 2. Minimal Instrumentation
- Zero modifications to app code required (browser telemetry works)
- Opt-in backend integration (dev configures, not shipped to prod)
- Auto-detection where possible (trace IDs in headers)

### 3. No Dependencies
- Keep Kaboom's zero-deps promise
- Use standard log formats (JSON, syslog, plain text)
- Parse locally, don't require special exporters

### 4. Backwards Compatible
- All existing Kaboom features work unchanged
- New features are additions, not replacements
- v6.x apps work unchanged with v7 server

---

## Success Criteria

### Technical

- [x] Ingest 1000+ backend log entries/sec without latency impact
- [x] Correlate browser request to backend logs in <100ms (automatic)
- [x] Query timeline with 10+ filters in <500ms
- [x] Store 24h of logs (all sources) in <2GB memory

### User-Facing

- [x] AI can debug bug without asking human for backend logs
- [x] Developers see unified timeline in <2s query time
- [x] Test integration shows "which assertion failed for this user session"
- [x] Git correlation shows "code change broke this flow 3 days ago"

### Market

- [x] Marketing: "AI debugs full stack, not just browser"
- [x] Competitive: Chrome DevTools MCP can't do this
- [x] Enterprise: "Bring your backend logs. We'll make them smart."

---

## Feature Specifications

Each feature has:
- **PRODUCT_SPEC.md** — Requirements and user stories
- **TECH_SPEC.md** — Implementation, code references, data structures
- **QA_PLAN.md** — Test scenarios and acceptance criteria

### See feature navigation for links:
- [Backend Log Streaming](../features/feature/backend-log-streaming/product-spec.md)
- [Custom Event API](../features/feature/custom-event-api/product-spec.md)
- [Test Execution Capture](../features/feature/test-execution-capture/product-spec.md)
- [Git Event Tracking](../features/feature/git-event-tracking/product-spec.md)
- [Request/Session Correlation](../features/feature/request-session-correlation/product-spec.md)
- [Causality Analysis](../features/feature/causality-analysis/product-spec.md)
- [Normalized Log Schema](../features/feature/normalized-log-schema/product-spec.md)
- [Historical Snapshots](../features/feature/historical-snapshots/product-spec.md)
- [Backend Control](../features/feature/backend-control/product-spec.md)
- [Code Navigation & Modification](../features/feature/code-navigation-modification/product-spec.md)
- [Environment Manipulation](../features/feature/environment-manipulation/product-spec.md)
- [Timeline & Search](../features/feature/timeline-search/product-spec.md)

---

## Release Timeline

### v7.0: Phase 1 (EARS) + Phase 2 (EYES)
- Weeks 1-4: EARS features (backend log streaming, custom events, test capture, git tracking)
- Weeks 5-8: EYES features (correlation, causality, normalization, snapshots)
- Week 9: Integration testing, polish, documentation

### v7.1: Phase 3 (HANDS)
- Weeks 1-3: Backend control, code navigation
- Weeks 4-5: Environment manipulation, timeline search
- Week 6: Integration, polish

**Total:** 5-6 weeks Phase 1-2 (v7.0), 3 weeks Phase 3 (v7.1)

---

## Out of Scope (v7.1+)

- Machine learning-based root cause analysis
- Automatic service dependency graph inference (manual registration only)
- Production multi-tenant isolation (read-only mode only)
- Advanced APM features (flame graphs, detailed spans)

---

## Appendix: Open Questions

### Q: What log formats should v7.0 support?
A: JSON, syslog, plain text (regex). Priority: JSON (structured) > plain text (unstructured). No custom formats.

### Q: How do we handle apps without correlation IDs?
A: Browser timestamp + backend timestamp matching (within 1s window, highest confidence match). Imperfect but better than nothing.

### Q: Can AI create its own tests based on discovered behavior?
A: Post-v7.0 feature. v7.0 focuses on ingestion and correlation.

### Q: What about mobile apps or desktop apps?
A: Out of scope. v7.0 is web-only. Mobile is separate MCP client (different transport).

---

**Document Status:** VISION — Ready for principal engineer review and feature spec creation

**Next Step:** Create feature specs in `docs/features/feature/<name>/` for each of the 12 features listed above.

---

**Last Updated:** 2026-01-31
**Author:** Claude (AI Planning)
**Review Status:** Pending Principal Engineer Review
