---
doc_type: qa-plan
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

# Issue Reporting QA Plan

## Unit Tests

### `internal/issuereport/` (18 tests)

| Test | Validates |
|------|-----------|
| `TestTemplateNames_ReturnsFiveTemplates` | 5 templates available |
| `TestTemplateNames_AreSorted` | Names are alphabetically sorted |
| `TestGetTemplate_Found` | All named templates exist with required fields |
| `TestGetTemplate_NotFound` | Unknown template returns nil |
| `TestGetTemplate_AllHaveUserReportedLabel` | Every template tagged user-reported |
| `TestTemplateCount_MatchesNames` | Count and names list are consistent |
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

### `cmd/browser-agent/` (14 tests)

| Test | Validates |
|------|-----------|
| `TestReportIssue_ListTemplates` | Returns 5 templates with required fields |
| `TestReportIssue_PreviewDefault` | Default operation is preview |
| `TestReportIssue_PreviewWithTemplate` | Template and user_context in preview |
| `TestReportIssue_PreviewContainsDiagnostics` | Diagnostics present in preview |
| `TestReportIssue_PreviewRedactsSecrets` | AWS key redacted across all content blocks |
| `TestReportIssue_SubmitRequiresTitle` | Submit without title returns error |
| `TestReportIssue_InvalidOperation` | Unknown operation returns error |
| `TestReportIssue_InvalidTemplate` | Unknown template returns error |
| `TestReportIssue_InvalidJSON` | Malformed JSON returns error |
| `TestReportIssue_SnakeCaseFields` | list_templates response uses snake_case |
| `TestReportIssue_PreviewSnakeCaseFields` | Preview response uses snake_case |
| `TestReportIssue_SubmitWithFakeRunner_Success` | Submit success via injected runner |
| `TestReportIssue_SubmitWithFakeRunner_GHNotFound` | Manual fallback via injected runner |
| `TestReportIssue_SubmitWithFakeRunner_GHError` | Error fallback via injected runner |

## Manual Verification

1. `configure(what="report_issue", operation="list_templates")` — returns 5 templates
2. `configure(what="report_issue")` — preview with diagnostics, nothing sent
3. `configure(what="report_issue", operation="preview", template="bug", user_context="test")` — sanitized preview
4. `configure(what="report_issue", operation="submit", title="Test", template="bug")` — files or returns manual fallback
