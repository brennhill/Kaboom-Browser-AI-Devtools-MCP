---
doc_type: architecture_index
status: active
last_reviewed: 2026-03-05
owners: []
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Architecture Documentation

## Purpose

This folder contains the architecture contract used to make safe edits.

## Start Here (Safe Edits)

1. [SAFE_EDIT_CHECKLIST.md](../boundaries/SAFE_EDIT_CHECKLIST.md)
2. [EDIT_REQUEST_TEMPLATE.md](../workflows/EDIT_REQUEST_TEMPLATE.md)
3. [system-overview.md](system-overview.md)
4. [module-map.md](../boundaries/module-map.md)
5. [invariants.md](../boundaries/invariants.md)
6. [interfaces.md](../boundaries/interfaces.md)
7. [PR Template](../../../.github/pull_request_template.md)

## Canonical Technical References

- [Server Architecture](../../core/architecture/server-architecture.md)
- [Extension Architecture](../../core/architecture/extension-architecture.md)
- [Extension Message Protocol](../../core/protocol/extension-message-protocol.md)
- [MCP Command Option Matrix](../../core/protocol/mcp-command-option-matrix.md)
- [Code Index](../../core/product/code-index.md)
- ADRs in this folder (`ADR-00x-*`)

## Comments in Code Files

Code comments and file headers explain local intent and navigation, but they are not architecture authority.
When behavior, boundaries, or compatibility rules change, update docs in this folder and tests in the same change.
