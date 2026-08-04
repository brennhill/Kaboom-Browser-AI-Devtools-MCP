---
doc_type: product-spec
feature_id: feature-operational-observability
last_reviewed: 2026-08-04
---

# Operational Observability — Product Specification

## Outcome

When Kaboom fails, hangs, reconnects, or recovers, users receive a coherent local Doctor incident while maintainers receive only privacy-safe aggregate product-health signals.

## Requirements

- Model failures as registry-owned typed incidents with stable codes, lifecycle stages, severity, retryability, correlation, generation, and bounded history.
- Project one incident into local Doctor guidance and a separate allowlisted telemetry envelope.
- Keep evidence, paths, URLs, page data, logs, correlation IDs, generations, timestamps, and support bundles local.
- Deduplicate repeated telemetry without suppressing a delivery that was dropped under queue pressure.
- Recover from transport panics, expose queue pressure locally, and shut down without stranded work.
- Provide preview-then-confirm local support-bundle export with owner-only permissions.
- Group recurring local incidents by a stable fingerprint derived only from registry-owned classification.

## User Experience

Doctor shows what failed, where in the lifecycle it failed, whether recovery is active or exhausted, and safe remediation. Resolved incidents remain in a bounded recovery timeline. Support export never uploads automatically.

## Acceptance

Every canonical failure has one incident source, stale generations cannot change current health, all exported evidence is redacted, and telemetry cannot accept caller-provided sensitive strings.
