---
status: proposed
scope: feature/web-vitals/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-web-vitals
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-web-vitals.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Web Vitals Review](web-vitals-review.md).

# Technical Spec: Web Vitals Capture

## Purpose

Google's Core Web Vitals — FCP, LCP, CLS, and INP — are the industry-standard metrics for measuring user-perceived page performance. They determine search ranking, they're what Lighthouse measures, and they're what developers talk about when discussing "is my page fast?"

Kaboom already captures FCP, LCP, and CLS via PerformanceObservers, but they're buried inside performance snapshots as implementation details. They're not exposed as first-class metrics the AI can reason about independently, and INP (Interaction to Next Paint, the metric that replaced FID in March 2024) isn't captured at all.

Web vitals capture promotes these metrics to a dedicated MCP tool with proper semantics: the AI can ask "what are the web vitals for this page?" and get a response that matches what Google's tools would report, using the same measurement methodology and the same good/needs-improvement/poor thresholds.

---

## Opportunity & Business Value

**Industry-standard language**: When the AI reports "LCP is 2.8s (poor)", it speaks the same language as Lighthouse, PageSpeed Insights, and Chrome User Experience Report (CrUX). Developers instantly understand the severity without needing to interpret raw milliseconds.

**Search ranking correlation**: Core Web Vitals directly affect Google Search ranking. An AI that monitors these during development and alerts on threshold crossings helps developers maintain SEO without production monitoring tools.

**INP fills the interaction gap**: The existing performance budget monitor measures load performance. INP measures responsiveness — how long the browser takes to respond to user interactions (clicks, taps, key presses). Without INP, the AI has no visibility into janky interactions that frustrate users but don't affect load time.

**Interoperability with Google's ecosystem**: The thresholds (good: <X, poor: >Y) and measurement methodology match Google's web-vitals library exactly. Reports from Kaboom are directly comparable to Lighthouse scores, CrUX data, and Web Vitals Chrome Extension readings. Teams using these tools get consistent numbers.

**Continuous measurement vs. one-shot audits**: Lighthouse runs once and gives you a snapshot. Kaboom captures web vitals continuously across every page load during development. The AI builds baselines over multiple loads and detects trends — "LCP has been creeping up over the last 5 reloads."

---

## How It Works

### Metric Capture

The extension captures all four Core Web Vitals using PerformanceObservers, following Google's measurement methodology:

**FCP (First Contentful Paint)**: Already captured. Time from navigation to the first DOM content painted (text, image, SVG, non-white canvas).

**LCP (Largest Contentful Paint)**: Already captured. Time from navigation to the largest visible content element rendered. Updated throughout loading — the final value is the one reported (last entry in the observer).

**CLS (Cumulative Layout Shift)**: Already captured. Sum of all unexpected layout shift scores, excluding shifts that occur within 500ms of user input.

**INP (Interaction to Next Paint)**: NEW. The worst interaction latency observed during the page's lifetime (or the 98th percentile if more than 50 interactions occur). Measured via the `event` performance observer type, tracking processing time + presentation delay for click, keydown, and pointerdown events.

### INP Implementation

INP requires observing every user interaction and tracking its processing time:

1. Register a PerformanceObserver for type `event` with `durationThreshold: 16` (captures all interactions slower than one frame)
2. For each entry, record: `entry.duration` (total input delay + processing + presentation)
3. Maintain a sorted list of interaction durations
4. Report the worst one (if ≤50 interactions) or the 98th percentile (if >50)
5. Group entries by `interactionId` — a single interaction (e.g., pointerdown + pointerup + click) counts as one interaction, using the longest duration

### Threshold Classification

Each metric is classified using Google's thresholds:

| Metric | Good | Needs Improvement | Poor |
|--------|------|-------------------|------|
| FCP | ≤1.8s | 1.8–3.0s | >3.0s |
| LCP | ≤2.5s | 2.5–4.0s | >4.0s |
| CLS | ≤0.1 | 0.1–0.25 | >0.25 |
| INP | ≤200ms | 200–500ms | >500ms |

### MCP Tool: `get_web_vitals`

A dedicated tool that returns the current page's web vitals in a structured, AI-friendly format.

**Parameters**:
- `include_history` (optional, boolean): If true, includes vitals from previous page loads in this session (up to 10).

**Response**:
```
{
  "url": "/dashboard",
  "timestamp": "2026-01-24T10:30:05Z",
  "vitals": {
    "fcp": { "value_ms": 820, "rating": "good", "threshold": { "good": 1800, "poor": 3000 } },
    "lcp": { "value_ms": 2100, "rating": "good", "threshold": { "good": 2500, "poor": 4000 } },
    "cls": { "value": 0.05, "rating": "good", "threshold": { "good": 0.1, "poor": 0.25 } },
    "inp": { "value_ms": 180, "rating": "good", "threshold": { "good": 200, "poor": 500 }, "interaction_count": 12, "worst_target": "button.submit-form" }
  },
  "overall_rating": "good",
  "summary": "All Core Web Vitals pass. INP is close to threshold (180ms / 200ms good limit)."
}
```

The `overall_rating` is the worst individual rating (one "poor" makes the page "poor").

The `summary` is a one-sentence AI-friendly interpretation highlighting the most actionable insight.

### INP Attribution

When INP is above the "good" threshold, the response includes attribution data to help the AI identify the slow interaction:

- `worst_target`: CSS selector of the element that triggered the slowest interaction
- `worst_type`: Event type (click, keydown, pointerdown)
- `worst_processing_ms`: How long the event handler took (excluding input delay and presentation)
- `worst_delay_ms`: Input delay (time between event dispatch and handler start)
- `worst_presentation_ms`: Time from handler completion to next paint

This breakdown tells the AI whether the problem is a slow handler (optimize the code), input delay (reduce main thread blocking), or presentation (reduce DOM mutations after interaction).

---

## Data Model

### Vital Entry

Each page load produces a vitals entry:
- URL
- Capture timestamp
- FCP value (ms) — finalized after paint observer fires
- LCP value (ms) — finalized when page visibility changes or user interacts (LCP stops reporting after interaction)
- CLS value — accumulated until page unload or visibility change
- INP value (ms) — updated with each new interaction, finalized at page unload
- INP attribution: target selector, event type, processing/delay/presentation breakdown
- Interaction count (for INP percentile calculation)
- Rating per metric and overall

### Vitals History

The server stores the last 10 vitals entries per URL path. This enables:
- Trend detection ("LCP is getting worse across reloads")
- Baseline comparison ("INP was 80ms before your change, now it's 350ms")
- Statistical confidence ("After 5 loads, LCP averages 2.1s ± 200ms")

---

## Extension Changes

### inject.js

New observer for INP:

```
const eventObserver = new PerformanceObserver((list) => {
  for (const entry of list.getEntries()) {
    recordInteraction(entry)
  }
})
eventObserver.observe({ type: 'event', durationThreshold: 16, buffered: true })
```

The `recordInteraction` function groups entries by `interactionId`, takes the longest duration per group, and maintains the sorted interaction list for percentile calculation.

LCP finalization: Register a listener for `visibilitychange` and first user input to stop updating LCP (per Google's methodology — LCP is only valid until the user interacts or the tab goes to background).

CLS session windowing: The existing CLS calculation is correct (sums all shifts without recent input). No change needed.

### Performance Snapshot Enhancement

The existing performance snapshot POST is extended with an `inp` field and proper LCP/CLS finalization flags. The server knows whether the vitals are "in progress" (page still loading) or "final" (page fully loaded and user has interacted).

---

## Integration with Performance Budget Monitor

The performance budget monitor's regression thresholds are updated to include INP:
- INP regression: >50ms increase from baseline (interaction responsiveness is very sensitive to changes)

The push notification on regression includes INP alerts: "INP regressed from 120ms to 380ms after your last change. Worst interaction: button.submit-form (click handler took 250ms)."

---

## Edge Cases

- **No interactions on page**: INP is reported as null (not applicable). This is common for pages the developer loads and immediately reloads without clicking.
- **Page hidden before LCP finalizes**: LCP uses the last reported value. The entry is marked "estimated" since the user may not have seen the largest element.
- **Very many interactions** (>200): Only the top 50 by duration are stored. The 98th percentile is computed from this sample.
- **iframe interactions**: INP only measures main-frame interactions. Cross-origin iframes are excluded (browser limitation).
- **SPA navigations**: Soft navigations (pushState) don't reset vitals. FCP/LCP are only valid for the initial hard navigation. CLS and INP accumulate across the page lifetime. The `get_web_vitals` response notes whether the page has had soft navigations since load.
- **Extension pages**: Chrome extension pages and `chrome://` URLs don't support all observers. Vitals are reported as null with a note.
- **Browser compatibility**: The `event` observer type (for INP) requires Chrome 96+. If unavailable, INP is reported as null with "browser does not support INP measurement."

---

## Performance Constraints

- INP observer overhead: under 0.05ms per interaction (push to sorted array)
- Vitals calculation: under 0.1ms (read accumulated values)
- Memory for 200 interactions: under 20KB
- No impact on the interactions being measured (observer is passive)

---

## Test Scenarios

1. FCP captured and classified correctly (good/needs-improvement/poor)
2. LCP finalized on user interaction or visibility change
3. CLS accumulated correctly, excluding input-adjacent shifts
4. INP calculated as worst interaction duration (≤50 interactions)
5. INP calculated as 98th percentile (>50 interactions)
6. INP groups entries by interactionId, uses longest duration
7. `get_web_vitals` returns all four metrics with ratings
8. `overall_rating` is worst individual rating
9. INP attribution includes target selector and breakdown
10. Vitals history stores up to 10 entries per URL
11. `include_history` parameter returns previous loads
12. No interactions → INP is null
13. LCP marked "estimated" if page hidden before finalization
14. `summary` highlights most actionable insight
15. Integration with push notification: INP regression generates alert
16. Browser without `event` observer → INP null with explanation
17. Phase 1 perf observers (FCP, LCP, CLS) work even with interception deferral

---

## File Locations

Extension implementation: `extension/inject.js` (INP observer, LCP finalization, vitals collection).

Server implementation: `cmd/browser-agent/performance.go` (vitals storage, MCP tool handler, threshold classification).

Tests: `extension-tests/web-vitals.test.js` (extension-side), `cmd/browser-agent/performance_test.go` (server-side).
