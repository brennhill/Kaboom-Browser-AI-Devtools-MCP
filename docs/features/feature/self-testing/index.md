---
doc_type: feature_index
feature_id: feature-self-testing
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-07-27
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
  - scripts/tests/framework.sh
  - cmd/browser-agent/server_routes.go
  - cmd/browser-agent/internal/testpages/http.go
  - cmd/browser-agent/internal/testpages/websocket.go
  - cmd/browser-agent/internal/wsframe/frame.go
test_paths:
  - scripts/smoke-tests/14-browser-push.sh
  - scripts/smoke-tests/15-file-upload.sh
  - scripts/smoke-tests/29-framework-selector-resilience.sh
  - scripts/smoke-tests/30-stability-shutdown.sh
  - scripts/tests/cat-01-protocol.sh
  - scripts/tests/cat-02-observe.sh
  - scripts/tests/cat-03-generate.sh
  - scripts/tests/cat-04-configure.sh
  - scripts/tests/cat-05-interact.sh
  - scripts/tests/cat-06-lifecycle.sh
  - scripts/tests/cat-07-concurrency.sh
  - scripts/tests/cat-08-security.sh
  - scripts/tests/cat-09-http.sh
  - scripts/tests/cat-10-regression.sh
  - scripts/tests/cat-11-data-pipeline.sh
  - scripts/tests/cat-12-rich-actions.sh
  - scripts/tests/cat-13-pilot-contract.sh
  - scripts/tests/cat-14-extension-startup.sh
  - scripts/tests/cat-15-pilot-success-path.sh
  - scripts/tests/cat-16-api-contract.sh
  - scripts/tests/cat-17-performance.sh
  - scripts/tests/cat-18-recording.sh
  - scripts/tests/cat-19-link-health.sh
  - scripts/tests/cat-20-noise-persistence.sh
  - scripts/tests/cat-21-stress.sh
  - scripts/tests/cat-22-advanced.sh
  - scripts/tests/cat-29-reproduction.sh
  - scripts/tests/cat-30-recording-automation.sh
  - scripts/tests/cat-31-link-crawling.sh
  - scripts/tests/cat-32-auto-detect.sh
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
- Category daemon lifecycle and result-file contract: `scripts/tests/framework.sh`
- UAT category suites: `scripts/tests/cat-*.sh`
- HTTP fixtures and embedded test pages: `cmd/browser-agent/internal/testpages/http.go`
- WebSocket harness: `cmd/browser-agent/internal/testpages/websocket.go`
- RFC 6455 frame codec (shared with the terminal relay): `cmd/browser-agent/internal/wsframe/frame.go`
- Behavior tests: `cmd/browser-agent/internal/testpages/testpages_test.go`, `cmd/browser-agent/internal/wsframe/frame_test.go`
