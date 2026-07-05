---
doc_type: product_spec
feature_id: feature-kaboom-content-platform
status: in_progress
last_reviewed: 2026-07-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec

## Objective

Keep `gokaboom.dev` aligned with current product capabilities while preserving a high-conversion marketing layout and machine-readable docs output (`*.md`, `llms.txt`, `llms-full.txt`).

## Scope

- Homepage and discovery pages with modern rhythm and spacing.
- Tool reference pages for `observe`, `analyze`, `interact`, `configure`, and `generate`.
- Blog/article surfaces and markdown mirror routes for agent consumption.
  - `/blog/*`: release-note chronology.
  - `/articles/*`: evergreen, topic-organized guides.
- Contract checks that prevent docs drift when tool modes/actions change in code.
- Writing-style contract checks that keep tutorials beginner-friendly and machine-parseable.
- Single-channel version references (`latest`) sourced from root `VERSION` across docs surfaces.

## User Outcomes

- Developers can scan homepage + solutions quickly and understand core capabilities.
- Readers can find accurate, current tool parameters/actions in reference docs.
- Agents can parse deterministic markdown endpoints for every docs/blog/articles route.
- Readers and agents can see the exact docs version currently published.

## Non-Goals

- Final illustration system (placeholder graphics are acceptable during redesign).
- New backend feature delivery unrelated to documentation or site presentation.

## Linked Architecture

- Canonical flow map: [flow-map.md](./flow-map.md)
