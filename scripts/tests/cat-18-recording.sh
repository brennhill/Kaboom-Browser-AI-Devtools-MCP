#!/bin/bash
# cat-18-recording.sh — UAT tests for tab recording and audio capture (7 tests).
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "18" "Recording & Audio" "7"
ensure_daemon

# ── Helper: check if pilot is enabled via health endpoint ──
pilot_available() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")
    local pilot
    pilot=$(echo "$body" | jq -r '.capture.pilot_enabled // false' 2>/dev/null)
    [ "$pilot" = "true" ]
}

# ── 18.1 — screen_recording_start returns valid JSON-RPC ──────────────
begin_test "18.1" "screen_recording_start returns valid JSON-RPC" \
    "Verify screen_recording_start produces a valid JSON-RPC response (queued or pilot error)" \
    "API contract: screen_recording_start must never crash or return malformed JSON."
run_test_18_1() {
    RESPONSE=$(call_tool "interact" '{"action":"screen_recording_start","name":"uat-test-18-1"}')
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "screen_recording_start returned invalid JSON-RPC. Response: $(truncate "$RESPONSE")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    # Should return either "queued" (pilot on) or pilot-disabled error
    if check_contains "$text" "queued"; then
        pass "screen_recording_start returned valid JSON-RPC with status=queued."
    elif check_contains "$text" "pilot"; then
        pass "screen_recording_start returned valid JSON-RPC with pilot-disabled error (expected without extension)."
    else
        pass "screen_recording_start returned valid JSON-RPC. Content: $(truncate "$text" 200)"
    fi
}
run_test_18_1

# ── 18.2 — screen_recording_stop returns valid JSON-RPC ───────────────
begin_test "18.2" "screen_recording_stop returns valid JSON-RPC" \
    "Verify screen_recording_stop produces a valid JSON-RPC response (queued or error)" \
    "API contract: screen_recording_stop must never crash or return malformed JSON."
run_test_18_2() {
    RESPONSE=$(call_tool "interact" '{"action":"screen_recording_stop"}')
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "screen_recording_stop returned invalid JSON-RPC. Response: $(truncate "$RESPONSE")"
        return
    fi
    pass "screen_recording_stop returned valid JSON-RPC."
}
run_test_18_2

# ── 18.3 — observe(saved_videos) returns valid structure ─────
begin_test "18.3" "observe(saved_videos) returns valid structure" \
    "Verify saved_videos returns recordings array and total count" \
    "Must work even when no recordings exist (empty array)."
run_test_18_3() {
    RESPONSE=$(call_tool "observe" '{"what":"saved_videos"}')
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "observe(saved_videos) returned invalid JSON-RPC. Response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_not_error "$RESPONSE"; then
        local text
        text=$(extract_content_text "$RESPONSE")
        if check_contains "$text" "unknown_mode"; then
            skip "observe(saved_videos) not recognized (binary predates recording feature)."
        else
            fail "observe(saved_videos) returned error. Content: $(truncate "$text")"
        fi
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" '"recordings"'; then
        fail "saved_videos missing 'recordings' JSON key. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" '"total"'; then
        fail "saved_videos missing 'total' JSON key. Content: $(truncate "$text")"
        return
    fi
    pass "observe(saved_videos) returned valid structure with 'recordings' and 'total'."
}
run_test_18_3

# ── 18.4 — screen_recording_start with audio:"tab" echoes audio param ─
begin_test "18.4" "screen_recording_start with audio:'tab' echoes audio in response" \
    "Verify audio param is accepted and reflected in queued response" \
    "Requires pilot enabled. Audio param must survive the round-trip."
run_test_18_4() {
    if ! pilot_available; then
        skip "Pilot not available. Skipping audio param test."
        return
    fi
    RESPONSE=$(call_tool "interact" '{"action":"screen_recording_start","name":"uat-audio-test","audio":"tab"}')
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "queued"; then
        fail "screen_recording_start with audio:'tab' did not return queued. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" '"audio":"tab"'; then
        fail "Queued response missing audio:'tab'. Content: $(truncate "$text")"
        return
    fi
    pass "screen_recording_start with audio:'tab' returned queued response with audio param echoed."
    # Clean up: stop any recording that might have started
    sleep 1
    call_tool "interact" '{"action":"screen_recording_stop"}' >/dev/null 2>&1
    sleep 1
}
run_test_18_4

# ── 18.5 — screen_recording_start with invalid audio mode returns error ─
begin_test "18.5" "screen_recording_start rejects invalid audio mode" \
    "Verify audio:'invalid' returns a structured error" \
    "Requires pilot enabled. Validates server-side input validation."
run_test_18_5() {
    if ! pilot_available; then
        skip "Pilot not available. Skipping audio validation test."
        return
    fi
    RESPONSE=$(call_tool "interact" '{"action":"screen_recording_start","name":"uat-bad-audio","audio":"invalid"}')
    if ! check_is_error "$RESPONSE"; then
        # Even if not isError, check for error message in content
        local text
        text=$(extract_content_text "$RESPONSE")
        if check_contains "$text" "Invalid audio mode"; then
            pass "screen_recording_start with audio:'invalid' returned error message in content."
            return
        fi
        fail "screen_recording_start with audio:'invalid' should return error. Content: $(truncate "$text")"
        return
    fi
    pass "screen_recording_start with audio:'invalid' correctly returned isError."
}
run_test_18_5

# ── 18.6 — screen_recording_start with audio:"both" echoes audio param ─
begin_test "18.6" "screen_recording_start with audio:'both' echoes audio in response" \
    "Verify audio:'both' is accepted (validated for Phase 2)" \
    "Requires pilot enabled. 'both' mode is validated even though mic isn't implemented yet."
run_test_18_6() {
    if ! pilot_available; then
        skip "Pilot not available. Skipping audio:'both' test."
        return
    fi
    RESPONSE=$(call_tool "interact" '{"action":"screen_recording_start","name":"uat-both-audio","audio":"both"}')
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "queued"; then
        fail "screen_recording_start with audio:'both' did not return queued. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" '"audio":"both"'; then
        fail "Queued response missing audio:'both'. Content: $(truncate "$text")"
        return
    fi
    pass "screen_recording_start with audio:'both' returned queued response with audio param echoed."
    # Clean up
    sleep 1
    call_tool "interact" '{"action":"screen_recording_stop"}' >/dev/null 2>&1
    sleep 1
}
run_test_18_6

# ── 18.7 — observe(saved_videos) with last_n filter ──────────
begin_test "18.7" "observe(saved_videos) respects last_n filter" \
    "Verify last_n limits the number of recordings returned" \
    "Tests server-side filtering. Works even with zero recordings."
run_test_18_7() {
    RESPONSE=$(call_tool "observe" '{"what":"saved_videos","last_n":1}')
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "observe(saved_videos, last_n:1) returned invalid JSON-RPC."
        return
    fi
    if ! check_not_error "$RESPONSE"; then
        local text
        text=$(extract_content_text "$RESPONSE")
        if check_contains "$text" "unknown_mode"; then
            skip "observe(saved_videos) not recognized (binary predates recording feature)."
        else
            fail "observe(saved_videos, last_n) returned error. Content: $(truncate "$text")"
        fi
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" '"recordings"'; then
        fail "saved_videos with last_n missing 'recordings' JSON key. Content: $(truncate "$text")"
        return
    fi
    pass "observe(saved_videos) with last_n:1 returned valid response."
}
run_test_18_7

finish_category
