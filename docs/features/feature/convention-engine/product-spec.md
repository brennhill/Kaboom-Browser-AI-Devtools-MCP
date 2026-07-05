---
doc_type: product-spec
feature_id: feature-convention-engine
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
---

# Convention Engine Product Spec

## TL;DR

- Problem: AI agents repeat mistakes because they don't know a project's conventions. Code review catches drift after the fact. Every project reinvents the same quality rules.
- User value: The agent sees how the project already does things — before writing code that drifts. Convention enforcement without manual code review. Works on any codebase from day one.
- Architecture: Plugin-based convention discovery + enforcement. Universal principles, language plugins, framework plugins.

## The Four-Step Cycle

Convention enforcement operates as a continuous cycle that matures with the codebase:

### Step 1 — Discover what exists

The engine walks the codebase and finds patterns that repeat. Call-site patterns (`json.Marshal(` in 91 files), structural patterns (error handling style, JSON tag casing), and dependency usage patterns (which packages are used and how).

This runs automatically. No configuration needed. The engine observes the codebase and reports what it finds.

### Step 2 — Suggest what should exist

The engine identifies code that should follow an established convention but doesn't. Two mechanisms:

- **Mechanical**: The edit introduces `log.Printf(` but the project uses `slog.Error(` in 20 files. The convention summary is injected and the LLM flags the drift.
- **Pattern catalog**: An LLM pass (one-time) reads the codebase against a catalog of known software patterns (repository, command, middleware, etc.) and identifies which are present, absent-but-needed, or partially implemented.

Suggestions are presented to the developer for approval or rejection. Approved suggestions become enforced conventions (step 3). Rejected suggestions are recorded and not repeated.

### Step 3 — Enforce settled standards

Approved conventions become rules. The quality gate checks every edit against them. Violations produce actionable feedback: what the convention is, where the project already follows it, and what the edit should do instead.

Enforcement has two severity levels:
- **Convention** (informational): "This project uses `slog.Error(` — consider aligning." The LLM sees the context and adjusts.
- **Decision** (blocking): "DECISION-003: No direct stdout. Use WriteOutput." Explicit architectural rules in `decisions.json`.

### Step 4 — Update when standards change

When a team re-architects (migrating from `log.Printf` to `slog.Error`), the engine needs to know the migration direction. Without this, both patterns coexist and the engine can't recommend one over the other.

Migration declarations specify from/to patterns. During migration, new code follows the target pattern. Once migration is complete (zero remaining legacy usages), the migration converts to a permanent convention.

The anti-thrash mechanism: if a migration is declared but the codebase moves in the opposite direction, the engine surfaces the conflict rather than silently enforcing a dead migration.

## The 10 Universal Principles

Every plugin starts with these. They apply to every language, every framework, every codebase. They are the product's opinion on how professional software is built.

### U1 — Errors are values, not ignored

Every error return is checked. Errors carry context about what went wrong and where. No swallowing errors silently. No panic/throw for control flow.

**Go probe**: `_, _ = someFunc()` — discarded error. `os.WriteFile(path, data, 0644)` — unchecked return.
**TS probe**: `.catch(() => {})` — empty catch. `await someFunc()` without try/catch at a boundary. Unhandled promise rejection.

### U2 — Single responsibility

One concept per file. One purpose per function. If a function does two things, it's two functions. If a file has two unrelated concerns, it's two files.

**Probe**: Function length > configurable threshold. File length > configurable threshold (default 800 LOC). Multiple unrelated exports from one module.

### U3 — Separation of concerns

Data access doesn't live in handlers. Business logic doesn't live in transport. Presentation doesn't know about storage. The layers have clear boundaries and communicate through interfaces, not concrete types.

**Probe**: Raw `db.Query(` / `fetch(` in handler/controller code. Database imports in presentation layer files. Storage calls outside designated data access files.

### U4 — No magic globals

State is passed explicitly, not accessed through singletons or module-level mutable variables. Dependencies are injected, not imported and called directly. Configuration enters at the boundary and flows inward.

**Probe**: `var globalState = ...` with mutations. Module-level `let` that gets reassigned. Singleton pattern without explicit initialization. `os.Getenv()` / `process.env` deep in business logic instead of at startup.

### U5 — Immutability by default

Don't mutate what you can replace. Don't share what you can copy. Mutable shared state is the source of most concurrency bugs and most debugging sessions.

**Probe**: `let` where `const` suffices (TS). Append to a shared slice without a mutex (Go). Object mutation instead of spread. Direct array modification instead of map/filter.

### U6 — Fail fast, fail loud

Validate at boundaries. Don't pass bad data deeper into the system hoping something downstream will catch it. If a precondition isn't met, stop immediately with a clear error.

**Probe**: Nil/null checks deep in the call stack that should have been caught at the entry point. Late validation of inputs that were available earlier. Silent fallback to defaults instead of erroring on invalid config.

### U7 — Explicit over implicit

No magic string keys to look things up. No convention-based routing where renaming a file changes behavior. No metaprogramming that makes control flow invisible. A reader should be able to trace what happens by reading the code.

**Probe**: Map lookups with string literals as keys for dispatch. Reflection-based dependency injection. Dynamic imports based on computed strings. `eval()`.

### U8 — No raw resource access

Don't scatter `db.Query`, `http.Get`, `fs.readFile` through business logic. Wrap external dependencies. One place to add retry, timeout, logging, circuit breaking. Not twenty.

**Probe**: Direct database calls outside of `*_store.go` / `*_repo.go` / `*Repository.ts`. Raw `http.Get(` / `fetch(` outside of a client/service module. Filesystem calls outside of a storage abstraction.

### U9 — Testing is structural

Tests live next to what they test. Tests are self-contained — no shared mutable state between tests. Test setup is explicit, not inherited from a base class three levels up.

**Probe**: Test files in a separate directory tree from source (some projects do this intentionally — configurable). Tests that depend on execution order. Shared mutable setup objects modified by individual tests.

### U10 — Dead code is deleted

No commented-out blocks. No unused functions kept "just in case." No feature flags for features that shipped two years ago. Version control is the backup.

**Probe**: `// ` followed by valid code syntax (commented-out code). Exported functions with zero callers. Unreachable branches after early returns. Stale TODO comments older than N months.

## Plugin Architecture

### Plugin types

**Universal plugin** (always active): The 10 principles above. Every project gets these. Free tier.

**Language base plugins** (auto-activated by file extension):
- Go: error handling patterns, context propagation, interface design, constructor patterns, struct tags, concurrency primitives, test structure
- TypeScript: type safety (any/unknown), async patterns, null handling, module structure, class vs function, event patterns
- Python: type hints, exception handling, context managers, import organization
- C#: async/await, DI patterns, nullable references, LINQ usage

**Framework plugins** (auto-activated by import detection):
- Go + Gin, Go + Echo, Go + Chi, Go + stdlib HTTP
- TS + React, TS + Vue, TS + Next.js, TS + Express
- Python + Flask, Python + FastAPI, Python + Django

**Community plugins**: User-contributed, hosted in a registry.

### Plugin interface

Each plugin declares:

```
signals    — how to detect if this plugin applies (imports, file patterns)
patterns   — catalog of named patterns with detection/violation signals
probes     — per-edit mechanical checks for approved patterns
```

### Activation

1. Engine scans `go.mod` / `package.json` / `requirements.txt`
2. Matches import signals against registered plugins
3. Activates matching plugins automatically
4. `.kaboom.json` can override: force-enable, force-disable, or configure thresholds

### Configuration

```json
{
  "conventions": {
    "plugins": ["auto"],
    "thresholds": {
      "function_length": 50,
      "file_length": 800
    },
    "disabled_principles": ["U9"]
  }
}
```

`"plugins": ["auto"]` means auto-detect from dependencies. Explicit list overrides.

## Monetization

**Free tier**: Universal plugin (10 principles) + language base plugins. Call-site discovery. Per-edit convention summary injection.

**Pro tier**: Framework plugins. Pattern catalog LLM assessment. Migration support. Structural probes (error handling, JSON tags, etc.). Custom convention approval/rejection workflow.

**Enterprise tier**: Custom/proprietary plugins. Organization-wide convention enforcement across repos. Convention compliance reporting.

## Rollout

### Phase 1 — Discovery (built)
Call-site pattern discovery. Convention summary injection on every edit. Discovered probes replace hardcoded probe list.

### Phase 2 — Universal principles
Implement the 10 principle probes for Go and TypeScript. Mechanical checks, no LLM needed at edit time.

### Phase 3 — Suggestion + approval loop
Surface discovered conventions as suggestions. Store approvals/rejections in `.kaboom/conventions.json`. Approved conventions become enforced.

### Phase 4 — Pattern catalog + LLM pass
One-time LLM assessment of codebase against pattern catalog. Framework plugin activation. Migration declarations.

### Phase 5 — Plugin marketplace
Community plugin format. Registry hosting. Paid framework plugins.

## Non-Goals

- Replacing linters (ESLint, golangci-lint). We catch architectural and convention drift, not syntax errors.
- AST-level analysis. We use regex and frequency analysis, not full parsing. This keeps us zero-dep and fast.
- Enforcing style preferences (tabs vs spaces, brace placement). Formatters handle that.
- Real-time pair programming. This is edit-time context injection, not interactive coaching.
