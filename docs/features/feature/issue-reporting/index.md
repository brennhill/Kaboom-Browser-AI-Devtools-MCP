---
doc_type: feature_index
feature_id: feature-issue-reporting
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/tools_configure_report_issue.go
  - internal/schema/configure/properties_runtime.go
  - internal/issuereport/types.go
  - internal/issuereport/templates.go
  - internal/issuereport/sanitize.go
  - internal/issuereport/submit.go
test_paths:
  - cmd/browser-agent/tools_configure_report_issue_test.go
  - internal/issuereport/templates_test.go
  - internal/issuereport/sanitize_test.go
  - internal/issuereport/submit_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Issue Reporting

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | shipped                                 |
| **Tool**      | configure                               |
| **Mode**      | `what="report_issue"`                   |
| **Schema**    | `internal/schema/configure/properties_runtime.go` |

## Specs

- [Product Spec](./product-spec.md)
- [Tech Spec](./tech-spec.md)
- [QA Plan](./qa-plan.md)

## Canonical Note

Opt-in issue reporting via `configure(what="report_issue")` — collects sanitized diagnostics and files GitHub issues via `gh` CLI, with explicit user approval before any data leaves the machine.
