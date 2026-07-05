---
doc_type: test_plan
feature_id: feature-macro-recording
status: proposed
issue: "#88"
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Interactive Macro Recording (Replay) — Test Plan

**Status:** [x] Product Tests Defined | [x] Tech Tests Designed | [ ] Tests Generated | [ ] All Tests Passing

---

## Product Tests

### Valid State Tests

- **Test:** Save a sequence with valid name and steps
  - **Given:** No sequence named "login-flow" exists
  - **When:** Agent calls `configure({action: "save_sequence", name: "login-flow", steps: [{action: "navigate", url: "https://app.local"}]})`
  - **Then:** Response has `status: "saved"`, `step_count: 1`, `saved_at` is a valid ISO8601 timestamp

- **Test:** Overwrite an existing sequence (upsert)
  - **Given:** Sequence "login-flow" exists with 2 steps
  - **When:** Agent calls `configure({action: "save_sequence", name: "login-flow", steps: [{action: "navigate", url: "https://app.local"}, {action: "click", selector: "#btn"}, {action: "type", selector: "#input", text: "hello"}]})`
  - **Then:** Response has `status: "saved"`, `step_count: 3`; old 2-step sequence is replaced

- **Test:** Save a sequence with description and tags
  - **Given:** No sequence named "admin-setup" exists
  - **When:** Agent calls `configure({action: "save_sequence", name: "admin-setup", description: "Login and navigate to admin panel", tags: ["auth", "admin"], steps: [{action: "navigate", url: "https://app.local/login"}]})`
  - **Then:** Response has `status: "saved"`; get_sequence returns description and tags

- **Test:** Replay a saved sequence successfully
  - **Given:** Sequence "login-flow" exists with 3 steps: navigate, fill_form_and_submit, click
  - **When:** Agent calls `configure({action: "replay_sequence", name: "login-flow"})`
  - **Then:** Response has `status: "ok"`, `steps_executed: 3`, `steps_failed: 0`, `steps_total: 3`
  - **And:** `results` array has 3 entries, each with `status: "ok"` and `duration_ms > 0`

- **Test:** Get a saved sequence
  - **Given:** Sequence "login-flow" exists with 3 steps, description, and tags
  - **When:** Agent calls `configure({action: "get_sequence", name: "login-flow"})`
  - **Then:** Response has `status: "ok"`, `name: "login-flow"`, `step_count: 3`, full `steps` array, `description`, `tags`, `saved_at`

- **Test:** List all saved sequences
  - **Given:** 3 sequences exist: "login-flow", "admin-setup", "checkout-flow"
  - **When:** Agent calls `configure({action: "list_sequences"})`
  - **Then:** Response has `status: "ok"`, `count: 3`, `sequences` array with 3 entries
  - **And:** Each entry has `name`, `step_count`, `saved_at`; no `steps` array (summary only)

- **Test:** List sequences filtered by tag
  - **Given:** "login-flow" tagged ["auth"], "admin-setup" tagged ["auth", "admin"], "checkout-flow" tagged ["e2e"]
  - **When:** Agent calls `configure({action: "list_sequences", tags: ["auth"]})`
  - **Then:** Response has `count: 2`, returning only "login-flow" and "admin-setup"

- **Test:** Delete a saved sequence
  - **Given:** Sequence "login-flow" exists
  - **When:** Agent calls `configure({action: "delete_sequence", name: "login-flow"})`
  - **Then:** Response has `status: "deleted"`, `name: "login-flow"`
  - **And:** Subsequent get_sequence returns `no_data` error

- **Test:** Sequences persist across daemon restart
  - **Given:** Sequence "login-flow" saved
  - **When:** Daemon is restarted (configure action="restart" or process kill + respawn)
  - **Then:** `configure({action: "get_sequence", name: "login-flow"})` returns the sequence with all steps intact

- **Test:** Replay with stop_after_step
  - **Given:** Sequence "full-flow" exists with 5 steps
  - **When:** Agent calls `configure({action: "replay_sequence", name: "full-flow", stop_after_step: 3})`
  - **Then:** Response has `steps_executed: 3`, `steps_total: 5`, only 3 entries in `results`

- **Test:** Replay with override_steps
  - **Given:** Sequence "login-flow" with step[1] = `{action: "type", selector: "#email", text: "admin@test.com"}`
  - **When:** Agent calls `configure({action: "replay_sequence", name: "login-flow", override_steps: [null, {action: "type", selector: "#email", text: "viewer@test.com"}, null]})`
  - **Then:** Step 0 and step 2 use saved versions; step 1 uses the override with text "viewer@test.com"

### Edge Case Tests (Negative)

- **Test:** Save with empty name
  - **Given:** Agent attempts to save a sequence
  - **When:** `configure({action: "save_sequence", name: "", steps: [{action: "navigate", url: "..."}]})`
  - **Then:** Error: `missing_param: name is required`

- **Test:** Save with invalid name characters
  - **Given:** Agent attempts to save a sequence with special characters
  - **When:** `configure({action: "save_sequence", name: "my sequence!", steps: [{action: "navigate", url: "..."}]})`
  - **Then:** Error: `invalid_param: name must match ^[a-zA-Z0-9_-]+$ and be max 64 chars`

- **Test:** Save with name exceeding 64 characters
  - **Given:** Agent provides a name longer than 64 characters
  - **When:** `configure({action: "save_sequence", name: "a]x65 repeated chars", steps: [...]})`
  - **Then:** Error: `invalid_param: name must match ^[a-zA-Z0-9_-]+$ and be max 64 chars`

- **Test:** Save with empty steps array
  - **Given:** Agent provides no steps
  - **When:** `configure({action: "save_sequence", name: "empty", steps: []})`
  - **Then:** Error: `invalid_param: steps must be a non-empty array`

- **Test:** Save with more than 50 steps
  - **Given:** Agent provides 51 steps
  - **When:** `configure({action: "save_sequence", name: "too-many", steps: [51 items]})`
  - **Then:** Error: `invalid_param: steps exceeds maximum of 50`

- **Test:** Save with step missing action field
  - **Given:** Agent provides a step without an action field
  - **When:** `configure({action: "save_sequence", name: "bad-step", steps: [{"selector": "#btn"}]})`
  - **Then:** Error: `invalid_param: step[0] missing required 'action' field`

- **Test:** Replay a non-existent sequence
  - **Given:** No sequence named "does-not-exist"
  - **When:** `configure({action: "replay_sequence", name: "does-not-exist"})`
  - **Then:** Error: `no_data: Sequence not found: does-not-exist`
  - **And:** Recovery hint: `Use configure with action='list_sequences' to see available sequences`

- **Test:** Get a non-existent sequence
  - **Given:** No sequence named "ghost"
  - **When:** `configure({action: "get_sequence", name: "ghost"})`
  - **Then:** Error: `no_data: Sequence not found: ghost`

- **Test:** Delete a non-existent sequence
  - **Given:** No sequence named "ghost"
  - **When:** `configure({action: "delete_sequence", name: "ghost"})`
  - **Then:** Error: `no_data: Sequence not found: ghost`

- **Test:** Replay with override_steps length mismatch
  - **Given:** Sequence "login-flow" has 3 steps
  - **When:** `configure({action: "replay_sequence", name: "login-flow", override_steps: [null, null]})`
  - **Then:** Error: `invalid_param: override_steps length (2) does not match sequence step count (3)`

- **Test:** Replay when extension is disconnected
  - **Given:** Sequence "login-flow" exists, extension is not connected
  - **When:** `configure({action: "replay_sequence", name: "login-flow"})`
  - **Then:** Error: `extension_disconnected: Cannot replay, extension not connected`

- **Test:** Replay when pilot is disabled
  - **Given:** Sequence "login-flow" exists, AI Web Pilot toggle is off
  - **When:** `configure({action: "replay_sequence", name: "login-flow"})`
  - **Then:** Error: `pilot_disabled: Cannot replay, AI Web Pilot not enabled`

- **Test:** List sequences when none exist
  - **Given:** No sequences saved
  - **When:** `configure({action: "list_sequences"})`
  - **Then:** Response has `status: "ok"`, `count: 0`, `sequences: []`

### Concurrent/Race Condition Tests

- **Test:** Concurrent replay attempts
  - **Given:** Sequence "login-flow" exists, a replay is currently in progress
  - **When:** Second `configure({action: "replay_sequence", name: "login-flow"})` is called
  - **Then:** Error: `sequence_busy: Another sequence is currently replaying. Wait for it to complete.`
  - **And:** First replay continues unaffected

- **Test:** Save during active replay
  - **Given:** A replay of "login-flow" is in progress
  - **When:** Agent calls `configure({action: "save_sequence", name: "login-flow", steps: [...]})`
  - **Then:** Save succeeds (persistence is independent of replay execution)
  - **And:** Currently running replay continues with the old steps (loaded at replay start)

- **Test:** Delete during active replay
  - **Given:** A replay of "login-flow" is in progress
  - **When:** Agent calls `configure({action: "delete_sequence", name: "login-flow"})`
  - **Then:** Delete succeeds (removes from disk)
  - **And:** Currently running replay continues (steps already loaded in memory)

### Failure & Recovery Tests

- **Test:** Replay with one failing step (continue_on_error=true)
  - **Given:** Sequence with 4 steps; step 2 has a stale selector
  - **When:** `configure({action: "replay_sequence", name: "stale-seq"})`
  - **Then:** Response has `status: "partial"`, `steps_executed: 3`, `steps_failed: 1`
  - **And:** results[2] has `status: "error"` with descriptive error message
  - **And:** Steps 0, 1, 3 have `status: "ok"`

- **Test:** Replay with one failing step (continue_on_error=false)
  - **Given:** Sequence with 4 steps; step 2 has a stale selector
  - **When:** `configure({action: "replay_sequence", name: "stale-seq", continue_on_error: false})`
  - **Then:** Response has `status: "error"`, `steps_executed: 2`, `steps_failed: 1`, `stopped_at_step: 2`
  - **And:** Only 3 results returned (steps 0, 1, 2)
  - **And:** Step 3 was never attempted

- **Test:** Replay where navigate step times out
  - **Given:** Sequence starts with `{action: "navigate", url: "https://unreachable.local"}`
  - **When:** `configure({action: "replay_sequence", name: "timeout-seq", step_timeout_ms: 5000})`
  - **Then:** Step 0 result has `status: "error"`, `duration_ms` close to 5000
  - **And:** Remaining steps proceed if continue_on_error=true

- **Test:** Recovery after stale sequence -- agent reads error, updates, re-saves
  - **Given:** Replay of "admin-setup" fails at step 2 (selector changed)
  - **When:** Agent calls get_sequence, updates step 2's selector, calls save_sequence, then replay_sequence again
  - **Then:** Second replay succeeds with all steps ok

---

## Technical Tests

### Unit Tests

#### Coverage Areas:
- Sequence validation (name format, step count, action field presence)
- Sequence serialization/deserialization (JSON round-trip)
- Session store integration (save, load, list, delete in "sequences" namespace)
- Replay execution loop (step dispatch, result collection, error handling)
- Override steps merging logic
- Tag-based filtering for list_sequences
- Concurrent replay guard (mutex/flag)

**Test File:** `cmd/browser-agent/tools_configure_sequence_test.go`

#### Unit Test Cases:

| # | Test | Description |
|---|------|-------------|
| 1 | `TestSaveSequence_Valid` | Save with valid name, steps, description, tags; verify stored data round-trips |
| 2 | `TestSaveSequence_Upsert` | Save twice with same name; verify second overwrites first |
| 3 | `TestSaveSequence_EmptyName` | Empty name returns missing_param error |
| 4 | `TestSaveSequence_InvalidName` | Name with spaces/special chars returns invalid_param error |
| 5 | `TestSaveSequence_LongName` | Name > 64 chars returns invalid_param error |
| 6 | `TestSaveSequence_EmptySteps` | Empty steps array returns invalid_param error |
| 7 | `TestSaveSequence_TooManySteps` | 51 steps returns invalid_param error |
| 8 | `TestSaveSequence_StepMissingAction` | Step without "action" key returns invalid_param error |
| 9 | `TestSaveSequence_MaxStepsEdge` | Exactly 50 steps succeeds |
| 10 | `TestGetSequence_Exists` | Get returns full sequence with steps, description, tags |
| 11 | `TestGetSequence_NotFound` | Get for non-existent name returns no_data error |
| 12 | `TestListSequences_Empty` | List when no sequences returns empty array, count: 0 |
| 13 | `TestListSequences_Multiple` | List returns summaries without steps |
| 14 | `TestListSequences_FilterByTag` | List with tags filter returns matching sequences only |
| 15 | `TestListSequences_FilterByMultipleTags` | Sequences must match ALL specified tags |
| 16 | `TestDeleteSequence_Exists` | Delete removes sequence; subsequent get returns not found |
| 17 | `TestDeleteSequence_NotFound` | Delete non-existent returns no_data error |
| 18 | `TestReplaySequence_AllStepsOk` | All steps succeed; status: "ok", correct counts |
| 19 | `TestReplaySequence_NotFound` | Replay non-existent returns no_data error |
| 20 | `TestReplaySequence_PartialFailure` | One step fails with continue_on_error=true; status: "partial" |
| 21 | `TestReplaySequence_StopOnError` | One step fails with continue_on_error=false; stops immediately |
| 22 | `TestReplaySequence_StopAfterStep` | stop_after_step=2 runs only 2 steps |
| 23 | `TestReplaySequence_OverrideSteps` | Override steps replace saved steps at matching indices |
| 24 | `TestReplaySequence_OverrideLengthMismatch` | override_steps wrong length returns invalid_param error |
| 25 | `TestReplaySequence_ExtensionDisconnected` | Returns extension_disconnected error |
| 26 | `TestReplaySequence_PilotDisabled` | Returns pilot_disabled error |
| 27 | `TestReplaySequence_ConcurrentReplay` | Second concurrent replay returns sequence_busy error |
| 28 | `TestReplaySequence_ResultsTiming` | Each result has non-zero duration_ms and correct action field |
| 29 | `TestSequencePersistence_Restart` | Save sequence, reinitialize store, verify sequence loads |
| 30 | `TestSequenceNameValidation` | Parametric test: valid names (alphanumeric, hyphens, underscores) pass; others fail |

### Integration Tests

#### Scenarios:

**Integration Test 1: Save-Replay-Verify cycle**

```
SCENARIO: Save a login sequence, replay it, verify all steps executed

GIVEN:
  - Kaboom server running
  - Extension connected with pilot enabled
  - Test app running at https://app.local

WHEN:
  Step 1: Save sequence
    configure({
      action: "save_sequence",
      name: "login-admin",
      steps: [
        {action: "navigate", url: "https://app.local/login"},
        {action: "type", selector: "#email", text: "admin@test.com", clear: true},
        {action: "type", selector: "#password", text: "test123", clear: true},
        {action: "click", selector: "#login-btn"}
      ]
    })
    -> status: "saved"

  Step 2: Replay sequence
    configure({action: "replay_sequence", name: "login-admin"})
    -> status: "ok", steps_executed: 4, steps_failed: 0

  Step 3: Verify page state
    interact({action: "get_text", selector: ".user-name"})
    -> Returns "admin@test.com" or similar logged-in indicator

THEN:
  - Sequence saved and replayed successfully
  - Browser is in the expected logged-in state
```

**Integration Test 2: Full CRUD lifecycle**

```
SCENARIO: Create, read, list, update, delete a sequence

GIVEN:
  - Kaboom server running

WHEN:
  Step 1: Save
    configure({action: "save_sequence", name: "test-seq", description: "Test", steps: [{action: "navigate", url: "https://example.com"}]})
    -> saved

  Step 2: Get
    configure({action: "get_sequence", name: "test-seq"})
    -> Returns full sequence with 1 step

  Step 3: List
    configure({action: "list_sequences"})
    -> count: 1, includes "test-seq"

  Step 4: Update (upsert)
    configure({action: "save_sequence", name: "test-seq", description: "Updated", steps: [{action: "navigate", url: "https://example.com"}, {action: "click", selector: "#btn"}]})
    -> saved, step_count: 2

  Step 5: Verify update
    configure({action: "get_sequence", name: "test-seq"})
    -> description: "Updated", step_count: 2

  Step 6: Delete
    configure({action: "delete_sequence", name: "test-seq"})
    -> deleted

  Step 7: Verify deletion
    configure({action: "get_sequence", name: "test-seq"})
    -> no_data error

THEN:
  - Full CRUD lifecycle works correctly
  - Each operation returns expected status
```

**Integration Test 3: Replay with partial failure and recovery**

```
SCENARIO: Replay fails on stale selector, agent fixes and retries

GIVEN:
  - Sequence "settings-nav" with step that clicks a now-removed element
  - Extension connected

WHEN:
  Step 1: First replay attempt
    configure({action: "replay_sequence", name: "settings-nav"})
    -> status: "partial", results show step 2 failed with "Element not found"

  Step 2: Agent reads sequence
    configure({action: "get_sequence", name: "settings-nav"})
    -> Returns steps including the broken one

  Step 3: Agent updates sequence with fixed selector
    configure({action: "save_sequence", name: "settings-nav", steps: [updated steps]})
    -> saved

  Step 4: Second replay attempt
    configure({action: "replay_sequence", name: "settings-nav"})
    -> status: "ok", all steps pass

THEN:
  - First replay properly reports the failure
  - Agent can read, fix, re-save, and retry
  - Second replay succeeds
```

**Test File:** `cmd/browser-agent/tools_configure_sequence_integration_test.go`

### UAT/Acceptance Tests

**Framework:** Manual testing via Claude Code with kaboom-mcp from PATH

#### UAT 1: Save and Replay a Navigation Sequence

**Objective:** Verify end-to-end save and replay of a multi-step browser navigation

**Steps:**
1. Open a browser with the extension enabled and connected
2. Navigate to a test application (e.g., https://example.com)
3. Call via Claude Code:
   ```
   configure({
     action: "save_sequence",
     name: "example-nav",
     description: "Navigate to example.com and interact",
     steps: [
       {action: "navigate", url: "https://example.com"},
       {action: "get_text", selector: "h1"}
     ]
   })
   ```
4. Navigate browser to a different page (e.g., about:blank)
5. Call:
   ```
   configure({action: "replay_sequence", name: "example-nav"})
   ```

**Expected Result:**
- Save returns `status: "saved"`, `step_count: 2`
- Replay returns `status: "ok"`, `steps_executed: 2`
- Browser navigates to example.com
- Results array shows both steps succeeded

#### UAT 2: List and Delete Sequences

**Objective:** Verify sequence management operations

**Steps:**
1. Save 2 sequences: "seq-a" and "seq-b"
2. Call `configure({action: "list_sequences"})`
3. Call `configure({action: "delete_sequence", name: "seq-a"})`
4. Call `configure({action: "list_sequences"})` again

**Expected Result:**
- First list returns count: 2 with both sequences
- Delete returns `status: "deleted"`
- Second list returns count: 1, only "seq-b"

#### UAT 3: Replay with Step Failure

**Objective:** Verify graceful handling of failing steps

**Steps:**
1. Save a sequence with a step that has an invalid selector:
   ```
   configure({
     action: "save_sequence",
     name: "bad-selector",
     steps: [
       {action: "navigate", url: "https://example.com"},
       {action: "click", selector: "#does-not-exist-at-all"},
       {action: "get_text", selector: "h1"}
     ]
   })
   ```
2. Call `configure({action: "replay_sequence", name: "bad-selector"})`

**Expected Result:**
- Replay returns `status: "partial"`
- `steps_executed: 2`, `steps_failed: 1`
- results[1] has `status: "error"` with a descriptive message
- results[0] and results[2] have `status: "ok"`

#### UAT 4: Persistence Across Restart

**Objective:** Verify sequences survive daemon restart

**Steps:**
1. Save a sequence: `configure({action: "save_sequence", name: "persist-test", steps: [{action: "navigate", url: "https://example.com"}]})`
2. Restart the daemon: `configure({action: "restart"})`
3. Wait for daemon to be healthy
4. Call `configure({action: "get_sequence", name: "persist-test"})`

**Expected Result:**
- Sequence is returned with all steps intact after restart

#### UAT 5: Replay with Override Steps

**Objective:** Verify step override functionality

**Steps:**
1. Save a sequence with a type step:
   ```
   configure({
     action: "save_sequence",
     name: "search-flow",
     steps: [
       {action: "navigate", url: "https://example.com"},
       {action: "type", selector: "input", text: "original query"}
     ]
   })
   ```
2. Replay with override:
   ```
   configure({
     action: "replay_sequence",
     name: "search-flow",
     override_steps: [null, {action: "type", selector: "input", text: "modified query"}]
   })
   ```

**Expected Result:**
- Step 0 uses saved navigate action
- Step 1 uses the overridden type action with "modified query"
- All steps succeed

### Manual Testing (if applicable)

#### Steps:
1. Open browser with extension enabled
2. Verify extension is connected (health check)
3. Open Claude Code session
4. Save a multi-step sequence (navigate + click + type)
5. Replay the sequence
6. Verify browser ends up in expected state
7. Check ~/.kaboom/store/sequences/ for the persisted JSON file
8. Verify file contents match the saved sequence
9. Restart daemon, verify sequence still accessible

---

## Test Status

### Links to generated test files (update as tests are created):

| Test Type | File | Status | Notes |
|-----------|------|--------|-------|
| Unit | `cmd/browser-agent/tools_configure_sequence_test.go` | Pending | 30 test cases |
| Integration | `cmd/browser-agent/tools_configure_sequence_integration_test.go` | Pending | 3 scenarios |
| UAT | Manual via Claude Code | Pending | 5 scenarios |
| Manual | Manual browser verification | Pending | 1 walkthrough |

**Overall:** All product test scenarios must pass before feature is considered complete.

---

## Test Summary

| Module | Test Cases | Estimated LOC | Pass Rate | Coverage Target |
|--------|-----------|---------------|-----------|-----------------|
| Validation (save) | 10 | ~150 | Target: 100% | 95%+ |
| CRUD (get/list/delete) | 7 | ~100 | Target: 100% | 95%+ |
| Replay execution | 11 | ~200 | Target: 100% | 90%+ |
| Concurrency | 2 | ~50 | Target: 100% | 90%+ |
| **Integration** | 3 scenarios | ~150 | Target: 100% | 85%+ |
| **UAT** | 5 workflows | Manual | Target: 100% | Complete |
| **TOTAL** | 38 test cases | ~650 LOC | **Target: 100%** | **>= 90%** |

---

## Success Criteria (Gate)

**Test Completeness:**
- [ ] All 30 unit test cases defined with inputs/outputs
- [ ] 3 integration test scenarios cover end-to-end workflows
- [ ] 5 UAT steps cover manual testing
- [ ] Tests are executable without implementation knowledge

**Test Independence:**
- [ ] Each test case is standalone (no dependencies on other tests)
- [ ] Mock objects defined for session store and interact handler
- [ ] Cleanup/teardown procedures specified (delete temp sequences after each test)

**Coverage Targets:**
- [ ] Validation logic: >= 95% code coverage
- [ ] CRUD operations: >= 95% code coverage
- [ ] Replay execution: >= 90% code coverage
- [ ] Concurrency guards: >= 90% code coverage
- [ ] **Overall: >= 90%**

**Feasibility:**
- [ ] QA engineer can execute without asking "how do we...?"
- [ ] Test setup/teardown is clear
- [ ] Expected results are specific and measurable
- [ ] Mock objects are documented

---

## Implementation Order (TDD)

1. **Write failing tests first**
   ```bash
   make test -> Tests fail
   ```

2. **Implement in order:**
   - Phase 1: Validation + Save + Get + List + Delete (~100 LOC handlers, ~150 LOC tests)
   - Phase 2: Replay execution loop (~100 LOC handler, ~200 LOC tests)
   - Phase 3: Edge cases (override_steps, concurrency guard, continue_on_error) (~50 LOC, ~150 LOC tests)
   - Phase 4: Schema changes + integration tests (~20 LOC schema, ~150 LOC tests)

3. **All tests pass**
   ```bash
   make test -> All tests pass
   make ci-local -> Full CI passes
   ```
