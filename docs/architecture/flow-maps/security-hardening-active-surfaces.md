---
doc_type: flow_map
flow_id: security-hardening-active-surfaces
status: active
last_reviewed: 2026-04-19
owners:
  - Brenn
entrypoints:
  - cmd/browser-agent/internal/toolanalyze/security.go
  - cmd/browser-agent/internal/toolgenerate/artifacts_security_impl.go
  - cmd/browser-agent/internal/toolconfigure/security_mode.go
  - docs/features/feature/security-hardening/index.md
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
  - tests/docs/security-hardening-doc-contract.test.js
last_verified_version: 0.8.5
last_verified_date: 2026-04-19
---

# Security Hardening Active Surfaces

## Scope

Covers the live user-facing security surfaces: security audits under `analyze`, CSP/SRI generation under `generate`, and altered-environment debugging under `configure(what:"security_mode")`.

Related docs:

- `docs/features/feature/security-hardening/index.md`
- `docs/features/feature/security-hardening/product-spec.md`
- `docs/features/feature/security-hardening/tech-spec.md`
- `docs/features/feature/security-hardening/qa-plan.md`

## Entrypoints

1. `analyze(what:"security_audit")` and `analyze(what:"third_party_audit")`
2. `generate(what:"csp")` and `generate(what:"sri")`
3. `configure(what:"security_mode")`

## Primary Flow

1. `toolanalyze.HandleSecurityAudit` and `HandleThirdPartyAudit` collect captured network, console, and page context, then run the security scanner.
2. `toolgenerate.HandleGenerateCSP` turns captured network bodies into a CSP policy string and directives.
3. `toolgenerate.HandleGenerateSRI` reuses captured network bodies to generate SRI hashes.
4. `toolconfigure.HandleSecurityMode` toggles `normal` versus `insecure_proxy` for altered-environment debugging when CSP rewriting is explicitly confirmed.

## Error and Recovery Paths

1. If no network bodies are available, `generate(csp)` and `generate(sri)` return `status: unavailable` with a capture hint instead of failing blindly.
2. Invalid `security_mode` values or missing `confirm=true` for `insecure_proxy` return parameter errors.
3. If security audit inputs are incomplete, the analyze handlers still return structured results through the shared security summary path.

## State and Contracts

1. The live feature surface is `security_audit`, `third_party_audit`, `csp`, `sri`, and `security_mode`.
2. The removed `internal/security/security_config_*` internals are not part of the current user-facing contract.
3. Feature docs must describe the active surfaces above and keep code/test anchors aligned with the files that implement them.

## Code Paths

- `cmd/browser-agent/internal/toolanalyze/security.go`
- `cmd/browser-agent/internal/toolanalyze/security_summaries.go`
- `cmd/browser-agent/internal/toolgenerate/artifacts_security_impl.go`
- `cmd/browser-agent/internal/toolconfigure/security_mode.go`
- `internal/security/security_scan.go`
- `internal/security/sri_tooling.go`

## Test Paths

- `cmd/browser-agent/tools_analyze_security_test.go`
- `cmd/browser-agent/tools_generate_csp_test.go`
- `cmd/browser-agent/tools_configure_security_mode_test.go`
- `internal/security/sri_test.go`
- `tests/docs/security-hardening-doc-contract.test.js`

## Edit Guardrails

1. Do not describe `security_config_*` files as shipped feature surfaces unless those files and the corresponding behavior are restored.
2. Keep the feature index, feature-local index, and this canonical flow map aligned when security surfaces move between `analyze`, `generate`, and `configure`.
3. If new user-facing security modes are added, update the feature row in `docs/features/feature-index.md` and the feature-local docs in the same change.
