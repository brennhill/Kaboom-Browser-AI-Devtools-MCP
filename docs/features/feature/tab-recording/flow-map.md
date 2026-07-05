---
doc_type: feature_flow_map_pointer
feature_id: feature-tab-recording
status: active
last_reviewed: 2026-07-05
canonical_flow_map: docs/architecture/flow-maps/tab-recording-and-media-ingest.md
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Flow Map Pointer

Canonical flow map:

- [Tab Recording and Media Ingest](../../../architecture/flow-maps/tab-recording-and-media-ingest.md)
- [Shared Extraction and Contract Normalization](../../../architecture/flow-maps/shared-extraction-and-contract-normalization.md)

Latest sync update (2026-03-05): popup `Record screen` now prioritizes the tracked tab, context-menu labels now reflect live state (`Control/Release`, annotation start/stop, screen/action recording start/stop), MCP pending approval shows `?` on the extension badge until approved/denied, and recording badge timer state now comes from shared start/stop lifecycle handlers across all entry points.
