---
doc_type: product-spec
feature_id: feature-mcp-persistent-server
status: shipped
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# MCP Persistent Server Product Spec

## Purpose
Keep a daemon process alive independently of any single MCP stdio session so multiple clients can reconnect without reinitializing full server state.

## Requirements
- `PERSISTENT_SERVER_PROD_001`: daemon survives client disconnect/reconnect cycles.
- `PERSISTENT_SERVER_PROD_002`: process identity checks prevent stale PID reuse hazards.
- `PERSISTENT_SERVER_PROD_003`: restart/health controls remain available while persistent mode is active.
