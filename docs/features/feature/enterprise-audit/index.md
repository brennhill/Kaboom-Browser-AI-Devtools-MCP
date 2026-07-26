---
doc_type: feature_index
feature_id: feature-enterprise-audit
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/tools_session_audit_reader.go
  - cmd/browser-agent/tools_session_audit_recording.go
  - internal/analysis/thirdparty/audit.go
  - internal/analysis/thirdparty/entries.go
  - internal/analysis/thirdparty/origins.go
  - internal/analysis/thirdparty/reputation.go
  - internal/analysis/thirdparty/summary.go
  - internal/audit/audit_trail.go
test_paths:
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

Add concrete implementation and test links here as this feature evolves.
