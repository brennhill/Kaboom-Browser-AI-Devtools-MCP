---
doc_type: tech-spec
feature_id: feature-idl-migration
status: draft
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# IDL Migration Tech Spec

## Current State
- Wire TS files are generated from Go structs (`scripts/generate-wire-types.js`).
- Tool schemas are still hand-authored Go `map[string]any` structures.

## Target Direction
- Introduce schema-first IDL files for wire + tool contracts.
- Generate:
  - Go wire structs / schema bindings
  - TS wire interfaces
  - schema constants for MCP tool definitions

## Migration Constraints
- Preserve byte-equivalent MCP schema output where possible.
- Keep generators idempotent and checkable in CI.
- Avoid introducing runtime-only schema mutation paths.
