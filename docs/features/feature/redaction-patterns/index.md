---
doc_type: feature_index
feature_id: feature-redaction-patterns
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-04
code_paths:
  - internal/capture/extension_logs.go
  - internal/mcp/types.go
  - internal/redaction/redaction.go
  - internal/redaction/redaction_engine.go
  - internal/redaction/redaction_builtin_patterns.go
  - internal/redaction/redaction_map.go
  - internal/redaction/redaction_types.go
  - internal/security/scan/credentials.go
  - internal/security/scan/credentials_patterns.go
  - src/background/runtime-state/log-queue.ts
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
  - internal/incident/store_test.go
  - internal/security/scan/unit_test.go
  - internal/security/scan/coverage_part2_test.go
  - tests/extension/reliability/diagnostic-log-queue.test.js
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
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

Deterministic property suites exercise canonical MCP text and image blocks,
nested metadata, errors, diagnostics, malformed envelopes, and extension runtime
diagnostics. Structured JSON text and metadata are recursively key-redacted,
including every scalar leaf beneath sensitive container keys; canonical image
fields and nonsensitive correlation metadata round-trip unchanged. Fixed seeds
cover malformed-image credentials, while generated Go and TypeScript cases
prove raw secret markers never reach MCP output, persisted diagnostics, or
Doctor input.

Operational evidence also applies case-insensitive Bearer and Basic patterns,
accepts horizontal whitespace in authorization schemes, and redacts complete
quoted structured credential values so whitespace cannot expose a suffix.
