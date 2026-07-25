---
doc_type: feature_index
feature_id: feature-kaboom-content-platform
status: in_progress
feature_type: feature
owners:
  - Brenn
last_reviewed: 2026-07-05
code_paths:
  - package.json
  - .vale.ini
  - .vale/styles/Kaboom/*.yml
  - gokaboom.dev/astro.config.mjs
  - gokaboom.dev/src/content.config.ts
  - gokaboom.dev/public/images/integrations/*.svg
  - gokaboom.dev/public/images/landing/*.svg
  - gokaboom.dev/public/images/solutions-seo-signal.svg
  - gokaboom.dev/src/content/docs/articles/*.md
  - gokaboom.dev/src/content/docs/blog/*.md
  - gokaboom.dev/src/content/docs/downloads.md
  - gokaboom.dev/src/content/docs/guides/start-here-by-role.md
  - gokaboom.dev/src/content/docs/guides/tracks/*.md
  - gokaboom.dev/src/content/docs/guides/visual-evidence-standards.md
  - gokaboom.dev/src/content/docs/guides/seo-analysis.md
  - gokaboom.dev/src/content/docs/guides/annotation-skill-terminal-workflow.md
  - gokaboom.dev/src/content/docs/index.mdx
  - gokaboom.dev/src/content/docs/reference/index.md
  - gokaboom.dev/src/content/docs/reference/examples/*.md
  - gokaboom.dev/src/content/docs/reference/observe.md
  - gokaboom.dev/src/content/docs/reference/analyze.md
  - gokaboom.dev/src/content/docs/reference/interact.md
  - gokaboom.dev/src/content/docs/reference/configure.md
  - gokaboom.dev/src/content/docs/reference/generate.md
  - gokaboom.dev/src/components/Footer.astro
  - gokaboom.dev/src/components/Head.astro
  - gokaboom.dev/src/components/Landing.astro
  - gokaboom.dev/src/components/WorkflowLibrary.astro
  - gokaboom.dev/src/components/ArticlesLibrary.astro
  - gokaboom.dev/src/data/relatedGuides.ts
  - gokaboom.dev/src/data/searchSynonyms.ts
  - gokaboom.dev/src/data/workflows.ts
  - gokaboom.dev/src/pages/[...slug].md.ts
  - gokaboom.dev/src/pages/index.md.ts
  - gokaboom.dev/src/pages/llms.txt.ts
  - gokaboom.dev/src/pages/llms-full.txt.ts
  - gokaboom.dev/src/pages/markdown/[...slug].md.ts
  - gokaboom.dev/src/pages/search-synonyms.json.ts
  - gokaboom.dev/src/styles/custom.css
  - gokaboom.dev/src/utils/markdownPaths.ts
  - gokaboom.dev/src/utils/siteVersion.ts
  - scripts/docs/check-docs-quality-gates.mjs
  - scripts/docs/check-site-content-ids.mjs
  - scripts/docs/check-gokaboom-content-contract.mjs
  - scripts/docs/check-content-style-contract.mjs
  - scripts/docs/check-downloads-page-contract.mjs
  - scripts/docs/check-landing-layout-contract.mjs
  - scripts/docs/generate-reference-executable-examples.mjs
  - scripts/docs/normalize-site-tags.mjs
  - scripts/docs/sync-verification-metadata.mjs
  - scripts/docs/run-vale-on-changed.mjs
  - scripts/docs/check-reference-schema-sync.mjs
  - scripts/docs/check-feature-bundles.js
  - .github/workflows/ci.yml
test_paths:
  - scripts/docs/check-docs-quality-gates.mjs
  - scripts/docs/check-site-content-ids.mjs
  - scripts/docs/check-feature-bundles.js
  - scripts/docs/check-gokaboom-content-contract.mjs
  - scripts/docs/check-content-style-contract.mjs
  - scripts/docs/check-downloads-page-contract.mjs
  - scripts/docs/check-landing-layout-contract.mjs
  - scripts/docs/generate-reference-executable-examples.mjs
  - scripts/docs/normalize-site-tags.mjs
  - scripts/docs/sync-verification-metadata.mjs
  - scripts/docs/run-vale-on-changed.mjs
  - scripts/docs/check-reference-schema-sync.mjs
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Gokaboom Content Platform

## TL;DR

- Status: in_progress
- Scope: full homepage theme reset to an updated modern layout, plus hero flame-only favicon-style flicker, light-theme header title color tuning, reference-page readability fixes (heading dedupe + contrast), schema-synced reference mode/action coverage, scroll-rhythm spacing/overflow hardening, workflow discovery, split `blog` (date-driven) vs `articles` (topic-driven), automated markdown mirrors for docs/blog/articles pages, downloads-page clarity updates (native binary runtime guidance + expressive-code terminal frame visual cleanup), and large-screen staggered left/right solution panel offsets for better scan rhythm. Home solutions now use five split panels with large left-side visual mocks, Kaboom-themed annotation callouts, and right-side CTA/copy blocks. The integrations band now uses a branded card with real agent logos and a hover-widget concept preview, and CTA/footer links were updated based on annotation feedback.
- Current IA policy: `/blog/*` is release-note history, `/articles/*` is evergreen topic content.
- Versioning policy: single live channel (`latest`) with docs version sourced from repo `VERSION` and rendered globally.
- Primary guardrail: CI docs contracts (`content-contract` + `style-contract` + `Vale` + `downloads-contract` + `landing-layout-contract` + reference schema sync + feature bundle strict check)

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Code and Tests

- Content and layout code paths are listed in frontmatter.
- CI gate is enforced through `docs:ci` in `.github/workflows/ci.yml`.
