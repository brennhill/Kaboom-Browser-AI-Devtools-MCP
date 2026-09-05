---
doc_type: feature_index
feature_id: feature-reproduction-scripts
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - cmd/browser-agent/internal/toolgenerate/dispatcher.go
  - cmd/browser-agent/internal/toolgenerate/deps.go
  - internal/reproduction/reproduction.go
  - internal/reproduction/reproduction_kaboom.go
  - internal/reproduction/reproduction_playwright.go
  - internal/reproduction/reproduction_selectors.go
  - internal/reproduction/reproduction_locators.go
  - internal/reproduction/reproduction_utils.go
  - src/lib/page/reproduction.ts
  - internal/capture/actionstore/store.go
test_paths:
  - cmd/browser-agent/internal/toolgenerate/toolgenerate_test.go
  - scripts/tests/workflows/cat-29-reproduction.sh
  - internal/tools/interact/reproduction_test.go
  - internal/reproduction/reproduction_test.go
  - internal/reproduction/reproduction_locators_test.go
  - internal/reproduction/golden_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - tests/extension/reproduction/reproduction-script-fixture.js
  - tests/extension/reproduction/reproduction-script-generation.test.js
  - tests/extension/reproduction/reproduction-script.test.js
  - internal/capture/actionstore/store_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Reproduction Scripts

## TL;DR

- Status: proposed
- Tool: generate
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/reproduction-scripts`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_REPRODUCTION_SCRIPTS_001
- FEATURE_REPRODUCTION_SCRIPTS_002
- FEATURE_REPRODUCTION_SCRIPTS_003

## Code and Tests

- MCP adapter and script formatting: `cmd/browser-agent/internal/toolgenerate/dispatcher.go`
- `output_format` accepts `kaboom-agentic-browser` or `playwright`. The rejection
  advice used to read "Use 'kaboom' or 'playwright'" — a value
  `ValidateOutputFormat` rejects — so a caller following the recovery playbook
  retried forever on a value that could never succeed. A contract test asserts
  every value named in the advice is one the validator accepts.
- The dispatcher receives the canonical capture owner through explicit generate
  composition rather than a ToolHandler-satisfied host interface.
- Page-side reproduction capture: `src/lib/page/reproduction.ts`
- Go behavior coverage: `internal/tools/interact/reproduction_test.go`
- Reproduction evidence reads a detached snapshot from the canonical enhanced-
  action store; the capture root does not provide an action forwarding API.

## Three locators per step, and the environment they ran in

Both backends here now emit, per targeted step, the two locators that survive a re-render the CSS
selector does not — accessibility role plus accessible name, and the viewport coordinate with its
frame — and a header stating what the recording session pinned. The fallback order
(`selector -> ax -> coordinate`) and the pin report live in
`internal/reproduction/reproduction_locators.go`.

See [Session to test](../session-to-test/index.md) for the full contract, the reasoning behind the
order, and the extension-side capture and pinning.
