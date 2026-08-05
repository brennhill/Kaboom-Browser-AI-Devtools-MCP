---
doc_type: feature_index
feature_id: feature-redaction-patterns
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/capture/logstore/store.go
  - internal/mcp/types.go
  - internal/redaction/redaction.go
  - internal/redaction/redaction_engine.go
  - internal/redaction/redaction_map.go
  - internal/security/scan/credentials.go
  - src/background/runtime-state/log-queue.ts
  - Makefile
test_paths:
  - internal/capture/logstore/diagnostic_test.go
  - internal/capture/logstore/store_test.go
  - internal/redaction/redaction_test.go
  - internal/redaction/redaction_engine_test.go
  - internal/redaction/redaction_fuzz_test.go
  - internal/redaction/redaction_property_test.go
  - internal/redaction/redaction_unit_test.go
  - internal/redaction/race_disabled_test.go
  - internal/redaction/race_enabled_test.go
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

The package is organized by change coupling rather than primitive size. The
engine owner contains configuration, compiled types, built-in patterns, string
redaction, and Luhn validation; the structured owner contains sensitive-key
classification and recursive map traversal. Behavior/configuration,
engine/edge/performance, structured-wire, property/fuzz, and race-build tests
live with their corresponding concerns. The package contains exactly ten files
and every file remains below 800 lines.

Security-audit credential and PII patterns are separately owned together in
`internal/security/scan/credentials.go`; they share the audit scanner's bounded
input and evidence-redaction policy rather than the MCP response-redaction
engine.

Capture diagnostics apply the canonical redaction engine at the focused
`internal/capture/logstore` ingestion boundary. The owner recursively redacts
structured extension data and daemon HTTP fields before retention, returns
detached snapshots, and exposes no legacy root-package store aliases.

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
Built-in patterns use correctness-preserving literal and numeric prefilters;
fixed-format AWS, Bearer, and SSN candidates use allocation-bounded scanners.
Custom patterns remain unrestricted. This avoids repeatedly running impossible
regular expressions over large captured payloads while preserving the same
redaction results and the documented 5 KB/100 KB latency budgets.
Those wall-clock budgets run in the serial `make test-performance` lane rather
than competing with the eight-package unit shard. The normal short/race lanes
continue to run all deterministic redaction correctness and safety tests.
