---
doc_type: documentation_standard
scope: docs
last_reviewed: 2026-02-16
---

# Documentation System Standard

## Goal

Make requirements and implementation context easy to discover for humans and LLM agents.

## Canonical Structure

- Feature bundle: `docs/features/<group>/<feature>/`
- Required files per feature: `product-spec.md`, `tech-spec.md`, `qa-plan.md`, `index.md`
- Shared components: `docs/components/<component>.md`
- Cross-feature mapping: `docs/traceability/feature-map.md`
- Content writing standard: [content-style-voice-guide.md](./content-style-voice-guide.md)

## Required Frontmatter Fields

Every feature spec file must include at least:

- `doc_type`
- `feature_id`
- `status`
- `last_reviewed`
- `links` (product/tech/qa/index where applicable)

## Requirement ID Standard

- Prefix derives from `feature_id` in uppercase with non-alphanumeric replaced by `_`.
- Use stable IDs in specs and tests, for example:
  - `FEATURE_QUERY_DOM_001`
  - `FEATURE_QUERY_DOM_002`

## Linking Rules

- `index.md` is the entrypoint for every feature bundle.
- Product, tech, and QA specs must link to each other.
- Shared component docs must link back to related feature bundles.
- Top-level indexes must link only to canonical files.

## Update Workflow

1. Update feature bundle docs first.
2. Update `docs/components/*.md` for shared behavior changes.
3. Regenerate feature/traceability indexes:
   - `node scripts/docs/features/normalize-docs.js`
4. Verify docs links and metadata:
   - `python3 scripts/docs/lint-documentation.py`
5. Enforce feature bundle contract:
   - `node scripts/docs/features/check-feature-bundles.js`
6. Enforce content writing/voice contract:
   - `node scripts/docs/site/check-content-style-contract.mjs`
   - `node scripts/docs/site/run-vale-on-changed.mjs`

## Definition of Done (Docs)

- Feature has all 4 docs (`product/tech/qa/index`).
- Feature appears in `docs/features/feature-index.md`.
- Feature appears in `docs/traceability/feature-map.md`.
- Requirement IDs are present in product/tech/qa specs.
