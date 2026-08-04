---
doc_type: feature_index
feature_id: feature-binary-format-detection
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-04
code_paths:
  - internal/capture/events.go
  - internal/util/binary.go
test_paths:
  - internal/util/binary_test.go
  - internal/util/binary_protobuf_bson_test.go
  - internal/capture/binary_format_integration_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
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

Binary detection is owned by one production module. General behavior and edge
coverage change together in `binary_test.go`; format-specific protobuf/BSON,
property, and fuzz coverage change together in
`binary_protobuf_bson_test.go`. This keeps the utility package below the
ten-file boundary without weakening its deterministic, property, or fuzz
coverage.
