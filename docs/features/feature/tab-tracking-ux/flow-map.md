---
doc_type: feature_flow_map_pointer
feature_id: feature-tab-tracking-ux
status: active
last_reviewed: 2026-07-05
canonical_flow_map: docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Flow Map Pointer

Canonical flow map:

- [Tracked Tab Hover Quick Actions](../../../architecture/flow-maps/tracked-tab-hover-quick-actions.md)
- The tracked hover launcher now uses `src/content/ui/terminal-panel-bridge.ts` to open the terminal side panel and observe panel visibility.
- The launcher hides only while the workspace side panel is open and reappears for both minimized and fully closed panel states.
- The hover island now keeps the restored Kaboom flame icon on hover and only changes button elevation, not the asset itself.
