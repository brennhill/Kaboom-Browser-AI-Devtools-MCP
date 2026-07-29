---
doc_type: feature_index
feature_id: feature-multiline-rich-editor
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-29
code_paths:
  - src/background/dom/primitives/dom-primitives-form.ts
  - src/background/dom/dom-dispatch.ts
  - scripts/templates/dom-primitives.ts.tpl
test_paths:
  - tests/extension/dom/dom-primitives-branding.test.js
  - tests/extension/dom/dom-action-family-routing.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Multiline Rich Editor

## TL;DR
- Status: proposed
- Tool: `interact`
- Actions: `type`, `paste`

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Canonical Note
Multiline insertion behavior should prioritize framework-native editor semantics when detectable and fall back to keyboard-simulation paths when not.
