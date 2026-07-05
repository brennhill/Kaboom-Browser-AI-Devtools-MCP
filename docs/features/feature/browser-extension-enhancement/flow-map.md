---
doc_type: feature_flow_map_pointer
feature_id: feature-browser-extension-enhancement
status: active
last_reviewed: 2026-07-05
canonical_flow_map: ../../../architecture/flow-maps/extension-heartbeat-connection-status.md
last_verified_version: 0.8.1
last_verified_date: 2026-04-13
---

# Browser Extension Enhancement Flow Map

Canonical flow maps:

- [DRY Test Helpers and Daemon Header Consolidation](../../../architecture/flow-maps/dry-test-helper-and-daemon-header-consolidation.md)
- [Extension Heartbeat Connection Status](../../../architecture/flow-maps/extension-heartbeat-connection-status.md)

Latest update (2026-04-13): popup `Connected` now means daemon-confirmed extension heartbeat; daemon reachability without heartbeat renders `Offline` plus a recovery hint.
