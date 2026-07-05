---
doc_type: qa-plan
feature_id: feature-idl-migration
status: draft
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# IDL Migration QA Plan

## Required Scenarios
1. Generated wire outputs are deterministic across repeated runs.
2. Drift check fails when generated artifacts are stale.
3. MCP `tools/list` output remains schema-compatible after migration.
4. Existing schema invariant tests continue passing.

## Automated Targets
- Generator check mode (`--check`)
- `internal/schema/invariants_test.go`
