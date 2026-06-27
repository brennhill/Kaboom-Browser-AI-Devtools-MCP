---
doc_type: guide
feature_id: feature-quality-gates
last_reviewed: 2026-03-07
---

# Quality Gates Setup Guide

Automated code quality enforcement via Claude Code hooks.

## Quick Start

### 1. Run setup (one command)

```
configure(what="setup_quality_gates")
```

This automatically:
- Creates `.kaboom.json` — points to your standards doc, configures thresholds
- Creates `kaboom-code-standards.md` — starter coding conventions
- Installs hooks into `.claude/settings.json` — merges with existing settings, never overwrites

After this single command, quality gates are active on every Edit/Write and output compression runs on every Bash command.

What gets injected:
- **On Edit/Write:** Your code standards doc, file size warnings, codebase convention examples, helper extraction suggestions
- **On Bash:** Compressed test/build output (91-99% token savings on verbose output)

### 2. (Optional) Add Haiku review

For belt-and-suspenders AI review, add a prompt hook to the Edit|Write matcher in `.claude/settings.json`:

```json
{
  "type": "prompt",
  "prompt": "Review this code change against the project standards and any quality gate findings above. Only flag clear violations. Respond {\"ok\": true} if acceptable, or {\"ok\": false, \"reason\": \"specific findings\"} if not.",
  "model": "haiku",
  "timeout": 30
}
```

### 3. Edit your standards

Open `kaboom-code-standards.md` and add your project's patterns. Be specific — vague rules cause false positives.

Good rule: "Request validation must use `validateAndRespond()` from `internal/util/guards.go`. Do not write inline if/else validation chains."

Bad rule: "Use good patterns." (flags everything)

## What Gets Checked

### By `kaboom-hooks quality-gate`

| Check | Trigger | Data injected |
|-------|---------|---------------|
| **Standards doc** | Every Edit/Write | Full standards content (first 150 lines) |
| **File size** | File > 90% of limit | Warning with line count and limit |
| **Convention detection** | New code uses a known pattern | Existing codebase examples + helper extraction suggestion if 2+ instances |

### By `kaboom-hooks compress-output`

| Pattern | Detection | Compression |
|---------|-----------|-------------|
| go test | Command or `--- PASS/FAIL` markers | Summary + failed tests only |
| jest/vitest | Command or `Test Suites:` markers | Summary + failure files |
| pytest | Command or `N passed` markers | Summary + failures |
| cargo test | Command or `test result:` markers | Summary + failures |
| go build/vet | Command | Error lines only |
| make | Command | Error/warning lines only |
| tsc | Command | `error TS` lines only |
| npm/webpack | Command | ERROR lines only |
| cargo build | Command | `error[E` lines only |
| Generic | >100 lines, no pattern | First 30 + last 20 lines |

### By the standards doc (AI self-review or Haiku)

| Default rule | Trigger |
|-------------|---------|
| Max 800 LOC per file | File exceeds limit |
| No orphan/dead code | Commented-out blocks, unused imports |
| Semantic naming | Non-descriptive names, abbreviations |
| Error handling | Ignored error returns, unstructured messages |
| Extract helpers | 3+ similar lines repeated |
| Structural patterns | 3+ switch branches, 4+ nesting levels, 50+ line functions |
| Test coverage | New functions without tests, bug fixes without regression tests |
| Security | Logged secrets, unvalidated external input |

## How It Works

```
AI calls Edit/Write
  -> PostToolUse hook fires
  -> `kaboom-hooks quality-gate` runs:
     1. Finds .kaboom.json (walks up from changed file)
     2. Reads code_standards doc
     3. Checks file line count vs limit
     4. Detects patterns in new code (http.Client{, map[string]func, type decls, etc.)
     5. Searches codebase for existing usage of those patterns
     6. If 2+ instances found, suggests extracting a shared helper
     7. Outputs findings as additionalContext (JSON)
  -> AI sees standards + conventions + findings in its context
  -> Fixes violations before proceeding

AI calls Bash (test/build)
  -> PostToolUse hook fires
  -> `kaboom-hooks compress-output` runs:
     1. Detects output pattern (go test, jest, tsc, etc.)
     2. Compresses to summary + errors only
     3. Posts savings to daemon for tracking
     4. Outputs compressed result as additionalContext (JSON)
  -> AI sees compressed output instead of verbose logs
```

## Configuration

### `.kaboom.json`

```json
{
  "code_standards": "kaboom-code-standards.md",
  "file_size_limit": 800,
  "duplicate_threshold": 3
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `code_standards` | `kaboom-code-standards.md` | Path to your standards doc |
| `file_size_limit` | `800` | Warn when files exceed this LOC |
| `duplicate_threshold` | `3` | Min lines for duplicate detection |

### Pointing to an existing doc

If you already have a conventions doc:

```json
{
  "code_standards": "docs/core/common-patterns.md"
}
```

## Token Savings

Token savings are tracked per session and across lifetime:
- **Session summary** logged to stderr on daemon shutdown
- **Lifetime stats** persisted to `~/.kaboom/stats/lifetime.json`

| Approach | Cost per edit |
|----------|--------------|
| Quality gate hook (inject standards) | ~200-400 tokens |
| Compression hook (Bash output) | Saves 91-99% on test/build output |
| Prompt hook (Haiku review) | ~$0.0001 |
| Manual review pass | ~2,000-5,000 tokens |

## Troubleshooting

**Hook not firing?**
- Verify `.claude/settings.json` exists under the project root
- Check that `matcher` matches the tool name exactly: `Edit|Write` or `Bash`
- Run `claude --debug` to see hook execution

**kaboom-hooks not found?**
- Install hooks standalone: `curl -fsSL https://kaboom.dev/install.sh | bash -s -- --hooks-only`
- Or install the full suite: `curl -fsSL https://kaboom.dev/install.sh | bash`
- Via npm: `npm install -g kaboom-agentic-devtools`

**Too many false positives?**
- Make rules more specific in the standards doc
- Focus on patterns that cause real bugs, not style preferences
- Remove generic rules and add project-specific ones
