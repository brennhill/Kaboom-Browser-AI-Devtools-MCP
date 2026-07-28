---
doc_type: feature_index
feature_id: feature-enterprise-audit
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/session/runtime_reader.go
  - cmd/browser-agent/tools_core.go
  - internal/audit/recorder.go
  - internal/analysis/thirdparty/audit.go
  - internal/analysis/thirdparty/entries.go
  - internal/analysis/thirdparty/origins.go
  - internal/analysis/thirdparty/reputation.go
  - internal/analysis/thirdparty/summary.go
  - internal/audit/audit_trail.go
test_paths:
  - internal/audit/audit_trail_test.go
  - internal/audit/no_facade_test.go
  - cmd/browser-agent/tools_configure_audit_test.go
  - cmd/browser-agent/tools_configure_wave_abc_tdd_test.go
  - internal/session/runtime_reader_test.go
  - internal/analysis/thirdparty/audit_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Enterprise Audit

## TL;DR

- Status: shipped
- Tool: configure, observe
- Mode/Action: audit_log, security_audit
- Location: `docs/features/feature/enterprise-audit`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ENTERPRISE_AUDIT_001
- FEATURE_ENTERPRISE_AUDIT_002
- FEATURE_ENTERPRISE_AUDIT_003

## Code and Tests

Third-party audit tests consume `internal/types.NetworkBody` directly, keeping
the test contract aligned with production ownership.

`internal/audit/recorder.go` owns tool-call filtering, error interpretation,
per-client audit sessions, and session reset. `ToolHandler` retains only the
canonical recorder and trail references needed by dispatch and configure.
Callers use the canonical `AuditEntry`, `AuditTrail`, `AuditFilter`, and
`AuditConfig` contracts directly; the former alias-only type facade is deleted.
The runtime session projection receives the canonical performance-entry reader
as an injected function, so its capture interface no longer requires a
performance compatibility method.
