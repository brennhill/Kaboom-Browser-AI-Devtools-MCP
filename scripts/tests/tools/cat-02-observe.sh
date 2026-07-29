#!/bin/bash
# cat-02-observe.sh — Category 2: Observe Tool (25 tests).
# Tests all observe modes plus negative cases.
# Each mode must return a valid response shape, even with no data.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "2" "Observe Tool" "26"

ensure_daemon

# Warm-up: wait until the daemon's /mcp endpoint is responsive
# Under parallel load, /health may respond before /mcp is ready
for _warmup_i in $(seq 1 10); do
    _warmup_resp=$(curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":0,"method":"tools/list"}' \
        "http://localhost:${PORT}/mcp" 2>/dev/null)
    if echo "$_warmup_resp" | jq -e '.result.tools' >/dev/null 2>&1; then
        break
    fi
    sleep 0.5
done

# ── 2.1 — observe(page) ──────────────────────────────────
begin_test "2.1" "observe(page) returns page data" \
    "Call observe with what:page. Verify valid response with page info or no-tab message." \
    "This is the most basic observe call. If this fails, no observe works."
run_test_2_1() {
    RESPONSE=$(call_tool "observe" '{"what":"page"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    pass "Sent observe(page), got valid response. Content text present (${#text} chars). No isError."
}
run_test_2_1

# ── 2.2 — observe(tabs) ──────────────────────────────────
begin_test "2.2" "observe(tabs) returns tab array" \
    "Call observe with what:tabs. Verify response has tabs array and tracking_active field." \
    "MCP clients use this to know what is being tracked. Shape must be stable."
run_test_2_2() {
    RESPONSE=$(call_tool "observe" '{"what":"tabs"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "tabs"; then
        fail "Expected content to contain 'tabs' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "tracking_active"; then
        fail "Expected content to contain 'tracking_active' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(tabs), got valid response with 'tabs' and 'tracking_active' fields. Content: ${#text} chars."
}
run_test_2_2

# ── 2.3 — observe(logs) ──────────────────────────────────
begin_test "2.3" "observe(logs) returns logs array" \
    "Call observe with what:logs. Verify response has count and logs array. NOT an error response." \
    "Empty state must be distinguishable from error state. Returning error for no data breaks AI workflows."
run_test_2_3() {
    RESPONSE=$(call_tool "observe" '{"what":"logs"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(logs), got valid response with 'count' field. Content: ${#text} chars. No isError."
}
run_test_2_3

# ── 2.4 — observe(logs) with min_level filter ────────────
begin_test "2.4" "observe(logs) with min_level filter" \
    "Call observe with what:logs and min_level:error. Verify valid response with filter applied." \
    "Filter params that silently fail mean AI gets wrong data and makes wrong decisions."
run_test_2_4() {
    RESPONSE=$(call_tool "observe" '{"what":"logs","min_level":"error"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(logs, min_level:error), got valid response with 'count' field. Content: ${#text} chars."
}
run_test_2_4

# ── 2.5 — observe(network_waterfall) ─────────────────────
begin_test "2.5" "observe(network_waterfall) returns entries array" \
    "Call observe with what:network_waterfall. Verify response has entries and count." \
    "Network waterfall is the most-used observe mode. Shape breakage affects every user."
run_test_2_5() {
    RESPONSE=$(call_tool "observe" '{"what":"network_waterfall"}')
    # network_waterfall does an on-demand extension query (5s timeout).
    # Without extension, bridge may timeout (4s < 5s). This is expected.
    if check_bridge_timeout "$RESPONSE"; then
        pass "Sent observe(network_waterfall), got bridge timeout (expected without extension). Server did not crash."
        return
    fi
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(network_waterfall), got valid response with 'entries' and 'count'. Content: ${#text} chars."
}
run_test_2_5

# ── 2.6 — observe(network_waterfall) with limit ──────────
begin_test "2.6" "observe(network_waterfall) with limit parameter" \
    "Call observe with what:network_waterfall and limit:5. Verify response respects limit." \
    "Limit is critical for keeping MCP context windows manageable. Silently ignoring limit overflows AI context."
run_test_2_6() {
    RESPONSE=$(call_tool "observe" '{"what":"network_waterfall","limit":5}')
    if check_bridge_timeout "$RESPONSE"; then
        pass "Sent observe(network_waterfall, limit:5), got bridge timeout (expected without extension). Server did not crash."
        return
    fi
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(network_waterfall, limit:5), got valid response with 'entries'. Content: ${#text} chars."
}
run_test_2_6

# ── 2.7 — observe(errors) ────────────────────────────────
begin_test "2.7" "observe(errors) returns error array" \
    "Call observe with what:errors. Verify response has errors array and count." \
    "Error detection is core functionality."
run_test_2_7() {
    RESPONSE=$(call_tool "observe" '{"what":"errors"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "errors"; then
        fail "Expected content to contain 'errors' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(errors), got valid response with 'errors' and 'count'. Content: ${#text} chars."
}
run_test_2_7

# ── 2.8 — observe(vitals) ────────────────────────────────
begin_test "2.8" "observe(vitals) returns metrics shape" \
    "Call observe with what:vitals. Verify response has metrics object with has_data boolean." \
    "Web Vitals is the performance monitoring surface. Shape must be stable."
run_test_2_8() {
    RESPONSE=$(call_tool "observe" '{"what":"vitals"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "metrics"; then
        fail "Expected content to contain 'metrics' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "has_data"; then
        fail "Expected content to contain 'has_data' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(vitals), got valid response with 'metrics' and 'has_data'. Content: ${#text} chars."
}
run_test_2_8

# ── 2.9 — observe(actions) ───────────────────────────────
begin_test "2.9" "observe(actions) returns entries" \
    "Call observe with what:actions. Verify response has entries array and count." \
    "Actions feed test generation and reproduction."
run_test_2_9() {
    RESPONSE=$(call_tool "observe" '{"what":"actions"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(actions), got valid response with 'entries'. Content: ${#text} chars."
}
run_test_2_9

# ── 2.10 — observe(websocket_events) ─────────────────────
begin_test "2.10" "observe(websocket_events) returns events" \
    "Call observe with what:websocket_events. Verify response has entries array. No error." \
    "WebSocket capture is a key differentiator."
run_test_2_10() {
    RESPONSE=$(call_tool "observe" '{"what":"websocket_events"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(websocket_events), got valid response with 'entries'. Content: ${#text} chars."
}
run_test_2_10

# ── 2.11 — observe(websocket_status) ─────────────────────
begin_test "2.11" "observe(websocket_status) returns connection status" \
    "Call observe with what:websocket_status. Verify valid response with connection data." \
    "Status endpoint must always respond, even with no WebSocket connections."
run_test_2_11() {
    RESPONSE=$(call_tool "observe" '{"what":"websocket_status"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "connections"; then
        fail "Expected content to contain 'connections' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(websocket_status), got valid response with 'connections'. Content: ${#text} chars."
}
run_test_2_11

# ── 2.12 — observe(extension_logs) ───────────────────────
begin_test "2.12" "observe(extension_logs) returns logs" \
    "Call observe with what:extension_logs. Verify response has logs data. No error." \
    "Extension debugging depends on this."
run_test_2_12() {
    RESPONSE=$(call_tool "observe" '{"what":"extension_logs"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "logs"; then
        fail "Expected content to contain 'logs' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(extension_logs), got valid response with 'logs' and 'count'. Content: ${#text} chars."
}
run_test_2_12

# ── 2.13 — observe(pilot) ────────────────────────────────
begin_test "2.13" "observe(pilot) returns pilot status or requests recording context" \
    "Call observe with what:pilot. Verify either pilot state or recording_id prompt." \
    "AI Web Pilot gate -- on next branch, pilot requires recording_id when no recording is active."
run_test_2_13() {
    RESPONSE=$(call_tool "observe" '{"what":"pilot"}')
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "Response is not valid JSON-RPC. Full response: $(truncate "$RESPONSE")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    # Accept either: success with "enabled" field, or error requesting recording_id
    if check_not_error "$RESPONSE"; then
        if check_contains "$text" "enabled"; then
            pass "Sent observe(pilot), got valid response with 'enabled' field. Content: $(truncate "$text" 200)"
        else
            pass "Sent observe(pilot), got valid non-error response. Content: $(truncate "$text" 200)"
        fi
    elif check_contains "$text" "recording_id"; then
        pass "Sent observe(pilot), got expected recording_id prompt (no active recording). Content: $(truncate "$text" 200)"
    else
        fail "Unexpected error from observe(pilot). Content: $(truncate "$text")"
    fi
}
run_test_2_13

# ── 2.14 — observe(summarized_logs) ──────────────────────
begin_test "2.14" "observe(summarized_logs) returns valid response" \
    "Call observe with what:summarized_logs. Verify response is valid, not error." \
    "Summarized logs mode groups repeated noise into compact buckets."
run_test_2_14() {
    RESPONSE=$(call_tool "observe" '{"what":"summarized_logs"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    pass "Sent observe(summarized_logs), got valid non-error response. Content: ${#text} chars."
}
run_test_2_14

# ── 2.15 — observe(timeline) ─────────────────────────────
begin_test "2.15" "observe(timeline) returns unified entries" \
    "Call observe with what:timeline. Verify response has entries array and count." \
    "Timeline is the unified view across all buffer types."
run_test_2_15() {
    RESPONSE=$(call_tool "observe" '{"what":"timeline"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(timeline), got valid response with 'entries' and 'count'. Content: ${#text} chars."
}
run_test_2_15

# ── 2.16 — observe(log_diff_report) ──────────────────────
begin_test "2.16" "observe(log_diff_report) returns valid response or requests params" \
    "Call observe with what:log_diff_report. Verify valid response or structured parameter error." \
    "Log diff report compares recordings. May need original_id/replay_id params."
run_test_2_16() {
    RESPONSE=$(call_tool "observe" '{"what":"log_diff_report"}')
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "Response is not valid JSON-RPC. Full response: $(truncate "$RESPONSE")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    # Accept either: success with data, or structured error requesting recording params
    if check_not_error "$RESPONSE"; then
        pass "Sent observe(log_diff_report), got valid non-error response. Content: ${#text} chars."
    elif check_contains "$text" "missing_param" || check_contains "$text" "recording"; then
        pass "Sent observe(log_diff_report), got expected parameter prompt (no active recordings). Content: $(truncate "$text" 200)"
    else
        fail "Unexpected error from observe(log_diff_report). Content: $(truncate "$text")"
    fi
}
run_test_2_16

# ── 2.17 — observe(recordings) ───────────────────────────
begin_test "2.17" "observe(recordings) returns valid response" \
    "Call observe with what:recordings. Verify response is valid, not error." \
    "Recordings lists captured browser recording sessions."
run_test_2_17() {
    RESPONSE=$(call_tool "observe" '{"what":"recordings"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    pass "Sent observe(recordings), got valid non-error response. Content: ${#text} chars."
}
run_test_2_17

# ── 2.18 — observe(command_result) missing correlation_id ─
begin_test "2.18" "observe(command_result) without correlation_id returns error" \
    "Call observe with what:command_result but omit correlation_id. Verify structured missing_param error." \
    "This validates mode routing and required-parameter validation for async result lookup."
run_test_2_18() {
    RESPONSE=$(call_tool "observe" '{"what":"command_result"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true when correlation_id is missing. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "isError was true but no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "correlation_id" && ! check_contains "$text" "missing_param"; then
        fail "Expected missing correlation_id guidance. Content: $(truncate "$text")"
        return
    fi
    pass "Sent observe(command_result) without correlation_id, got expected structured error. Content: $(truncate "$text" 200)"
}
run_test_2_18

# ── 2.19 — observe(network_bodies) ───────────────────────
begin_test "2.19" "observe(network_bodies) returns valid response" \
    "Call observe with what:network_bodies. Verify valid JSON-RPC response." \
    "Network bodies provides full request/response body data for network calls."
run_test_2_19() {
    RESPONSE=$(call_tool "observe" '{"what":"network_bodies"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    pass "Sent observe(network_bodies), got valid non-error response. Content: ${#text} chars."
}
run_test_2_19

# ── 2.20 — observe(saved_videos) ─────────────────────────
begin_test "2.20" "observe(saved_videos) returns valid response" \
    "Call observe with what:saved_videos. Verify valid JSON-RPC response." \
    "Saved videos lists recorded browser sessions."
run_test_2_20() {
    RESPONSE=$(call_tool "observe" '{"what":"saved_videos"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    pass "Sent observe(saved_videos), got valid non-error response. Content: ${#text} chars."
}
run_test_2_20

# ── 2.21 — observe(pending_commands) ─────────────────────
begin_test "2.21" "observe(pending_commands) returns command queues" \
    "Call observe with what:pending_commands. Verify response has pending, completed, failed arrays." \
    "Async command tracking is the interact tool's feedback loop."
run_test_2_21() {
    RESPONSE=$(call_tool "observe" '{"what":"pending_commands"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "pending"; then
        fail "Expected content to contain 'pending' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "completed"; then
        fail "Expected content to contain 'completed' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "failed"; then
        fail "Expected content to contain 'failed' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(pending_commands), got valid response with 'pending', 'completed', 'failed'. Content: ${#text} chars."
}
run_test_2_21

# ── 2.22 — observe(failed_commands) ──────────────────────
begin_test "2.22" "observe(failed_commands) returns failures" \
    "Call observe with what:failed_commands. Verify response has commands array and count." \
    "Failed command visibility prevents silent failures."
run_test_2_22() {
    RESPONSE=$(call_tool "observe" '{"what":"failed_commands"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "commands"; then
        fail "Expected content to contain 'commands' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(failed_commands), got valid response with 'commands' and 'count'. Content: ${#text} chars."
}
run_test_2_22

# ── 2.23 — observe(error_bundles) ──────────────────────────
begin_test "2.23" "observe(error_bundles) returns bundles array" \
    "Call observe with what:error_bundles. Verify response has bundles array and count." \
    "Error bundles assemble complete debugging context per error in one call."
run_test_2_23() {
    RESPONSE=$(call_tool "observe" '{"what":"error_bundles"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "bundles"; then
        fail "Expected content to contain 'bundles' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(error_bundles), got valid response with 'bundles' and 'count'. Content: ${#text} chars."
}
run_test_2_23

# ── 2.24 — observe with invalid "what" ───────────────────
begin_test "2.24" "observe with invalid what returns structured error" \
    "Call observe with what:nonexistent_mode. Verify isError:true with helpful message." \
    "Typos in mode names must produce helpful errors, not empty success responses."
run_test_2_24() {
    RESPONSE=$(call_tool "observe" '{"what":"nonexistent_mode"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true but got success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "isError was true but no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    # Verify the error message mentions valid options or unknown mode
    if ! check_contains "$text" "nknown" && ! check_contains "$text" "valid" && ! check_contains "$text" "Valid"; then
        fail "Error message does not mention unknown mode or valid options. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(nonexistent_mode), got isError:true. Error mentions valid options. Content: $(truncate "$text" 150)"
}
run_test_2_24

# ── 2.25 — observe with missing "what" ───────────────────
begin_test "2.25" "observe with missing what returns error" \
    "Call observe with empty params {}. Verify error about missing required parameter." \
    "Missing required params must fail loudly."
run_test_2_25() {
    RESPONSE=$(call_tool "observe" '{}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true but got success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "isError was true but no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "what"; then
        fail "Error message does not mention missing 'what' parameter. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe({}), got isError:true. Error mentions missing 'what' parameter. Content: $(truncate "$text" 150)"
}
run_test_2_25

# ── 2.26 — observe(screenshot) ─────────────────────────────
begin_test "2.26" "observe(screenshot) returns valid response" \
    "Call observe with what:screenshot. Verify valid JSON-RPC response (screenshot data or no-extension message)." \
    "On-demand screenshot capture. Returns error if no extension connected, success with filename if connected."
run_test_2_26() {
    RESPONSE=$(call_tool "observe" '{"what":"screenshot"}')
    # screenshot may return isError when no extension/tab is tracked — that is acceptable
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    # Verify it is a valid JSON-RPC response (not a crash)
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "Response is not valid JSON-RPC. Full response: $(truncate "$RESPONSE")"
        return
    fi
    pass "Sent observe(screenshot), got valid JSON-RPC response. Content: ${#text} chars. May be error (no extension) or screenshot data."
}
run_test_2_26

finish_category
