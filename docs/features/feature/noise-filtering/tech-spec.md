---
status: proposed
scope: feature/noise-filtering/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-noise-filtering
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-noise-filtering.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Noise Filtering Review](noise-filtering-review.md).

# Technical Spec: Noise Filtering

## Purpose

Browsers are noisy. Between extension console warnings, favicon 404s, analytics pings, HMR messages, CORS preflights, and React DevTools suggestions, a typical page session generates dozens of entries that have nothing to do with the application being developed. An AI agent reading raw buffers wastes tokens on noise and may hallucinate bugs that are actually just browser overhead.

Noise filtering lets the server classify and suppress these irrelevant entries so that diffs, health checks, and buffer reads only contain real application signals.

---

## How It Works

The server maintains a list of **noise rules**, each specifying a pattern to match and a classification for why it's noise (e.g., "extension", "framework", "analytics"). Rules match against console messages, network URLs, or WebSocket URLs using regex patterns.

There are two layers:

1. **Built-in heuristics** — a fixed set of ~13 rules that ship with the server and are always active. These cover the most common browser noise: chrome-extension:// sources, favicon.ico requests, HMR/Vite/Webpack messages, React DevTools prompts, CORS preflight OPTIONS requests, and analytics domains.

2. **User/agent rules** — rules added by the AI agent during a session (via `configure_noise` or `dismiss_noise`), or rules detected automatically by the `auto_detect` action.

When any other tool reads from the buffers (like `get_changes_since` or `get_console_logs`), it calls the noise filter's matching functions. Entries that match a noise rule are excluded from the response.

All rules are pre-compiled: regex patterns are compiled once when rules change, and matching is a linear scan of the compiled set. This keeps per-entry matching well under 0.1ms even with 100 rules.

---

## Data Model

### Noise Rule

A noise rule has:
- A unique ID (prefixed with "builtin_" for built-ins, "user_" for manual, "auto_" for auto-detected, "dismiss_" for quick dismissals)
- A category: "console", "network", or "websocket"
- A match specification containing one or more of: message regex pattern, source URL regex, network URL regex, HTTP method, status code range, and console level
- A classification explaining what type of noise it is: "extension", "framework", "cosmetic", "analytics", "infrastructure", "repetitive", or "dismissed"
- Whether it was auto-detected
- A creation timestamp
- An optional reason string (for dismissed patterns)

### Built-in Rules (Global)

The server ships with ~50 always-active rules organized by category. These represent universal browser noise that applies regardless of project. They cannot be removed.

#### Browser internals:
- Console messages from chrome-extension:// or moz-extension:// sources
- Network requests to favicon.ico
- Source map 404s (.map files returning 4xx)
- CORS preflight (OPTIONS 2xx)
- Service worker lifecycle info messages
- "Added non-passive event listener" warnings
- [Deprecation] warnings
- "DevTools failed to load source map" messages
- net::ERR_BLOCKED_BY_CLIENT (ad blockers)
- "Indicate whether to send a cookie" SameSite warnings

#### Dev tooling:
- Console messages starting with [HMR], [vite], [webpack], or [next]
- Network requests to HMR/Vite/webpack hot-update URLs (__vite_ping, *.hot-update.json, sockjs-node, _next/webpack-hmr)
- React DevTools download prompts
- "Download the React DevTools" message
- Angular debug mode messages
- Vue.js devtools connection messages
- Svelte HMR messages

#### Analytics & tracking:
- Network requests to: Google Analytics (google-analytics.com, analytics.google.com), Segment (api.segment.io, cdn.segment.com), Mixpanel (api.mixpanel.com), Hotjar (*.hotjar.com), Amplitude (api.amplitude.com), Plausible (plausible.io), PostHog (app.posthog.com), Datadog RUM (rum.browser-intake-*.datadoghq.com), Sentry (*.ingest.sentry.io), LogRocket, FullStory, Heap

#### Framework noise (activated by framework detection):
- React: "Warning: Each child in a list should have a unique key", strict mode double-render logs, "Cannot update a component while rendering a different component"
- Next.js: "next-dev.js" internal messages, hydration mismatch warnings (info-level only, not errors), Fast Refresh messages
- Vite: plugin resolution messages, dependency pre-bundling logs
- Angular: "Angular is running in development mode"
- Create React App: "The development server has disconnected"

### Framework Detection

The server auto-detects the project's framework by scanning network requests and page scripts for signatures:

| Framework | Detection signals |
|-----------|------------------|
| React | `react.development.js` in scripts, `__REACT_DEVTOOLS_GLOBAL_HOOK__` in console sources |
| Next.js | `/_next/` URL prefix, `__NEXT_DATA__` in page |
| Vite | `/@vite/client` in scripts, `__vite_ping` requests |
| Webpack | `webpack-dev-server` in requests, `webpackHotUpdate` in scripts |
| Angular | `angular.js` or `@angular/core` in scripts |
| Svelte | `svelte-hmr` in requests |
| Vue | `vue.js` or `@vue` in scripts |

Framework-specific rules are only active when the framework is detected. Detection runs once on the first batch of captured data and caches the result for the session.

These cannot be removed.

### Noise Statistics

The server tracks how many entries have been filtered in total, broken down by rule ID, and when the last real signal and last noise entry were seen.

---

## Tool Interface

### `configure_noise`

**Parameters**:
- `action` (required): One of "add", "remove", "list", "reset", or "auto_detect"
- `rules`: Array of rule objects (for action "add")
- `rule_id`: ID of rule to remove (for action "remove")

**Behavior by action**:
- **add**: Adds the provided rules to the config. Each gets a unique ID and timestamp. Recompiles patterns.
- **remove**: Removes the rule with the given ID, unless it's a built-in (built-ins are protected). Recompiles.
- **list**: Returns all current rules and statistics.
- **reset**: Removes all user/auto rules, reverting to only built-ins. Recompiles.
- **auto_detect**: Analyzes current buffers and proposes new noise rules based on frequency and source analysis. High-confidence proposals (≥0.9) are automatically applied. Lower-confidence ones are returned as suggestions for the agent to review.

### `dismiss_noise`

A convenience shortcut for quickly dismissing a pattern without constructing a full rule object.

**Parameters**:
- `pattern` (required): Regex pattern to dismiss
- `category`: Which buffer it applies to (default: "console")
- `reason`: Why this is noise (for the audit trail)

Creates a rule with classification "dismissed" and applies it immediately.

---

## Matching Behavior

### Console entries

An entry is noise if:
1. Any rule with category "console" matches, AND
2. The rule's level filter (if set) matches the entry's level, AND
3. Either the message regex matches the entry's message text, OR the source regex matches the entry's source URL

Both message and source patterns are checked — matching either one is sufficient.

### Network entries

A network body is noise if:
1. Any rule with category "network" matches, AND
2. The rule's method filter (if set) matches the request method, AND
3. The status code falls within the rule's status range (if set), AND
4. The URL regex matches the request URL

### WebSocket entries

A WebSocket event is noise if any rule with category "websocket" has a URL regex that matches the event URL.

---

## Auto-Detection

The `auto_detect` action scans current buffers looking for noise patterns using four heuristics:

1. **Frequency analysis** — Console messages that repeat 10+ times (after fingerprinting by stripping numbers/hashes) are candidates. Confidence scales with count: 0.7 base + count/100, capped at 0.99.

2. **Source analysis** — Console entries from chrome-extension://, moz-extension://, or node_modules paths are flagged with 0.85 confidence.

3. **Periodicity detection** — Network requests or WebSocket messages arriving at regular intervals (±10% jitter tolerance) are flagged as infrastructure. Examples: health checks every 10s, analytics pings every 30s, keepalive every 60s. Confidence: 0.8 for clear periodicity (≥3 intervals observed). The server tracks inter-arrival times per URL path and flags paths where the standard deviation of intervals is less than 10% of the mean.

4. **Entropy scoring** — Console messages are scored by information content. Messages that are mostly static (low unique token ratio after fingerprinting) score low entropy. A message like "Rendering component X" repeated with different component names has higher entropy than "[HMR] Updated 1 modules" which is nearly identical each time. Low-entropy messages (score < 0.3) with 5+ occurrences are flagged with 0.75 confidence.

5. **Network frequency** — URL paths hit 20+ times that look like infrastructure (/health, /ping, /ready, /__*, /sockjs-node, /ws) are flagged with 0.8 confidence.

Rules with confidence ≥ 0.9 are automatically applied. All proposals are returned to the agent regardless so it can review and add lower-confidence ones manually. The AI agent is the final decision-maker for ambiguous cases.

Auto-detection skips patterns already covered by existing rules to avoid duplicates.

---

## Security Invariants

These are non-negotiable safety rules:

1. **Auth responses are never noise** — Any network entry with status 401 or 403 is never classified as noise, regardless of rules.
2. **Error-level from application sources are never auto-detected** — Only user-explicit dismissal can suppress console errors from application code.
3. **Security audit ignores filtering** — If a `security_audit` tool is added later, it always reads raw unfiltered buffers.
4. **Audit trail** — Every rule records who created it (auto vs. agent) and when, so the agent can review what's being hidden.

---

## Edge Cases

- Invalid regex in a rule pattern: the rule is silently skipped during compilation (nil regex means it never matches). No panic.
- Max 100 rules total (built-in + user + auto). Additional rules are silently dropped.
- Concurrent access: noise config has its own RWMutex. Reads (matching) take a read lock. Writes (add/remove/compile) take a write lock.
- Reset preserves built-in rules and only removes user/auto rules.

---

## Performance Constraints

- Matching a single entry against all rules: under 0.1ms (pre-compiled regex, linear scan)
- Recompiling all patterns: under 5ms (only happens on rule changes)
- Auto-detection full buffer scan: under 50ms
- Memory for 100 rules: under 50KB
- Memory for compiled regex objects: under 100KB

---

## Test Scenarios

1. NewNoiseConfig has all built-in rules present
2. Chrome extension source → correctly identified as noise
3. Application error from localhost:3000 → not noise
4. Favicon 404 → noise
5. API endpoint 500 → not noise (real failure)
6. Segment analytics URL → noise
7. OPTIONS 204 preflight → noise
8. [vite] hot update message → noise
9. React DevTools download prompt → noise
10. Adding a custom rule → new matches are filtered
11. Removing a custom rule → entries no longer filtered
12. Cannot remove built-in rules → they remain after removal attempt
13. Max rules reached → additional rules silently dropped
14. Reset → only built-ins remain
15. Auto-detect with 15 identical messages → proposed rule with confidence > 0.7
16. Auto-detect doesn't duplicate existing rules
17. High-confidence auto-detected rules are automatically applied
18. dismiss_noise defaults to console category
19. dismiss_noise with network category sets URL pattern
20. Statistics track filtered counts per rule
21. Invalid regex pattern → rule skipped, no panic
22. Concurrent read/write → no race conditions
23. Auth-related entries (401 response) → never filtered

---

## File Location

Implementation lives in `internal/noise/*.go` and tool integration is in `cmd/browser-agent/tools_configure_noise_actions.go` with tests in `internal/noise/*_test.go` and `cmd/browser-agent/tools_configure_noise_actions_test.go`.
