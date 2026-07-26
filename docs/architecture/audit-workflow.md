---
doc_type: canonical_flow_map
flow_id: audit-workflow
feature_refs:
  - feature-auto-fix
  - feature-tab-tracking-ux
last_reviewed: 2026-04-03
code_anchors:
  - src/lib/request-audit.ts
  - src/popup/tab-tracking.ts
  - src/popup/tab-tracking-api.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/background/message-handlers.ts
  - cmd/browser-agent/handler_tools_call_postprocess.go
  - cmd/browser-agent/internal/terminal/intent_handlers.go
  - cmd/browser-agent/internal/terminal/intent_store.go
  - plugin/kaboom-workflows/commands/audit.md
  - npm/kaboom-agentic-browser/skills/audit/SKILL.md
  - npm/kaboom-agentic-browser/skills/qa/SKILL.md
test_anchors:
  - tests/extension/request-audit.test.js
  - tests/extension/message-handlers.test.js
  - tests/extension/popup-audit-button.test.js
  - tests/extension/popup-tab-tracking-sync.test.js
  - tests/extension/tracked-hover-launcher.test.js
  - tests/packaging/kaboom-audit-workflow.test.js
  - tests/packaging/kaboom-skills-branding.test.js
  - cmd/browser-agent/handler_tools_call_postprocess_test.go
last_verified_version: 0.8.1
last_verified_date: 2026-04-03
---

# Audit Workflow

## Scope

Define the Phase 1 Kaboom audit workflow: tracked-site popup and hover entrypoints, one shared audit trigger, one background runtime bridge, one repo-owned `/kaboom/audit` command plus bundled `audit` skill, and one local six-lane report.

Related feature docs:

- `docs/features/feature/auto-fix/index.md`
- `docs/features/feature/tab-tracking-ux/index.md`

## Entrypoints

- `extension/popup.html` tracked-state `Audit` button
- `src/popup/tab-tracking.ts` tracked-state popup wiring
- `src/content/ui/tracked-hover-launcher.ts` hover-surface `Audit` action
- `plugin/kaboom-workflows/commands/audit.md`
- `npm/kaboom-agentic-browser/skills/audit/SKILL.md`

## Primary Flow

1. A user tracks a site, then launches `Audit` from the popup or tracked hover launcher.
2. Both UI surfaces call `requestAudit` in `src/lib/request-audit.ts`.
3. `requestAudit` opens the terminal side panel first via `open_terminal_panel`.
4. The helper then sends the existing `qa_scan_requested` runtime message with the tracked page URL.
5. `src/background/message-handlers.ts` converts that runtime message into audit-oriented prompt text for terminal injection.
6. Primary path: the terminal server injects the prompt directly into the active PTY session.
7. Fallback path: the terminal server stores a `qa_scan` intent for later pickup.
8. On the next MCP tool response, `cmd/browser-agent/handler_tools_call_postprocess.go` prepends an `ACTION REQUIRED` warning that points the operator at `/kaboom/audit` or `/audit`.
9. The agent runs the repo-owned command or bundled skill:
   - health check
   - baseline `page_issues` summary
   - page exploration and interaction discovery
   - six-lane review: Functionality, UX Polish, Accessibility, Performance, Release Risk, SEO
10. The final output is one local markdown report with:
   - Overall Score
   - Lane Scores
   - Executive Summary
   - Top Findings
   - Fast Wins
   - Ship Blockers
   - Coverage And Limits

## Error and Recovery Paths

- If no tracked site exists, popup and skill copy should tell the user to track a site first.
- `requestAudit` treats side-panel open failures as best-effort and still sends the audit request bridge.
- If PTY injection is unavailable, the daemon persists the intent and nudges the next MCP response.
- If namespaced slash commands are unsupported, operators fall back from `/kaboom/audit` to `/audit`.

## State and Contracts

- Keep `qa_scan_requested` as the internal runtime message in Phase 1.
- Keep `qa_scan` as the daemon intent action name in Phase 1.
- User-facing product copy is `Audit`, not `Find Problems`.
- The audit workflow is local-only in Phase 1: no watch mode, history, hosted reports, or team workflow.
- The report contract is fixed across the command and bundled skill.

## Code Paths

- `src/lib/request-audit.ts`
- `src/popup/tab-tracking.ts`
- `src/popup/tab-tracking-api.ts`
- `src/content/ui/tracked-hover-launcher.ts`
- `src/background/message-handlers.ts`
- `cmd/browser-agent/handler_tools_call_postprocess.go`
- `cmd/browser-agent/internal/terminal/intent_handlers.go`
- `cmd/browser-agent/internal/terminal/intent_store.go`
- `plugin/kaboom-workflows/commands/audit.md`
- `npm/kaboom-agentic-browser/skills/audit/SKILL.md`
- `npm/kaboom-agentic-browser/skills/qa/SKILL.md`

## Test Paths

- `tests/extension/request-audit.test.js`
- `tests/extension/message-handlers.test.js`
- `tests/extension/popup-audit-button.test.js`
- `tests/extension/popup-tab-tracking-sync.test.js`
- `tests/extension/tracked-hover-launcher.test.js`
- `tests/packaging/kaboom-audit-workflow.test.js`
- `tests/packaging/kaboom-skills-branding.test.js`
- `cmd/browser-agent/handler_tools_call_postprocess_test.go`

## Edit Guardrails

- Do not rename the runtime bridge or intent action in Phase 1 unless both extension and daemon contracts are updated together.
- Keep popup and hover entrypoints on the same `requestAudit` helper.
- Preserve the terminal-first behavior before requesting the audit bridge.
- Keep the command and bundled skill aligned on one report structure and one six-lane methodology.
