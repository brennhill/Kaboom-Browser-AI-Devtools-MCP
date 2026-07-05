---
doc_type: tech-spec
feature_id: feature-hook-eval-rig
status: proposed
owners: []
last_reviewed: 2026-07-05
links:
  index: ./index.md
  product: ./product-spec.md
code_paths:
  - internal/hook/eval/
  - cmd/hooks/main.go
  - scripts/eval/
test_paths:
  - internal/hook/eval/eval_test.go
  - cmd/hooks/eval_test.go
---

# Hook Eval Rig Tech Spec

## TL;DR

- Design: Three-tier eval framework — unit (Go tests), integration (scripted sessions against real codebases), live (opt-in session metrics)
- Key constraints: Deterministic, CI-friendly, no AI API keys needed for tier 1-2
- Rollout risk: Low — eval-only, no changes to hook runtime behavior

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    kaboom-hooks eval                    │
│                                                          │
│  Tier 1: Unit Evals                                      │
│  ┌─────────────────────────────────────────────────┐     │
│  │  internal/hook/eval/                             │     │
│  │  - Synthetic hook inputs (JSON fixtures)         │     │
│  │  - Expected outputs (golden files)               │     │
│  │  - Latency benchmarks                            │     │
│  │  - Run as `go test` — CI-integrated              │     │
│  └─────────────────────────────────────────────────┘     │
│                                                          │
│  Tier 2: Integration Evals                               │
│  ┌─────────────────────────────────────────────────┐     │
│  │  scripts/eval/                                   │     │
│  │  - Real codebases pinned at specific commits     │     │
│  │  - Scripted session sequences (JSONL)            │     │
│  │  - A/B comparison (hooks on vs off)              │     │
│  │  - Run as `kaboom-hooks eval` or `make eval`   │     │
│  └─────────────────────────────────────────────────┘     │
│                                                          │
│  Tier 3: Live Metrics                                    │
│  ┌─────────────────────────────────────────────────┐     │
│  │  internal/tracking/                              │     │
│  │  - Per-hook invocation counters                  │     │
│  │  - Latency histograms                            │     │
│  │  - Token injection/savings aggregation           │     │
│  │  - Written to ~/.kaboom/stats/sessions/        │     │
│  └─────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
```

## Tier 1: Unit Evals

### Fixture format

Each hook has a directory of test fixtures under `internal/hook/eval/testdata/`:

```
internal/hook/eval/testdata/
  quality-gate/
    001-standards-injection.json      # input
    001-standards-injection.golden    # expected additionalContext output
    002-file-size-warning.json
    002-file-size-warning.golden
    ...
  compress-output/
    001-go-test-pass.json
    001-go-test-pass.golden
    002-go-test-fail.json
    002-go-test-fail.golden
    ...
  session-track/
    001-first-read.json
    001-first-read.golden             # empty (no context on first read)
    002-redundant-read.json
    002-redundant-read.golden         # "You already read this file..."
    ...
  blast-radius/
    001-edit-exported-function.json
    001-edit-exported-function.golden
    002-edit-internal-only.json
    002-edit-internal-only.golden      # empty (no blast radius)
    ...
  decision-guard/
    001-pattern-match.json
    001-pattern-match.golden
    002-no-match.json
    002-no-match.golden               # empty
    ...
```

### Fixture JSON schema

```json
{
  "description": "Edit an exported function in handlers.go",
  "hook": "blast-radius",
  "project_root": "testdata/codebase-go-web",
  "session_state": {
    "touches": [
      {"t": "2026-03-07T14:00:00Z", "tool": "Read", "file": "routes.go", "action": "read"}
    ]
  },
  "input": {
    "tool_name": "Edit",
    "tool_input": {
      "file_path": "testdata/codebase-go-web/handlers.go",
      "new_string": "func HandleUser(w http.ResponseWriter, r *http.Request) {"
    }
  },
  "expect": {
    "has_output": true,
    "contains": ["files import this module", "routes.go (already in session)"],
    "not_contains": ["internal-only"],
    "max_tokens": 200,
    "max_latency_ms": 50
  }
}
```

### Running unit evals

```go
func TestEval_AllFixtures(t *testing.T) {
    fixtures := loadFixtures(t, "testdata/")
    for _, fix := range fixtures {
        t.Run(fix.Description, func(t *testing.T) {
            t.Parallel()
            // Setup session state if specified
            if fix.SessionState != nil {
                setupSessionDir(t, fix.SessionState)
            }
            // Run the hook
            start := time.Now()
            result := runHook(fix.Hook, fix.Input, fix.ProjectRoot)
            elapsed := time.Since(start)
            // Validate
            if fix.Expect.HasOutput && result == "" {
                t.Error("expected output but got empty")
            }
            for _, s := range fix.Expect.Contains {
                if !strings.Contains(result, s) {
                    t.Errorf("output missing %q", s)
                }
            }
            if fix.Expect.MaxLatencyMs > 0 && elapsed.Milliseconds() > int64(fix.Expect.MaxLatencyMs) {
                t.Errorf("latency %dms exceeds budget %dms", elapsed.Milliseconds(), fix.Expect.MaxLatencyMs)
            }
        })
    }
}
```

## Tier 2: Integration Evals

### Real codebases, pinned commits

Instead of purpose-built toy projects, we use real open-source codebases pinned at specific commits. This ensures the eval exercises real-world complexity — real import graphs, real file sizes, real naming conventions.

```
scripts/eval/codebases.json:
[
  {
    "name": "kaboom",
    "repo": ".",
    "commit": "HEAD",
    "description": "Kaboom itself — Go + TS, ~300 source files"
  },
  {
    "name": "chi",
    "repo": "https://github.com/go-chi/chi",
    "commit": "v5.1.0",
    "description": "Go HTTP router — clean, small, well-structured (~40 files)"
  },
  {
    "name": "hono",
    "repo": "https://github.com/honojs/hono",
    "commit": "v4.0.0",
    "description": "TypeScript web framework — ESM, moderate size (~100 files)"
  }
]
```

Each codebase gets:
- A `.kaboom.json` with standards and decisions
- A set of scripted session sequences
- Golden metrics for regression comparison

### Session sequence format

```
scripts/eval/scenarios/kaboom-refactor-handler.jsonl
```

Each line is a tool use event. The rig replays them in order, running all hooks after each step:

```jsonl
{"tool_name":"Read","tool_input":{"file_path":"internal/hook/quality_gate.go"}}
{"tool_name":"Read","tool_input":{"file_path":"internal/hook/protocol.go"}}
{"tool_name":"Edit","tool_input":{"file_path":"internal/hook/quality_gate.go","new_string":"func RunQualityGate(input Input) *QualityGateResult {"}}
{"tool_name":"Read","tool_input":{"file_path":"internal/hook/quality_gate.go"}}
{"tool_name":"Bash","tool_input":{"command":"go test ./internal/hook/..."},"tool_response":{"stdout":"ok  \tinternal/hook\t0.5s\n"}}
```

### Eval runner

```bash
scripts/eval/run.sh [--codebase=kaboom] [--scenario=refactor-handler]
```

For each scenario:
1. Clone/checkout the codebase at the pinned commit (or use local)
2. Write `.kaboom.json` and `decisions.json` fixtures
3. Replay the session sequence, running hooks after each step
4. Collect metrics: tokens injected, tokens saved, latency, hook outputs
5. Compare against golden metrics file
6. Print report

### A/B comparison

The runner can diff two configurations:

```bash
scripts/eval/run.sh --config=baseline    # no hooks
scripts/eval/run.sh --config=full        # all hooks

scripts/eval/compare.sh baseline.json full.json
```

Output:
```
Scenario: kaboom-refactor-handler
  ┌───────────────────┬──────────┬──────────┬────────┐
  │ Metric            │ Baseline │ Hooks On │ Delta  │
  ├───────────────────┼──────────┼──────────┼────────┤
  │ Total tokens      │ 15,200   │ 12,800   │ -15.8% │
  │ File re-reads     │ 4        │ 1        │ -75%   │
  │ Blast warnings    │ 0        │ 2        │ +2     │
  │ Decisions fired   │ 0        │ 1        │ +1     │
  │ Hook latency p50  │ -        │ 8ms      │ -      │
  │ Hook latency p99  │ -        │ 42ms     │ -      │
  └───────────────────┴──────────┴──────────┴────────┘
```

## Tier 3: Live Session Metrics

### Collection

Each hook writes a single metric line to `~/.kaboom/sessions/<id>/metrics.jsonl` after execution:

```jsonl
{"hook":"quality-gate","t":"2026-03-07T14:30:01Z","latency_ms":6,"tokens_out":180,"action":"injected"}
{"hook":"compress-output","t":"2026-03-07T14:30:05Z","latency_ms":4,"tokens_in":3200,"tokens_out":120,"action":"compressed"}
{"hook":"session-track","t":"2026-03-07T14:30:06Z","latency_ms":1,"tokens_out":0,"action":"recorded"}
{"hook":"session-track","t":"2026-03-07T14:31:00Z","latency_ms":2,"tokens_out":35,"action":"redundant_read"}
```

### Aggregation

On session end (or via `kaboom-hooks eval --live`), aggregate metrics:

```go
type SessionMetrics struct {
    SessionID    string        `json:"session_id"`
    Duration     time.Duration `json:"duration"`
    HooksEnabled []string      `json:"hooks_enabled"`
    Totals       struct {
        Invocations          int `json:"invocations"`
        TokensInjected       int `json:"tokens_injected"`
        TokensSaved          int `json:"tokens_saved"`
        RedundantReadsCaught int `json:"redundant_reads_caught"`
        BlastRadiusWarnings  int `json:"blast_radius_warnings"`
        DecisionsEnforced    int `json:"decisions_enforced"`
    } `json:"totals"`
    Latency struct {
        P50Ms int `json:"p50_ms"`
        P99Ms int `json:"p99_ms"`
        MaxMs int `json:"max_ms"`
    } `json:"latency"`
}
```

### Lifetime aggregation

Roll up into `~/.kaboom/stats/lifetime.json` (extends existing token tracker):

```json
{
  "sessions_tracked": 47,
  "total_tokens_saved": 892000,
  "total_tokens_injected": 45000,
  "net_savings": 847000,
  "redundant_reads_prevented": 312,
  "blast_radius_warnings": 89,
  "decisions_enforced": 23,
  "avg_hook_latency_ms": 7
}
```

## CI Integration

### Makefile target

```makefile
eval:
    go test ./internal/hook/eval/ -v -count=1
    ./scripts/eval/run.sh --all --ci
```

### Regression detection

```bash
scripts/eval/check-regression.sh previous.json current.json
```

Fails if:
- Any hook latency p99 regresses by > 20%
- Any expected hook output stops matching its golden file
- Net token savings drops by > 10% on any scenario

### GitHub Actions

```yaml
- name: Run hook evals
  run: make eval
- name: Check regression
  run: scripts/eval/check-regression.sh ${{ previous_report }} eval-report.json
```

## Performance

The full eval suite must complete in < 60 seconds:
- Tier 1 unit evals: < 10s (parallel Go tests)
- Tier 2 integration evals: < 45s (3 codebases × 3 scenarios × ~5s each)
- Report generation: < 5s

## File Structure

```
internal/hook/eval/
  eval.go              # Eval runner library
  eval_test.go         # Tier 1 unit eval runner
  testdata/
    quality-gate/      # Fixtures per hook
    compress-output/
    session-track/
    blast-radius/
    decision-guard/

scripts/eval/
  run.sh               # Tier 2 integration runner
  compare.sh           # A/B comparison
  check-regression.sh  # CI regression check
  codebases.json       # Pinned codebase list
  scenarios/            # JSONL session sequences
    kaboom-refactor-handler.jsonl
    kaboom-add-endpoint.jsonl
    chi-add-middleware.jsonl
    hono-extract-component.jsonl
  golden/              # Expected metrics for regression
    kaboom-refactor-handler.json
    chi-add-middleware.json
```
