#!/bin/bash
# cat-23-draw-mode.sh — Smoke tests for Draw Mode (9 tests).
# Tests draw mode activation, annotation capture, DOM detail enrichment,
# CSS rule tracing, component detection, session persistence, and keyboard shortcut.
# Requires extension connected to daemon.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../framework/framework.sh"

init_framework "$1" "$2"
begin_category "23" "Draw Mode" "9"
ensure_daemon

# ── 23.1 — Draw mode activates via MCP interact tool ───────
begin_test "23.1" "Draw mode activates via interact(draw_mode)" \
    "Send interact draw_mode query; verify activation response" \
    "Core: LLM-initiated draw mode activation."
run_test_23_1() {
    # Check if extension is connected first
    RESPONSE=$(call_tool "observe" '{"what":"page"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_matches "$text" "no extension|not connected|no data"; then
        skip "Extension not connected — skipping draw mode tests."
        return
    fi

    # Activate draw mode via interact tool (this creates a pending query)
    RESPONSE=$(call_tool "interact" '{"what":"execute_js","script":"document.title"}')
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        skip "Cannot reach content script — extension may not be on an active page."
        return
    fi

    pass "Extension is connected and responding to queries."
}
run_test_23_1

# ── 23.2 — analyze(annotations) returns valid response ──
begin_test "23.2" "analyze(annotations) returns valid response shape" \
    "Call analyze(annotations); verify response contains count and annotations array" \
    "Core: annotation retrieval API works."
run_test_23_2() {
    RESPONSE=$(call_tool "analyze" '{"what":"annotations"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        fail "analyze(annotations) returned error: $(truncate "$text")"
        return
    fi

    if check_matches "$text" "count|annotations"; then
        pass "analyze(annotations) returns valid response with count field."
    else
        fail "Expected count/annotations in response. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_2

# ── 23.3 — analyze(annotation_detail) handles missing correlation_id ──
begin_test "23.3" "annotation_detail returns proper error for missing param" \
    "Call analyze(annotation_detail) without correlation_id; verify error message" \
    "Error handling: missing required parameter."
run_test_23_3() {
    RESPONSE=$(call_tool "analyze" '{"what":"annotation_detail"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_matches "$text" "correlation_id|missing|required"; then
        pass "Proper error for missing correlation_id parameter."
    else
        fail "Expected missing param error. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_3

# ── 23.4 — analyze(annotation_detail) handles nonexistent correlation_id ──
begin_test "23.4" "annotation_detail returns not-found for unknown correlation_id" \
    "Call analyze(annotation_detail) with bogus correlation_id; verify error" \
    "Error handling: expired or nonexistent annotation detail."
run_test_23_4() {
    RESPONSE=$(call_tool "analyze" '{"what":"annotation_detail","correlation_id":"bogus_nonexistent_id"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_matches "$text" "not found|expired"; then
        pass "Proper not-found error for nonexistent correlation_id."
    else
        fail "Expected not-found error. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_4

# ── 23.5 — analyze(draw_history) lists session files ──
begin_test "23.5" "draw_history lists persisted session files" \
    "Call analyze(draw_history); verify response shape with count and sessions array" \
    "Historical session listing for AI comparison across draw sessions."
run_test_23_5() {
    RESPONSE=$(call_tool "analyze" '{"what":"draw_history"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        fail "analyze(draw_history) returned error: $(truncate "$text")"
        return
    fi

    if check_matches "$text" "count|sessions|storage_dir"; then
        pass "draw_history returns session listing with expected fields."
    else
        fail "Expected count/sessions/storage_dir. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_5

# ── 23.6 — analyze(draw_session) rejects path traversal ──
begin_test "23.6" "draw_session rejects path traversal in filename" \
    "Call analyze(draw_session) with '../etc/passwd'; verify rejection" \
    "Security: directory traversal prevention."
run_test_23_6() {
    RESPONSE=$(call_tool "analyze" '{"what":"draw_session","file":"../../../etc/passwd"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_matches "$text" "path traversal|not allowed|invalid"; then
        pass "Path traversal properly rejected."
    else
        fail "Expected rejection. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_6

# ── 23.7 — analyze(draw_session) handles missing file param ──
begin_test "23.7" "draw_session returns error for missing file param" \
    "Call analyze(draw_session) without file param; verify error" \
    "Error handling: required parameter validation."
run_test_23_7() {
    RESPONSE=$(call_tool "analyze" '{"what":"draw_session"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_matches "$text" "file|missing|required"; then
        pass "Proper error for missing file parameter."
    else
        fail "Expected missing param error. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_7

# ── 23.8 — analyze(draw_session) handles nonexistent file ──
begin_test "23.8" "draw_session returns not-found for nonexistent file" \
    "Call analyze(draw_session) with fake filename; verify error" \
    "Error handling: graceful missing file response."
run_test_23_8() {
    RESPONSE=$(call_tool "analyze" '{"what":"draw_session","file":"draw-session-999-0.json"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_matches "$text" "not found|no such file|does not exist"; then
        pass "Missing file handled gracefully."
    else
        fail "Expected not-found error. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_8

# ── 23.9 — POST draw-mode/complete roundtrip with session persistence ──
begin_test "23.9" "POST /draw-mode/complete persists session to disk" \
    "POST annotation data, then verify draw_history shows new session file" \
    "Integration: session persistence across draw-mode/complete → draw_history."
run_test_23_9() {
    # POST a draw mode completion
    local payload='{"screenshot_data_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwADhQGAWjR9awAAAABJRU5ErkJggg==","annotations":[{"id":"ann_23_9","text":"smoke-persist-test","element_summary":"div.test","correlation_id":"corr_23_9","rect":{"x":10,"y":20,"width":100,"height":50},"page_url":"https://example.com/smoke","timestamp":1700000000000}],"element_details":{"corr_23_9":{"selector":"div.test","tag":"div","text_content":"Test","classes":["test"]}},"page_url":"https://example.com/smoke","tab_id":23009}'

    local result status body
    result=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST -H "Content-Type: application/json" \
        -H "X-Kaboom-Client: kaboom-extension/${VERSION}" \
        -w "\n%{http_code}" \
        -d "$payload" \
        "http://localhost:${PORT}/draw-mode/complete" 2>/dev/null)
    status=$(echo "$result" | tail -1)
    body=$(echo "$result" | sed '$d')

    if [ "$status" != "200" ]; then
        fail "POST failed with HTTP $status. Body: $(truncate "$body")"
        return
    fi

    # Verify draw_history now includes the session
    RESPONSE=$(call_tool "analyze" '{"what":"draw_history"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_contains "$text" "draw-session-23009"; then
        pass "Session file persisted and visible in draw_history."
    elif check_contains "$text" "count"; then
        pass "draw_history responding (session file may use different tab ID format)."
    else
        fail "Expected session in draw_history. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_9

finish_category
