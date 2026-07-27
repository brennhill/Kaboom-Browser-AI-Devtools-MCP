---
feature: Interactive Macro Recording (Replay)
status: proposed
tool: configure, interact, observe
mode: macro-save, macro-replay, macro-management
version: v0.8.x
doc_type: product-spec
feature_id: feature-macro-recording
issue: "#88"
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interactive Macro Recording (Replay) — Feature Spec

## Problem Statement

During iterative debugging sessions, LLM agents repeatedly navigate through the same UI states to reach the point where a bug occurs. A typical cycle looks like:

1. Agent navigates to login page
2. Agent fills credentials, submits
3. Agent navigates to settings, opens a sub-panel
4. Agent reproduces the bug
5. Agent reads error, proposes a fix
6. Developer applies the fix
7. **Agent repeats steps 1-4 to verify the fix**

Each cycle costs 30-60 seconds of browser automation and 4-8 tool calls. During a debugging session with 5-10 fix attempts, this adds up to **5-10 minutes of pure navigation overhead** and significant token waste.

### What exists today

Kaboom already has two relevant but incomplete primitives:

- **`configure(action="recording_start/recording_stop")` + `configure(action="playback")`** -- Records raw browser events (clicks, keypresses) with timestamps and replays them. Designed for regression testing, not for quick state recovery. Recordings capture low-level DOM events, are tied to exact page state, and have no concept of named reusable sequences.

- **`interact(action="save_state/load_state")`** -- Captures and restores page-level state (form values, scroll position, localStorage, sessionStorage, cookies). This restores *data* but not *navigation path*. It cannot click through a multi-step wizard or authenticate through an OAuth flow.

Neither primitive solves the "get me back to this exact UI state via a repeatable sequence of interact actions" problem.

### The gap

What agents need is a way to **save a named sequence of high-level interact actions** (navigate, click, type, fill_form_and_submit) and **replay that sequence in a single tool call** to skip repetitive setup steps. This is conceptually different from low-level recording (which captures raw DOM events) -- it operates at the MCP tool call level, making sequences readable, editable, and composable by the LLM.

---

## Solution

**`configure(action="save_sequence")`** saves a named sequence of interact actions. **`configure(action="replay_sequence")`** replays it. The sequence is a list of interact action objects that the server executes in order, with configurable wait/retry behavior between steps.

### Key design decisions

1. **Operates at interact-action level, not DOM-event level.** Sequences contain the same action objects you pass to `interact()` (navigate, click, type, etc.), not raw browser events. This makes them human-readable, LLM-editable, and resilient to minor DOM changes.

2. **Builds on existing interact infrastructure.** Each step in a sequence is dispatched through the same handler that processes `interact()` calls. No new browser-side code needed.

3. **Persisted via session store.** Uses the existing `configure(action="store/load")` persistence layer with a dedicated namespace, so sequences survive daemon restarts.

4. **Composable with state snapshots.** A sequence can end with a `save_state` to capture the resulting page state, or begin with a `load_state` to restore context before navigating.

---

## User Workflows

### Workflow 1: Save a navigation sequence during debugging

```
Agent debugging a bug in the admin settings panel:

1. interact({action: "navigate", url: "https://app.local/login"})
2. interact({action: "fill_form_and_submit", fields: [{selector: "#email", value: "admin@test.com"}, {selector: "#password", value: "test123"}], submit_selector: "#login-btn"})
3. interact({action: "click", selector: "[data-testid=settings-nav]"})
4. interact({action: "click", selector: "[data-testid=advanced-tab]"})

Agent finds the bug, then saves the sequence:

5. configure({
     action: "save_sequence",
     name: "admin-settings-advanced",
     description: "Navigate to admin > settings > advanced tab",
     steps: [
       {action: "navigate", url: "https://app.local/login"},
       {action: "fill_form_and_submit", fields: [{selector: "#email", value: "admin@test.com"}, {selector: "#password", value: "test123"}], submit_selector: "#login-btn"},
       {action: "click", selector: "[data-testid=settings-nav]"},
       {action: "click", selector: "[data-testid=advanced-tab]"}
     ]
   })
```

### Workflow 2: Replay to verify a fix

```
Developer applies a fix. Agent verifies:

1. configure({
     action: "replay_sequence",
     name: "admin-settings-advanced"
   })
   -> Executes all 4 steps automatically
   -> Returns: {status: "ok", steps_executed: 4, steps_total: 4, duration_ms: 3200}

2. Agent inspects the page to verify the fix
```

### Workflow 3: List and manage sequences

```
Agent checks available sequences:

1. configure({action: "list_sequences"})
   -> Returns: [{name: "admin-settings-advanced", step_count: 4, description: "...", saved_at: "..."}]

Agent deletes an outdated sequence:

2. configure({action: "delete_sequence", name: "admin-settings-advanced"})
```

### Workflow 4: LLM edits a sequence before replay

```
Agent reads the sequence, modifies it, and replays:

1. configure({action: "get_sequence", name: "admin-settings-advanced"})
   -> Returns full step list

2. Agent modifies step 2 to use different credentials

3. configure({
     action: "replay_sequence",
     name: "admin-settings-advanced",
     override_steps: [
       null,
       {action: "fill_form_and_submit", fields: [{selector: "#email", value: "viewer@test.com"}, {selector: "#password", value: "test456"}], submit_selector: "#login-btn"},
       null,
       null
     ]
   })
   -> Steps at null indices use the saved version; non-null steps are overridden
```

---

## API Design

All macro operations use the **configure** tool, consistent with how recording_start/recording_stop and store/load already live in configure.

### `configure(action="save_sequence")`

Saves a named sequence of interact actions.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | yes | `"save_sequence"` |
| `name` | string | yes | Unique name for the sequence (alphanumeric, hyphens, underscores; max 64 chars) |
| `description` | string | no | Human-readable description of what the sequence does |
| `steps` | array | yes | Ordered list of interact action objects |
| `tags` | array | no | Labels for categorization (e.g., `["auth", "setup"]`) |

Each element in `steps` is a JSON object with the same schema as `interact()` arguments:

```json
{
  "action": "navigate",
  "url": "https://app.local/login"
}
```

```json
{
  "action": "click",
  "selector": "[data-testid=settings-nav]"
}
```

```json
{
  "action": "type",
  "selector": "#search",
  "text": "admin users",
  "clear": true
}
```

```json
{
  "action": "fill_form_and_submit",
  "fields": [
    {"selector": "#email", "value": "admin@test.com"},
    {"selector": "#password", "value": "test123"}
  ],
  "submit_selector": "#login-btn"
}
```

**Response:**

```json
{
  "status": "saved",
  "name": "admin-settings-advanced",
  "step_count": 4,
  "saved_at": "2026-02-20T14:30:00Z",
  "message": "Sequence saved: admin-settings-advanced (4 steps)"
}
```

**Validation rules:**
- `name` must be non-empty, max 64 characters, matching `^[a-zA-Z0-9_-]+$`
- `steps` must be a non-empty array with at most 50 elements
- Each step must have a valid `action` field matching a known interact action
- Saving with an existing `name` overwrites the previous sequence (upsert semantics)

### `configure(action="replay_sequence")`

Replays a saved sequence by executing each step in order.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | yes | `"replay_sequence"` |
| `name` | string | yes | Name of the saved sequence |
| `override_steps` | array | no | Sparse array of step overrides (null = use saved, non-null = override) |
| `step_timeout_ms` | int | no | Timeout per step (default: 10000ms) |
| `continue_on_error` | bool | no | Continue executing remaining steps if one fails (default: true) |
| `stop_after_step` | int | no | Stop after executing this many steps (for partial replay) |

**Response (success):**

```json
{
  "status": "ok",
  "name": "admin-settings-advanced",
  "steps_executed": 4,
  "steps_failed": 0,
  "steps_total": 4,
  "duration_ms": 3200,
  "results": [
    {"step_index": 0, "action": "navigate", "status": "ok", "duration_ms": 1200},
    {"step_index": 1, "action": "fill_form_and_submit", "status": "ok", "duration_ms": 800},
    {"step_index": 2, "action": "click", "status": "ok", "duration_ms": 600},
    {"step_index": 3, "action": "click", "status": "ok", "duration_ms": 600}
  ],
  "message": "Sequence replayed: 4/4 steps executed in 3200ms"
}
```

**Response (partial failure with continue_on_error=true):**

```json
{
  "status": "partial",
  "name": "admin-settings-advanced",
  "steps_executed": 3,
  "steps_failed": 1,
  "steps_total": 4,
  "duration_ms": 4500,
  "results": [
    {"step_index": 0, "action": "navigate", "status": "ok", "duration_ms": 1200},
    {"step_index": 1, "action": "fill_form_and_submit", "status": "ok", "duration_ms": 800},
    {"step_index": 2, "action": "click", "status": "error", "duration_ms": 10000, "error": "Element not found: [data-testid=settings-nav]"},
    {"step_index": 3, "action": "click", "status": "ok", "duration_ms": 600}
  ],
  "message": "Sequence partially replayed: 3/4 steps executed, 1 failed"
}
```

**Response (failure with continue_on_error=false):**

```json
{
  "status": "error",
  "name": "admin-settings-advanced",
  "steps_executed": 2,
  "steps_failed": 1,
  "steps_total": 4,
  "stopped_at_step": 2,
  "duration_ms": 12000,
  "results": [
    {"step_index": 0, "action": "navigate", "status": "ok", "duration_ms": 1200},
    {"step_index": 1, "action": "fill_form_and_submit", "status": "ok", "duration_ms": 800},
    {"step_index": 2, "action": "click", "status": "error", "duration_ms": 10000, "error": "Element not found: [data-testid=settings-nav]"}
  ],
  "error": "Sequence stopped at step 2: Element not found: [data-testid=settings-nav]",
  "message": "Sequence failed at step 2/4"
}
```

### `configure(action="get_sequence")`

Retrieves a saved sequence with all its steps.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | yes | `"get_sequence"` |
| `name` | string | yes | Name of the sequence to retrieve |

**Response:**

```json
{
  "status": "ok",
  "name": "admin-settings-advanced",
  "description": "Navigate to admin > settings > advanced tab",
  "step_count": 4,
  "tags": ["auth", "settings"],
  "saved_at": "2026-02-20T14:30:00Z",
  "steps": [
    {"action": "navigate", "url": "https://app.local/login"},
    {"action": "fill_form_and_submit", "fields": [{"selector": "#email", "value": "admin@test.com"}, {"selector": "#password", "value": "test123"}], "submit_selector": "#login-btn"},
    {"action": "click", "selector": "[data-testid=settings-nav]"},
    {"action": "click", "selector": "[data-testid=advanced-tab]"}
  ]
}
```

### `configure(action="list_sequences")`

Lists all saved sequences.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | yes | `"list_sequences"` |
| `tags` | array | no | Filter by tags (sequences must have all specified tags) |

**Response:**

```json
{
  "status": "ok",
  "sequences": [
    {
      "name": "admin-settings-advanced",
      "description": "Navigate to admin > settings > advanced tab",
      "step_count": 4,
      "tags": ["auth", "settings"],
      "saved_at": "2026-02-20T14:30:00Z"
    },
    {
      "name": "login-as-viewer",
      "description": "Login as viewer role user",
      "step_count": 2,
      "tags": ["auth"],
      "saved_at": "2026-02-20T14:25:00Z"
    }
  ],
  "count": 2
}
```

### `configure(action="delete_sequence")`

Deletes a saved sequence.

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | yes | `"delete_sequence"` |
| `name` | string | yes | Name of the sequence to delete |

**Response:**

```json
{
  "status": "deleted",
  "name": "admin-settings-advanced",
  "message": "Sequence deleted: admin-settings-advanced"
}
```

---

## How It Builds on Existing Infrastructure

### Session Store (persistence)

Sequences are stored using the existing `configure(action="store")` persistence layer (`internal/persistence/persistence_store.go`). The session store supports namespaced key-value storage with Save/Load/List/Delete operations. Sequences use a dedicated namespace `"sequences"` to avoid collisions with other stored data.

**Storage format on disk:** `~/.kaboom/store/sequences/{name}.json`

### Interact tool (execution)

Each step in a sequence is dispatched through the existing interact tool handler. The replay_sequence handler constructs a synthetic `JSONRPCRequest` for each step and calls the same `handleInteract()` code path that processes direct `interact()` calls. This guarantees identical behavior -- no separate "macro engine" needed.

### Recording infrastructure (comparison)

| Aspect | Existing Recording (configure recording_start/stop) | Macro Sequences (this feature) |
|--------|------------------------------------------------------|-------------------------------|
| **Capture method** | Passive: records raw DOM events as user interacts | Active: agent explicitly declares the steps |
| **Granularity** | Low-level: mouse coords, keystrokes, timestamps | High-level: interact action objects |
| **Editability** | Opaque to LLM (raw events) | Fully readable/editable by LLM |
| **Resilience** | Brittle: tied to exact DOM state and timing | Robust: uses semantic selectors, no timing dependency |
| **Use case** | Regression testing, bug reproduction | Iterative debugging, state recovery |
| **Playback model** | Event-level replay with self-healing selectors | Action-level dispatch through interact handler |

### State snapshots (complementary)

Macro sequences and state snapshots (`save_state`/`load_state`) are complementary:

- **Sequence**: replays *actions* to reach a UI state (handles authentication, navigation, multi-step flows)
- **State snapshot**: restores *data* at a point in time (form values, storage, cookies)

An agent can combine them: replay a sequence to navigate to a page, then load a state snapshot to restore form values.

---

## Data Model

### Sequence (persisted)

```json
{
  "name": "admin-settings-advanced",
  "description": "Navigate to admin > settings > advanced tab",
  "tags": ["auth", "settings"],
  "saved_at": "2026-02-20T14:30:00Z",
  "step_count": 4,
  "steps": [
    {"action": "navigate", "url": "https://app.local/login"},
    {"action": "fill_form_and_submit", "fields": [...], "submit_selector": "#login-btn"},
    {"action": "click", "selector": "[data-testid=settings-nav]"},
    {"action": "click", "selector": "[data-testid=advanced-tab]"}
  ]
}
```

### Go types

```go
// Sequence represents a named, replayable list of interact actions.
type Sequence struct {
    Name        string            `json:"name"`
    Description string            `json:"description,omitempty"`
    Tags        []string          `json:"tags,omitempty"`
    SavedAt     string            `json:"saved_at"`
    StepCount   int               `json:"step_count"`
    Steps       []json.RawMessage `json:"steps"` // Preserved as raw JSON for flexibility
}

// SequenceSummary is returned by list_sequences (omits step details).
type SequenceSummary struct {
    Name        string   `json:"name"`
    Description string   `json:"description,omitempty"`
    Tags        []string `json:"tags,omitempty"`
    SavedAt     string   `json:"saved_at"`
    StepCount   int      `json:"step_count"`
}

// SequenceStepResult captures the outcome of one step during replay.
type SequenceStepResult struct {
    StepIndex  int    `json:"step_index"`
    Action     string `json:"action"`
    Status     string `json:"status"`      // "ok", "error"
    DurationMs int64  `json:"duration_ms"`
    Error      string `json:"error,omitempty"`
}
```

---

## Error Handling

### Save errors

| Condition | Error | Recovery |
|-----------|-------|----------|
| Empty name | `missing_param: name is required` | Provide a name |
| Invalid name format | `invalid_param: name must match ^[a-zA-Z0-9_-]+$ and be max 64 chars` | Fix the name |
| Empty steps array | `invalid_param: steps must be a non-empty array` | Provide at least one step |
| Too many steps | `invalid_param: steps exceeds maximum of 50` | Split into smaller sequences |
| Invalid step (no action field) | `invalid_param: step[N] missing required 'action' field` | Fix the step |
| Store write failure | `internal: Failed to save sequence` | Check disk space |

### Replay errors

| Condition | Error | Recovery |
|-----------|-------|----------|
| Sequence not found | `no_data: Sequence not found: {name}` | Use list_sequences to see available sequences |
| Extension disconnected | `extension_disconnected: Cannot replay, extension not connected` | Reconnect extension |
| Pilot disabled | `pilot_disabled: Cannot replay, AI Web Pilot not enabled` | Enable AI Web Pilot |
| Step timeout | Per-step error in results array | Increase step_timeout_ms or fix selector |
| Selector not found | Per-step error in results array | Update the step's selector |
| Page changed between steps | Per-step error in results array | Add wait_for or navigate steps |
| override_steps length mismatch | `invalid_param: override_steps length (N) does not match sequence step count (M)` | Fix array length |

### Stale selector handling

Sequences are stored as high-level interact actions, which means they use CSS selectors or semantic selectors (text=, role=, label=). When a selector becomes stale:

1. The failing step's error is reported in the `results` array with the exact error message
2. If `continue_on_error=true` (default), subsequent steps still execute
3. The LLM can read the error, update the step, and re-save the sequence

This is more robust than DOM-event recording because:
- Semantic selectors (`text=Submit`, `role=button`) survive many UI changes
- data-testid selectors survive all CSS/layout changes
- The LLM can interpret the error and fix the selector immediately

---

## Edge Cases

### Cross-page sequences

Sequences that navigate across pages (e.g., login -> dashboard -> settings) work naturally because each step is executed independently. The `navigate` action waits for the page to load before the next step begins. The server adds a default 2-second settle delay after navigate actions (configurable via `step_timeout_ms`).

### Timing between steps

Unlike low-level recordings, macro sequences do not record or replay timing. Each step executes as fast as the browser allows. For steps that require the page to settle (e.g., after a click that triggers an animation or AJAX load), the agent should include a `wait_for` action or use `navigate_and_wait_for`.

### Authentication and session expiry

Sequences that include login steps (fill_form_and_submit with credentials) will work as long as:
- The credentials are still valid
- The login form structure has not changed

If the session from a previous replay is still active, login steps will either succeed (redirect to dashboard) or be a no-op (already logged in). The agent should handle both cases.

### Concurrent replay

Only one sequence can replay at a time. If a replay is already in progress, a second `replay_sequence` call returns an error: `"sequence_busy: Another sequence is currently replaying. Wait for it to complete."` This prevents conflicting browser actions.

### Empty page / wrong starting state

If the browser is on an unexpected page when replay begins, the first `navigate` step will correct it. Sequences should always start with a `navigate` action to establish a known starting point.

### Maximum sequence size

Sequences are limited to 50 steps to prevent runaway macros. The JSON payload for a sequence should not exceed 100KB. These limits are enforced at save time.

---

## Privacy Considerations

- **All data stays local.** Sequences are stored on disk at `~/.kaboom/store/sequences/` and never transmitted externally.
- **Credentials in steps.** If a sequence includes `fill_form_and_submit` with password fields, those values are stored in plaintext in the sequence JSON. This is the same security model as `configure(action="store")`. The LLM should use test credentials only, not production secrets.
- **Redaction support.** The existing server-side redaction engine (`RedactionEngine`) is applied to sequence data before persistence, consistent with `save_state` behavior. Sensitive field patterns (password, token, secret) are redacted unless the agent explicitly opts in.
- **No network transmission.** Sequences are never sent to the extension or to any external service. They are server-side only.

---

## Implementation Approach

### Go server changes

#### New file: `cmd/browser-agent/tools_configure_sequence.go` (~200 LOC)

Handlers for all sequence actions:
- `toolConfigureSaveSequence()` -- validates input, serializes to JSON, persists via session store
- `toolConfigureReplaySequence()` -- loads sequence, iterates steps, dispatches each through interact handler, collects results
- `toolConfigureGetSequence()` -- loads and returns full sequence
- `toolConfigureListSequences()` -- lists all sequences in namespace with metadata
- `toolConfigureDeleteSequence()` -- removes sequence from store

#### Modified file: `cmd/browser-agent/tools_configure.go` (~20 LOC)

Add cases to the configure action switch for: `save_sequence`, `replay_sequence`, `get_sequence`, `list_sequences`, `delete_sequence`.

#### Modified file: `internal/schema/configure/tool.go` (~15 LOC)

Add new actions to the configure tool schema enum and add parameter definitions for sequence-related fields.

#### Module registration owner: `cmd/browser-agent/internal/toolmodule/registry.go`

No changes needed -- configure tool is already registered.

### Extension changes

**None.** Sequences are composed of interact actions that are already dispatched to the extension via existing handlers. The extension does not need to know about sequences.

### Wire type changes

**None.** Sequences do not introduce new HTTP wire types. They are stored server-side and dispatched through existing interact handlers. No new extension-to-server or server-to-extension payloads are needed.

### Replay execution model

```
replay_sequence("admin-settings-advanced")
  |
  v
Load sequence from store: ~/.kaboom/store/sequences/admin-settings-advanced.json
  |
  v
For each step[i] in sequence.steps:
  |
  +---> Build synthetic JSONRPCRequest with step[i] as arguments
  +---> Call handleInteract(req, step[i])
  +---> Record result: {step_index, action, status, duration_ms, error}
  +---> If step failed AND continue_on_error=false: stop
  +---> If stop_after_step reached: stop
  |
  v
Return aggregate result: {status, steps_executed, steps_failed, results, duration_ms}
```

### Persistence model

```
~/.kaboom/store/
  sequences/
    admin-settings-advanced.json
    login-as-viewer.json
    checkout-flow-setup.json
```

Each file is a self-contained JSON object with the `Sequence` schema. The session store's existing Save/Load/List/Delete operations handle all I/O.

---

## Schema Changes (configure tool)

New actions added to the configure tool enum:

```
"save_sequence", "replay_sequence", "get_sequence", "list_sequences", "delete_sequence"
```

New parameters added to the configure tool input schema:

```json
{
  "steps": {
    "description": "Ordered list of interact action objects (save_sequence, replay_sequence override)",
    "type": "array",
    "items": {"type": "object"}
  },
  "tags": {
    "description": "Labels for sequence categorization",
    "type": "array",
    "items": {"type": "string"}
  },
  "override_steps": {
    "description": "Sparse array of step overrides for replay (null = use saved)",
    "type": "array",
    "items": {}
  },
  "step_timeout_ms": {
    "description": "Timeout per step during replay (default 10000)",
    "type": "number"
  },
  "continue_on_error": {
    "description": "Continue replay if a step fails (default true)",
    "type": "boolean"
  },
  "stop_after_step": {
    "description": "Stop replay after executing this many steps",
    "type": "number"
  },
  "description": {
    "description": "Human-readable description for saved sequence",
    "type": "string"
  }
}
```

The `name` parameter already exists on the configure tool schema (used by recording_start). It is reused for sequences.

---

## Out of Scope

- **Visual recording UI in the extension popup.** Sequences are created by the LLM agent, not by a human clicking through a UI.
- **Automatic sequence inference from action history.** The agent explicitly defines what goes into a sequence. Automatic extraction from `observe(what="actions")` is a future enhancement.
- **Cross-session sharing.** Sequences are local to a single machine. Sharing between developers is out of scope.
- **Conditional steps / branching logic.** Sequences are linear. If-then-else logic is handled by the LLM between separate replay calls.
- **Parallel step execution.** Steps execute sequentially. Parallel execution would require a different execution model.

---

## Success Criteria

### Functional
- LLM agent can save a sequence of interact actions with a name
- LLM agent can replay a saved sequence in a single tool call
- LLM agent can list, get, and delete saved sequences
- Sequences persist across daemon restarts
- Replay returns per-step results with timing and error details
- Replay continues on error by default (configurable)
- Sequences can be partially replayed (stop_after_step)
- Steps can be overridden at replay time (override_steps)

### Non-Functional
- Save operation completes in < 50ms (local disk I/O)
- Replay overhead per step: < 10ms beyond the interact action itself
- Maximum 50 steps per sequence
- Maximum 100KB per sequence JSON payload
- Zero new dependencies (Go or extension)

### Integration
- Sequences use the same selectors and action format as direct interact() calls
- Sequences compose with save_state/load_state for data + navigation recovery
- Existing redaction engine applies to sequence data
- Server-side only -- no extension changes required

---

## Metrics and Observability

### Server log entries

Each replay generates structured log entries:

```
[SEQUENCE_REPLAY_START] Replaying sequence: admin-settings-advanced (4 steps)
[SEQUENCE_STEP_OK] Step 0/4: navigate completed in 1200ms
[SEQUENCE_STEP_OK] Step 1/4: fill_form_and_submit completed in 800ms
[SEQUENCE_STEP_ERROR] Step 2/4: click failed: Element not found (10000ms)
[SEQUENCE_STEP_OK] Step 3/4: click completed in 600ms
[SEQUENCE_REPLAY_COMPLETE] Sequence admin-settings-advanced: 3/4 steps ok, 1 failed (12600ms)
```

### Action audit trail

Each replay is recorded in the AI action audit trail via `recordAIAction("replay_sequence", ...)`, consistent with other interact actions.

---

## Future Enhancements (not in scope for v1)

1. **Auto-capture from action history.** `configure(action="save_sequence_from_history", last_n: 5)` would save the last N interact actions as a sequence.
2. **Sequence chaining.** Allow a step to reference another sequence by name, enabling composition.
3. **Parameterized sequences.** Define placeholder variables in steps (e.g., `{{email}}`) that are substituted at replay time.
4. **Sequence versioning.** Track changes to a sequence over time with version history.
5. **Extension UI.** Show saved sequences in the extension popup with a "replay" button.
