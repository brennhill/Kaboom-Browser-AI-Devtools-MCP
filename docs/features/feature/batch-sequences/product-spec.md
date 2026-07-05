---
doc_type: product-spec
feature_id: feature-batch-sequences
status: proposed
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Batch Sequences Product Spec

## Purpose
Enable deterministic multi-step browser workflows in one MCP call, with optional saved/replayable named sequences.

## User-Facing Surface
- `interact({what:"batch", steps:[...]})`
- `configure({what:"save_sequence"|"replay_sequence"|"list_sequences"|"get_sequence"|"delete_sequence"})`

## Requirements
- `BATCH_PROD_001`: Steps execute in order and return per-step status + duration.
- `BATCH_PROD_002`: `continue_on_error` controls whether failures halt execution.
- `BATCH_PROD_003`: `stop_after_step` supports bounded dry-run/partial replay.
- `BATCH_PROD_004`: Saved sequences are namespaced and replayable with deterministic ordering.
- `BATCH_PROD_005`: Batch and replay behavior stays contract-compatible across `interact` and `configure` surfaces.
