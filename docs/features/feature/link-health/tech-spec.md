---
doc_type: tech-spec
feature_id: feature-link-health
status: shipped
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Link Health Tech Spec (TARGET)

## `analyze.what = "link_health"` flow
1. Server queues pending query type `link_health` (`toolAnalyzeLinkHealth`).
2. Extension receives command via `/sync`.
3. Background sends `LINK_HEALTH_QUERY` to content/inject path.
4. Inject runs `checkLinkHealth` in `src/lib/analysis/link-health.ts`.
5. Result returns via sync command results and is surfaced via command tracker.

## `analyze.what = "link_validation"` flow
- Server-only path in `toolValidateLinks`:
- validates `urls`, `timeout_ms`, `max_workers`
- filters for http/https
- executes worker pool HTTP checks
- uses SSRF-safe transport and redirect bounds

## Option Handling
- `domain` filters extracted links for `link_health`.
- `timeout_ms` and `max_workers` apply to both client and server variants (bounded by implementation limits).

## Code Anchors
- `cmd/browser-agent/tools_analyze.go`
- `src/lib/analysis/link-health.ts`
- `src/background/pending-queries.ts`
- `src/content/message-handlers.ts`
- `src/inject/message-handlers.ts`
