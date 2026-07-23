---
doc_type: flow_map
flow_id: release-asset-contract-and-sarif-distribution
status: active
last_reviewed: 2026-04-18
owners:
  - Brenn
entrypoints:
  - .github/workflows/release.yml:build-and-release
  - docs/core/release.md
code_paths:
  - .github/workflows/release.yml
  - docs/core/release.md
test_paths:
  - tests/packaging/release-workflow-contract.test.js
last_verified_version: 0.8.5
last_verified_date: 2026-04-18
---

# Release Asset Contract and SARIF Distribution

## Scope

Covers the public GitHub Release asset set for tagged releases and the boundary that keeps SARIF on the CI/code-scanning path instead of the public release bundle.

Related docs:

- `docs/core/release.md`
- `docs/features/feature/kaboom-ci/product-spec.md`
- `docs/features/feature/kaboom-ci/tech-spec.md`
- `docs/features/feature/sarif-export/tech-spec.md`

## Entrypoints

1. Tagged release workflow: `.github/workflows/release.yml`.
2. Maintainer-facing process doc: `docs/core/release.md`.

## Primary Flow

1. A `v*.*.*` tag on a `STABLE` commit enters `build-and-release`.
2. The workflow builds binaries, compiles the extension, creates the extension zip, and generates `dist/checksums.txt`.
3. `softprops/action-gh-release` uploads the fixed public asset list.
4. `fail_on_unmatched_files: true` fails the release immediately if any expected public asset is missing.
5. SARIF remains outside the GitHub Release upload list.
6. CI/reporting workflows generate SARIF separately and send it to GitHub Code Scanning.

## Error and Recovery Paths

1. Missing public asset: release job fails at the GitHub Release step instead of silently publishing a partial bundle.
2. Missing SARIF: does not affect the public release because SARIF is not part of the release asset contract.
3. If maintainers want SARIF in CI, they must add or fix the producer in the CI/code-scanning path rather than expanding the public release bundle.

## State and Contracts

1. Public release assets are binaries, hooks, extension zip, and `checksums.txt`.
2. SARIF is a CI/code-scanning artifact, not a public download artifact.
3. Release docs and workflow must stay aligned on this boundary.

## Code Paths

- `.github/workflows/release.yml`
- `docs/core/release.md`

## Test Paths

- `tests/packaging/release-workflow-contract.test.js`

## Edit Guardrails

1. Do not add new public release assets without updating both `docs/core/release.md` and this flow map in the same change.
2. Keep `fail_on_unmatched_files: true` unless the release contract is intentionally weakened and reviewed.
3. Route SARIF changes through CI/code-scanning docs and workflows, not the public release asset list.
