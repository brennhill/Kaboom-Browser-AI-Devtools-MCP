---
title: "Noise Filtering"
description: "Keep your AI focused on real issues. Auto-detect browser noise, add manual rules, filter extension errors and analytics failures, and manage noise rules across sessions."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'noise', 'filtering']
---

Every browser tab generates noise — extension errors, analytics script failures, framework warnings, Content Security Policy violations from third-party scripts. Without filtering, your AI spends time investigating false leads instead of real bugs.

KaBOOM's noise filtering lets you suppress irrelevant errors so the AI only sees what matters.

## Auto-Detect Noise

The fastest way to clean up:

```js
configure({what: "noise_rule", noise_action: "auto_detect"})
```

KaBOOM scans current errors and identifies patterns that are likely noise:
- Browser extension errors (`chrome-extension://`, `moz-extension://`)
- Analytics failures (Google Analytics, Segment, Mixpanel)
- Framework development warnings (React, Angular, Vue dev-mode messages)
- Third-party script errors from ad networks and trackers
- CSP violations from injected scripts

Auto-detect creates rules automatically. Review them to make sure nothing important was filtered:

```js
configure({what: "noise_rule", noise_action: "list"})
```

---

## Manual Rules

### Add a Rule

```js
configure({what: "noise_rule",
           noise_action: "add",
           pattern: "analytics\\.google",
           category: "console",
           reason: "Google Analytics noise"})
```

| Parameter | Description |
|-----------|-------------|
| `pattern` | Regex pattern to match against error messages |
| `category` | Which buffer to filter: `console`, `network`, or `websocket` |
| `reason` | Human-readable explanation (helps when reviewing rules later) |

### Pattern Examples

| What to Filter | Pattern | Category |
|---------------|---------|----------|
| Google Analytics | `analytics\\.google` | `network` |
| Facebook Pixel | `facebook\\.com\\/tr` | `network` |
| React dev warnings | `Warning: Each child in a list` | `console` |
| Angular dev mode | `Angular is running in development mode` | `console` |
| Browser extensions | `chrome-extension://` | `console` |
| Hot reload noise | `\\[HMR\\]` | `console` |
| Source map warnings | `DevTools failed to load source map` | `console` |
| Favicon 404 | `favicon\\.ico` | `network` |
| Service worker updates | `service-worker\\.js` | `network` |

### Batch Add Rules

Add multiple rules at once:

```js
configure({what: "noise_rule",
           noise_action: "add",
           rules: [
             {pattern: "analytics\\.google", category: "network", reason: "Analytics"},
             {pattern: "facebook\\.com\\/tr", category: "network", reason: "Facebook Pixel"},
             {pattern: "\\[HMR\\]", category: "console", reason: "Hot module reload"},
             {pattern: "chrome-extension://", category: "console", reason: "Browser extensions"}
           ]})
```

---

## Managing Rules

### List All Rules

```js
configure({what: "noise_rule", noise_action: "list"})
```

Returns every active rule with its ID, pattern, category, and reason.

### Remove a Rule

If you filtered something that turned out to be important:

```js
configure({what: "noise_rule", noise_action: "remove", rule_id: "rule-123"})
```

Get the rule ID from the list output.

### Reset All Rules

Start fresh:

```js
configure({what: "noise_rule", noise_action: "reset"})
```

Removes all noise rules. Useful when switching projects or when you've over-filtered.

---

## Workflow: Clean Session Setup

When starting a new debugging session:

### 1. Check Current Errors

```
"What browser errors do you see?"
```

```js
observe({what: "errors"})
```

### 2. Auto-Detect Noise

```
"Auto-detect noise and filter it out."
```

```js
configure({what: "noise_rule", noise_action: "auto_detect"})
```

### 3. Review What Was Filtered

```
"Show me the noise rules — make sure nothing important got filtered."
```

```js
configure({what: "noise_rule", noise_action: "list"})
```

### 4. Check Cleaned Errors

```
"Now show me the errors again."
```

```js
observe({what: "errors"})
```

The error list now contains only your application's real issues.

### 5. Add Project-Specific Rules

If your project has known noisy patterns:

```
"Also filter out hot reload messages and favicon 404s."
```

```js
configure({what: "noise_rule", noise_action: "add",
           rules: [
             {pattern: "\\[HMR\\]", category: "console", reason: "Dev server noise"},
             {pattern: "favicon\\.ico", category: "network", reason: "Missing favicon"}
           ]})
```

---

## Categories

Noise rules filter different buffers:

| Category | What It Filters | Examples |
|----------|----------------|---------|
| `console` | Console logs, errors, and warnings | Extension errors, framework warnings, dev-mode messages |
| `network` | Network requests and responses | Analytics calls, tracking pixels, CDN errors |
| `websocket` | WebSocket events | Heartbeat messages, ping/pong noise |

A rule in one category doesn't affect the others. An `analytics.google` rule in the `network` category won't filter a `console.error` that mentions Google Analytics — you'd need a separate `console` rule for that.

---

## Tips

**Run auto-detect on a fresh page load.** The first load generates the most noise — extensions initialize, analytics fire, framework warnings appear. Auto-detect catches the most patterns if you run it immediately after loading.

**Keep reasons descriptive.** When you review rules weeks later, "Google Analytics noise" is more useful than "network filter 3." Good reasons also help the AI understand why something is filtered.

**Don't over-filter.** If you're not sure whether something is noise, leave it unfiltered. It's better to investigate a false lead than to miss a real error.

**Filter by category, not globally.** A pattern like `error` in the console category would filter too broadly. Be specific with patterns — match the exact string or URL that generates the noise.

**Reset when switching projects.** Noise rules are session-scoped. If you switch from a React app to a Vue app, the framework warnings change. Reset and re-detect.
