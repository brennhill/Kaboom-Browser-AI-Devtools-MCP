---
doc_type: qa-plan
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

# Link Health QA Plan (TARGET)

## Automated Coverage
- `cmd/browser-agent/tools_analyze_validation_test.go`
- `cmd/browser-agent/tools_analyze_route_test.go`
- Extension/unit coverage for `src/lib/link-health.ts`

## Required Scenarios
1. `link_health` classification
- healthy 2xx links
- redirects
- auth-required links (401/403)
- hard failures (4xx/5xx)
- timeouts
- CORS-blocked external links

2. `link_validation` server path
- empty URL list rejection
- non-http(s) filtering
- max URL limit enforcement
- timeout and worker clamping
- redirect handling

3. End-to-end command path
- queued command with correlation
- completion via sync and command tracker
- retrieval via `observe(command_result)`

## Manual UAT
1. Run `analyze(what:"link_health")` on a page with mixed links.
2. Extract `needsServerVerification` URLs and run `analyze(what:"link_validation", urls:[...])`.
3. Compare client vs server classification consistency.
