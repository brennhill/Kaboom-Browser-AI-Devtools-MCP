---
doc_type: feature_index
feature_id: feature-quality-gates
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-07-29
code_paths:
  - kaboom-code-standards.md
  - cmd/browser-agent/internal/toolconfigure/qualitygates/handler.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - internal/tools/configure/capabilities/modespecs_configure.go
  - internal/schema/configure/properties_core.go
  - internal/schema/configure/properties_runtime.go
  - internal/hook/protocol.go
  - internal/hook/compress_output.go
  - internal/hook/quality_gate.go
  - internal/hook/convention_detect.go
  - internal/tracking/token_tracker.go
  - internal/tracking/stats_endpoint.go
  - cmd/hooks/main.go
  - scripts/check-file-length.sh
  - scripts/check-folder-size.cjs
  - scripts/check-dormant-tests.sh
  - scripts/test-js-sharded.sh
  - scripts/build/run-go-coverage.sh
  - scripts/build/merge-go-coverage.mjs
  - .prettierignore
  - internal/testsync/testsync.go
  - package.json
  - .github/workflows/ci.yml
test_paths:
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/internal/toolconfigure/qualitygates/handler_test.go
  - cmd/browser-agent/tools_configure_quality_gates_test.go
  - cmd/hooks/main_test.go
  - internal/hook/protocol_test.go
  - internal/hook/compress_output_test.go
  - internal/hook/quality_gate_test.go
  - internal/hook/convention_detect_test.go
  - internal/tracking/token_tracker_test.go
  - internal/tracking/stats_endpoint_test.go
  - scripts/check-file-length.test.mjs
  - scripts/check-folder-size.test.mjs
  - scripts/tests/contracts/go-coverage-profile.test.mjs
  - internal/testsync/testsync_test.go
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/test-install-hooks-only.sh
  - scripts/docs/features/check-feature-paths.test.mjs
  - tests/cli/contracts/root-metadata-branding.test.cjs
  - tests/site/gokaboom-domain-contract.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Quality Gates

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
