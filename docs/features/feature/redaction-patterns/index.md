---
doc_type: feature_index
feature_id: feature-redaction-patterns
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-03
code_paths:
  - internal/capture/extension_logs.go
  - internal/mcp/types.go
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
  - internal/redaction/redaction_coverage_test.go
  - internal/redaction/redaction_fuzz_test.go
  - internal/redaction/redaction_map_test.go
  - internal/redaction/redaction_property_test.go
  - internal/redaction/redaction_unit_test.go
  - internal/security/scan/unit_test.go
  - internal/security/scan/coverage_part2_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-03
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

The redaction package consumes the canonical `internal/mcp` result and content
block wire types directly. It owns only `RedactionEngine`, `RedactionConfig`,
and `RedactionPattern`; duplicate or alias compatibility types are prohibited
and guarded by a package regression test.
