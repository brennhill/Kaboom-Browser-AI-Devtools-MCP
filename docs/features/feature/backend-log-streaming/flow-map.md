---
doc_type: feature_flow_map_pointer
feature_id: feature-backend-log-streaming
status: active
last_reviewed: 2026-07-05
canonical_flow_map: ../../../architecture/flow-maps/extension-heartbeat-connection-status.md
last_verified_version: 0.8.6
last_verified_date: 2026-04-13
---

# Backend Log Streaming Flow Map

Canonical flow maps:

- [Capture Buffer Store Extraction](../../../architecture/flow-maps/capture-buffer-store.md)
- [Extension Heartbeat Connection Status](../../../architecture/flow-maps/extension-heartbeat-connection-status.md)
- [Shared Extraction and Contract Normalization](../../../architecture/flow-maps/shared-extraction-and-contract-normalization.md)
- [DRY Test Helpers and Daemon Header Consolidation](../../../architecture/flow-maps/dry-test-helper-and-daemon-header-consolidation.md)

Latest sync update (2026-04-13): extension connection status now requires daemon-reported `/sync` heartbeat confirmation; a reachable daemon without heartbeat remains offline in popup/UAT state.
