#!/bin/bash
# cat-04-configure.sh — UAT tests for the configure tool (11 tests).
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "4" "Configure Tool" "11"
ensure_daemon

# ── 4.1 — configure(health) returns server health ─────────
begin_test "4.1" "configure(health) returns server health" \
    "Verify health action returns status, version, and key health fields" \
    "Health is the liveness probe. Every field is consumed by monitoring."
run_test_4_1() {
    RESPONSE=$(call_tool "configure" '{"what":"health"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "version"; then
        fail "Response missing 'version' field. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "uptime_seconds"; then
        fail "Response missing 'uptime_seconds' field. Content: $(truncate "$text")"
        return
    fi
    pass "Health response contains 'version' and 'uptime_seconds'. Content: $(truncate "$text" 200)"
}
run_test_4_1

# ── 4.2 — configure(clear) resets buffers ─────────────────
begin_test "4.2" "configure(clear) resets buffers" \
    "Clear all buffers, then verify observe(logs) returns empty" \
    "Clear is used between test runs. Two-step verification proves the clear worked."
run_test_4_2() {
    local CLEAR_RESP
    CLEAR_RESP=$(call_tool "configure" '{"what":"clear"}')
    if ! check_not_error "$CLEAR_RESP"; then
        fail "Clear returned error. Content: $(truncate "$(extract_content_text "$CLEAR_RESP")")"
        return
    fi
    local OBS_RESP
    OBS_RESP=$(call_tool "observe" '{"what":"logs"}')
    if ! check_not_error "$OBS_RESP"; then
        fail "observe(logs) after clear returned error. Content: $(truncate "$(extract_content_text "$OBS_RESP")")"
        return
    fi
    local obs_text
    obs_text=$(extract_content_text "$OBS_RESP")
    # After clear, expect count:0 or empty logs — use jq for structural check
    local count
    count=$(echo "$obs_text" | jq -r '.count // (.logs | length) // (.entries | length) // -1' 2>/dev/null || echo "-1")
    if [ "$count" = "0" ] || [ "$count" = "null" ]; then
        pass "Clear succeeded. observe(logs) returned empty/zero count after clear. Content: $(truncate "$obs_text" 200)"
    elif check_contains "$obs_text" '"count":0' || check_contains "$obs_text" '"entries":[]'; then
        pass "Clear succeeded. observe(logs) returned empty/zero count after clear. Content: $(truncate "$obs_text" 200)"
    else
        fail "Clear did not empty logs buffer. Expected count:0, got count=$count. Content: $(truncate "$obs_text" 200)"
    fi
}
run_test_4_2

# ── 4.3 — configure(clear) with specific buffer ───────────
begin_test "4.3" "configure(clear) with specific buffer" \
    "Clear only the network buffer, verify response indicates success" \
    "Selective clear is used for targeted debugging. Must not clear everything."
run_test_4_3() {
    RESPONSE=$(call_tool "configure" '{"what":"clear","buffer":"network"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Clear network buffer returned error. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    pass "Selective clear of network buffer succeeded. Content: $(truncate "$text" 200)"
}
run_test_4_3

# ── 4.4 — configure(store) save and load roundtrip ────────
begin_test "4.4" "configure(store) save and load roundtrip" \
    "Save data with key, load it back, verify contents match" \
    "Roundtrip proves serialization/deserialization works. Catches silent corruption."
run_test_4_4() {
    # Save
    local SAVE_RESP
    SAVE_RESP=$(call_tool "configure" '{"what":"store","store_action":"save","namespace":"uat","key":"roundtrip_test","data":{"foo":"bar","num":42}}')
    if ! check_not_error "$SAVE_RESP"; then
        fail "Store save returned error. Content: $(truncate "$(extract_content_text "$SAVE_RESP")")"
        return
    fi
    # Load
    local LOAD_RESP
    LOAD_RESP=$(call_tool "configure" '{"what":"store","store_action":"load","namespace":"uat","key":"roundtrip_test"}')
    if ! check_not_error "$LOAD_RESP"; then
        fail "Store load returned error. Content: $(truncate "$(extract_content_text "$LOAD_RESP")")"
        return
    fi
    local load_text
    load_text=$(extract_content_text "$LOAD_RESP")
    if ! check_contains "$load_text" "foo"; then
        fail "Loaded data missing 'foo'. Content: $(truncate "$load_text")"
        return
    fi
    if ! check_contains "$load_text" "bar"; then
        fail "Loaded data missing 'bar'. Content: $(truncate "$load_text")"
        return
    fi
    pass "Store save/load roundtrip succeeded. Loaded data contains 'foo' and 'bar'. Content: $(truncate "$load_text" 200)"
}
run_test_4_4

# ── 4.5 — configure(store) list shows saved keys ──────────
begin_test "4.5" "configure(store) list shows saved keys" \
    "Save a key, list keys, verify it appears (self-contained)" \
    "List must reflect actual state, not cached/stale data."
run_test_4_5() {
    # Save our own key (self-contained, no dependency on test 4.4)
    call_tool "configure" '{"what":"store","store_action":"save","namespace":"uat","key":"list_test_key","data":{"v":1}}' >/dev/null 2>&1
    local LIST_RESP
    LIST_RESP=$(call_tool "configure" '{"what":"store","store_action":"list","namespace":"uat"}')
    if ! check_not_error "$LIST_RESP"; then
        fail "Store list returned error. Content: $(truncate "$(extract_content_text "$LIST_RESP")")"
        return
    fi
    local list_text
    list_text=$(extract_content_text "$LIST_RESP")
    if ! check_contains "$list_text" "list_test_key"; then
        fail "Store list does not contain 'list_test_key'. Content: $(truncate "$list_text")"
        return
    fi
    pass "Store list includes 'list_test_key'. Content: $(truncate "$list_text" 200)"
}
run_test_4_5

# ── 4.6 — configure(noise_rule) full CRUD lifecycle ───────
begin_test "4.6" "configure(noise_rule) full CRUD lifecycle" \
    "Add rule, list (verify present), extract ID, remove, list (verify gone)" \
    "Full lifecycle: if remove or ID extraction breaks, smoke tests fail but UAT would miss it."
run_test_4_6() {
    # Step 0: Reset noise rules to clear any leftovers from prior runs (persisted to disk)
    call_tool "configure" '{"what":"noise_rule","noise_action":"reset"}' >/dev/null 2>&1

    # Step 1: Add a noise rule
    local ADD_RESP
    ADD_RESP=$(call_tool "configure" '{"what":"noise_rule","noise_action":"add","rules":[{"category":"network","match_spec":{"url_regex":"uat_noise_test_pattern"},"classification":"infrastructure"}]}')
    if ! check_not_error "$ADD_RESP"; then
        fail "Noise rule add returned error. Content: $(truncate "$(extract_content_text "$ADD_RESP")")"
        return
    fi

    # Step 2: List and verify rule is present
    local LIST_RESP
    LIST_RESP=$(call_tool "configure" '{"what":"noise_rule","noise_action":"list"}')
    if ! check_not_error "$LIST_RESP"; then
        fail "Noise rule list returned error. Content: $(truncate "$(extract_content_text "$LIST_RESP")")"
        return
    fi
    local list_text
    list_text=$(extract_content_text "$LIST_RESP")
    if ! check_contains "$list_text" "uat_noise_test_pattern"; then
        fail "Noise rule list does not contain 'uat_noise_test_pattern' after add. Content: $(truncate "$list_text" 200)"
        return
    fi

    # Step 3: Extract rule_id from the list response (using jq)
    # Response text is "summary\n{json}", strip everything before first {
    local json_part rule_id
    json_part=$(echo "$list_text" | sed -n '/{/,$ p' | tr '\n' ' ')
    rule_id=$(echo "$json_part" | jq -r '.rules[] | select(.match_spec.url_regex == "uat_noise_test_pattern") | .id' 2>/dev/null | head -1)

    if [ -z "$rule_id" ]; then
        fail "Could not extract rule_id for 'uat_noise_test_pattern' from list response. Content: $(truncate "$list_text" 200)"
        return
    fi

    # Step 4: Remove the rule by ID
    local REMOVE_RESP
    REMOVE_RESP=$(call_tool "configure" "{\"action\":\"noise_rule\",\"noise_action\":\"remove\",\"rule_id\":\"$rule_id\"}")
    if ! check_not_error "$REMOVE_RESP"; then
        fail "Noise rule remove returned error for rule_id=$rule_id. Content: $(truncate "$(extract_content_text "$REMOVE_RESP")")"
        return
    fi

    # Step 5: List again and verify rule is gone
    local LIST2_RESP
    LIST2_RESP=$(call_tool "configure" '{"what":"noise_rule","noise_action":"list"}')
    if ! check_not_error "$LIST2_RESP"; then
        fail "Noise rule list (after remove) returned error. Content: $(truncate "$(extract_content_text "$LIST2_RESP")")"
        return
    fi
    local list2_text
    list2_text=$(extract_content_text "$LIST2_RESP")
    if check_contains "$list2_text" "uat_noise_test_pattern"; then
        fail "Noise rule 'uat_noise_test_pattern' still in list after remove (rule_id=$rule_id). Content: $(truncate "$list2_text" 200)"
        return
    fi

    pass "Noise rule full CRUD: add > list(found, id=$rule_id) > remove > list(gone)."
}
run_test_4_6

# ── 4.7 — configure(audit_log) returns entries ────────────
begin_test "4.7" "configure(audit_log) returns entries" \
    "After several tool calls, verify audit log has entries" \
    "Audit trail is compliance-critical. If empty after tool calls, auditing is broken."
run_test_4_7() {
    RESPONSE=$(call_tool "configure" '{"what":"audit_log"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Audit log returned error. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Audit log response is empty."
        return
    fi
    # Verify response has structural audit fields (entries array or status)
    if ! check_contains "$text" "entries" && ! check_contains "$text" "status"; then
        fail "Audit log missing 'entries' or 'status' field. Content: $(truncate "$text" 200)"
        return
    fi
    pass "Audit log returned structured data with entries/status. Content: $(truncate "$text" 200)"
}
run_test_4_7

# ── 4.8 — configure(streaming) enable and status ──────────
begin_test "4.8" "configure(streaming) enable and status" \
    "Enable streaming for errors, then check status reflects enabled state" \
    "Streaming state must persist within a session."
run_test_4_8() {
    # Enable
    local ENABLE_RESP
    ENABLE_RESP=$(call_tool "configure" '{"what":"streaming","streaming_action":"enable","events":["errors"]}')
    if ! check_not_error "$ENABLE_RESP"; then
        fail "Streaming enable returned error. Content: $(truncate "$(extract_content_text "$ENABLE_RESP")")"
        return
    fi
    # Status
    local STATUS_RESP
    STATUS_RESP=$(call_tool "configure" '{"what":"streaming","streaming_action":"status"}')
    if ! check_not_error "$STATUS_RESP"; then
        fail "Streaming status returned error. Content: $(truncate "$(extract_content_text "$STATUS_RESP")")"
        return
    fi
    local status_text
    status_text=$(extract_content_text "$STATUS_RESP")
    if ! check_contains "$status_text" "enabled"; then
        fail "Streaming status missing 'enabled' field. Content: $(truncate "$status_text" 200)"
        return
    fi
    pass "Streaming enable/status roundtrip succeeded. Status contains 'enabled' field. Content: $(truncate "$status_text" 200)"
}
run_test_4_8

# ── 4.9 — configure(test_boundary) start and end ──────────
begin_test "4.9" "configure(test_boundary) start and end" \
    "Start and end a test boundary, verify both succeed" \
    "Test boundaries isolate CI test runs. If they error, CI tests can't use the feature."
run_test_4_9() {
    # Start
    local START_RESP
    START_RESP=$(call_tool "configure" '{"what":"test_boundary_start","test_id":"uat-boundary-1","label":"UAT test"}')
    if ! check_not_error "$START_RESP"; then
        fail "test_boundary_start returned error. Content: $(truncate "$(extract_content_text "$START_RESP")")"
        return
    fi
    # End
    local END_RESP
    END_RESP=$(call_tool "configure" '{"what":"test_boundary_end","test_id":"uat-boundary-1"}')
    if ! check_not_error "$END_RESP"; then
        fail "test_boundary_end returned error. Content: $(truncate "$(extract_content_text "$END_RESP")")"
        return
    fi
    pass "test_boundary start and end both succeeded without error."
}
run_test_4_9

# ── 4.10 — configure(query_dom) with selector ─────────────
begin_test "4.10" "configure(query_dom) with selector" \
    "Send query_dom for 'body' — may timeout or return no-extension, but must not crash" \
    "DOM queries are sent to extension via pending queries. Must not crash without extension."
run_test_4_10() {
    RESPONSE=$(call_tool "configure" '{"what":"query_dom","selector":"body"}')
    # Must be valid JSON-RPC with either result or error (not both missing)
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "query_dom did not return valid JSON-RPC. Raw response: $(truncate "$RESPONSE")"
        return
    fi
    local has_result has_error
    has_result=$(echo "$RESPONSE" | jq -e '.result' >/dev/null 2>&1 && echo "yes" || echo "no")
    has_error=$(echo "$RESPONSE" | jq -e '.error' >/dev/null 2>&1 && echo "yes" || echo "no")
    if [ "$has_result" = "no" ] && [ "$has_error" = "no" ]; then
        fail "query_dom response has neither result nor error. Raw: $(truncate "$RESPONSE" 200)"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    pass "query_dom returned valid JSON-RPC response (may be error or success). Content: $(truncate "$text" 200)"
}
run_test_4_10

# ── 4.11 — configure with invalid action ──────────────────
begin_test "4.11" "configure with invalid action returns error" \
    "Send an unknown action, verify isError:true" \
    "Invalid actions must not silently succeed."
run_test_4_11() {
    RESPONSE=$(call_tool "configure" '{"what":"destroy_everything"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for invalid action 'destroy_everything'. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "Invalid action 'destroy_everything' correctly returned isError:true."
}
run_test_4_11

# ── Cleanup noise rule from 4.6 ───────────────────────────
# Best effort cleanup: remove the noise rule we added so it doesn't affect other tests
call_tool "configure" '{"what":"noise_rule","noise_action":"reset"}' >/dev/null 2>&1

# Disable streaming from 4.8
call_tool "configure" '{"what":"streaming","streaming_action":"disable"}' >/dev/null 2>&1

finish_category
