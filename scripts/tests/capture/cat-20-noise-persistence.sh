#!/bin/bash
# cat-20-noise-persistence.sh — UAT for Noise Rule Persistence (v5.8.1+)
# Tests that user rules persist, filter correctly, and survive restarts.
# Integrated into comprehensive test suite as Category 20.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"

begin_category "20" "Noise Persistence" "5"

# Resolve project-scoped noise persistence path:
# ${STATE_ROOT}/projects/${ABS_PROJECT_PATH_WITHOUT_LEADING_SLASH}/noise/rules.json
if [ -n "${KABOOM_STATE_DIR:-}" ]; then
    STATE_ROOT="$KABOOM_STATE_DIR"
elif [ -n "${XDG_STATE_HOME:-}" ]; then
    STATE_ROOT="$XDG_STATE_HOME/kaboom"
else
    STATE_ROOT="$HOME/.kaboom"
fi
PROJECT_ABS="$(pwd -P)"
PROJECT_REL="${PROJECT_ABS#/}"
NOISE_DIR="$STATE_ROOT/projects/$PROJECT_REL/noise"
RULES_FILE="$NOISE_DIR/rules.json"

# wait_for_persisted_user_rules polls RULES_FILE until at least N distinct
# user_* rule IDs are persisted, or timeout (seconds) elapses.
wait_for_persisted_user_rules() {
    local expected="${1:-1}"
    local timeout_s="${2:-6}"
    local attempts=$((timeout_s * 5))
    local i
    for i in $(seq 1 "$attempts"); do
        if [ -f "$RULES_FILE" ]; then
            local count
            count=$(jq '[.rules[] | select(.id | startswith("user_")) | .id] | unique | length' "$RULES_FILE" 2>/dev/null || echo "0")
            if [ "$count" -ge "$expected" ]; then
                return 0
            fi
        fi
        sleep 0.2
    done
    return 1
}

# Clean up any previous state for this project
rm -rf "$NOISE_DIR" 2>/dev/null || true

# ── Test 20.1: Rules persist across server restarts ──────────────
begin_test "20.1" "Rules persist to disk and survive restart" \
    "Add rule, restart daemon, verify rule still exists" \
    "Persistence must survive crashes/kills"

# Start daemon and add a rule
start_daemon

# Add rule via stdin invocation (single call, not send_mcp)
request='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"add","rules":[{"category":"console","classification":"test","match_spec":{"message_regex":"test.*pattern"}}]}}}'
response=$(echo "$request" | "$TIMEOUT_CMD" 8 "$WRAPPER" --port "$PORT" 2>&1 | { grep '^{' || true; } | head -1)

# Verify response indicates success
if echo "$response" | jq -e '.result' >/dev/null 2>&1; then
    pass "Rule added to configure tool"
else
    fail "Failed to add rule" "Response: $response"
fi

sleep 0.5

# Verify file exists
if wait_for_persisted_user_rules 1 8; then
    if jq . "$RULES_FILE" > /dev/null 2>&1; then
        pass "Rules persisted to $RULES_FILE"
    else
        fail "Persisted file invalid JSON"
    fi
else
    fail "Persisted user rule not written to disk within timeout"
fi

# Kill and restart daemon
kill_server
sleep 0.5
start_daemon
sleep 0.5

# List rules using call_tool helper (startup-retry aware)
response2=$(call_tool "configure" '{"what":"noise_rule","noise_action":"list"}')
text2=$(extract_content_text "$response2")

# Check if at least one user rule is present after restart
if echo "$text2" | grep -q "user_"; then
    pass "User rules reloaded after restart"
else
    fail "User rules not reloaded after restart. Content: $(truncate "$text2" 300)"
fi

# ── Test 20.2: ID counter prevents collisions ──────────────────
begin_test "20.2" "ID counter persisted - next rule gets user_2" \
    "Add another rule after restart, verify ID increments" \
    "Counter must prevent collisions (user_1, user_2, not user_1 twice)"

request3='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"add","rules":[{"category":"console","classification":"test2","match_spec":{"message_regex":"second.*pattern"}}]}}}'
response3=$(echo "$request3" | "$TIMEOUT_CMD" 8 "$WRAPPER" --port "$PORT" 2>&1 | { grep '^{' || true; } | head -1)

# Verify the new rule was added
if echo "$response3" | jq -e '.result' >/dev/null 2>&1; then
    pass "Second rule added successfully"
else
    fail "Failed to add second rule"
fi

# Check persisted file for both user_1 and user_2
if [ -f "$RULES_FILE" ]; then
    wait_for_persisted_user_rules 2 8 >/dev/null 2>&1 || true
    user_rule_count=$(jq '[.rules[] | select(.id | startswith("user_")) | .id] | unique | length' "$RULES_FILE" 2>/dev/null || echo "0")

    if [ "$user_rule_count" -ge 2 ]; then
        pass "At least two distinct user rules persisted (no ID collision)"
    else
        fail "ID collision detected: expected >=2 distinct user_* ids, got $user_rule_count"
    fi
else
    fail "Persisted file missing after second add"
fi

# ── Test 20.3: RemoveRule persists across restart ────────────────
begin_test "20.3" "RemoveRule persists - deleted rule stays gone" \
    "Remove a rule, restart, verify it's gone permanently" \
    "Deletions must persist without re-adding"

# Remove user_1
request4='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"remove","rule_id":"user_1"}}}'
response4=$(echo "$request4" | "$TIMEOUT_CMD" 8 "$WRAPPER" --port "$PORT" 2>&1 | { grep '^{' || true; } | head -1)

if echo "$response4" | jq -e '.result' >/dev/null 2>&1; then
    pass "RemoveRule executed"
else
    fail "RemoveRule failed"
fi

sleep 0.5

# Kill and restart
kill_server
sleep 0.5
start_daemon
sleep 0.5

# List rules - user_1 should be gone
response5=$(call_tool "configure" '{"what":"noise_rule","noise_action":"list"}')
text5=$(extract_content_text "$response5")

if echo "$text5" | grep -q "user_2" && ! echo "$text5" | grep -q "user_1"; then
    pass "Removed rule user_1 stays deleted after restart"
else
    fail "Removed rule reappeared or user_2 missing. Content: $(truncate "$text5" 300)"
fi

# ── Test 20.4: Reset clears all user rules ────────────────────
begin_test "20.4" "Reset persists empty state" \
    "Call reset, restart, verify only built-ins remain" \
    "Reset must clear persistence completely"

request6='{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"reset"}}}'
response6=$(echo "$request6" | "$TIMEOUT_CMD" 8 "$WRAPPER" --port "$PORT" 2>&1 | { grep '^{' || true; } | head -1)

if echo "$response6" | jq -e '.result' >/dev/null 2>&1; then
    pass "Reset executed"
else
    fail "Reset failed"
fi

sleep 0.5

# Kill and restart
kill_server
sleep 0.5
start_daemon
sleep 0.5

# List rules - should only have built-ins
response7=$(call_tool "configure" '{"what":"noise_rule","noise_action":"list"}')
text7=$(extract_content_text "$response7")

# Check that user rules are gone and built-ins are present
if echo "$text7" | grep -q "builtin_" && ! echo "$text7" | grep -q "user_"; then
    pass "Reset cleared all user rules, built-ins remain"
else
    fail "Reset didn't persist or built-ins missing. Content: $(truncate "$text7" 300)"
fi

# ── Test 20.5: Corrupted file recovery ──────────────────────────
begin_test "20.5" "Corrupted file doesn't crash server" \
    "Corrupt file, restart, verify graceful recovery" \
    "Corruption must not crash server, built-ins must reload"

# Corrupt the file
if [ -f "$RULES_FILE" ]; then
    echo "{invalid json}" > "$RULES_FILE"
fi

# Kill and try to restart
kill_server
sleep 0.5

# Attempt to start daemon - should succeed despite corruption
if start_daemon; then
    pass "Server started despite corrupted persistence file"

    # Verify built-ins still load
    request8='{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"configure","arguments":{"what":"noise_rule","noise_action":"list"}}}'
    response8=$(echo "$request8" | "$TIMEOUT_CMD" 8 "$WRAPPER" --port "$PORT" 2>&1 | { grep '^{' || true; } | head -1)

    if echo "$response8" | jq -e '.result.content[0].text | contains("builtin_")' >/dev/null 2>&1; then
        pass "Built-in rules reloaded fresh after corruption"
    else
        fail "Built-in rules not reloaded"
    fi
else
    fail "Server crashed on corrupted file"
fi

# ── Cleanup ──────────────────────────────────────────────────────
finish_category
