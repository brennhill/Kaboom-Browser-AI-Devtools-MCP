---
doc_type: feature_index
feature_id: feature-security-hardening
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/tools/analyze/handler/securityaudit/handler.go
  - internal/tools/generate/handler/dispatcher.go
  - internal/tools/generate/handler/deps.go
  - internal/tools/generate/handler/artifacts_security_impl.go
  - internal/tools/configure/handler/runtime_modes.go
  - internal/tools/mcp/response.go
  - internal/analysis/security/diff/types.go
  - internal/analysis/security/diff/compare.go
  - internal/analysis/security/diff/snapshot.go
  - internal/analysis/security/diff/helpers_headers_cookies.go
  - internal/analysis/security/diff/helpers_maps_urls.go
  - internal/analysis/security/diff/tool.go
  - internal/analysis/security/policy/policy.go
  - internal/analysis/security/policy/mode.go
  - internal/analysis/security/policy/audit.go
  - internal/analysis/security/scan/scan.go
  - internal/platform/types/wire_log.go
  - internal/analysis/security/scan/checks_policy.go
  - internal/analysis/security/scan/credentials.go
  - internal/analysis/security/csp/types.go
  - internal/analysis/security/csp/store.go
  - internal/analysis/security/csp/generate.go
  - internal/analysis/security/csp/helpers.go
  - internal/analysis/security/csp/tooling.go
  - internal/analysis/security/sri/doc.go
  - internal/analysis/security/sri/types.go
  - internal/analysis/security/sri/generate.go
  - internal/analysis/security/sri/helpers.go
  - internal/analysis/security/sri/tooling.go
  - internal/analysis/security/netflag/netflag.go
  - internal/analysis/security/netflag/flag.go
  - internal/analysis/security/netflag/data.go
  - internal/analysis/security/netflag/detectors.go
  - internal/analysis/security/netflag/distance.go
  - internal/analysis/security/httpsec/url.go
  - internal/analysis/security/httpsec/cookie.go
test_paths:
  - internal/tools/analyze/handler/securityaudit/handler_test.go
  - internal/tools/analyze/handler/handlers_coverage_test.go
  - internal/tools/analyze/handler/toolanalyze_test.go
  - internal/analysis/security/diff/diff_test.go
  - internal/analysis/security/diff/compare_test.go
  - internal/analysis/security/diff/tool_test.go
  - internal/analysis/security/diff/helpers_test.go
  - internal/analysis/security/policy/policy_test.go
  - internal/analysis/security/policy/boundary_test.go
  - internal/analysis/security/policy/config_path_test.go
  - internal/analysis/security/scan/scan_test.go
  - internal/analysis/security/scan/scan_sensitive_data_test.go
  - internal/analysis/security/scan/scan_transport_policy_test.go
  - internal/analysis/security/scan/unit_test.go
  - internal/analysis/security/scan/coverage_test.go
  - internal/analysis/security/scan/coverage_part2_test.go
  - internal/analysis/security/scan/network_wiring_test.go
  - internal/analysis/security/csp/csp_test.go
  - internal/analysis/security/csp/csp_store_test.go
  - internal/analysis/security/csp/csp_tooling_test.go
  - internal/analysis/security/csp/csp_helpers_test.go
  - internal/analysis/security/csp/boundary_test.go
  - internal/analysis/security/sri/sri_test.go
  - internal/analysis/security/sri/helpers_test.go
  - internal/analysis/security/netflag/netflag_test.go
  - internal/analysis/security/netflag/flag_test.go
  - internal/analysis/security/netflag/boundary_test.go
  - internal/analysis/security/netflag/detectors_unit_test.go
  - internal/analysis/security/httpsec/url_test.go
  - internal/analysis/security/httpsec/cookie_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Security Hardening

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: security config
- Location: `docs/features/feature/security-hardening`

Security scanning consumes the canonical `internal/platform/types.LogEntry` contract
directly, without a scan-package compatibility alias.
Generate-time CSP and SRI handlers receive their capture dependency through the
explicit generate composition bundle, not through ToolHandler forwarding methods.
Response redaction is owned by the MCP transport configuration and applied after
tool execution. Tool implementations do not expose a redaction-engine getter.
Security policy configuration likewise resolves only through the canonical
state path and does not fall back to historical config directories.
Analyze-time security handlers and their summary projections share one
change-coupled owner, and the package enforces its ten-file boundary in tests.
Process-local security, telemetry, and action-jitter configuration handlers
share `runtime_modes.go`; these toggles use the same dependency bundle and
change together with runtime configuration policy.
Suspicious-network findings are owned by the leaf `netflag` package. Its
canonical snake-case `Flag` contract no longer imports or leaks the broad
capture package into security detection.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_SECURITY_HARDENING_001
- FEATURE_SECURITY_HARDENING_002
- FEATURE_SECURITY_HARDENING_003

## Code and Tests

Security scan, diff, and SRI tests use `internal/platform/types.NetworkBody` directly;
the former test-only aliases have been removed.

The aggregate scanner has three change-coupled production owners:
`scan.go` owns contracts and orchestration, `checks_policy.go` owns browser
transport/network/response policy, and `credentials.go` owns credential and
PII detection plus redaction. Invalid handler JSON is rejected explicitly, the
default check registry is constructed per scan, and a package regression test
enforces the ten-file boundary.

`internal/analysis/security` is a namespace of focused subpackages, not a package of its own.
The dependency direction is one-way — `httpsec`, `netflag`, `policy` and `sri` import
no sibling; `csp` imports `policy`; `scan` imports `httpsec` and `netflag`; `diff`
imports `httpsec`.

| Subpackage | Responsibility |
| --- | --- |
| `scan/` | Aggregate security audit — credentials, PII, headers, cookies, transport, auth, network |
| `csp/` | CSP generation from accumulated origin observations |
| `sri/` | Subresource Integrity hash generation for third-party scripts/styles |
| `diff/` | Named posture snapshots and regression/improvement comparison |
| `policy/` | MCP-mode trust boundary, manual-only config guards, audit trail |
| `netflag/` | Suspicious-origin detection (abusive TLDs, ports, IP origins, typosquatting, mixed content) |
| `httpsec/` | Shared URL classification and Set-Cookie parsing (leaf; imported by `scan` and `diff`) |

- `internal/analysis/security/policy/policy.go` — manual-only security config mutation guards (`AddToWhitelist`, `SetMinSeverity`, `ClearWhitelist`) with explicit human-review guidance and in-memory audit events.
- `internal/analysis/security/policy/mode.go` — MCP-mode and interactive-terminal gating flags.
- `internal/analysis/security/policy/audit.go` — session-scoped in-memory audit trail for security config actions/attempts.
- `internal/analysis/security/policy/policy_test.go` — manual-only policy and audit-event behavior.
- `internal/analysis/security/policy/boundary_test.go` — LLM trust boundary: MCP mode detection and blocked config mutations.
- `internal/analysis/security/csp/csp_test.go` — generated CSP policy and directive behavior.
- `internal/analysis/security/csp/csp_store_test.go` — origin observation accumulation and timestamps.
- CSP observation time comes from the generator's private clock boundary, so
  timestamp ordering is tested with exact instants instead of elapsed sleeps.
- `internal/analysis/security/csp/csp_tooling_test.go` — CSP tool parameter handling and dispatch.
- `internal/analysis/security/csp/csp_helpers_test.go` — resource classification and URL extraction.
- `internal/analysis/security/csp/boundary_test.go` — session-only whitelist overrides are applied, warned about, audited, and never persisted.
- CSP package documentation and its response audit contract live with the
  canonical models in `types.go`. Store eviction/page-listing branches live
  with store tests, while directive mapping branches live with helper tests;
  an executable package boundary keeps the owner at ten files.
- `internal/analysis/security/diff/diff_test.go` — snapshot lifecycle and retention with shared setup helpers.
- Security-diff retention owns a private clock boundary. Snapshot timestamps,
  ages, TTL checks, and current comparisons share that source; tests advance a
  fixed clock and never sleep to manufacture ordering or expiration.
- `internal/analysis/security/diff/compare_test.go` — regression/improvement comparison and extraction.
- `internal/analysis/security/diff/tool_test.go` — tool dispatch and summary behavior.
- Security-diff summary construction lives with comparison orchestration in
  `compare.go`; snapshot age rendering calls the canonical shared duration
  formatter directly. The former one-function summary/helper owner and its
  duplicate formatter tests were deleted, keeping the package at ten files.
- `internal/analysis/security/scan/scan_test.go` — scanner orchestration, filtering, serialization, and fuzz safety.
- `internal/analysis/security/scan/scan_sensitive_data_test.go` — credential, PII, and evidence-redaction findings.
- `internal/analysis/security/scan/scan_transport_policy_test.go` — headers, cookies, and transport-policy findings.
