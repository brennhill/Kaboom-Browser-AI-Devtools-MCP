---
doc_type: feature_index
feature_id: feature-security-hardening
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - cmd/browser-agent/internal/toolgenerate/dispatcher.go
  - cmd/browser-agent/internal/toolgenerate/deps.go
  - cmd/browser-agent/internal/toolgenerate/artifacts_security_impl.go
  - cmd/browser-agent/internal/toolconfigure/security_mode.go
  - internal/mcp/response.go
  - internal/security/diff/types.go
  - internal/security/diff/compare.go
  - internal/security/diff/snapshot.go
  - internal/security/diff/helpers_headers_cookies.go
  - internal/security/diff/helpers_maps_urls.go
  - internal/security/diff/tool.go
  - internal/security/policy/policy.go
  - internal/security/policy/mode.go
  - internal/security/policy/audit.go
  - internal/security/scan/doc.go
  - internal/security/scan/scan.go
  - internal/security/scan/types.go
  - internal/types/wire_log.go
  - internal/security/scan/checks_cookies.go
  - internal/security/scan/checks_headers.go
  - internal/security/scan/checks_pii.go
  - internal/security/scan/checks_transport_auth.go
  - internal/security/scan/checks_network.go
  - internal/security/csp/doc.go
  - internal/security/csp/types.go
  - internal/security/csp/store.go
  - internal/security/csp/generate.go
  - internal/security/csp/helpers.go
  - internal/security/csp/tooling.go
  - internal/security/csp/audit.go
  - internal/security/sri/doc.go
  - internal/security/sri/types.go
  - internal/security/sri/generate.go
  - internal/security/sri/helpers.go
  - internal/security/sri/tooling.go
  - internal/security/netflag/netflag.go
  - internal/security/netflag/data.go
  - internal/security/netflag/detectors.go
  - internal/security/netflag/distance.go
  - internal/security/httpsec/url.go
  - internal/security/httpsec/cookie.go
test_paths:
  - internal/security/diff/diff_test.go
  - internal/security/diff/compare_test.go
  - internal/security/diff/tool_test.go
  - internal/security/diff/helpers_test.go
  - internal/security/policy/policy_test.go
  - internal/security/policy/boundary_test.go
  - internal/security/policy/config_path_test.go
  - internal/security/scan/scan_test.go
  - internal/security/scan/scan_sensitive_data_test.go
  - internal/security/scan/scan_transport_policy_test.go
  - internal/security/scan/unit_test.go
  - internal/security/scan/coverage_test.go
  - internal/security/scan/coverage_part2_test.go
  - internal/security/scan/network_wiring_test.go
  - internal/security/csp/csp_test.go
  - internal/security/csp/csp_store_test.go
  - internal/security/csp/csp_tooling_test.go
  - internal/security/csp/csp_helpers_test.go
  - internal/security/csp/boundary_test.go
  - internal/security/csp/coverage_test.go
  - internal/security/sri/sri_test.go
  - internal/security/sri/helpers_test.go
  - internal/security/netflag/netflag_test.go
  - internal/security/netflag/detectors_unit_test.go
  - internal/security/httpsec/url_test.go
  - internal/security/httpsec/cookie_test.go
  - cmd/browser-agent/lint_hardening_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Security Hardening

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: security config
- Location: `docs/features/feature/security-hardening`

Security scanning consumes the canonical `internal/types.LogEntry` contract
directly, without a scan-package compatibility alias.
Generate-time CSP and SRI handlers receive their capture dependency through the
explicit generate composition bundle, not through ToolHandler forwarding methods.
Response redaction is owned by the MCP transport configuration and applied after
tool execution. Tool implementations do not expose a redaction-engine getter.
Security policy configuration likewise resolves only through the canonical
state path and does not fall back to historical config directories.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_SECURITY_HARDENING_001
- FEATURE_SECURITY_HARDENING_002
- FEATURE_SECURITY_HARDENING_003

## Code and Tests

Security scan, diff, and SRI tests use `internal/types.NetworkBody` directly;
the former test-only aliases have been removed.

`internal/security` is a namespace of focused subpackages, not a package of its own.
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

- `internal/security/policy/policy.go` — manual-only security config mutation guards (`AddToWhitelist`, `SetMinSeverity`, `ClearWhitelist`) with explicit human-review guidance and in-memory audit events.
- `internal/security/policy/mode.go` — MCP-mode and interactive-terminal gating flags.
- `internal/security/policy/audit.go` — session-scoped in-memory audit trail for security config actions/attempts.
- `internal/security/policy/policy_test.go` — manual-only policy and audit-event behavior.
- `internal/security/policy/boundary_test.go` — LLM trust boundary: MCP mode detection and blocked config mutations.
- `internal/security/csp/csp_test.go` — generated CSP policy and directive behavior.
- `internal/security/csp/csp_store_test.go` — origin observation accumulation and timestamps.
- CSP observation time comes from the generator's private clock boundary, so
  timestamp ordering is tested with exact instants instead of elapsed sleeps.
- `internal/security/csp/csp_tooling_test.go` — CSP tool parameter handling and dispatch.
- `internal/security/csp/csp_helpers_test.go` — resource classification and URL extraction.
- `internal/security/csp/boundary_test.go` — session-only whitelist overrides are applied, warned about, audited, and never persisted.
- `internal/security/diff/diff_test.go` — snapshot lifecycle and retention with shared setup helpers.
- Security-diff retention owns a private clock boundary. Snapshot timestamps,
  ages, TTL checks, and current comparisons share that source; tests advance a
  fixed clock and never sleep to manufacture ordering or expiration.
- `internal/security/diff/compare_test.go` — regression/improvement comparison and extraction.
- `internal/security/diff/tool_test.go` — tool dispatch and summary behavior.
- Security-diff summary construction lives with comparison orchestration in
  `compare.go`; snapshot age rendering calls the canonical shared duration
  formatter directly. The former one-function summary/helper owner and its
  duplicate formatter tests were deleted, keeping the package at ten files.
- `internal/security/scan/scan_test.go` — scanner orchestration, filtering, serialization, and fuzz safety.
- `internal/security/scan/scan_sensitive_data_test.go` — credential, PII, and evidence-redaction findings.
- `internal/security/scan/scan_transport_policy_test.go` — headers, cookies, and transport-policy findings.
