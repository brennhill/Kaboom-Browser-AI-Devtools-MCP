---
doc_type: tech-spec
feature_id: feature-quality-gates
status: proposed
owners: []
last_reviewed: 2026-08-05
links:
  index: ./index.md
  product: ./product-spec.md
  setup_guide: ./setup-guide.md
code_paths:
  - cmd/hooks/main.go
  - internal/hook/hook_policy.go
  - internal/hook/conventions.go
  - internal/hook/compress_output.go
  - cmd/browser-agent/internal/toolconfigure/qualitygates/handler.go
  - cmd/browser-agent/tools_configure.go
  - internal/tracking/token_tracker.go
test_paths:
  - cmd/browser-agent/internal/toolconfigure/qualitygates/handler_test.go
  - cmd/hooks/main_test.go
  - internal/hook/hook_policy_test.go
  - internal/hook/conventions_test.go
  - internal/hook/compress_output_test.go
  - cmd/browser-agent/tools_configure_quality_gates_test.go
  - internal/tracking/token_tracker_test.go
---

# Quality Gates Tech Spec

> Plain language. Describes how setup, enforcement, and compression are wired across the MCP server, the standalone hooks binary, and the local daemon. No code.

## TL;DR

- Design: A standalone `kaboom-hooks` binary runs one subcommand per PostToolUse hook; the MCP server scaffolds config and installs the hook entries; the daemon aggregates token savings.
- Key constraints: Zero dependencies, short-lived process per invocation, silent on bad input, output only the `additionalContext` envelope.
- Rollout risk: Low — additive context injection; nothing blocks an edit.

## Architecture Overview

Quality gates span three binaries:

1. **`kaboom-hooks`** (`cmd/hooks/`) — the standalone hook runner. Each subcommand reads PostToolUse JSON from stdin, computes findings, and writes an `additionalContext` envelope to stdout.
2. **`kaboom-agentic-browser`** (`cmd/browser-agent/`) — the MCP server that scaffolds `.kaboom.json` and `kaboom-code-standards.md` and installs hook entries via `configure(what="setup_quality_gates")`.
3. **The local daemon** — receives best-effort token-savings posts from the compression hook and persists lifetime stats.

## Key Components

### Hook protocol — `internal/hook/hook_policy.go`

Defines the wire types. `Input` holds `tool_name`, `tool_input`, and `tool_response` (the JSON Claude Code sends). `ToolInputFields` pulls the common fields (`file_path`, `command`, `new_string`, `content`). `Output` carries `additionalContext` (tagged `SPEC:claude-code-hooks`, camelCase per protocol).

`DetectAgent` reads environment variables to identify Claude, Gemini, or Codex. `WriteOutput` adapts the envelope: Claude gets a flat `additionalContext`; Gemini gets it nested under `hookSpecificOutput`. It writes nothing when the context is empty, so a hook that finds nothing produces no output.

### Quality gate — `internal/hook/hook_policy.go`

`RunQualityGate(input)` is the core check on Edit/Write:

1. Bail out unless the tool is Edit or Write and the file exists.
2. `FindProjectRoot` walks up from the edited file for `.kaboom.json`.
3. `loadKaboomConfig` reads the config with safe defaults (`code_standards`, `file_size_limit` 800).
4. Inject the standards doc — first `maxStandardsLines` (150) lines, wrapped in `=== PROJECT CODE STANDARDS ===` markers.
5. File-size check: above the limit emits a `WARNING ... must be split`; above 90% emits an approaching-limit `NOTE`.
6. Inject the always-on convention summary (`ConventionSummary`).
7. When the edit's new content contains a known pattern, inject specific examples via `DetectConventions` and `FormatConventions`.
8. Append the `QUALITY GATE` review instruction.

The result is the joined parts, or nil when there is nothing to say.

### Convention detection — `internal/hook/conventions.go`

Documented in detail in the convention-engine tech spec. In short: it merges auto-discovered call-site probes with static probes (`http.Client{`, `map[string]func`, `sync.Mutex`, `chrome.storage.`, and so on) and `type X struct` declarations, searches the codebase for examples (capped, skipping vendored/generated/oversized files), and suggests extracting a helper at `helperThreshold` (2) instances.

### Output compression — `internal/hook/compress_output.go`

On Bash, `CompressOutput(input)` inspects the command and its output, detects the tool family (go test, jest/vitest, pytest, cargo test/build, go build/vet, make, tsc, npm/webpack), and collapses output to a summary plus the failing lines. Generic output over a threshold is reduced to head plus tail. It fires only when output exceeds the line threshold (so short output is untouched) and returns a `CompressResult` with category and before/after token counts.

### Setup handler — `cmd/browser-agent/internal/toolconfigure/qualitygates/handler.go`

`qualitygates.Handle`:

1. Resolves the project directory from `GetActiveCodebase`; validates `target_dir` is within the project (path-traversal guard).
2. Writes `.kaboom.json` if missing (never overwrites); reads the configured `code_standards` path.
3. Writes `kaboom-code-standards.md` with starter content only when the path is the default name and the file is missing.
4. `installClaudeCodeHooks` merges hook entries into `.claude/settings.json`: Edit/Write gets quality-gate + blast-radius + decision-guard + session-track; Read gets session-track; Bash gets compress-output + session-track. `containsManagedHooks` makes install idempotent by recognizing prior managed entries.
5. Returns config/standards paths, defaults, suggestions, and the install result.

### Token tracking — `internal/tracking/`

The compression hook posts `{category, tokens_before, tokens_after}` to the daemon (`/api/token-savings`) with a 200ms timeout so it never delays process exit. The daemon aggregates per-session savings, logs a summary on shutdown, and persists lifetime stats to `~/.kaboom/stats/lifetime.json`.

## Data Flow

```
Setup:
  configure(what="setup_quality_gates")
    -> resolve + validate target dir (within project)
    -> write .kaboom.json (if missing) + kaboom-code-standards.md (if default + missing)
    -> install hooks into .claude/settings.json (idempotent)

Enforcement (Edit/Write):
  Claude Code PostToolUse hook -> kaboom-hooks quality-gate
    -> FindProjectRoot -> loadKaboomConfig
    -> standards + size check + convention summary + matched examples + review line
    -> WriteOutput(additionalContext)  (agent-adapted)

Compression (Bash):
  Claude Code PostToolUse hook -> kaboom-hooks compress-output
    -> detect tool family -> summary + failures
    -> WriteOutput(additionalContext)
    -> POST /api/token-savings (best-effort, 200ms timeout)
```

## Implementation Strategy

Each hook is a short-lived process that does one thing and exits. The design favors silence: bad stdin, a missing project root, or no findings all result in zero output and a clean exit, so a misconfigured hook never breaks the host agent. The standalone binary lets users install hooks without the full MCP server (`--hooks-only`).

**Trade-offs:**
- Standalone binary vs in-server: a separate binary keeps hooks usable without the daemon and keeps the hot path dependency-free.
- Inject-on-every-edit vs match-only: the standards doc and convention summary are always injected (a few hundred tokens) so the model has context even when the edit contains no known probe.
- Best-effort stats posting vs synchronous: token savings are posted with a tight timeout so tracking never delays the agent.

## Edge Cases & Assumptions

- **No `.kaboom.json`**: the quality gate is silent.
- **Malformed config**: `loadKaboomConfig` falls back to defaults.
- **File deleted between edit and hook**: the gate bails on the `os.Stat` check.
- **Write with empty content field**: `extractNewContent` falls back to reading the just-written file.
- **`target_dir` outside the project**: setup rejects it (`ErrPathNotAllowed`).
- **Existing config/standards**: setup preserves them; it never overwrites.
- **Hooks already installed**: `containsManagedHooks` makes install a no-op.
- **Daemon unreachable**: compression still returns; the stats post simply fails silently.
- **Short Bash output**: below threshold, compression leaves it untouched.
- **Non-Claude agent**: `WriteOutput` adapts the envelope for Gemini/Codex.

## Performance

| Operation | Budget | Method |
|-----------|--------|--------|
| Quality gate (incl. file scan) | < 100ms | capped file/scan limits, cached discovery |
| Compress-output | < 20ms | single-pass detection and summary |
| Token-savings post | <= 200ms cap | HTTP client timeout, best-effort |

## Security Considerations

- **Locality**: hooks read local files and the local daemon only. No external network calls.
- **Stdout discipline**: hooks emit only the JSON envelope; stray output would break the host protocol.
- **Standards content**: the scaffolded standards doc carries no secrets; users are warned not to place credentials in it.
- **Path validation**: setup constrains `target_dir` to the project root to prevent writing config outside it.

## Validation

`cmd/hooks/main_test.go`, `internal/hook/hook_policy_test.go`, `internal/hook/conventions_test.go`, and `internal/hook/compress_output_test.go` cover the hooks. `cmd/browser-agent/tools_configure_quality_gates_test.go` covers setup. `internal/tracking/token_tracker_test.go` covers stats. The hook eval rig exercises the quality-gate and compress-output fixtures against the Kaboom codebase. Install behavior is covered by `scripts/release/install-upgrade-regression.contract.test.mjs` and `scripts/setup/test-install-hooks-only.sh`.
