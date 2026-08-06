---
status: shipped
scope: feature/query-dom/qa
ai-priority: high
tags: [testing, qa, dom]
relates-to: [product-spec.md, tech-spec.md, ../../core/protocol/mcp-command-option-matrix.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-query-dom
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Query DOM QA Plan (TARGET)

## Automated Coverage
- `cmd/browser-agent/tools_analyze_handler_test.go`
- `cmd/browser-agent/tools_analyze_route_test.go`

## Required Scenarios
1. Request validation
- Missing selector fails.
- Invalid JSON fails.

2. Selector behavior
- Valid selector with matches.
- Valid selector with zero matches.
- Invalid selector error propagation.

3. Frame behavior
- Default frame query.
- `frame:"all"` aggregation.
- `frame` index and selector targeting.
- Invalid frame target behavior.

4. Async control behavior
- Default sync completion.
- Background mode returns correlation handle.
- Polling through `observe(command_result)` succeeds.

5. Resilience
- Extension disconnected path.
- Pending command expiration behavior.

## Manual UAT
1. `analyze({what:"dom", selector:"h1"})`
2. `analyze({what:"dom", selector:"*", frame:"all"})`
3. `analyze({what:"dom", selector:"[invalid::syntax"})`
4. Run with `background:true` and poll command result.
