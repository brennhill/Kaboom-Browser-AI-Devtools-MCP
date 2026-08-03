---
doc_type: feature_index
feature_id: feature-kaboom-ci
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - Makefile
  - .github/workflows/architecture-validation.yml
  - .github/workflows/ci.yml
  - .github/workflows/cut-release.yml
  - .github/workflows/release.yml
  - .github/workflows/validate-versions.yml
  - .golangci.yml
  - scripts/build/generate-wire-types.js
  - scripts/build/run-go-coverage.sh
  - scripts/security/check-npm-audit.mjs
  - scripts/security/npm-audit-policy.json
  - scripts/docs/features/check-feature-bundles.js
  - scripts/docs/site/check-gokaboom-content-contract.mjs
  - scripts/docs/reference/check-reference-schema-sync.mjs
  - gokaboom.dev/src/content/docs/reference/configure.md
  - scripts/lint-documentation.py
  - scripts/check-dormant-tests.sh
  - scripts/test-js-sharded.sh
  - package.json
test_paths:
  - scripts/security/check-npm-audit.test.mjs
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
last_verified_version: 0.9.0
last_verified_date: 2026-08-03
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
- Hosted CI and release workflows invoke Make-owned entrypoints for version,
  wire, schema, architecture, coverage, and security invariants. Contract tests
  reject copied thresholds, reduced scopes, direct script bypasses, and workflow
  drift, so the documented local command reproduces the hosted failure.
- The canonical security gate scans every `cmd/browser-agent` and `internal` Go
  package with gosec and govulncheck, audits production npm dependencies with
  zero tolerance, and admits build-tool findings only through exact advisory
  fingerprints with Beads references, owners, meaningful rationales, explicit
  `build_only` scope, and expiry dates. Expired or stale entries fail the gate;
  the daily scheduled workflow reruns the same canonical check so newly
  disclosed vulnerabilities surface without a source change. Active workflows
  pin the patched Go 1.25.12 toolchain declared by `go.mod`.

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
