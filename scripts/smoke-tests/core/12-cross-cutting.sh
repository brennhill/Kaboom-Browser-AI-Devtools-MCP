#!/bin/bash
# 12-cross-cutting.sh — 12.1-12.3: Pagination, error recovery, buffer overflow.
set -eo pipefail

begin_category "12" "Cross-Cutting Concerns" "3"

# ── Test 12.1: Pagination via cursors ────────────────────
begin_test "12.1" "[BROWSER] Pagination: observe(logs) with limit and cursor" \
    "Fetch logs with limit:3, then use cursor to get next page, verify no overlap" \
    "Tests: cursor-based pagination across observe modes"

run_test_12_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Seed enough log entries to paginate
    for i in $(seq 1 8); do
        interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Seed log $i\",\"script\":\"console.log('PAGINATION_TEST_$i')\"}"
    done
    sleep 2

    # Page 1: get first 3
    local page1_response
    page1_response=$(call_tool "observe" '{"what":"logs","limit":3}')
    local page1_text
    page1_text=$(extract_content_text "$page1_response")

    # Extract cursor from response
    local cursor
    cursor=$(content_json_cursor "$page1_text")

    echo "  [page 1]"
    echo "    entries: $(content_json_entry_count "$page1_text")"

    if [ -z "$cursor" ]; then
        # We seeded 8 entries with limit 3 — there SHOULD be a cursor
        fail "Pagination: no cursor returned after seeding 8 log entries with limit=3. Pagination may be broken. Content: $(truncate "$page1_text" 200)"
        return
    fi

    # Page 2: get next 3 using cursor
    local page2_response
    page2_response=$(call_tool "observe" "{\"what\":\"logs\",\"limit\":3,\"after_cursor\":\"$cursor\"}")
    local page2_text
    page2_text=$(extract_content_text "$page2_response")

    echo "  [page 2]"
    echo "    entries: $(content_json_entry_count "$page2_text")"

    # Verify pages are different (simple check: page2 text differs from page1)
    if [ "$page1_text" != "$page2_text" ] && [ -n "$page2_text" ]; then
        pass "Pagination: page 1 and page 2 returned different entries via cursor."
    else
        fail "Pagination: pages appear identical or page 2 empty. Page 1: $(truncate "$page1_text" 150), Page 2: $(truncate "$page2_text" 150)"
    fi
}
run_test_12_1

# ── Test 12.2: Error recovery ────────────────────────────
begin_test "12.2" "[DAEMON ONLY] Error recovery: 4 invalid calls, daemon still healthy" \
    "Send 4 different malformed/invalid tool calls, then verify daemon health" \
    "Tests: daemon resilience to bad input"

run_test_12_2() {
    # Invalid tool name
    local r1
    r1=$(call_tool "nonexistent_tool" '{}')
    log_diagnostic "12.2" "invalid tool" "$r1"

    # Missing required param
    local r2
    r2=$(call_tool "observe" '{}')
    log_diagnostic "12.2" "missing param" "$r2"

    # Invalid enum value
    local r3
    r3=$(call_tool "observe" '{"what":"invalid_mode_xyz"}')
    log_diagnostic "12.2" "invalid enum" "$r3"

    # Malformed JSON in arguments (framework may catch this)
    local r4
    r4=$(call_tool "interact" '{"what":""}')
    log_diagnostic "12.2" "empty action" "$r4"

    # Verify daemon is still healthy
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")
    local status_val
    status_val=$(echo "$body" | jq -r '.status // empty' 2>/dev/null)

    local all_structured=true
    for resp in "$r1" "$r2" "$r3" "$r4"; do
        if [ -z "$resp" ]; then
            all_structured=false
            break
        fi
        if ! echo "$resp" | jq -e '.jsonrpc == "2.0"' >/dev/null 2>&1; then
            all_structured=false
            break
        fi
    done

    if [ "$status_val" = "ok" ] && [ "$all_structured" = "true" ]; then
        pass "Error recovery: 4 invalid calls all returned structured JSON-RPC, daemon still healthy."
    elif [ "$status_val" = "ok" ]; then
        pass "Error recovery: daemon still healthy after 4 invalid calls (some responses not structured)."
    else
        fail "Daemon unhealthy after invalid calls. Health status: $status_val."
    fi
}
run_test_12_2

# ── Test 12.3: Buffer overflow / eviction ────────────────
begin_test "12.3" "[BROWSER] Buffer overflow: inject 1000+ logs, verify eviction" \
    "Inject 1000+ log entries via a single execute_js loop, verify buffer is capped" \
    "Tests: daemon buffer eviction under load"

run_test_12_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Inject 1000 log entries in a single JS call
    interact_and_wait "execute_js" '{"what":"execute_js","reason":"Flood log buffer","script":"for(var i=0;i<1000;i++){console.log(\"FLOOD_TEST_\"+i)} \"flooded\""}'
    sleep 3

    local response
    response=$(call_tool "observe" '{"what":"logs"}')
    local content_text
    content_text=$(extract_content_text "$response")

    local count
    count=$(content_json_count "$content_text")

    echo "  [buffer after flood]"
    echo "    log count: $count"

    # Verify daemon still healthy
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")
    local status_val
    status_val=$(echo "$body" | jq -r '.status // empty' 2>/dev/null)

    if [ "$status_val" != "ok" ]; then
        fail "Daemon unhealthy after buffer flood. Health status: $status_val."
    elif [ "$count" -gt 0 ] 2>/dev/null && [ "$count" -lt 1000 ] 2>/dev/null; then
        pass "Buffer eviction working: injected 1000 logs, buffer capped at $count entries, daemon healthy."
    elif [ "$count" -gt 0 ] 2>/dev/null; then
        fail "Buffer NOT evicting: injected 1000 logs and buffer has $count entries (should be < 1000). Eviction policy may be broken."
    else
        fail "Buffer empty after flood: injected 1000 logs but count=$count. Log pipeline may be broken."
    fi
}
run_test_12_3
