---
doc_type: feature_index
feature_id: feature-security-hardening
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - internal/security/security_diff.go
  - internal/security/security_diff_compare.go
  - internal/security/security_diff_snapshot.go
  - internal/security/security_diff_helpers_headers_cookies.go
  - internal/security/security_diff_helpers_maps_urls.go
  - internal/security/security_diff_helpers_summary.go
  - internal/security/security_diff_tool.go
  - internal/security/security_config_policy.go
  - internal/security/security_config_mode.go
  - internal/security/security_config_audit.go
test_paths:
  - internal/security/security_diff_test.go
  - internal/security/security_config_unit_test.go
  - internal/security/security_boundary_test.go
  - internal/security/security_config_path_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Security Hardening

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: security config
- Location: `docs/features/feature/security-hardening`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_SECURITY_HARDENING_001
- FEATURE_SECURITY_HARDENING_002
- FEATURE_SECURITY_HARDENING_003

## Code and Tests

- `internal/security/security_config_policy.go` — manual-only security config mutation guards (`AddToWhitelist`, `SetMinSeverity`, `ClearWhitelist`) with explicit human-review guidance and in-memory audit events.
- `internal/security/security_config_mode.go` — MCP-mode and interactive-terminal gating flags.
- `internal/security/security_config_audit.go` — session-scoped in-memory audit trail for security config actions/attempts.
- `internal/security/security_config_unit_test.go` — manual-only policy and audit-event behavior.
- `internal/security/security_diff_test.go` — regression/improvement diff coverage with shared snapshot/compare test helpers for consistent setup.
