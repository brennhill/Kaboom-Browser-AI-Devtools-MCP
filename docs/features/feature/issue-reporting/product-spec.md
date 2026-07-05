---
doc_type: product-spec
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

# Issue Reporting Product Spec

## Purpose

Enable LLMs and users to file sanitized bug reports to GitHub Issues directly from a Kaboom session. Explicit, opt-in exception to Rule 7 ("all data stays local") — the user must approve every submission.

## Operations (`operation`)

- `list_templates` — returns available issue categories
- `preview` (default) — collects diagnostics, sanitizes, shows payload — nothing leaves the machine
- `submit` — sanitizes and sends via `gh issue create`; falls back to manual if gh unavailable

## Templates

| Name | Purpose |
|------|---------|
| `bug` | Report unexpected behavior or errors |
| `crash` | Report daemon crash or hang |
| `extension_issue` | Report extension connectivity or behavior problems |
| `performance` | Report slow responses or high resource usage |
| `feature_request` | Suggest a new feature or improvement |

## User Outcomes

1. Preview exactly what would be sent before any data leaves the machine.
2. File a bug report with automatic diagnostics collection.
3. Secrets are automatically redacted from all submitted content.

## Requirements

- `IR_PROD_001`: Default operation is `preview` — no data transmission.
- `IR_PROD_002`: All user-supplied text passes through the redaction engine.
- `IR_PROD_003`: `submit` requires a `title` parameter.
- `IR_PROD_004`: If `gh` CLI is unavailable, return the formatted body for manual filing.
- `IR_PROD_005`: Template validation rejects unknown template names.

## Non-Goals

- No automatic/background telemetry.
- No HTTP fallback endpoint in the daemon.
- No file attachments or screenshots.

## Related

- Command matrix: `docs/core/mcp-command-option-matrix.md`
