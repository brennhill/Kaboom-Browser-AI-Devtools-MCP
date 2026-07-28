---
feature: pr-preview-exploration
status: proposed
version: null
tool: observe, generate, interact, configure
mode: multi-mode (orchestration pattern)
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-pr-preview-exploration
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# PR Preview Exploration

> AI coding agents autonomously navigate to PR preview/staging environments, capture runtime telemetry, compare against baselines, and report findings — bridging static code review with runtime behavior validation.

## Problem

Code review catches logic errors, style issues, and architectural concerns. It does not catch runtime bugs. Preview deployments (Vercel, Netlify, Render, Railway, etc.) exist for exactly this purpose, but they rely on humans to manually open the URL, click around, and spot problems. In practice:

1. **Preview links go unclicked.** Most PR preview deployments are never visited by a reviewer. The deployment badge sits in the PR checks section, ignored.
2. **Manual testing is shallow.** When someone does visit, they check the happy path. Edge cases (empty states, error states, slow networks) are never tested.
3. **Runtime regressions slip through.** Console errors, failed API calls, performance regressions, and accessibility violations are invisible in a code diff. They surface only after merge, in production.
4. **No baseline comparison.** Even if a reviewer visits the preview, they have no objective baseline to compare against. "Does this feel slower?" is not a useful signal.

The gap is clear: code review has no runtime signal. Preview deployments have runtime signal but no one looking.

## Solution

PR Preview Exploration is a **multi-step workflow that the AI agent orchestrates** using existing Kaboom MCP tools. It is not a new Kaboom tool. Kaboom provides the observation and interaction primitives; the agent composes them into an exploration workflow.

The workflow proceeds in five phases:

1. **Discover** — The agent learns the preview URL (from the human, from a convention, or from the GitHub deployment status API).
2. **Baseline** — The agent captures a behavioral baseline of the stable branch (or loads a previously saved baseline).
3. **Explore** — The agent navigates to the preview URL, exercises the app, and lets Kaboom passively capture telemetry.
4. **Compare** — The agent compares the preview telemetry against the baseline to find regressions.
5. **Report** — The agent produces a structured findings report, optionally posted as a PR comment.

### Architecture: Orchestration, Not a New Tool

This feature strictly follows the Kaboom architecture: **5 tools, no more**. The agent uses existing modes across all five tools:

| Phase | Tool | Mode/Action | Purpose |
|-------|------|-------------|---------|
| Discover | (external) | GitHub API / user input | Get preview URL |
| Baseline | `configure` | `diff_sessions` (capture) | Snapshot stable branch state |
| Baseline | `configure` | `save_baseline` / `compare_baseline` | Behavioral fingerprint |
| Explore | `interact` | `navigate` | Open the preview URL |
| Explore | `interact` | `execute_js` | Click, type, scroll |
| Explore | `interact` | `new_tab` | Isolate preview in its own tab |
| Capture | `observe` | `errors`, `network_waterfall`, `performance`, `accessibility` | Read telemetry |
| Compare | `configure` | `diff_sessions` (compare) | Before/after diff |
| Compare | `configure` | `compare_baseline` | Behavioral regression detection |
| Report | `generate` | `pr_summary` | Structured markdown output |
| Report | `generate` | `sarif` | Machine-readable findings |

No new tools. No new modes. The entire feature is an agent behavior pattern that composes existing primitives.

## User Stories

- As an AI coding agent, I want to navigate to a PR preview URL so that I can observe its runtime behavior.
- As an AI coding agent, I want to compare preview telemetry against a stable-branch baseline so that I can detect regressions introduced by the PR.
- As an AI coding agent, I want to produce a structured findings report so that the PR author knows what runtime issues to fix before merging.
- As a developer, I want AI to automatically explore my PR previews so that runtime bugs are caught before merge without manual testing.
- As a team lead, I want PR preview exploration results posted as PR comments so that the review process includes runtime signal alongside code signal.
- As a developer, I want to provide my preview URL convention (e.g., `preview-{pr_number}.myapp.dev`) so that exploration happens automatically without manual input.

## Workflow Detail

### Phase 1: Discovery — How the Agent Learns the Preview URL

The agent needs to know where the preview is deployed. Three mechanisms, in priority order:

**Mechanism A: Human provides the URL.** The simplest case. The developer pastes the preview URL into the conversation. The agent proceeds immediately.

**Mechanism B: URL convention.** The developer (or team) configures a URL template in Kaboom's persistent memory:

```
configure({what: "store", store_action: "save", key: "preview_url_template", data: "https://preview-{pr_number}.myapp.dev"})
```

The agent retrieves this template and substitutes the PR number.

**Mechanism C: GitHub Deployment Status API.** The agent uses external tools (not Kaboom) to query the GitHub API for deployment status on the PR. This returns the preview URL from the `environment_url` field of the most recent successful deployment. This mechanism is outside Kaboom's scope — it depends on the agent's access to GitHub APIs via `gh` CLI or similar.

The agent tries mechanisms in order: if the human provided a URL, use it. If not, check persistent memory for a convention. If not, try the GitHub API. If all fail, ask the human.

### Phase 2: Baseline Capture

Before exploring the preview, the agent needs a "known good" state to compare against. Two strategies:

**Strategy A: Live baseline.** The agent navigates to the production/stable-branch version of the app, captures a session snapshot and a behavioral baseline, then navigates to the preview. This is the most accurate but doubles the exploration time.

**Strategy B: Stored baseline.** The agent loads a previously saved baseline (from `compare_baseline` / `diff_sessions`). This is faster but may be stale. The agent checks the baseline's creation timestamp and warns if it is older than a configurable threshold (default: 24 hours).

The choice between strategies depends on context. If the agent has access to the production URL and the exploration budget allows it, Strategy A is preferred. If a recent baseline exists, Strategy B saves time.

Baseline capture uses:
- `configure({what: "diff_sessions", session_action: "capture", name: "stable-baseline"})` for session-level diffing
- Behavioral baselines (`save_baseline`) for structured regression detection of network patterns, console errors, WebSocket state, and timing

### Phase 3: Exploration

The agent opens the preview URL and exercises the application. This is where Kaboom's interact tool and passive telemetry capture work together.

#### Step 3.1: Open the preview in a new tab.

```
interact({what: "new_tab", url: "https://preview-42.myapp.dev"})
```

This isolates the preview exploration from the developer's browsing. The agent notes the returned tab_id and targets all subsequent interactions to this tab.

#### Step 3.2: Wait for page load.

The agent polls `observe({what: "page"})` until the page reports ready state, or until a timeout (default: 30 seconds).

#### Step 3.3: Explore the application.

The agent performs a bounded set of interactions:
- Click navigation links to visit key pages
- Fill and submit forms to test input handling
- Trigger error states (empty inputs, invalid data)
- Scroll to load lazy content
- Exercise key user flows identified by the page structure

All interactions use `interact({what: "execute_js", ...})` with scripts that simulate user behavior (clicking elements, typing into inputs, submitting forms).

While the agent interacts, Kaboom passively captures:
- Console errors and warnings (log buffer)
- Network requests and responses (network waterfall, network bodies)
- WebSocket events
- Web Vitals (LCP, FID, CLS)

#### Step 3.4: Capture telemetry at exploration end.

```
configure({what: "diff_sessions", session_action: "capture", name: "preview-pr-42"})
```

### Phase 4: Comparison

The agent compares the preview session against the baseline:

#### Session diff:
```
configure({what: "diff_sessions", session_action: "compare", compare_a: "stable-baseline", compare_b: "preview-pr-42"})
```

This returns a structured diff showing new errors, changed network responses, performance deltas, and other differences.

#### Behavioral baseline comparison (if baseline feature is available):
```
configure({what: "compare_baseline", name: "stable-baseline"})
```

This returns regression categorization: network regressions (status code changes), timing regressions (latency spikes), console regressions (new errors), and WebSocket regressions.

#### Incremental error checking:

The agent also reads telemetry directly to catch issues that diffing might miss:

```
observe({what: "errors"})
observe({what: "performance"})
observe({what: "accessibility"})
observe({what: "security_audit"})
```

### Phase 5: Report

The agent synthesizes findings into a report. Two output formats:

#### Human-readable PR comment (primary):
```
generate({what: "pr_summary"})
```

The existing `pr_summary` mode already produces markdown with error summaries, network issues, and performance data. For PR preview exploration, the agent augments this by prepending context about what was explored and what was compared.

#### Machine-readable SARIF (optional):
```
generate({what: "sarif"})
```

For teams that consume SARIF in GitHub's code scanning, this provides structured findings that appear inline on the PR diff.

The agent composes the final output, which includes:
- Preview URL explored
- Pages visited and actions taken
- Regressions found (categorized by severity)
- New errors not present in the baseline
- Performance comparison (load times, Web Vitals)
- Accessibility issues
- Reproduction steps for each finding

## Exploration Bounds

To prevent runaway exploration, the workflow enforces explicit bounds. These are not Kaboom server limits — they are agent-side constraints defined in the skill/workflow configuration.

| Bound | Default | Rationale |
|-------|---------|-----------|
| Max pages visited | 20 | Covers primary user flows without exhaustive crawl |
| Max actions per page | 10 | Click, type, scroll — not infinite interaction |
| Max total actions | 100 | Hard ceiling on agent activity |
| Exploration timeout | 15 minutes | Wall-clock time budget |
| Max navigation depth | 3 | Clicks deep from landing page |
| Page load timeout | 30 seconds | Per-page load wait |

If any bound is reached, the agent stops exploration, captures whatever telemetry has been collected, and proceeds to comparison and reporting. Partial exploration is explicitly acceptable — the report notes which areas were explored and which were not.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Agent can navigate to a preview URL using `interact({what: "navigate"})` or `interact({what: "new_tab"})` | must |
| R2 | Agent can capture a baseline session snapshot before exploring the preview | must |
| R3 | Agent can compare preview telemetry against baseline using `diff_sessions` | must |
| R4 | Agent produces a structured findings report via `generate({what: "pr_summary"})` | must |
| R5 | Agent respects exploration bounds (max pages, max actions, timeout) | must |
| R6 | Agent detects and reports new console errors not present in baseline | must |
| R7 | Agent detects and reports network regressions (new 4xx/5xx responses) | must |
| R8 | Agent detects and reports performance regressions (Web Vitals, load times) | should |
| R9 | Agent detects and reports accessibility violations | should |
| R10 | Agent supports preview URL discovery via stored convention template | should |
| R11 | Agent supports behavioral baseline comparison when the feature is available | should |
| R12 | Agent can produce SARIF output for GitHub code scanning integration | could |
| R13 | Agent supports authenticated preview environments via cookie/token injection | could |
| R14 | Agent supports exploration of multiple PR preview URLs in sequence | could |

## Non-Goals

- **This feature does NOT add new MCP tools or modes.** It composes existing primitives. The 5-tool model is inviolate.
- **This feature does NOT implement CI/CD webhook handling.** How the agent gets triggered (webhook, cron, manual) is outside Kaboom's scope. Kaboom provides observation; triggering is the CI system's job.
- **This feature does NOT implement GitHub PR comment posting.** The agent produces the report content; posting it to GitHub is the agent's responsibility via external tools (e.g., `gh pr comment`).
- **This feature does NOT implement visual regression testing.** Kaboom captures behavioral telemetry (errors, network, performance), not pixel-level screenshots. Visual diffing is a separate concern.
- **This feature does NOT crawl the entire application.** Exploration is bounded and focused on key user flows, not exhaustive site mapping.
- **Out of scope: Preview environment provisioning.** Kaboom does not create, manage, or tear down preview environments. It only observes them.

## Performance SLOs

| Metric | Target | Notes |
|--------|--------|-------|
| Navigate to preview | < 2s (Kaboom overhead) | Actual page load depends on preview environment |
| Session capture | < 200ms | Existing `diff_sessions` capture performance |
| Session comparison | < 500ms | Depends on session size |
| Baseline comparison | < 20ms | Existing `compare_baseline` performance |
| PR summary generation | < 300ms | Existing `pr_summary` generation performance |
| Full exploration workflow | < 15 minutes | Agent-side bound, not a Kaboom SLO |

Kaboom's per-operation overhead is small. The dominant cost is page load time and agent reasoning time, both of which are outside Kaboom's control.

## Security Considerations

### Preview URL validation

The agent must only navigate to URLs that are plausible preview environments. Kaboom's `interact({what: "navigate"})` currently accepts any URL with no scheme or domain validation. The agent-side workflow should validate:

- URL uses HTTPS (not HTTP, file://, javascript:, data:, or other schemes)
- URL matches a configured domain pattern (e.g., `*.vercel.app`, `*.netlify.app`, or a custom domain)
- URL does not target internal/private network addresses (no 10.x.x.x, 192.168.x.x, 127.0.0.x other than Kaboom itself)

This validation belongs in the agent workflow, not in Kaboom. Kaboom is a general-purpose browser observation tool; it should not opinionately restrict navigation targets. The skill definition enforces the allowlist.

### Authentication for preview environments

Many preview environments require authentication (basic auth, OAuth, API key). The agent workflow supports two mechanisms:

**Cookie injection:** The agent uses `interact({what: "execute_js", script: "document.cookie = '...'"})` to set authentication cookies before navigating. The cookie value must come from a secure source (CI secret, vault, etc.), never hardcoded in the skill definition.

**Bearer token in URL:** Some preview environments accept a token as a query parameter (e.g., `?token=abc`). The agent appends the token before navigating.

#### Security constraints:
- Auth tokens must never appear in Kaboom's telemetry buffers. Kaboom already strips Authorization headers from network captures. Cookie values set via `execute_js` are not captured in the log buffer (the script is captured, but not its side effects).
- Auth tokens must never appear in the PR comment output. The agent must redact any token values from the exploration report.
- The skill definition must document which secrets are required and how they are injected (environment variable, CI secret, etc.).

### Data sensitivity

Preview environments may contain test data that resembles real user data. Kaboom's existing privacy controls apply:

- Network body capture is opt-in (off by default). The agent should enable it selectively for API endpoints it wants to validate.
- Kaboom strips sensitive headers (Authorization, Cookie, tokens) from captured network requests.
- Redaction patterns (configured via `configure({what: "noise_rule"})`) apply to all captured data.

### execute_js security surface

The exploration phase uses `execute_js` to interact with the page. The principal engineer review correctly identified this as a security surface. Mitigations:

- The agent generates scripts that simulate user behavior (clicking, typing, scrolling). It does not read sensitive page data (localStorage, sessionStorage, form values).
- The skill definition should include a guidance prompt that restricts `execute_js` usage to interaction-only patterns.
- Kaboom's AI Web Pilot toggle must be enabled by the human before `execute_js` works. This is an existing security gate — the human explicitly opts in to browser control.

## Edge Cases

- **Preview is not yet deployed.** The preview environment may not be ready when the agent starts. Expected behavior: The agent polls the URL (or GitHub deployment status) with exponential backoff, up to 5 minutes. If still not available, report "preview not ready" and stop.

- **Preview returns 404 or 500.** The preview environment is deployed but broken at the root. Expected behavior: The agent captures the error, reports it immediately ("preview environment returns HTTP 500 at root URL"), and stops exploration. This itself is a useful finding.

- **Preview requires authentication the agent does not have.** Expected behavior: The agent detects a redirect to a login page (or 401/403 response), reports "preview requires authentication — provide credentials or configure auth in the skill", and stops.

- **Extension is disconnected.** All interact and observe calls depend on the browser extension. Expected behavior: The agent checks extension connectivity via `observe({what: "page"})` before starting. If the extension disconnects mid-exploration, tool calls return timeout errors. The agent captures partial results and reports what it found.

- **Buffers overflow during exploration.** Exploring many pages generates high event volume. The log buffer (1000 entries) and network body buffer (100 entries) may evict early entries. Expected behavior: The agent processes telemetry incrementally — after each page visit, it reads errors and network data before moving on. This prevents early findings from being evicted.

- **Multiple concurrent explorations.** Two agents explore two different PRs simultaneously. Expected behavior: Each agent uses a unique session name (e.g., `preview-pr-42`, `preview-pr-43`) for `diff_sessions`. Session captures are scoped by name and do not conflict. However, the shared telemetry buffers (logs, network) are not per-session — concurrent explorations will see each other's data. This is a known limitation. Mitigation: Single-tab tracking isolation (shipped in v6) ensures each agent's telemetry is scoped to its tracked tab.

- **Preview URL changes mid-exploration.** Some preview environments use dynamic URLs that change on redeployment. Expected behavior: The agent navigates to the URL once at the start. If the URL becomes invalid mid-exploration (page errors), the agent stops and reports partial results.

- **No baseline available.** The agent cannot capture or load a baseline (first run, stable branch unavailable). Expected behavior: The agent skips the comparison phase and reports absolute findings only (errors found, performance numbers, accessibility violations). The report notes "no baseline available — showing absolute findings, not regressions."

## Dependencies

- **Depends on:**
  - `interact` tool: `navigate`, `new_tab`, `execute_js`, `refresh` actions (shipped)
  - `observe` tool: `errors`, `network_waterfall`, `performance`, `accessibility`, `page` modes (shipped)
  - `configure` tool: `diff_sessions`, `store`/`load` actions (shipped)
  - `generate` tool: `pr_summary`, `sarif` formats (shipped)
  - Behavioral baselines: `save_baseline`, `compare_baseline` (in-progress, optional dependency)
  - Single-tab tracking isolation (shipped in v6): ensures per-tab telemetry scoping
  - AI Web Pilot toggle (shipped): human opt-in required for `execute_js`

- **Depended on by:**
  - Agentic CI/CD (proposed): PR preview exploration is one of the four agentic workflows
  - Deployment Watchdog (proposed): shares the baseline comparison pattern

## Assumptions

- A1: The browser extension is connected and tracking the tab used for preview exploration.
- A2: The AI Web Pilot toggle is enabled (human has opted in to browser control).
- A3: The preview environment is accessible from the machine running the browser (localhost or network-accessible).
- A4: The Kaboom server is running and accepting MCP tool calls.
- A5: The agent has sufficient context window to hold the exploration report plus the PR diff (for correlation).
- A6: Preview environments serve a web application that can be meaningfully explored via browser interactions (not a raw API, not a native app).

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should Kaboom add URL scheme validation to `interact({what: "navigate"})`? | open | The current implementation accepts any URL. The PR preview review recommended scheme/domain allowlists. Question: should this be in Kaboom (all users benefit) or in the agent skill (more flexible)? Recommendation: add basic scheme validation (block `javascript:`, `data:`, `file://`) in Kaboom, leave domain allowlists to the skill. |
| OI-2 | How should the agent handle preview environments behind SSO/OAuth? | open | Cookie injection works for simple auth. OAuth flows require multi-step browser interaction that may exceed `execute_js` capabilities. May need a dedicated auth flow pattern in the skill definition. |
| OI-3 | Should the exploration report include a confidence score? | open | The agent could rate its confidence in findings based on exploration coverage (pages visited vs total pages, actions taken vs available actions). This helps reviewers assess how thorough the exploration was. Risk: false confidence from arbitrary scoring. |
| OI-4 | Should Kaboom add a "scan page" meta-action to `interact`? | open | The principal engineer review suggested this for performance. A single call that returns all interactive elements (links, buttons, inputs) would reduce the number of `execute_js` calls needed for exploration. This would be a new action under `interact`, not a new tool. |
| OI-5 | How should baseline staleness be handled? | open | If a stored baseline is 7 days old, is it still useful for comparison? The agent could warn on staleness, or automatically recapture. Default threshold proposal: 24 hours, configurable via persistent memory. |
| OI-6 | Should incremental telemetry processing be a Kaboom feature or agent-side logic? | open | Processing errors per-page (to avoid buffer eviction) could be implemented as agent-side polling, or as a Kaboom mode that returns "errors since last read" (like a cursor). Agent-side is simpler and requires no Kaboom changes. |
| OI-7 | What is the interaction model for the developer? | open | Options: (a) Developer explicitly triggers exploration ("explore my preview at URL"), (b) Exploration is triggered automatically by CI webhook, (c) Agent proactively offers to explore when it detects a preview URL in the conversation. All three should eventually be supported, but which is the v1 default? Recommendation: (a) for v1, with (b) as a fast-follow. |
