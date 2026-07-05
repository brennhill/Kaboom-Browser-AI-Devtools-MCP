---
doc_type: tech-spec
feature_id: feature-framework-selector-resilience
status: shipped
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Framework Selector Resilience Tech Spec

## Architecture

1. Source fixtures live in `scripts/smoke-tests/framework-fixtures/`.
2. Build pipeline in `build-framework-fixtures.mjs` compiles/exports fixtures into harness-served output:
   - React bundle (`esbuild`)
   - Vue bundle (`esbuild`)
   - Svelte bundle (`svelte/compiler` + `esbuild`)
   - Next.js static export (`next build` with `output: "export"`)
3. Output target is `cmd/browser-agent/testpages/frameworks/`.
4. Smoke module `29-framework-selector-resilience.sh` runs a shared hard-case matrix per framework.

## Fixture Contract

All framework fixtures expose a common semantic contract:
1. Hydration marker: `#hydrated-ready`
2. Overlay action: `text=Accept Cookies`
3. Route controls: `text=Profile Tab`, `text=Settings Tab`
4. Main input: `placeholder=Enter name`
5. Primary submit: `text=Submit Profile`
6. Async action: `text=Async Save`, result in `#async-result`
7. Virtualized deep target: `text=Framework Target 80`, result in `#deep-result`
8. Selector churn marker: `#selector-token`
9. Remount marker: `#mount-token`

## Runtime Flow

1. Module starts by ensuring fixture build is current.
2. For each framework page:
   - navigate and verify framework detection
   - wait for hydration + dismiss overlay
   - run async + virtualized flows
   - run stale-handle remount flow
   - run 3 refresh cycles with semantic and list-interactive round-trip interactions
3. Any failure reports exact payload context and step.

## Reliability Considerations

1. Hard-case checks are run before refresh loops to validate baseline complexity.
2. Refresh loops validate selector churn + remount recovery repeatedly.
3. Fixture semantics are stable while IDs/classes churn, preventing brittle test coupling.

## Guardrails

1. Do not change fixture semantic labels/selectors without updating module `29` and docs.
2. Keep Next export static-compatible (`output: "export"`).
3. Keep bundle generation deterministic and isolated to generated output directories.
