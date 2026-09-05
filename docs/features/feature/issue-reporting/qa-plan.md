---
doc_type: qa-plan
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

# Issue Reporting QA Plan

## Unit Tests

### `internal/issuereport/` (28 tests)

| Test | Validates |
|------|-----------|
| `TestTemplateNames_ReturnsFiveTemplates` | 5 templates available |
| `TestTemplateNames_AreSorted` | Names are alphabetically sorted |
| `TestGetTemplate_Found` | All named templates exist with required fields |
| `TestGetTemplate_NotFound` | Unknown template returns nil |
| `TestGetTemplate_AllHaveUserReportedLabel` | Every template tagged user-reported |
| `TestSanitizeReport_RedactsTitle` | AWS key redacted from title |
| `TestSanitizeReport_RedactsUserContext` | GitHub PAT redacted from user_context |
| `TestSanitizeReport_RedactsExtensionSource` | Secrets redacted from extension source |
| `TestSanitizeReport_PreservesNonSensitiveData` | Non-secret fields unchanged |
| `TestSanitizeReport_DoesNotMutateOriginal` | Original report is not modified |
| `TestSubmitViaGH_GHNotFound` | Returns manual fallback when gh not installed |
| `TestSubmitViaGH_Success` | Parses issue URL from stdout |
| `TestSubmitViaGH_GHError` | Returns error with stderr context |
| `TestSubmitViaGH_IncludesLabels` | Labels passed to gh CLI |
| `TestFormatIssueBody_ContainsAllSections` | All diagnostic sections present |
| `TestFormatIssueBody_NoDescriptionWhenEmpty` | No description section when user_context empty |
| `TestSubmitViaGH_TargetRepo` | Target repo constant is correct |
| `TestSubmitViaGHSendsExactlyTheDocumentedPayload` | Outbound body and gh argv match the documented payload character-for-character |
| `TestSubmitWithoutConfirmTransmitsNothing` | list_templates, preview, submit without `confirm`, and `confirm:false` all reach the submit dependency zero times |
| `TestUnconfirmedSubmitNamesTheDestinationAndTheGate` | Refusal names `confirm`, the target repo, and `preview` |
| `TestConfirmedSubmitTransmitsOnce` | `confirm:true` reaches the submit dependency exactly once |
| `TestConfirmedSubmitStillValidatesItsPayload` | `confirm` cannot push a missing title or unknown template through |
| `TestModeSpecDescribesWhatSubmitActuallyDoes` | describe_capabilities text for report_issue matches the handler's real transmission behavior |

### Operation routing (`internal/issuereport/handler_test.go`)

| Test | Validates |
|------|-----------|
| `TestHandleListsAndPreviewsKnownTemplates` | list_templates, default preview, and named preview all succeed |
| `TestHandleRejectsMalformedAndIncompleteIssueRequests` | Malformed JSON, missing title, unknown template, and unconfirmed submit all return errors without leaking context |
| `TestHandleRejectsUnknownOperation` | Unknown operation returns an error |
| `TestHandleSubmitsSanitizedReport` | Confirmed submit succeeds |
| `TestHandlePreviewReturnsSanitizedDiagnosticsWithSnakeCaseFields` | Preview redacts secrets and returns snake_case diagnostics |

## Manual Verification

1. `configure(what="report_issue", operation="list_templates")` — returns 5 templates
2. `configure(what="report_issue")` — preview with diagnostics, nothing sent
3. `configure(what="report_issue", operation="preview", template="bug", user_context="test")` — sanitized preview
4. `configure(what="report_issue", operation="submit", title="Test", template="bug")` — refused with `missing_param: confirm`; nothing is sent

Do **not** exercise `operation="submit"` with `confirm: true` during UAT: it publishes
a real public issue on `brennhill/Kaboom-Browser-AI-Devtools-MCP` under the tester's
own GitHub account. Step 4 above verifies the gate holds, which is the behaviour
under test; the confirmed path is covered by unit tests with an injected runner.
