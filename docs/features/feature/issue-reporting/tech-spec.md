---
doc_type: tech-spec
feature_id: feature-issue-reporting
status: shipped
owners: []
last_reviewed: 2026-09-05
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

- Entry: `issuereport.Handle` in `internal/issuereport/handler.go`
- Registry: the `report_issue` handler supplied to `internal/toolconfigure.Dispatcher` in `tools_configure.go`
- Schema: `report_issue` in `internal/schema/configure/properties_runtime.go`
- Mode spec: `report_issue` in `internal/tools/configure/capabilities/modespecs_configure.go`

## Package: `internal/issuereport/`

| File | Purpose |
|------|---------|
| `types.go` | Core types: IssueTemplate, IssueReport, DiagnosticData, SubmitResult |
| `handler.go` | Operation routing, validation, preview shaping, and submission orchestration |
| `templates.go` | 5 hardcoded templates + GetTemplate lookup |
| `sanitize.go` | Sanitizer wrapping Redactor interface |
| `submit.go` | gh CLI submission with manual fallback |

## Diagnostics Collection

`ToolHandler.CollectIssueReport()` gathers:
1. Server version from `version` global
2. Platform from `runtime.GOOS/GOARCH/Version()`
3. Uptime and audit stats from `healthMetrics`
4. Extension connectivity from `capture.GetHealthSnapshot()`
5. Buffer counts from capture and server

## Redaction

`ToolHandler.SanitizeIssueReport()` creates an `issuereport.Sanitizer` backed by the handler's `RedactionEngine` interface. Redacts:
- Title string
- UserContext string
- Extension source string

The `Redact(string) string` method was added to the `RedactionEngine` interface for this feature.

## Submission Flow

`operation=submit` is the only path in the daemon that sends session-derived text
off the machine, and it sends it to a public repository under the user's own
GitHub identity. It is gated accordingly.

1. `submit()` in `handler.go` refuses unless the call carries `confirm: true`, and
   returns `missing_param` naming `confirm`, the destination repo, and `preview`.
   The refusal happens before `Collect`, `Sanitize`, or `Submit` is reached, so an
   unconfirmed call performs no work and sends nothing.
2. `title` and `template` are validated next; an invalid payload cannot be pushed
   through by setting `confirm`.
3. `SubmitViaGH()` checks `exec.LookPath("gh")`
4. If found: `gh issue create --repo {target} --title {title} --body {body} --label {labels}`
5. If not found: returns `{status: "manual", formatted_body: "...", repo_url: "..."}` so the LLM can file directly
6. `CommandRunner` interface enables test injection

The outbound argv and body are pinned character-for-character by
`TestSubmitViaGHSendsExactlyTheDocumentedPayload`, so a new diagnostic field
cannot start leaving the machine without that test being edited.

## Capability Text Contract

`TestModeSpecDescribesWhatSubmitActuallyDoes` drives the real handler and the real
`describe_capabilities` output together. It fails if the mode spec's `Returns`
claims the mode transmits nothing, if it stops naming `confirm` or the target
repo, if the `confirm` gate stops being enforced, or if `confirm` is dropped from
the mode's advertised params. This is the regression guard for the mode spec once
having read "Text only — nothing is submitted for you" while `submit` filed real
public issues.

## Code Anchors

- `cmd/browser-agent/tools_configure.go`
- `cmd/browser-agent/tools_configure_report_issue_test.go`
- `internal/issuereport/handler.go`
- `internal/issuereport/handler_test.go`
- `internal/issuereport/types.go`
- `internal/issuereport/templates.go`
- `internal/issuereport/sanitize.go`
- `internal/issuereport/submit.go`
- `internal/issuereport/templates_test.go`
- `internal/issuereport/sanitize_test.go`
- `internal/issuereport/submit_test.go`
