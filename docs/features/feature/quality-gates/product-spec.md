---
doc_type: product-spec
feature_id: feature-quality-gates
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  setup_guide: ./setup-guide.md
---

# Quality Gates Product Spec

## TL;DR

- Problem: AI coding agents drift from a project's standards, reintroduce patterns that already exist, grow files past sane limits, and bury real signal under verbose test/build output — all of which burn tokens and produce review churn.
- User value: Automated, low-cost code-quality enforcement that injects the project's standards and existing conventions at the moment of each edit, and compresses noisy command output so the agent sees signal instead of scroll.
- Surfaces: `configure(what="setup_quality_gates")` for one-command setup, the standalone `kaboom-hooks` binary for enforcement, and `.kaboom.json` plus `kaboom-code-standards.md` as the committed configuration.

## Problem

When an AI agent edits code, it does not automatically know the project's conventions. It re-derives patterns, creates a second `http.Client{}` when a shared one exists, lets files grow unbounded, and ignores the structured-error format the team agreed on. Catching this in human review is expensive and late.

Separately, running tests and builds produces hundreds of lines of output. Most of it is passing noise. Feeding all of it back into the agent's context wastes tokens and hides the few lines that matter — the failures.

There is no lightweight, automatic mechanism that (a) reminds the agent of the standards and existing conventions on every edit and (b) strips verbose command output down to its signal.

## Solution

Quality gates run as Claude Code (and Gemini/Codex) PostToolUse hooks, delivered by the standalone `kaboom-hooks` binary:

1. **On every Edit/Write**, the quality gate finds the project's `.kaboom.json`, injects the standards doc, warns when a file approaches or exceeds its line limit, injects the top auto-discovered conventions, and — when the edit uses a known pattern — shows concrete existing examples and suggests extracting a shared helper when the pattern already appears in two or more files.

2. **On every Bash command**, output compression detects the tool (go test, jest/vitest, pytest, cargo, tsc, build tools) and collapses the output to a summary plus the failures, saving 91–99% of tokens on verbose runs. Savings are posted to the local daemon for tracking.

3. **One-command setup** via `configure(what="setup_quality_gates")` scaffolds `.kaboom.json` and `kaboom-code-standards.md`, and installs the hook entries into `.claude/settings.json` without overwriting existing settings.

Findings are injected as `additionalContext`, so the primary model sees them inline and fixes violations before proceeding. An optional Haiku prompt hook adds a cheap second-opinion review at roughly $0.0001 per edit.

## User Stories

- As a developer, I want one command to turn on quality gates so that I do not hand-write hook configuration.
- As an AI coding agent, I want the project's standards and existing conventions injected on each edit so that I write code that fits the codebase the first time.
- As a developer, I want files that approach the size limit flagged so that the agent splits them before they become unmaintainable.
- As an AI agent under a token budget, I want verbose test output compressed to its failures so that I do not waste context on passing noise.
- As a team lead, I want the standards doc and thresholds committed to the repo so that enforcement is shared and versioned.

## Product Contract

### Setup — `configure(what="setup_quality_gates")`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `target_dir` | string | active codebase | Directory to scaffold; must be within the project root |

The handler writes `.kaboom.json` and `kaboom-code-standards.md` (only if missing — it never overwrites), installs hook entries, and returns `config_path`, `defaults`, `suggestions`, `hooks_installed`, and `settings_path`.

### Configuration — `.kaboom.json`

| Field | Default | Description |
|-------|---------|-------------|
| `code_standards` | `kaboom-code-standards.md` | Path to the standards doc (can point at an existing conventions file) |
| `file_size_limit` | `800` | Warn at 90% of this, require split above it |
| `duplicate_threshold` | `3` | Minimum lines for duplicate detection |

### Enforcement — `kaboom-hooks`

The binary exposes one subcommand per hook: `quality-gate`, `compress-output`, `session-track`, `blast-radius`, `decision-guard`. Each reads PostToolUse JSON from stdin and writes an `additionalContext` envelope to stdout, auto-detecting Claude, Gemini, or Codex.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | One-command setup that scaffolds config + standards and installs hooks without overwriting | must |
| R2 | Inject the standards doc (first 150 lines) on every Edit/Write | must |
| R3 | Warn at 90% of the file size limit and require a split above it | must |
| R4 | Inject the top auto-discovered conventions on every edit | must |
| R5 | Show existing examples and suggest a shared helper at 2+ instances when the edit matches a pattern | must |
| R6 | Compress verbose Bash output to a summary plus failures | must |
| R7 | Track token savings per session and across lifetime | should |
| R8 | Auto-detect the calling agent and adapt the output envelope | must |
| R9 | Be installable standalone (`--hooks-only`) or as part of the full suite | should |
| R10 | Treat prior managed hook entries as replaceable during install/update | should |

## Non-Goals

- Replacing linters and formatters (ESLint, golangci-lint, Prettier). Quality gates catch architectural and convention drift, not syntax or style.
- Full AST analysis. Detection uses regex and frequency, keeping the binary zero-dependency and fast.
- Blocking edits. Hooks inject context; they do not reject changes.
- Transmitting any code off the local machine.

## Assumptions

- A1: The project has (or will get) a `.kaboom.json` at its root, found by walking up from the edited file.
- A2: The standards doc is written with specific, actionable rules; vague rules cause false positives.
- A3: The host agent honors PostToolUse hooks and reads `additionalContext`.
- A4: The local daemon is reachable for best-effort token-savings posting; if not, compression still works.
