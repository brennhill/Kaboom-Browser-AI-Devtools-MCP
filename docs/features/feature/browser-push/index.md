---
doc_type: feature_index
feature_id: browser-push
status: implementation
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - internal/push/
  - cmd/browser-agent/internal/pushapi/runtime.go
  - cmd/browser-agent/internal/pushapi/handler.go
  - cmd/browser-agent/internal/mediaapi/screenshots.go
  - cmd/browser-agent/internal/mediaapi/draw_mode.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/deps.go
  - cmd/browser-agent/internal/toolobserve/inbox.go
  - src/background/push-handler.ts
  - src/content/ui/chat-widget.ts
test_paths:
  - cmd/browser-agent/internal/pushapi/handler_test.go
  - cmd/browser-agent/internal/pushapi/runtime_test.go
  - internal/push/inbox_test.go
  - internal/push/router_test.go
  - internal/push/sampling_test.go
  - cmd/browser-agent/internal/pushapi/runtime_test.go
  - cmd/browser-agent/internal/pushapi/handler_test.go
  - cmd/browser-agent/tools_analyze_annotations_draw_test.go
  - cmd/browser-agent/tools_observe_inbox_test.go
  - cmd/browser-agent/internal/toolobserve/toolobserve_coverage_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - tests/extension/branding/push-handler-branding.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Browser Push

Push browser content (annotations, screenshots, chat messages) to the AI automatically — no chat round-trip required.

## TL;DR

- Sampling request IDs mask the sign bit before their documented safe integer
  conversion; the expanded Go security gate verifies the full package.

- Status: implementation
- Tool: observe (inbox), internal (push delivery)
- Mode/Action: MCP sampling, notifications fallback, inbox polling fallback
- Shortcuts: Alt+Shift+S (screenshot), Alt+Shift+C (chat widget)
- Location: `docs/features/feature/browser-push`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- PUSH_001 — MCP sampling delivery
- PUSH_002 — Notifications fallback
- PUSH_003 — Inbox polling fallback
- PUSH_004 — Screenshot push hotkey (Alt+Shift+S)
- PUSH_005 — Client capability detection
- PUSH_006 — Chat widget (Alt+Shift+C)
- PUSH_007 — Draw mode auto-push on ESC

## Code and Tests

### Go (daemon)

| File | Purpose | Tests |
|------|---------|-------|
| `internal/push/types.go` | PushEvent, ClientCapabilities, SamplingRequest | — |
| `internal/push/inbox.go` | Bounded FIFO queue (50 events) | `inbox_test.go` (8 tests) |
| `internal/push/router.go` | Delivery router: sampling→notification→inbox | `router_test.go` (6 tests) |
| `internal/push/sampling.go` | MCP sampling/createMessage builder | `sampling_test.go` (5 tests) |
| `cmd/browser-agent/internal/pushapi/runtime.go` | Negotiated capability/framing state and MCP outbound delivery | `runtime_test.go` |
| `cmd/browser-agent/internal/pushapi/handler.go` | Push HTTP parsing, event delivery, draining, and annotation routing | `handler_test.go` |
| `cmd/browser-agent/internal/toolobserve/dispatcher.go` | observe(inbox) adapter + piggyback wiring | `tools_observe_inbox_test.go` (6 tests) |
| `cmd/browser-agent/internal/toolobserve/inbox.go` | inbox response and piggyback behavior | `tools_observe_inbox_test.go` (6 tests) |

The observe dispatcher receives the inbox and command operations through an
explicit local dependency bundle. It does not require a ToolHandler host or
retain inbox/connectivity forwarding methods.

### TypeScript (extension)

| File | Purpose |
|------|---------|
| `src/background/push-handler.ts` | Keyboard listeners, fetch push, capability cache |
| `src/content/ui/chat-widget.ts` | Inline chat widget with ARIA, focus trapping, pin toggle |

### Extension Tests

- `tests/extension/branding/push-handler-branding.test.js` validates Kaboom-branded error toasts when screenshot push cannot reach the daemon.
