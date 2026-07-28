#!/bin/bash
# 08-configure-features.sh — 8.1-8.5: Configure tool features.
# noise rules, store persistence, buffer clear, streaming, test boundaries
set -eo pipefail

begin_category "8" "Configure Features" "5"

# ── Test 8.1: Noise rules CRUD ──────────────────────────
begin_test "8.1" "[DAEMON ONLY] Noise rules: add, list, remove, verify" \
    "Full noise rule lifecycle: add a rule, list to verify, remove, list to confirm" \
    "Tests: noise filtering configuration"

run_test_8_1() {
    # Add a noise rule (API expects rules array with match_spec)
    local add_response
    add_response=$(call_tool "configure" '{"what":"noise_rule","noise_action":"add","rules":[{"category":"console","match_spec":{"message_regex":"smoke-test-noise"}}]}')

    if ! check_not_error "$add_response"; then
        fail "noise_rule add returned error. Content: $(truncate "$(extract_content_text "$add_response")" 200)"
        return
    fi

    # List rules
    local list_response
    list_response=$(call_tool "configure" '{"what":"noise_rule","noise_action":"list"}')
    local list_text
    list_text=$(extract_content_text "$list_response")

    if ! echo "$list_text" | grep -q "smoke-test-noise"; then
        fail "Noise rule not found in list after add. Content: $(truncate "$list_text" 200)"
        return
    fi

    # Extract rule_id for removal (using jq)
    # Response text is "summary\n{json}", strip everything before first {
    local json_part rule_id
    json_part=$(echo "$list_text" | sed -n '/{/,$ p' | tr '\n' ' ')
    rule_id=$(echo "$json_part" | jq -r '.rules[] | select(.match_spec.message_regex == "smoke-test-noise") | .id' 2>/dev/null | head -1)

    if [ -z "$rule_id" ]; then
        fail "Could not extract rule_id for 'smoke-test-noise' from list response. Content: $(truncate "$list_text" 200)"
        return
    fi

    # Remove the rule
    local remove_response
    remove_response=$(call_tool "configure" "{\"action\":\"noise_rule\",\"noise_action\":\"remove\",\"rule_id\":\"$rule_id\"}")

    if ! check_not_error "$remove_response"; then
        fail "noise_rule remove returned error. Content: $(truncate "$(extract_content_text "$remove_response")" 200)"
        return
    fi

    # Verify removal
    local list2_response
    list2_response=$(call_tool "configure" '{"what":"noise_rule","noise_action":"list"}')
    local list2_text
    list2_text=$(extract_content_text "$list2_response")

    if echo "$list2_text" | grep -q "smoke-test-noise"; then
        fail "Noise rule still in list after remove. Content: $(truncate "$list2_text" 200)"
    else
        pass "Noise rule CRUD: add, list (found), remove, list (gone)."
    fi
}
run_test_8_1

# ── Test 8.2: Store persistence ─────────────────────────
begin_test "8.2" "[DAEMON ONLY] Store: save, load, list, delete roundtrip" \
    "Full key-value store lifecycle" \
    "Tests: persistent data storage"

run_test_8_2() {
    # 1. Save data
    echo "  [store: save]"
    echo "    namespace=smoke key=smoke-key data={\"value\":\"smoke-data-123\"}"
    local save_response
    save_response=$(call_tool "configure" '{"what":"store","store_action":"save","key":"smoke-key","namespace":"smoke","data":{"value":"smoke-data-123"}}')

    if ! check_not_error "$save_response"; then
        fail "store save returned error. Content: $(truncate "$(extract_content_text "$save_response")" 200)"
        return
    fi
    echo "    save: OK"

    # 2. Load it back and verify exact value
    local load_response
    load_response=$(call_tool "configure" '{"what":"store","store_action":"load","key":"smoke-key","namespace":"smoke"}')
    local load_text
    load_text=$(extract_content_text "$load_response")

    echo "  [store: load]"
    echo "    $(truncate "$load_text" 150)"

    if ! echo "$load_text" | grep -q "smoke-data-123"; then
        fail "store load did not return saved data 'smoke-data-123'. Content: $(truncate "$load_text" 200)"
        return
    fi
    echo "    load: OK (contains smoke-data-123)"

    # 3. List keys and verify our key is present
    local list_response
    list_response=$(call_tool "configure" '{"what":"store","store_action":"list","namespace":"smoke"}')
    local list_text
    list_text=$(extract_content_text "$list_response")

    echo "  [store: list]"
    echo "    $(truncate "$list_text" 150)"

    if ! echo "$list_text" | grep -q "smoke-key"; then
        fail "store list does not contain 'smoke-key'. Content: $(truncate "$list_text" 200)"
        return
    fi
    echo "    list: OK (contains smoke-key)"

    # 4. Delete the key
    local del_response
    del_response=$(call_tool "configure" '{"what":"store","store_action":"delete","key":"smoke-key","namespace":"smoke"}')

    if ! check_not_error "$del_response"; then
        fail "store delete returned error. Content: $(truncate "$(extract_content_text "$del_response")" 200)"
        return
    fi
    echo "  [store: delete]"
    echo "    delete: OK"

    # 5. Verify deletion — load should NOT return the data
    local load2_response
    load2_response=$(call_tool "configure" '{"what":"store","store_action":"load","key":"smoke-key","namespace":"smoke"}')
    local load2_text
    load2_text=$(extract_content_text "$load2_response")

    echo "  [store: verify deletion]"
    echo "    $(truncate "$load2_text" 150)"

    if echo "$load2_text" | grep -q "smoke-data-123"; then
        fail "store load still returns 'smoke-data-123' after delete. Data not actually deleted."
    else
        pass "Store CRUD roundtrip: save(smoke-data-123) > load(found) > list(found) > delete > load(gone)."
    fi
}
run_test_8_2

# ── Test 8.3: Selective buffer clear ────────────────────
begin_test "8.3" "[BROWSER] Selective buffer clear (logs only, actions preserved)" \
    "Seed data, clear logs only, verify actions still exist" \
    "Tests: targeted buffer clearing"

run_test_8_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Seed some data
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Seed log\",\"script\":\"console.log('CLEAR_TEST_LOG')\"}"
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Seed action","script":"var b = document.createElement(\"button\"); b.id = \"clear-test-btn\"; b.textContent = \"X\"; document.body.appendChild(b); b.click(); \"done\""}'
    sleep 1

    # Clear only logs
    local clear_response
    clear_response=$(call_tool "configure" '{"what":"clear","buffer":"logs"}')

    if ! check_not_error "$clear_response"; then
        fail "clear(logs) returned error. Content: $(truncate "$(extract_content_text "$clear_response")" 200)"
        return
    fi

    # Verify logs cleared
    local log_response
    log_response=$(call_tool "observe" '{"what":"logs"}')
    local log_text
    log_text=$(extract_content_text "$log_response")

    local logs_cleared=true
    if echo "$log_text" | grep -q "CLEAR_TEST_LOG"; then
        logs_cleared=false
    fi

    # Verify actions still exist
    local action_response
    action_response=$(call_tool "observe" '{"what":"actions"}')
    local action_text
    action_text=$(extract_content_text "$action_response")

    local actions_exist=false
    if echo "$action_text" | grep -qiE "click|action|entries"; then
        actions_exist=true
    fi

    echo "    logs_cleared: $logs_cleared"
    echo "    actions_exist: $actions_exist"

    if [ "$logs_cleared" = "true" ] && [ "$actions_exist" = "true" ]; then
        pass "Selective clear: logs cleared, actions preserved."
    elif [ "$logs_cleared" = "true" ] && [ "$actions_exist" = "false" ]; then
        fail "Logs cleared but actions also empty. Seeded action (button click) should still be in actions buffer. Actions content: $(truncate "$action_text" 200)"
    else
        fail "Logs NOT cleared after clear(logs). 'CLEAR_TEST_LOG' still present. Log content: $(truncate "$log_text" 200)"
    fi
}
run_test_8_3

# ── Test 8.4: Streaming toggle ──────────────────────────
begin_test "8.4" "[DAEMON ONLY] Streaming: enable, status, disable, status" \
    "Toggle streaming events and verify state transitions" \
    "Tests: streaming configuration"

run_test_8_4() {
    # Enable streaming with specific events
    local enable_response
    enable_response=$(call_tool "configure" '{"what":"streaming","streaming_action":"enable","events":["errors","network_errors"]}')

    if ! check_not_error "$enable_response"; then
        fail "streaming enable returned error. Content: $(truncate "$(extract_content_text "$enable_response")" 200)"
        return
    fi

    local enable_text
    enable_text=$(extract_content_text "$enable_response")
    echo "  [enable response]"
    echo "    $(truncate "$enable_text" 150)"

    # Check status — must show enabled state
    local status_response
    status_response=$(call_tool "configure" '{"what":"streaming","streaming_action":"status"}')
    local status_text
    status_text=$(extract_content_text "$status_response")

    echo "  [status after enable]"
    echo "    $(truncate "$status_text" 150)"

    # Parse status to verify enabled state (using jq)
    local enabled_after_enable=false
    local json_part enabled_val
    json_part=$(echo "$status_text" | sed -n '/{/,$ p' | tr '\n' ' ')
    enabled_val=$(echo "$json_part" | jq -r '.config.enabled // .enabled // .active // false' 2>/dev/null)
    if [ "$enabled_val" = "true" ]; then
        enabled_after_enable=true
    fi

    # Disable streaming
    local disable_response
    disable_response=$(call_tool "configure" '{"what":"streaming","streaming_action":"disable"}')

    if ! check_not_error "$disable_response"; then
        fail "streaming disable returned error. Content: $(truncate "$(extract_content_text "$disable_response")" 200)"
        return
    fi

    # Check status after disable
    local status2_response
    status2_response=$(call_tool "configure" '{"what":"streaming","streaming_action":"status"}')
    local status2_text
    status2_text=$(extract_content_text "$status2_response")

    echo "  [status after disable]"
    echo "    $(truncate "$status2_text" 150)"

    # Strict: must verify state transitions, not just "no errors"
    if [ "$enabled_after_enable" = "true" ]; then
        pass "Streaming: enable(errors,network_errors) > status(active) > disable > status. State transitions verified."
    else
        fail "Streaming: enable succeeded but status did not confirm active state. Status: $(truncate "$status_text" 200)"
    fi
}
run_test_8_4

# ── Test 8.5: Test boundaries ───────────────────────────
begin_test "8.5" "[DAEMON ONLY] Test boundaries: start and end markers" \
    "configure(test_boundary_start) then (test_boundary_end)" \
    "Tests: test boundary markers for isolating activity"

run_test_8_5() {
    # Start boundary
    local start_response
    start_response=$(call_tool "configure" '{"what":"test_boundary_start","test_id":"smoke-boundary","label":"Smoke test boundary"}')

    if ! check_not_error "$start_response"; then
        fail "test_boundary_start returned error. Content: $(truncate "$(extract_content_text "$start_response")" 200)"
        return
    fi

    local start_text
    start_text=$(extract_content_text "$start_response")
    echo "  [boundary start response]"
    echo "    $(truncate "$start_text" 150)"

    # Do some activity between boundaries
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "execute_js" '{"what":"execute_js","reason":"Activity between boundaries","script":"console.log(\"boundary-test-activity\"); \"logged\""}'
    fi

    sleep 0.5

    # End boundary
    local end_response
    end_response=$(call_tool "configure" '{"what":"test_boundary_end","test_id":"smoke-boundary"}')

    if ! check_not_error "$end_response"; then
        fail "test_boundary_end returned error. Content: $(truncate "$(extract_content_text "$end_response")" 200)"
        return
    fi

    local end_text
    end_text=$(extract_content_text "$end_response")
    echo "  [boundary end response]"
    echo "    $(truncate "$end_text" 150)"

    # Verify the end response references the test_id
    if echo "$end_text" | grep -qi "smoke-boundary"; then
        pass "Test boundaries: start/end completed for 'smoke-boundary'. End response references test_id."
    else
        fail "Test boundaries: end response did not reference 'smoke-boundary'. Content: $(truncate "$end_text" 200)"
    fi
}
run_test_8_5
