---
doc_type: feature_index
feature_id: feature-issue-reporting
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - internal/composition/toolruntime/tools_core.go
  - internal/composition/toolruntime/tools_configure.go
  - internal/tools/configure/handler/dispatcher.go
  - internal/tools/issuereport/handler.go
  - internal/tools/schema/configure/properties_core.go
  - internal/tools/schema/configure/properties_runtime.go
  - internal/tools/configure/capabilities/modespecs_configure.go
  - internal/tools/issuereport/types.go
  - internal/tools/issuereport/templates.go
  - internal/tools/issuereport/sanitize.go
  - internal/tools/issuereport/submit.go
test_paths:
  - cmd/browser-agent/composition_test.go
  - internal/tools/configure/handler/dispatcher_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/tools/issuereport/handler_test.go
  - internal/tools/issuereport/templates_test.go
  - internal/tools/issuereport/sanitize_test.go
  - internal/tools/issuereport/submit_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Issue Reporting

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | shipped                                 |
| **Tool**      | configure                               |
| **Mode**      | `what="report_issue"`                   |
| **Schema**    | `internal/tools/schema/configure/properties_runtime.go` |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)
- [QA Plan](./qa-plan.md)

## Canonical Note

Opt-in issue reporting via `configure(what="report_issue")` — collects sanitized diagnostics and files GitHub issues via `gh` CLI. `list_templates` and `preview` are local. `operation="submit"` is the daemon's only outbound path for session-derived text: it publishes a **public** issue on `brennhill/Kaboom-Browser-AI-Devtools-MCP` under the local `gh` identity, and is refused unless the same call carries `confirm: true` — the explicit approval that makes this an exception to Rule 7.

Collection, sanitization, and submission are composed once as explicit
issue-report dependencies. The handler no longer requires a host interface,
and ToolHandler exposes no issue-report forwarding methods.
