---
doc_type: tech-spec
feature_id: feature-issue-reporting
status: shipped
owners: []
last_reviewed: 2026-07-05
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Issue Reporting Tech Spec

## Dispatcher

- Entry: `toolConfigureReportIssue` in `cmd/browser-agent/tools_configure_report_issue.go`
- Registry: `configureHandlers["report_issue"]` in `tools_configure_registry.go`
- Schema: `report_issue` in `internal/schema/configure/properties_runtime.go`
- Mode spec: `report_issue` in `internal/tools/configure/capabilities/modespecs_configure.go`

## Package: `internal/issuereport/`

| File | Purpose |
|------|---------|
| `types.go` | Core types: IssueTemplate, IssueReport, DiagnosticData, SubmitResult |
| `templates.go` | 5 hardcoded templates + GetTemplate lookup |
| `sanitize.go` | Sanitizer wrapping Redactor interface |
| `submit.go` | gh CLI submission with manual fallback |

## Diagnostics Collection

`collectIssueReport()` gathers:
1. Server version from `version` global
2. Platform from `runtime.GOOS/GOARCH/Version()`
3. Uptime and audit stats from `healthMetrics`
4. Extension connectivity from `capture.GetHealthSnapshot()`
5. Buffer counts from capture and server

## Redaction

`sanitizeIssueReport()` creates an `issuereport.Sanitizer` backed by the handler's `RedactionEngine` interface. Redacts:
- Title string
- UserContext string
- Extension source string

The `Redact(string) string` method was added to the `RedactionEngine` interface for this feature.

## Submission Flow

1. `SubmitViaGH()` checks `exec.LookPath("gh")`
2. If found: `gh issue create --repo {target} --title {title} --body {body} --label {labels}`
3. If not found: returns `{status: "manual", formatted_body: "...", repo_url: "..."}` so the LLM can file directly
4. `CommandRunner` interface enables test injection

## Code Anchors

- `cmd/browser-agent/tools_configure_report_issue.go`
- `cmd/browser-agent/tools_configure_report_issue_test.go`
- `internal/issuereport/types.go`
- `internal/issuereport/templates.go`
- `internal/issuereport/sanitize.go`
- `internal/issuereport/submit.go`
- `internal/issuereport/templates_test.go`
- `internal/issuereport/sanitize_test.go`
- `internal/issuereport/submit_test.go`
