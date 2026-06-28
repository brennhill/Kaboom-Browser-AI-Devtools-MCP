---
name: kaboom
description: >
  Use when user asks to check browser state, debug page errors, inspect network
  traffic, take screenshots, automate clicks or form fills, run accessibility or
  security audits, measure performance, generate tests, or record browser sessions.
  Trigger phrases: "check my browser", "debug this page", "take a screenshot",
  "browser errors", "network requests", "automate the browser", "record a test",
  "accessibility audit", "performance check", "what's on the page".
license: AGPL-3.0
compatibility: >
  Requires kaboom-agentic-browser binary on PATH and Kaboom Chrome
  extension connected. macOS, Linux, Windows.
metadata:
  author: Kaboom AI
  version: 0.8.4
  category: developer-tools
  tags: [browser, debugging, automation, testing, observability]
---

# Kaboom

## Critical: Daemon Setup

Before any tool call, ensure the daemon is running:

```bash
bash scripts/ensure-daemon.sh
```

If this fails, run the connection doctor:

```bash
bash scripts/connection-doctor.sh
```

## How to Call Tools

All 5 tools are called via the helper script:

```bash
bash scripts/kaboom-call.sh <tool> '<json_arguments>'
```

Examples:

```bash
# Read browser errors
bash scripts/kaboom-call.sh observe '{"what":"errors","limit":20}'

# Take a screenshot
bash scripts/kaboom-call.sh observe '{"what":"screenshot","format":"png"}'

# Click an element
bash scripts/kaboom-call.sh interact '{"what":"click","selector":"button.submit"}'

# Run accessibility audit
bash scripts/kaboom-call.sh analyze '{"what":"accessibility"}'

# Generate a Playwright test
bash scripts/kaboom-call.sh generate '{"what":"test","test_name":"login_flow"}'

# Check health
bash scripts/kaboom-call.sh configure '{"what":"health"}'
```

## Tools Quick Reference

### observe — Read captured browser telemetry (passive)

Top modes: `errors`, `logs`, `network_waterfall`, `network_bodies`, `actions`, `screenshot`, `page`, `tabs`, `storage`, `recordings`, `websocket_events`, `error_bundles`, `timeline`, `transients`, `page_inventory`

Common params: `what` (required), `limit`, `since_cursor`, `summary`, `format`, `url`, `scope`

Pagination: pass `after_cursor`/`before_cursor`/`since_cursor` from response metadata. Set `restart_on_eviction=true` if cursor expired.

Full reference: `references/observe-modes.md`

### analyze — Active analysis queries (sent to extension)

Top modes: `dom`, `accessibility`, `security_audit`, `performance`, `forms`, `link_health`, `page_summary`, `computed_styles`, `visual_diff`, `api_validation`, `third_party_audit`, `audit`

Common params: `what` (required), `selector`, `timeout_ms`, `background`, `summary`

Async mode: set `background:true` to get `correlation_id`, then poll with `observe(what='command_result', correlation_id=...)`.

Full reference: `references/analyze-modes.md`

### generate — Create artifacts from captured data

Top modes: `test`, `reproduction`, `pr_summary`, `har`, `csp`, `sarif`, `visual_test`, `test_from_context`, `test_heal`, `test_classify`

Common params: `what` (required), `test_name`, `save_to`, `last_n`

Full reference: `references/generate-modes.md`

### configure — Session management and diagnostics

Top modes: `health`, `doctor`, `store`, `clear`, `describe_capabilities`, `event_recording_start`, `event_recording_stop`, `playback`, `noise_rule`, `streaming`, `telemetry`, `diff_sessions`

Common params: `what` (required), `store_action`, `key`, `value`

Full reference: `references/configure-modes.md`

### interact — Browser automation

Top modes: `click`, `type`, `navigate`, `screenshot`, `execute_js`, `explore_page`, `list_interactive`, `fill_form`, `scroll_to`, `hover`, `wait_for`, `batch`

Common params: `what` (required), `selector`, `text`, `url`, `script`

Targeting priority: `selector` > `element_id` > `index` > `x/y` coordinates

Semantic selectors: `text=Submit`, `role=button`, `placeholder=Email`

Composable enrichment: add `include_screenshot:true`, `action_diff:true`, or `wait_for_stable:true` to any action.

Full reference: `references/interact-actions.md`

## Workflows

Choose the workflow that matches your task:

| Task | Workflow |
|------|----------|
| Debug errors, triage failures, create regression tests | `references/workflow-debug-and-triage.md` |
| Site audit, UX review, accessibility, security, API validation | `references/workflow-audit-and-security.md` |
| Browser automation, demos, test generation, release gates | `references/workflow-automate-and-test.md` |
| Performance measurement and comparison | `references/workflow-performance.md` |

## Troubleshooting

### Daemon not running

Run: `bash scripts/ensure-daemon.sh`
If fails: check `which kaboom` — binary must be on PATH.
Install: `npm install -g kaboom-agentic-browser`

### Extension not connected

Run: `bash scripts/connection-doctor.sh`
Check Chrome extension is installed and enabled on the target page.
The extension POSTs telemetry to localhost:7890.

### No data returned

Ensure a tab is tracked:

```bash
bash scripts/kaboom-call.sh observe '{"what":"tabs"}'
```

If no tracked tab, open a page with the extension active.

### Tool call returns error

Use describe_capabilities for parameter help:

```bash
bash scripts/kaboom-call.sh configure '{"what":"describe_capabilities","tool":"observe","mode":"errors"}'
```

### Connection lost mid-session

Re-run ensure-daemon and retry:

```bash
bash scripts/ensure-daemon.sh
bash scripts/kaboom-call.sh configure '{"what":"health"}'
```
