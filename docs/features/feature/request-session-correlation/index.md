---
doc_type: feature_index
feature_id: feature-request-session-correlation
status: active
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - cmd/browser-agent/client_registry_adapter.go
  - cmd/browser-agent/main_connection_mcp_bootstrap.go
  - cmd/browser-agent/server_routes_clients.go
  - internal/capture/interfaces.go
  - internal/capture/client_registry_setter.go
  - internal/session/client_registry.go
  - internal/session/types.go
  - internal/session/verify_actions.go
test_paths:
  - cmd/browser-agent/server_routes_clients_test.go
  - internal/session/client_registry_test.go
  - internal/session/verify_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Request Session Correlation

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/request-session-correlation`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_REQUEST_SESSION_CORRELATION_001
- FEATURE_REQUEST_SESSION_CORRELATION_002
- FEATURE_REQUEST_SESSION_CORRELATION_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
