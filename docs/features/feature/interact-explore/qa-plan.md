---
doc_type: qa-plan
feature_id: feature-interact-explore
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

# Interact QA Plan (TARGET)

## Automated Coverage
- `cmd/browser-agent/tools_interact_rich_test.go`
- `cmd/browser-agent/tools_interact_upload_test.go`
- `cmd/browser-agent/tools_interact_state_test.go`
- `cmd/browser-agent/tools_interact_draw_test.go`

## Required Scenarios
1. Action dispatch
- Every schema action reaches a valid handler path.

2. Pilot dependency
- Disabled pilot returns structured error for browser-control actions.

3. Async/sync control
- Default sync completion.
- Background/async queued behavior with correlation IDs.

4. DOM primitives
- Required parameter validation (`selector`, `text`, `value`, `name`).
- Command result contains enrichment fields and error propagation.

5. Special actions
- Upload queueing and completion.
- Draw mode start + downstream annotation retrieval.
- Screenshot alias behavior.

## Manual UAT
1. `interact(what:"navigate", url:"https://example.com")`
2. `interact(what:"click", selector:"text=Submit")`
3. `interact(what:"execute_js", script:"document.title")`
4. `interact(what:"upload", selector:"input[type=file]", file_path:"<abs path>")`
5. `interact(what:"draw_mode_start")` then retrieve annotations with analyze.
