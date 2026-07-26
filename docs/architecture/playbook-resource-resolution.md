---
doc_type: flow_map
flow_id: playbook-resource-resolution
status: active
last_reviewed: 2026-03-28
owners:
  - Brenn
entrypoints:
  - cmd/browser-agent/handler_dispatch.go:handleResourcesRead
  - cmd/browser-agent/bridge_fastpath.go:handleFastPath
code_paths:
  - cmd/browser-agent/mcp_resources.go
  - cmd/browser-agent/playbooks.go
  - cmd/browser-agent/playbooks_performance.go
  - cmd/browser-agent/playbooks_accessibility.go
  - cmd/browser-agent/playbooks_security.go
  - cmd/browser-agent/playbooks_automation.go
  - cmd/browser-agent/playbooks_resolver.go
test_paths:
  - cmd/browser-agent/handler_unit_test.go
  - cmd/browser-agent/bridge_fastpath_unit_test.go
  - cmd/browser-agent/bridge_faststart_extended_test.go
  - cmd/browser-agent/playbooks_content_test.go
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Playbook Resource Resolution

## Scope

Covers MCP `resources/list`, `resources/templates/list`, and `resources/read` resolution for capability index, guide, quickstart, and playbooks.

## Entrypoints

- `handleResourcesRead` serves canonical resource content in normal handler path.
- `handleFastPath` serves `resources/read` immediately in bridge fast-path mode.

## Primary Flow

1. Client requests resource templates and discovers `kaboom://playbook/{capability}/{level}`.
2. Client calls `resources/read` with a URI.
3. `resolveResourceContent` routes URI family (`capabilities`, `guide`, `quickstart`, `playbook`, `demo`).
4. Playbook URIs are normalized via `resolvePlaybookKey` and capability aliases.
5. Canonical URI and markdown content are returned in MCP resource result.
6. Fast-path requests record telemetry success/failure counters in bridge metrics.

## Error and Recovery Paths

- Unknown resource URI returns MCP not-found error (`-32002`).
- Invalid playbook capability/shape resolves to not-found instead of ambiguous fallback.
- Fast-path path emits telemetry for both success and failure to support diagnostics.

## State and Contracts

- `playbooks` map is built from capability-specific sets with duplicate-key panic guard.
- Canonical capability aliases must remain stable for compatibility (`security_audit` -> `security`).
- Resource payloads are text/markdown and intended for token-efficient LLM retrieval.

## Code Paths

- `cmd/browser-agent/mcp_resources.go`
- `cmd/browser-agent/playbooks.go`
- `cmd/browser-agent/playbooks_performance.go`
- `cmd/browser-agent/playbooks_accessibility.go`
- `cmd/browser-agent/playbooks_security.go`
- `cmd/browser-agent/playbooks_automation.go`
- `cmd/browser-agent/playbooks_resolver.go`

## Test Paths

- `cmd/browser-agent/handler_unit_test.go`
- `cmd/browser-agent/bridge_fastpath_unit_test.go`
- `cmd/browser-agent/bridge_faststart_extended_test.go`
- `cmd/browser-agent/playbooks_content_test.go`

## Edit Guardrails

- Keep URI normalization in `playbooks_resolver.go`; avoid scattered alias logic.
- Keep playbook content modular by capability file; do not reintroduce monolithic map blobs.
- Preserve canonical URI behavior and test coverage before adding new capabilities.
