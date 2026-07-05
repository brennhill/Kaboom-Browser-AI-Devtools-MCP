---
doc_type: product-spec
feature_id: feature-link-health
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

# Link Health Product Spec (TARGET)

## Purpose
Detect broken, redirected, auth-gated, timed-out, and CORS-blocked links from the current page, then optionally validate unresolved links server-side.

## Canonical APIs
1. Extension-backed page scan
```json
{"tool":"analyze","arguments":{"what":"link_health","domain":"example.com"}}
```

2. Server-side URL verification
```json
{"tool":"analyze","arguments":{"what":"link_validation","urls":["https://..."],"timeout_ms":15000,"max_workers":20}}
```

## Behavior Guarantees
1. `link_health` classifies outcomes: `ok`, `redirect`, `requires_auth`, `broken`, `timeout`, `cors_blocked`.
2. `link_validation` verifies explicit URL lists with bounded concurrency and timeout.
3. Non-http(s) URLs are ignored in server-side validation.
4. SSRF protection is applied during server-side validation transport.

## Requirements
- `LHEALTH_PROD_001`: `link_health` must run through extension query path with correlation support.
- `LHEALTH_PROD_002`: `link_validation` requires non-empty `urls` and enforces limits.
- `LHEALTH_PROD_003`: server-side checker must classify 2xx/3xx/401/403/4xx/5xx deterministically.
- `LHEALTH_PROD_004`: CORS-blocked client checks must remain distinguishable from confirmed broken links.
