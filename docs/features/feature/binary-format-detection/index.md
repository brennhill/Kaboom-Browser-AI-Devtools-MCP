---
doc_type: feature_index
feature_id: feature-binary-format-detection
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - internal/capture/network.go
  - internal/capture/websocket.go
  - internal/util/binary.go
test_paths:
  - internal/util/binary_test.go
  - internal/util/binary_coverage_test.go
  - internal/util/binary_fuzz_test.go
  - internal/util/binary_property_test.go
  - internal/util/binary_protobuf_bson_test.go
  - internal/capture/binary_format_integration_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Binary Format Detection

## TL;DR

- Status: shipped
- Tool: observe
- Mode/Action: network_bodies
- Location: `docs/features/feature/binary-format-detection`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BINARY_FORMAT_DETECTION_001
- FEATURE_BINARY_FORMAT_DETECTION_002
- FEATURE_BINARY_FORMAT_DETECTION_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
