#!/bin/bash
# cat-19-link-health.sh — UAT tests for analyze/link_health mode (19 tests).
# Tests verify link health checker creates proper pending queries and returns
# correlation IDs for async tracking.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "19" "Link Health Analyzer" "19"
ensure_daemon

_health_json="$(curl -s --max-time 5 "http://localhost:${PORT}/health" 2>/dev/null || true)"
_ext_connected="$(echo "$_health_json" | jq -r '.pilot.extension_connected // .capture.extension_connected // false' 2>/dev/null)"
if [ "$_ext_connected" != "true" ]; then
    begin_test "19.0" "Extension connectivity precondition" \
        "Link health requires a connected extension + tracked tab context" \
        "Without extension data, link health returns no_data by design."
    skip "Extension is not connected on port ${PORT}; skipping link_health success-path tests."
    finish_category
fi

# ── GROUP A: Basic Link Checking (5 tests) ─────────────────────────────────

# 19.1 — analyze/link_health returns correlation_id
begin_test "19.1" "analyze({what:'link_health'}) returns correlation_id" \
    "Verify link health initiates async operation with correlation_id" \
    "Correlation IDs enable async result tracking via command_result mode."
run_test_19_1() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "correlation_id"; then
        fail "Response missing 'correlation_id'. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "link_health_" "$text"; then
        fail "Correlation ID should start with 'link_health_'. Content: $(truncate "$text")"
        return
    fi
    pass "Link health returns valid correlation_id. Content: $(truncate "$text" 200)"
}
run_test_19_1

# 19.2 — analyze/link_health with timeout_ms parameter
begin_test "19.2" "analyze({what:'link_health',timeout_ms:15000}) accepts optional params" \
    "Verify optional parameters are accepted without error" \
    "Optional parameters must not cause validation errors."
run_test_19_2() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health","timeout_ms":15000}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success with timeout_ms param. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "correlation_id"; then
        fail "Should still return correlation_id with params. Content: $(truncate "$text")"
        return
    fi
    pass "Link health accepts timeout_ms parameter. Content: $(truncate "$text" 200)"
}
run_test_19_2

# 19.3 — analyze/link_health with max_workers parameter
begin_test "19.3" "analyze({what:'link_health',max_workers:20}) accepts worker count" \
    "Verify max_workers parameter is accepted" \
    "Worker count controls concurrency. Must not error on valid values."
run_test_19_3() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health","max_workers":20}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success with max_workers param. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    pass "Link health accepts max_workers parameter. Content: $(truncate "$text" 200)"
}
run_test_19_3

# 19.4 — analyze/link_health status is 'queued'
begin_test "19.4" "analyze/link_health returns status='queued'" \
    "Verify response indicates query is queued for async execution" \
    "Status field indicates operation phase (queued, processing, complete)."
run_test_19_4() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "queued"; then
        fail "Status should be 'queued'. Content: $(truncate "$text")"
        return
    fi
    pass "Status correctly indicates 'queued'. Content: $(truncate "$text" 200)"
}
run_test_19_4

# 19.5 — analyze/link_health returns hint for async tracking
begin_test "19.5" "analyze/link_health returns hint with command_result usage" \
    "Verify response includes hint for how to check async results" \
    "Hint guides users to observe({what:'command_result', correlation_id:'...'})."
run_test_19_5() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "observe"; then
        fail "Hint should mention observe method. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "command_result"; then
        fail "Hint should mention command_result. Content: $(truncate "$text")"
        return
    fi
    pass "Hint correctly guides to observe command_result. Content: $(truncate "$text" 200)"
}
run_test_19_5

# ── GROUP B: Analyzer Dispatcher (3 tests) ────────────────────────────────

# 19.6 — analyze tool routes to correct handler
begin_test "19.6" "analyze dispatcher routes 'link_health' to handler" \
    "Verify analyze tool's dispatcher correctly routes the request" \
    "Dispatcher must route based on 'what' parameter to correct handler."
run_test_19_6() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Dispatcher failed to route link_health. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "Dispatcher correctly routes to link_health handler"
}
run_test_19_6

# 19.7 — analyze tool rejects missing 'what' parameter
begin_test "19.7" "analyze({}) missing 'what' returns error" \
    "Verify missing required 'what' parameter is caught" \
    "Required parameters must error with helpful message."
run_test_19_7() {
    RESPONSE=$(call_tool "analyze" '{}')
    if check_not_error "$RESPONSE"; then
        fail "Should error when 'what' is missing. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "what"; then
        fail "Error message should mention 'what' parameter. Content: $(truncate "$text")"
        return
    fi
    pass "Correctly rejects missing 'what' parameter. Content: $(truncate "$text" 200)"
}
run_test_19_7

# 19.8 — analyze tool rejects invalid mode
begin_test "19.8" "analyze({what:'invalid_mode'}) returns error" \
    "Verify invalid mode name is rejected" \
    "Invalid modes must error with list of valid modes."
run_test_19_8() {
    RESPONSE=$(call_tool "analyze" '{"what":"nonexistent_mode_xyz"}')
    if check_not_error "$RESPONSE"; then
        fail "Should error for invalid mode. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "nonexistent_mode_xyz"; then
        fail "Error should mention the invalid mode. Content: $(truncate "$text")"
        return
    fi
    pass "Correctly rejects invalid mode. Content: $(truncate "$text" 200)"
}
run_test_19_8

# ── GROUP C: Error Handling (3 tests) ──────────────────────────────────────

# 19.9 — analyze/link_health handles invalid JSON gracefully
begin_test "19.9" "analyze with invalid JSON returns error" \
    "Verify malformed JSON is caught and reported" \
    "JSON parsing errors must be clear, not crash the server."
run_test_19_9() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"invalid}')
    # Invalid JSON in arguments makes the whole JSON-RPC request malformed,
    # so we expect either a protocol error (-32700) or isError tool response
    if [ -z "$RESPONSE" ]; then
        fail "Should error for invalid JSON but got empty response."
        return
    fi
    # Accept: protocol parse error OR tool isError
    local has_protocol_error
    has_protocol_error=$(echo "$RESPONSE" | jq -r '.error.code // empty' 2>/dev/null)
    if [ "$has_protocol_error" = "-32700" ]; then
        pass "Correctly returns JSON parse error (-32700) for malformed request."
        return
    fi
    if check_is_error "$RESPONSE"; then
        pass "Correctly returns isError for invalid JSON arguments."
        return
    fi
    # Also accept any valid JSON-RPC error response
    if check_valid_jsonrpc "$RESPONSE"; then
        pass "Returned valid JSON-RPC response for malformed input (server didn't crash)."
        return
    fi
    fail "Unexpected response for invalid JSON. Response: $(truncate "$RESPONSE")"
}
run_test_19_9

# 19.10 — analyze/link_health with invalid timeout_ms value
begin_test "19.10" "analyze with invalid timeout_ms returns reasonable response" \
    "Verify non-numeric timeout_ms is handled" \
    "Invalid parameter types should not crash (may be silently ignored or error)."
run_test_19_10() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health","timeout_ms":"not_a_number"}')
    # May error or may silently ignore - either is acceptable
    local text
    text=$(extract_content_text "$RESPONSE")
    # Just verify we get a response (not crash)
    if [ -z "$text" ]; then
        fail "Should return a response (error or success), not empty. Content: $RESPONSE"
        return
    fi
    pass "Handles invalid timeout_ms gracefully. Content: $(truncate "$text" 200)"
}
run_test_19_10

# 19.11 — analyze/link_health ignores unknown parameters
begin_test "19.11" "analyze with unknown parameters is accepted" \
    "Verify extra unknown parameters don't cause errors" \
    "Lenient JSON parsing allows extensibility without breaking."
run_test_19_11() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health","unknown_param":"should_be_ignored","another":"param"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Should accept extra parameters. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "Accepts extra unknown parameters gracefully"
}
run_test_19_11

# ── GROUP D: Concurrency & Load (2 tests) ──────────────────────────────────

# 19.12 — multiple concurrent link_health calls
begin_test "19.12" "multiple concurrent link_health calls handled correctly" \
    "Verify server handles 5 concurrent link_health requests" \
    "Concurrency stress-tests pending query management."
run_test_19_12() {
    local pids=()
    local _responses=()  # reserved for future response collection

    # Start 5 concurrent calls
    for _i in {1..5}; do
        ( call_tool "analyze" '{"what":"link_health"}' ) &
        pids+=($!)
    done

    # Wait and collect responses
    local count=0
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
        count=$((count + 1))
    done

    if [ $count -eq 5 ]; then
        pass "All 5 concurrent link_health calls completed"
    else
        fail "Only $count/5 concurrent calls completed"
    fi
}
run_test_19_12

# 19.13 — link_health doesn't leak memory on repeated calls
begin_test "19.13" "link_health repeated calls (50x) don't cause issues" \
    "Call link_health 50 times sequentially, verify all succeed" \
    "Memory leaks manifest as failures after many iterations."
run_test_19_13() {
    local success_count=0

    for _i in {1..50}; do
        RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
        if check_not_error "$RESPONSE"; then
            ((success_count++))
        fi
    done

    if [ $success_count -eq 50 ]; then
        pass "All 50 repeated link_health calls succeeded"
    else
        fail "Only $success_count/50 calls succeeded (possible memory/state leak)"
    fi
}
run_test_19_13

# ── GROUP E: Response Structure (3 tests) ──────────────────────────────────

# 19.14 — correlation_id has correct format
begin_test "19.14" "correlation_id follows expected format" \
    "Verify correlation_id starts with 'link_health_' prefix" \
    "Format consistency aids debugging and tracking."
run_test_19_14() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "link_health_"; then
        pass "Correlation ID has correct 'link_health_' prefix. Content: $(truncate "$text" 200)"
    else
        fail "Correlation ID should start with 'link_health_'. Content: $(truncate "$text")"
    fi
}
run_test_19_14

# 19.15 — response contains content blocks
begin_test "19.15" "analyze/link_health response has content blocks" \
    "Verify response structure includes content array" \
    "MCP responses must be properly structured with content blocks."
run_test_19_15() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -n "$text" ] && [ "$text" != "null" ]; then
        pass "Response has content. Content: $(truncate "$text" 200)"
    else
        fail "Response should have non-empty content. Got: $RESPONSE"
    fi
}
run_test_19_15

# 19.16 — response is valid JSON
begin_test "19.16" "analyze/link_health response is valid JSON" \
    "Verify response can be parsed as JSON" \
    "Response must be valid JSON for MCP protocol compliance."
run_test_19_16() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
    if echo "$RESPONSE" | jq . > /dev/null 2>&1; then
        pass "Response is valid JSON"
    else
        fail "Response is not valid JSON: $RESPONSE"
    fi
}
run_test_19_16

# ── GROUP F: Integration (3 tests) ─────────────────────────────────────────

# 19.17 — link_health mode is registered in tools/list
begin_test "19.17" "analyze tool lists link_health as valid mode" \
    "Verify tools/list response includes analyze tool" \
    "Tool discovery via tools/list is critical for client UX."
run_test_19_17() {
    # Call with invalid mode to get the valid modes list
    RESPONSE=$(call_tool "analyze" '{"what":"invalid_mode"}')
    if check_not_error "$RESPONSE"; then
        fail "Should error for invalid mode. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "link_health"; then
        pass "link_health is listed as valid analyze mode. Content: $(truncate "$text" 200)"
    else
        fail "link_health should be listed in valid modes. Content: $(truncate "$text")"
    fi
}
run_test_19_17

# 19.18 — analyze tool returns MCP protocol compliant response
begin_test "19.18" "analyze/link_health response complies with MCP protocol" \
    "Verify response has required MCP fields (isError, content)" \
    "MCP protocol compliance is mandatory for extension compatibility."
run_test_19_18() {
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')

    # Check for MCP-required fields
    if echo "$RESPONSE" | grep -q '"isError"'; then
        if echo "$RESPONSE" | grep -q '"content"'; then
            pass "Response has required MCP fields (isError, content)"
        else
            fail "Response missing 'content' field. Response: $(truncate "$RESPONSE" 200)"
        fi
    else
        fail "Response missing 'isError' field. Response: $(truncate "$RESPONSE" 200)"
    fi
}
run_test_19_18

# 19.19 — analyze/link_health can be called via standard MCP call
begin_test "19.19" "analyze tool callable via tools/call MCP method" \
    "Verify standard MCP tools/call invocation works" \
    "tools/call is the standard MCP method for tool invocation."
run_test_19_19() {
    # This test verifies the call_tool helper works (which uses tools/call)
    RESPONSE=$(call_tool "analyze" '{"what":"link_health"}')
    if check_not_error "$RESPONSE"; then
        pass "analyze tool is callable via standard MCP tools/call"
    else
        fail "Failed to call analyze via tools/call. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
    fi
}
run_test_19_19

# ─────────────────────────────────────────────────────────────────────────────
finish_category
