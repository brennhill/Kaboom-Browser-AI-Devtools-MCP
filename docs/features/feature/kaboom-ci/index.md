---
doc_type: feature_index
feature_id: feature-kaboom-ci
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - Makefile
  - .github/workflows/ci.yml
  - .golangci.yml
  - scripts/build/generate-wire-types.js
  - scripts/build/run-go-coverage.sh
  - scripts/docs/features/check-feature-bundles.js
  - scripts/docs/site/check-gokaboom-content-contract.mjs
  - scripts/docs/reference/check-reference-schema-sync.mjs
  - scripts/lint-documentation.py
  - scripts/check-dormant-tests.sh
  - scripts/test-js-sharded.sh
  - package.json
test_paths:
  - tests/extension/contracts/tooling-contracts.test.js
  - scripts/docs/features/check-feature-bundles.test.mjs
  - cmd/browser-agent/tools_schema_parity_test.go
  - cmd/browser-agent/tools_interact_navigate_document_test.go
  - cmd/browser-agent/tools_contract_enforcement_test.go
  - tests/cli/runtime/cli-integration.test.cjs
  - tests/cli/runtime/config.test.cjs
  - tests/cli/runtime/doctor.test.cjs
  - tests/cli/lifecycle/server-install-hardening.test.cjs
  - tests/site/gokaboom-domain-contract.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Kaboom Ci

## TL;DR

- Status: proposed
- Tool: observe, generate
- Mode/Action: observe(errors, logs, network_waterfall, network_bodies, websocket_events, performance, timeline), generate(har, sarif)
- Location: `docs/features/feature/kaboom-ci`
- Fast Gate: `make verify-llm` (typical warm-cache runtime ~60-120s)
- Added Gates: docs integrity (`docs:lint:integrity`) and Go import boundaries (`depguard`)
- Hosted and local Go CI both invoke `make test-cover`, whose canonical aggregate
  runner merges package and real-binary subprocess coverage and enforces the
  unchanged 89% minimum. GitHub retains the merged and component profiles for
  diagnosis even when the gate fails.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_KABOOM_CI_001
- FEATURE_KABOOM_CI_002
- FEATURE_KABOOM_CI_003

## Code and Tests

- `tests/extension/contracts/tooling-contracts.test.js` prevents hosted CI,
  `ci-go`, the 89% threshold, and retained coverage artifacts from drifting.
