---
doc_type: canonical_flow_map
flow_id: auto-fix-qa-flow
feature_refs:
  - feature-auto-fix
last_reviewed: 2026-03-28
code_anchors:
  - cmd/browser-agent/tools_analyze_page_issues.go
  - cmd/browser-agent/intent_store.go
  - cmd/browser-agent/intent_handlers.go
  - src/content/ui/tracked-hover-launcher.ts
  - src/background/message-handlers.ts
  - npm/gasoline-agentic-browser/skills/qa/SKILL.md
test_anchors:
  - cmd/browser-agent/tools_analyze_page_issues_test.go
  - cmd/browser-agent/intent_store_test.go
---

# Auto-Fix QA Flow

## Flow 1: AI-initiated ("QA my app")

```
User says "QA my app"
  → AI discovers qa skill (installed at ~/.claude/skills/qa.md)
  → AI follows methodology:
      1. configure(what:"health") — verify connectivity
      2. analyze(what:"page_issues", summary:true) — baseline sweep
         → Go handler: toolAnalyzePageIssues
         → Prefetches shared data (logs, network, waterfall)
         → Fan-out: console errors | network failures | a11y | security
         → Aggregates results with severity ranking
      3. observe(what:"screenshot") — visual state
      4. interact(what:"explore_page") — discover navigation
      5. Per-page: navigate → screenshot → page_issues → interact
      6. Gather evidence: error_bundles, network_bodies, dom
      7. Produce prioritized report
      8. Offer to fix (if source code accessible)
```

## Flow 2: Button-initiated ("Find Problems")

```
User clicks "Find Problems" on hover widget
  → tracked-hover-launcher.ts: createActionButton click handler
  → chrome.runtime.sendMessage({ type: 'qa_scan_requested' })
  → message-handlers.ts: handleQaScanRequestedAsync
      ├── Try: POST /terminal/inject { text: QA prompt }
      │     → intent_handlers.go: handleTerminalInject
      │     → terminal_relay.go: writeToFirst
      │     → PTY session receives prompt → AI acts on it
      │     ✓ Success: done
      │
      └── Fallback: POST /intent { action: "qa_scan" }
            → intent_handlers.go: handleIntentCreate
            → intent_store.go: Add (with TTL + correlation ID)
            → On next AI tool call:
                handler_tools_call_postprocess.go: maybeAddPendingIntents
                → intentStore.NudgeAndClean()
                → Prepends "ACTION REQUIRED" nudge to response
                → Nudges up to 3 times before discarding
```

## Flow 3: page_issues internal aggregation

```
analyze(what:"page_issues")
  → tools_analyze_dispatch.go → toolAnalyzePageIssues
  → prefetchSharedData: single copy of logs, network bodies, waterfall
  → Parallel fan-out (4 goroutines, 5s timeout each):
      ├── collectConsoleErrors(shared.logEntries)
      ├── collectNetworkFailures(shared.networkBodies)
      ├── collectA11yIssues (async extension query)
      └── collectSecurityIssues(shared)
  → Aggregate: sections, totalIssues, bySeverity
  → If summary=true: buildPageIssuesSummary (top 10 by severity)
  → Return unified report
```
