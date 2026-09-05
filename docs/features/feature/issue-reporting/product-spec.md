---
doc_type: product-spec
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

# Issue Reporting Product Spec

## Purpose

Enable LLMs and users to file sanitized bug reports to GitHub Issues directly from a Kaboom session. Explicit, opt-in exception to Rule 7 ("all data stays local") — the user must approve every submission, and the approval is carried by `confirm: true` on the submitting call itself.

## Operations (`operation`)

- `list_templates` — returns available issue categories; nothing leaves the machine
- `preview` (default) — collects diagnostics, sanitizes, shows payload; nothing leaves the machine
- `submit` — requires `confirm: true`. Sanitizes and publishes a **public** issue on `brennhill/Kaboom-Browser-AI-Devtools-MCP` via `gh issue create`, under whichever GitHub account the local `gh` CLI is signed in as; falls back to returning the formatted body if `gh` is unavailable. Without `confirm: true` the call is refused and nothing is sent.

## What `submit` Publishes

Exactly these fields, and nothing else: `title` and `user_context` (both redaction-engine output), Kaboom version, OS/arch/Go version, uptime seconds, total call count, total error count, error rate, extension-connected flag, extension session id, and console/network/action buffer **counts**. No URLs, page content, log lines, or captured request/response bodies. `internal/issuereport.FormatIssueBody` is the single place that builds the outbound body, and its output is pinned character-for-character by `TestSubmitViaGHSendsExactlyTheDocumentedPayload`.

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
- `IR_PROD_006`: `submit` requires `confirm: true` on the same call. Without it the handler refuses before reaching `gh`, and no other operation can reach it. An agent enumerating configure modes must not be able to publish an issue on a user's behalf.
- `IR_PROD_007`: The `describe_capabilities` text for this mode states that a confirmed `submit` publishes publicly, and names the destination repository. It may not describe the mode as text-only.

## Non-Goals

- No automatic/background telemetry.
- No HTTP fallback endpoint in the daemon.
- No file attachments or screenshots.

## Related

- Command matrix: `docs/core/protocol/mcp-command-option-matrix.md`
