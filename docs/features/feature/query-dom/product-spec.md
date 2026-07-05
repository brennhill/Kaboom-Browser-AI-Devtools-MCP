---
feature: query-dom
status: shipped
version: 0.7.12
tool: analyze
mode: dom
authors: []
created: 2026-01-28
updated: 2026-02-17
doc_type: product-spec
feature_id: feature-query-dom
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Query DOM Product Spec (TARGET)

## Purpose
Query live DOM state using CSS selectors and return structured, model-readable element data.

## Canonical API
```json
{
  "tool": "analyze",
  "arguments": {
    "what": "dom",
    "selector": "button.submit"
  }
}
```

Optional targeting and execution controls:
- `tab_id`
- `frame` (`"all"`, selector, or zero-based frame index)
- `sync`, `wait`, `background`

## Behavior Guarantees
1. `selector` is required.
2. Default behavior is synchronous wait; async controls are supported.
3. Results are correlation-addressable when queued/background.
4. Frame-aware queries support single-frame and aggregated multi-frame results.
5. Invalid selector/frame errors are returned as structured failures.

## Requirements
- `QDOM_PROD_001`: `analyze(what:"dom")` is the canonical DOM query contract.
- `QDOM_PROD_002`: unsupported legacy docs using `configure(query_dom)` must be treated as historical only.
- `QDOM_PROD_003`: server must preserve extension error semantics (`invalid_frame`, `frame_not_found`, selector errors).
- `QDOM_PROD_004`: result payload must clearly indicate total vs returned matches for truncation awareness.

## Out of Scope
- XPath and semantic selector synthesis.
- Shadow DOM policy changes beyond current implementation defaults.
