---
doc_type: feature_index
feature_id: feature-quality-gates
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
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
  - docs/features/capture/
  - docs/features/generation/
  - docs/features/pilot/
  - docs/features/protocol/
  - docs/features/testing/
  - docs/specs/contracts/
  - docs/specs/reviews/agent-workflows/
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
  - scripts/check-file-length.sh
  - scripts/check-folder-size.cjs
  - scripts/check-dormant-tests.sh
  - scripts/contracts/check_go_test_determinism.go
  - scripts/contracts/goarchitecture/main.go
  - .go-architecture-baseline.json
  - scripts/test-js-sharded.sh
  - scripts/build/run-go-coverage.sh
  - scripts/build/merge-go-coverage.mjs
  - .prettierignore
  - internal/testsync/testsync.go
  - package.json
  - .github/workflows/ci.yml
test_paths:
  - scripts/lint-documentation.py
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/internal/toolconfigure/qualitygates/handler_test.go
  - cmd/browser-agent/tools_configure_quality_gates_test.go
  - cmd/hooks/main_test.go
  - internal/hook/hook_policy_test.go
  - internal/hook/compress_output_test.go
  - internal/hook/conventions_test.go
  - internal/tracking/token_tracker_test.go
  - internal/tracking/stats_endpoint_test.go
  - scripts/check-file-length.test.mjs
  - scripts/check-folder-size.test.mjs
  - scripts/contracts/check_go_test_determinism_test.go
  - scripts/contracts/goarchitecture/main_test.go
  - scripts/tests/contracts/go-coverage-profile.test.mjs
  - internal/testsync/testsync_test.go
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/test-install-hooks-only.sh
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
