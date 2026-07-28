---
feature: pr-preview-exploration
status: proposed
doc_type: tech-spec
feature_id: feature-pr-preview-exploration
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: PR Preview Exploration

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

PR Preview Exploration is an **agent orchestration pattern**, not a Kaboom MCP tool or mode. Kaboom provides the primitives (`observe`, `generate`, `configure`, `interact`), and the AI agent composes them into a multi-phase workflow that validates preview deployments against behavioral baselines.

The feature adds no new MCP tools, no new modes, and requires zero changes to the Kaboom server or extension. It is entirely implemented as agent-side logic that uses existing capabilities in a specific sequence.

## Key Components

This is a workflow pattern with five sequential phases:

### Phase 1: Discovery
The agent learns the preview URL through one of three mechanisms (in priority order):

- **Human provides URL**: Developer pastes the preview URL directly. Immediate workflow start.
- **URL convention template**: Agent retrieves a stored template from persistent memory (e.g., `preview-{pr_number}.myapp.dev`) and substitutes the PR number.
- **GitHub Deployment API**: Agent queries GitHub's deployment status API for the preview URL (requires external `gh` CLI access, not part of Kaboom).

If all mechanisms fail, the agent prompts the human for the URL.

### Phase 2: Baseline Capture
Before visiting the preview, the agent establishes a "known good" behavioral baseline:

- **Live baseline (preferred)**: Navigate to the production/stable-branch version, capture a session snapshot via `configure({what: "diff_sessions", session_action: "capture"})`, and optionally save a behavioral baseline via `configure({what: "save_baseline"})`.
- **Stored baseline (faster)**: Load a previously saved baseline from persistent memory. Agent checks staleness (warns if > 24 hours old).

The baseline includes console error fingerprints, network endpoint status codes and latencies, performance metrics (LCP, FCP, CLS, load time), and WebSocket connection state.

### Phase 3: Exploration
The agent opens the preview URL in a new isolated tab and exercises the application:

#### Step 3.1: Open preview in new tab
```
interact({what: "new_tab", url: "https://preview-42.myapp.dev"})
```

#### Step 3.2: Wait for page load
Poll `observe({what: "page"})` until ready state is reached or timeout (30 seconds).

#### Step 3.3: Exercise the application
The agent performs a bounded set of interactions using `interact({what: "execute_js"})`:
- Click navigation links
- Fill and submit forms
- Trigger error states (empty inputs, invalid data)
- Scroll to load lazy content
- Exercise key user flows

Kaboom passively captures telemetry during this phase (console logs, network requests, WebSocket events, Web Vitals).

#### Step 3.4: Capture session snapshot
```
configure({what: "diff_sessions", session_action: "capture", name: "preview-pr-42"})
```

### Phase 4: Comparison
The agent compares the preview session against the baseline:

**Session diff**:
```
configure({what: "diff_sessions", session_action: "compare", compare_a: "stable-baseline", compare_b: "preview-pr-42"})
```

Returns structured diff: new errors, changed network responses, performance deltas, WebSocket state changes.

**Behavioral baseline comparison** (if available):
```
configure({what: "compare_baseline", name: "stable-baseline"})
```

Returns categorized regressions: network, timing, console, WebSocket.

**Incremental checks**:
The agent also queries telemetry directly to catch issues diffing might miss:
```
observe({what: "errors"})
observe({what: "performance"})
observe({what: "accessibility"})
observe({what: "security_audit"})
```

### Phase 5: Report
The agent synthesizes findings into a structured report:

**Human-readable PR comment**:
```
generate({what: "pr_summary"})
```

Produces markdown with error summaries, network issues, performance data, prepended with exploration context (preview URL, pages visited, actions taken).

**Machine-readable SARIF** (optional):
```
generate({what: "sarif"})
```

For GitHub code scanning integration.

The report includes: preview URL explored, pages visited, actions taken, regressions found (categorized by severity), new errors not in baseline, performance comparison (load times, Web Vitals), accessibility issues, and reproduction steps for each finding.

## Data Flows

```
Human/API provides preview URL
  |
  v
Agent captures baseline (production/main)
  -> configure({what: "diff_sessions", session_action: "capture"})
  -> configure({what: "save_baseline"}) [optional]
  |
  v
Agent navigates to preview
  -> interact({what: "new_tab", url: preview_url})
  |
  v
Agent exercises application
  -> interact({what: "execute_js"}) [multiple calls]
  |
  v
Kaboom captures telemetry passively
  -> Console errors, network, WebSocket, performance
  |
  v
Agent captures preview session
  -> configure({what: "diff_sessions", session_action: "capture", name: "preview-pr-N"})
  |
  v
Agent compares preview vs baseline
  -> configure({what: "diff_sessions", session_action: "compare"})
  -> configure({what: "compare_baseline"}) [if available]
  -> observe({what: "errors"})
  -> observe({what: "performance"})
  |
  v
Agent generates findings report
  -> generate({what: "pr_summary"})
  -> generate({what: "sarif"}) [optional]
  |
  v
Agent posts report to PR (via external gh CLI, not Kaboom)
```

## Implementation Strategy

**No Kaboom changes required.** This feature is entirely agent-side logic. The implementation is:

1. **Skill definition file** (if using Claude Code skills): A YAML or JSON file that encodes the workflow phases, exploration bounds, and thresholds.
2. **Agent prompt enhancement**: Instructions for the agent on how to orchestrate the five phases when the developer requests preview exploration.
3. **External GitHub integration**: Use `gh` CLI or GitHub API to post PR comments (not part of Kaboom's scope).

The trade-off: no new server overhead, but the workflow requires agent reasoning to execute. The agent must remember to capture the baseline before deploying, navigate to the preview after deployment, and wait for page loads between actions.

## Edge Cases & Assumptions

### Edge Cases

- **Preview not yet deployed**: Agent polls the preview URL (or GitHub deployment status) with exponential backoff, up to 5 minutes. If still unavailable, report "preview not ready" and stop.

- **Preview returns 404/500 at root**: Agent captures the error, reports "preview environment broken at root URL", and stops. This is a useful finding.

- **Preview requires authentication**: Agent detects redirect to login page or 401/403 response. Reports "preview requires authentication — provide credentials or configure auth in the skill" and stops.

- **Extension disconnected mid-exploration**: All `interact` and `observe` calls timeout. Agent captures partial results and reports what it found before disconnection.

- **Buffers overflow during exploration**: Log buffer (1000 entries) and network body buffer (100 entries) may evict early entries during long exploration. Agent processes telemetry incrementally (after each page visit) to prevent eviction of early findings.

- **Multiple concurrent explorations**: Each agent uses a unique session name (`preview-pr-42`, `preview-pr-43`) for `diff_sessions`. Session captures are scoped by name and do not conflict. However, shared telemetry buffers (logs, network) are not per-session — concurrent explorations will see each other's data. Mitigation: single-tab tracking isolation (v6) ensures each agent's telemetry is scoped to its tracked tab.

- **Preview URL changes mid-exploration**: Some preview environments use dynamic URLs that change on redeployment. Agent navigates once at start. If URL becomes invalid mid-exploration, agent stops and reports partial results.

- **No baseline available**: First run or stable branch unavailable. Agent skips comparison phase and reports absolute findings only (errors found, performance numbers, accessibility violations). Report notes "no baseline available — showing absolute findings, not regressions."

### Assumptions

- A1: The browser extension is connected and tracking the tab used for preview exploration.
- A2: The AI Web Pilot toggle is enabled (human has opted in to browser control via `execute_js`).
- A3: The preview environment is accessible from the machine running the browser (localhost or network-accessible).
- A4: The Kaboom server is running and accepting MCP tool calls.
- A5: The agent has sufficient context window to hold the exploration report plus the PR diff (for correlation).
- A6: Preview environments serve a web application that can be meaningfully explored via browser interactions (not a raw API, not a native app).

## Risks & Mitigations

### Risk 1: Preview URL validation bypass
- **Description**: Agent navigates to a malicious URL disguised as a preview (e.g., `javascript:`, `file://`, or internal network address).
- **Mitigation**: Agent-side workflow validates URL scheme (HTTPS only), domain pattern (matches configured allowlist like `*.vercel.app`, `*.netlify.app`, or custom domain), and excludes private network addresses (10.x.x.x, 192.168.x.x, 127.0.0.x except Kaboom localhost).

### Risk 2: Unbounded exploration
- **Description**: Agent explores infinitely, visiting hundreds of pages and consuming excessive time/tokens.
- **Mitigation**: Workflow enforces explicit bounds: max 20 pages visited, max 10 actions per page, max 100 total actions, 15-minute wall-clock timeout, max navigation depth of 3 clicks. Partial exploration is acceptable — agent stops at bound and reports what it found.

### Risk 3: Auth token leakage
- **Description**: Preview environments requiring authentication expose tokens in URL params or cookies, which leak into telemetry.
- **Mitigation**: Kaboom strips Authorization headers and redacts sensitive cookies from network captures (existing privacy layer). Agent-side workflow must redact any token values from the exploration report before posting to PR.

### Risk 4: Extension disconnects mid-workflow
- **Description**: Network interruption or browser crash disconnects the extension during exploration.
- **Mitigation**: Agent checks extension connectivity via `observe({what: "page"})` before starting. If disconnection occurs mid-exploration, tool calls timeout. Agent captures partial results and reports what was found before disconnection.

### Risk 5: False positives from stale baseline
- **Description**: Baseline is 7 days old, application evolved legitimately, agent reports new features as "regressions."
- **Mitigation**: Agent warns when baseline age exceeds 24 hours (configurable threshold). Developer can choose to recapture baseline or proceed with awareness of staleness. Agent should note baseline age in the report.

## Dependencies

### Depends on:
- `interact` tool: `navigate`, `new_tab`, `execute_js`, `refresh` actions (shipped)
- `observe` tool: `errors`, `network_waterfall`, `performance`, `accessibility`, `page` modes (shipped)
- `configure` tool: `diff_sessions`, `store`/`load` actions (shipped)
- `generate` tool: `pr_summary`, `sarif` formats (shipped)
- Behavioral baselines: `save_baseline`, `compare_baseline` (in-progress, optional dependency)
- Single-tab tracking isolation (shipped in v6): ensures per-tab telemetry scoping
- AI Web Pilot toggle (shipped): human opt-in required for `execute_js`

### Depended on by:
- Agentic CI/CD (proposed): PR preview exploration is one of the four agentic workflows
- Deployment Watchdog (proposed): shares the baseline comparison pattern

## Performance Considerations

Kaboom per-operation overhead is minimal:

| Operation | Kaboom overhead | Notes |
|-----------|------------------|-------|
| Navigate to preview | < 2s | Actual page load depends on preview environment |
| Session capture | < 200ms | Existing `diff_sessions` performance |
| Session comparison | < 500ms | Depends on session size |
| Baseline comparison | < 20ms | Existing `compare_baseline` performance |
| PR summary generation | < 300ms | Existing `pr_summary` generation |
| Full exploration workflow | < 15 minutes | Agent-side bound, not a Kaboom SLO |

Dominant costs are page load time (preview environment performance) and agent reasoning time (outside Kaboom's control).

## Security Considerations

**Preview URL validation**: Agent must validate URLs before navigation. Only HTTPS scheme, domain matches configured allowlist (e.g., `*.vercel.app`, `*.netlify.app`, or custom domain), no private network addresses except Kaboom's localhost.

**Authentication for preview environments**: Many previews require auth. Two mechanisms:
- **Cookie injection**: Agent uses `execute_js` to set cookies before navigating. Cookie value must come from secure source (CI secret, vault), never hardcoded.
- **Bearer token in URL**: Token appended as query param (e.g., `?token=abc`).

Security constraints:
- Auth tokens must never appear in Kaboom telemetry. Kaboom already strips Authorization headers. Cookie values set via `execute_js` are not captured in logs.
- Auth tokens must be redacted from PR comment output.
- Skill definition must document which secrets are required and how they're injected.

**Data sensitivity**: Preview environments may contain test data. Kaboom's existing privacy controls apply:
- Network body capture is opt-in (off by default). Agent enables selectively for API endpoints it wants to validate.
- Kaboom strips sensitive headers (Authorization, Cookie, tokens).
- Redaction patterns (configured via `configure({what: "noise_rule"})`) apply to all captured data.

**execute_js security surface**: Agent generates scripts simulating user behavior (clicking, typing, scrolling). It does not read sensitive page data (localStorage, sessionStorage, form values). AI Web Pilot toggle must be enabled by human before `execute_js` works (existing security gate).
