---
doc_type: feature_index
feature_id: feature-environment-manipulation
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-02
code_paths:
  - cmd/browser-agent/internal/cli/cli_tool_parsers_generate_configure.go
  - cmd/browser-agent/internal/playbooks/playbooks_guides.go
  - cmd/browser-agent/internal/toolconfigure/qafixture/handler.go
  - cmd/browser-agent/internal/toolinteract/action_owners.go
  - cmd/browser-agent/internal/toolinteract/interact_storage.go
  - cmd/browser-agent/tools_configure.go
  - internal/qafixture/fixture.go
  - internal/schema/configure/properties_fixture.go
  - internal/tools/configure/capabilities/modespecs_configure.go
test_paths:
  - cmd/browser-agent/golden_test.go
  - cmd/browser-agent/testdata/mcp-tools-list.golden.json
  - cmd/browser-agent/tools_configure_handler_test.go
  - cmd/browser-agent/internal/toolinteract/interact_storage_test.go
  - cmd/browser-agent/internal/toolconfigure/qafixture/handler_test.go
  - internal/qafixture/fixture_test.go
  - internal/schema/configure/schema_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Environment Manipulation

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/environment-manipulation`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ENVIRONMENT_MANIPULATION_001
- FEATURE_ENVIRONMENT_MANIPULATION_002
- FEATURE_ENVIRONMENT_MANIPULATION_003

## Code and Tests

- Storage and cookie mutation handlers share one canonical execution-target
  contract for tab, timeout, and JavaScript world selection.
- Characterization tests verify that every storage mutation preserves that
  contract through the queued extension command.
- `configure(what="qa_fixture", fixture_action="validate")` now validates the
  canonical version-1 QA environment document without mutating the browser or
  echoing cookies, storage, flags, or seed values. The contract bounds setup
  time and state size, rejects unknown fields and unsupported capabilities,
  and only advertises validation until transactional apply and restore ship.
