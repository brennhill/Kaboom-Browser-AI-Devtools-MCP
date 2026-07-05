---
status: proposed
scope: feature/advanced-filtering
ai-priority: medium
tags: [proposal]
relates-to: [tech-spec.md, qa-plan.md]
last-verified: 2026-03-05
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Advanced Filtering for Signal-to-Noise - Feature Proposal

**Feature:** Advanced Filtering for Large Datasets
**Priority:** ⭐⭐⭐ HIGH
**Version:** v6.0 (v6.1 - Smart Observation tier)
**Status:** Proposed for Roadmap
**Created:** 2026-01-30

---

## The Insight

> "Most traffic is noise and we want to make it easier for LLMs"

### The Problem:
- v5.3 pagination solves **token limits** (can't return 440K characters)
- But LLMs still waste tokens analyzing **irrelevant requests**
- 80% of network traffic is often noise: analytics, tracking, static assets

### The Solution:
- **Pagination** (v5.3) = "Don't send everything at once"
- **Filtering** (v6.0) = "Don't send irrelevant data at all"

---

## Current State (v5.3)

### What We Have Today

Basic filtering already exists:
```javascript
// URL substring filter
observe({what: "network_waterfall", url: "/api"})

// Method filter (network_bodies only)
observe({what: "network_bodies", method: "POST"})

// Status code range (network_bodies only)
observe({what: "network_bodies", status_min: 400, status_max: 599})
```

### What's Missing

#### No way to filter out noise domains:
```javascript
// Can't exclude analytics/tracking
observe({what: "network_waterfall"})
// Returns: Google Analytics, Facebook Pixel, Sentry, Hotjar, ...
// AI wastes tokens analyzing tracking scripts
```

#### No content-type filtering:
```javascript
// Can't exclude images/fonts/css
observe({what: "network_waterfall"})
// Returns: 50 PNG images, 10 font files, 5 CSS files
// AI only cares about API calls
```

#### No regex patterns:
```javascript
// Can't match patterns like /api/v1/*/details
observe({what: "network_waterfall", url: "???"})
// Substring matching too crude
```

---

## Proposed Solution (v6.0)

### Domain Filtering

**Problem:** Analytics and tracking domains pollute results

#### Solution:
```javascript
// Allowlist: only show these domains
observe({
  what: "network_waterfall",
  domains: {
    allowlist: ["api.example.com", "cdn.example.com"]
  }
})

// Blocklist: exclude these domains
observe({
  what: "network_waterfall",
  domains: {
    blocklist: ["google-analytics.com", "doubleclick.net", "facebook.net"]
  }
})

// Built-in presets
observe({
  what: "network_waterfall",
  filter_preset: "api_only" // Excludes common tracking/analytics
})
```

#### Built-in Presets:
- `"api_only"` - Only JSON/XML API calls (excludes static assets)
- `"no_tracking"` - Excludes Google Analytics, Facebook Pixel, etc.
- `"errors_only"` - Only failed requests (4xx, 5xx)
- `"slow_only"` - Only requests >500ms

### Content-Type Filtering

**Problem:** Images, fonts, CSS clutter network view

#### Solution:
```javascript
// Only show specific content types
observe({
  what: "network_waterfall",
  content_types: ["application/json", "application/xml"]
})

// Exclude specific content types
observe({
  what: "network_waterfall",
  exclude_content_types: ["image/*", "font/*", "text/css"]
})
```

### Regex Pattern Matching

**Problem:** Substring matching too crude

#### Solution:
```javascript
// Match URL patterns
observe({
  what: "network_waterfall",
  url_pattern: "^https://api\\.example\\.com/v1/users/\\d+$"
})

// Multiple patterns (OR logic)
observe({
  what: "network_waterfall",
  url_patterns: [
    "/api/v1/.*",
    "/graphql.*"
  ]
})
```

### Size Thresholds

**Problem:** Huge responses (images, videos) waste tokens

#### Solution:
```javascript
// Only show requests with response size < 1MB
observe({
  what: "network_waterfall",
  max_response_size: 1048576
})

// Only show large responses (find bottlenecks)
observe({
  what: "network_waterfall",
  min_response_size: 100000 // >100KB
})
```

### Duration Thresholds

**Problem:** Fast requests often not interesting for debugging

#### Solution:
```javascript
// Only show slow requests
observe({
  what: "network_waterfall",
  min_duration_ms: 500 // >500ms
})

// Only show fast requests (caching verification)
observe({
  what: "network_waterfall",
  max_duration_ms: 50 // <50ms
})
```

### Combined Filters

#### Power of composition:
```javascript
// "Show me slow API calls that failed"
observe({
  what: "network_waterfall",
  content_types: ["application/json"],
  status_min: 400,
  min_duration_ms: 1000,
  limit: 50
})

// "Show me all tracking requests so I can block them"
observe({
  what: "network_waterfall",
  filter_preset: "tracking_only",
  domains: {
    blocklist_invert: true // Invert the built-in tracking list
  }
})
```

---

## Why v6.0, Not v5.3?

### v5.3 Scope (Must Ship Fast)

**Goal:** Unblock AI workflows TODAY
- Pagination: 2-4 hours
- Buffer clearing: 1-2 hours
- **Total:** 3-6 hours → Ship in days

#### Filtering is more complex:
- Content-type filtering: Need to parse response headers
- Regex patterns: Requires regex engine + error handling
- Presets: Need to research common tracking domains
- **Total:** 10-15 hours → Would delay v5.3 by weeks

### v6.0 Scope (Core Thesis)

**Goal:** Prove AI closes the feedback loop autonomously

Filtering fits perfectly in **v6.1: Smart Observation**:
- Solves Problem A (tokens) by reducing noise
- Complements pagination (pagination = chunk data, filtering = remove noise)
- Enables "semantic debugging context" (thesis goal)

---

## Real-World Examples

### Before Filtering

```javascript
observe({what: "network_waterfall"})
```
Returns 200 requests:
- ❌ 50 Google Analytics events
- ❌ 30 Facebook Pixel tracking
- ❌ 20 PNG images
- ❌ 15 font files
- ❌ 10 CSS files
- ✅ 25 API calls (what AI actually needs)
- ❌ 50 other noise

#### LLM wastes 175/200 of its attention on noise.

### After Filtering (v6.0)

```javascript
observe({
  what: "network_waterfall",
  filter_preset: "api_only",
  domains: {
    allowlist: ["api.example.com"]
  }
})
```
Returns 25 requests:
- ✅ 25 API calls (100% signal)

#### LLM focuses on relevant data only. 7× more efficient.

---

## Implementation Strategy

### Phase 1: Domain Filtering (Most Impact)

**Effort:** 3-4 hours
- Add domain allowlist/blocklist parameters
- Create built-in tracking domain list
- Apply filters in existing GetNetworkWaterfall()

### Phase 2: Content-Type Filtering

**Effort:** 2-3 hours
- Parse content-type from network waterfall entries
- Add content-type filter parameters
- Support wildcards (image/*, text/*)

### Phase 3: Regex Patterns

**Effort:** 2-3 hours
- Add url_pattern parameter
- Use Go's regexp package
- Error handling for invalid patterns

### Phase 4: Size & Duration Thresholds

**Effort:** 1-2 hours
- Add min/max size and duration parameters
- Apply numeric range filters

### Phase 5: Filter Presets

**Effort:** 2-3 hours
- Research common tracking domains
- Define preset configurations
- Add preset parameter

**Total Effort:** 10-15 hours (fits v6.0 timeline)

---

## Success Metrics

### Primary

- **Token efficiency:** Average observe() response uses 50% fewer tokens
- **Signal-to-noise ratio:** >80% of returned requests are relevant to debugging task

### Secondary

- Filter usage in >60% of network_waterfall calls
- LLM debugging accuracy improves (fewer "I see analytics requests" distractions)

---

## User Feedback Integration

**User Quote:** "Most traffic is noise and we want to make it easier for LLMs"

### Key Insights:
1. Pagination alone isn't enough - LLMs still waste tokens on noise
2. Filtering is about **semantic relevance**, not just size
3. Common patterns (analytics, tracking, static assets) should have presets

---

## Comparison with Competitors

| Tool | Pagination | Basic Filters | Advanced Filters | Presets |
|------|------------|---------------|------------------|---------|
| Chrome DevTools | ❌ No | ✅ Yes | ✅ Yes | ⚠️ Manual |
| Kaboom v5.3 | ✅ Yes | ⚠️ URL only | ❌ No | ❌ No |
| **Kaboom v6.0** | **✅ Yes** | **✅ Yes** | **✅ Yes** | **✅ Yes** |

**Competitive Advantage:** AI-optimized filter presets that understand common noise patterns.

---

## Open Questions

1. **Q:** Should filters apply to all observe() modes or just network_waterfall?
   **A:** Start with network_waterfall (biggest noise problem), expand to websocket_events and logs if needed

2. **Q:** Should we allow saving custom filter presets?
   **A:** Not in v6.0 - just ship built-in presets. Custom presets can come in v6.2+

3. **Q:** How do filters interact with pagination?
   **A:** Apply filters first, then paginate. `total` in metadata shows filtered count, not raw count.

---

## Dependencies

### Before This Feature

- ✅ v5.3 shipped (pagination + buffer clearing)
- ✅ Basic filtering works (URL substring)

### After This Feature

- 📋 Custom filter presets (v6.2+)
- 📋 Filter composition UI (v6.3+)
- 📋 AI-suggested filters (v6.5+)

---

## Approval Status

**Product:** ✅ Approved for v6.0 roadmap
**Engineering:** 📋 Needs technical spec
**Effort:** 10-15 hours
**Target:** v6.0 (v6.1 - Smart Observation)

### Next Steps:
1. Add to v6.0 roadmap ✅ (DONE)
2. Create detailed product-spec.md (after v5.3 ships)
3. Create tech-spec.md
4. Research common tracking domains for presets
5. Implement in v6.0 timeline

---

## Related Features

- **Pagination (v5.3)** - Complementary: pagination chunks data, filtering removes noise
- **Smart DOM Pruning (v6.1)** - Similar philosophy: remove noise before AI sees it
- **Context Streaming (v6.0)** - Benefits from filtering: less noise = clearer streaming context

---

## Conclusion

### Why This Matters:

Pagination solves "too much data."
Filtering solves "wrong data."

Together, they enable LLMs to focus on **relevant debugging context** within token limits. This is core to the v6.0 thesis: "AI closes the feedback loop autonomously."

### User Impact:
- 7× more efficient token usage
- Faster debugging (less noise to analyze)
- Better AI suggestions (focused on signal, not noise)

### Strategic Value:
- Differentiates Kaboom from basic MCP tools
- Aligns with "semantic debugging context" vision
- Enables enterprise adoption (production traffic is noisy)
