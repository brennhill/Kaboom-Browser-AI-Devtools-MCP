---
doc_type: feature_index
feature_id: feature-self-testing
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-07-30
code_paths:
  - scripts/smoke-test.sh
  - scripts/smoke-tests/framework-smoke.sh
  - scripts/smoke-tests/14-browser-push.sh
  - scripts/smoke-tests/15-file-upload.sh
  - scripts/smoke-tests/29-framework-selector-resilience.sh
  - scripts/smoke-tests/30-stability-shutdown.sh
  - scripts/test-all-split.sh
  - scripts/test-original-uat.sh
  - scripts/test-new-uat.sh
  - scripts/uat-result-lib.sh
  - scripts/tests/framework/framework.sh
  - scripts/tests/framework/uat-artifacts.sh
  - scripts/tests/framework/uat-user-state.sh
  - scripts/contracts/check-architecture-boundaries.cjs
  - .architecture-boundaries.json
  - scripts/test-all-tools-comprehensive.sh
  - scripts/cleanup-test-daemons.sh
  - cmd/browser-agent/server.go
  - cmd/browser-agent/internal/testpages/http.go
  - cmd/browser-agent/internal/testpages/websocket.go
  - cmd/browser-agent/internal/wsframe/frame.go
test_paths:
  - cmd/browser-agent/internal/testpages/websocket_test.go
  - tests/cli/contracts/uat-harness-regressions.test.cjs
  - scripts/contracts/check-architecture-boundaries.test.cjs
  - tests/cli/contracts/test-layout-contract.test.cjs
  - scripts/smoke-tests/14-browser-push.sh
  - scripts/smoke-tests/15-file-upload.sh
  - scripts/smoke-tests/29-framework-selector-resilience.sh
  - scripts/smoke-tests/30-stability-shutdown.sh
  - scripts/tests/contracts/cat-01-protocol.sh
  - scripts/tests/tools/cat-02-observe.sh
  - scripts/tests/tools/cat-03-generate.sh
  - scripts/tests/tools/cat-04-configure.sh
  - scripts/tests/tools/cat-05-interact.sh
  - scripts/tests/runtime/cat-06-lifecycle.sh
  - scripts/tests/runtime/cat-07-concurrency.sh
  - scripts/tests/contracts/cat-08-security.sh
  - scripts/tests/contracts/cat-09-http.sh
  - scripts/tests/contracts/cat-10-regression.sh
  - scripts/tests/capture/cat-11-data-pipeline.sh
  - scripts/tests/tools/cat-12-rich-actions.sh
  - scripts/tests/browser/cat-13-pilot-contract.sh
  - scripts/tests/browser/cat-14-extension-startup.sh
  - scripts/tests/browser/cat-15-pilot-success-path.sh
  - scripts/tests/contracts/cat-16-api-contract.sh
  - scripts/tests/runtime/cat-17-performance.sh
  - scripts/tests/capture/cat-18-recording.sh
  - scripts/tests/browser/cat-19-link-health.sh
  - scripts/tests/capture/cat-20-noise-persistence.sh
  - scripts/tests/runtime/cat-21-stress.sh
  - scripts/tests/runtime/cat-22-advanced.sh
  - scripts/tests/browser/cat-23-draw-mode.sh
  - scripts/tests/browser/cat-24-upload.sh
  - scripts/tests/browser/cat-33-connected-action-coverage.sh
  - scripts/tests/runtime/cat-26-dynamic-upgrade.sh
  - scripts/tests/workflows/cat-29-reproduction.sh
  - scripts/tests/workflows/cat-30-recording-automation.sh
  - scripts/tests/workflows/cat-31-link-crawling.sh
  - scripts/tests/workflows/cat-32-auto-detect.sh
  - cmd/browser-agent/internal/testpages/testpages_test.go
  - cmd/browser-agent/internal/wsframe/frame_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Self Testing

## TL;DR

- Status: in-progress
- Tool: interact, generate
- Mode/Action: execute_js, test
- Location: `docs/features/feature/self-testing`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_SELF_TESTING_001
- FEATURE_SELF_TESTING_002
- FEATURE_SELF_TESTING_003

## Code and Tests

- Smoke runner lifecycle and post-run daemon availability: `scripts/smoke-test.sh`, `scripts/smoke-tests/framework-smoke.sh`
- Smoke module contracts for push/upload/framework resilience/stability: `scripts/smoke-tests/14-browser-push.sh`, `scripts/smoke-tests/15-file-upload.sh`, `scripts/smoke-tests/29-framework-selector-resilience.sh`, `scripts/smoke-tests/30-stability-shutdown.sh`
- Split UAT orchestration + integrity checks: `scripts/test-all-split.sh`, `scripts/test-original-uat.sh`, `scripts/test-new-uat.sh`
- Shared UAT result parsing: `scripts/uat-result-lib.sh`
- Comprehensive aggregation reports pass/fail/skip counts and fails closed when
  any selected category omits or corrupts its structured result file.
- `make uat` runs both suite boundaries and emits canonical
  `artifacts/uat/uat-results.json` and `uat-results.xml` reports. Both contain
  every selected category, skip reasons, durations, prerequisite readiness,
  aggregation completeness, and user-state restoration status.
- `make check-structure` enforces runtime-context dependency direction,
  ratcheted public-surface budgets, the 800-line and 10-file physical limits,
  dormant-test detection, circular dependency reporting, and zero non-trivial
  clones across background/popup.
- Category daemon lifecycle and result-file contract: `scripts/tests/framework/framework.sh`
- User-state guard: `scripts/tests/framework/uat-user-state.sh` snapshots the
  prior daemon executable, LaunchAgent lifecycle, version, and tracked tab
  before connected UAT, then restores them idempotently on completion or signal.
  Port cleanup is listener-only, so browser processes connected to the daemon
  are never mistaken for port owners or terminated.
- Comprehensive UAT has explicit `offline`, `connected`, and `all` suites.
  Offline contracts use an isolated daemon port; connected-browser categories
  run sequentially on the extension's configured port. The connected preflight
  and every category daemon start use the same bounded readiness barrier for
  daemon health, extension heartbeat, and a tracked browser tab, so startup
  races cannot be misreported as feature skips. Run either boundary with
  `--suite offline|connected`.
- Connected UAT opens and tracks a dedicated local test-harness tab rather than
  navigating the user's tab. Cleanup closes that tab before restoring the
  exact prior daemon and tracked-tab state, including signal-driven exits.
- Connected action coverage derives every live tool action from `tools/list`,
  runs it against the dedicated tracked fixture tab, and fails closed if a
  schema action is omitted. The connected preflight uses isolated daemon state
  so malformed user persistence cannot make the test environment
  nondeterministic; production persistence recovery is validated separately.
- UAT category suites: `scripts/tests/cat-*.sh`
- HTTP fixtures and embedded test pages: `cmd/browser-agent/internal/testpages/http.go`
- WebSocket harness: `cmd/browser-agent/internal/testpages/websocket.go`
- RFC 6455 frame codec (shared with the terminal relay): `cmd/browser-agent/internal/wsframe/frame.go`
- Behavior tests: `cmd/browser-agent/internal/testpages/testpages_test.go`, `cmd/browser-agent/internal/wsframe/frame_test.go`
