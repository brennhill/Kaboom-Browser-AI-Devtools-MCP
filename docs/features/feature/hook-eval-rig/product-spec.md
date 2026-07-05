---
doc_type: product-spec
feature_id: feature-hook-eval-rig
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  tech: ./tech-spec.md
---

# Hook Eval Rig Product Spec

## TL;DR

- Problem: We build hooks that claim to save tokens and prevent mistakes, but we have no way to prove it. Without measurement, we can't tell if a hook helps, hurts, or does nothing.
- User value: Confidence that each hook delivers measurable improvement. Data-driven decisions about which hooks to enable. Regression detection when hook logic changes.
- Binary: `kaboom-hooks eval` (subcommand) + `scripts/eval/` (orchestration)

## Requirements

### EVAL_001: Three evaluation tiers

The rig operates at three levels, each with increasing realism and decreasing control:

**Tier 1 — Unit evals (deterministic, fast, CI-friendly)**
Feed synthetic hook inputs to each hook, verify the output matches expected context injection. No AI agent involved. Answers: "Does the hook produce the right output for a given input?"

**Tier 2 — Integration evals (controlled codebase, scripted tasks)**
Run hooks against purpose-built test codebases with known dependency graphs, standards violations, and decision files. Scripted sequences of tool uses simulate an AI session. Answers: "Does the hook inject the right context for realistic coding scenarios?"

**Tier 3 — Live session metrics (real usage, aggregated)**
Collect metrics from real AI coding sessions (opt-in). Token counts, file re-reads, hook injection counts, test pass/fail correlation. Answers: "How much do hooks improve real-world AI coding?"

### EVAL_002: Metrics to measure

Every eval must produce these metrics:

| Metric | Unit | What it measures |
|--------|------|-----------------|
| **tokens_injected** | count | Tokens added to context by all hooks combined |
| **tokens_saved** | count | Tokens eliminated by compress-output |
| **redundant_reads_prevented** | count | File re-reads that session-track caught |
| **blast_radius_warnings** | count | Times blast-radius injected dependency warnings |
| **decisions_enforced** | count | Times decision-guard injected a rule |
| **standards_injected** | count | Times quality-gate injected the standards doc |
| **conventions_detected** | count | Codebase patterns found by convention detection |
| **hook_latency_p50** | ms | Median hook execution time |
| **hook_latency_p99** | ms | 99th percentile hook execution time |

### EVAL_003: Synthetic test codebases

Purpose-built codebases for integration evals. Each designed to exercise specific hook behavior:

**`eval/codebase-go-web`** — Go web server (~30 files)
- Known import graph: `main.go` -> `routes.go` -> `handlers.go` -> `db.go`
- Standards doc with 5 specific rules
- 3 locked decisions in `.kaboom/decisions.json`
- 2 convention probes (http.Client, handler map pattern)
- 1 file at 780/800 LOC (near limit)

**`eval/codebase-ts-react`** — TypeScript React app (~25 files)
- Component hierarchy with shared hooks
- Standards doc with TypeScript-specific rules
- Import chain for blast-radius testing
- `chrome.storage` usage for convention detection

**`eval/codebase-py-api`** — Python API (~20 files)
- Flask/FastAPI structure
- Import chains across packages
- Pytest output for compress-output testing

### EVAL_004: Scripted session sequences

Each test codebase comes with scripted sequences — ordered lists of hook inputs that simulate an AI coding session:

```json
[
  {"tool_name": "Read", "tool_input": {"file_path": "routes.go"}},
  {"tool_name": "Read", "tool_input": {"file_path": "handlers.go"}},
  {"tool_name": "Edit", "tool_input": {"file_path": "handlers.go", "new_string": "func NewHandler() http.Client{...}"}},
  {"tool_name": "Read", "tool_input": {"file_path": "routes.go"}},
  {"tool_name": "Bash", "tool_input": {"command": "go test ./..."}, "tool_response": {"stdout": "--- FAIL: TestHandler..."}}
]
```

For each step, the rig:
1. Runs every installed hook against that input
2. Records what each hook outputs (or doesn't output)
3. Measures latency
4. Validates output against expected behavior

The scripted sequences test specific scenarios:
- **Redundant read detection**: Read file A, edit file A, read file A again → session-track should warn
- **Blast radius**: Edit an exported function in a heavily-imported file → blast-radius should list dependents
- **Decision violation**: Introduce `http.Client{` when a decision says to use the shared client → decision-guard should fire
- **Convention drift**: Add a handler map when the codebase already has 3 → convention detection should suggest reuse
- **Compression**: Run `go test` with 200 lines of output → compress-output should reduce to summary

### EVAL_005: Baseline comparison (A/B)

For each scripted sequence, run twice:
1. **Baseline** — no hooks enabled. Record raw tool inputs/outputs.
2. **With hooks** — all hooks enabled. Record hook outputs + metrics.

Compute deltas:
- `tokens_saved = baseline_tokens - hooks_tokens` (from compression)
- `context_added = sum(tokens_injected)` (cost of hook context)
- `net_token_impact = tokens_saved - context_added`
- `redundant_reads_baseline` vs `redundant_reads_hooks`

Report as a table:
```
Scenario: go-web-refactor
  Baseline: 15,200 tokens consumed
  With hooks: 12,800 tokens consumed
  Net savings: 2,400 tokens (15.8%)
  Redundant reads prevented: 3
  Blast radius warnings: 2
  Decisions enforced: 1
  Hook latency (p50): 8ms
```

### EVAL_006: Regression detection

Every eval run produces a JSON report. CI compares against the previous run's report. Alert if:
- Any hook latency regresses by > 20%
- Token injection count changes unexpectedly (hook producing more/less context than before)
- Expected hook outputs stop matching (hook logic changed)

### EVAL_007: Live session metrics (opt-in)

Extend the existing token-savings tracking to record per-hook metrics during real sessions:

```json
{
  "session_id": "abc123",
  "hooks_enabled": ["quality-gate", "compress-output", "session-track", "blast-radius", "decision-guard"],
  "duration_minutes": 45,
  "metrics": {
    "tokens_injected": 3200,
    "tokens_saved": 18500,
    "redundant_reads_prevented": 7,
    "blast_radius_warnings": 4,
    "decisions_enforced": 2,
    "hook_invocations": 156,
    "hook_latency_p50_ms": 6,
    "hook_latency_p99_ms": 42
  }
}
```

Persisted to `~/.kaboom/stats/sessions/` alongside the existing `lifetime.json`. Users can opt-in via `.kaboom.json`:

```json
{
  "eval_metrics": true
}
```

### EVAL_008: CLI report command

`kaboom-hooks eval` runs the tier-1 and tier-2 evals and prints a summary:

```
kaboom-hooks eval

Running unit evals...
  quality-gate:    12/12 passed (avg 4ms)
  compress-output: 8/8 passed (avg 3ms)
  session-track:   10/10 passed (avg 2ms)
  blast-radius:    9/9 passed (avg 18ms)
  decision-guard:  6/6 passed (avg 2ms)

Running integration evals...
  codebase-go-web:
    Scenario: refactor-handler    PASS  (net savings: 2,400 tokens, 15.8%)
    Scenario: add-endpoint        PASS  (net savings: 1,100 tokens, 8.2%)
    Scenario: fix-bug             PASS  (net savings: 3,800 tokens, 22.1%)
  codebase-ts-react:
    Scenario: extract-component   PASS  (net savings: 900 tokens, 6.4%)
    Scenario: rename-hook         PASS  (net savings: 1,500 tokens, 11.3%)

All evals passed. 45/45 scenarios.
Aggregate: 9,700 tokens saved across 5 scenarios (avg 12.8% savings).
```

### EVAL_009: Performance budget enforcement

The eval rig enforces the performance budgets defined in each hook's spec:

| Hook | Budget | Measured in eval |
|------|--------|-----------------|
| session-track | < 20ms | Yes — fail if exceeded |
| blast-radius (warm) | < 50ms | Yes |
| blast-radius (cold) | < 250ms | Yes |
| decision-guard | < 15ms | Yes |
| quality-gate | < 100ms | Yes (includes file scan) |
| compress-output | < 20ms | Yes |

CI fails if any hook exceeds its budget on 3 consecutive runs (avoids flaky timing failures).

## Non-Goals

- Measuring AI reasoning quality (we measure what hooks inject, not whether the AI follows it)
- Full end-to-end AI benchmarks (too slow, non-deterministic, requires API keys)
- A/B testing framework for users (this is internal engineering tooling)
- Statistical significance testing (small sample sizes, directional metrics are sufficient)
