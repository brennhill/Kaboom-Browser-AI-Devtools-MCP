---
doc_type: product-spec
feature_id: feature-idl-migration
status: draft
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# IDL Migration Product Spec

## Purpose
Reduce schema drift and manual contract maintenance between daemon-side Go types and extension/client TypeScript types.

## Requirements
- `IDL_PROD_001`: one canonical contract source for wire/tool schemas.
- `IDL_PROD_002`: generated Go and TS artifacts must be deterministic.
- `IDL_PROD_003`: CI drift checks must fail on stale generated output.
- `IDL_PROD_004`: migration should preserve current external MCP schema behavior.
