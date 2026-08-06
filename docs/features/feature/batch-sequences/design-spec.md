---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-27
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Batch Sequences: AI-Oriented Enhancements

**Issue:** #340
**Status:** Design
**Author:** Design spec for review
**Date:** 2026-02-28

---

## 1. Current State

The `interact(what="batch")` handler (`cmd/browser-agent/internal/toolinteract/action_owners.go`) accepts an array of steps
and executes them sequentially. It shares execution machinery with `replay_sequence`.

### What exists today

| Capability | Status |
|---|---|
| Sequential step execution | Shipped |
| `continue_on_error` (default true) | Shipped |
| `stop_after_step` (early cutoff) | Shipped |
| Per-step status/error/duration tracking | Shipped |
| Replay mutex (prevents concurrent batches) | Shipped |
| Max 50 steps per batch | Shipped |
| Nested batch detection (deadlock prevention) | Shipped |

### Current limitations

1. **No step output capture.** Steps return `status`, `action`, `duration_ms`, and `error` --
   but never the actual result data (text content, element counts, screenshot bytes). The agent
   gets pass/fail per step but cannot read what a `get_text` or `query` step returned.

2. **No variable binding.** Step N cannot reference output from step M. Every step must use
   fully static selectors and values provided up-front.

3. **No conditional execution.** All steps run unconditionally (or halt on error). There is no
   way to say "skip step 4 if step 3 found zero elements."

4. **No loop construct.** The "click each Settings link" pattern from #340 requires the agent
   to enumerate all N steps manually. There is no "for each matching element, do X."

5. **No assertions.** The agent cannot express "step 2 should return text containing 'Success'"
   -- it must parse the batch result post-hoc and re-issue commands.

6. **No inter-step delays.** Some UI patterns (modal animations, debounced inputs) need a
   pause between steps. The only timing control is `step_timeout_ms` which is a ceiling, not
   a floor.

---

## 2. Proposed Enhancements

### 2.1 Step Output Capture

Extend `SequenceStepResult` with an `output` field that captures the data payload from the
extension response. This is the foundation that variable binding and assertions build on.

```go
type SequenceStepResult struct {
    StepIndex     int             `json:"step_index"`
    Action        string          `json:"action"`
    Status        string          `json:"status"`
    DurationMs    int64           `json:"duration_ms"`
    CorrelationID string          `json:"correlation_id,omitempty"`
    Error         string          `json:"error,omitempty"`
    Output        json.RawMessage `json:"output,omitempty"`   // NEW
}
```

**Output content by action type:**

| Action | Output contains |
|---|---|
| `get_text` | `{"text": "..."}` |
| `get_value` | `{"value": "..."}` |
| `query` (exists) | `{"exists": true}` |
| `query` (count) | `{"count": 5}` |
| `query` (text) | `{"text": "..."}` |
| `query` (text_all) | `{"texts": ["a", "b"]}` |
| `list_interactive` | `{"count": N, "elements": [...]}` (truncated to first 20) |
| `screenshot` | `{"captured": true}` (binary stays in content block, not in output) |
| `get_attribute` | `{"value": "..."}` |
| `navigate` | `{"url": "...", "title": "..."}` |
| Other actions | `null` (no meaningful data to capture) |

**Size guard:** Output is capped at 8 KB per step. If the raw response exceeds this, it is
truncated with a `"_truncated": true` marker.

### 2.2 Variable Binding

Steps can reference output from previous steps using `$steps[N].field` template syntax in
string-valued fields.

```json
{
  "what": "batch",
  "steps": [
    {"what": "get_text", "selector": "#order-id", "as": "order_id"},
    {"what": "navigate", "url": "https://app.example.com/orders/$vars.order_id"}
  ]
}
```

**Mechanism:**

- The `as` field on any step binds that step's output to a named variable.
- `$vars.<name>` references resolve before the step is dispatched to the extension.
- `$steps[N].output.<path>` is also supported for positional references without naming.
- Resolution is string interpolation only -- no expression evaluation.
- Unresolvable references are a step error (status: "error", step does not execute).

**Supported locations for interpolation:** `selector`, `text`, `value`, `url`, `key`,
`script`, `wait_for`, `name`. Not supported in `what` or structural fields.

### 2.3 Assertions

Steps can include an `assert` object that validates the step's output before proceeding.

```json
{
  "what": "query",
  "selector": ".success-banner",
  "query_type": "exists",
  "assert": {"equals": true}
}
```

**Assertion operators:**

| Operator | Meaning | Applies to |
|---|---|---|
| `equals` | Output value === expected | any |
| `not_equals` | Output value !== expected | any |
| `contains` | String output contains substring | string |
| `not_contains` | String output does not contain substring | string |
| `greater_than` | Numeric output > threshold | number |
| `less_than` | Numeric output < threshold | number |
| `exists` | Output field is present and non-null | any |

**Assertion target:** By default, assertions apply to the primary output value (the `text`
for `get_text`, the `count` for `query(count)`, etc.). An optional `assert_field` specifies a
different path within the output.

**On assertion failure:** The step status becomes `"assertion_failed"`. The `error` field
contains the mismatch details. Behavior follows `continue_on_error` semantics -- if false,
batch halts; if true, subsequent steps continue.

### 2.4 Conditional Steps

A step can include an `if` clause that references a previous step's output or variable.

```json
{
  "what": "click",
  "selector": "#confirm-button",
  "if": {"var": "modal_visible", "equals": true}
}
```

**Semantics:**

- `if` is evaluated before the step dispatches.
- If the condition is false, the step is skipped with status `"skipped"`.
- Skipped steps do not count toward `steps_failed`.
- The `if` clause supports the same operators as assertions (equals, contains, greater_than, etc.).
- The `var` field references a named variable (set via `as`) or `$steps[N].output.<path>`.

### 2.5 Loops: `for_each`

A new top-level batch parameter enables iterating over a dynamic set.

```json
{
  "what": "batch",
  "for_each": {
    "selector": "text=Settings",
    "as": "item"
  },
  "steps": [
    {"what": "click", "element_id": "$vars.item.element_id"},
    {"what": "screenshot"},
    {"what": "dismiss_top_overlay"}
  ]
}
```

**Semantics:**

- Before executing steps, the server issues an implicit `list_interactive` using
  `for_each.selector` (and optional `for_each.visible_only`, `for_each.limit`).
- The result set is frozen at invocation time. Steps iterate over a snapshot, not a live query.
- For each matched element, the full `steps` array executes. The element is bound to
  `$vars.<for_each.as>` with fields: `element_id`, `index`, `text`, `selector`.
- `for_each.max_iterations` caps the loop (default: 20, hard max: 50).
- Steps within each iteration share variable scope; iterations do not bleed variables.

**Result structure with for_each:**

```json
{
  "status": "ok",
  "iterations": 7,
  "iterations_failed": 0,
  "results": [
    {
      "iteration": 0,
      "element": {"element_id": "e-abc", "text": "Settings"},
      "steps": [...]
    }
  ]
}
```

### 2.6 Inter-Step Delay

A new optional `delay_ms` field on individual steps pauses execution before the step runs.

```json
{"what": "screenshot", "delay_ms": 500}
```

- Max delay: 5000 ms. Values above this are clamped.
- A batch-level `step_delay_ms` applies a default delay between all steps.
- Step-level `delay_ms` overrides the batch-level default.

### 2.7 Early Exit on Condition

Beyond `continue_on_error: false` (halt on error), add `exit_on` for conditional batch exit.

```json
{
  "what": "batch",
  "exit_on": {"var": "found_target", "equals": true},
  "steps": [...]
}
```

- `exit_on` is evaluated after each step completes.
- If the condition becomes true, the batch halts with status `"exit_condition_met"`.
- Remaining steps are not executed. This is not an error.

---

## 3. Output Schema

### Batch Response (current, extended)

```json
{
  "status": "ok | error | partial | queued | exit_condition_met",
  "steps_executed": 5,
  "steps_failed": 0,
  "steps_skipped": 1,
  "steps_queued": 0,
  "steps_total": 6,
  "duration_ms": 2340,
  "variables": {
    "order_id": "ORD-123"
  },
  "results": [
    {
      "step_index": 0,
      "action": "get_text",
      "status": "ok",
      "duration_ms": 120,
      "output": {"text": "ORD-123"}
    },
    {
      "step_index": 1,
      "action": "click",
      "status": "skipped",
      "duration_ms": 0,
      "skip_reason": "Condition not met: modal_visible equals true"
    }
  ],
  "message": "Batch executed: 5/6 steps in 2340ms (1 skipped)"
}
```

### For-Each Response

```json
{
  "status": "ok",
  "iterations": 3,
  "iterations_failed": 0,
  "steps_per_iteration": 3,
  "steps_total": 9,
  "duration_ms": 4500,
  "results": [
    {
      "iteration": 0,
      "element": {"element_id": "e-001", "text": "Settings", "index": 0},
      "steps": [
        {"step_index": 0, "action": "click", "status": "ok", "duration_ms": 200},
        {"step_index": 1, "action": "screenshot", "status": "ok", "duration_ms": 150},
        {"step_index": 2, "action": "dismiss_top_overlay", "status": "ok", "duration_ms": 80}
      ]
    }
  ],
  "message": "Batch iterated 3 elements, 9/9 steps in 4500ms"
}
```

---

## 4. Error Recovery Patterns

### 4.1 Per-Step Error Handling

`continue_on_error` remains the primary knob. Assertion failures and conditional skips are
distinct from errors -- they do not trigger the `continue_on_error` halt.

### 4.2 Retry

Steps can specify `retry: {max: 2, delay_ms: 500}` to retry on failure.

- On retry, the step re-executes from scratch (new extension command, new correlation ID).
- Each retry attempt counts toward the step's `duration_ms`.
- Max retries capped at 3 to prevent runaway execution.
- Only the final attempt's result is recorded. `retry_attempts` is added to the step result.

### 4.3 Fallback Steps

A step can include `on_error` with an alternative action:

```json
{
  "what": "click",
  "selector": "#primary-button",
  "on_error": {"what": "click", "selector": "#fallback-button"}
}
```

- If the primary step fails, the `on_error` step executes instead.
- The fallback result replaces the primary in the results array.
- Nesting `on_error` within `on_error` is not allowed (one level only).

### 4.4 Variable-Dependent Error

If a variable reference cannot be resolved (the source step failed or was skipped), the
dependent step fails with a clear error message: `"Variable 'order_id' not available:
source step 0 failed"`.

---

## 5. Security Considerations

### 5.1 Loop Bounds

- `for_each.max_iterations` defaults to 20, hard-capped at 50.
- The total step count across all iterations must not exceed `maxSequenceSteps` (50).
  Formula: `iterations * len(steps) <= 50`. The batch rejects at invocation time if the
  element count would exceed this.
- If the `list_interactive` query returns more elements than `max_iterations`, the excess
  is silently dropped with a warning in the response.

### 5.2 Resource Exhaustion Prevention

- **Total batch timeout:** New parameter `batch_timeout_ms` (default: 60000, max: 120000).
  If the batch exceeds this wall-clock time, remaining steps are skipped with status
  `"timeout"`.
- **Output size cap:** 8 KB per step output, 256 KB total across all step outputs.
  Exceeding the total cap disables output capture for remaining steps.
- **Delay budget:** Total `delay_ms` across all steps cannot exceed 30 seconds.

### 5.3 No Arbitrary Code in Control Flow

- Variable interpolation is string substitution only. No expression evaluation, no
  arithmetic, no function calls.
- `if` conditions are simple comparisons against literal values or variable references.
  No compound boolean logic (no AND/OR combinators). This is intentional -- compound
  logic belongs in the agent, not in the batch engine.
- `script` fields (for `execute_js` steps) are pass-through to the extension's existing
  sandboxed execution. They are not subject to variable interpolation by default. An explicit
  `interpolate_script: true` flag opts in, with the understanding that the agent is
  constructing JS.

### 5.4 No Recursive Structures

- Nested `batch` within `batch` is already blocked by the replay mutex.
- `on_error` cannot contain another `on_error`.
- `for_each` cannot contain another `for_each`.
- These are validated at parse time before any execution begins.

### 5.5 Audit Trail

- Batch execution is already recorded via `recordAIAction`. Enhanced batches will also
  log: total iterations, variables bound, assertions evaluated, conditions checked.
- Variable values are logged but truncated to 256 chars to avoid logging sensitive content.

---

## 6. Implementation Phases

### Phase 1: Output Capture (minimal, high value)

- Add `output` field to `SequenceStepResult`.
- Extract response data from extension command results.
- Apply per-step and total size caps.
- Add `delay_ms` support.
- **Estimated effort:** Small. Server-side only, no extension changes.

### Phase 2: Variable Binding + Conditionals

- Add `as` field support, variable storage in batch context.
- Implement `$vars.<name>` and `$steps[N].output.<path>` interpolation.
- Add `if` clause evaluation.
- Add `exit_on` support.
- **Estimated effort:** Medium. Requires template engine (string substitution, not full
  templating) and condition evaluator.

### Phase 3: Assertions

- Add `assert` clause parsing and evaluation.
- Add `assertion_failed` status and reporting.
- **Estimated effort:** Small. Builds directly on Phase 1 output capture.

### Phase 4: For-Each Loops

- Implement `for_each` with implicit `list_interactive`.
- Restructure result format for iterations.
- Apply iteration bounds and total step cap.
- **Estimated effort:** Medium. Requires refactoring the step execution loop to support
  an outer iteration wrapper.

### Phase 5: Error Recovery

- Add `retry` support.
- Add `on_error` fallback steps.
- **Estimated effort:** Small-medium. Retry is straightforward; fallback requires careful
  variable scope handling.

---

## 7. Compatibility

- All new fields are optional. Existing `interact(what="batch")` calls with only `steps`,
  `continue_on_error`, `stop_after_step`, and `step_timeout_ms` continue to work unchanged.
- The `output` field in `SequenceStepResult` is `omitempty` -- old clients see no change
  unless steps produce output.
- `replay_sequence` benefits from the same enhancements since it shares the step execution
  path.
- Schema additions are backward-compatible (new optional properties only).

---

## 8. Non-Goals

- **Parallel step execution.** All steps run sequentially. Parallel branches add significant
  complexity (race conditions, resource contention) for marginal benefit in the single-tab
  model.
- **Persistent loop state.** Loops do not checkpoint progress. If a batch times out at
  iteration 5/10, the agent must re-issue the batch (potentially with `stop_after_step`
  adjustments).
- **Cross-tab batch orchestration.** Batches operate on the tracked tab. Multi-tab workflows
  require the agent to issue separate batches.
- **Full scripting language.** The batch engine is not Turing-complete by design. Complex
  decision logic belongs in the calling agent, not in the batch DSL.
