---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Feature-to-Strategy Map

**Quick lookup**: How does each feature align with product strategy and competitive positioning?

## Strategic Pillars

### Pillar 1: AI-Native Architecture
**Goal:** Make Kaboom the default observability tool for AI coding assistants

**Features:**
- MCP protocol integration (all tools)
- Async queue-and-poll (non-blocking browser control)
- Correlation ID tracking (command lifecycle visibility)
- Structured error messages (AI-parseable)

**Competitive edge:**
- Competitors: Traditional DevTools (not AI-aware)
- Kaboom: Built specifically for AI agent consumption

---

### Pillar 2: Zero-Dependency Self-Hosted
**Goal:** Enterprise-grade reliability without vendor lock-in

**Features:**
- Zero production dependencies (Go + vanilla JS)
- Localhost-only (no SaaS, no external APIs)
- Customer-controlled storage (local disk or their infra)
- Open source AGPL-3.0

**Competitive edge:**
- Competitors: SaaS observability (Sentry, LogRocket) - data leaves network
- Kaboom: All data stays on customer infrastructure

---

### Pillar 3: Real-Time Browser Control
**Goal:** Enable autonomous debugging by AI agents

**Features:**
- Browser automation via MCP (`interact()` tool)
- DOM query without page instrumentation
- Network interception and replay
- State snapshots and time-travel

**Competitive edge:**
- Competitors: Playwright (requires test framework integration)
- Kaboom: Works on production sites, no code changes

---

### Pillar 4: Visual Feedback
**Goal:** Developers know when AI is in control

**Features:**
- Flickering flame favicon (tab indicator)
- Connection status badge (extension popup)
- Tab tracking UI (explicit opt-in)

**Competitive edge:**
- Competitors: Silent background operation (confusing)
- Kaboom: Clear visual feedback, peace of mind

---

## Feature Categorization

### Core Infrastructure (Must Never Break)
- Async queue-and-poll → Pillar 1 + 3
- MCP protocol handling → Pillar 1
- Extension polling → Pillar 1 + 3

**Protection:** 5-layer enforcement (ADR-002)

---

### Differentiators (Unique to Kaboom)
- Correlation ID tracking → Pillar 1
- Flickering flame favicon → Pillar 4
- Zero dependencies → Pillar 2
- Localhost-only → Pillar 2

**Marketing:** Highlight in all communications

---

### Nice-to-Have (Enhance UX)
- Clickable tracked tab URL → Pillar 4
- React/Vue/Svelte form helpers → Pillar 3
- Performance snapshots → Future pillar (observability depth)

**Priority:** After core features ship

---

## Competitive Positioning

| Feature | Kaboom | Traditional DevTools | SaaS Observability | Playwright |
|---------|----------|---------------------|--------------------|-----------
| AI-Native | ✅ MCP protocol | ❌ Human-focused UI | ⚠️ API exists | ❌ Test framework |
| Self-Hosted | ✅ Localhost only | ✅ Browser-local | ❌ Cloud SaaS | ✅ Local test runner |
| Zero Setup | ✅ npx + extension | ✅ Built-in | ❌ Account signup | ❌ Framework integration |
| Real-Time Control | ✅ Async queue | ❌ Manual only | ❌ Replay only | ✅ Automation API |
| Production Safe | ✅ No instrumentation | ✅ No instrumentation | ⚠️ Needs SDK | ❌ Test env only |
| Visual Feedback | ✅ Flame flicker | ❌ Silent | ❌ Dashboard | ❌ Silent |

**Unique combination:** AI-Native + Self-Hosted + Real-Time Control

---

## Product Roadmap Alignment

### v5.4 (Current) - Foundation
**Shipped:**
- ✅ Async queue reliability (Pillar 1, 3)
- ✅ Correlation ID tracking (Pillar 1)
- ✅ Visual feedback (Pillar 4)
- ✅ 5-layer protection (Operational excellence)

**Strategic impact:**
- Enables autonomous debugging (core value prop)
- Eliminates timeout frustrations (table stakes)
- Provides peace of mind (UX differentiator)

---

### v6.0 (Planned) - Depth
**Proposed:**
- Backend log ingestion (extend to server-side)
- Flow recording & playback (reproduce issues)
- API schema detection (understand contracts)
- Performance regression detection

**Strategic alignment:**
- Backend logs → Pillar 1 (AI needs full-stack context)
- Flow recording → Pillar 3 (enable regression testing)
- API schema → Pillar 1 (structured data for AI)

**Competitive impact:**
- Only tool that covers browser + backend
- Only tool with AI-first workflow

---

## Feature Decision Framework

### Should we build feature X?

**Ask:**
1. **Does it align with a strategic pillar?**
   - Yes → Continue
   - No → Reject or defer

2. **Does it increase AI agent effectiveness?**
   - Yes → High priority
   - No → Medium/low priority

3. **Does it require external dependencies?**
   - No → Good fit
   - Yes → Reject (violates Pillar 2)

4. **Can competitors easily copy it?**
   - No (unique to our architecture) → Differentiator
   - Yes (table stakes) → Must-have but not marketing focus

5. **Does it compromise customer data control?**
   - No → Safe to build
   - Yes → Reject (violates Pillar 2)

---

## Feature Sunset Criteria

**Consider removing if:**
1. <1% usage (instrumentation needed)
2. High maintenance cost vs. value
3. Better achieved by external tool
4. Conflicts with strategic pillar

**Do NOT remove if:**
- Protected by ADR (e.g., async queue)
- Core to value proposition
- Required by >5% users

---

## Feature-to-Market-Segment

### Individual Developers
**Value:** Fast debugging, learn from AI
**Key features:**
- Quick setup (npx)
- Visual feedback (know when AI is working)
- No account needed

### Engineering Teams
**Value:** Shared debugging, consistent tooling
**Key features:**
- Self-hosted (data stays internal)
- Zero dependencies (IT approval easier)
- Multi-client support (team collaboration)

### Enterprise
**Value:** Security, compliance, control
**Key features:**
- AGPL-3.0 (no vendor lock-in)
- Customer-controlled storage
- No external dependencies
- Audit trail (correlation IDs)

---

## Strategic Risks & Mitigations

### Risk: Competitors add MCP support
**Mitigation:**
- Async queue architecture (hard to replicate correctly)
- 5-layer protection (maintains reliability advantage)
- Deep AI integration (not just API wrapper)

### Risk: Browser vendors restrict extensions
**Mitigation:**
- Follow MV3 standards strictly
- Minimal permissions
- No content script injection by default

### Risk: Enterprise adoption slow
**Mitigation:**
- Emphasize self-hosted (no SaaS concerns)
- Highlight AGPL-3.0 (no licensing overhead)
- Provide professional diagrams (ADR-002, architecture diagrams)

---

## References

**For product design:**
- [docs/features/](../features/) - Feature specifications
- [docs/roadmap-detailed.md](../../roadmap-detailed.md) - Planned features

**For implementation:**
- [docs/core/product/code-index.md](code-index.md) - Code → feature map
- [docs/architecture/diagrams/](../architecture/diagrams/) - Architecture diagrams

**For decision-making:**
- This document (feature-to-strategy.md)
- Product specs in docs/features/feature/*/product-spec.md
