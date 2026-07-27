---
doc_type: feature_index
feature_id: feature-auto-fix
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/tools_analyze_page_issues.go
  - cmd/browser-agent/internal/toolanalyze/page_issues_summary.go
  - cmd/browser-agent/tools_analyze_dispatch.go
  - cmd/browser-agent/handler_tools_call_postprocess.go
  - cmd/browser-agent/internal/terminal/intent_store.go
  - cmd/browser-agent/internal/terminal/intent_handlers.go
  - internal/schema/analyze.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
  - plugin/kaboom-workflows/commands/audit.md
  - npm/kaboom-agentic-browser/skills/audit/SKILL.md
  - npm/kaboom-agentic-browser/skills/qa/SKILL.md
  - src/lib/tabs/request-audit.ts
  - src/popup/tabs/tab-tracking.ts
  - src/popup/tabs/tab-tracking-api.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/background/message-handlers.ts
  - src/types/runtime-messages.ts
test_paths:
  - cmd/browser-agent/handler_tools_call_postprocess_test.go
  - cmd/browser-agent/tools_analyze_page_issues_test.go
  - cmd/browser-agent/internal/terminal/intent_store_test.go
  - tests/extension/request-audit.test.js
  - tests/extension/message-handlers.test.js
  - tests/extension/popup-audit-button.test.js
  - tests/extension/popup-tab-tracking-sync.test.js
  - tests/extension/tracked-hover-launcher.test.js
  - tests/packaging/kaboom-audit-workflow.test.js
  - tests/packaging/kaboom-skills-branding.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-04-03
---

# Auto-Fix

## TL;DR
- Status: shipped
- Phase 1 audit workflow backed by one shared runtime bridge
- Spec: `specs/auto-fix.md`
- Plan: `specs/auto-fix-plan.md`
- Flow Map: `flow-map.md`

## Components

### 1. `analyze(what: "page_issues")`
Internal evidence sweep that aggregates console errors, network failures (4xx/5xx), a11y violations, and security findings into a unified prioritized report. Runs checks in parallel with per-check timeouts. Supports `summary:true` for ~80% token reduction and serves as the audit baseline.

### 2. `/kaboom/audit` command + `audit` skill
Repo-owned workflow assets that turn the raw primitives into one product-shaped Phase 1 audit. They require a tracked site, run the six-lane review, and return a polished local report with scores, findings, Fast Wins, and Ship Blockers.

### 3. Tracked-site `Audit` entrypoints
Popup and hover-surface `Audit` actions both call `requestAudit`, which:
1. opens the terminal side panel first
2. sends the existing `qa_scan_requested` bridge
3. falls back to the daemon intent store when PTY injection is unavailable

The bundled `qa` skill is now a compatibility alias that redirects older QA requests to the same audit workflow.
