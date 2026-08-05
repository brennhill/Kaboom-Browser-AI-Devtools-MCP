---
doc_type: feature_index
feature_id: feature-kaboom-ci
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - Makefile
  - .github/workflows/architecture-validation.yml
  - .github/workflows/ci.yml
  - .github/workflows/fuzz.yml
  - .github/workflows/cut-release.yml
  - .github/workflows/release.yml
  - .github/workflows/validate-versions.yml
  - .golangci.yml
  - scripts/build/generate-wire-types.js
  - scripts/build/run-go-coverage.sh
  - scripts/security/check-npm-audit.mjs
  - scripts/security/npm-audit-policy.json
  - scripts/security/go-tool-versions.env
  - scripts/security/install-go-tools.sh
  - scripts/hooks/pre-commit
  - docs/DEVELOPMENT.md
  - scripts/docs/features/check-feature-bundles.js
  - scripts/docs/site/check-gokaboom-content-contract.mjs
  - scripts/docs/reference/check-reference-schema-sync.mjs
  - gokaboom.dev/src/content/docs/reference/configure.md
  - scripts/lint-documentation.py
  - scripts/check-dormant-tests.sh
  - scripts/ci/run-fuzz-campaigns.sh
  - scripts/ci/mutation-cases.json
  - scripts/ci/run-targeted-mutations.mjs
  - scripts/test-js-sharded.sh
  - package.json
  - package-lock.json
test_paths:
  - scripts/security/check-npm-audit.test.mjs
  - scripts/security/go-tool-versions.test.mjs
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
  - internal/capture/sync_security_mode_test.go
  - internal/qafixture/fixture_test.go
  - internal/qafixture/registry_test.go
  - internal/redaction/redaction_fuzz_test.go
  - internal/security/scan/scan_test.go
  - internal/statediag/collector_test.go
  - scripts/ci/run-targeted-mutations.test.mjs
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
- Go scanner versions have one shell/Make-compatible source of truth. Local
  bootstrap, pre-commit guidance, development docs, and hosted CI consume the
  same installer; a contract test rejects direct workflow installs, `latest`,
  and silent pre-commit security skips.
- The current build dependency graph has no audit exceptions. Patched direct
  tooling and narrowly pinned transitive overrides keep the complete npm audit
  clean; the empty policy remains checked in so future exceptions cannot appear
  without the policy's owner, scope, expiry, and issue-link review.
- Pull requests replay the committed seeds for critical fixture, transaction,
  Doctor, runtime-message, generation, redaction, and scanner state machines.
  A scheduled bounded mutation campaign uses the same target inventory and
  retains its manifest, per-target logs, and generated crash corpora so every
  discovered failure can become a permanent deterministic regression seed.
- A pinned, zero-dependency semantic mutation campaign runs each declared
  recovery, generation, queue, redaction, authentication, and reconnect mutant
  in an isolated worktree. It first proves every package baseline is green,
  requires a 100% kill score, and retains a machine-readable survivor report.
- Async bridge contract tests deliver results immediately and prove the query
  store is lossless regardless of whether delivery or waiter subscription wins;
  they never use scheduler delay as a synchronization primitive.
- Command-result elapsed time is a nonnegative millisecond-resolution value;
  contract tests no longer sleep merely to force it above zero.

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
- `scripts/ci/run-fuzz-campaigns.sh` is the sole owner of both deterministic
  seed replay and nightly mutation target selection; `make fuzz-smoke` and
  `make fuzz-nightly` are the canonical local and hosted entrypoints.
- `scripts/ci/run-targeted-mutations.test.mjs` proves that killed and surviving
  mutants are classified honestly and that a broken baseline cannot inflate the
  score; `make mutation-test` runs the scheduled production campaign.
