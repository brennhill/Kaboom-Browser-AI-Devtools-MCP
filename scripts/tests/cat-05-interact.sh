#!/bin/bash
# cat-05-interact.sh — UAT tests for the interact tool (19 tests).
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "5" "Interact Tool" "19"
ensure_daemon

# ── 5.1 — interact(list_states) returns array ─────────────
begin_test "5.1" "interact(list_states) returns array" \
    "Verify list_states returns states array and count" \
    "list_states doesn't require pilot. Must always work."
run_test_5_1() {
    RESPONSE=$(call_tool "interact" '{"what":"list_states"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "states"; then
        fail "list_states response missing 'states' field. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "list_states response missing 'count' field. Content: $(truncate "$text")"
        return
    fi
    pass "list_states returned valid response with 'states' and 'count'. Content: $(truncate "$text" 200)"
}
run_test_5_1

# ── 5.2 — interact save/load/delete roundtrip ─────────────
begin_test "5.2" "interact save/load/delete state roundtrip" \
    "Full CRUD: save state, list to verify, load it, delete it, list to confirm deletion" \
    "Full CRUD roundtrip. If any step fails, state management is broken."
run_test_5_2() {
    # Step 1: Save state
    local SAVE_RESP
    SAVE_RESP=$(call_tool "interact" '{"what":"save_state","snapshot_name":"uat-state-test"}')
    if ! check_not_error "$SAVE_RESP"; then
        fail "save_state returned error. Content: $(truncate "$(extract_content_text "$SAVE_RESP")")"
        return
    fi

    # Step 2: List and verify it appears
    local LIST_RESP
    LIST_RESP=$(call_tool "interact" '{"what":"list_states"}')
    if ! check_not_error "$LIST_RESP"; then
        fail "list_states after save returned error. Content: $(truncate "$(extract_content_text "$LIST_RESP")")"
        return
    fi
    local list_text
    list_text=$(extract_content_text "$LIST_RESP")
    if ! check_contains "$list_text" "uat-state-test"; then
        fail "list_states does not contain 'uat-state-test' after save. Content: $(truncate "$list_text")"
        return
    fi

    # Step 3: Load the state
    local LOAD_RESP
    LOAD_RESP=$(call_tool "interact" '{"what":"load_state","snapshot_name":"uat-state-test"}')
    if ! check_not_error "$LOAD_RESP"; then
        fail "load_state returned error. Content: $(truncate "$(extract_content_text "$LOAD_RESP")")"
        return
    fi

    # Step 4: Delete the state
    local DEL_RESP
    DEL_RESP=$(call_tool "interact" '{"what":"delete_state","snapshot_name":"uat-state-test"}')
    if ! check_not_error "$DEL_RESP"; then
        fail "delete_state returned error. Content: $(truncate "$(extract_content_text "$DEL_RESP")")"
        return
    fi

    # Step 5: List again and verify it's gone
    local LIST2_RESP
    LIST2_RESP=$(call_tool "interact" '{"what":"list_states"}')
    if ! check_not_error "$LIST2_RESP"; then
        fail "list_states after delete returned error. Content: $(truncate "$(extract_content_text "$LIST2_RESP")")"
        return
    fi
    local list2_text
    list2_text=$(extract_content_text "$LIST2_RESP")
    if check_contains "$list2_text" "uat-state-test"; then
        fail "list_states still contains 'uat-state-test' after delete. Content: $(truncate "$list2_text")"
        return
    fi

    pass "Full CRUD roundtrip: save, list (found), load, delete, list (gone). All steps succeeded."
}
run_test_5_2

# ── 5.3 — interact(execute_js) without pilot returns error ─
begin_test "5.3" "interact(execute_js) without pilot returns error" \
    "execute_js requires pilot enabled; without extension it should return isError" \
    "Pilot-gated actions must fail clearly when pilot is off."
run_test_5_3() {
    RESPONSE=$(call_tool "interact" '{"what":"execute_js","script":"return 1+1"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for execute_js without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "execute_js correctly returned isError:true without pilot enabled."
}
run_test_5_3

# ── 5.4 — interact(navigate) without pilot returns error ──
begin_test "5.4" "interact(navigate) without pilot returns error" \
    "navigate requires pilot enabled; without extension it should return isError" \
    "Pilot-gated actions must fail clearly when pilot is off."
run_test_5_4() {
    RESPONSE=$(call_tool "interact" '{"what":"navigate","url":"https://example.com"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for navigate without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "navigate correctly returned isError:true without pilot enabled."
}
run_test_5_4

# ── 5.5 — interact(highlight) without pilot returns error ──
begin_test "5.5" "interact(highlight) without pilot returns error" \
    "highlight requires pilot enabled; without extension it should return isError" \
    "Pilot-gated actions must fail clearly when pilot is off."
run_test_5_5() {
    RESPONSE=$(call_tool "interact" '{"what":"highlight","selector":"body"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for highlight without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "highlight correctly returned isError:true without pilot enabled."
}
run_test_5_5

# ── 5.6 — interact with invalid action ────────────────────
begin_test "5.6" "interact with invalid action returns error" \
    "Send an unknown action, verify isError:true" \
    "Invalid actions must not crash."
run_test_5_6() {
    RESPONSE=$(call_tool "interact" '{"what":"fly_to_moon"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for invalid action 'fly_to_moon'. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "Invalid action 'fly_to_moon' correctly returned isError:true."
}
run_test_5_6

# ── 5.7 — interact(save_state) without name returns error ──
begin_test "5.7" "interact(save_state) without snapshot_name returns error" \
    "save_state requires snapshot_name parameter; omitting it should return isError" \
    "Required param validation."
run_test_5_7() {
    RESPONSE=$(call_tool "interact" '{"what":"save_state"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for save_state without snapshot_name. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "save_state without snapshot_name correctly returned isError:true."
}
run_test_5_7

# ── 5.8 — DOM primitives: click without pilot returns error ──
begin_test "5.8" "interact(click) without pilot returns error" \
    "click requires pilot enabled; without extension it should return isError" \
    "DOM primitive pilot gating."
run_test_5_8() {
    RESPONSE=$(call_tool "interact" '{"what":"click","selector":"#btn"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for click without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "pilot" || check_contains "$text" "Pilot"; then
        pass "click correctly returned pilot disabled error."
    else
        fail "click error should mention pilot. Content: $(truncate "$text")"
    fi
}
run_test_5_8

# ── 5.9 — DOM primitives: all actions pilot-gated ──────────
begin_test "5.9" "All DOM primitive actions pilot-gated" \
    "All 13 DOM actions (click/type/select/check/get_text/get_value/get_attribute/set_attribute/focus/scroll_to/wait_for/key_press/list_interactive) return pilot error" \
    "Every DOM primitive must be pilot-gated."
run_test_5_9() {
    local actions='click type select check get_text get_value get_attribute set_attribute focus scroll_to wait_for key_press list_interactive'
    local failed=""
    for action in $actions; do
        local args
        case "$action" in
            type) args="{\"action\":\"$action\",\"selector\":\"#x\",\"text\":\"hi\"}" ;;
            select) args="{\"action\":\"$action\",\"selector\":\"#x\",\"value\":\"v\"}" ;;
            get_attribute|set_attribute) args="{\"action\":\"$action\",\"selector\":\"#x\",\"name\":\"href\"}" ;;
            key_press) args="{\"action\":\"$action\",\"selector\":\"#x\",\"text\":\"Enter\"}" ;;
            list_interactive) args="{\"action\":\"$action\"}" ;;
            *) args="{\"action\":\"$action\",\"selector\":\"#x\"}" ;;
        esac
        RESPONSE=$(call_tool "interact" "$args")
        if ! check_is_error "$RESPONSE"; then
            failed="$failed $action(not_error)"
        else
            local err_text
            err_text=$(extract_content_text "$RESPONSE")
            if ! check_contains "$err_text" "pilot" && ! check_contains "$err_text" "Pilot"; then
                failed="$failed $action(no_pilot_mention)"
            fi
        fi
    done
    if [ -n "$failed" ]; then
        fail "These actions did NOT return isError without pilot:$failed"
    else
        pass "All 13 DOM primitive actions correctly return pilot disabled error."
    fi
}
run_test_5_9

# ── 5.10 — DOM primitives: missing selector returns error ──
begin_test "5.10" "DOM primitives missing selector returns error" \
    "click/type/focus without selector should return isError mentioning 'selector'" \
    "Required param validation for selector."
run_test_5_10() {
    local failed=""
    for action in click type focus get_text; do
        local args
        case "$action" in
            type) args="{\"action\":\"$action\",\"text\":\"hello\"}" ;;
            *) args="{\"action\":\"$action\"}" ;;
        esac
        RESPONSE=$(call_tool "interact" "$args")
        if ! check_is_error "$RESPONSE"; then
            failed="$failed $action(not_error)"
            continue
        fi
        local text
        text=$(extract_content_text "$RESPONSE")
        if ! check_contains "$text" "selector"; then
            failed="$failed $action(no_selector_mention)"
        fi
    done
    if [ -n "$failed" ]; then
        fail "Selector validation failed for:$failed"
    else
        pass "Missing selector correctly returns error mentioning 'selector' for all tested actions."
    fi
}
run_test_5_10

# ── 5.11 — DOM primitives: type missing text returns error ──
begin_test "5.11" "interact(type) missing text returns error" \
    "type action requires text parameter; omitting it should return isError" \
    "Required param validation for type."
run_test_5_11() {
    RESPONSE=$(call_tool "interact" '{"what":"type","selector":"#input"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for type without text. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "text"; then
        pass "type without text correctly returned error mentioning 'text' parameter."
    else
        fail "type error should mention 'text'. Content: $(truncate "$text")"
    fi
}
run_test_5_11

# ── 5.12 — DOM primitives: select missing value returns error ─
begin_test "5.12" "interact(select) missing value returns error" \
    "select action requires value parameter; omitting it should return isError" \
    "Required param validation for select."
run_test_5_12() {
    RESPONSE=$(call_tool "interact" '{"what":"select","selector":"#dropdown"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for select without value. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "value"; then
        pass "select without value correctly returned error mentioning 'value' parameter."
    else
        fail "select error should mention 'value'. Content: $(truncate "$text")"
    fi
}
run_test_5_12

# ── 5.13 — DOM primitives: get_attribute missing name returns error ─
begin_test "5.13" "interact(get_attribute) missing name returns error" \
    "get_attribute requires name parameter; omitting it should return isError" \
    "Required param validation for get_attribute."
run_test_5_13() {
    RESPONSE=$(call_tool "interact" '{"what":"get_attribute","selector":"#link"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for get_attribute without name. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "name"; then
        pass "get_attribute without name correctly returned error mentioning 'name' parameter."
    else
        fail "get_attribute error should mention 'name'. Content: $(truncate "$text")"
    fi
}
run_test_5_13

# ── 5.14 — DOM primitives: key_press without pilot returns error ──
begin_test "5.14" "interact(key_press) without pilot returns error" \
    "key_press requires pilot enabled; without extension it should return isError" \
    "key_press pilot gating."
run_test_5_14() {
    RESPONSE=$(call_tool "interact" '{"what":"key_press","selector":"#input","text":"Enter"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for key_press without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "pilot" || check_contains "$text" "Pilot"; then
        pass "key_press correctly returned pilot disabled error."
    else
        fail "key_press error should mention pilot. Content: $(truncate "$text")"
    fi
}
run_test_5_14

# ── 5.15 — DOM primitives: key_press missing selector returns error ──
begin_test "5.15" "interact(key_press) missing selector returns error" \
    "key_press requires selector parameter; omitting it should return isError mentioning 'selector'" \
    "Required param validation for key_press."
run_test_5_15() {
    RESPONSE=$(call_tool "interact" '{"what":"key_press","text":"Enter"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for key_press without selector. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "selector"; then
        pass "key_press without selector correctly returned error mentioning 'selector' parameter."
    else
        fail "key_press error should mention 'selector'. Content: $(truncate "$text")"
    fi
}
run_test_5_15

# ── 5.16 — interact(subtitle) standalone: set text ────────
begin_test "5.16" "interact(subtitle) standalone sets text" \
    "Send subtitle action with text, verify response confirms subtitle was set" \
    "Core subtitle functionality — standalone text overlay."
run_test_5_16() {
    RESPONSE=$(call_tool "interact" '{"what":"subtitle","text":"UAT subtitle test message"}')
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_is_error "$RESPONSE"; then
        # In headless/no-extension environments subtitle dispatch returns no_data.
        if check_matches "$text" "no_data|extension.*not connected|tracked"; then
            pass "subtitle returned expected no-extension error in this environment. Content: $(truncate "$text" 200)"
        else
            fail "subtitle returned unexpected error. Content: $(truncate "$text" 200)"
        fi
        return
    fi
    if check_contains "$text" "subtitle" || check_contains "$text" "queued" || check_contains "$text" "status"; then
        pass "subtitle standalone accepted. Response: $(truncate "$text" 200)"
    else
        fail "subtitle response missing expected fields (subtitle/queued/status). Content: $(truncate "$text" 200)"
    fi
}
run_test_5_16

# ── 5.17 — interact(subtitle) standalone: clear text ─────
begin_test "5.17" "interact(subtitle) with empty text clears subtitle" \
    "Send subtitle with empty string text, verify response confirms subtitle was cleared" \
    "Clearing subtitle must work — empty text = dismiss overlay."
run_test_5_17() {
    RESPONSE=$(call_tool "interact" '{"what":"subtitle","text":""}')
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_is_error "$RESPONSE"; then
        if check_matches "$text" "no_data|extension.*not connected|tracked"; then
            pass "subtitle clear returned expected no-extension error in this environment. Content: $(truncate "$text" 200)"
        else
            fail "subtitle clear returned unexpected error. Content: $(truncate "$text" 200)"
        fi
        return
    fi
    if check_contains "$text" "clear" || check_contains "$text" "subtitle" || check_contains "$text" "status"; then
        pass "subtitle clear accepted. Response: $(truncate "$text" 200)"
    else
        fail "subtitle clear response missing expected fields. Content: $(truncate "$text" 200)"
    fi
}
run_test_5_17

# ── 5.18 — interact(subtitle) missing text returns error ──
begin_test "5.18" "interact(subtitle) missing text returns error" \
    "subtitle action requires text parameter; omitting it should return isError" \
    "Required param validation for subtitle."
run_test_5_18() {
    RESPONSE=$(call_tool "interact" '{"what":"subtitle"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for subtitle without text. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "text"; then
        pass "subtitle without text correctly returned error mentioning 'text' parameter."
    else
        fail "subtitle error should mention 'text'. Content: $(truncate "$text")"
    fi
}
run_test_5_18

# ── 5.19 — interact(navigate) with subtitle param ────────
begin_test "5.19" "interact(navigate) with subtitle param accepted" \
    "Send navigate with optional subtitle param, verify response does not error on the param" \
    "Subtitle as optional param on any interact action — must not reject unknown field."
run_test_5_19() {
    RESPONSE=$(call_tool "interact" '{"what":"navigate","url":"https://example.com","subtitle":"Navigating to example.com for testing"}')
    # navigate without pilot will return isError (pilot not enabled in UAT),
    # but the error should be about pilot, NOT about an unknown 'subtitle' param
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "subtitle.*unknown|unknown.*subtitle|invalid.*param.*subtitle|unrecognized.*subtitle"; then
        fail "Server rejected 'subtitle' as unknown parameter. It should be accepted as optional. Content: $(truncate "$text" 200)"
        return
    fi
    # Expected: pilot error (not connected), OR success if pilot is running
    if check_matches "$text" "pilot|Pilot|queued|navigat"; then
        pass "navigate with subtitle param accepted (got expected pilot error or success, not param rejection)."
    else
        pass "navigate with subtitle param did not reject the subtitle field. Content: $(truncate "$text" 200)"
    fi
}
run_test_5_19

finish_category
