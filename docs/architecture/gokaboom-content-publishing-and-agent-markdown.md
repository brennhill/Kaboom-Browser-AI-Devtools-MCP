---
doc_type: flow_map
flow_id: gokaboom-content-publishing-and-agent-markdown
status: active
last_reviewed: 2026-03-28
owners:
  - Brenn
feature_ids:
  - feature-gokaboom-content-platform
entrypoints:
  - gokaboom.dev/src/content/docs/index.mdx
  - gokaboom.dev/src/content/docs/downloads.md
  - gokaboom.dev/src/pages/[...slug].md.ts
  - gokaboom.dev/src/pages/search-synonyms.json.ts
  - scripts/docs/check-docs-quality-gates.mjs
  - scripts/docs/check-site-content-ids.mjs
  - scripts/docs/check-gokaboom-content-contract.mjs
  - scripts/docs/check-content-style-contract.mjs
  - scripts/docs/check-downloads-page-contract.mjs
  - scripts/docs/check-landing-layout-contract.mjs
  - scripts/docs/check-light-theme-contract.mjs
  - scripts/docs/check-reference-schema-sync.mjs
  - scripts/docs/run-vale-on-changed.mjs
  - .vale.ini
code_paths:
  - package.json
  - .vale.ini
  - .vale/styles/*.yml
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
  - gokaboom.dev/src/components/ThemeProvider.astro
  - gokaboom.dev/src/components/ThemeSelect.astro
  - gokaboom.dev/src/components/WorkflowLibrary.astro
  - gokaboom.dev/src/components/ArticlesLibrary.astro
  - gokaboom.dev/src/styles/custom.css
  - gokaboom.dev/src/data/relatedGuides.ts
  - gokaboom.dev/src/data/searchSynonyms.ts
  - gokaboom.dev/src/data/workflows.ts
  - gokaboom.dev/src/pages/[...slug].md.ts
  - gokaboom.dev/src/pages/index.md.ts
  - gokaboom.dev/src/pages/llms.txt.ts
  - gokaboom.dev/src/pages/llms-full.txt.ts
  - gokaboom.dev/src/pages/markdown/[...slug].md.ts
  - gokaboom.dev/src/pages/search-synonyms.json.ts
  - gokaboom.dev/src/utils/markdownPaths.ts
  - gokaboom.dev/src/utils/siteVersion.ts
  - scripts/docs/check-docs-quality-gates.mjs
  - scripts/docs/check-site-content-ids.mjs
  - scripts/docs/check-gokaboom-content-contract.mjs
  - scripts/docs/check-content-style-contract.mjs
  - scripts/docs/check-downloads-page-contract.mjs
  - scripts/docs/check-landing-layout-contract.mjs
  - scripts/docs/check-light-theme-contract.mjs
  - scripts/docs/check-reference-schema-sync.mjs
  - scripts/docs/generate-reference-executable-examples.mjs
  - scripts/docs/normalize-site-tags.mjs
  - scripts/docs/sync-verification-metadata.mjs
  - scripts/docs/run-vale-on-changed.mjs
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
  - scripts/docs/check-light-theme-contract.mjs
  - scripts/docs/check-reference-schema-sync.mjs
  - scripts/docs/generate-reference-executable-examples.mjs
  - scripts/docs/normalize-site-tags.mjs
  - scripts/docs/sync-verification-metadata.mjs
  - scripts/docs/run-vale-on-changed.mjs
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Gokaboom Content Publishing and Agent Markdown Flow

## Scope

Covers complete homepage theme/layout replacement and messaging updates (including centered hero flame-only favicon-style flicker, tightened hero typography, light-theme header title color tuning, reference-page readability fixes, schema-synced tool-mode coverage, section spacing/overflow hardening for full-page scroll rhythm, a branded integrations card with real agent logos + hover-widget concept preview, split solutions panels with in-panel Kaboom mock visuals + annotation callouts, large-screen left/right stagger offsets for solutions visual rhythm, light-only theme enforcement with no user theme dropdown, updated CTA/footer link treatments, and new task-specific guide pages for SEO and annotation-skill-terminal workflows), plus downloads-page requirement clarity (native binary runtime with optional npm/Node path) and expressive-code frame chrome cleanup driven by annotation feedback. Also covers workflow discovery, split discovery surfaces for date-driven release-note `blog` and topic-driven `articles`, tool-reference navigation, and automatic per-route markdown mirrors for agent consumption.

## Entrypoints

- Splash homepage in `gokaboom.dev/src/content/docs/index.mdx`
- Downloads page content in `gokaboom.dev/src/content/docs/downloads.md`
- Agent markdown mirror route in `gokaboom.dev/src/pages/[...slug].md.ts`
- Synonym dictionary endpoint in `gokaboom.dev/src/pages/search-synonyms.json.ts`
- Phase-based docs quality gate in `scripts/docs/check-docs-quality-gates.mjs`
- Duplicate content-ID guard in `scripts/docs/check-site-content-ids.mjs`
- Content contract gate in `scripts/docs/check-gokaboom-content-contract.mjs`
- Downloads page contract gate in `scripts/docs/check-downloads-page-contract.mjs`
- Landing layout contract gate in `scripts/docs/check-landing-layout-contract.mjs`
- Light-theme contract gate in `scripts/docs/check-light-theme-contract.mjs`
- Reference/schema sync gate in `scripts/docs/check-reference-schema-sync.mjs`
- Style contract gate in `scripts/docs/check-content-style-contract.mjs`
- Vale plain-language gate in `scripts/docs/run-vale-on-changed.mjs`

## Primary Flow

1. Starlight loads docs/blog/articles entries from `gokaboom.dev/src/content/docs/*` through `docsLoader()`.
2. Site navigation and information architecture are defined in `gokaboom.dev/astro.config.mjs`.
3. Splash pages render reusable components for marketing and discovery (`Landing.astro`, `WorkflowLibrary.astro`, `ArticlesLibrary.astro`).
4. Every docs/blog/articles slug is mirrored as `/<slug>.md` via `src/pages/[...slug].md.ts`.
5. `<link rel="alternate" type="text/markdown">` in `Head.astro` points each HTML route to its markdown mirror.
6. `llms.txt` and `llms-full.txt` enumerate markdown/HTML URLs from `src/utils/markdownPaths.ts`.
7. CI executes `docs:ci` to enforce feature bundle completeness, content contract compliance, style/voice contract checks, version-surface integrity, and schema-to-reference mode coverage.
8. Downloads-page copy and expressive-code visual guardrails are enforced by `check-downloads-page-contract.mjs`.
9. Large-screen solutions-panel staggering is enforced by `check-landing-layout-contract.mjs`.
10. Light-only theme behavior and selector removal are enforced by `check-light-theme-contract.mjs`.
11. Phased docs quality gates enforce integrity, freshness, and executable reference-example coverage (`check-docs-quality-gates.mjs`).
12. Duplicate content IDs are blocked by deterministic slug checks (`check-site-content-ids.mjs`).

## Error and Recovery Paths

| Condition | Behavior |
| --- | --- |
| Missing slug in markdown mirror route | Returns markdown `404` response |
| Missing title/description on changed docs file | CI failure with explicit frontmatter error |
| Version reference missing in footer or markdown/LLM docs surfaces | CI failure from content contract version-surface checks |
| Missing date/authors/tags on changed article or blog file | CI failure from style contract |
| How-to article lacks `How to` title prefix or steps | CI failure from style contract |
| First-use acronym is not expanded in a changed how-to article | CI failure from style contract |
| Non-descriptive links or jargon in changed content | CI failure from Vale style gate |
| Reference page missing key sections | CI failure (`Quick Reference`, `Common Parameters`) |
| Reference page missing a live schema mode/action | CI failure from `check-reference-schema-sync.mjs` |
| Blog post missing date/authors/tags | CI failure with required key list |
| Outdated evergreen content appears under `/blog/*` | IA violation; evergreen guides must live under `/articles/*` |
| Articles page not grouped by topic | IA regression; `/articles/` no longer reflects tag-grouped sections |

## State and Contracts

- **SEO contract:** Changed docs/blog/articles files require `title` and `description` frontmatter.
- **Style contract:** Changed articles/blog/how-to docs enforce metadata completeness, tutorial structure, acronym expansion, and plain-language link-text checks.
- **Version contract:** Footer, markdown mirrors, and `llms*.txt` always publish root `VERSION` as the latest docs version.
- **LLM contract:** Every docs/blog/articles page has deterministic markdown at `*.md` and is discoverable in `llms.txt`.
- **Reference contract:** Pages under `/reference/` keep predictable section anchors and cover live schema modes/actions.
- **Backward compatibility:** Legacy `/markdown/<slug>.md` routes remain available.

## Code Paths

- `gokaboom.dev/astro.config.mjs`
- `gokaboom.dev/public/images/integrations/*.svg`
- `gokaboom.dev/public/images/landing/*.svg`
- `gokaboom.dev/src/components/Head.astro`
- `gokaboom.dev/src/components/Footer.astro`
- `gokaboom.dev/src/components/Landing.astro`
- `gokaboom.dev/src/components/ThemeProvider.astro`
- `gokaboom.dev/src/components/ThemeSelect.astro`
- `gokaboom.dev/src/components/WorkflowLibrary.astro`
- `gokaboom.dev/src/components/ArticlesLibrary.astro`
- `gokaboom.dev/src/styles/custom.css`
- `gokaboom.dev/src/content/docs/index.mdx`
- `gokaboom.dev/src/content/docs/downloads.md`
- `gokaboom.dev/src/content/docs/guides/seo-analysis.md`
- `gokaboom.dev/src/content/docs/guides/annotation-skill-terminal-workflow.md`
- `gokaboom.dev/src/data/workflows.ts`
- `gokaboom.dev/src/pages/[...slug].md.ts`
- `gokaboom.dev/src/pages/index.md.ts`
- `gokaboom.dev/src/pages/llms.txt.ts`
- `gokaboom.dev/src/pages/llms-full.txt.ts`
- `gokaboom.dev/src/pages/markdown/[...slug].md.ts`
- `gokaboom.dev/src/utils/markdownPaths.ts`
- `gokaboom.dev/src/utils/siteVersion.ts`
- `scripts/docs/check-gokaboom-content-contract.mjs`
- `scripts/docs/check-content-style-contract.mjs`
- `scripts/docs/check-downloads-page-contract.mjs`
- `scripts/docs/check-landing-layout-contract.mjs`
- `scripts/docs/check-light-theme-contract.mjs`
- `scripts/docs/check-reference-schema-sync.mjs`
- `scripts/docs/run-vale-on-changed.mjs`
- `scripts/docs/check-feature-bundles.js`
- `.vale.ini`
- `.vale/styles/*.yml`
- `.github/workflows/ci.yml`

## Test Paths

- `scripts/docs/check-gokaboom-content-contract.mjs`
- `scripts/docs/check-content-style-contract.mjs`
- `scripts/docs/check-downloads-page-contract.mjs`
- `scripts/docs/check-landing-layout-contract.mjs`
- `scripts/docs/check-light-theme-contract.mjs`
- `scripts/docs/check-reference-schema-sync.mjs`
- `scripts/docs/check-feature-bundles.js`
- `scripts/docs/run-vale-on-changed.mjs`

## Edit Guardrails

1. New docs/blog/articles routes must resolve to both HTML and `*.md` outputs.
2. Changes to markdown URL generation must update `llms.txt` and `llms-full.txt` behavior.
3. Reference pages must preserve `## Quick Reference` and `## Common Parameters` headings.
4. Reference pages must keep mode/action sections synchronized with `internal/schema/*`.
5. Keep legacy `/markdown/*` compatibility until consumers are migrated.
6. Changed how-to files must keep acronym expansions and step-by-step structure.
7. Docs version references must remain sourced from root `VERSION` (no hard-coded release strings).
8. Any site IA refactor must update this flow map and its feature index in the same PR.
