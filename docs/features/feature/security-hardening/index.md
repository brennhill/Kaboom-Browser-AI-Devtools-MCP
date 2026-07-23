---
doc_type: feature_index
feature_id: feature-security-hardening
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-04-19
code_paths:
  - cmd/browser-agent/internal/toolanalyze/security.go
  - cmd/browser-agent/internal/toolanalyze/security_summaries.go
  - cmd/browser-agent/internal/toolgenerate/artifacts_security_impl.go
  - cmd/browser-agent/internal/toolconfigure/security_mode.go
  - internal/security/security_scan.go
  - internal/security/sri_tooling.go
test_paths:
  - cmd/browser-agent/tools_analyze_security_test.go
  - cmd/browser-agent/tools_generate_csp_test.go
  - cmd/browser-agent/tools_configure_security_mode_test.go
  - internal/security/sri_test.go
last_verified_version: 0.8.5
last_verified_date: 2026-04-19
---

# Security Hardening

## TL;DR

- Status: shipped
- Tool: analyze, generate, configure
- Mode/Action: security_audit, third_party_audit, csp, sri, security_mode
- Location: `docs/features/feature/security-hardening`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_SECURITY_HARDENING_001
- FEATURE_SECURITY_HARDENING_002
- FEATURE_SECURITY_HARDENING_003

## Code and Tests

- `cmd/browser-agent/internal/toolanalyze/security.go` — live `analyze(what:"security_audit"|"third_party_audit")` entrypoints.
- `cmd/browser-agent/internal/toolgenerate/artifacts_security_impl.go` — live `generate(what:"csp"|"sri")` handlers.
- `cmd/browser-agent/internal/toolconfigure/security_mode.go` — live `configure(what:"security_mode")` altered-environment toggle.
- `cmd/browser-agent/tools_analyze_security_test.go` — security audit and third-party audit summary coverage.
- `cmd/browser-agent/tools_generate_csp_test.go` — CSP generation handler coverage.
- `cmd/browser-agent/tools_configure_security_mode_test.go` — security mode toggle and confirmation coverage.
