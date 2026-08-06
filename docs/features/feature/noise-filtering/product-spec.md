---
feature: noise-filtering
status: shipped
tool: configure
mode: noise_rule, dismiss
version: 0.7.12
doc_type: product-spec
feature_id: feature-noise-filtering
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Noise Filtering

Describes the user-facing requirements, rationale, and deprecations for the Noise Filtering feature.

- **Purpose:** Suppress irrelevant browser noise for AI agents and developers.
- **Requirements:**
  - Filter out extension, analytics, and framework noise from logs.
  - Allow user/agent to add custom noise rules.
  - Built-in rules always active; user rules session-scoped.
- **Deprecations:**
  - Any legacy noise filtering logic is replaced by this system.

---

## See also:
- [Noise Filtering Tech Spec](tech-spec.md)
- [Noise Filtering ADRs](./index.md)
- [Core Product Spec](../../../core/product/product-spec.md)
