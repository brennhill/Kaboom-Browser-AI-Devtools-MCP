---
doc_type: feature_index
feature_id: feature-redaction-patterns
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/capture/extension_logs.go
  - internal/redaction/redaction.go
  - internal/redaction/redaction_engine.go
  - internal/redaction/redaction_types.go
  - internal/security/scan/credentials.go
  - internal/security/scan/credentials_patterns.go
test_paths:
  - internal/capture/http_debug_redaction_test.go
  - internal/redaction/no_facade_test.go
  - internal/redaction/redaction_test.go
  - internal/redaction/redaction_config_test.go
  - internal/redaction/redaction_engine_test.go
  - internal/redaction/redaction_map_test.go
  - internal/security/scan/unit_test.go
  - internal/security/scan/coverage_part2_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Redaction Patterns

## TL;DR

- Status: shipped
- Tool: configure
- Mode/Action: data masking
- Location: `docs/features/feature/redaction-patterns`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_REDACTION_PATTERNS_001
- FEATURE_REDACTION_PATTERNS_002
- FEATURE_REDACTION_PATTERNS_003

## Code and Tests

The redaction package exposes its canonical `RedactionEngine`,
`RedactionConfig`, and `RedactionPattern` contracts directly. Alias-only type
facades are prohibited and guarded by a package regression test.
