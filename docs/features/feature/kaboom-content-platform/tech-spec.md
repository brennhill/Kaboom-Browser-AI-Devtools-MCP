---
doc_type: tech_spec
feature_id: feature-kaboom-content-platform
status: in_progress
last_reviewed: 2026-07-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec

## Primary Components

- `gokaboom.dev/src/components/Landing.astro`
- `gokaboom.dev/src/components/WorkflowLibrary.astro`
- `gokaboom.dev/src/components/ArticlesLibrary.astro`
- `gokaboom.dev/src/styles/custom.css`
- `gokaboom.dev/src/content/docs/articles/*.md`
- `gokaboom.dev/src/content/docs/reference/*.md`
- `gokaboom.dev/src/pages/[...slug].md.ts`
- `gokaboom.dev/src/pages/markdown/[...slug].md.ts`
- `gokaboom.dev/src/pages/llms.txt.ts`
- `gokaboom.dev/src/pages/llms-full.txt.ts`
- `gokaboom.dev/src/utils/siteVersion.ts`

## Contracts and Validation

- Content contract: `scripts/docs/site/check-gokaboom-content-contract.mjs`
- Style contract: `scripts/docs/site/check-content-style-contract.mjs`
- Vale style gate: `scripts/docs/site/run-vale-on-changed.mjs` + `.vale/styles/Kaboom/*`
- Reference schema sync contract: `scripts/docs/check-reference-schema-sync.mjs`
- Feature bundle contract: `scripts/docs/features/check-feature-bundles.js`
- Version surface contract: `check-gokaboom-content-contract.mjs` enforces global docs version references in footer + markdown/LLM outputs.

## Data/Content Sources

- Tool mode/action enums are sourced from:
  - `internal/schema/observe.go`
  - `internal/schema/analyze.go`
  - `internal/schema/configure_properties_core.go`
  - `internal/schema/generate.go`
  - `internal/schema/interact_actions.go`

## Failure Modes

- Missing required headings (`Quick Reference`, `Common Parameters`) in reference docs.
- Missing mode/action sections after schema changes.
- Missing required feature bundle docs for this feature directory.
- Version label drift between docs pages and repo `VERSION`.

## Linked Architecture

