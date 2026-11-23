---
doc_type: tech-spec
feature_id: feature-link-health
status: shipped
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Link Health Tech Spec (TARGET)

## `analyze.what = "link_health"` flow
1. Server queues pending query type `link_health` (`toolanalyze.HandleLinkHealth`, dispatched from `analyzedispatch`).
2. Extension receives command via `/sync`.
3. Background (`registerPassthrough('link_health', 'link_health_query', ...)` in `src/background/commands/analyze.ts`) sends a `link_health_query` runtime message; the content script relays it to the inject context as `kaboom_link_health_query`.
4. Inject runs `checkLinkHealth` in `src/lib/analysis/link-health.ts`.
5. Result returns via sync command results and is surfaced via command tracker.

## `analyze.what = "link_validation"` flow
- Server-only path in `linkvalidation.HandleLinkValidation` (core logic `ValidateLinksServerSide` in `internal/tools/analyze/link_validation.go`):
- validates `urls`, `timeout_ms`, `max_workers`
- filters for http/https
- executes worker pool HTTP checks
- uses SSRF-safe transport and redirect bounds

## Option Handling
- `domain` filters extracted links for `link_health`.
- `timeout_ms` and `max_workers` apply to both client and server variants (bounded by implementation limits).

## Code Anchors
- `internal/tools/analyze/handler/navigation.go`
- `internal/tools/analyze/handler/analyzedispatch/dispatcher.go`
- `internal/tools/analyze/handler/linkvalidation/handler.go`
- `internal/tools/analyze/link_validation.go`
- `src/lib/analysis/link-health.ts`
- `src/background/commands/analyze.ts`
- `src/content/runtime-message-listener.ts`
- `src/content/message-handlers.ts`
- `src/inject/message-handlers.ts`
