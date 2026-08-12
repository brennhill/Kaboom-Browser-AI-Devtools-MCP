---
doc_type: tech-spec
feature_id: feature-convention-engine
status: proposed
owners: []
last_reviewed: 2026-08-05
links:
  index: ./index.md
  product: ./product-spec.md
code_paths:
  - internal/hook/conventions.go
  - internal/hook/hook_policy.go
test_paths:
  - internal/hook/conventions_test.go
  - internal/hook/hook_policy_test.go
  - internal/hook/eval/testdata/quality-gate/
  - internal/hook/eval/testdata/u01-errors-not-ignored/
---

# Convention Engine Tech Spec

> Plain language. Describes how Phase 1 of the convention engine works inside the `kaboom-hooks` quality gate. No new binary — the engine runs as part of the existing PostToolUse Edit/Write hook.

## TL;DR

- Design: Walk the project, find call-site patterns that repeat across 3+ files, cache them per project root and language, and inject the top patterns plus targeted examples on every edit.
- Key constraints: Zero dependencies, regex-and-frequency only (no AST), 5-minute cache, file and scan caps to stay fast.
- Rollout risk: Low — additive context injection inside an existing hook; nothing is blocked.

## Architecture Overview

The convention engine is the discovery-and-enforcement layer that the quality gate calls on each Edit/Write. It has three cooperating pieces:

1. **Discovery** (`convention_discover.go`) — walks the codebase and extracts call-site patterns that repeat. This answers "what does this project already do?"
2. **Detection** (`convention_detect.go`) — matches the current edit against discovered patterns and static probes, then searches the codebase for concrete examples. This answers "does this edit align with what the project does, and where can the model see the existing way?"
3. **Injection** (`quality_gate.go`) — assembles the convention summary and per-pattern examples into the `additionalContext` the agent receives.

The engine deliberately avoids full parsing. It uses regular expressions and frequency counting so it stays zero-dependency and completes inside the hook's latency budget.

## Key Components

### Discovery engine — `DiscoverConventions(projectRoot, ext)`

Walks `projectRoot` with `filepath.WalkDir`, skipping vendored, generated, hidden, and oversized files. For each source file in the language family, it applies a call-site regular expression and records, per pattern, the set of distinct files it appears in.

- **Go call-site regex** (`goCallSite`): `\b([a-z][a-zA-Z]*)\.([A-Z][a-zA-Z]*)\(` — captures `pkg.ExportedFunc(` and `receiver.Method(`, the dominant Go convention shape.
- **TS/JS call-site regex** (`tsCallSite`): `\b([a-zA-Z][a-zA-Z]*)\.([a-zA-Z][a-zA-Z]*)\(` — captures `obj.method(`.
- **Noise filtering**: `goNoise` (90+ entries) and `tsNoise` (25+ entries) drop patterns so universal they carry no convention signal (`fmt.Sprintf(`, `strings.Contains(`, `JSON.stringify(`, `console.log(`, and so on).

After the walk, patterns appearing in fewer than `discoveryMinFiles` (3) distinct files are dropped. The rest are sorted by file count descending and truncated to `discoveryMaxProbes` (20).

Caps that protect latency:
- `discoveryMaxFiles` = 500 files scanned per walk.
- `maxFileSizeForScan` = 100KB per file (skips bundled/generated blobs).
- `discoveryCacheTTL` = 5 minutes.

### Discovery cache

`discoveryCache` is a process-level `sync.RWMutex`-guarded map keyed by `projectRoot + "\x00" + ext`. A cache hit within the 5-minute TTL returns the prior result without re-walking. Because hooks are short-lived, separate invocations within a session benefit only when the process is reused; the cache primarily protects repeated calls within a single hook process and keeps the design ready for a long-lived host.

### Detection — `DetectConventions(filePath, projectRoot, newContent)`

Merges **discovered probes** (`DiscoveredProbes`) with a small list of **static probes** that the call-site regex cannot find:

```
http.Client{   map[string]func   sync.Mutex   sync.RWMutex
new Map<        new Set<          chrome.storage.   chrome.runtime.
```

It also detects `type X struct` declarations via `typePattern` and adds `type X struct` as a duplicate-detection probe. Every probe that appears in the edit's new content is then handed to `searchProject`, which walks the codebase **once for all probes** (same caps and skip rules) and collects up to `maxExamplesPerProbe` (5) example lines per probe in `relative/path:line: content` form, one per file. Detection reports at most `maxConventionsToReport` (3) patterns per edit.

Searching every probe in one pass is a performance requirement, not a style choice: a walk per probe re-read the same files once per probe for identical results, because each walk covered the same first `maxFilesToScan` files in the same order. The walk stops early once every probe has its 5 examples.

### Convention summary — `ConventionSummary(projectRoot, ext)`

Returns the top `maxSummaryConventions` (10) discovered patterns for the file's language as a compact block:

```
=== PROJECT CONVENTIONS (auto-discovered) ===
This project consistently uses these patterns — align new code accordingly:
  json.Marshal( (91 files)
  ...
=== END PROJECT CONVENTIONS ===
```

This is injected on **every** edit, even when the edit contains no matching probe, so the model can judge drift from context.

### Example formatting — `FormatConventions(matches)`

Renders each matched pattern with its examples under a `=== CODEBASE CONVENTIONS (match existing patterns) ===` block. When a pattern already exists in `helperThreshold` (2) or more files, it appends a `SUGGESTION: Consider extracting a shared helper` note so the model reuses existing code instead of introducing a variant.

## Data Flow

```
AI calls Edit/Write
  -> PostToolUse hook: kaboom-hooks quality-gate
  -> RunQualityGate(input)
       1. FindProjectRoot (walk up for .kaboom.json)
       2. Inject standards doc (first 150 lines)
       3. File size check
       4. ConventionSummary(projectRoot, ext)  ── DiscoverConventions (cached)
       5. DetectConventions(filePath, projectRoot, newContent)
            - merge discovered + static probes
            - match probes against the edit
            - searchProject for examples (one walk, all probes)
            - FormatConventions (+ helper suggestion at 2+)
       6. Append the review instruction
  -> additionalContext returned to the agent
```

## Implementation Strategy

The engine is implemented entirely inside `internal/hook`. It introduces no new subcommand and no new config beyond the optional `duplicate_threshold` already present in `.kaboom.json`. Discovery and detection share helpers (`matchesExtension`, `extensionFamily`, `isGenerated`, `skipDirs`) so the walk rules stay consistent across both passes.

**Language families** (`extensionFamily`):
- `.go` → `.go`
- `.ts`/`.tsx` → `.ts .tsx .js .jsx`
- `.js`/`.jsx` → `.js .jsx .ts .tsx`
- `.py` → `.py`
- `.rs` → `.rs`

**Trade-offs:**
- Regex/frequency vs AST: regex is approximate but zero-dep and fast. The product spec lists AST analysis as an explicit non-goal.
- Inject-always summary vs match-only: the always-on summary costs a few hundred tokens per edit but lets the model catch drift even when the edit does not contain a known probe.
- Process-level cache vs persistent index: the 5-minute in-memory cache is simple and safe; a persistent index is deferred.

## Edge Cases & Assumptions

- **No `.kaboom.json`**: `FindProjectRoot` returns empty and the quality gate (and thus the engine) does nothing.
- **Large/generated files**: skipped via `maxFileSizeForScan` and `isGenerated` (`.bundled.`, `.min.`, `.map`).
- **Huge repositories**: scans cap at 500 files; discovery is best-effort, not exhaustive.
- **Universal patterns**: filtered by the noise sets so they never surface as conventions.
- **Empty edit content**: detection returns nil; only the always-on summary may appear.
- **Unknown extension**: `extensionFamily` falls back to the literal extension and the Go call-site regex.
- **Assumption**: the codebase is reasonably consistent, so frequency is a useful proxy for intent. Inconsistent codebases simply produce fewer high-count patterns.

## Risks & Mitigations

- **Risk: false conventions from coincidental repetition.** Mitigation: the 3-file minimum plus noise filtering removes most accidental matches; the model receives examples, not hard rules.
- **Risk: latency on large trees.** Mitigation: file-count and file-size caps, plus the 5-minute cache.
- **Risk: token cost of always-on summary.** Mitigation: capped at 10 patterns in a compact one-line-per-pattern format.
- **Risk: stale cache after a large refactor.** Mitigation: 5-minute TTL bounds staleness; the next walk picks up changes.

## Performance

| Operation | Budget | Method |
|-----------|--------|--------|
| Discovery walk (cold) | within the quality-gate hook budget | capped at 500 files, 100KB each |
| Discovery (warm) | negligible | 5-minute in-memory cache |
| Per-edit detection + search | a few example searches | capped at 3 reported patterns, 5 examples each |

## Validation

Phase 1 is validated by `convention_discover_test.go`, `convention_detect_test.go`, and `quality_gate_test.go`, plus eval fixtures under `internal/hook/eval/testdata/quality-gate/` and the universal-principle directories `u01`..`u10`. The eval rig runs these against the Kaboom codebase itself, so discovery is exercised on a real import graph.
