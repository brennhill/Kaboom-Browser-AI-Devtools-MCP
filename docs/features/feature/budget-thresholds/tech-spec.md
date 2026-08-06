---
status: shipped
scope: feature/budget-thresholds/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-budget-thresholds
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-budget-thresholds.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Budget Thresholds Review](./index.md).

# Technical Spec: Budget Thresholds as Config

## Purpose

Today, performance regression detection uses relative thresholds (">20% slower than baseline"). This works for detecting changes but doesn't enforce absolute limits. A page that started at 100ms load time and regressed to 150ms is flagged as a 50% regression — but it's still fast. Meanwhile, a page that was always 4 seconds and stays at 4 seconds is never flagged — even though it's unacceptably slow.

Budget thresholds let the developer declare absolute performance requirements: "load time must be under 2 seconds, bundle size must be under 500KB, CLS must be under 0.1." The AI enforces these continuously. If any budget is exceeded, the AI knows immediately — not relative to a baseline that might itself be unacceptable, but relative to a hard limit the team has agreed upon.

This makes the AI an enforcer of team standards, not just a detector of relative changes.

---

## Opportunity & Business Value

**Declarative performance contracts**: Teams can codify performance expectations in a config file that lives in version control. New developers, new AI agents, and CI systems all read the same budgets. No tribal knowledge needed.

**Lighthouse CI alignment**: Lighthouse CI uses a similar concept (performance budgets in JSON). Kaboom's config format is compatible — teams that already have Lighthouse budgets can reference the same numbers. The AI enforces locally what CI enforces remotely, catching issues before they reach the pipeline.

**Webpack/Vite bundle size budgets**: Bundlers have their own size warnings (`performance.maxAssetSize` in webpack). Kaboom's budgets complement these by measuring the actual transfer size (compressed, over the network) rather than the raw asset size. Transfer size is what users experience.

**Web Vitals thresholds built in**: Google's good/needs-improvement/poor thresholds are universal. A `.kaboom.json` file can reference "web-vitals-good" as a preset and automatically get FCP ≤1.8s, LCP ≤2.5s, CLS ≤0.1, INP ≤200ms — no manual configuration needed.

**Per-route budgets**: SPAs have routes with different performance characteristics. The dashboard with charts might have a 3s budget while the login page needs to load in under 1s. Per-route budgets let teams set appropriate limits for each part of their app.

**AI self-governance**: When the AI adds a large library to fix a bug, the budget system immediately flags "bundle size budget exceeded by 120KB." The AI can then choose to use a lighter alternative without human intervention. This is proactive self-correction.

---

## How It Works

### Config File

Performance budgets are defined in `.kaboom.json` at the project root (or `.kaboom/budgets.json` if the project uses the `.kaboom/` directory). The file is optional — without it, only relative regression detection applies.

### Config Format

```json
{
  "budgets": {
    "default": {
      "load_ms": 2000,
      "fcp_ms": 1800,
      "lcp_ms": 2500,
      "cls": 0.1,
      "inp_ms": 200,
      "ttfb_ms": 800,
      "total_transfer_kb": 500,
      "script_transfer_kb": 300,
      "image_transfer_kb": 200
    },
    "routes": {
      "/login": {
        "load_ms": 1000,
        "total_transfer_kb": 200
      },
      "/dashboard": {
        "load_ms": 3000,
        "lcp_ms": 3500,
        "total_transfer_kb": 800
      }
    },
    "presets": ["web-vitals-good"]
  }
}
```

### Config Resolution

When checking a URL against budgets:
1. Check if the URL matches a route-specific budget (longest prefix match)
2. Fall back to the `default` budget
3. For any metric not specified in the matching budget, fall back to presets
4. For any metric not in presets, no budget applies (only relative regression detection)

### Presets

Built-in presets provide common thresholds:

**`web-vitals-good`**: Google's "good" thresholds
- FCP: 1800ms, LCP: 2500ms, CLS: 0.1, INP: 200ms

**`web-vitals-needs-improvement`**: More lenient (the boundary between "needs improvement" and "poor")
- FCP: 3000ms, LCP: 4000ms, CLS: 0.25, INP: 500ms

**`performance-budget-default`**: Reasonable defaults for a modern web app
- Load: 3000ms, TTFB: 600ms, total transfer: 1MB, scripts: 500KB

Presets are referenced by name and their thresholds are merged (config values override preset values for the same metric).

### Budget Enforcement

The server loads the config file on startup (and watches for changes via file modification time, checked every 30 seconds). When a performance snapshot arrives:

1. Resolve the applicable budget for the snapshot's URL
2. Compare each metric against its budget threshold
3. If any metric exceeds its budget, generate a budget violation alert
4. Budget violations are included in the `performance_alerts` section of `get_changes_since` (alongside regression alerts)

### Budget Violation Alert

Similar to regression alerts, but with a different `type`:

```json
{
  "type": "budget_exceeded",
  "url": "/dashboard",
  "detected_at": "2026-01-24T10:30:05Z",
  "violations": [
    { "metric": "total_transfer_kb", "budget": 500, "actual": 620, "over_by": "120KB (24%)" },
    { "metric": "lcp_ms", "budget": 2500, "actual": 2800, "over_by": "300ms (12%)" }
  ],
  "summary": "2 budget violations on /dashboard: transfer size +120KB, LCP +300ms over budget.",
  "recommendation": "Reduce bundle size or increase budget in .kaboom.json"
}
```

### MCP Tool: `check_budgets`

A dedicated tool for explicit budget checking (in addition to automatic alerts).

**Parameters**:
- `url` (optional): Specific URL to check. If omitted, checks all URLs with recent snapshots.
- `show_passing` (optional, boolean): Include metrics within budget (default: only violations).

**Response**:
```
{
  "config_file": ".kaboom.json",
  "urls_checked": 3,
  "violations": [...],
  "passing": [...],  // if show_passing=true
  "summary": "1 of 3 routes exceeds budget. /dashboard over on transfer_kb and lcp."
}
```

---

## Data Model

### Budget Configuration

Loaded from disk on server start:
- Default thresholds (map of metric name → value)
- Route-specific overrides (map of route pattern → thresholds)
- Active presets (list of preset names)
- Resolved thresholds per route (computed once on load, re-computed on config change)

### Budget Violation

Stored alongside regression alerts:
- URL that violated
- List of violated metrics with budget value, actual value, and delta
- Detection timestamp
- Whether the violation is new (first time) or persistent (seen before)

### Config Watch

The server checks the config file's modification time every 30 seconds. If changed:
1. Re-read and parse the file
2. Re-resolve all route budgets
3. Log "Budget configuration updated" (visible in server logs)
4. Re-check all current snapshots against new budgets (may generate new violations or clear old ones)

---

## Edge Cases

- **No config file**: Budget enforcement is disabled. Only relative regression detection applies. The `check_budgets` tool returns "No budget configuration found."
- **Invalid JSON in config**: Server logs a warning and continues without budgets. Previous valid config (if any) is NOT retained — invalid config means no budgets.
- **Config file appears mid-session**: Picked up on the next 30-second check. All existing snapshots are re-evaluated against new budgets.
- **Route pattern matching**: Patterns are matched as URL prefixes. `/dashboard` matches `/dashboard`, `/dashboard/charts`, `/dashboard/charts/revenue`. The most specific (longest) match wins.
- **Budget of 0**: Treated as "no budget for this metric" (0 is not a valid threshold). Use a very small positive value (e.g., 1) to express "this metric must be near-zero."
- **Multiple presets**: Merged in order specified. Later presets override earlier ones for the same metric.
- **Unknown metric name in config**: Silently ignored (forward-compatible with future metrics).
- **Budget exceeded on first load (no baseline yet)**: Alert generated immediately. Budgets are absolute — they don't need a baseline.

---

## Performance Constraints

- Config file read: under 5ms (small JSON file, typically <1KB)
- Budget evaluation per snapshot: under 0.1ms (compare ~10 numbers)
- Config watch overhead: under 1ms every 30 seconds (stat() call)
- Memory for config: under 10KB (route map + thresholds)

---

## Test Scenarios

1. Metric within budget → no violation
2. Metric exceeds budget → violation alert generated
3. Route-specific budget overrides default
4. Longest prefix match for route resolution
5. Preset "web-vitals-good" provides correct thresholds
6. Config value overrides preset for same metric
7. No config file → no budget enforcement
8. Invalid config → warning logged, no budgets active
9. Config change detected → re-evaluated within 30 seconds
10. Budget violation in `get_changes_since` response
11. `check_budgets` returns all violations
12. `show_passing` includes within-budget metrics
13. Multiple violations on same URL listed together
14. Budget of 0 → treated as no budget
15. Unknown metric name silently ignored
16. First load exceeds budget → immediate alert (no baseline needed)
17. Violation cleared when metric improves below budget
18. Multiple presets merged correctly

---

## File Locations

Server implementation: `cmd/browser-agent/budgets.go` (config loading, evaluation, MCP tool).

Config file: `.kaboom.json` or `.kaboom/budgets.json` at project root.

Tests: `cmd/browser-agent/budgets_test.go`.
