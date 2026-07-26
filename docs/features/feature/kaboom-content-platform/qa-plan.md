---
doc_type: qa_plan
feature_id: feature-kaboom-content-platform
status: in_progress
last_reviewed: 2026-07-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan

## Automated Gates

1. `npm run docs:check:strict`
2. `npm run docs:lint:content-contract`
3. `npm run docs:lint:style-contract`
4. `npm run docs:lint:vale`
5. `npm run docs:lint:reference-schema-sync`
6. `npm run docs:ci`
7. `(cd gokaboom.dev && npm run build)`

## Manual Checks

1. Verify homepage hero spacing/centering at desktop and mobile breakpoints.
2. Verify light/dark theme readability for nav, sidebar, TOC, and body text.
3. Verify `/reference/*` pages render all documented modes/actions.
4. Verify `.md` mirrors resolve for docs, blog, and articles routes.
5. Verify footer on docs/blog/articles pages shows `Docs version: vX.Y.Z (latest)` from root `VERSION`.
6. Verify markdown mirror routes include `docs_version` and `docs_channel` frontmatter keys.

## Regression Focus

- Reference docs drift when schema enums change.
- Visual regressions in top hero layout and section spacing.
- Missing metadata/frontmatter on changed docs/blog/articles files.
- New tutorial/article pages bypassing tone/readability best practices.
- Version references missing or hard-coded in docs surfaces after release bumps.

## Linked Architecture

