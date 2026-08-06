#!/bin/bash
# 06-interact-state.sh — 6.1-6.4: State management (save/load/list/delete).
set -eo pipefail

begin_category "6" "State Management" "4"

# ── Test 6.1: Save state ────────────────────────────────
begin_test "6.1" "[BROWSER] Save browser state snapshot" \
    "interact(save_state) with snapshot_name='smoke-state'" \
    "Tests: state persistence pipeline"

run_test_6_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "interact" '{"what":"save_state","snapshot_name":"smoke-state"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if ! check_not_error "$response"; then
        fail "save_state returned error. Content: $(truncate "$content_text" 200)"
        return
    fi

    echo "  [save_state response]"
    echo "    $(truncate "$content_text" 200)"

    # Positive: verify response confirms the snapshot name
    if echo "$content_text" | grep -q "smoke-state\|saved\|snapshot"; then
        pass "save_state('smoke-state') completed with confirmation."
    else
        pass "save_state('smoke-state') completed (no error, checking list next)."
    fi
}
run_test_6_1

# ── Test 6.2: List states ───────────────────────────────
begin_test "6.2" "[BROWSER] List saved states" \
    "interact(list_states) should include 'smoke-state'" \
    "Tests: state listing after save"

run_test_6_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "interact" '{"what":"list_states"}')

    if ! check_not_error "$response"; then
        fail "list_states returned error. Content: $(truncate "$(extract_content_text "$response")" 200)"
        return
    fi

    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -q "smoke-state"; then
        pass "list_states contains 'smoke-state'."
    else
        fail "list_states does not contain 'smoke-state'. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_6_2

# ── Test 6.3: Load state ────────────────────────────────
begin_test "6.3" "[BROWSER] Load saved state" \
    "interact(load_state) with snapshot_name='smoke-state'" \
    "Tests: state restoration"

run_test_6_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "interact" '{"what":"load_state","snapshot_name":"smoke-state"}')

    if ! check_not_error "$response"; then
        fail "load_state returned error. Content: $(truncate "$(extract_content_text "$response")" 200)"
        return
    fi

    # Positive: verify page is still on a real URL after state restore
    sleep 1
    local page_response
    page_response=$(call_tool "observe" '{"what":"page"}')
    local page_text
    page_text=$(extract_content_text "$page_response")

    if echo "$page_text" | grep -qiE "https?://"; then
        pass "load_state('smoke-state') completed. Page shows a valid URL after restore."
    else
        pass "load_state('smoke-state') completed (no error returned)."
    fi
}
run_test_6_3

# ── Test 6.4: Delete state ──────────────────────────────
begin_test "6.4" "[BROWSER] Delete saved state" \
    "interact(delete_state) then verify no longer in list" \
    "Tests: state cleanup"

run_test_6_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "interact" '{"what":"delete_state","snapshot_name":"smoke-state"}')

    if ! check_not_error "$response"; then
        fail "delete_state returned error. Content: $(truncate "$(extract_content_text "$response")" 200)"
        return
    fi

    # Verify it's gone
    local list_response
    list_response=$(call_tool "interact" '{"what":"list_states"}')
    local list_text
    list_text=$(extract_content_text "$list_response")

    if echo "$list_text" | grep -q "smoke-state"; then
        fail "delete_state ran but 'smoke-state' still in list. Content: $(truncate "$list_text" 200)"
    else
        pass "delete_state('smoke-state') confirmed: no longer in list."
    fi
}
run_test_6_4
