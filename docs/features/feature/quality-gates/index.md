---
doc_type: feature_index
feature_id: feature-quality-gates
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-08-26
code_paths:
  - scripts/docs/
  - scripts/maintenance/
  - scripts/quality/contracts/
  - scripts/quality/verification/
  - scripts/quality/verification/lint-hardening.sh
  - scripts/setup/
  - scripts/uat/orchestration/
  - scripts/uat/protocol/
  - scripts/uat/runners/
  - docs/audits/historical/
  - docs/planning/product/
  - docs/planning/release/
  - docs/setup/
  - docs/core/architecture/
  - docs/core/product/
  - docs/core/protocol/
  - docs/core/quality/
  - docs/core/reliability/
  - docs/architecture/boundaries/
  - docs/architecture/decisions/
  - docs/architecture/platform/
  - docs/architecture/runtime/
  - docs/architecture/workflows/
  - docs/features/feature-navigation.md
  - docs/features/capture/
  - docs/features/generation/
  - docs/features/pilot/
  - docs/features/protocol/
  - docs/features/testing/
  - docs/specs/contracts/
  - docs/specs/reviews/runtime-data/
  - docs/specs/versioning/
  - docs/architecture/diagrams/quality/5-layer-protection.md
  - kaboom-code-standards.md
  - cmd/browser-agent/internal/toolconfigure/qualitygates/handler.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - internal/tools/configure/capabilities/modespecs_configure.go
  - internal/schema/configure/properties_core.go
  - internal/schema/configure/properties_runtime.go
  - internal/hook/hook_policy.go
  - internal/hook/compress_output.go
  - internal/hook/conventions.go
  - internal/tracking/token_tracker.go
  - internal/tracking/stats_endpoint.go
  - cmd/hooks/main.go
  - scripts/quality/contracts/check-folder-size.cjs
  - scripts/contracts/complexity/main.go
  - scripts/contracts/layering/main.go
  - scripts/quality/contracts/complexity/check-complexity.mjs
  - scripts/quality/contracts/file-length/check-file-length.sh
  - scripts/quality/contracts/ts-strictness/check-ts-strictness.mjs
  - scripts/quality/contracts/bundle-size/check-bundle-size.cjs
  - scripts/security/check-secrets.sh
  - scripts/build/run-go-coverage.sh
  - scripts/quality/contracts/check-baseline-currency.sh
  - .secrets-allowlist
  - .gitleaks.toml
  - .function-length-baseline-go.json
  - .function-length-baseline-ts.json
  - .ts-strictness-baseline.json
  - .coverage-baseline.json
  - scripts/docs/generate-feature-navigation.py
  - scripts/docs/generate-feature-navigation.sh
  - scripts/quality/contracts/check-dormant-tests.sh
  - scripts/contracts/check_go_test_determinism.go
  - scripts/contracts/goarchitecture/main.go
  - .go-architecture-baseline.json
  - scripts/uat/runners/test-js-sharded.sh
  - scripts/build/run-go-coverage.sh
  - scripts/build/merge-go-coverage.mjs
  - scripts/build/openapi-tooling.test.mjs
  - .prettierignore
  - internal/testsync/testsync.go
  - package.json
  - .github/workflows/ci.yml
test_paths:
  - tests/docs/documentation-link-parser.test.js
  - scripts/docs/lint-documentation.py
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/internal/toolconfigure/qualitygates/handler_test.go
  - cmd/hooks/main_test.go
  - internal/hook/hook_policy_test.go
  - internal/hook/compress_output_test.go
  - internal/hook/conventions_test.go
  - internal/tracking/token_tracker_test.go
  - internal/tracking/stats_endpoint_test.go
  - scripts/quality/contracts/file-length/check-file-length.test.mjs
  - scripts/quality/contracts/check-folder-size.test.mjs
  - scripts/contracts/complexity/main_test.go
  - scripts/contracts/layering/main_test.go
  - .interface-baseline.json
  - scripts/quality/contracts/complexity/check-complexity.test.mjs
  - scripts/quality/contracts/ts-strictness/check-ts-strictness.test.mjs
  - scripts/quality/contracts/bundle-size/check-bundle-size.test.mjs
  - scripts/security/check-secrets.test.sh
  - scripts/contracts/check_go_test_determinism_test.go
  - scripts/contracts/goarchitecture/main_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - scripts/tests/contracts/go-coverage-profile.test.mjs
  - scripts/tests/contracts/go-coverage-baseline.test.mjs
  - scripts/build/openapi-tooling.test.mjs
  - internal/testsync/testsync_test.go
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/setup/test-install-hooks-only.sh
  - scripts/docs/features/check-feature-paths.test.mjs
  - tests/cli/contracts/root-metadata-branding.test.cjs
  - tests/site/gokaboom-domain-contract.test.js
  - tests/extension/contracts/tooling-contracts.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Quality Gates

The hook package is organized into five paired change families: hook request
policy, convention discovery/detection, session persistence/tracking, blast
radius, and output compression. Its package-level regression test enforces ten
files, and every owner remains below 800 lines; no compatibility filenames or
forwarding surfaces remain after the atomic migration.

The OpenAPI drift gate uses an exact lockfile-pinned generator. Its explicit
TypeScript 6 peer override is exercised by regenerating and comparing the full
schema, and the vulnerable transitive YAML parser is pinned to its patched
release so clean CI installs remain deterministic and audit-clean.

| Field         | Value                                   |
|---------------|-----------------------------------------|
| **Status**    | in-progress                             |
| **Tool**      | configure                               |
| **Mode**      | `what="setup_quality_gates"`            |
| **Schema**    | `internal/schema/configure/properties_runtime.go` |
| **Issue**     | [#506](https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp/issues/506) |

## Specs

- [Setup Guide](./setup-guide.md)

## Summary

Automated code quality enforcement that catches architectural drift, duplicate code, and pattern violations without burning tokens. Scaffolds `.kaboom.json` and `kaboom-code-standards.md` in the project root. Quality gates are enforced via Claude Code hooks that inject standards, detect conventions (searching the codebase for existing usage of patterns like `http.Client{`, handler maps, type declarations), and suggest helper extraction when 2+ instances exist. The managed hook binary is `kaboom-hooks`, and setup treats prior managed hook entries as replaceable during install/update.

Convention scanning applies extension, generated-file, size, and directory
filters through one shared source-walk boundary so detection and discovery
cannot drift.

`make check-structure` parses Go tests with the standard Go AST and rejects every
executable `time.Sleep` call. There is no baseline or update escape hatch: tests
must synchronize through controlled channels, fake clocks, observable lifecycle
events, or explicit process/transport seams.

The shared asynchronous test helpers verify goroutine readiness and teardown
through channel barriers. Their own tests never delay the scheduler and then
guess whether work completed; each assertion follows an observed lifecycle
transition.

The same target inventories mutable Go package variables and exported Go
declarations at their actual public boundary: the package. Moving declarations
between files in one package is surface-neutral, while a new package starts
with zero allowance and package totals can only ratchet downward. This lets
atomic file consolidation proceed without disguising genuine API or shared-
state growth, turning instance ownership and minimal public interfaces into
deterministic merge gates instead of review-only preferences.

The folder boundary inventories every authored file under first-party source,
test, package, script, site, specification, workflow, and documentation roots.
Tests, fixtures, Markdown, shell scripts, schemas, and assets count toward the
same ten-file ownership limit; only generated/build output and vendored code are
explicitly exempt. Existing violations are recorded in a downward-only baseline,
and CI independently regenerates that baseline to reject stale improvements.

`make check-complexity` enforces a cyclomatic complexity budget of 15 per
authored function over production Go (`cmd/`, `internal/`, gocyclo counting)
and authored TS/JS (`src/`, `scripts/`, `packages/`, ESLint-style counting with
nested functions judged separately). Tests and generated output are exempt —
generated dom-primitives files are covered through their
`scripts/templates/*.tpl` sources, and wire types through their Go source of
truth. The Go checker is dependency-free (`go/ast`); the TS checker resolves
the repo's pinned TypeScript dev dependency for parsing. Both report worst-first
with file:line and fail the run; waivers are not allowed.

The same checkers enforce two further per-function budgets: at most six
parameters (hard limit — wider call sites must group into a parameter struct)
and at most 80 body lines, ratcheting like the folder gate through
`.function-length-baseline-go.json` / `.function-length-baseline-ts.json` so
injected/serialized functions that legitimately carry a whole payload are
frozen at their current size and may only shrink.

`make check-ts-strictness` ratchets TypeScript strictness escapes out of
authored `src/`: zero `@ts-nocheck` directives and a never-growing explicit
`any` annotation count (`.ts-strictness-baseline.json`, updated only via
`make ts-strictness-baseline-update`).

`make check-secrets` pattern-scans tracked files for full credential formats
(AWS/GitHub/Stripe/Slack/Anthropic/OpenAI keys, private key blocks, and
similar) in under twenty seconds; the pre-commit hook runs the same scan over
exactly the staged files, including renames and type-changes. A bare `*` or
`**` glob in `.secrets-allowlist` fails the scan closed instead of silently
silencing every finding. `make security-check` adds pinned gitleaks over the
full git history (`.gitleaks.toml` extends the default rules with custom
OpenAI key rules and anchored fixture paths). Intentional fake-key
fixtures are listed with reasons in `.secrets-allowlist` and mirrored in
`.gitleaks.toml`; a path on those lists may never contain a real credential.

Go coverage floors are enforced by `run-go-coverage.sh` as the maximum of the
historical minimum and the upward-only ratchet in `.coverage-baseline.json`;
`GO_COVERAGE_MINIMUM` can only raise the bar, and
`make coverage-baseline-update` locks in demonstrated improvements. A missing
baseline (first run) floors to 0, but an unparseable file, a wrong version, or
a non-numeric `go_total_percent` fails the run instead of silently degrading
the floor; the update path likewise refuses to lower the recorded baseline
(the run already cleared the floor above it) and refuses to overwrite a
corrupt baseline file. `scripts/tests/contracts/go-coverage-baseline.test.mjs`
pins those shell-embedded node fragments against fixtures.

`make check-baseline-currency` re-freezes every deterministic ratchet baseline
(`.function-length-baseline-go.json`, `.function-length-baseline-ts.json`,
`.ts-strictness-baseline.json`, `.interface-baseline.json`) and fails on any
byte difference, so a stale or hand-edited ratchet cannot survive CI; the
committed file is always restored afterward, leaving the tree clean. The TS
length baseline writer emits sorted keys so regeneration is byte-identical
across platforms. `.coverage-baseline.json` is excluded because it records
measured coverage that only a full `make test-cover` run can regenerate.
`make check-bundle-size` caps every artifact `compile-ts` emits at 250KB per
file and 600KB total, so extension footprint growth is a reviewable event;
missing bundle artifacts fail rather than passing as a size win.

`make check-layering` pins the hexagonal shape of the Go tree with a hard
dependency matrix: `internal/**` never imports `cmd/**`, `internal/types`
is the innermost leaf, `internal/mcp` (protocol) never imports the tool
domain or the capture port, `internal/schema` and `internal/tools` remain
siblings, and `capture.NewCapture` is constructed only by the composition
root in `cmd/browser-agent`. The same gate enforces interface segregation
(exported interfaces ≤7 methods, hard) and, via a downward-only
`.interface-baseline.json`, retires producer-owned interfaces (only
production implementations live in the declaring package) and dead
contracts (no implementation anywhere; same-package test fakes keep
cross-boundary seams alive and are exempt).

DRY is enforced on both languages: `make check-duplicates` holds
`src/background src/popup src/lib src/content src/inject` to zero jscpd
clones and ratchets `tests/extension` at 7%, while
`make lint-duplicates-go` runs the pinned golangci-lint dupl linter
(threshold 80 tokens, tests excluded) over production Go; the identical
dupl and layering invocations run in CI beside the depguard gate.

Prettier checks authored source and configuration while excluding the three
minified action-family DOM primitives whose canonical representation is owned
by `generate-dom-primitives.js`; generator drift checks validate those files
instead.

`make test-cover` performs one uncached, cross-package Go run and also captures
normally exiting black-box CLI binaries. The package profile supplies the
canonical block topology; differently shaped copies emitted by other test
binaries are projected onto that topology and merged by maximum execution
count. Cross-package behavior is therefore retained without adding phantom
denominator blocks for the same source statement.

Hosted CI invokes the canonical isolated `make test-performance` target rather
than copying its environment or test selection into workflow YAML. The tooling
contract fails if that SLO lane is removed, preventing a locally defined
performance gate from silently disappearing from pull requests.

## Architecture

1. **`kaboom-hooks` binary** — standalone CLI for Claude Code hooks (`cmd/hooks/`), installable independently
2. **`.kaboom.json`** — minimal config pointing to the standards doc, committed to repo
3. **`kaboom-code-standards.md`** — plain markdown coding conventions, read by Haiku
4. **Claude Code hooks** — PostToolUse on Edit/Write (quality gates) and Bash (output compression)
5. **Haiku review** — ~$0.0001/edit, findings injected as `additionalContext`
6. **Token tracking** — `internal/tracking/` tracks compression savings, logs on shutdown, persists lifetime stats to `~/.kaboom/stats/lifetime.json`

## Install

```bash
# Hooks only (standalone)
curl -fsSL https://gokaboom.dev/install.sh | bash -s -- --hooks-only

# Full Kaboom (includes hooks)
curl -fsSL https://gokaboom.dev/install.sh | bash
```

## Setup

```
configure(what="setup_quality_gates")
```

Creates both files with sensible defaults. Does not overwrite existing files.
