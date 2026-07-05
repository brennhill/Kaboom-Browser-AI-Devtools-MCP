---
doc_type: product-spec
feature_id: icon-regression
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Tab Icon State Indicator

## Problem

The tab icon (favicon) should dynamically change to indicate tracking and AI pilot state:
- **Tracked tab**: Show flame icon
- **AI Pilot active**: Animate flame
- **Disconnected**: Restore original favicon
- **URL changes**: Re-apply icon when URL changes within tracked tab

**Current state**: Broken - icon not updating properly.

## User Stories

1. **As a developer**, I want to see which tab is being tracked without opening the extension, so I can quickly identify the monitored page.

2. **As a developer**, I want to see when AI Pilot is actively controlling the page (animated flame), so I know when autonomous actions are happening.

3. **As a developer**, I want the original favicon restored when I stop tracking, so my tabs look normal again.

4. **As a developer**, I want the flame to persist when navigating within the same tab (SPA navigation), so I don't lose visual tracking context.

## Requirements

### Functional

1. **Tracking State**
   - When tab becomes tracked → inject flame icon
   - When tracking stops → restore original favicon
   - Icon persists across URL changes within same tab
   - Handle both standard navigation and SPA routing

2. **Pilot State**
   - Static flame when tracked + pilot disabled
   - Animated flame when tracked + pilot enabled
   - Animation should be subtle (CSS animation, not GIF flicker)

3. **State Transitions**
   - Tracked OFF → Tracked ON: Original → Flame (static)
   - Pilot OFF → Pilot ON: Flame (static) → Flame (animated)
   - Pilot ON → Pilot OFF: Flame (animated) → Flame (static)
   - Tracked ON → Tracked OFF: Flame → Original

4. **URL Change Handling**
   - Listen to `chrome.tabs.onUpdated` with URL filter
   - Re-inject icon when URL changes for tracked tab
   - Maintain animation state across URL changes

### Non-Functional

- Icon change should be instant (< 50ms perceived delay)
- No flicker during state transitions
- No memory leaks from animation listeners
- Works on all sites (including those with strict CSP)

## Edge Cases

1. **Page without favicon**: Use browser default, then inject flame
2. **Dynamic favicon** (e.g., notification badges): Save original before injecting
3. **Multiple rapid URL changes**: Debounce icon injection
4. **Tab reload**: Restore correct icon state after reload
5. **Extension reload**: Re-sync icon state with tracking status

## Out of Scope

- Custom icon colors (always flame)
- User-configurable icons
- Multiple tracking icons (only one tab tracked at a time)
- Icon badges (browser action icon separate concern)

## Success Metrics

- Icon updates within 50ms of state change
- Zero user reports of "icon stuck" or "icon not updating"
- Works on 100% of tested sites (including chrome://, file://, etc.)

## Alternatives Considered

1. **Browser Action Badge**: Rejected - not visible per-tab
2. **Title Prefix**: Rejected - too intrusive to page title
3. **Page Overlay**: Rejected - obscures content

## Dependencies

- `chrome.tabs.onUpdated` API
- Content script icon injection capability
- Tracking state from background service worker
