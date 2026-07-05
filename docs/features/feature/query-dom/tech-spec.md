---
doc_type: tech-spec
feature_id: feature-query-dom
status: shipped
owners: []
last_reviewed: 2026-07-05
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Query DOM Tech Spec (TARGET)

## Server Path
1. `toolQueryDOM` in `cmd/browser-agent/tools_analyze.go` validates `selector`.
2. Server queues pending query type `dom` with correlation ID.
3. Wait/queue behavior is governed by `maybeWaitForCommand`.

## Extension Path
1. `src/background/pending-queries.ts` handles `query.type === 'dom'`.
2. Background sends `DOM_QUERY` to content script.
3. Content relays `KABOOM_DOM_QUERY` to inject script.
4. Inject executes `executeDOMQuery` from `src/lib/dom-queries.ts`.
5. Result returns through sync command-results channel.

## Frame Support
- Frame selection resolves in background before dispatch.
- `frame` handling supports:
- default main frame
- `"all"` aggregation
- frame index
- iframe selector matching
- Aggregation includes per-frame metadata and combined counts.

## Failure Modes
- Invalid JSON args -> structured server error.
- Missing selector -> structured server error.
- Invalid frame target -> `invalid_frame` / `frame_not_found` path.
- Content/inject failure -> structured command error in command result payload.

## Code Anchors
- `cmd/browser-agent/tools_analyze.go`
- `src/background/pending-queries.ts`
- `src/content/message-handlers.ts`
- `src/inject/message-handlers.ts`
- `src/lib/dom-queries.ts`
