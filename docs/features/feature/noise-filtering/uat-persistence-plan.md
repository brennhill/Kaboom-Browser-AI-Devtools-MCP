---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# UAT Plan: Noise Rule Persistence (v5.8.1)

## Overview

This UAT plan proves that noise filtering rules persist across server restarts, crashes, and kills, without data loss or ID collisions.

### Test Environment:
- Single kaboom-mcp daemon
- Controlled restart cycles
- Filesystem inspection (.kaboom/noise/rules.json)
- MCP tool calls to configure/observe rules

---

## Test Scenarios

### Scenario 1: Basic Persistence (Round-Trip)
**Objective:** Prove user rules survive server restart

#### Steps:
1. Start daemon on port 7920
2. Call `configure(what: "noise_rule", noise_action: "add", rules: [{ category: "console", classification: "test", match_spec: { message_regex: "test.*pattern" } }])`
3. Call `configure(what: "noise_rule", noise_action: "list")` → verify rule added with ID `user_1`
4. Inspect `.kaboom/noise/rules.json` on disk → verify file created, contains `user_1`
5. Kill daemon (SIGKILL)
6. Start daemon on same port again
7. Call `configure(what: "noise_rule", noise_action: "list")` → verify rule still present with ID `user_1`
8. Verify rule is filtering entries (add console entry matching pattern, verify filtered)

#### Assertions:
- ✅ File created after add
- ✅ File contains JSON with version 1, next_user_id, rules array
- ✅ Rule reloaded with same ID after restart
- ✅ Rule still active and filtering

#### Failure Modes:
- ❌ File not created → persistence not called
- ❌ File contains built-in rules → filtering function broken
- ❌ Rule ID changed (user_1 → user_2) → counter not restored
- ❌ Rule not filtering after reload → compiled patterns not recompiled

---

### Scenario 2: ID Collision Prevention
**Objective:** Prove counter prevents ID collisions across sessions

#### Steps:
1. Start daemon
2. Add rule 1 → ID `user_1`
3. Check `.kaboom/noise/rules.json` → `next_user_id: 2`
4. Kill and restart daemon
5. Add rule 2 → ID `user_2` (NOT `user_1`)
6. Verify both rules present: `user_1` and `user_2`
7. Kill and restart daemon again
8. Add rule 3 → ID `user_3` (NOT `user_1` or `user_2`)
9. Verify all three present: `user_1`, `user_2`, `user_3`

#### Assertions:
- ✅ Counter persisted as `next_user_id`
- ✅ Counter restored correctly on each restart
- ✅ New rules get correct sequential IDs
- ✅ No collision after 3+ restart cycles

#### Failure Modes:
- ❌ Rule 2 gets ID `user_1` → counter not restored
- ❌ Rule 3 gets ID `user_2` → counter drift
- ❌ Duplicate IDs in list → collision detected

---

### Scenario 3: Graceful Failure on Corruption
**Objective:** Prove server continues on corrupted persisted data

#### Steps:
1. Start daemon and add rule → creates file
2. Manually corrupt `.kaboom/noise/rules.json` → `{invalid json}`
3. Kill and restart daemon
4. Verify daemon starts successfully (no crash)
5. Call `configure(what: "noise_rule", noise_action: "list")` → returns built-in rules only
6. Verify new rules can still be added (no state corruption)

#### Assertions:
- ✅ Daemon starts despite corrupted file
- ✅ No panic or error crash
- ✅ Warning logged to stderr (check process logs)
- ✅ User rules list shows only built-ins
- ✅ Can still add new rules normally

#### Failure Modes:
- ❌ Daemon crashes on startup
- ❌ Corrupted rules persisted/re-saved
- ❌ No warning logged

---

### Scenario 4: Built-in Rules Never Persisted
**Objective:** Prove built-in rules are always fresh from code

#### Steps:
1. Start daemon (has ~50 built-in rules)
2. Add user rule → file created with `next_user_id: 2`
3. Inspect `.kaboom/noise/rules.json` → verify NO `builtin_*` rules in file
4. Add another user rule
5. Inspect file again → verify still NO built-in rules
6. Kill and restart daemon
7. Call `configure(what: "noise_rule", noise_action: "list")` → verify all ~50 built-ins + 2 user rules

#### Assertions:
- ✅ Persisted file contains only user/auto/dismiss rules
- ✅ Built-in rules not in file after multiple adds
- ✅ Built-in rules reloaded fresh on restart
- ✅ Total = built-ins (50) + user rules (2)

#### Failure Modes:
- ❌ Built-in rules in persisted file
- ❌ Duplicate built-ins after restart
- ❌ Built-in count doesn't match code

---

### Scenario 5: Remove and Reset Persistence
**Objective:** Prove delete/remove operations persist

#### Steps:
1. Start daemon and add 3 rules → `user_1`, `user_2`, `user_3`
2. Call `configure(what: "noise_rule", noise_action: "remove", rule_id: "user_2")`
3. Inspect file → verify `user_2` removed, `user_1` and `user_3` remain
4. Kill and restart daemon
5. List rules → verify `user_2` still gone, only `user_1` and `user_3`
6. Call `configure(what: "noise_rule", noise_action: "reset")`
7. Inspect file → should be empty (user rules cleared) or missing
8. Kill and restart daemon
9. List rules → only built-ins present

#### Assertions:
- ✅ Remove persisted immediately
- ✅ Removed rule gone after restart
- ✅ Other rules unaffected
- ✅ Reset clears file
- ✅ Reset persists (empty state)
- ✅ Restart shows only built-ins

#### Failure Modes:
- ❌ Removed rule reappears after restart
- ❌ Reset doesn't persist
- ❌ Wrong rules removed/kept

---

### Scenario 6: Counter Desync Recovery
**Objective:** Prove counter safety check handles corrupted counter

#### Steps:
1. Start daemon and add rules → `user_1`, `user_2`, `user_3`
2. Manually edit `.kaboom/noise/rules.json`:
   - Set `next_user_id: 2` (corrupted, should be 4)
   - Keep all 3 rules (`user_1`, `user_2`, `user_3`)
3. Kill and restart daemon
4. Add new rule → should get ID `user_4` (not `user_2`)
5. Verify 4 total rules: `user_1`, `user_2`, `user_3`, `user_4`

#### Assertions:
- ✅ Desync detected (next_user_id < max rule ID)
- ✅ Counter corrected to 4
- ✅ New rule gets `user_4` (no collision)
- ✅ No data loss

#### Failure Modes:
- ❌ New rule gets `user_2` → counter not corrected
- ❌ Collision detected
- ❌ Rules lost

---

### Scenario 7: Filtering Works After Persistence
**Objective:** Prove persisted rules actually filter entries

#### Steps:
1. Start daemon
2. Add rule: `{ category: "console", match_spec: { message_regex: "polling.*update" } }`
3. Verify filtering works (add entry matching pattern, verify filtered)
4. Kill and restart daemon
5. Add entry matching same pattern → verify still filtered
6. Add entry NOT matching → verify NOT filtered

#### Assertions:
- ✅ Rule filters correctly before restart
- ✅ Rule filters correctly after restart
- ✅ Pattern matching still accurate
- ✅ Compiled patterns properly regenerated

#### Failure Modes:
- ❌ Rule doesn't filter after restart
- ❌ Wrong entries filtered
- ❌ Pattern lost or corrupted

---

### Scenario 8: Statistics Preserved
**Objective:** Prove statistics (counts) persist

#### Steps:
1. Start daemon and add rule
2. Generate 10 entries matching rule → should be filtered
3. Check `configure(what: "noise_rule", noise_action: "list")` → statistics show rule ID with count ~10
4. Manually verify `.kaboom/noise/rules.json` contains statistics
5. Kill and restart daemon
6. Check statistics → should still show count ~10 for that rule

#### Assertions:
- ✅ Statistics saved to file
- ✅ Statistics loaded on restart
- ✅ Counts maintained across sessions

#### Failure Modes:
- ❌ Statistics not in file
- ❌ Statistics reset after restart
- ❌ Counts lost

---

### Scenario 9: No SessionStore Fallback
**Objective:** Prove server works without persistence (backward compatibility)

#### Steps:
1. If SessionStore init fails (simulated by deleting .kaboom directory mid-test):
2. Start daemon without SessionStore
3. Add rule → no error
4. Verify rule in memory
5. Kill and restart daemon
6. Verify rule NOT present (no persistence)

#### Assertions:
- ✅ Server starts without SessionStore
- ✅ Rules work in-memory
- ✅ No crash on save failure
- ✅ Silent degradation (no error to user)

#### Failure Modes:
- ❌ Server crashes without SessionStore
- ❌ Error propagated to MCP response

---

### Scenario 10: Max Rules Limit on Load
**Objective:** Prove truncation on load when exceeding max

#### Steps:
1. Start daemon
2. Manually create `.kaboom/noise/rules.json` with 80 user rules (exceeds max of 55 with built-ins)
3. Start daemon
4. List rules → should have only 55 user rules + built-ins (100 total)
5. Check stderr logs → warning about truncation

#### Assertions:
- ✅ Rules truncated to max 100
- ✅ Oldest rules kept (not newest)
- ✅ Warning logged
- ✅ No error returned to user

#### Failure Modes:
- ❌ Rules exceed 100
- ❌ Wrong rules kept/removed
- ❌ No warning logged

---

## Validation Checklist

### File System
- [ ] `.kaboom/` directory created
- [ ] `.kaboom/noise/` subdirectory exists
- [ ] `rules.json` file exists and valid JSON
- [ ] File has correct permissions (0644)
- [ ] `.gitignore` updated to exclude `.kaboom/`

### JSON Schema
- [ ] `version: 1` field present
- [ ] `next_user_id: N` field present and correct
- [ ] `rules[]` array contains only user/auto/dismiss rules
- [ ] No `builtin_*` rules in file
- [ ] `statistics` object present (optional but included)
- [ ] Field names use `snake_case`

### Functionality
- [ ] User rules survive restart
- [ ] Rules filter entries after reload
- [ ] Counter prevents collisions (user_1, user_2, user_3...)
- [ ] Remove persists immediately
- [ ] Reset clears file
- [ ] Built-in rules reloaded fresh
- [ ] Invalid rules skipped on load (with warning)
- [ ] Corrupted file doesn't crash server

### Error Handling
- [ ] Errors logged to stderr only
- [ ] No errors in MCP response
- [ ] Server continues on persistence failure
- [ ] Graceful degradation (built-ins still work)

### Backward Compatibility
- [ ] Works without SessionStore (nil check)
- [ ] Existing tests still pass
- [ ] No breaking changes to tool interface

---

## Automated Test Script

```bash
#!/bin/bash
# test-noise-persistence.sh — Automated UAT for persistence feature

set -e

DAEMON_PORT=7920
TMPDIR=$(mktemp -d)
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Helper: start daemon and wait for ready
start_daemon() {
    local port=$1
    $PROJECT_ROOT/kaboom-mcp --daemon --port "$port" > "$TMPDIR/daemon.log" 2>&1 &
    sleep 1  # wait for startup
}

# Helper: make MCP call
mcp_call() {
    local port=$1
    local tool=$2
    local args=$3
    curl -s -X POST "http://localhost:$port/mcp" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\": \"2.0\", \"id\": 1, \"method\": \"tools/call\", \"params\": {\"name\": \"$tool\", \"arguments\": $args}}"
}

echo "=== Test 1: Basic Persistence (Round-Trip) ==="
start_daemon $DAEMON_PORT
mcp_call $DAEMON_PORT configure '{"action":"noise_rule","noise_action":"add","rules":[{"category":"console","classification":"test","match_spec":{"message_regex":"test.*pattern"}}]}'
mcp_call $DAEMON_PORT configure '{"action":"noise_rule","noise_action":"list"}' | jq '.result.rules[] | select(.id=="user_1")'

# Verify file exists
test -f "$PROJECT_ROOT/.kaboom/noise/rules.json" || exit 1

pkill -9 -f "kaboom-mcp.*$DAEMON_PORT" || true
sleep 1

start_daemon $DAEMON_PORT
mcp_call $DAEMON_PORT configure '{"action":"noise_rule","noise_action":"list"}' | jq '.result.rules[] | select(.id=="user_1")' || exit 1
echo "✅ Test 1 passed"

pkill -9 -f "kaboom-mcp.*$DAEMON_PORT" || true
rm -rf "$TMPDIR" "$PROJECT_ROOT/.kaboom"
```

---

## Execution Procedure

1. **Compile:**
   ```bash
   make compile-ts
   make build
   ```

2. **Run All Tests:**
   ```bash
   ./scripts/uat/runners/test-all-tools-comprehensive.sh
   ```

3. **Run Persistence Tests Only:**
   ```bash
   go test ./internal/noise -run TestNoiseConfigWithStore -v
   ```

4. **Manual UAT:**
   - Run `test-noise-persistence.sh` (automated) OR
   - Follow individual scenarios above (manual exploration)

5. **Inspection:**
   ```bash
   # Check persisted file
   cat .kaboom/noise/rules.json | jq .

   # Check server logs for persistence errors
   tail -f daemon.log | grep kaboom
   ```

---

## Success Criteria

✅ **All 10 scenarios pass**
✅ **No data loss** (user rules survive restart)
✅ **No collisions** (counter prevents user_1 → user_1)
✅ **Graceful failure** (corrupted file doesn't crash)
✅ **Filtering works** (persisted rules still match entries)
✅ **Built-ins fresh** (not persisted, always loaded from code)
✅ **Backward compatible** (works without SessionStore)
✅ **Fully tested** (9 unit tests + 10 UAT scenarios)
